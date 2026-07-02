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
	`ALTER TABLE service_tokens ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE service_tokens ADD COLUMN revoked_at INTEGER`,
}

// adminBackfill promotes the lowest-id service token to admin when — and
// only when — no admin exists yet. It exists to preserve issuing rights
// for databases created before M3 (M1/M2): those tokens were inserted
// without is_admin, so the additive ALTER above lands them all at the
// DEFAULT 0. Without this, an upgraded deployment would have zero tokens
// able to issue or revoke, locking the operator out of token management.
//
// The NOT EXISTS guard makes it idempotent and self-limiting: once any
// admin exists (fresh DBs bootstrap their first token with is_admin=1 —
// see TokenRepo.BootstrapIfEmpty), re-running Migrate on every boot is a
// no-op. On an empty table MIN(id) is NULL, so the UPDATE matches zero
// rows and a brand-new database is unaffected.
const adminBackfill = `
UPDATE service_tokens
   SET is_admin = 1
 WHERE id = (SELECT MIN(id) FROM service_tokens)
   AND NOT EXISTS (SELECT 1 FROM service_tokens WHERE is_admin = 1)`

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
	// Run after the additive columns so is_admin is guaranteed to exist.
	if _, err := db.ExecContext(ctx, adminBackfill); err != nil {
		return fmt.Errorf("admin backfill: %w", err)
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
