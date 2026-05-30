package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenAppliesForeignKeysPragma(t *testing.T) {
	db := newTestDB(t)

	// Inserting an environment that references a non-existent project
	// must fail when foreign_keys is ON. If the pragma was not applied
	// SQLite would silently accept the orphan row.
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO environments (project_id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		99999, "ghost", nowUnix(), nowUnix(),
	)
	if err == nil {
		t.Fatal("expected FK violation; got nil")
	}
	if !strings.Contains(err.Error(), "FOREIGN KEY") && !strings.Contains(err.Error(), "foreign key") {
		t.Fatalf("expected FK error, got %v", err)
	}
}

func TestOpenAcceptsURIAndBarePathForms(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{"barePath", filepath.Join(t.TempDir(), "bare.db")},
		{"fileURI", "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "uri.db"))},
		{"inMemory", ":memory:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := Open(tc.dsn)
			if err != nil {
				t.Fatalf("Open(%q): %v", tc.dsn, err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if err := db.PingContext(context.Background()); err != nil {
				t.Fatalf("ping: %v", err)
			}
		})
	}
}

func TestOpenIsIdempotentForPragma(t *testing.T) {
	// Passing a DSN that already names the pragma should not result in
	// the pragma appearing twice (which modernc would reject).
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "pragma.db")) + "?_pragma=foreign_keys(1)"
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
}

func TestIsUniqueViolationDetectsBothPhrasings(t *testing.T) {
	cases := map[string]bool{
		"UNIQUE constraint failed: projects.name":               true,
		"constraint failed: UNIQUE constraint failed: x.y":      true,
		"some other error":                                      false,
		"":                                                      false,
	}
	for msg, want := range cases {
		got := isUniqueViolation(stubErr(msg))
		if got != want {
			t.Errorf("isUniqueViolation(%q) = %v; want %v", msg, got, want)
		}
	}
	if isUniqueViolation(nil) {
		t.Error("isUniqueViolation(nil) returned true")
	}
}

func TestUnixSecondsRoundTrip(t *testing.T) {
	// Zero round-trips to the Go zero value.
	if got := unixSeconds(0); !got.IsZero() {
		t.Errorf("unixSeconds(0) = %v; want zero time", got)
	}
	// Non-zero round-trips to UTC.
	now := time.Now().UTC().Unix()
	got := unixSeconds(now)
	if got.Location() != time.UTC {
		t.Errorf("unixSeconds Location = %v; want UTC", got.Location())
	}
	if got.Unix() != now {
		t.Errorf("unixSeconds(%d) = %d; want %d", now, got.Unix(), now)
	}
}

// stubErr is a tiny error type for parser tests so we don't need to
// trigger a real SQLite UNIQUE violation just to exercise the matcher.
type stubError string

func (e stubError) Error() string { return string(e) }

func stubErr(msg string) error {
	if msg == "" {
		return stubError("")
	}
	return stubError(msg)
}
