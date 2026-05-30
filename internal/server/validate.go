package server

import (
	"fmt"
	"strings"
)

// Naming rules applied to projects, envs, and secret keys.
//
// We are intentionally narrow: names appear in URL paths and audit log
// targets, so the safe set is alphanumerics plus a few unambiguous
// separators. The lower bound (≥ 1 char) catches the empty-string case
// some operators hit by mistake (e.g. a trailing slash in the URL).
const (
	maxNameLen   = 128
	allowedExtra = "_-.+"
)

// validateName rejects names that wouldn't round-trip through a URL or
// would confuse the audit log. The returned error wraps errBadRequest
// so handlers map it to 400.
func validateName(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required: %w", field, errBadRequest)
	}
	if len(value) > maxNameLen {
		return fmt.Errorf("%s exceeds %d chars: %w", field, maxNameLen, errBadRequest)
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9':
			// ok
		case strings.ContainsRune(allowedExtra, r):
			// ok
		default:
			return fmt.Errorf("%s contains forbidden character %q: %w", field, r, errBadRequest)
		}
	}
	return nil
}
