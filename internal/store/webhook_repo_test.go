package store

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestWebhookRepo_CreateAndByID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	p := mustCreateProject(t, db, "app")
	e := mustCreateEnv(t, db, p.ID, "prod", "")
	repo := NewWebhookRepo(db)

	sealed := []byte("sealed-signing-key")
	w, err := repo.Create(ctx, p.ID, &e.ID, "https://ci.internal/hook", "secret.upsert,secret.delete", sealed)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if w.ID == 0 {
		t.Error("zero ID")
	}
	if !w.Enabled {
		t.Error("new webhook should be enabled")
	}
	if w.EnvID == nil || *w.EnvID != e.ID {
		t.Errorf("EnvID = %v; want %d", w.EnvID, e.ID)
	}

	got, err := repo.ByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if !bytes.Equal(got.SecretCiphertext, sealed) {
		t.Errorf("ByID SecretCiphertext = %q; want %q (ByID must return the sealed key)", got.SecretCiphertext, sealed)
	}
	if got.URL != "https://ci.internal/hook" {
		t.Errorf("URL = %q", got.URL)
	}
	if got.Events != "secret.upsert,secret.delete" {
		t.Errorf("Events = %q", got.Events)
	}
}

func TestWebhookRepo_CreateAllEnvs(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	p := mustCreateProject(t, db, "app")
	repo := NewWebhookRepo(db)

	w, err := repo.Create(ctx, p.ID, nil, "https://h/x", "secret.upsert", []byte("k"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.ByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.EnvID != nil {
		t.Errorf("EnvID = %v; want nil (all-envs webhook)", *got.EnvID)
	}
}

func TestWebhookRepo_ListOmitsSecret(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	p := mustCreateProject(t, db, "app")
	repo := NewWebhookRepo(db)
	if _, err := repo.Create(ctx, p.ID, nil, "https://h/1", "secret.upsert", []byte("secret-key-1")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.Create(ctx, p.ID, nil, "https://h/2", "secret.delete", []byte("secret-key-2")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List len = %d; want 2", len(list))
	}
	for _, w := range list {
		if w.SecretCiphertext != nil {
			t.Errorf("webhook %d: List leaked SecretCiphertext %q; want nil", w.ID, w.SecretCiphertext)
		}
	}
}

func TestWebhookRepo_DeleteAndNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	p := mustCreateProject(t, db, "app")
	repo := NewWebhookRepo(db)
	w, err := repo.Create(ctx, p.ID, nil, "https://h/x", "secret.upsert", []byte("k"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, w.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, w.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete err = %v; want ErrNotFound", err)
	}
	if _, err := repo.ByID(ctx, w.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ByID after delete err = %v; want ErrNotFound", err)
	}
}

func TestWebhookRepo_SetEnabled(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	p := mustCreateProject(t, db, "app")
	repo := NewWebhookRepo(db)
	w, err := repo.Create(ctx, p.ID, nil, "https://h/x", "secret.upsert", []byte("k"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.SetEnabled(ctx, w.ID, false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	got, err := repo.ByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Enabled {
		t.Error("Enabled = true after SetEnabled(false)")
	}
	if err := repo.SetEnabled(ctx, 9999, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetEnabled(absent) err = %v; want ErrNotFound", err)
	}
}

func TestWebhookRepo_ByIDNotFound(t *testing.T) {
	db := newTestDB(t)
	if _, err := NewWebhookRepo(db).ByID(context.Background(), 123); !errors.Is(err, ErrNotFound) {
		t.Errorf("ByID(absent) err = %v; want ErrNotFound", err)
	}
}

func TestWebhookRepo_MatchForEvent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	p := mustCreateProject(t, db, "app")
	other := mustCreateProject(t, db, "other")
	prod := mustCreateEnv(t, db, p.ID, "prod", "")
	dev := mustCreateEnv(t, db, p.ID, "dev", "")
	repo := NewWebhookRepo(db)

	// all-envs webhook for upsert+delete
	all, _ := repo.Create(ctx, p.ID, nil, "https://h/all", "secret.upsert,secret.delete", []byte("k"))
	// prod-only webhook for upsert
	prodHook, _ := repo.Create(ctx, p.ID, &prod.ID, "https://h/prod", "secret.upsert", []byte("k"))
	// disabled webhook — must never match
	disabled, _ := repo.Create(ctx, p.ID, nil, "https://h/off", "secret.upsert", []byte("k"))
	if err := repo.SetEnabled(ctx, disabled.ID, false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	// webhook on a different project — must never match app's events
	if _, err := repo.Create(ctx, other.ID, nil, "https://h/other", "secret.upsert", []byte("k")); err != nil {
		t.Fatalf("Create other: %v", err)
	}

	idsFor := func(projectID, envID int64, event string) []int64 {
		t.Helper()
		hits, err := repo.MatchForEvent(ctx, projectID, envID, event)
		if err != nil {
			t.Fatalf("MatchForEvent: %v", err)
		}
		var ids []int64
		for _, h := range hits {
			ids = append(ids, h.ID)
			if h.SecretCiphertext != nil {
				t.Errorf("MatchForEvent leaked SecretCiphertext for webhook %d", h.ID)
			}
		}
		return ids
	}

	// prod + upsert → all (all-envs) and prodHook, never disabled/other-project.
	if got := idsFor(p.ID, prod.ID, "secret.upsert"); !sameIDs(got, []int64{all.ID, prodHook.ID}) {
		t.Errorf("prod upsert matched %v; want %v", got, []int64{all.ID, prodHook.ID})
	}
	// dev + upsert → only the all-envs webhook (prodHook is prod-scoped).
	if got := idsFor(p.ID, dev.ID, "secret.upsert"); !sameIDs(got, []int64{all.ID}) {
		t.Errorf("dev upsert matched %v; want %v", got, []int64{all.ID})
	}
	// prod + delete → only the all-envs webhook (prodHook subscribes to upsert only).
	if got := idsFor(p.ID, prod.ID, "secret.delete"); !sameIDs(got, []int64{all.ID}) {
		t.Errorf("prod delete matched %v; want %v", got, []int64{all.ID})
	}
	// rollback → nobody subscribes.
	if got := idsFor(p.ID, prod.ID, "secret.rollback"); len(got) != 0 {
		t.Errorf("rollback matched %v; want none", got)
	}
	// substring guard: a bare "upsert" probe must NOT match "secret.upsert".
	if got := idsFor(p.ID, prod.ID, "upsert"); len(got) != 0 {
		t.Errorf("partial-token 'upsert' matched %v; want none (comma-wrapped exact match)", got)
	}
}

// sameIDs reports whether a and b contain the same ids (order-independent).
func sameIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[int64]int{}
	for _, id := range a {
		seen[id]++
	}
	for _, id := range b {
		seen[id]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}
