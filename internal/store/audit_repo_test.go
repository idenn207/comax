package store

import (
	"context"
	"testing"
	"time"
)

func TestAuditRepo_AppendAndList(t *testing.T) {
	db := newTestDB(t)
	repo := NewAuditRepo(db)
	ctx := context.Background()

	// FK on actor_token → service_tokens.id; seed a real token row.
	tok, err := NewTokenRepo(db).Create(ctx, "actor", []byte("hash"), false)
	if err != nil {
		t.Fatalf("seed token: %v", err)
	}
	actor := tok.ID
	e, err := repo.Append(ctx, &actor, "secret.upsert", "project=app env=dev key=DB_URL", `{"version":2}`)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if e.ID == 0 {
		t.Error("zero ID")
	}
	if e.Action != "secret.upsert" {
		t.Errorf("Action = %q", e.Action)
	}
	if e.Metadata != `{"version":2}` {
		t.Errorf("Metadata = %q", e.Metadata)
	}
	if e.ActorToken == nil || *e.ActorToken != actor {
		t.Errorf("ActorToken = %v; want %d", e.ActorToken, actor)
	}

	list, err := repo.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d; want 1", len(list))
	}
	if list[0].ID != e.ID {
		t.Errorf("list[0].ID = %d; want %d", list[0].ID, e.ID)
	}
}

func TestAuditRepo_AppendSystemEvent(t *testing.T) {
	// Actor token nil, metadata empty — both should round-trip cleanly.
	db := newTestDB(t)
	e, err := NewAuditRepo(db).Append(context.Background(), nil, "server.boot", "version=dev", "")
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if e.ActorToken != nil {
		t.Errorf("ActorToken = %v; want nil", e.ActorToken)
	}
	if e.Metadata != "" {
		t.Errorf("Metadata = %q; want empty", e.Metadata)
	}
}

func TestAuditRepo_ListRecentRespectsLimit(t *testing.T) {
	db := newTestDB(t)
	repo := NewAuditRepo(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := repo.Append(ctx, nil, "noop", "x", ""); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	list, err := repo.ListRecent(ctx, 3)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len = %d; want 3", len(list))
	}
}

func TestAuditRepo_ListNewestFirst(t *testing.T) {
	// Within the same Unix-second the secondary sort is by id DESC, so
	// the most recently inserted row should appear first even when
	// created_at ties.
	db := newTestDB(t)
	repo := NewAuditRepo(db)
	ctx := context.Background()

	first, err := repo.Append(ctx, nil, "a", "1", "")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	// Sleep just enough to allow ordering checks in a portable way even
	// when clocks have ~1s resolution — but rely on id-DESC tiebreak
	// rather than the sleep.
	time.Sleep(20 * time.Millisecond)
	second, err := repo.Append(ctx, nil, "b", "2", "")
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	list, _ := repo.ListRecent(ctx, 10)
	if len(list) != 2 {
		t.Fatalf("len = %d", len(list))
	}
	if list[0].ID != second.ID {
		t.Errorf("expected newest first; got id %d, want %d", list[0].ID, second.ID)
	}
	if list[1].ID != first.ID {
		t.Errorf("expected first second; got id %d, want %d", list[1].ID, first.ID)
	}
}

func TestAuditRepo_ListEmpty(t *testing.T) {
	db := newTestDB(t)
	list, err := NewAuditRepo(db).ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty; got %d", len(list))
	}
}

func TestAuditRepo_ListFilterByAction(t *testing.T) {
	db := newTestDB(t)
	repo := NewAuditRepo(db)
	ctx := context.Background()
	if _, err := repo.Append(ctx, nil, "secret.upsert", "project=app env=dev key=A", ""); err != nil {
		t.Fatalf("seed upsert: %v", err)
	}
	if _, err := repo.Append(ctx, nil, "secret.delete", "project=app env=dev key=B", ""); err != nil {
		t.Fatalf("seed delete: %v", err)
	}
	got, err := repo.List(ctx, AuditFilter{Action: "secret.delete"}, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d; want 1", len(got))
	}
	if got[0].Action != "secret.delete" {
		t.Errorf("Action=%q; want secret.delete", got[0].Action)
	}
}

func TestAuditRepo_ListFilterByProjectEnv(t *testing.T) {
	db := newTestDB(t)
	repo := NewAuditRepo(db)
	ctx := context.Background()
	rows := []struct{ action, target string }{
		{"secret.upsert", "project=app env=dev key=A"},
		{"secret.upsert", "project=app env=prod key=B"},
		{"secret.upsert", "project=other env=dev key=C"},
	}
	for _, r := range rows {
		if _, err := repo.Append(ctx, nil, r.action, r.target, ""); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, err := repo.List(ctx, AuditFilter{Project: "app", Env: "dev"}, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d; want 1 (project=app env=dev)", len(got))
	}
	if got[0].Target != "project=app env=dev key=A" {
		t.Errorf("Target=%q", got[0].Target)
	}
}

func TestAuditRepo_ListBeforeCursor(t *testing.T) {
	db := newTestDB(t)
	repo := NewAuditRepo(db)
	ctx := context.Background()
	ids := []int64{}
	for i := 0; i < 5; i++ {
		e, err := repo.Append(ctx, nil, "noop", "x", "")
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
		ids = append(ids, e.ID)
	}
	// Cursor at the 3rd row (ids[2]) — expect rows with id < ids[2].
	got, err := repo.List(ctx, AuditFilter{BeforeID: ids[2]}, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d; want 2 (rows below cursor)", len(got))
	}
	for _, e := range got {
		if e.ID >= ids[2] {
			t.Errorf("returned row id=%d >= cursor=%d", e.ID, ids[2])
		}
	}
}

func TestAuditRepo_ListFilterByActor(t *testing.T) {
	db := newTestDB(t)
	repo := NewAuditRepo(db)
	ctx := context.Background()
	tok1, err := NewTokenRepo(db).Create(ctx, "a", []byte("h1"), false)
	if err != nil {
		t.Fatalf("seed tok1: %v", err)
	}
	tok2, err := NewTokenRepo(db).Create(ctx, "b", []byte("h2"), false)
	if err != nil {
		t.Fatalf("seed tok2: %v", err)
	}
	id1 := tok1.ID
	id2 := tok2.ID
	if _, err := repo.Append(ctx, &id1, "x", "t", ""); err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	if _, err := repo.Append(ctx, &id2, "y", "t", ""); err != nil {
		t.Fatalf("seed 2: %v", err)
	}

	got, err := repo.List(ctx, AuditFilter{ActorToken: &id1}, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d; want 1", len(got))
	}
	if got[0].ActorToken == nil || *got[0].ActorToken != id1 {
		t.Errorf("ActorToken=%v; want %d", got[0].ActorToken, id1)
	}
}
