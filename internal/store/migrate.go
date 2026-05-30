package store

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

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
	return nil
}
