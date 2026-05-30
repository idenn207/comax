// Package secret implements the env-snapshot resolver — the layer that
// applies inherits_from inheritance and expands ${{ env.KEY }} references
// at pull time.
//
// Two features live here and they interact:
//
//   - **Inheritance**: an env may declare inherits_from = "parent"; the
//     resolved view is parent's secrets overlaid with child's own keys
//     (child wins on conflict). Recursive: grandparents chain through.
//   - **References**: a secret value may contain ${{ <env>.<KEY> }};
//     pull-time the value is replaced with the *currently resolved*
//     plaintext of that key. The referenced env is itself resolved
//     (inheritance + nested refs), so chains work.
//
// Two cycle detectors guard the system:
//
//   - Inheritance cycle (envID chain): dev → shared → dev → error.
//   - Reference cycle ((envID, key) chain): A.K1 → ${{ B.K2 }} →
//     ${{ A.K1 }} → error.
//
// Both cycles produce a 400 with the cycle path so operators can fix
// the bad config without a server log dive.
package secret

import "regexp"

// referencePattern matches ${{ env.KEY }} with optional whitespace
// inside the braces. The env and key names use the same charset as
// server.validateName: [A-Za-z0-9_.+-]. The pattern is intentionally
// strict — anything outside that charset is left as a literal so
// operators can write {{ }} in places that aren't refs (e.g. inside a
// JSON template) without surprise.
var referencePattern = regexp.MustCompile(`\$\{\{\s*([A-Za-z0-9_.+-]+)\.([A-Za-z0-9_.+-]+)\s*\}\}`)

// reference is one parsed reference site inside a plaintext.
type reference struct {
	// envName, keyName are the parsed pieces between ${{ and }}.
	envName, keyName string
	// start, end mark the byte range of the full ${{ ... }} match in
	// the original plaintext, so the expander can splice the resolved
	// value back in place.
	start, end int
}

// findReferences returns every ${{ env.KEY }} match in plaintext, in
// the order they appear. The returned slice may be empty (no refs).
func findReferences(plaintext string) []reference {
	idxs := referencePattern.FindAllStringSubmatchIndex(plaintext, -1)
	if len(idxs) == 0 {
		return nil
	}
	out := make([]reference, 0, len(idxs))
	for _, m := range idxs {
		// m layout: [matchStart, matchEnd, env1Start, env1End, key1Start, key1End]
		out = append(out, reference{
			envName: plaintext[m[2]:m[3]],
			keyName: plaintext[m[4]:m[5]],
			start:   m[0],
			end:     m[1],
		})
	}
	return out
}
