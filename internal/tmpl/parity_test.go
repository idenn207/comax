package tmpl

import (
	"testing"

	"github.com/idenn207/comax-secrets/internal/secret"
)

// TestGrammarParity asserts the local reference pattern is byte-identical
// to the server-side internal/secret.ReferencePattern, so the client-side
// renderer and the server resolver can never diverge on what counts as a
// ${{ env.KEY }} reference. This drift guard is what lets internal/tmpl
// keep its own dependency-light copy (see package doc) instead of
// importing internal/secret (and its sql/crypto/store deps) into the CLI.
//
// This test imports internal/secret, but only the test binary pays that
// cost — the shipped CLI links only internal/tmpl.
func TestGrammarParity(t *testing.T) {
	got := pattern.String()
	want := secret.ReferencePattern.String()
	if got != want {
		t.Fatalf("reference grammar drift:\n tmpl:   %s\n secret: %s", got, want)
	}
}
