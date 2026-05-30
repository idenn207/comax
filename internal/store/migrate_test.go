package store

import (
	"context"
	"errors"
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
