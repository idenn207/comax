package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
)

// runChildCapturingStdout runs the CLI with args against credPath and
// returns the child's stdout. It builds its own root so os.Exit on the
// child-failure branch is avoided (we only pass succeeding children).
func runChildCapturingStdout(t *testing.T, credPath string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(append([]string{"--credentials", credPath}, args...))
	err := root.Execute()
	if err != nil {
		return stdout.String() + stderr.String(), err
	}
	return stdout.String(), nil
}

// TestLoadContext_ProjectFlagWorksWithoutSecretrc is the CI scenario: no
// .secretrc is checked out on a runner, so --project must supply the
// project. We seed a secret while .secretrc still exists, delete it, then
// prove `run --project` resolves and injects the secret anyway.
func TestLoadContext_ProjectFlagWorksWithoutSecretrc(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "CI_KEY=ci-value", "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Remove(filepath.Join(cwd, secretrc.FileName)); err != nil {
		t.Fatalf("remove .secretrc: %v", err)
	}

	args := append([]string{"run", "--project", "comax", "--env", "dev", "--quiet", "--"}, printenvCmd("CI_KEY")...)
	out, err := runChildCapturingStdout(t, credPath, args...)
	if err != nil {
		t.Fatalf("run --project without .secretrc: %v (out=%s)", err, out)
	}
	if got := strings.TrimSpace(out); got != "ci-value" {
		t.Errorf("child CI_KEY=%q; want ci-value", got)
	}
}

// TestLoadContext_ProjectFlagOverridesSecretrc pins .secretrc to a project
// that does not exist on the server, then passes --project pointing at the
// real one. Success proves the flag (not .secretrc) drove resolution — had
// .secretrc won, the pull would 404 against the bogus project.
func TestLoadContext_ProjectFlagOverridesSecretrc(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "OK=yes", "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := secretrc.Save(cwd, secretrc.Config{Project: "ghost-nonexistent", DefaultEnv: "dev"}); err != nil {
		t.Fatalf("repin .secretrc: %v", err)
	}

	args := append([]string{"run", "--project", "comax", "--env", "dev", "--quiet", "--"}, printenvCmd("OK")...)
	out, err := runChildCapturingStdout(t, credPath, args...)
	if err != nil {
		t.Fatalf("run --project override: %v (out=%s)", err, out)
	}
	if got := strings.TrimSpace(out); got != "yes" {
		t.Errorf("child OK=%q; want yes (--project should override .secretrc)", got)
	}
}

// TestLoadContext_NoProjectNoSecretrcErrors confirms the failure path is
// explicit rather than silent: no --project and no .secretrc must return
// an error that mentions the project so operators know how to fix it.
func TestLoadContext_NoProjectNoSecretrcErrors(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	if err := os.Remove(filepath.Join(cwd, secretrc.FileName)); err != nil {
		t.Fatalf("remove .secretrc: %v", err)
	}
	// loadContext fails before any child is spawned, so `true` is never run.
	_, err := runCmd(t, credPath, "run", "--env", "dev", "--quiet", "--", "true")
	if err == nil {
		t.Fatal("run without --project or .secretrc returned nil; want error")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Errorf("error = %v; want it to mention project", err)
	}
}
