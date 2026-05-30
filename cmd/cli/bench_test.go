// Cold-start benchmark for `secret run`. The PRD's headline latency
// budget is p95 ≤ 300 ms end-to-end (exec → child reads the secret).
//
// What this measures:
//
//   1. fork+exec of the secret binary (Cobra subcommand resolution,
//      package init, flag parsing)
//   2. credentials.json load
//   3. HTTP round-trip to the local server
//   4. AES-256-GCM decrypt of one secret
//   5. fork+exec of a trivial child that exits immediately
//
// What it does NOT measure:
//
//   - The first-time `go build` of the secret binary (we build once
//     in the bench setup; per-iter overhead is just exec)
//   - Network latency across machines (we use loopback)
//
// Why we keep a regression test alongside the benchmark: `go test
// -bench` doesn't fail on a budget violation — it just prints
// ns/op. We need a hard gate, so TestSecretRunColdStartP95 runs the
// same harness with 20 samples and asserts p95 < 300 ms.

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/idenn207/comax-secrets/internal/cli/credentials"
	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/server"
	"github.com/idenn207/comax-secrets/internal/store"
)

// coldStartP95BudgetMs is the plan's gate. Bumped CI-side under
// COMAX_BENCH_BUDGET_MS for slower runners or local dev; this is the
// hard default applied to GH ubuntu-latest.
const coldStartP95BudgetMs = 300

// childArgs returns the cross-platform invocation of a trivial
// no-op child process. The child must spawn and exit fast enough
// that we are measuring the secret CLI overhead, not the child.
func childArgs() []string {
	if runtime.GOOS == "windows" {
		// `cmd /c ver` writes a single line and exits in ~20 ms on a
		// modern Windows host — a typical exec-cost floor.
		return []string{"cmd", "/c", "ver"}
	}
	// `true` is a builtin or /usr/bin/true; either way it's the
	// minimum-cost child process available.
	return []string{"true"}
}

// benchFixture is the shared setup for the benchmark and the
// regression test. It builds the CLI binary once, starts a server,
// bootstraps + seeds, and returns the absolute paths needed for an
// invocation.
type benchFixture struct {
	binary   string
	credPath string
	cwd      string
	srv      *httptest.Server
}

func (f *benchFixture) close() {
	if f.srv != nil {
		f.srv.Close()
	}
}

// setupColdStartFixture stands up the server + CLI binary. The TB
// abstraction lets benchmarks and tests share it.
func setupColdStartFixture(tb testing.TB) *benchFixture {
	tb.Helper()
	// Build the CLI binary into a temp dir. Use a fresh tempdir, not
	// t.TempDir(), so we can choose the path explicitly and let the
	// OS clean it on test exit via tb.Cleanup.
	binDir := tb.TempDir()
	binPath := filepath.Join(binDir, "secret")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", binPath, "github.com/idenn207/comax-secrets/cmd/cli")
	if out, err := build.CombinedOutput(); err != nil {
		tb.Fatalf("build CLI: %v\n%s", err, out)
	}

	// Server with an in-memory DB and a random master key.
	db, err := store.Open(filepath.Join(tb.TempDir(), "bench.db"))
	if err != nil {
		tb.Fatalf("store.Open: %v", err)
	}
	tb.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		tb.Fatalf("Migrate: %v", err)
	}
	key := make([]byte, crypto.KeySize)
	if _, err := rand.Read(key); err != nil {
		tb.Fatalf("rand: %v", err)
	}
	srv := httptest.NewServer(server.NewServer(server.Options{
		DB: db, Keys: staticKey(key),
	}).Handler())
	tb.Cleanup(srv.Close)

	// Bootstrap.
	resp, err := srv.Client().Post(srv.URL+"/api/v1/bootstrap", "application/json", nil)
	if err != nil {
		tb.Fatalf("bootstrap: %v", err)
	}
	var body struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		tb.Fatalf("read bootstrap: %v", err)
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		tb.Fatalf("decode bootstrap: %v: %s", err, raw)
	}

	// Persist credentials + seed project/env via the CLI itself so we
	// exercise the same code path the real operator uses.
	cwd := tb.TempDir()
	credPath := filepath.Join(tb.TempDir(), "creds.json")
	if err := credentials.SaveTo(credPath, credentials.Credentials{Server: srv.URL, Token: body.Data.Token}); err != nil {
		tb.Fatalf("creds: %v", err)
	}
	if err := secretrc.Save(cwd, secretrc.Config{Project: "comax", DefaultEnv: "dev"}); err != nil {
		tb.Fatalf("secretrc: %v", err)
	}
	// Drive the binary to create the project and seed one secret. The
	// per-iter call only does `run`, so the cost of init + set is
	// amortised out of the bench timing.
	for _, args := range [][]string{
		{"--credentials", credPath, "init", "--project", "comax", "--envs", "dev,prod", "--default-env", "dev"},
		{"--credentials", credPath, "set", "BENCH_KEY=hello", "--quiet"},
	} {
		cmd := exec.Command(binPath, args...)
		cmd.Dir = cwd
		if out, err := cmd.CombinedOutput(); err != nil {
			tb.Fatalf("seed %v: %v\n%s", args, err, out)
		}
	}
	return &benchFixture{binary: binPath, credPath: credPath, cwd: cwd, srv: srv}
}

// singleColdStart runs one `secret run -- <child>` and returns the
// wall-clock duration. Used by both the benchmark and the
// percentile test.
func singleColdStart(fx *benchFixture) (time.Duration, error) {
	args := append([]string{"--credentials", fx.credPath, "run", "--quiet", "--"}, childArgs()...)
	cmd := exec.Command(fx.binary, args...)
	cmd.Dir = fx.cwd
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("run: %w", err)
	}
	return time.Since(start), nil
}

// BenchmarkSecretRunColdStart reports ns/op. Compare on the GH
// ubuntu-latest runner profile; expect ~80–200 ms on idle hardware
// with room under the 300 ms gate.
func BenchmarkSecretRunColdStart(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping cold-start bench in -short mode")
	}
	fx := setupColdStartFixture(b)
	defer fx.close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := singleColdStart(fx); err != nil {
			b.Fatalf("iter %d: %v", i, err)
		}
	}
}

// TestSecretRunColdStartP95Budget gates p95 against the 300 ms
// budget. Skipped under -short because building + 20 spawns is too
// expensive for the per-commit test loop; CI runs it explicitly.
func TestSecretRunColdStartP95Budget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cold-start budget gate in -short mode")
	}
	if v := os.Getenv("COMAX_SKIP_BENCH"); v == "1" {
		t.Skip("COMAX_SKIP_BENCH=1")
	}
	budgetMs := coldStartP95BudgetMs
	if v := os.Getenv("COMAX_BENCH_BUDGET_MS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			budgetMs = n
		}
	}

	fx := setupColdStartFixture(t)
	defer fx.close()

	// Warm-up to amortise any first-call costs (DNS cache, http.Client
	// keepalives, JIT-warming of go's runtime). The plan's budget
	// applies to *cold* start of the CLI itself, but the *server* may
	// reasonably be warm — it's a long-running process in production.
	for i := 0; i < 2; i++ {
		_, _ = singleColdStart(fx)
	}

	const samples = 20
	durs := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		d, err := singleColdStart(fx)
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		durs = append(durs, d)
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	p95 := durs[int(float64(samples)*0.95)-1]
	p50 := durs[samples/2-1]
	t.Logf("cold-start p50=%s p95=%s (budget=%dms)", p50, p95, budgetMs)
	if p95 > time.Duration(budgetMs)*time.Millisecond {
		t.Fatalf("p95 cold start %s exceeds budget %dms (samples sorted: %v)", p95, budgetMs, durs)
	}
}
