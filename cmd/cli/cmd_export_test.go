package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runExport runs `secret export ...` capturing stdout and stderr
// separately, so mask directives (stdout) can be asserted apart from the
// resolver banner (stderr).
func runExport(t *testing.T, credPath string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"--credentials", credPath, "export"}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestExport_DotenvFormat(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "DB_URL=postgres://x", "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, _, err := runExport(t, credPath, "--format", "dotenv", "--quiet")
	if err != nil {
		t.Fatalf("export dotenv: %v", err)
	}
	if !strings.Contains(out, "DB_URL=") {
		t.Errorf("dotenv output missing DB_URL; got:\n%s", out)
	}
}

func TestExport_JSONFormat(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "API_KEY=sk_test", "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out, _, err := runExport(t, credPath, "--format", "json", "--quiet")
	if err != nil {
		t.Fatalf("export json: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("export json is not valid JSON: %v\n%s", err, out)
	}
	if m["API_KEY"] != "sk_test" {
		t.Errorf("json API_KEY = %q; want sk_test", m["API_KEY"])
	}
}

func TestExport_UnknownFormatErrors(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	_, _, err := runExport(t, credPath, "--format", "yaml", "--quiet")
	if err == nil {
		t.Fatal("unknown --format returned nil; want error")
	}
	if !strings.Contains(err.Error(), "unknown --format") {
		t.Errorf("error = %v; want unknown --format", err)
	}
}

func TestExport_GithubEnvRequiresTarget(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	t.Setenv("GITHUB_ENV", "") // force unset even if this suite runs inside Actions
	_, _, err := runExport(t, credPath, "--format", "github-env", "--quiet")
	if err == nil {
		t.Fatal("github-env without $GITHUB_ENV returned nil; want error")
	}
	if !strings.Contains(err.Error(), "GITHUB_ENV") {
		t.Errorf("error = %v; want it to mention GITHUB_ENV", err)
	}
}

func TestExport_GithubEnvWritesHeredocAndMasks(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	if _, err := runCmd(t, credPath, "set", "TOKEN=canary-123", "--quiet"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	envFile := filepath.Join(cwd, "gh.env")
	stdout, _, err := runExport(t, credPath, "--format", "github-env", "--github-env-file", envFile, "--quiet")
	if err != nil {
		t.Fatalf("export github-env: %v", err)
	}
	// stdout registers the mask so downstream logs redact the value.
	if !strings.Contains(stdout, "::add-mask::canary-123") {
		t.Errorf("stdout missing add-mask for TOKEN; got:\n%s", stdout)
	}
	raw, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "TOKEN<<") || !strings.Contains(got, "canary-123") {
		t.Errorf("github env file missing TOKEN heredoc; got:\n%s", got)
	}
}
