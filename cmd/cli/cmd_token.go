package main

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/pkg/client"
)

// newTokenCmd binds `secret token {create,list,revoke}` — admin-only
// service-token management. The server enforces admin authorization; the
// CLI simply surfaces the endpoints. Token management needs no project/env
// context, so these commands use credentials directly rather than
// loadContext.
func newTokenCmd(st *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage service tokens (admin only)",
		Long: `Issue, list, and revoke service tokens.

Requires an admin token (the bootstrap token, or another admin). Issued
tokens are always non-admin: a leaked CI token cannot mint or revoke
further tokens. Revocation is a soft-revoke — the token stops
authenticating immediately, on both the bearer and dashboard-session arms.`,
	}
	cmd.AddCommand(newTokenCreateCmd(st), newTokenListCmd(st), newTokenRevokeCmd(st))
	return cmd
}

// newClientFromCreds builds a client from the saved credentials. Shared by
// the token subcommands, which need auth but no env resolution.
func newClientFromCreds(st *rootState) (*client.Client, error) {
	creds, err := loadCredentials(st)
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w (run `secret login` first)", err)
	}
	return client.New(creds.Server, creds.Token, 10*time.Second)
}

func newTokenCreateCmd(st *rootState) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create --name NAME",
		Short: "Issue a new non-admin service token (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			tok, err := cl.CreateToken(ctx, name)
			if err != nil {
				return fmt.Errorf("create token: %w", err)
			}
			// The plaintext is shown exactly once. Print ONLY the token to
			// stdout so `secret token create --name ci` can be captured
			// cleanly in a pipe; the human guidance goes to stderr.
			fmt.Fprintln(cmd.OutOrStdout(), tok.Token)
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Issued token %q (id %d). Store it now — it is shown once and cannot be recovered.\n"+
					"Add it to your GitHub repository as a secret (e.g. COMAX_TOKEN).\n",
				tok.Name, tok.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "token name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func newTokenListCmd(st *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List service tokens (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			tokens, err := cl.ListTokens(ctx)
			if err != nil {
				return fmt.Errorf("list tokens: %w", err)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tADMIN\tCREATED\tLAST USED\tSTATUS")
			for _, t := range tokens {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
					t.ID, t.Name, yesNo(t.IsAdmin),
					fmtDate(&t.CreatedAt), fmtDate(t.LastUsedAt), tokenStatus(t))
			}
			return w.Flush()
		},
	}
	return cmd
}

func newTokenRevokeCmd(st *rootState) *cobra.Command {
	var id int64
	cmd := &cobra.Command{
		Use:   "revoke --id ID",
		Short: "Revoke a service token by id (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			if err := cl.RevokeToken(ctx, id); err != nil {
				return fmt.Errorf("revoke token %d: %w", id, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Revoked token %d.\n", id)
			return nil
		},
	}
	cmd.Flags().Int64Var(&id, "id", 0, "token id to revoke (required)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// fmtDate renders a nullable timestamp as a short UTC date, or "-" when
// absent (nil pointer or zero time).
func fmtDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02")
}

func tokenStatus(t client.Token) string {
	if t.RevokedAt != nil {
		return "revoked"
	}
	return "live"
}
