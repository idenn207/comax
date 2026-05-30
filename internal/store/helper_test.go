package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// newTestDB opens a fresh on-disk SQLite database inside t.TempDir(),
// applies the embedded schema, and registers a cleanup that closes the
// connection. The file lives in TempDir so Go's test runner deletes it
// when the test ends.
//
// We deliberately use a file-backed DB (not :memory:) because some
// connection-pooling behaviour in modernc.org/sqlite differs between
// :memory: and file-backed handles, and the production path is
// file-backed.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

// mustCreateProject inserts a project with the given name and fails the
// test on error. Used by repo tests that need a parent row in place.
func mustCreateProject(t *testing.T, db *sql.DB, name string) Project {
	t.Helper()
	p, err := NewProjectRepo(db).Create(context.Background(), name)
	if err != nil {
		t.Fatalf("seed project %q: %v", name, err)
	}
	return p
}

// mustCreateEnv inserts an env and fails the test on error.
func mustCreateEnv(t *testing.T, db *sql.DB, projectID int64, name, inheritsFrom string) Environment {
	t.Helper()
	e, err := NewEnvRepo(db).Create(context.Background(), projectID, name, inheritsFrom)
	if err != nil {
		t.Fatalf("seed env %q: %v", name, err)
	}
	return e
}
