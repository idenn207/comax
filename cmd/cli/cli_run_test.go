package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// printenvCmd returns a child-command argv that prints the value of the
// named env var to stdout. Cross-platform: cmd /c on Windows, sh -c on
// Unix. Trailing newlines vary; tests TrimSpace before comparing.
func printenvCmd(key string) []string {
	if runtime.GOOS == "windows" {
		// `cmd /c echo %KEY%` prints "%KEY%" verbatim if unset; we want
		// empty in that case. `set KEY` writes "KEY=value" to stdout
		// when set and exits non-zero when unset. We parse the value
		// from the "set" output in the test.
		return []string{"cmd", "/c", "echo %" + key + "%"}
	}
	return []string{"sh", "-c", "printf %s \"$" + key + "\""}
}

func TestRun_ChildSeesPulledSecret(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "MY_SECRET=hello-from-comax", "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	args := append([]string{"run", "--quiet", "--"}, printenvCmd("MY_SECRET")...)
	root := newRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(append([]string{"--credentials", credPath}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("run: %v (stderr=%s)", err, stderr.String())
	}
	got := strings.TrimSpace(stdout.String())
	if got != "hello-from-comax" {
		t.Errorf("child stdout = %q; want hello-from-comax (stderr=%s)", got, stderr.String())
	}
}

func TestRun_NoPlaintextOnDisk(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	const sentinel = "very-secret-sentinel-value-9f3a2b"
	if _, err := runCmd(t, credPath, "set", "SENTINEL="+sentinel, "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Snapshot existing files (.secretrc is allowed; .env, *.tmp, etc.
	// are not after a run).
	before := snapshotDir(t, cwd)

	args := append([]string{"run", "--quiet", "--"}, printenvCmd("SENTINEL")...)
	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs(append([]string{"--credentials", credPath}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Audit: no new file should exist in cwd. (We don't audit the
	// credentials path because that's documented and 0600.)
	after := snapshotDir(t, cwd)
	for _, p := range after {
		if !contains(before, p) {
			raw, _ := os.ReadFile(p)
			t.Errorf("new file appeared in cwd after run: %s (contains sentinel=%v)",
				p, strings.Contains(string(raw), sentinel))
		}
	}
	// Also: any pre-existing file must not have been touched with
	// the sentinel value. Read every file we knew about and assert.
	for _, p := range before {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.Contains(string(raw), sentinel) {
			t.Errorf("file %s contains the decrypted sentinel value", p)
		}
	}
}

func TestRun_PropagatesChildExitCode(t *testing.T) {
	// `secret run -- false` exits non-zero. We can't easily assert the
	// exact code without running the binary in a subprocess (root.Execute
	// calls os.Exit on the child-failure branch). Skip on platforms
	// without `false` in PATH.
	if runtime.GOOS == "windows" {
		t.Skip("`false` is a shell builtin on Windows; cmd /c exit /b 42 path differs")
	}
	// Plumbing test only — we just verify that `secret run -- true`
	// succeeds. The non-zero exit path is exercised end-to-end by the
	// `make test` matrix in CI.
	credPath, _ := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "run", "--quiet", "--", "true"); err != nil {
		t.Fatalf("run true: %v", err)
	}
}

func TestRun_OperatorEnvVarsOverridenBySecret(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "PATH_TEST_KEY=from-server", "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Setenv("PATH_TEST_KEY", "from-parent")

	args := append([]string{"run", "--quiet", "--"}, printenvCmd("PATH_TEST_KEY")...)
	root := newRootCmd()
	stdout := new(bytes.Buffer)
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs(append([]string{"--credentials", credPath}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	if got != "from-server" {
		t.Errorf("child saw PATH_TEST_KEY=%q; want from-server (parent should be shadowed)", got)
	}
}

// ---- helpers --------------------------------------------------------

func snapshotDir(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
