package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/pkg/client"
)

// newRunCmd binds `secret run -- <cmd> [args...]`.
//
// The headline command. Resolves the context, pulls secrets in memory
// (never touches disk), spawns the child with those secrets merged into
// its environment, and forwards stdin/stdout/stderr. Exits with the
// child's exit code so the run-secret invocation is transparent to
// upstream tooling (CI, supervisors, watch scripts).
//
// Security invariant: no decrypted secret is ever written to a file
// during run. The integration test asserts this with a directory audit.
//
// Layout note: cobra requires "--" between secret-run flags and the
// child command so flag parsing stops there.
func newRunCmd(st *rootState) *cobra.Command {
	var (
		envFlag     string
		projectFlag string
		quiet       bool
	)
	cmd := &cobra.Command{
		Use:                   "run -- COMMAND [ARGS...]",
		Short:                 "Run a child command with secrets injected as env vars",
		DisableFlagsInUseLine: true,
		Args:                  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, project, env, err := loadContext(st, envFlag, cmd, quiet)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			c, err := client.New(creds.Server, creds.Token, 10*time.Second)
			if err != nil {
				return err
			}
			secrets, err := c.ListSecrets(ctx, project, env)
			if err != nil {
				return fmt.Errorf("pull secrets: %w", err)
			}

			merged := mergeEnv(os.Environ(), secrets)

			child := exec.Command(args[0], args[1:]...) //nolint:gosec // operator-supplied command is the whole point
			child.Env = merged
			child.Stdin = os.Stdin
			child.Stdout = cmd.OutOrStdout()
			child.Stderr = cmd.ErrOrStderr()

			if err := child.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					// Propagate the child's exit code. SilenceErrors
					// stops cobra from printing the wrapped error
					// twice; we use ExitError directly so the caller
					// sees the actual code, not 1.
					cmd.SilenceErrors = true
					os.Exit(exitErr.ExitCode())
				}
				return fmt.Errorf("spawn %s: %w", args[0], err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "target env (overrides resolver)")
	cmd.Flags().StringVar(&projectFlag, "project", "", "project name (overrides .secretrc; required in CI where no .secretrc exists)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress resolver banner")
	return cmd
}

// mergeEnv overlays secrets on top of the current process env. Existing
// vars from os.Environ() that share a key with a pulled secret are
// shadowed by the secret — same precedence as docker-compose env_file.
//
// Returned slice is suitable for exec.Cmd.Env: each entry is "KEY=VAL".
// We don't filter the parent env (PATH, HOME, etc. flow through) — the
// child will need those to do anything useful.
func mergeEnv(parent []string, secrets []client.Secret) []string {
	overrides := make(map[string]string, len(secrets))
	for _, s := range secrets {
		overrides[s.Key] = s.Value
	}

	out := make([]string, 0, len(parent)+len(secrets))
	seen := make(map[string]bool, len(secrets))
	for _, kv := range parent {
		if eq := indexByteOf(kv, '='); eq > 0 {
			k := kv[:eq]
			if v, ok := overrides[k]; ok {
				out = append(out, k+"="+v)
				seen[k] = true
				continue
			}
		}
		out = append(out, kv)
	}
	// Append secrets that didn't shadow an existing parent var.
	for k, v := range overrides {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func indexByteOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
