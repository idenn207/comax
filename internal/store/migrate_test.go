package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestMigrateIsIdempotent(t *testing.T) {
	db := newTestDB(t) // already runs Migrate once
	// Second call should be a no-op since every CREATE uses IF NOT EXISTS.
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	// And a third for good measure — ensures repeated boots stay clean.
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("third Migrate: %v", err)
	}
}

func TestMigrateSeedsAllExpectedTables(t *testing.T) {
	db := newTestDB(t)
	want := []string{
		"projects",
		"environments",
		"secrets",
		"secret_versions",
		"service_tokens",
		"audit_log",
	}
	for _, table := range want {
		var name string
		err := db.QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`,
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
			continue
		}
		if name != table {
			t.Errorf("table %q: scanned %q", table, name)
		}
	}
}

func TestMigrateBackfillsAdminOnUpgrade(t *testing.T) {
	// Simulate an M1/M2 database: service_tokens predates is_admin/revoked_at.
	dbPath := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	// Old-shape table (no is_admin, no revoked_at) + two pre-existing tokens.
	if _, err := db.ExecContext(ctx, `CREATE TABLE service_tokens (
		id           INTEGER PRIMARY KEY,
		name         TEXT NOT NULL UNIQUE,
		token_hash   BLOB NOT NULL UNIQUE,
		created_at   INTEGER NOT NULL,
		last_used_at INTEGER
	)`); err != nil {
		t.Fatalf("seed old table: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO service_tokens (name, token_hash, created_at)
		 VALUES ('bootstrap', X'00', 100), ('ci', X'01', 200)`,
	); err != nil {
		t.Fatalf("seed tokens: %v", err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate upgrade: %v", err)
	}

	adminOf := func(name string) int {
		t.Helper()
		var v int
		if err := db.QueryRowContext(ctx,
			`SELECT is_admin FROM service_tokens WHERE name = ?`, name,
		).Scan(&v); err != nil {
			t.Fatalf("scan is_admin(%s): %v", name, err)
		}
		return v
	}

	// The additive columns now exist and only the lowest-id token is admin.
	if got := adminOf("bootstrap"); got != 1 {
		t.Errorf("bootstrap is_admin = %d; want 1 (min-id promoted)", got)
	}
	if got := adminOf("ci"); got != 0 {
		t.Errorf("ci is_admin = %d; want 0 (only min-id promoted)", got)
	}

	// revoked_at is present and defaults to NULL.
	var revoked sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT revoked_at FROM service_tokens WHERE name = 'bootstrap'`,
	).Scan(&revoked); err != nil {
		t.Fatalf("scan revoked_at: %v", err)
	}
	if revoked.Valid {
		t.Errorf("revoked_at = %d; want NULL after upgrade", revoked.Int64)
	}

	// Idempotent: with an admin already present, a second Migrate must not
	// re-promote. Move admin to the ci token and confirm it survives.
	if _, err := db.ExecContext(ctx, `UPDATE service_tokens SET is_admin = 0 WHERE name = 'bootstrap'`); err != nil {
		t.Fatalf("demote bootstrap: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE service_tokens SET is_admin = 1 WHERE name = 'ci'`); err != nil {
		t.Fatalf("promote ci: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if got := adminOf("bootstrap"); got != 0 {
		t.Errorf("after re-migrate bootstrap is_admin = %d; want 0 (no re-promote)", got)
	}
	if got := adminOf("ci"); got != 1 {
		t.Errorf("after re-migrate ci is_admin = %d; want 1 (preserved)", got)
	}
}

func TestMigrateSurfaceErrors(t *testing.T) {
	// Closed DB → ExecContext fails → Migrate wraps the error.
	db := newTestDB(t)
	_ = db.Close()

	err := Migrate(context.Background(), db)
	if err == nil {
		t.Fatal("expected error from closed DB, got nil")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("unexpected sentinel: %v", err)
	}
}
