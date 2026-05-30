package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/internal/cli/secretrc"
	"github.com/idenn207/comax-secrets/pkg/client"
)

// newInitCmd binds `secret init --project NAME [--envs CSV] [--default-env NAME]`.
//
// Idempotent: re-running on the same project skips the create call if
// the project already exists; same for envs. So an operator who runs
// init twice in different worktrees gets each worktree's .secretrc
// without server-side conflicts.
func newInitCmd(st *rootState) *cobra.Command {
	var (
		projectName string
		envsCSV     string
		defaultEnv  string
	)
	cmd := &cobra.Command{
		Use:   "init --project NAME",
		Short: "Bind the current directory to a Comax Secrets project",
		Long: `Create (or reuse) the project + envs on the server and write a
.secretrc file pinning this worktree to that project. The default
envs are local, dev, prod — override with --envs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			creds, err := loadCredentials(st)
			if err != nil {
				return fmt.Errorf("load credentials: %w (run `secret login` first)", err)
			}
			c, err := client.New(creds.Server, creds.Token, 5*time.Second)
			if err != nil {
				return err
			}

			envs := splitCSV(envsCSV)
			if defaultEnv == "" {
				defaultEnv = envs[0] // splitCSV guarantees at least one entry
			}
			if !containsString(envs, defaultEnv) {
				return fmt.Errorf("--default-env %q is not in --envs (%v)", defaultEnv, envs)
			}

			if err := ensureProject(ctx, c, projectName, cmd); err != nil {
				return err
			}
			for _, e := range envs {
				if err := ensureEnv(ctx, c, projectName, e, cmd); err != nil {
					return err
				}
			}

			cfg := secretrc.Config{
				Project:    projectName,
				DefaultEnv: defaultEnv,
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get cwd: %w", err)
			}
			if err := secretrc.Save(cwd, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s; project=%s default_env=%s\n",
				secretrc.FileName, projectName, defaultEnv)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectName, "project", "", "project name (required)")
	cmd.Flags().StringVar(&envsCSV, "envs", "local,dev,prod", "comma-separated env names to create")
	cmd.Flags().StringVar(&defaultEnv, "default-env", "", "env written into .secretrc (defaults to first --envs entry)")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}

// ensureProject creates the project if it doesn't exist; treats
// ErrConflict as "already there, that's fine".
func ensureProject(ctx context.Context, c *client.Client, name string, cmd *cobra.Command) error {
	_, err := c.CreateProject(ctx, name)
	if err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  created project %s\n", name)
		return nil
	}
	if errors.Is(err, client.ErrConflict) {
		fmt.Fprintf(cmd.OutOrStdout(), "  project %s already exists\n", name)
		return nil
	}
	return fmt.Errorf("create project %q: %w", name, err)
}

func ensureEnv(ctx context.Context, c *client.Client, project, env string, cmd *cobra.Command) error {
	_, err := c.CreateEnv(ctx, project, env, "")
	if err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  created env %s\n", env)
		return nil
	}
	if errors.Is(err, client.ErrConflict) {
		fmt.Fprintf(cmd.OutOrStdout(), "  env %s already exists\n", env)
		return nil
	}
	return fmt.Errorf("create env %q: %w", env, err)
}

// splitCSV splits a comma-separated list, trims spaces, and drops
// empty entries. Always returns at least one non-empty element when
// the input is non-empty; falls back to ["local"] for empty input so
// callers don't have to nil-check.
func splitCSV(s string) []string {
	if s == "" {
		return []string{"local"}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"local"}
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
