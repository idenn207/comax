package main

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/pkg/client"
)

// newWebhookCmd binds `secret webhook {create,list,delete,deliveries}` —
// admin-only webhook management. A webhook POSTs a signed, metadata-only event
// to an operator URL when a matching secret changes; the receiver re-pulls the
// value with its own credential. The server enforces admin authorization.
func newWebhookCmd(st *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage webhooks (admin only)",
		Long: `Register, list, and delete webhooks.

A webhook fires a signed HTTP POST when a subscribed secret change commits
(secret.upsert / secret.rollback / secret.delete). The payload carries only
metadata — project, env, key, version — never the secret value. The receiver
verifies the X-Comax-Signature (HMAC-SHA256 over "<timestamp>.<body>") using
the signing secret shown once at creation, then re-pulls the value itself.

Requires an admin token. The signing secret cannot be recovered later — to
rotate it, delete the webhook and create a new one.`,
	}
	cmd.AddCommand(
		newWebhookCreateCmd(st),
		newWebhookListCmd(st),
		newWebhookDeleteCmd(st),
		newWebhookDeliveriesCmd(st),
		newWebhookEnableCmd(st, true),
		newWebhookEnableCmd(st, false),
	)
	return cmd
}

// newWebhookEnableCmd binds `webhook enable` (enable=true) or `webhook disable`
// (enable=false). A soft-disabled webhook keeps its registration and delivery
// history but stops receiving new events until re-enabled — the reversible
// alternative to delete+recreate.
func newWebhookEnableCmd(st *rootState, enable bool) *cobra.Command {
	var id int64
	use, verb, past := "disable --id ID", "Disable", "Disabled"
	if enable {
		use, verb, past = "enable --id ID", "Enable", "Enabled"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: verb + " a webhook by id (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			if err := cl.SetWebhookEnabled(ctx, id, enable); err != nil {
				return fmt.Errorf("%s webhook %d: %w", strings.ToLower(verb), id, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "%s webhook %d.\n", past, id)
			return nil
		},
	}
	cmd.Flags().Int64Var(&id, "id", 0, "webhook id (required)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newWebhookCreateCmd(st *rootState) *cobra.Command {
	var (
		project string
		env     string
		url     string
		events  []string
	)
	cmd := &cobra.Command{
		Use:   "create --project NAME --url URL [--env NAME] [--events e1,e2]",
		Short: "Register a webhook (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			wh, err := cl.CreateWebhook(ctx, client.CreateWebhookInput{
				Project: project,
				Env:     env,
				URL:     url,
				Events:  events,
			})
			if err != nil {
				return fmt.Errorf("create webhook: %w", err)
			}
			// The signing secret is shown exactly once. Print ONLY it to stdout
			// so it can be captured in a pipe; guidance goes to stderr.
			fmt.Fprintln(cmd.OutOrStdout(), wh.SigningSecret)
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Registered webhook id %d → %s (events: %s).\n"+
					"Store this signing secret now — it is shown once and cannot be recovered.\n"+
					"Configure your receiver to verify X-Comax-Signature with it.\n",
				wh.ID, wh.URL, strings.Join(wh.Events, ","))
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project name (required)")
	cmd.Flags().StringVar(&env, "env", "", "environment name (optional; default: all environments)")
	cmd.Flags().StringVar(&url, "url", "", "receiver URL, http/https (required)")
	cmd.Flags().StringSliceVar(&events, "events", nil,
		"event kinds to subscribe to (default: all): secret.upsert,secret.rollback,secret.delete")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func newWebhookListCmd(st *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List webhooks (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			hooks, err := cl.ListWebhooks(ctx)
			if err != nil {
				return fmt.Errorf("list webhooks: %w", err)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSCOPE\tURL\tEVENTS\tENABLED\tCREATED")
			for _, h := range hooks {
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
					h.ID, webhookScope(h.Project, h.Env), h.URL,
					strings.Join(h.Events, ","), yesNo(h.Enabled), fmtDate(&h.CreatedAt))
			}
			return w.Flush()
		},
	}
	return cmd
}

func newWebhookDeleteCmd(st *rootState) *cobra.Command {
	var id int64
	cmd := &cobra.Command{
		Use:   "delete --id ID",
		Short: "Delete a webhook by id (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			if err := cl.DeleteWebhook(ctx, id); err != nil {
				return fmt.Errorf("delete webhook %d: %w", id, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted webhook %d.\n", id)
			return nil
		},
	}
	cmd.Flags().Int64Var(&id, "id", 0, "webhook id to delete (required)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newWebhookDeliveriesCmd(st *rootState) *cobra.Command {
	var id int64
	cmd := &cobra.Command{
		Use:   "deliveries --id ID",
		Short: "Show recent delivery attempts for a webhook (admin only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, err := newClientFromCreds(st)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			deliveries, err := cl.ListDeliveries(ctx, id)
			if err != nil {
				return fmt.Errorf("list deliveries for webhook %d: %w", id, err)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tEVENT\tSTATUS\tATTEMPTS\tHTTP\tCREATED")
			for _, d := range deliveries {
				fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\n",
					d.ID, d.Event, d.Status, d.Attempts, httpStatus(d.LastStatus), fmtDate(&d.CreatedAt))
			}
			return w.Flush()
		},
	}
	cmd.Flags().Int64Var(&id, "id", 0, "webhook id (required)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

// webhookScope renders a webhook's target as "project" or "project/env".
func webhookScope(project string, env *string) string {
	if env == nil {
		return project + "/*"
	}
	return project + "/" + *env
}

// httpStatus renders a nullable last HTTP status, or "-" when there was none
// (never attempted, or a transport error with no response).
func httpStatus(code *int64) string {
	if code == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *code)
}
