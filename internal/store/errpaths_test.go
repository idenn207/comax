package store

import (
	"context"
	"testing"
)

// TestOpenFailsWhenPathIsADirectory exercises the ping-failure branch of
// Open: SQLite cannot open a path that already exists as a directory. We
// point Open at t.TempDir() itself (which is guaranteed to be a directory)
// so Ping fails on first use, surfacing the wrapped "ping sqlite" error.
func TestOpenFailsWhenPathIsADirectory(t *testing.T) {
	_, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("expected error opening a directory as a DB file; got nil")
	}
}

// TestRepoMethodsErrorOnClosedDB drives every repo entrypoint against a
// closed *sql.DB to exercise the non-UNIQUE failure branches. These would
// otherwise stay uncovered because the success and UNIQUE-conflict paths
// dominate normal test flow.
func TestRepoMethodsErrorOnClosedDB(t *testing.T) {
	db := newTestDB(t)
	_ = db.Close()
	ctx := context.Background()

	cases := []struct {
		name string
		fn   func() error
	}{
		{"Project.Create", func() error { _, err := NewProjectRepo(db).Create(ctx, "x"); return err }},
		{"Project.ByName", func() error { _, err := NewProjectRepo(db).ByName(ctx, "x"); return err }},
		{"Project.List", func() error { _, err := NewProjectRepo(db).List(ctx); return err }},
		{"Env.Create", func() error { _, err := NewEnvRepo(db).Create(ctx, 1, "x", ""); return err }},
		{"Env.ByName", func() error { _, err := NewEnvRepo(db).ByName(ctx, 1, "x"); return err }},
		{"Env.ListByProject", func() error { _, err := NewEnvRepo(db).ListByProject(ctx, 1); return err }},
		{"Secret.Upsert", func() error { _, err := NewSecretRepo(db).Upsert(ctx, 1, "x", nil); return err }},
		{"Secret.ByKey", func() error { _, err := NewSecretRepo(db).ByKey(ctx, 1, "x"); return err }},
		{"Secret.ListByEnv", func() error { _, err := NewSecretRepo(db).ListByEnv(ctx, 1); return err }},
		{"Version.Create", func() error {
			_, err := NewVersionRepo(db).Create(ctx, 1, 1, nil, nil)
			return err
		}},
		{"Version.ListBySecret", func() error { _, err := NewVersionRepo(db).ListBySecret(ctx, 1); return err }},
		{"Token.Count", func() error { _, err := NewTokenRepo(db).Count(ctx); return err }},
		{"Token.Create", func() error { _, err := NewTokenRepo(db).Create(ctx, "x", nil); return err }},
		{"Token.ByHash", func() error { _, err := NewTokenRepo(db).ByHash(ctx, nil); return err }},
		{"Token.TouchLastUsed", func() error { return NewTokenRepo(db).TouchLastUsed(ctx, 1) }},
		{"Audit.Append", func() error { _, err := NewAuditRepo(db).Append(ctx, nil, "x", "x", ""); return err }},
		{"Audit.ListRecent", func() error { _, err := NewAuditRepo(db).ListRecent(ctx, 1); return err }},
	}
	for _, tc := range cases {
		if err := tc.fn(); err == nil {
			t.Errorf("%s: expected error on closed DB; got nil", tc.name)
		}
	}
}
