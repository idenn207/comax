package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/pkg/client"
)

// newGetCmd binds `secret get KEY [--env NAME]`. Prints the resolved
// value to stdout with no trailing newline so it composes with shell
// substitution: `KEY=$(secret get DB_URL)`.
func newGetCmd(st *rootState) *cobra.Command {
	var (
		envFlag string
		quiet   bool
	)
	cmd := &cobra.Command{
		Use:   "get KEY",
		Short: "Print one resolved secret value",
		Args:  cobra.ExactArgs(1),
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
			s, err := c.GetSecret(ctx, project, env, args[0])
			if err != nil {
				return err
			}
			// No newline: composable with $(secret get ...).
			fmt.Fprint(cmd.OutOrStdout(), s.Value)
			return nil
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "target env (overrides resolver)")
	cmd.Flags().BoolVar(&quiet, "quiet", true, "suppress resolver banner (default for get)")
	return cmd
}

// newSetCmd binds `secret set KEY=VALUE [--env NAME]`.
//
// We deliberately accept the K=V form (one argument) so the command
// can be embedded inside shell pipelines like
// `secret set "PASSWORD=$(generate-pw)"` without quoting ambiguity.
func newSetCmd(st *rootState) *cobra.Command {
	var (
		envFlag string
		quiet   bool
	)
	cmd := &cobra.Command{
		Use:   "set KEY=VALUE",
		Short: "Upsert one secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eq := strings.IndexByte(args[0], '=')
			if eq <= 0 {
				return fmt.Errorf("argument must be KEY=VALUE; got %q", args[0])
			}
			key, value := args[0][:eq], args[0][eq+1:]

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
			s, err := c.PutSecret(ctx, project, env, key, value)
			if err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Set %s in env=%s (version=%d)\n", key, env, s.Version)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "target env (overrides resolver)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress resolver/status output")
	return cmd
}
