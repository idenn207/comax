package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// newDeliveryFixture seeds a project + webhook so delivery rows have a valid FK
// parent, and returns a DeliveryRepo plus the webhook id.
func newDeliveryFixture(t *testing.T) (*DeliveryRepo, int64) {
	t.Helper()
	db := newTestDB(t)
	p := mustCreateProject(t, db, "app")
	w, err := NewWebhookRepo(db).Create(context.Background(), p.ID, nil, "https://h/x", "secret.upsert", []byte("k"))
	if err != nil {
		t.Fatalf("seed webhook: %v", err)
	}
	return NewDeliveryRepo(db), w.ID
}

func TestDeliveryRepo_EnqueueDefaults(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()

	d, err := repo.Enqueue(ctx, wid, "secret.upsert", `{"key":"DB_URL"}`)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if d.Status != DeliveryPending {
		t.Errorf("Status = %q; want %q", d.Status, DeliveryPending)
	}
	if d.Attempts != 0 {
		t.Errorf("Attempts = %d; want 0", d.Attempts)
	}
	got, err := repo.ByID(ctx, d.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Payload != `{"key":"DB_URL"}` {
		t.Errorf("Payload = %q", got.Payload)
	}
}

func TestDeliveryRepo_ClaimDueRespectsSchedule(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()

	due, err := repo.Enqueue(ctx, wid, "secret.upsert", "{}")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// A claim in the far past sees nothing due yet (the row's next_attempt_at
	// is ~now, which is after this claim horizon).
	past := time.Now().Add(-time.Hour)
	if claimed, err := repo.ClaimDue(ctx, past, 10); err != nil || len(claimed) != 0 {
		t.Fatalf("ClaimDue(past) = %v, %v; want none", claimed, err)
	}

	// A claim now takes the row and moves it to in_progress with a lease.
	claimed, err := repo.ClaimDue(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("ClaimDue(now): %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != due.ID {
		t.Fatalf("ClaimDue(now) = %v; want [%d]", claimed, due.ID)
	}
	if claimed[0].Status != DeliveryInProgress || claimed[0].ClaimedAt == nil {
		t.Errorf("claimed row not marked in_progress with lease: %+v", claimed[0])
	}

	// A second claim finds nothing — the row is already in_progress.
	if again, err := repo.ClaimDue(ctx, time.Now(), 10); err != nil || len(again) != 0 {
		t.Fatalf("second ClaimDue = %v, %v; want none", again, err)
	}
}

func TestDeliveryRepo_ConcurrentClaimNoDouble(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()

	const n = 12
	want := map[int64]bool{}
	for i := 0; i < n; i++ {
		d, err := repo.Enqueue(ctx, wid, "secret.upsert", "{}")
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		want[d.ID] = true
	}

	// Two workers race to claim the same pool. The per-row compare-and-swap
	// must hand each row to exactly one of them.
	var (
		mu      sync.Mutex
		claimed []int64
		wg      sync.WaitGroup
	)
	worker := func() {
		defer wg.Done()
		rows, err := repo.ClaimDue(ctx, time.Now(), n)
		if err != nil {
			t.Errorf("ClaimDue: %v", err)
			return
		}
		mu.Lock()
		for _, d := range rows {
			claimed = append(claimed, d.ID)
		}
		mu.Unlock()
	}
	wg.Add(2)
	go worker()
	go worker()
	wg.Wait()

	if len(claimed) != n {
		t.Fatalf("total claimed = %d; want %d (no row claimed twice, none dropped)", len(claimed), n)
	}
	seen := map[int64]bool{}
	for _, id := range claimed {
		if seen[id] {
			t.Errorf("delivery %d claimed more than once", id)
		}
		seen[id] = true
		if !want[id] {
			t.Errorf("claimed unexpected id %d", id)
		}
	}
	if len(seen) != n {
		t.Errorf("distinct claimed = %d; want %d", len(seen), n)
	}
}

func TestDeliveryRepo_MarkDeliveredGuarded(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()
	d, _ := repo.Enqueue(ctx, wid, "secret.upsert", "{}")

	// Guard: a pending (unclaimed) row cannot be delivered.
	if err := repo.MarkDelivered(ctx, d.ID, 200); !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkDelivered(pending) = %v; want ErrNotFound (WHERE in_progress guard)", err)
	}

	if _, err := repo.ClaimDue(ctx, time.Now(), 10); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if err := repo.MarkDelivered(ctx, d.ID, 204); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}
	got, _ := repo.ByID(ctx, d.ID)
	if got.Status != DeliveryDelivered {
		t.Errorf("Status = %q; want delivered", got.Status)
	}
	if got.Attempts != 1 {
		t.Errorf("Attempts = %d; want 1", got.Attempts)
	}
	if got.DeliveredAt == nil {
		t.Error("DeliveredAt not set")
	}
	if got.LastStatus == nil || *got.LastStatus != 204 {
		t.Errorf("LastStatus = %v; want 204", got.LastStatus)
	}
	if got.ClaimedAt != nil {
		t.Error("ClaimedAt should be cleared after delivery")
	}
}

func TestDeliveryRepo_MarkRetryReschedules(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()
	d, _ := repo.Enqueue(ctx, wid, "secret.upsert", "{}")
	if _, err := repo.ClaimDue(ctx, time.Now(), 10); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	next := time.Now().Add(30 * time.Second)
	if err := repo.MarkRetry(ctx, d.ID, next, 503, "service unavailable"); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	got, _ := repo.ByID(ctx, d.ID)
	if got.Status != DeliveryPending {
		t.Errorf("Status = %q; want pending (retry re-enters the queue)", got.Status)
	}
	if got.Attempts != 1 {
		t.Errorf("Attempts = %d; want 1", got.Attempts)
	}
	if got.ClaimedAt != nil {
		t.Error("ClaimedAt should be cleared on retry")
	}
	if got.NextAttemptAt.Unix() != next.UTC().Unix() {
		t.Errorf("NextAttemptAt = %v; want %v", got.NextAttemptAt, next.UTC())
	}
	// A retried row is not due until next_attempt_at elapses.
	if claimed, _ := repo.ClaimDue(ctx, time.Now(), 10); len(claimed) != 0 {
		t.Errorf("retried row claimed too early: %v", claimed)
	}
	// ...but is claimable once the backoff window passes.
	if claimed, _ := repo.ClaimDue(ctx, next.Add(time.Second), 10); len(claimed) != 1 {
		t.Errorf("retried row not claimable after backoff: %v", claimed)
	}
}

func TestDeliveryRepo_MarkRetryTransportError(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()
	d, _ := repo.Enqueue(ctx, wid, "secret.upsert", "{}")
	if _, err := repo.ClaimDue(ctx, time.Now(), 10); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	// statusCode 0 = transport failure (no HTTP response) → NULL last_status.
	if err := repo.MarkRetry(ctx, d.ID, time.Now().Add(time.Minute), 0, "dial timeout"); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	got, _ := repo.ByID(ctx, d.ID)
	if got.LastStatus != nil {
		t.Errorf("LastStatus = %v; want nil for transport error", *got.LastStatus)
	}
	if got.LastError != "dial timeout" {
		t.Errorf("LastError = %q", got.LastError)
	}
}

func TestDeliveryRepo_MarkDead(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()
	d, _ := repo.Enqueue(ctx, wid, "secret.upsert", "{}")
	if _, err := repo.ClaimDue(ctx, time.Now(), 10); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if err := repo.MarkDead(ctx, d.ID, 500, "gave up"); err != nil {
		t.Fatalf("MarkDead: %v", err)
	}
	got, _ := repo.ByID(ctx, d.ID)
	if got.Status != DeliveryDead {
		t.Errorf("Status = %q; want dead", got.Status)
	}
	// A dead row is terminal — never claimed again.
	if claimed, _ := repo.ClaimDue(ctx, time.Now().Add(time.Hour), 10); len(claimed) != 0 {
		t.Errorf("dead row was claimed: %v", claimed)
	}
}

func TestDeliveryRepo_ReclaimStale(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()
	d, _ := repo.Enqueue(ctx, wid, "secret.upsert", "{}")

	claimT := time.Now()
	if _, err := repo.ClaimDue(ctx, claimT, 10); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	// A horizon before the lease reclaims nothing (worker is still "alive").
	if n, err := repo.ReclaimStale(ctx, claimT.Add(-time.Minute)); err != nil || n != 0 {
		t.Fatalf("ReclaimStale(fresh) = %d, %v; want 0", n, err)
	}
	// A horizon after the lease reclaims the crashed worker's row.
	n, err := repo.ReclaimStale(ctx, claimT.Add(time.Minute))
	if err != nil {
		t.Fatalf("ReclaimStale: %v", err)
	}
	if n != 1 {
		t.Fatalf("ReclaimStale = %d; want 1", n)
	}
	got, _ := repo.ByID(ctx, d.ID)
	if got.Status != DeliveryPending {
		t.Errorf("Status = %q; want pending after reclaim", got.Status)
	}
	if got.ClaimedAt != nil {
		t.Error("ClaimedAt should be cleared after reclaim")
	}
	if got.Attempts != 0 {
		t.Errorf("Attempts = %d; want 0 (reclaim does not count as an attempt)", got.Attempts)
	}
}

func TestDeliveryRepo_ListByWebhook(t *testing.T) {
	repo, wid := newDeliveryFixture(t)
	ctx := context.Background()
	var last int64
	for i := 0; i < 3; i++ {
		d, err := repo.Enqueue(ctx, wid, "secret.upsert", "{}")
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		last = d.ID
	}
	list, err := repo.ListByWebhook(ctx, wid, 10)
	if err != nil {
		t.Fatalf("ListByWebhook: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d; want 3", len(list))
	}
	if list[0].ID != last {
		t.Errorf("newest-first ordering broken: list[0].ID = %d; want %d", list[0].ID, last)
	}
}

func TestDeliveryRepo_ByIDNotFound(t *testing.T) {
	repo, _ := newDeliveryFixture(t)
	if _, err := repo.ByID(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("ByID(absent) = %v; want ErrNotFound", err)
	}
}
