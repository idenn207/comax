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

	tok, err := repo.Create(ctx, "bootstrap-admin", hashOf("plain-secret-token"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tok.ID == 0 {
		t.Error("zero ID")
	}
	if !bytes.Equal(tok.TokenHash, hashOf("plain-secret-token")) {
		t.Error("TokenHash mismatch")
	}

	if n, _ := repo.Count(ctx); n != 1 {
		t.Errorf("Count = %d; want 1", n)
	}
}

func TestTokenRepo_CreateConflictOnName(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "dup", hashOf("a")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, "dup", hashOf("b"))
	if !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v; want %v", err, ErrConflict)
	}
}

func TestTokenRepo_CreateConflictOnHash(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "first", hashOf("same")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, "second", hashOf("same"))
	if !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v; want %v", err, ErrConflict)
	}
}

func TestTokenRepo_ByHash(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	in, err := repo.Create(ctx, "ci-runner", hashOf("xyz"))
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
}

func TestTokenRepo_ByHashNotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := NewTokenRepo(db).ByHash(context.Background(), hashOf("missing"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v; want %v", err, ErrNotFound)
	}
}

func TestTokenRepo_TouchLastUsed(t *testing.T) {
	db := newTestDB(t)
	repo := NewTokenRepo(db)
	ctx := context.Background()

	tok, _ := repo.Create(ctx, "t", hashOf("h"))

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

	if _, err := repo.Create(ctx, "bootstrap", hashOf("seed")); err != nil {
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
