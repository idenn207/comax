package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/idenn207/comax-secrets/pkg/client"
)

// newDiffCmd binds `secret diff [--env NAME] [--against NAME]`.
//
// Compares the resolved view of two envs. Output is three sections:
//
//   added:    keys in --against that are missing from current
//   removed:  keys in current that are missing from --against
//   changed:  keys present in both but with different values
//
// Exit code is 0 even when differences exist — this is a report, not
// a check. Operators who want a CI gate compose with grep / wc.
func newDiffCmd(st *rootState) *cobra.Command {
	var (
		envFlag string
		against string
		quiet   bool
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare the current env to another env",
		RunE: func(cmd *cobra.Command, args []string) error {
			if against == "" {
				return fmt.Errorf("--against is required (e.g. --against dev)")
			}
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

			a, err := c.ListSecrets(ctx, project, env)
			if err != nil {
				return fmt.Errorf("list %s: %w", env, err)
			}
			b, err := c.ListSecrets(ctx, project, against)
			if err != nil {
				return fmt.Errorf("list %s: %w", against, err)
			}
			printDiff(cmd.OutOrStdout(), env, against, a, b)
			return nil
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "current env (overrides resolver)")
	cmd.Flags().StringVar(&against, "against", "", "env to diff against (required)")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress resolver banner")
	return cmd
}

// printDiff formats the comparison. Keys are sorted in each section so
// the output is stable.
func printDiff(out fmtWriter, leftName, rightName string, left, right []client.Secret) {
	leftMap := indexByKey(left)
	rightMap := indexByKey(right)

	var added, removed, changed []string
	for k := range rightMap {
		if _, ok := leftMap[k]; !ok {
			added = append(added, k)
		}
	}
	for k, lv := range leftMap {
		rv, ok := rightMap[k]
		if !ok {
			removed = append(removed, k)
			continue
		}
		if lv != rv {
			changed = append(changed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)

	fmt.Fprintf(out, "diff %s -> %s:\n", leftName, rightName)
	if len(added)+len(removed)+len(changed) == 0 {
		fmt.Fprintln(out, "  (no differences)")
		return
	}
	for _, k := range added {
		fmt.Fprintf(out, "  + %s\n", k)
	}
	for _, k := range removed {
		fmt.Fprintf(out, "  - %s\n", k)
	}
	for _, k := range changed {
		fmt.Fprintf(out, "  ~ %s\n", k)
	}
}

// fmtWriter is the subset of io.Writer that fmt.Fprintf accepts. Used
// to avoid pulling in io into yet another file.
type fmtWriter interface {
	Write(p []byte) (int, error)
}

func indexByKey(s []client.Secret) map[string]string {
	out := make(map[string]string, len(s))
	for _, v := range s {
		out[v.Key] = v.Value
	}
	return out
}
