package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/internal/cli/dotenv"
	"github.com/idenn207/comax-secrets/internal/cli/envctx"
	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
	"github.com/idenn207/comax-secrets/pkg/client"
)

// newPullCmd binds `secret pull [--env NAME] [--out FILE]`.
//
// Reads the env's resolved secrets from the server and writes them as
// a .env file. Default output is ".env" in the cwd; --out switches it.
// We always write to a tempfile then rename so a half-written .env
// never replaces a good one.
func newPullCmd(st *rootState) *cobra.Command {
	var (
		envFlag string
		out     string
		quiet   bool
	)
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull all secrets for the current env into a .env file",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, project, env, err := loadContext(st, envFlag, cmd, quiet)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			c, err := client.New(creds.Server, creds.Token, 5*time.Second)
			if err != nil {
				return err
			}
			secrets, err := c.ListSecrets(ctx, project, env)
			if err != nil {
				return fmt.Errorf("pull secrets: %w", err)
			}

			entries := make(map[string]string, len(secrets))
			for _, s := range secrets {
				entries[s.Key] = s.Value
			}

			if out == "-" {
				return dotenv.Emit(cmd.OutOrStdout(), entries)
			}
			if out == "" {
				out = ".env"
			}
			if err := writeAtomic(out, func(w io.Writer) error {
				return dotenv.Emit(w, entries)
			}); err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d secrets to %s\n", len(entries), out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "target env (overrides resolver)")
	cmd.Flags().StringVar(&out, "out", "", "output file path (default .env; '-' for stdout)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress resolver/status output")
	return cmd
}

// writeAtomic creates a tempfile in the same dir as path, hands it to
// fn for content writing, then renames. Crash mid-write leaves the
// previous .env intact.
func writeAtomic(path string, fn func(io.Writer) error) error {
	dir := "."
	if d := dirOf(path); d != "" {
		dir = d
	}
	tmp, err := os.CreateTemp(dir, ".env.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if err := fn(tmp); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	// Windows: target must not exist for Rename to succeed.
	_ = os.Remove(path)
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %q -> %q: %w", tmpPath, path, err)
	}
	return nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}

// loadContext is shared by every command that needs (credentials,
// project, env). It centralises the env-resolver call and prints the
// resolved source to stderr unless quiet is set — that visibility is
// the Task 8 "worktree resolution surprises operator" mitigation.
func loadContext(st *rootState, envFlag string, cmd *cobra.Command, quiet bool) (creds credsLike, project, env string, err error) {
	c, err := loadCredentials(st)
	if err != nil {
		return credsLike{}, "", "", fmt.Errorf("load credentials: %w (run `secret login` first)", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return credsLike{}, "", "", fmt.Errorf("get cwd: %w", err)
	}
	res, err := envctx.Load(cwd, envFlag)
	if err != nil {
		return credsLike{}, "", "", fmt.Errorf("resolve env: %w", err)
	}
	// Resolving project requires .secretrc (no flag override in M1 —
	// operators who want a non-pinned project edit .secretrc).
	rcCfg, rcErr := secretrc.Load(cwd)
	if rcErr != nil && !errors.Is(rcErr, secretrc.ErrNotFound) {
		return credsLike{}, "", "", fmt.Errorf("load .secretrc: %w", rcErr)
	}
	if rcCfg.Project == "" {
		return credsLike{}, "", "", fmt.Errorf("no project pinned; run `secret init --project NAME`")
	}
	if !quiet {
		fmt.Fprintf(cmd.ErrOrStderr(), "→ resolved env=%s via %s\n", res.Env, res.Source)
	}
	return credsLike{Server: c.Server, Token: c.Token}, rcCfg.Project, res.Env, nil
}

// credsLike duplicates the fields we need from credentials.Credentials
// so this file doesn't have to import the package transitively where
// it isn't needed elsewhere.
type credsLike struct {
	Server string
	Token  string
}
