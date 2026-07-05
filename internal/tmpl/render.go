// Package tmpl renders infra-config templates by substituting
// ${{ env.KEY }} placeholders with resolved secret values.
//
// It is intentionally dependency-light (regexp + strings only) so the CLI
// can import it without pulling in the server-side resolver's
// database/sql + crypto + store dependencies, which would inflate the CLI
// binary and threaten its 300 ms cold-start budget (docs/perf.md).
//
// The placeholder grammar mirrors internal/secret.ReferencePattern
// exactly — ${{ env.KEY }} with the [A-Za-z0-9_.+-] charset. The ${{ }}
// delimiter is deliberately chosen because infra configs (nginx $host,
// redis) never use it, so a template's literal $ stays literal.
// TestGrammarParity guards against drift from the server pattern.
//
// The reserved first-segment word "self" always resolves to the env
// currently being rendered, so a single template renders across
// local/dev/prod without hardcoding an env name. Any other first segment
// is a cross-env reference. Secret values are NEVER included in error
// messages (CLAUDE.md: 시크릿은 절대 로그에 남기지 않는다).
package tmpl

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// SelfEnv is the reserved first-segment alias meaning "the env currently
// being rendered". ${{ self.KEY }} renders KEY from currentEnv regardless
// of context — deterministic, never a literal env named "self".
const SelfEnv = "self"

// pattern mirrors internal/secret.ReferencePattern. Kept as a local copy
// so this package stays dependency-light; TestGrammarParity asserts the
// two pattern strings are byte-identical.
var pattern = regexp.MustCompile(`\$\{\{\s*([A-Za-z0-9_.+-]+)\.([A-Za-z0-9_.+-]+)\s*\}\}`)

// Ref is one ${{ env.KEY }} site in a template.
type Ref struct {
	Env, Key   string
	start, end int
}

// String renders the reference as "env.KEY" — names only, never a value.
func (r Ref) String() string { return r.Env + "." + r.Key }

// References returns every ${{ env.KEY }} placeholder in tmpl, in source
// order. Callers use this to learn which envs a template needs before
// fetching their snapshots. The returned slice may be nil (no refs).
func References(tmpl string) []Ref {
	idxs := pattern.FindAllStringSubmatchIndex(tmpl, -1)
	if len(idxs) == 0 {
		return nil
	}
	out := make([]Ref, 0, len(idxs))
	for _, m := range idxs {
		// m layout: [matchStart, matchEnd, env1Start, env1End, key1Start, key1End]
		out = append(out, Ref{
			Env:   tmpl[m[2]:m[3]],
			Key:   tmpl[m[4]:m[5]],
			start: m[0],
			end:   m[1],
		})
	}
	return out
}

// Envs returns the distinct env names referenced by tmpl, with the
// reserved "self" resolved to currentEnv. The result is the set of envs a
// caller must fetch to render the template. Sorted for determinism.
func Envs(tmpl, currentEnv string) []string {
	seen := map[string]struct{}{}
	for _, r := range References(tmpl) {
		env := r.Env
		if env == SelfEnv {
			env = currentEnv
		}
		seen[env] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// Render substitutes every ${{ env.KEY }} in tmpl with the matching value
// from snapshots. "self" resolves to currentEnv. snapshots maps env name
// -> (key -> value); the caller pre-fetches each env returned by Envs.
//
// Any placeholder whose env or key is absent from snapshots is collected
// into missing and the render fails closed (err non-nil, rendered ""). A
// currentEnv literally named "self" is rejected — it would make the alias
// ambiguous. Error messages carry only env/key names and positions, never
// the resolved secret values.
func Render(tmpl, currentEnv string, snapshots map[string]map[string]string) (rendered string, missing []Ref, err error) {
	if currentEnv == SelfEnv {
		return "", nil, fmt.Errorf("tmpl: current env is %q, which is reserved as the current-env alias; rename the env", SelfEnv)
	}
	refs := References(tmpl)
	if len(refs) == 0 {
		return tmpl, nil, nil
	}
	// First pass: collect every gap so the error reports all at once.
	for _, r := range refs {
		env := r.Env
		if env == SelfEnv {
			env = currentEnv
		}
		snap, ok := snapshots[env]
		if !ok {
			missing = append(missing, r)
			continue
		}
		if _, ok := snap[r.Key]; !ok {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return "", missing, fmt.Errorf("tmpl: %d unresolved reference(s): %s", len(missing), joinRefs(missing))
	}
	// Second pass: splice in reverse so byte offsets stay valid as we go.
	out := tmpl
	for i := len(refs) - 1; i >= 0; i-- {
		r := refs[i]
		env := r.Env
		if env == SelfEnv {
			env = currentEnv
		}
		out = out[:r.start] + snapshots[env][r.Key] + out[r.end:]
	}
	return out, nil, nil
}

// joinRefs formats refs as "${{ env.KEY }}" names for error messages —
// never the resolved values.
func joinRefs(refs []Ref) string {
	parts := make([]string, len(refs))
	for i, r := range refs {
		parts[i] = "${{ " + r.String() + " }}"
	}
	return strings.Join(parts, ", ")
}
