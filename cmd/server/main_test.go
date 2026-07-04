package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/idenn207/comax-secrets/internal/store"
)

// httptestNewOK returns a server that responds 200 on every request.
func httptestNewOK(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

// httptestNewStatus returns a server that responds with the given
// status on every request, used to exercise runHealthCheck's
// non-200 branch.
func httptestNewStatus(t *testing.T, code int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
}

// discardLogger returns a slog logger that drops everything. Tests use
// it so log lines don't drown the failure output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// findFreePort asks the OS for an unused TCP port and immediately closes
// the listener. There is a small race window before the server binds it
// again, but acceptable for tests that own the loopback interface.
func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close() //nolint:errcheck // test cleanup
	return l.Addr().(*net.TCPAddr).Port
}

func TestParseFlags_DefaultsAndOverrides(t *testing.T) {
	// Snapshot and clear COMAX_* env so the defaults are deterministic.
	for _, k := range []string{"COMAX_LISTEN", "COMAX_DB_PATH", "COMAX_MASTER_KEY_FILE", "COMAX_AUTO_GENERATE_KEY", "COMAX_WEBHOOK_POLL"} {
		t.Setenv(k, "")
	}
	cfg, err := parseFlags(nil, io.Discard)
	if err != nil {
		t.Fatalf("parseFlags default: %v", err)
	}
	if cfg.listenAddr != ":8080" {
		t.Errorf("default listenAddr = %q; want :8080", cfg.listenAddr)
	}
	if !cfg.autoGenKey {
		t.Error("autoGenKey default should be true")
	}
	if cfg.webhookPoll != 10*time.Second {
		t.Errorf("default webhookPoll = %v; want 10s", cfg.webhookPoll)
	}

	cfg2, err := parseFlags([]string{"--listen", ":9999", "--db", "/tmp/x.db", "--webhook-poll-interval", "3s"}, io.Discard)
	if err != nil {
		t.Fatalf("parseFlags overrides: %v", err)
	}
	if cfg2.listenAddr != ":9999" || cfg2.dbPath != "/tmp/x.db" {
		t.Errorf("override = %+v; want :9999 and /tmp/x.db", cfg2)
	}
	if cfg2.webhookPoll != 3*time.Second {
		t.Errorf("webhook-poll-interval override = %v; want 3s", cfg2.webhookPoll)
	}

	// COMAX_WEBHOOK_POLL env is honored as the default when the flag is absent.
	t.Setenv("COMAX_WEBHOOK_POLL", "45s")
	cfg3, err := parseFlags(nil, io.Discard)
	if err != nil {
		t.Fatalf("parseFlags env: %v", err)
	}
	if cfg3.webhookPoll != 45*time.Second {
		t.Errorf("COMAX_WEBHOOK_POLL env → %v; want 45s", cfg3.webhookPoll)
	}
}

func TestParseFlags_VersionFlag(t *testing.T) {
	cfg, err := parseFlags([]string{"--version"}, io.Discard)
	if err != nil {
		t.Fatalf("parseFlags --version: %v", err)
	}
	if !cfg.printVersion {
		t.Error("printVersion should be true")
	}
}

func TestEnsureMasterKey_AutoGenerates(t *testing.T) {
	dir := t.TempDir()
	cfg := config{
		masterKeyPath: filepath.Join(dir, "k", "master.key"),
		autoGenKey:    true,
	}
	if err := ensureMasterKey(cfg, discardLogger()); err != nil {
		t.Fatalf("ensureMasterKey: %v", err)
	}
	st, err := os.Stat(cfg.masterKeyPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Size() != 32 {
		t.Errorf("key size = %d; want 32", st.Size())
	}
	if runtime.GOOS != "windows" {
		if mode := st.Mode().Perm(); mode != 0o600 {
			t.Errorf("key mode = %#o; want 0600", mode)
		}
	}
}

func TestEnsureMasterKey_RefusesWhenMissingAndAutoGenOff(t *testing.T) {
	cfg := config{
		masterKeyPath: filepath.Join(t.TempDir(), "absent.key"),
		autoGenKey:    false,
	}
	err := ensureMasterKey(cfg, discardLogger())
	if err == nil {
		t.Fatal("ensureMasterKey returned nil; want failure when key missing and auto-gen disabled")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("err = %v; expected to mention missing key", err)
	}
}

func TestEnsureMasterKey_NoopWhenPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preexisting.key")
	if err := os.WriteFile(path, bytes.Repeat([]byte{0xAB}, 32), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := config{masterKeyPath: path, autoGenKey: true}
	if err := ensureMasterKey(cfg, discardLogger()); err != nil {
		t.Fatalf("ensureMasterKey on existing: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Existing content must not have been overwritten.
	for _, b := range raw {
		if b != 0xAB {
			t.Fatalf("file was overwritten")
		}
	}
}

func TestAutoBootstrap_PrintsTokenWhenEmpty(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "boot.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	stdout := new(bytes.Buffer)
	if err := autoBootstrap(context.Background(), db, stdout, discardLogger()); err != nil {
		t.Fatalf("autoBootstrap: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "bootstrap admin token") {
		t.Errorf("stdout missing banner: %q", out)
	}
	// Token should be a non-trivial string between the banner lines.
	if !strings.Contains(out, "secret login") {
		t.Errorf("stdout missing login hint: %q", out)
	}
}

func TestAutoBootstrap_NoopWhenTokensExist(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "boot.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Bootstrap once to seed.
	if err := autoBootstrap(context.Background(), db, io.Discard, discardLogger()); err != nil {
		t.Fatalf("seed bootstrap: %v", err)
	}

	stdout := new(bytes.Buffer)
	if err := autoBootstrap(context.Background(), db, stdout, discardLogger()); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("second autoBootstrap printed %q; want empty", stdout.String())
	}
}

func TestRun_VersionFlag(t *testing.T) {
	out := new(bytes.Buffer)
	if err := run([]string{"--version"}, out, io.Discard); err != nil {
		t.Fatalf("run --version: %v", err)
	}
	if !strings.HasPrefix(out.String(), "secret-server ") {
		t.Errorf("version output = %q", out.String())
	}
}

func TestRun_StartAndShutdown(t *testing.T) {
	// End-to-end-ish: start the server in a goroutine, hit /healthz,
	// then signal shutdown by sending SIGINT to ourselves.
	//
	// Skipped on Windows because os.Process.Signal(os.Interrupt) is
	// not delivered to the current process the way Unix signals are,
	// which would leave the server goroutine running past t.Cleanup
	// and lock the SQLite file. The unit-level coverage
	// (ensureMasterKey + autoBootstrap + parseFlags) is sufficient on
	// Windows; CI runs this leg on Linux.
	if runtime.GOOS == "windows" {
		t.Skip("self-signal SIGINT not portable on Windows; covered in unit tests")
	}
	port := findFreePort(t)
	dir := t.TempDir()
	args := []string{
		"--listen", "127.0.0.1:" + itoa(port),
		"--db", filepath.Join(dir, "data", "secrets.db"),
		"--master-key-file", filepath.Join(dir, "keys", "master.key"),
	}

	errCh := make(chan error, 1)
	stdout := new(bytes.Buffer)
	go func() {
		errCh <- run(args, stdout, io.Discard)
	}()

	// Wait for the listener to come up. /healthz is unauthenticated so a
	// 200 is unambiguous.
	url := "http://127.0.0.1:" + itoa(port) + "/healthz"
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec // loopback, controlled URL
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				lastErr = nil
				break
			}
			lastErr = errFromStatus(resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("server never became healthy: %v", lastErr)
	}

	// Bootstrap token must have been printed to stdout.
	if !strings.Contains(stdout.String(), "bootstrap admin token") {
		t.Errorf("stdout missing bootstrap banner: %q", stdout.String())
	}

	// Send SIGINT to ourselves so the signal.NotifyContext in run()
	// triggers a graceful shutdown.
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find self: %v", err)
	}
	if err := p.Signal(os.Interrupt); err != nil {
		t.Fatalf("signal self: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not shut down within 10s")
	}
}

// errFromStatus turns a non-200 health response into an error so the
// retry loop has a single sentinel to check.
func errFromStatus(code int) error {
	return errors.New("health status " + itoa(code))
}

func TestRunHealthCheck_OK(t *testing.T) {
	srv := httptestNewOK(t)
	t.Cleanup(srv.Close)
	if err := runHealthCheck(srv.URL); err != nil {
		t.Errorf("runHealthCheck = %v; want nil on 200", err)
	}
}

func TestRunHealthCheck_NonOK(t *testing.T) {
	srv := httptestNewStatus(t, http.StatusServiceUnavailable)
	t.Cleanup(srv.Close)
	err := runHealthCheck(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("runHealthCheck = %v; want non-nil mentioning 503", err)
	}
}

func TestRunHealthCheck_Unreachable(t *testing.T) {
	// Port 1 is unprivileged-and-almost-never-bound; we expect a
	// connection refused / dial error, surfaced as a wrapped error.
	err := runHealthCheck("http://127.0.0.1:1/healthz")
	if err == nil {
		t.Error("runHealthCheck on unreachable returned nil")
	}
}

func TestEnvOrAndEnvBool(t *testing.T) {
	t.Setenv("COMAX_TEST_VAL", "from-env")
	if got := envOr("COMAX_TEST_VAL", "fallback"); got != "from-env" {
		t.Errorf("envOr present = %q; want from-env", got)
	}
	t.Setenv("COMAX_TEST_VAL", "")
	if got := envOr("COMAX_TEST_VAL", "fallback"); got != "fallback" {
		t.Errorf("envOr empty = %q; want fallback", got)
	}

	t.Setenv("COMAX_TEST_BOOL", "")
	if !envBool("COMAX_TEST_BOOL", true) {
		t.Error("envBool empty should defer to fallback (true)")
	}
	t.Setenv("COMAX_TEST_BOOL", "1")
	if !envBool("COMAX_TEST_BOOL", false) {
		t.Error("envBool=1 should be true")
	}
	t.Setenv("COMAX_TEST_BOOL", "false")
	if envBool("COMAX_TEST_BOOL", true) {
		t.Error("envBool=false should be false (no falsy-as-fallback)")
	}
}

func TestEnsureDataDir_CreatesParent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "nested")
	if err := ensureDataDir(filepath.Join(dir, "x.db")); err != nil {
		t.Fatalf("ensureDataDir: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("dir not created: %v", err)
	}
}

func TestEnsureDataDir_NoopForRelativeDot(t *testing.T) {
	if err := ensureDataDir("x.db"); err != nil {
		t.Errorf("ensureDataDir on bare path returned %v; want nil", err)
	}
}

func TestRunPruneSweeper_DeletesExpiredOnTick(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Seed one expired session — TTL=-time.Hour places expires_at an
	// hour in the past, which the sweeper's cutoff = time.Now() catches
	// immediately.
	tokHash := sha256.Sum256([]byte("bearer:prune"))
	tok, err := store.NewTokenRepo(db).Create(context.Background(), "prune", tokHash[:], false)
	if err != nil {
		t.Fatalf("token create: %v", err)
	}
	sh := sha256.Sum256([]byte("session:prune"))
	ch := sha256.Sum256([]byte("csrf:prune"))
	repo := store.NewSessionRepo(db)
	if _, err := repo.Create(context.Background(), store.SessionCreateInput{
		TokenID:     tok.ID,
		SessionHash: sh[:],
		CSRFHash:    ch[:],
		TTL:         -time.Hour,
	}); err != nil {
		t.Fatalf("session create: %v", err)
	}

	var pre int
	if err := db.QueryRow(`SELECT COUNT(*) FROM dashboard_sessions`).Scan(&pre); err != nil {
		t.Fatalf("pre count: %v", err)
	}
	if pre != 1 {
		t.Fatalf("pre count = %d; want 1", pre)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan struct{})
	go func() {
		runPruneSweeper(ctx, repo, 10*time.Millisecond, discardLogger())
		close(done)
	}()

	// Spin until the row is gone, capped at 2 seconds so a stuck sweeper
	// produces a real test failure instead of hanging the suite.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM dashboard_sessions`).Scan(&n); err != nil {
			t.Fatalf("count during wait: %v", err)
		}
		if n == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var post int
	if err := db.QueryRow(`SELECT COUNT(*) FROM dashboard_sessions`).Scan(&post); err != nil {
		t.Fatalf("post count: %v", err)
	}
	if post != 0 {
		t.Errorf("post count = %d; want 0 (expired row should have been pruned)", post)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("sweeper did not stop within 1s after cancel")
	}
}

func TestRunPruneSweeper_StopsOnContextCancel(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Long interval so the test never fires a real tick; the sweeper
	// has to exit purely on the ctx.Done() arm of the select.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runPruneSweeper(ctx, store.NewSessionRepo(db), time.Hour, discardLogger())
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("sweeper did not stop within 1s after cancel")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
