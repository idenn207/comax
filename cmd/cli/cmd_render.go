package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/internal/tmpl"
	"github.com/idenn207/comax-secrets/pkg/client"
)

// newRenderCmd binds `secret render --template FILE --out FILE [--env NAME]`.
//
// render fills an infra-config template (redis.conf/nginx.conf, ...) with
// resolved secret values and writes the result to a STAGING file. It is
// the M7 infra-config-templating PoC (a decision milestone): it reuses the
// ${{ env.KEY }} grammar and the server resolver, adding only a
// client-side file-render layer.
//
//	${{ self.KEY }}   -> KEY in the env being rendered (one template, N envs)
//	${{ shared.KEY }} -> KEY in another env (cross-env reference)
//
// Rendered output contains PLAINTEXT secrets (redis requirepass, TLS
// material), so the safe-path policy is fail-closed: --out must be a
// gitignored path inside a git worktree, judged after {env} substitution.
// A symlinked target is refused (final component only, re-checked at write
// time); the parent path is not symlink-resolved, so this is best-effort
// within the self-host threat model where the operator owns the machine.
// Active/live config is never overwritten in place (the PoC is
// staging-only). See the plan's Security Reviewer triage.
func newRenderCmd(st *rootState) *cobra.Command {
	var (
		templatePath string
		out          string
		envFlag      string
		quiet        bool
	)
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render an infra-config template with an env's secrets (M7 PoC, staging-only)",
		Long: `Fill ${{ self.KEY }} / ${{ env.KEY }} placeholders in a config
template with resolved secret values and write to a staging file.

  ${{ self.KEY }}   -> KEY in the env being rendered (single template, many envs)
  ${{ shared.KEY }} -> KEY in another env (cross-env reference)

Rendered files contain plaintext secrets, so --out must be a gitignored
path inside a git worktree; --out '-' (stdout) is not supported. Active
config is never overwritten in place (PoC is staging-only).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if templatePath == "" {
				return fmt.Errorf("--template is required")
			}
			if out == "" {
				return fmt.Errorf("--out is required")
			}
			if out == "-" {
				return fmt.Errorf("--out '-' (stdout) is not supported: rendered output contains plaintext secrets; write to a gitignored staging file")
			}
			creds, project, env, err := loadContext(st, envFlag, cmd, quiet)
			if err != nil {
				return err
			}
			if env == tmpl.SelfEnv {
				return fmt.Errorf("env name %q is reserved as the current-env alias; rename the env", tmpl.SelfEnv)
			}

			// #nosec G304 -- operator-supplied template of their own config
			// on a self-host box; reading it is the command's purpose.
			tmplBytes, err := os.ReadFile(templatePath)
			if err != nil {
				return fmt.Errorf("read template %q: %w", templatePath, err)
			}
			tmplText := string(tmplBytes)

			outPath, err := resolveOutPath(out, env)
			if err != nil {
				return err
			}

			// Fail-closed safe-path enforcement runs BEFORE any secret fetch
			// so an invalid sink fails fast and we never pull plaintext into
			// memory for a path we are not allowed to write. It is re-asserted
			// IN FULL just before the write (below): the network fetch widens
			// the check->write window, and the gitignore/worktree decision
			// must be FRESH at write time, not stale from this pre-fetch pass.
			if err := assertSafeOutPath(outPath); err != nil {
				return err
			}

			// Fetch each referenced env's resolved snapshot once.
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			c, err := client.New(creds.Server, creds.Token, 10*time.Second)
			if err != nil {
				return err
			}
			snapshots := map[string]map[string]string{}
			for _, e := range tmpl.Envs(tmplText, env) {
				secrets, err := c.ListSecrets(ctx, project, e)
				if err != nil {
					return fmt.Errorf("render: fetch env %q: %w", e, err)
				}
				m := make(map[string]string, len(secrets))
				for _, s := range secrets {
					m[s.Key] = s.Value
				}
				snapshots[e] = m
			}

			rendered, _, err := tmpl.Render(tmplText, env, snapshots)
			if err != nil {
				// tmpl.Render errors name env.KEY refs only, never values.
				return fmt.Errorf("render %q: %w", templatePath, err)
			}

			// Re-assert the full safe-path policy immediately before writing.
			// The fetch above widened the check->write window, so the
			// gitignore/worktree decision (the primary guard against
			// committing plaintext secrets) is re-verified fresh here rather
			// than trusted stale from the pre-fetch pass. writeRenderedSecret
			// additionally re-Lstats for a symlink race just before the rename.
			if err := assertSafeOutPath(outPath); err != nil {
				return err
			}
			existed := fileExists(outPath)
			if err := writeRenderedSecret(outPath, rendered); err != nil {
				return err
			}
			if !quiet {
				verb := "Wrote"
				if existed {
					verb = "Overwrote"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s rendered %s -> %s (env=%s, 0600)\n",
					verb, filepath.Base(templatePath), outPath, env)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&templatePath, "template", "", "template file to render (required)")
	cmd.Flags().StringVar(&out, "out", "", "output staging file; the {env} token is replaced with the resolved env (required)")
	cmd.Flags().StringVar(&envFlag, "env", "", "target env (overrides resolver)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress status output (does NOT suppress security refusals)")
	return cmd
}

// resolveOutPath substitutes the {env} token and rejects any template
// marker syntax in --out. Allowing ${ or {{ in --out would let a secret
// reference or path-traversal payload escape the safe-path check; env
// names are charset-limited ([A-Za-z0-9_.+-]) so the {env} token itself
// cannot inject path separators.
func resolveOutPath(out, env string) (string, error) {
	if strings.Contains(out, "${") || strings.Contains(out, "{{") {
		return "", fmt.Errorf("--out %q must be a literal path; template markers (${ or {{) are not allowed", out)
	}
	if strings.Contains(out, "{env}") {
		// {env} expands into a path component, so the env value must not
		// carry a path separator or be a traversal element — otherwise a
		// crafted --env could relocate the rendered secret file out of the
		// staging dir before assertSafeOutPath even runs. Env names are
		// charset-limited server-side ([A-Za-z0-9_.+-]); re-validate here
		// as defense at the substitution boundary.
		if !validEnvPathComponent(env) {
			return "", fmt.Errorf("env %q is not a safe path component for {env} substitution (want [A-Za-z0-9_.+-], not '.'/'..')", env)
		}
	}
	return strings.ReplaceAll(out, "{env}", env), nil
}

// validEnvPathComponent reports whether env is safe to splice into a file
// path via the {env} token: the server-side env-name charset, with empty
// and all-dot names ("." / ".." / "..." ...) rejected — "." and ".." are
// traversal elements, and any longer all-dot name is not a real env name.
func validEnvPathComponent(env string) bool {
	if env == "" || strings.Trim(env, ".") == "" {
		return false
	}
	for _, r := range env {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '.', r == '+', r == '-':
		default:
			return false
		}
	}
	return true
}

// assertSafeOutPath enforces the fail-closed policy for a file that will
// hold plaintext secrets:
//   - the parent dir must exist and not be world-writable,
//   - the final path must not already be a symlink,
//   - the canonical path must be inside a git worktree AND gitignored.
//
// Any ambiguity (git absent, not a worktree, check-ignore error, tracked
// path) is a hard error — never a warning. The symlink/perms checks are
// best-effort defense-in-depth (the self-host threat model already assumes
// the operator owns the machine); the gitignore check is the primary
// guard against committing rendered secrets.
func assertSafeOutPath(outPath string) error {
	abs, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolve --out path: %w", err)
	}
	parent := filepath.Dir(abs)
	pinfo, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("--out parent dir %q must exist (create a gitignored staging dir first): %w", parent, err)
	}
	if !pinfo.IsDir() {
		return fmt.Errorf("--out parent %q is not a directory", parent)
	}
	// World-writable staging dirs are a symlink-plant vector. Unix only:
	// Go reports 0777 for dirs on Windows, so the bit is meaningless there.
	// Best-effort defense-in-depth (the self-host threat model already
	// assumes the operator owns the machine).
	if runtime.GOOS != "windows" && pinfo.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("--out parent dir %q is world-writable; refusing to write secrets there", parent)
	}
	// Refuse to write through an existing symlink (re-checked just before
	// the write in writeRenderedSecret to narrow the TOCTOU window).
	if li, err := os.Lstat(abs); err == nil && li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("--out %q is a symlink; refusing to write secrets through it", abs)
	}
	if err := runGitQuiet(parent, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("--out %q must be inside a git worktree (render writes only to gitignored staging paths): %w", abs, err)
	}
	if err := runGitQuiet(parent, "check-ignore", "-q", abs); err != nil {
		return fmt.Errorf("--out %q is not gitignored; refusing to write plaintext secrets to a tracked path (add it to .gitignore, e.g. tmp-render/)", abs)
	}
	return nil
}

// runGitQuiet runs git in dir and returns an error if git is absent or
// exits non-zero. Both callers treat any non-zero result as fail-closed:
// check-ignore exits 1 when the path is NOT ignored, rev-parse exits
// non-zero outside a worktree, and a missing git binary makes Run fail.
func runGitQuiet(dir string, args ...string) error {
	full := append([]string{"-C", dir}, args...)
	c := exec.Command("git", full...) // #nosec G204 -- fixed subcommand + operator path
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	return c.Run()
}

// writeRenderedSecret writes content to path atomically with 0600 perms.
// It creates a temp file in the same dir, chmods it 0600 BEFORE writing
// the secret (not relying on CreateTemp's platform default), re-checks for
// a symlink at the target, then renames. A crash mid-write leaves any
// previous file intact.
func writeRenderedSecret(path, content string) error {
	// Narrow the check→write TOCTOU window: re-verify the target is not a
	// symlink immediately before writing.
	if li, err := os.Lstat(path); err == nil && li.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("--out %q became a symlink; refusing to write secrets through it", path)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".render.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp 0600: %w", err)
	}
	if _, err := io.WriteString(tmp, content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write rendered content: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	// Windows: Rename fails when the target exists, so remove it first. On
	// Unix, Rename atomically replaces the target — skipping the Remove
	// there avoids a window where no file exists at path (a crash between
	// Remove and Rename would otherwise lose the previous render).
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %q -> %q: %w", tmpPath, path, err)
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
