package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idenn207/comax-secrets/internal/cli/credentials"
	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
)

// loggedInWorktree starts a server, runs login + init, and returns
// (server URL, credentials path, cwd). Subsequent commands run in the
// cwd against the live server.
func loggedInWorktree(t *testing.T) (string, string) {
	t.Helper()
	srv, token := startTestServer(t)
	credPath := filepath.Join(t.TempDir(), "creds.json")
	if err := credentials.SaveTo(credPath, credentials.Credentials{
		Server: srv.URL, Token: token,
	}); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}
	cwd := t.TempDir()
	pushd(t, cwd)
	if err := secretrc.Save(cwd, secretrc.Config{
		Project: "comax", DefaultEnv: "dev",
	}); err != nil {
		t.Fatalf("seed secretrc: %v", err)
	}
	// Create the project + env on the server so push/pull have somewhere
	// to land. Skip cobra and call the server directly via a single set
	// command which idempotently creates nothing — instead, we run init
	// to set up the project and dev env.
	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--credentials", credPath,
		"init", "--project", "comax", "--envs", "dev,prod",
		"--default-env", "dev",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return credPath, cwd
}

func runCmd(t *testing.T, credPath string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	root.SetOut(stdout)
	root.SetErr(stderr)
	full := append([]string{"--credentials", credPath}, args...)
	root.SetArgs(full)
	err := root.Execute()
	return stdout.String() + stderr.String(), err
}

func TestPushPullRoundTrip(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)

	envFile := filepath.Join(cwd, ".env")
	if err := os.WriteFile(envFile, []byte(
		"DB_URL=postgres://x\nAPI_KEY=secret123\nNOTES=\"a b c\"\n",
	), 0o644); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	if _, err := runCmd(t, credPath, "push", "--file", envFile, "--quiet"); err != nil {
		t.Fatalf("push: %v", err)
	}

	// Remove the file then pull and verify the contents come back
	// faithfully (modulo sort order — Emit is sorted).
	if err := os.Remove(envFile); err != nil {
		t.Fatalf("remove .env: %v", err)
	}
	if _, err := runCmd(t, credPath, "pull", "--out", envFile, "--quiet"); err != nil {
		t.Fatalf("pull: %v", err)
	}
	raw, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read pulled .env: %v", err)
	}
	got := string(raw)
	for _, want := range []string{"API_KEY=secret123", "DB_URL=postgres://x", `NOTES="a b c"`} {
		if !strings.Contains(got, want) {
			t.Errorf("pulled .env missing %q; got:\n%s", want, got)
		}
	}
}

func TestSetThenGet(t *testing.T) {
	credPath, _ := loggedInWorktree(t)

	if _, err := runCmd(t, credPath, "set", "FOO=bar", "--quiet"); err != nil {
		t.Fatalf("set: %v", err)
	}
	out, err := runCmd(t, credPath, "get", "FOO", "--quiet")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out != "bar" {
		t.Errorf("get output = %q; want bar", out)
	}
}

func TestSet_RejectsBadArg(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "no-equals-sign", "--quiet"); err == nil {
		t.Error("set with bad arg returned nil; want error")
	}
}

func TestDiff_AddedRemovedChanged(t *testing.T) {
	credPath, _ := loggedInWorktree(t)

	// dev has FOO=1 and SHARED=v1.
	if _, err := runCmd(t, credPath, "set", "FOO=1", "--quiet"); err != nil {
		t.Fatalf("seed dev.FOO: %v", err)
	}
	if _, err := runCmd(t, credPath, "set", "SHARED=v1", "--quiet"); err != nil {
		t.Fatalf("seed dev.SHARED: %v", err)
	}
	// prod has BAR=2 and SHARED=v2.
	if _, err := runCmd(t, credPath, "set", "BAR=2", "--env", "prod", "--quiet"); err != nil {
		t.Fatalf("seed prod.BAR: %v", err)
	}
	if _, err := runCmd(t, credPath, "set", "SHARED=v2", "--env", "prod", "--quiet"); err != nil {
		t.Fatalf("seed prod.SHARED: %v", err)
	}

	// diff dev -> prod:
	//   + BAR (in prod, not in dev)
	//   - FOO (in dev, not in prod)
	//   ~ SHARED (different)
	out, err := runCmd(t, credPath, "diff", "--against", "prod", "--quiet")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	for _, want := range []string{"+ BAR", "- FOO", "~ SHARED"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff output missing %q; got:\n%s", want, out)
		}
	}
}

func TestDiff_RequiresAgainst(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "diff"); err == nil {
		t.Error("diff without --against returned nil; want error")
	}
}

func TestPull_NoSecretsProducesEmptyFile(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	outPath := filepath.Join(cwd, "empty.env")
	if _, err := runCmd(t, credPath, "pull", "--out", outPath, "--quiet"); err != nil {
		t.Fatalf("pull: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(raw) != 0 {
		t.Errorf("expected empty file; got %q", raw)
	}
}
