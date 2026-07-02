package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
)

func hashOf(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func TestTokenRepo_CountStartsAtZero(t *testing.T) {
	db := newTestDB(t)
	n, err := NewTokenRepo(db).Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("Count = %d; want 0", n)
	}
}

func TestTokenRepo_CreateAndCount(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, err := repo.Create(ctx, "bootstrap-admin", hashOf("plain-secret-token"), false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tok.ID == 0 {
		t.Error("zero ID")
	}
	if !bytes.Equal(tok.TokenHash, hashOf("plain-secret-token")) {
		t.Error("TokenHash mismatch")
	}
	if tok.IsAdmin {
		t.Error("IsAdmin = true; want false for Create(...,false)")
	}

	if n, _ := repo.Count(ctx); n != 1 {
		t.Errorf("Count = %d; want 1", n)
	}
}

func TestTokenRepo_CreateAdmin(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, err := repo.Create(ctx, "admin", hashOf("admin-tok"), true)
	if err != nil {
		t.Fatalf("Create admin: %v", err)
	}
	if !tok.IsAdmin {
		t.Error("returned IsAdmin = false; want true")
	}
	// And the flag round-trips through a fresh lookup.
	got, err := repo.ByHash(ctx, hashOf("admin-tok"))
	if err != nil {
		t.Fatalf("ByHash: %v", err)
	}
	if !got.IsAdmin {
		t.Error("ByHash IsAdmin = false; want true")
	}
}

func TestTokenRepo_CreateConflictOnName(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "dup", hashOf("a"), false); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, "dup", hashOf("b"), false)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v; want %v", err, ErrConflict)
	}
}

func TestTokenRepo_CreateConflictOnHash(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "first", hashOf("same"), false); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, "second", hashOf("same"), false)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v; want %v", err, ErrConflict)
	}
}

func TestTokenRepo_ByHash(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	in, err := repo.Create(ctx, "ci-runner", hashOf("xyz"), false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.ByHash(ctx, hashOf("xyz"))
	if err != nil {
		t.Fatalf("ByHash: %v", err)
	}
	if got.ID != in.ID || got.Name != "ci-runner" {
		t.Errorf("ByHash mismatch: %+v vs %+v", got, in)
	}
	if got.LastUsedAt != nil {
		t.Error("LastUsedAt should be nil before TouchLastUsed")
	}
	if got.RevokedAt != nil {
		t.Error("RevokedAt should be nil for a live token")
	}
}

func TestTokenRepo_ByHashNotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := NewTokenRepo(db).ByHash(context.Background(), hashOf("missing"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v; want %v", err, ErrNotFound)
	}
}

func TestTokenRepo_ByHashRejectsRevoked(t *testing.T) {
	// The bearer-auth arm must treat a revoked token as if it never
	// existed — ByHash returns ErrNotFound after Revoke.
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, err := repo.Create(ctx, "ci", hashOf("bearer"), false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Revoke(ctx, tok.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	_, err = repo.ByHash(ctx, hashOf("bearer"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ByHash after revoke err = %v; want %v", err, ErrNotFound)
	}
}

func TestTokenRepo_ByIDSeesRevoked(t *testing.T) {
	// The dashboard session arm (ByID) must still observe a revoked token
	// so it can terminate the live session (R2-1). Unlike ByHash, ByID
	// does not filter revoked rows.
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, err := repo.Create(ctx, "sess", hashOf("sess-bearer"), false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Revoke(ctx, tok.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got, err := repo.ByID(ctx, tok.ID)
	if err != nil {
		t.Fatalf("ByID after revoke: %v", err)
	}
	if got.RevokedAt == nil {
		t.Error("ByID RevokedAt = nil; want non-nil so the session arm can 401")
	}
}

func TestTokenRepo_TouchLastUsed(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, _ := repo.Create(ctx, "t", hashOf("h"), false)

	if err := repo.TouchLastUsed(ctx, tok.ID); err != nil {
		t.Fatalf("TouchLastUsed: %v", err)
	}

	got, _ := repo.ByHash(ctx, hashOf("h"))
	if got.LastUsedAt == nil {
		t.Fatal("LastUsedAt still nil after TouchLastUsed")
	}
}

func TestTokenRepo_TouchLastUsedNotFound(t *testing.T) {
	db := newTestDB(t)
	err := NewTokenRepo(db).TouchLastUsed(context.Background(), 9999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v; want %v", err, ErrNotFound)
	}
}

func TestTokenRepo_List(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	admin, err := repo.Create(ctx, "admin", hashOf("adm"), true)
	if err != nil {
		t.Fatalf("Create admin: %v", err)
	}
	ci, err := repo.Create(ctx, "ci", hashOf("ci"), false)
	if err != nil {
		t.Fatalf("Create ci: %v", err)
	}
	if err := repo.Revoke(ctx, ci.ID); err != nil {
		t.Fatalf("Revoke ci: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List len = %d; want 2", len(list))
	}
	// Ordered by id: admin first, ci second.
	if list[0].ID != admin.ID || list[1].ID != ci.ID {
		t.Errorf("List order = [%d,%d]; want [%d,%d]", list[0].ID, list[1].ID, admin.ID, ci.ID)
	}
	if !list[0].IsAdmin {
		t.Error("list[0].IsAdmin = false; want true")
	}
	// token_hash never leaves the store on a listing.
	if list[0].TokenHash != nil || list[1].TokenHash != nil {
		t.Error("List populated TokenHash; want nil (hash must not leak)")
	}
	// The revoked ci token surfaces its RevokedAt so callers can render status.
	if list[1].RevokedAt == nil {
		t.Error("list[1].RevokedAt = nil; want non-nil for the revoked token")
	}
}

func TestTokenRepo_Revoke(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, err := repo.Create(ctx, "victim", hashOf("v"), false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Revoke(ctx, tok.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	// Idempotent-hostile by design: a second revoke reports ErrNotFound.
	if err := repo.Revoke(ctx, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Revoke err = %v; want %v", err, ErrNotFound)
	}
}

func TestTokenRepo_RevokeAbsent(t *testing.T) {
	db := newTestDB(t)
	err := NewTokenRepo(db).Revoke(context.Background(), 9999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Revoke(absent) err = %v; want %v", err, ErrNotFound)
	}
}

func TestTokenRepo_BootstrapIfEmpty(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, created, err := repo.BootstrapIfEmpty(ctx, "bootstrap", hashOf("plain-one"))
	if err != nil {
		t.Fatalf("first BootstrapIfEmpty: %v", err)
	}
	if !created || tok.ID == 0 {
		t.Fatalf("first call: created=%v id=%d; want created=true id>0", created, tok.ID)
	}
	// The bootstrap token is admin so a fresh deployment can manage tokens.
	if !tok.IsAdmin {
		t.Error("bootstrap token IsAdmin = false; want true")
	}

	// Second call must be a no-op — table is no longer empty.
	zero, created2, err := repo.BootstrapIfEmpty(ctx, "another", hashOf("plain-two"))
	if err != nil {
		t.Fatalf("second BootstrapIfEmpty: %v", err)
	}
	if created2 {
		t.Error("second call: created=true; want false")
	}
	if zero.ID != 0 {
		t.Errorf("second call ID = %d; want 0", zero.ID)
	}

	if n, _ := repo.Count(ctx); n != 1 {
		t.Errorf("Count = %d; want 1 after two bootstrap attempts", n)
	}
}

func TestTokenRepo_BootstrapIfEmpty_NameCollisionFoldsToFalse(t *testing.T) {
	// Pre-seed a row, then call BootstrapIfEmpty with the same name. The
	// WHERE COUNT(*) = 0 guard already makes this a no-op; the UNIQUE on
	// name is the defensive belt-and-braces, exercised here to lock in
	// that the helper does not surface ErrConflict in this branch.
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "bootstrap", hashOf("seed"), false); err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	_, created, err := repo.BootstrapIfEmpty(ctx, "bootstrap", hashOf("new"))
	if err != nil {
		t.Fatalf("BootstrapIfEmpty: %v", err)
	}
	if created {
		t.Error("created=true on pre-seeded table; want false")
	}
}
