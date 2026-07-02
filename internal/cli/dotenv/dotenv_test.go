package dotenv

import (
	"bytes"
	"strings"
	"testing"
)

func TestParse_Basics(t *testing.T) {
	in := `
# comment
KEY=value
ALSO_KEY=another value
QUOTED="with spaces and # hash"
SINGLE='literal \n stays as text'
EXPORTED=plain
export EXPORT_PREFIX=yes
EMPTY=
WITH_INLINE=val # trailing comment
`
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := map[string]string{
		"KEY":           "value",
		"ALSO_KEY":      "another value",
		"QUOTED":        "with spaces and # hash",
		"SINGLE":        `literal \n stays as text`,
		"EXPORTED":      "plain",
		"EXPORT_PREFIX": "yes",
		"EMPTY":         "",
		"WITH_INLINE":   "val",
	}
	if len(got) != len(want) {
		t.Fatalf("entries=%d want %d (got=%+v)", len(got), len(want), got)
	}
	for _, e := range got {
		if want[e.Key] != e.Value {
			t.Errorf("%s = %q; want %q", e.Key, e.Value, want[e.Key])
		}
	}
}

func TestParse_EscapesInDoubleQuotes(t *testing.T) {
	got, err := Parse(strings.NewReader(`MULTILINE="line1\nline2\twith\ttab"`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got[0].Value != "line1\nline2\twith\ttab" {
		t.Errorf("got %q", got[0].Value)
	}
}

func TestParse_RejectsMissingEquals(t *testing.T) {
	_, err := Parse(strings.NewReader("BARE_LINE"))
	if err == nil {
		t.Fatal("Parse returned nil on bare line; want error")
	}
}

func TestParse_RejectsInvalidKey(t *testing.T) {
	for _, line := range []string{
		"123BAD=v",     // starts with digit
		"=missing-key", // empty key
		"bad-key=v",    // hyphen not allowed
		"a b=v",        // space in key
	} {
		t.Run(line, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(line)); err == nil {
				t.Errorf("Parse(%q) returned nil; want error", line)
			}
		})
	}
}

func TestParse_RejectsUnterminatedQuotes(t *testing.T) {
	for _, line := range []string{
		`K="unterminated`,
		`K='unterminated`,
	} {
		t.Run(line, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(line)); err == nil {
				t.Errorf("Parse(%q) returned nil; want unterminated error", line)
			}
		})
	}
}

func TestEmit_Deterministic(t *testing.T) {
	in := map[string]string{
		"ZED":   "1",
		"ALPHA": "2",
		"BETA":  "3",
	}
	var buf bytes.Buffer
	if err := Emit(&buf, in); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	want := "ALPHA=2\nBETA=3\nZED=1\n"
	if buf.String() != want {
		t.Errorf("Emit produced:\n%q\nwant:\n%q", buf.String(), want)
	}
}

func TestEmit_QuotesValuesWithSpecials(t *testing.T) {
	cases := []struct {
		key, value, wantLine string
	}{
		{"PLAIN", "value", "PLAIN=value"},
		{"SPACES", "a b c", `SPACES="a b c"`},
		{"HASH", "v#hash", `HASH="v#hash"`},
		{"NEWLINE", "line1\nline2", `NEWLINE="line1\nline2"`},
		{"QUOTE", `with "quote"`, `QUOTE="with \"quote\""`},
		{"DOLLAR", `$VAR`, `DOLLAR="$VAR"`},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			var buf bytes.Buffer
			_ = Emit(&buf, map[string]string{tc.key: tc.value})
			got := strings.TrimRight(buf.String(), "\n")
			if got != tc.wantLine {
				t.Errorf("Emit(%q) =\n%s\nwant\n%s", tc.value, got, tc.wantLine)
			}
		})
	}
}

func TestRoundTrip_PreservesValues(t *testing.T) {
	in := map[string]string{
		"PLAIN":   "value",
		"SPACES":  "a b c",
		"NEWLINE": "line1\nline2\tindented",
		"QUOTE":   `it's "quoted"`,
		"EMPTY":   "",
		"DOLLAR":  "$RAW",
	}
	var buf bytes.Buffer
	if err := Emit(&buf, in); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := make(map[string]string, len(parsed))
	for _, e := range parsed {
		got[e.Key] = e.Value
	}
	for k, v := range in {
		if got[k] != v {
			t.Errorf("round-trip %s: got %q; want %q", k, got[k], v)
		}
	}
}

func TestParse_StripsBOM(t *testing.T) {
	// "\xEF\xBB\xBF" is the UTF-8 BOM; Windows editors insert one and
	// the parser must strip it before reading the first key.
	in := "\xEF\xBB\xBFKEY=value"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 || got[0].Key != "KEY" || got[0].Value != "value" {
		t.Errorf("BOM not stripped: got %+v", got)
	}
}

func TestParse_DuplicateKeyLastWins(t *testing.T) {
	// Documented behaviour: the parser returns both entries in source
	// order; the *caller* (e.g. secret push) decides which one wins.
	// Our convention is "last wins" — same as dotenv-cli.
	got, err := Parse(strings.NewReader("K=first\nK=second\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d; want 2", len(got))
	}
	if got[1].Value != "second" {
		t.Errorf("second entry = %q; want second", got[1].Value)
	}
}
