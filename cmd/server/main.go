// Command secret-server is the Comax Secrets HTTP server binary.
//
// Milestone 1 entrypoint: only prints version. REST handlers, store, and
// crypto layers are wired in later tasks.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/idenn207/comax-secrets/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "secret-server:", err)
		os.Exit(1)
	}
}

func run(_ []string, out io.Writer) error {
	_, err := fmt.Fprintln(out, "secret-server", version.String())
	return err
}
