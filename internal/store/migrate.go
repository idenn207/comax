package store

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed schema.sql
var schemaSQL string

// additiveColumns are ALTER TABLE statements applied after schemaSQL.
// Each entry is idempotent: if the column already exists (fresh DB built
// from the latest schema.sql), SQLite reports "duplicate column name" and
// Migrate tolerates that. If the column is missing (existing M1 DB being
// upgraded to M2), SQLite adds it.
//
// SQLite does not support "ALTER TABLE ... ADD COLUMN IF NOT EXISTS", so
// the try-and-tolerate dance is the canonical workaround.
var additiveColumns = []string{
	`ALTER TABLE secrets ADD COLUMN deleted_at INTEGER`,
}

// Migrate applies the embedded schema to db. It is idempotent: every
// CREATE statement uses IF NOT EXISTS, so calling Migrate multiple times
// (e.g. on every server boot) is safe.
//
// Returning early on a partial apply is intentional — SQLite executes
// statements sequentially and a constraint mid-script leaves the
// previously-applied CREATEs in place, which IF NOT EXISTS will
// no-op on the next attempt.
func Migrate(ctx context.Context, db DBTX) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	for _, stmt := range additiveColumns {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isDuplicateColumn(err) {
				continue
			}
			return fmt.Errorf("apply additive column %q: %w", stmt, err)
		}
	}
	return nil
}

// isDuplicateColumn returns true when err is SQLite's "duplicate column
// name" failure — i.e. the column was already added by an earlier run
// or by a fresh schema.sql. Substring match because modernc.org/sqlite
// does not expose a typed sentinel for this case.
func isDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate column name")
}
