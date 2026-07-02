package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/internal/cli/dotenv"
	"github.com/idenn207/comax-secrets/internal/cli/ghenv"
	"github.com/idenn207/comax-secrets/pkg/client"
)

// newExportCmd binds `secret export [--project NAME] [--env NAME]
// [--format dotenv|json|github-env]`.
//
// export is the CI-facing sibling of pull/run: it resolves an env's
// secrets and writes them in a chosen format. The github-env format is the
// OPT-IN, job-wide injection path — it appends heredoc blocks to the file
// named by $GITHUB_ENV and registers ::add-mask:: for every value. The
// default process-env path (secret run) is preferred; github-env exists
// only when a downstream step genuinely needs the vars in its own
// environment.
func newExportCmd(st *rootState) *cobra.Command {
	var (
		envFlag       string
		projectFlag   string
		format        string
		githubEnvFile string
		quiet         bool
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export an env's secrets as dotenv, json, or github-env",
		Long: `Resolve an env's secrets and emit them in the requested format.

Formats:
  dotenv       KEY="value" lines (default; same as pull --out -).
  json         a single JSON object of key -> value.
  github-env   append KEY<<DELIM heredoc blocks to $GITHUB_ENV and print
               ::add-mask:: for each value. OPT-IN job-wide injection —
               prefer 'secret run' (process-env) unless a later step needs
               the vars in its own environment.`,
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
				return fmt.Errorf("export secrets: %w", err)
			}
			entries := make(map[string]string, len(secrets))
			for _, s := range secrets {
				entries[s.Key] = s.Value
			}

			switch format {
			case "dotenv":
				return dotenv.Emit(cmd.OutOrStdout(), entries)
			case "json":
				return emitJSON(cmd.OutOrStdout(), entries)
			case "github-env":
				return emitGithubEnv(cmd, entries, githubEnvFile, quiet)
			default:
				return fmt.Errorf("unknown --format %q (want dotenv, json, or github-env)", format)
			}
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "target env (overrides resolver)")
	cmd.Flags().StringVar(&projectFlag, "project", "", "project name (overrides .secretrc; required in CI where no .secretrc exists)")
	cmd.Flags().StringVar(&format, "format", "dotenv", "output format: dotenv, json, or github-env")
	cmd.Flags().StringVar(&githubEnvFile, "github-env-file", "", "target file for github-env (default: $GITHUB_ENV)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress resolver banner")
	return cmd
}

// emitJSON writes entries as a single JSON object. encoding/json sorts map
// keys, so the output is deterministic without extra work.
func emitJSON(w io.Writer, entries map[string]string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// emitGithubEnv appends heredoc blocks to the $GITHUB_ENV file (or the
// --github-env-file override) and prints ::add-mask:: directives to
// stdout. A missing $GITHUB_ENV is a hard error — silently doing nothing
// would leave the downstream step without its secrets and no signal why.
func emitGithubEnv(cmd *cobra.Command, entries map[string]string, override string, quiet bool) error {
	path := override
	if path == "" {
		path = os.Getenv("GITHUB_ENV")
	}
	if path == "" {
		return fmt.Errorf("--format github-env requires $GITHUB_ENV to be set (or pass --github-env-file); are you running inside GitHub Actions?")
	}

	// #nosec G304 G703 -- path is $GITHUB_ENV or the operator's explicit
	// --github-env-file; writing to that file is the entire purpose of the
	// github-env format (mirrors cmd_run.go's operator-supplied command).
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // operator-controlled github-env target by design
	if err != nil {
		return fmt.Errorf("open github env file %q: %w", path, err)
	}
	// Masks go to stdout (the workflow log) so GitHub registers redactions;
	// the heredoc blocks are appended to the $GITHUB_ENV file.
	emitErr := ghenv.Emit(cmd.OutOrStdout(), f, entries)
	closeErr := f.Close()
	if emitErr != nil {
		return emitErr
	}
	if closeErr != nil {
		return fmt.Errorf("close github env file: %w", closeErr)
	}
	if !quiet {
		fmt.Fprintf(cmd.ErrOrStderr(), "→ exported %d secrets to $GITHUB_ENV\n", len(entries))
	}
	return nil
}
