package ghenv

import (
	"bytes"
	"strings"
	"testing"
)

// parseHeredocs reconstructs the (key -> value) map from $GITHUB_ENV
// heredoc output, exactly as the GitHub Actions runner would. Any value
// line equal to the block's delimiter would break this — which is the
// property delimiterFor guarantees against.
func parseHeredocs(t *testing.T, s string) map[string]string {
	t.Helper()
	out := map[string]string{}
	lines := strings.Split(s, "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		if line == "" {
			i++
			continue
		}
		idx := strings.Index(line, "<<")
		if idx < 0 {
			t.Fatalf("malformed heredoc header: %q", line)
		}
		key := line[:idx]
		delim := line[idx+2:]
		i++
		var body []string
		for i < len(lines) && lines[i] != delim {
			body = append(body, lines[i])
			i++
		}
		if i >= len(lines) {
			t.Fatalf("unterminated heredoc for key %q (delim %q)", key, delim)
		}
		i++ // skip the closing delimiter line
		out[key] = strings.Join(body, "\n")
	}
	return out
}

func maskLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "::add-mask::") {
			out = append(out, strings.TrimPrefix(l, "::add-mask::"))
		}
	}
	return out
}

func TestEmit_HeredocRoundTrips(t *testing.T) {
	entries := map[string]string{
		"SIMPLE":    "hello",
		"WITH_EQ":   "key=value=more",
		"MULTILINE": "line1\nline2\nline3",
		"EMPTY":     "",
	}
	var mask, env bytes.Buffer
	if err := Emit(&mask, &env, entries); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := parseHeredocs(t, env.String())
	for k, want := range entries {
		if got[k] != want {
			t.Errorf("round-trip %s = %q; want %q", k, got[k], want)
		}
	}
}

func TestEmit_SortedKeysDeterministic(t *testing.T) {
	entries := map[string]string{"ZETA": "1", "ALPHA": "2", "MIKE": "3"}
	var mask, env bytes.Buffer
	if err := Emit(&mask, &env, entries); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	// The heredoc headers must appear in sorted key order.
	var order []string
	for _, l := range strings.Split(env.String(), "\n") {
		if idx := strings.Index(l, "<<"); idx > 0 {
			order = append(order, l[:idx])
		}
	}
	want := []string{"ALPHA", "MIKE", "ZETA"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("key order = %v; want %v", order, want)
	}
}

func TestEmit_MasksEveryNonEmptyLine(t *testing.T) {
	entries := map[string]string{"K": "top\nmiddle\nbottom"}
	var mask, env bytes.Buffer
	if err := Emit(&mask, &env, entries); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := maskLines(mask.String())
	want := []string{"top", "middle", "bottom"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("mask lines = %v; want %v (multiline must be masked per line)", got, want)
	}
}

func TestEmit_EmptyValueEmitsNoMaskButKeepsHeredoc(t *testing.T) {
	entries := map[string]string{"BLANK": ""}
	var mask, env bytes.Buffer
	if err := Emit(&mask, &env, entries); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if lines := maskLines(mask.String()); len(lines) != 0 {
		t.Errorf("empty value produced mask directives %v; want none", lines)
	}
	if got := parseHeredocs(t, env.String()); got["BLANK"] != "" {
		t.Errorf("BLANK = %q; want empty", got["BLANK"])
	}
}

func TestEmit_DelimiterNeverAppearsInValue(t *testing.T) {
	// Every value line must differ from the block's delimiter, otherwise a
	// hostile secret could break out of the heredoc.
	entries := map[string]string{"X": "a\nb\nghadelim_fake\nc"}
	var mask, env bytes.Buffer
	if err := Emit(&mask, &env, entries); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := env.String()
	header := out[:strings.Index(out, "\n")]
	delim := header[strings.Index(header, "<<")+2:]
	for _, l := range strings.Split(out, "\n") {
		if l == delim {
			continue // the two legitimate delimiter lines
		}
		if l == "ghadelim_fake" && delim == "ghadelim_fake" {
			t.Fatal("delimiter collided with a value line")
		}
	}
	// And the value still round-trips despite containing a delimiter-shaped line.
	if got := parseHeredocs(t, out)["X"]; got != "a\nb\nghadelim_fake\nc" {
		t.Errorf("value round-trip = %q; want the original including ghadelim_fake line", got)
	}
}

func TestContainsLine(t *testing.T) {
	lines := []string{"a", "b", "c"}
	if !containsLine(lines, "b") {
		t.Error("containsLine miss on present element")
	}
	if containsLine(lines, "z") {
		t.Error("containsLine false positive")
	}
}

func TestDelimiterFor_AvoidsValueLines(t *testing.T) {
	d, err := delimiterFor("some\nvalue\nlines")
	if err != nil {
		t.Fatalf("delimiterFor: %v", err)
	}
	if !strings.HasPrefix(d, "ghadelim_") {
		t.Errorf("delimiter %q missing expected prefix", d)
	}
	if containsLine([]string{"some", "value", "lines"}, d) {
		t.Error("delimiter collided with a value line")
	}
	// Two calls yield distinct (random) delimiters.
	d2, _ := delimiterFor("other")
	if d == d2 {
		t.Error("delimiters are not random (two calls returned the same value)")
	}
}

func TestEmit_RejectsNewlineInKey(t *testing.T) {
	// Defense-in-depth: a key with a newline (or CR) would break the
	// KEY<<DELIM header and could inject arbitrary env entries. Server-side
	// validateName forbids this, but Emit must fail loudly rather than emit
	// a corruptible block if an unvalidated write path ever reaches it.
	for _, k := range []string{"GOOD\nINJECTED=evil", "CR\rKEY"} {
		var mask, env bytes.Buffer
		if err := Emit(&mask, &env, map[string]string{k: "v"}); err == nil {
			t.Errorf("Emit(key=%q) = nil; want error", k)
		}
	}
}
