package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestSecretRepo_DeleteSoftDeletes(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	repo := NewSecretRepo(db)
	ctx := context.Background()

	if _, err := repo.Upsert(ctx, env.ID, "DB_URL", []byte("ct-v1")); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}
	if err := repo.Delete(ctx, env.ID, "DB_URL"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// ByKey filters deleted rows.
	if _, err := repo.ByKey(ctx, env.ID, "DB_URL"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ByKey after delete: err=%v; want ErrNotFound", err)
	}
	// ListByEnv excludes deleted rows.
	got, err := repo.ListByEnv(ctx, env.ID)
	if err != nil {
		t.Fatalf("ListByEnv: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListByEnv len=%d; want 0 (only secret was deleted)", len(got))
	}
	// ByKeyAny still finds the row.
	any, err := repo.ByKeyAny(ctx, env.ID, "DB_URL")
	if err != nil {
		t.Fatalf("ByKeyAny: %v", err)
	}
	if !bytes.Equal(any.Ciphertext, []byte("ct-v1")) {
		t.Errorf("ByKeyAny ciphertext = %q; want ct-v1", any.Ciphertext)
	}
}

func TestSecretRepo_DeleteReturnsNotFound(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	repo := NewSecretRepo(db)
	ctx := context.Background()

	// Never created.
	if err := repo.Delete(ctx, env.ID, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete on missing key: err=%v; want ErrNotFound", err)
	}
	// Created then deleted twice — second delete must return ErrNotFound.
	if _, err := repo.Upsert(ctx, env.ID, "K", []byte("v")); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}
	if err := repo.Delete(ctx, env.ID, "K"); err != nil {
		t.Fatalf("first Delete: %v", err)
	}
	if err := repo.Delete(ctx, env.ID, "K"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete: err=%v; want ErrNotFound", err)
	}
}

func TestSecretRepo_UpsertReactivatesDeleted(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	repo := NewSecretRepo(db)
	ctx := context.Background()

	first, err := repo.Upsert(ctx, env.ID, "K", []byte("v1"))
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if err := repo.Delete(ctx, env.ID, "K"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// Re-PUT brings the key back; version continues (v1 → v2), Created
	// remains false because the row physically exists.
	second, err := repo.Upsert(ctx, env.ID, "K", []byte("v2"))
	if err != nil {
		t.Fatalf("reactivating Upsert: %v", err)
	}
	if second.Secret.ID != first.Secret.ID {
		t.Errorf("ID changed after reactivation: %d -> %d", first.Secret.ID, second.Secret.ID)
	}
	if second.Secret.Version != 2 {
		t.Errorf("Version after reactivation = %d; want 2", second.Secret.Version)
	}
	if second.Created {
		t.Error("Created=true after reactivation; want false (row was reused)")
	}
	got, err := repo.ByKey(ctx, env.ID, "K")
	if err != nil {
		t.Fatalf("ByKey after reactivation: %v", err)
	}
	if !bytes.Equal(got.Ciphertext, []byte("v2")) {
		t.Errorf("Ciphertext after reactivation = %q; want v2", got.Ciphertext)
	}
}

func TestSecretRepo_ByKeyAnyMissing(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")

	_, err := NewSecretRepo(db).ByKeyAny(context.Background(), env.ID, "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err=%v; want ErrNotFound", err)
	}
}
