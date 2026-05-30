package store

import (
	"context"
	"errors"
	"testing"
)

func TestProjectRepo_CreateAndByName(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	created, err := repo.Create(ctx, "comax-api")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Error("Create returned zero ID")
	}
	if created.Name != "comax-api" {
		t.Errorf("Name = %q; want %q", created.Name, "comax-api")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("timestamps should be set after Create")
	}

	got, err := repo.ByName(ctx, "comax-api")
	if err != nil {
		t.Fatalf("ByName: %v", err)
	}
	if got.ID != created.ID || got.Name != created.Name {
		t.Errorf("ByName mismatch: got %+v want %+v", got, created)
	}
}

func TestProjectRepo_CreateConflict(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "dup"); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, "dup")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("second Create error = %v; want %v", err, ErrConflict)
	}
}

func TestProjectRepo_ByNameNotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectRepo(db)

	_, err := repo.ByName(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ByName error = %v; want %v", err, ErrNotFound)
	}
}

func TestProjectRepo_ListOrderedByName(t *testing.T) {
	db := newTestDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if _, err := repo.Create(ctx, name); err != nil {
			t.Fatalf("seed %q: %v", name, err)
		}
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"alpha", "bravo", "charlie"}
	if len(got) != len(want) {
		t.Fatalf("List len = %d; want %d", len(got), len(want))
	}
	for i, p := range got {
		if p.Name != want[i] {
			t.Errorf("got[%d] = %q; want %q", i, p.Name, want[i])
		}
	}
}

func TestProjectRepo_ListEmpty(t *testing.T) {
	db := newTestDB(t)
	got, err := NewProjectRepo(db).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list; got %d", len(got))
	}
}
