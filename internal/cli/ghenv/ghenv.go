// Package ghenv emits secrets in the GitHub Actions $GITHUB_ENV format.
//
// This is the OPT-IN, job-wide injection path (action input
// export-to: github-env). It lives alongside the default process-env path
// (secret run), which never exposes secrets to the job at large. When an
// operator explicitly opts into job-wide env, this package makes the
// exposure as safe as the mechanism allows:
//
//   - Every value is registered with GitHub's ::add-mask:: workflow
//     command so it is redacted if it later surfaces in a log line.
//     Multiline values are masked line-by-line because GitHub's masker
//     matches per-line, not across newlines.
//   - Values are written with the heredoc form (KEY<<DELIM ... DELIM) so
//     multiline secrets and values containing '=' round-trip intact. The
//     delimiter is randomised and collision-checked against the value, so
//     a hostile secret cannot terminate the heredoc early and inject
//     arbitrary environment variables.
//
// Masking is best-effort by GitHub's own admission: very short or
// low-entropy values may still surface. That limitation is documented in
// docs/github-actions.md; the process-env default avoids it entirely.
package ghenv

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
)

// maxDelimiterAttempts bounds the collision-retry loop. A 128-bit random
// delimiter colliding with a value line is astronomically unlikely, so a
// handful of attempts is a safety net, not a hot path.
const maxDelimiterAttempts = 8

// Emit writes entries to two sinks:
//
//   - maskW receives one "::add-mask::<line>" per non-empty line of every
//     value. Point this at os.Stdout in the workflow step so GitHub
//     registers the redactions before any later step could log the value.
//   - envW receives the "KEY<<DELIM\n<value>\nDELIM\n" heredoc blocks.
//     Point this at the file named by $GITHUB_ENV.
//
// Keys are emitted in sorted order so output is deterministic. A value
// line equal to a freshly generated delimiter triggers regeneration; if
// that cannot be resolved within maxDelimiterAttempts, Emit returns an
// error rather than emit a breakable heredoc.
func Emit(maskW, envW io.Writer, entries map[string]string) error {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := entries[k]

		// Defense-in-depth: a newline in a key would break the KEY<<DELIM
		// line and let a crafted key inject arbitrary env entries. Server-side
		// validateName already forbids this, so reaching here means an
		// unvalidated write path exists — fail loudly rather than emit a
		// corruptible block.
		if strings.ContainsAny(k, "\n\r") {
			return fmt.Errorf("ghenv: key %q contains a newline", k)
		}

		// Mask every non-empty line. GitHub's masker is per-line, so a
		// multiline value needs one directive per line to be fully redacted.
		for _, line := range strings.Split(v, "\n") {
			if line == "" {
				continue
			}
			if _, err := fmt.Fprintf(maskW, "::add-mask::%s\n", line); err != nil {
				return fmt.Errorf("ghenv: mask %s: %w", k, err)
			}
		}

		delim, err := delimiterFor(v)
		if err != nil {
			return fmt.Errorf("ghenv: %s: %w", k, err)
		}
		if _, err := fmt.Fprintf(envW, "%s<<%s\n%s\n%s\n", k, delim, v, delim); err != nil {
			return fmt.Errorf("ghenv: emit %s: %w", k, err)
		}
	}
	return nil
}

// delimiterFor returns a random heredoc delimiter guaranteed not to appear
// as a standalone line within value. A collision would prematurely
// terminate the heredoc, letting a hostile secret inject arbitrary env
// vars into the job.
func delimiterFor(value string) (string, error) {
	lines := strings.Split(value, "\n")
	for attempt := 0; attempt < maxDelimiterAttempts; attempt++ {
		d, err := randomDelimiter()
		if err != nil {
			return "", err
		}
		if !containsLine(lines, d) {
			return d, nil
		}
	}
	return "", fmt.Errorf("could not find a collision-free delimiter after %d attempts", maxDelimiterAttempts)
}

// randomDelimiter returns a 128-bit random, prefixed hex delimiter. The
// prefix keeps it recognisable in a $GITHUB_ENV dump and well outside the
// charset of any realistic secret value.
func randomDelimiter() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random delimiter: %w", err)
	}
	return "ghadelim_" + hex.EncodeToString(b), nil
}

// containsLine reports whether target exactly equals any element of lines.
func containsLine(lines []string, target string) bool {
	for _, l := range lines {
		if l == target {
			return true
		}
	}
	return false
}
