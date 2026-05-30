package store

import (
	"context"
	"errors"
	"testing"
)

func TestEnvRepo_CreateAndByName(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	repo := NewEnvRepo(db)
	ctx := context.Background()

	created, err := repo.Create(ctx, proj.ID, "dev", "shared")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.InheritsFrom != "shared" {
		t.Errorf("InheritsFrom = %q; want %q", created.InheritsFrom, "shared")
	}

	got, err := repo.ByName(ctx, proj.ID, "dev")
	if err != nil {
		t.Fatalf("ByName: %v", err)
	}
	if got.ID != created.ID || got.InheritsFrom != "shared" {
		t.Errorf("ByName mismatch: %+v vs %+v", got, created)
	}
}

func TestEnvRepo_CreateWithoutInheritance(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	repo := NewEnvRepo(db)

	created, err := repo.Create(context.Background(), proj.ID, "shared", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.InheritsFrom != "" {
		t.Errorf("InheritsFrom = %q; want empty", created.InheritsFrom)
	}

	got, _ := repo.ByName(context.Background(), proj.ID, "shared")
	if got.InheritsFrom != "" {
		t.Errorf("ByName InheritsFrom = %q; want empty", got.InheritsFrom)
	}
}

func TestEnvRepo_CreateConflictPerProject(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	repo := NewEnvRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, proj.ID, "dev", ""); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, proj.ID, "dev", "")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("dup error = %v; want %v", err, ErrConflict)
	}
}

func TestEnvRepo_SameNameDifferentProjects(t *testing.T) {
	// "dev" is allowed in two different projects — uniqueness is scoped.
	db := newTestDB(t)
	a := mustCreateProject(t, db, "a")
	b := mustCreateProject(t, db, "b")
	repo := NewEnvRepo(db)
	ctx := context.Background()

	if _, err := repo.Create(ctx, a.ID, "dev", ""); err != nil {
		t.Fatalf("a/dev: %v", err)
	}
	if _, err := repo.Create(ctx, b.ID, "dev", ""); err != nil {
		t.Fatalf("b/dev: %v", err)
	}
}

func TestEnvRepo_ByNameNotFound(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	_, err := NewEnvRepo(db).ByName(context.Background(), proj.ID, "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v; want %v", err, ErrNotFound)
	}
}

func TestEnvRepo_ListByProject(t *testing.T) {
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	repo := NewEnvRepo(db)
	ctx := context.Background()

	for _, name := range []string{"prod", "dev", "shared"} {
		mustCreateEnv(t, db, proj.ID, name, "")
	}

	got, err := repo.ListByProject(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	want := []string{"dev", "prod", "shared"} // ordered by name
	if len(got) != len(want) {
		t.Fatalf("len = %d; want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.Name != want[i] {
			t.Errorf("got[%d].Name = %q; want %q", i, e.Name, want[i])
		}
	}
}

func TestEnvRepo_ProjectFKCascade(t *testing.T) {
	// Deleting the parent project should remove its envs (ON DELETE CASCADE).
	db := newTestDB(t)
	proj := mustCreateProject(t, db, "app")
	mustCreateEnv(t, db, proj.ID, "dev", "")

	if _, err := db.ExecContext(context.Background(),
		`DELETE FROM projects WHERE id = ?`, proj.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	got, err := NewEnvRepo(db).ListByProject(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected envs cascaded; got %d", len(got))
	}
}
