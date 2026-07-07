package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gitInitStaging turns the (non-git) test worktree into a real git repo
// with a gitignored `tmp-render/` staging dir, so the render command's
// fail-closed safe-path check has something valid to accept.
func gitInitStaging(t *testing.T, cwd string) {
	t.Helper()
	run := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", cwd}, args...)...) // #nosec G204 -- test helper, fixed git subcommands + test tempdir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(cwd, ".gitignore"), []byte("tmp-render/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, "tmp-render"), 0o750); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
}

func TestResolveOutPath(t *testing.T) {
	got, err := resolveOutPath("tmp-render/redis.{env}.conf", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if got != "tmp-render/redis.prod.conf" {
		t.Errorf("resolveOutPath = %q; want tmp-render/redis.prod.conf", got)
	}
	// ${ (secret-ref start) and {{ (template) are rejected; a bare $ in a
	// literal path is fine (render does no shell expansion).
	for _, bad := range []string{"out/${{ x.Y }}.conf", "out/{{env}}.conf"} {
		if _, err := resolveOutPath(bad, "dev"); err == nil {
			t.Errorf("resolveOutPath(%q) = nil err; want marker rejection", bad)
		}
	}
	if _, err := resolveOutPath("weird$path/x.conf", "dev"); err != nil {
		t.Errorf("bare $ in a literal path should be allowed, got %v", err)
	}

	// {env} expands into a path component: a traversal/separator-bearing
	// env must be rejected at the substitution boundary.
	for _, badEnv := range []string{"../../etc", "a/b", "..", ".", "...", `a\b`} {
		if _, err := resolveOutPath("tmp-render/{env}.conf", badEnv); err == nil {
			t.Errorf("resolveOutPath with env=%q = nil err; want rejection", badEnv)
		}
	}
	// An unusual env is fine when {env} is not in the path (env value never
	// reaches the path).
	if _, err := resolveOutPath("tmp-render/fixed.conf", "../../etc"); err != nil {
		t.Errorf("env not spliced into path should not be validated, got %v", err)
	}
}

func TestRender_HappyPath(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)

	// Seed before git-init so `set` (no --env) resolves via .secretrc
	// default (dev), not a branch mapping introduced by gitInitStaging.
	if _, err := runCmd(t, credPath, "set", "REDIS_MAXMEM=256mb", "--quiet"); err != nil {
		t.Fatalf("seed MAXMEM: %v", err)
	}
	if _, err := runCmd(t, credPath, "set", "REDIS_PW=s3cret-canary", "--quiet"); err != nil {
		t.Fatalf("seed PW: %v", err)
	}
	gitInitStaging(t, cwd)

	tmplPath := filepath.Join(cwd, "redis.conf.tmpl")
	if err := os.WriteFile(tmplPath, []byte(
		"maxmemory ${{ self.REDIS_MAXMEM }}\nrequirepass ${{ self.REDIS_PW }}\n# literal: $host\n",
	), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	if _, err := runCmd(t, credPath, "render",
		"--template", tmplPath, "--out", "tmp-render/redis.{env}.conf", "--env", "dev", "--quiet"); err != nil {
		t.Fatalf("render: %v", err)
	}

	outPath := filepath.Join(cwd, "tmp-render", "redis.dev.conf")
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read rendered: %v", err)
	}
	got := string(raw)
	for _, want := range []string{"maxmemory 256mb", "requirepass s3cret-canary", "$host"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q; got:\n%s", want, got)
		}
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(outPath)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("perm = %o; want 0600", fi.Mode().Perm())
		}
	}
}

func TestRender_RejectsStdout(t *testing.T) {
	credPath, _ := loggedInWorktree(t)
	_, err := runCmd(t, credPath, "render", "--template", "x", "--out", "-")
	if err == nil || !strings.Contains(err.Error(), "stdout") {
		t.Fatalf("expected stdout-not-supported error, got %v", err)
	}
}

func TestRender_RejectsNonIgnoredPath(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	gitInitStaging(t, cwd)
	tmplPath := filepath.Join(cwd, "t.tmpl")
	if err := os.WriteFile(tmplPath, []byte("plain no refs\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	// out at repo root is tracked/non-ignored -> fail-closed.
	_, err := runCmd(t, credPath, "render",
		"--template", tmplPath, "--out", "tracked.conf", "--env", "dev", "--quiet")
	if err == nil || !strings.Contains(err.Error(), "not gitignored") {
		t.Fatalf("expected not-gitignored refusal, got %v", err)
	}
	if fileExists(filepath.Join(cwd, "tracked.conf")) {
		t.Error("render wrote to a non-ignored path despite refusal")
	}
}

// TestRender_RejectsDirectoryOutPath guards the Windows edge where an --out
// pointing at an existing (empty) dir would be deleted by writeRenderedSecret's
// os.Remove before the rename. The command must refuse AND leave the dir intact.
func TestRender_RejectsDirectoryOutPath(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	gitInitStaging(t, cwd)
	tmplPath := filepath.Join(cwd, "t.tmpl")
	if err := os.WriteFile(tmplPath, []byte("plain no refs\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	dirTarget := filepath.Join(cwd, "tmp-render", "adir")
	if err := os.MkdirAll(dirTarget, 0o750); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	_, err := runCmd(t, credPath, "render",
		"--template", tmplPath, "--out", "tmp-render/adir", "--env", "dev", "--quiet")
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory-refusal error, got %v", err)
	}
	if fi, statErr := os.Stat(dirTarget); statErr != nil || !fi.IsDir() {
		t.Error("render removed/replaced the target directory despite refusal")
	}
}

func TestRender_MissingKeyFailsClosed(t *testing.T) {
	credPath, cwd := loggedInWorktree(t)
	gitInitStaging(t, cwd)
	tmplPath := filepath.Join(cwd, "t.tmpl")
	if err := os.WriteFile(tmplPath, []byte("x ${{ self.NOPE }}\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	_, err := runCmd(t, credPath, "render",
		"--template", tmplPath, "--out", "tmp-render/out.conf", "--env", "dev", "--quiet")
	if err == nil {
		t.Fatal("expected fail-closed error for missing key")
	}
	if fileExists(filepath.Join(cwd, "tmp-render", "out.conf")) {
		t.Error("render wrote a file despite an unresolved reference")
	}
}
