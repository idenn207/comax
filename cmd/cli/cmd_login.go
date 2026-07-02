package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/internal/cli/credentials"
	"github.com/idenn207/comax-secrets/pkg/client"
)

// newLoginCmd binds `secret login --server URL --token TOKEN`.
//
// Verification: we hit GET /healthz and GET /api/v1/projects against
// the supplied URL+token before saving. This catches the two common
// operator mistakes — typo'd URL and bad/expired token — at login
// time instead of on the next pull when the user no longer has the
// terminal open.
func newLoginCmd(st *rootState) *cobra.Command {
	var (
		server string
		token  string
	)
	cmd := &cobra.Command{
		Use:   "login --server URL --token TOKEN",
		Short: "Save credentials for a Comax Secrets server",
		Long: `Verify the server URL + token and save them to the credentials file.
The token is shown exactly once by the server at /bootstrap; pass it via
--token or the $COMAX_TOKEN environment variable.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fall back to $COMAX_TOKEN when --token is omitted so callers
			// (notably the GitHub Action) can keep the plaintext off the
			// command line and out of process listings. The flag wins when
			// both are set.
			if token == "" {
				token = os.Getenv("COMAX_TOKEN")
			}
			if token == "" {
				return fmt.Errorf("token is required: pass --token or set $COMAX_TOKEN")
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			c, err := client.New(server, token, 5*time.Second)
			if err != nil {
				return err
			}
			if err := c.Health(ctx); err != nil {
				return fmt.Errorf("server health check failed: %w", err)
			}
			if _, err := c.ListProjects(ctx); err != nil {
				return fmt.Errorf("token verification failed: %w", err)
			}

			creds := credentials.Credentials{Server: server, Token: token}
			path, err := saveCredentials(st, creds)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %s; credentials saved to %s\n", server, path)
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "server base URL (required)")
	cmd.Flags().StringVar(&token, "token", "", "bearer token (or set $COMAX_TOKEN)")
	_ = cmd.MarkFlagRequired("server")
	return cmd
}

// saveCredentials honours st.credPath when set; otherwise uses the
// platform default. Returns the resolved path so the caller can print
// it for the operator.
func saveCredentials(st *rootState, c credentials.Credentials) (string, error) {
	if st.credPath != "" {
		if err := credentials.SaveTo(st.credPath, c); err != nil {
			return "", err
		}
		return st.credPath, nil
	}
	if err := credentials.Save(c); err != nil {
		return "", err
	}
	p, _ := credentials.Path()
	return p, nil
}

// loadCredentials is the inverse used by every other subcommand.
func loadCredentials(st *rootState) (credentials.Credentials, error) {
	if st.credPath != "" {
		return credentials.LoadFrom(st.credPath)
	}
	return credentials.Load()
}
