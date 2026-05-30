// Package envctx implements the CLI's environment-name resolver.
//
// Operators run the same `secret pull` / `secret run` in different
// worktrees and on different branches; the resolver picks which env
// the command targets by walking a fixed precedence list:
//
//  1. --env flag (highest precedence)
//  2. COMAX_ENV environment variable
//  3. .secretrc.env (explicit pin in the worktree config)
//  4. .secretrc.branches[<current branch>] (mapping)
//  5. DefaultBranchMap[<current branch>] (built-in defaults)
//  6. .secretrc.default_env
//  7. error: "could not resolve env; pass --env"
//
// Each step is a pure function of inputs we can stub in tests, so the
// resolver has zero filesystem I/O of its own — the caller passes the
// loaded Config, the cwd, the env var lookup, and a git-branch
// resolver. Production wiring is in Resolve.
package envctx

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
)

// DefaultBranchMap is the built-in branch → env table used when
// .secretrc.branches doesn't have a match. main/master → prod is the
// only opinion the resolver enforces; everything else falls through.
var DefaultBranchMap = map[string]string{
	"main":    "prod",
	"master":  "prod",
	"prod":    "prod",
	"staging": "staging",
	"dev":     "dev",
}

// Source describes which rule fired. Callers print this so operators
// understand which signal produced the resolved env — "→ resolved
// env=local via .secretrc.default_env" is much easier to debug than
// the bare env name.
type Source string

const (
	SourceFlag        Source = "--env flag"
	SourceEnvVar      Source = "COMAX_ENV"
	SourceSecretrcEnv Source = ".secretrc.env"
	SourceBranchMap   Source = ".secretrc.branches"
	SourceBuiltinMap  Source = "built-in branch map"
	SourceDefault     Source = ".secretrc.default_env"
)

// Result is what Resolve returns. Env is the resolved env name;
// Source identifies which rule fired.
type Result struct {
	Env    string
	Source Source
}

// ErrUnresolved is returned when no rule produced an env. The CLI
// surfaces this with a suggestion to pass --env.
var ErrUnresolved = errors.New("envctx: could not resolve env")

// Inputs captures everything Resolve needs. Tests build this directly;
// production callers use Load() to populate it from real signals.
type Inputs struct {
	// Flag is the value of --env. Empty when not set.
	Flag string
	// EnvVar is the value of COMAX_ENV. Empty when not set.
	EnvVar string
	// Cfg is the loaded .secretrc (zero value when absent — that's OK).
	Cfg secretrc.Config
	// Branch is the current git branch, or "" when not in a git repo
	// or when the branch can't be determined (detached HEAD, etc.).
	Branch string
}

// Resolve walks the precedence list and returns the first match.
func Resolve(in Inputs) (Result, error) {
	if in.Flag != "" {
		return Result{Env: in.Flag, Source: SourceFlag}, nil
	}
	if in.EnvVar != "" {
		return Result{Env: in.EnvVar, Source: SourceEnvVar}, nil
	}
	if in.Cfg.Env != "" {
		return Result{Env: in.Cfg.Env, Source: SourceSecretrcEnv}, nil
	}
	if in.Branch != "" {
		if v, ok := in.Cfg.Branches[in.Branch]; ok && v != "" {
			return Result{Env: v, Source: SourceBranchMap}, nil
		}
		if v, ok := DefaultBranchMap[in.Branch]; ok {
			return Result{Env: v, Source: SourceBuiltinMap}, nil
		}
	}
	if in.Cfg.DefaultEnv != "" {
		return Result{Env: in.Cfg.DefaultEnv, Source: SourceDefault}, nil
	}
	return Result{}, fmt.Errorf("%w (no flag, env var, .secretrc, or matching branch)", ErrUnresolved)
}

// Load is the production wiring of Resolve. It reads .secretrc from
// dir (typically cwd), looks up COMAX_ENV, and runs `git branch
// --show-current` in dir to find the active branch. Any of these
// signals being absent is fine — the resolver handles missing inputs.
func Load(dir, flag string) (Result, error) {
	cfg, err := secretrc.Load(dir)
	if err != nil && !errors.Is(err, secretrc.ErrNotFound) {
		return Result{}, fmt.Errorf("load .secretrc: %w", err)
	}
	return Resolve(Inputs{
		Flag:   flag,
		EnvVar: os.Getenv("COMAX_ENV"),
		Cfg:    cfg,
		Branch: currentGitBranch(dir),
	})
}

// currentGitBranch runs `git branch --show-current` in dir. Returns
// "" on any error (not in a git repo, detached HEAD, git not on
// $PATH) so the caller falls through to the next rule.
//
// We deliberately don't import a git library here: the CLI cold-start
// budget is tight (Task 11) and shelling out for one branch name on
// commands that need it is cheaper than pulling a Go git package.
func currentGitBranch(dir string) string {
	cmd := exec.Command("git", "-C", dir, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
