package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestSecretRepo_UpsertInsertPath(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	repo := NewSecretRepo(db)
	ctx := context.Background()

	res, err := repo.Upsert(ctx, env.ID, "DB_URL", []byte("ciphertext-v1"))
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !res.Created {
		t.Error("Created flag false on first upsert; want true")
	}
	if res.Secret.Version != 1 {
		t.Errorf("Version = %d; want 1", res.Secret.Version)
	}
	if !bytes.Equal(res.Secret.Ciphertext, []byte("ciphertext-v1")) {
		t.Errorf("Ciphertext mismatch")
	}
}

func TestSecretRepo_UpsertUpdatePath(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	repo := NewSecretRepo(db)
	ctx := context.Background()

	first, err := repo.Upsert(ctx, env.ID, "DB_URL", []byte("v1"))
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	// modernc's resolution is 1 second; sleep ensures updated_at advances
	// past created_at so the Created==false detection is reliable.
	time.Sleep(1100 * time.Millisecond)

	second, err := repo.Upsert(ctx, env.ID, "DB_URL", []byte("v2"))
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if second.Created {
		t.Error("Created flag true on update; want false")
	}
	if second.Secret.ID != first.Secret.ID {
		t.Errorf("ID changed: %d -> %d", first.Secret.ID, second.Secret.ID)
	}
	if second.Secret.Version != 2 {
		t.Errorf("Version = %d; want 2", second.Secret.Version)
	}
	if !bytes.Equal(second.Secret.Ciphertext, []byte("v2")) {
		t.Errorf("Ciphertext not updated")
	}
}

func TestSecretRepo_ByKey(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	repo := NewSecretRepo(db)
	ctx := context.Background()

	if _, err := repo.Upsert(ctx, env.ID, "KEY", []byte("value")); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := repo.ByKey(ctx, env.ID, "KEY")
	if err != nil {
		t.Fatalf("ByKey: %v", err)
	}
	if !bytes.Equal(got.Ciphertext, []byte("value")) {
		t.Errorf("Ciphertext mismatch")
	}
}

func TestSecretRepo_ByKeyNotFound(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")

	_, err := NewSecretRepo(db).ByKey(context.Background(), env.ID, "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v; want %v", err, ErrNotFound)
	}
}

func TestSecretRepo_ListByEnvOrdered(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	repo := NewSecretRepo(db)
	ctx := context.Background()

	// Insert out-of-order; expect alphabetic readback.
	for _, k := range []string{"ZED", "ALPHA", "MIDDLE"} {
		if _, err := repo.Upsert(ctx, env.ID, k, []byte(k)); err != nil {
			t.Fatalf("seed %q: %v", k, err)
		}
	}

	got, err := repo.ListByEnv(ctx, env.ID)
	if err != nil {
		t.Fatalf("ListByEnv: %v", err)
	}
	want := []string{"ALPHA", "MIDDLE", "ZED"}
	if len(got) != len(want) {
		t.Fatalf("len = %d; want %d", len(got), len(want))
	}
	for i, s := range got {
		if s.Key != want[i] {
			t.Errorf("got[%d].Key = %q; want %q", i, s.Key, want[i])
		}
	}
}

func TestSecretRepo_ListByEnvEmpty(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")

	got, err := NewSecretRepo(db).ListByEnv(context.Background(), env.ID)
	if err != nil {
		t.Fatalf("ListByEnv: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty; got %d", len(got))
	}
}
