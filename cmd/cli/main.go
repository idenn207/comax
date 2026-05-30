// Command secret is the Comax Secrets CLI binary.
//
// Milestone 1 entrypoint: only prints version. Subcommands (login, init,
// pull, push, run, set, get, diff) are wired in Tasks 7-10.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/idenn207/comax-secrets/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "secret:", err)
		os.Exit(1)
	}
}

func run(_ []string, out io.Writer) error {
	_, err := fmt.Fprintln(out, "secret", version.String())
	return err
}
