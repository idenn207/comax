// Package dotenv parses and emits .env files.
//
// We deliberately implement a small, predictable subset rather than
// pulling in a full library:
//
//   - Lines: KEY=VALUE; export KEY=VALUE; # comment; blank.
//   - Quoting: "double-quoted" preserves spaces and supports the escape
//     sequences \n, \t, \r, \\, \"; 'single-quoted' is literal.
//   - Unquoted values are taken verbatim through the first '#' (which
//     starts an inline comment) or end of line, trimmed of trailing
//     whitespace.
//
// Anything more clever (interpolation, multiline) is intentionally out
// of scope — those are reference-resolution concerns and live on the
// server.
package dotenv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Entry is one parsed (key, value) pair plus its 1-based line number
// for error messages.
type Entry struct {
	Line  int
	Key   string
	Value string
}

// Parse reads .env content from r and returns the entries in source
// order. Duplicate keys are allowed; later entries win (matching
// dotenv-cli behaviour).
func Parse(r io.Reader) ([]Entry, error) {
	scanner := bufio.NewScanner(r)
	// Some .env files contain very long machine-generated values
	// (encoded JWKs, base64 blobs). Default 64KB is enough in practice;
	// we leave the budget at the default and surface a clear error if
	// it ever bites.
	var out []Entry
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		// Strip a leading UTF-8 BOM on the very first line — Windows
		// editors love adding these. "\xEF\xBB\xBF" is the BOM byte
		// sequence; we use the escape form because Go's parser rejects
		// the raw BOM character inside string literals.
		if lineNo == 1 {
			line = strings.TrimPrefix(line, "\xEF\xBB\xBF")
		}
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		entry, err := parseLine(line, lineNo)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("dotenv: read: %w", err)
	}
	return out, nil
}

// parseLine parses one non-blank, non-comment line into an Entry.
func parseLine(line string, lineNo int) (Entry, error) {
	// Tolerate the "export KEY=VALUE" prefix produced by some
	// shells / generators.
	trim := strings.TrimSpace(line)
	if strings.HasPrefix(trim, "export ") {
		trim = strings.TrimSpace(trim[len("export "):])
	}

	eq := strings.IndexByte(trim, '=')
	if eq <= 0 {
		return Entry{}, fmt.Errorf("dotenv: line %d: missing '=' (%q)", lineNo, line)
	}
	key := strings.TrimSpace(trim[:eq])
	if !validKey(key) {
		return Entry{}, fmt.Errorf("dotenv: line %d: invalid key %q", lineNo, key)
	}
	rest := strings.TrimLeft(trim[eq+1:], " \t")
	value, err := parseValue(rest)
	if err != nil {
		return Entry{}, fmt.Errorf("dotenv: line %d: %w", lineNo, err)
	}
	return Entry{Line: lineNo, Key: key, Value: value}, nil
}

// parseValue decodes the right-hand side of KEY=. Handles quoting +
// escapes; bare values run through the first '#' or end of line.
func parseValue(rest string) (string, error) {
	if rest == "" {
		return "", nil
	}
	switch rest[0] {
	case '"':
		return parseDoubleQuoted(rest)
	case '\'':
		return parseSingleQuoted(rest)
	default:
		return parseBare(rest), nil
	}
}

func parseDoubleQuoted(s string) (string, error) {
	// Walk from index 1 looking for an unescaped closing ". Resolve
	// supported \-escapes as we go.
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			// Anything after the closing quote up to '#' or EOL is
			// ignored (treated as comment).
			return b.String(), nil
		}
		if c == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				// Unknown escape — preserve verbatim so values like
				// "C:\Users" don't get mangled if the operator forgot
				// to double the backslash.
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(c)
	}
	return "", errors.New("unterminated double-quoted string")
}

func parseSingleQuoted(s string) (string, error) {
	// Single-quoted strings are literal — no escapes.
	end := strings.IndexByte(s[1:], '\'')
	if end < 0 {
		return "", errors.New("unterminated single-quoted string")
	}
	return s[1 : 1+end], nil
}

func parseBare(s string) string {
	// Stop at the first '#' (inline comment) or end of line. Strip
	// trailing whitespace from the result.
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimRight(s, " \t")
}

// validKey enforces the conventional shell envvar charset:
// [A-Za-z_][A-Za-z0-9_]*. Operators with non-standard keys can quote
// them client-side; we won't write those into .env emission anyway.
func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '_':
			// ok
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// Emit writes entries as a .env file to w. Output is sorted by key for
// deterministic diffs (which is what pull → commit → diff cycles need).
// Values are double-quoted only when they would otherwise be ambiguous
// (contain whitespace, '#', or special chars).
func Emit(w io.Writer, entries map[string]string) error {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "%s=%s\n", k, formatValue(entries[k])); err != nil {
			return fmt.Errorf("emit %s: %w", k, err)
		}
	}
	return nil
}

// formatValue decides whether to quote. Quoting always uses double
// quotes with the same escape set Parse accepts, so a parse → emit
// → parse cycle is value-preserving.
func formatValue(v string) string {
	if v == "" {
		return ""
	}
	if !needsQuoting(v) {
		return v
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(v[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}

func needsQuoting(v string) bool {
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case ' ', '\t', '\n', '\r', '"', '\'', '#', '\\', '$':
			return true
		}
	}
	return false
}
