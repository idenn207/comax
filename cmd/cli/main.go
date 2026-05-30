// Command secret is the Comax Secrets CLI binary.
//
// All cross-cutting state (server URL, token, etc.) lives on rootState
// and is populated by the root command's PersistentPreRunE so each
// subcommand starts with a fully-resolved environment. This keeps the
// per-command files thin (just flag binding + RunE) and the cobra root
// the one place that owns dependency wiring.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/internal/version"
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		// Cobra already prints the error message to stderr; we exit
		// non-zero so shell pipelines notice. Subcommands return
		// rich-enough errors via fmt.Errorf and ExitError types when
		// they need a specific exit code.
		os.Exit(1)
	}
}

// rootState carries the bits every subcommand might need. Populated in
// PersistentPreRunE; nil until then.
type rootState struct {
	// credPath overrides the default ~/.config/comax/credentials.json
	// when --credentials is set.
	credPath string
}

func newRootCmd() *cobra.Command {
	st := &rootState{}
	root := &cobra.Command{
		Use:           "secret",
		Short:         "Comax Secrets CLI — pull, push, run with managed envvars",
		Long:          "secret talks to a self-hosted Comax Secrets server. See https://github.com/idenn207/comax-secrets for setup.",
		Version:       version.String(),
		SilenceUsage:  true, // we print our own error messages
		SilenceErrors: false,
	}
	root.PersistentFlags().StringVar(&st.credPath, "credentials", "",
		"override the credentials file path (default: OS user config dir)")

	root.AddCommand(
		newLoginCmd(st),
		newInitCmd(st),
		newPullCmd(st),
		newPushCmd(st),
		newGetCmd(st),
		newSetCmd(st),
		newDiffCmd(st),
		newRunCmd(st),
	)

	// Silence cobra's "unknown command" usage dump — we want stderr to
	// be terse for piping. Help is still discoverable via -h.
	root.SetUsageFunc(func(c *cobra.Command) error {
		fmt.Fprintln(c.ErrOrStderr(), c.UsageString())
		return nil
	})
	return root
}
