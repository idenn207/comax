package auth

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/idenn207/comax-secrets/internal/store"
)

// openTestDB returns a fresh on-disk SQLite handle with the schema
// migrated. Mirrors the store package helper so the auth tests stay
// self-contained.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("store.Migrate: %v", err)
	}
	return db
}

func TestBootstrap_FirstCallIssuesToken(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	res, err := Bootstrap(ctx, db)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if res.Plaintext == "" {
		t.Error("Plaintext empty; operator can never login")
	}
	if res.Token.ID == 0 {
		t.Error("Token.ID is zero")
	}
	if res.Token.Name != BootstrapTokenName {
		t.Errorf("Token.Name = %q; want %q", res.Token.Name, BootstrapTokenName)
	}

	// The plaintext must verify as a valid bearer for that token. This
	// guards against a refactor that drifts hashing between Bootstrap
	// and Verify.
	tok, err := Verify(ctx, store.NewTokenRepo(db), res.Plaintext)
	if err != nil {
		t.Fatalf("Verify on freshly issued token: %v", err)
	}
	if tok.ID != res.Token.ID {
		t.Errorf("Verify returned id=%d; want %d", tok.ID, res.Token.ID)
	}
}

func TestBootstrap_SecondCallReturnsErrAlreadyBootstrapped(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := Bootstrap(ctx, db); err != nil {
		t.Fatalf("first Bootstrap: %v", err)
	}
	if _, err := Bootstrap(ctx, db); !errors.Is(err, ErrAlreadyBootstrapped) {
		t.Errorf("second Bootstrap err = %v; want %v", err, ErrAlreadyBootstrapped)
	}

	// Database state must show exactly one row.
	n, err := store.NewTokenRepo(db).Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Errorf("Count = %d; want 1 after two Bootstrap calls", n)
	}
}

func TestBootstrap_WritesAuditRow(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	res, err := Bootstrap(ctx, db)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	entries, err := store.NewAuditRepo(db).ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d; want 1", len(entries))
	}
	e := entries[0]
	if e.Action != "auth.bootstrap" {
		t.Errorf("audit.Action = %q; want auth.bootstrap", e.Action)
	}
	if e.ActorToken == nil || *e.ActorToken != res.Token.ID {
		t.Errorf("audit.ActorToken = %v; want %d", e.ActorToken, res.Token.ID)
	}
}

// TestBootstrap_ConcurrentCallsExactlyOneWins exercises the race-safety
// claim: even with N callers hitting Bootstrap at once on an empty DB,
// exactly one returns success and the rest return ErrAlreadyBootstrapped.
//
// Two ingredients make this deterministic:
//
//   - store.Open configures busy_timeout(5000), so the SQLite single-writer
//     lock is *waited on* instead of bouncing with SQLITE_BUSY. The
//     contending writers therefore actually get to run their transaction.
//   - TokenRepo.BootstrapIfEmpty's WHERE (SELECT COUNT(*) FROM
//     service_tokens) = 0 re-evaluates against the committed state, so
//     once one writer wins the others see count=1 and insert zero rows.
func TestBootstrap_ConcurrentCallsExactlyOneWins(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	const concurrency = 8
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		successes   int
		alreadyErrs int
		otherErrs   []error
	)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Bootstrap(ctx, db)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrAlreadyBootstrapped):
				alreadyErrs++
			default:
				otherErrs = append(otherErrs, err)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("successes = %d; want exactly 1", successes)
	}
	if alreadyErrs+successes != concurrency {
		t.Errorf("alreadyErrs+successes = %d; want %d (with %d unexpected errors)",
			alreadyErrs+successes, concurrency, len(otherErrs))
		for _, e := range otherErrs {
			t.Errorf("unexpected error: %v", e)
		}
	}
	if n, _ := store.NewTokenRepo(db).Count(ctx); n != 1 {
		t.Errorf("post-race Count = %d; want exactly 1", n)
	}
}
