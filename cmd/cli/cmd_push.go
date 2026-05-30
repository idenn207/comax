package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/internal/cli/dotenv"
	"github.com/idenn207/comax-secrets/pkg/client"
)

// newPushCmd binds `secret push --file PATH [--env NAME]`.
//
// Parses the .env file and PUTs every entry. Order is the source-file
// order; later duplicates in the same file overwrite earlier ones in
// flight (and the final upsert is what lands).
//
// Not transactional across keys — if the network drops halfway through,
// some keys land and others don't. The operator can re-run; PUT is
// idempotent on the same value (creates a new version though).
func newPushCmd(st *rootState) *cobra.Command {
	var (
		envFlag string
		file    string
		quiet   bool
	)
	cmd := &cobra.Command{
		Use:   "push --file .env",
		Short: "Push a .env file into the current env",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, project, env, err := loadContext(st, envFlag, cmd, quiet)
			if err != nil {
				return err
			}

			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("open %q: %w", file, err)
			}
			defer f.Close()
			entries, err := dotenv.Parse(f)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			c, err := client.New(creds.Server, creds.Token, 10*time.Second)
			if err != nil {
				return err
			}

			var pushed int
			for _, e := range entries {
				if _, err := c.PutSecret(ctx, project, env, e.Key, e.Value); err != nil {
					return fmt.Errorf("put %s: %w", e.Key, err)
				}
				pushed++
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Pushed %d secrets to env=%s\n", pushed, env)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "target env (overrides resolver)")
	cmd.Flags().StringVar(&file, "file", ".env", "path to the .env file to push")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress resolver/status output")
	return cmd
}
