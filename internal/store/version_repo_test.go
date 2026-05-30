package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestVersionRepo_CreateAndList(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	secretRepo := NewSecretRepo(db)
	versRepo := NewVersionRepo(db)
	ctx := context.Background()

	up, err := secretRepo.Upsert(ctx, env.ID, "KEY", []byte("v1"))
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Seed a real service token so the FK to service_tokens.id resolves.
	tok, err := NewTokenRepo(db).Create(ctx, "actor", []byte("hash"))
	if err != nil {
		t.Fatalf("seed token: %v", err)
	}
	actor := tok.ID
	v, err := versRepo.Create(ctx, up.Secret.ID, up.Secret.Version, up.Secret.Ciphertext, &actor)
	if err != nil {
		t.Fatalf("Create version: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("Version = %d; want 1", v.Version)
	}
	if v.ActorToken == nil || *v.ActorToken != actor {
		t.Errorf("ActorToken = %v; want %d", v.ActorToken, actor)
	}

	list, err := versRepo.ListBySecret(ctx, up.Secret.ID)
	if err != nil {
		t.Fatalf("ListBySecret: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d; want 1", len(list))
	}
	if !bytes.Equal(list[0].Ciphertext, []byte("v1")) {
		t.Errorf("Ciphertext mismatch")
	}
}

func TestVersionRepo_CreateWithNilActor(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	up, _ := NewSecretRepo(db).Upsert(context.Background(), env.ID, "K", []byte("v"))

	v, err := NewVersionRepo(db).Create(context.Background(), up.Secret.ID, 1, []byte("v"), nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.ActorToken != nil {
		t.Errorf("ActorToken = %v; want nil", v.ActorToken)
	}
}

func TestVersionRepo_DuplicateVersionConflict(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	up, _ := NewSecretRepo(db).Upsert(context.Background(), env.ID, "K", []byte("v"))
	versRepo := NewVersionRepo(db)
	ctx := context.Background()

	if _, err := versRepo.Create(ctx, up.Secret.ID, 1, []byte("v"), nil); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := versRepo.Create(ctx, up.Secret.ID, 1, []byte("v"), nil)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("dup version err = %v; want %v", err, ErrConflict)
	}
}

func TestVersionRepo_ListBySecretNewestFirst(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	env := mustCreateEnv(t, db, proj.ID, "dev", "")
	up, _ := NewSecretRepo(db).Upsert(context.Background(), env.ID, "K", []byte("v"))
	versRepo := NewVersionRepo(db)
	ctx := context.Background()

	for ver := int64(1); ver <= 3; ver++ {
		if _, err := versRepo.Create(ctx, up.Secret.ID, ver, []byte{byte(ver)}, nil); err != nil {
			t.Fatalf("seed v%d: %v", ver, err)
		}
	}

	list, err := versRepo.ListBySecret(ctx, up.Secret.ID)
	if err != nil {
		t.Fatalf("ListBySecret: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d; want 3", len(list))
	}
	for i, want := range []int64{3, 2, 1} {
		if list[i].Version != want {
			t.Errorf("list[%d].Version = %d; want %d", i, list[i].Version, want)
		}
	}
}

func TestVersionRepo_ListEmpty(t *testing.T) {
	db := newTestDB(t)
	list, err := NewVersionRepo(db).ListBySecret(context.Background(), 9999)
	if err != nil {
		t.Fatalf("ListBySecret: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty; got %d", len(list))
	}
}
