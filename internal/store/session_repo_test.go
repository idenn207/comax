package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

// makeHashes makes deterministic (session, csrf) hashes for one test
// case. SHA-256 over the seed strings is good enough — the repo never
// inspects the bytes beyond byte-equality.
func makeHashes(seed string) (sessionHash, csrfHash []byte) {
	s := sha256.Sum256([]byte("session:" + seed))
	c := sha256.Sum256([]byte("csrf:" + seed))
	return s[:], c[:]
}

func mustSeedToken(t *testing.T, db DBTX, name string) ServiceToken {
	t.Helper()
	hash := sha256.Sum256([]byte("bearer:" + name))
	tok, err := NewTokenRepo(db).Create(context.Background(), name, hash[:], false)
	if err != nil {
		t.Fatalf("seed token: %v", err)
	}
	return tok
}

func TestSessionRepo_CreateRoundTrip(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	sh, ch := makeHashes("a")

	repo := NewSessionRepo(db)
	sess, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID:     tok.ID,
		SessionHash: sh,
		CSRFHash:    ch,
		UserAgent:   "ua/1.0",
		IPPrefix:    "10.0.0.0/24",
		TTL:         time.Hour,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.ID == 0 || sess.TokenID != tok.ID {
		t.Errorf("returned session = %+v", sess)
	}
	if !bytes.Equal(sess.SessionHash, sh) || !bytes.Equal(sess.CSRFHash, ch) {
		t.Errorf("hashes did not round-trip")
	}
	if sess.UserAgent != "ua/1.0" || sess.IPPrefix != "10.0.0.0/24" {
		t.Errorf("metadata did not round-trip: %+v", sess)
	}
	if !sess.ExpiresAt.After(sess.CreatedAt) {
		t.Errorf("expires_at=%v not after created_at=%v", sess.ExpiresAt, sess.CreatedAt)
	}
}

func TestSessionRepo_ByHashRetrievesActiveOnly(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	sh, ch := makeHashes("active")

	repo := NewSessionRepo(db)
	created, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: sh, CSRFHash: ch, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.ByHash(context.Background(), sh)
	if err != nil {
		t.Fatalf("ByHash: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ByHash id=%d; want %d", got.ID, created.ID)
	}
}

func TestSessionRepo_ByHashSkipsRevoked(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	sh, ch := makeHashes("revoked")

	repo := NewSessionRepo(db)
	created, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: sh, CSRFHash: ch, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Revoke(context.Background(), created.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := repo.ByHash(context.Background(), sh); !errors.Is(err, ErrNotFound) {
		t.Errorf("ByHash after Revoke err=%v; want ErrNotFound", err)
	}
}

func TestSessionRepo_ByHashSkipsExpired(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	sh, ch := makeHashes("expired")

	repo := NewSessionRepo(db)
	if _, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID:     tok.ID,
		SessionHash: sh,
		CSRFHash:    ch,
		TTL:         -time.Hour, // already past
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.ByHash(context.Background(), sh); !errors.Is(err, ErrNotFound) {
		t.Errorf("ByHash on expired err=%v; want ErrNotFound", err)
	}
}

func TestSessionRepo_RevokeUnknownIs404(t *testing.T) {
	db := newTestDB(t)
	if err := NewSessionRepo(db).Revoke(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("Revoke(unknown) err=%v; want ErrNotFound", err)
	}
}

func TestSessionRepo_CreateRejectsDuplicateHash(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	sh, ch := makeHashes("dup")

	repo := NewSessionRepo(db)
	if _, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: sh, CSRFHash: ch, TTL: time.Hour,
	}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: sh, CSRFHash: ch, TTL: time.Hour,
	})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate Create err=%v; want ErrConflict", err)
	}
}

func TestSessionRepo_ListByTokenIDIsolatesOwner(t *testing.T) {
	db := newTestDB(t)
	owner := mustSeedToken(t, db, "owner")
	other := mustSeedToken(t, db, "other")
	repo := NewSessionRepo(db)

	for _, seed := range []string{"o-a", "o-b"} {
		sh, ch := makeHashes(seed)
		if _, err := repo.Create(context.Background(), SessionCreateInput{
			TokenID: owner.ID, SessionHash: sh, CSRFHash: ch, TTL: time.Hour,
		}); err != nil {
			t.Fatalf("Create owner %s: %v", seed, err)
		}
	}
	sh, ch := makeHashes("x-a")
	if _, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: other.ID, SessionHash: sh, CSRFHash: ch, TTL: time.Hour,
	}); err != nil {
		t.Fatalf("Create other: %v", err)
	}

	got, err := repo.ListByTokenID(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("ListByTokenID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got)=%d; want 2", len(got))
	}
	for _, s := range got {
		if s.TokenID != owner.ID {
			t.Errorf("foreign row leaked: token_id=%d session_id=%d", s.TokenID, s.ID)
		}
	}
}

func TestSessionRepo_ListByTokenIDExcludesRevokedAndExpired(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	repo := NewSessionRepo(db)

	live := makeHashesPair("live")
	if _, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: live.s, CSRFHash: live.c, TTL: time.Hour,
	}); err != nil {
		t.Fatalf("Create live: %v", err)
	}

	revoked := makeHashesPair("revoked")
	rv, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: revoked.s, CSRFHash: revoked.c, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("Create revoked: %v", err)
	}
	if err := repo.Revoke(context.Background(), rv.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	expired := makeHashesPair("expired")
	if _, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: expired.s, CSRFHash: expired.c, TTL: -time.Hour,
	}); err != nil {
		t.Fatalf("Create expired: %v", err)
	}

	got, err := repo.ListByTokenID(context.Background(), tok.ID)
	if err != nil {
		t.Fatalf("ListByTokenID: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got)=%d; want 1 (only the live row)", len(got))
	}
	if !bytes.Equal(got[0].SessionHash, live.s) {
		t.Errorf("returned row is not the live row")
	}
}

func TestSessionRepo_RevokeByIDAndTokenIDOwnerOnly(t *testing.T) {
	db := newTestDB(t)
	owner := mustSeedToken(t, db, "owner")
	other := mustSeedToken(t, db, "other")
	repo := NewSessionRepo(db)

	mine := makeHashesPair("mine")
	mineSess, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: owner.ID, SessionHash: mine.s, CSRFHash: mine.c, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("Create mine: %v", err)
	}

	theirs := makeHashesPair("theirs")
	theirsSess, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: other.ID, SessionHash: theirs.s, CSRFHash: theirs.c, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("Create theirs: %v", err)
	}

	if n, err := repo.RevokeByIDAndTokenID(context.Background(), mineSess.ID, owner.ID); err != nil || n != 1 {
		t.Fatalf("revoke own n=%d err=%v; want 1/nil", n, err)
	}

	// Owner tries other's id: affected == 0, and the foreign row stays live.
	if n, err := repo.RevokeByIDAndTokenID(context.Background(), theirsSess.ID, owner.ID); err != nil || n != 0 {
		t.Errorf("cross-token revoke n=%d err=%v; want 0/nil (no oracle)", n, err)
	}
	survivor, err := repo.ByHash(context.Background(), theirs.s)
	if err != nil {
		t.Fatalf("foreign row vanished: %v", err)
	}
	if survivor.RevokedAt != nil {
		t.Errorf("cross-token attempt mutated foreign row: revoked_at=%v", *survivor.RevokedAt)
	}
}

func TestSessionRepo_RevokeByIDAndTokenIDIdempotent(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	repo := NewSessionRepo(db)

	pair := makeHashesPair("idem")
	sess, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: pair.s, CSRFHash: pair.c, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if n, err := repo.RevokeByIDAndTokenID(context.Background(), sess.ID, tok.ID); err != nil || n != 1 {
		t.Fatalf("first revoke n=%d err=%v; want 1/nil", n, err)
	}
	// Already revoked: 0 rows — no error, no oracle.
	if n, err := repo.RevokeByIDAndTokenID(context.Background(), sess.ID, tok.ID); err != nil || n != 0 {
		t.Errorf("re-revoke n=%d err=%v; want 0/nil (idempotent)", n, err)
	}
	// Unknown id: 0 rows — same shape as cross-token / already revoked.
	if n, err := repo.RevokeByIDAndTokenID(context.Background(), 999_999, tok.ID); err != nil || n != 0 {
		t.Errorf("unknown id revoke n=%d err=%v; want 0/nil", n, err)
	}
}

type hashPair struct{ s, c []byte }

func makeHashesPair(seed string) hashPair {
	s, c := makeHashes(seed)
	return hashPair{s: s, c: c}
}

func TestSessionRepo_Prune(t *testing.T) {
	db := newTestDB(t)
	tok := mustSeedToken(t, db, "dash")
	repo := NewSessionRepo(db)

	// One revoked-long-ago row, one active row.
	sh1, ch1 := makeHashes("p1")
	created, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: sh1, CSRFHash: ch1, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("Create p1: %v", err)
	}
	if err := repo.Revoke(context.Background(), created.ID); err != nil {
		t.Fatalf("Revoke p1: %v", err)
	}

	sh2, ch2 := makeHashes("p2")
	if _, err := repo.Create(context.Background(), SessionCreateInput{
		TokenID: tok.ID, SessionHash: sh2, CSRFHash: ch2, TTL: time.Hour,
	}); err != nil {
		t.Fatalf("Create p2: %v", err)
	}

	// Use a future cutoff so the revoked-now row qualifies for deletion.
	n, err := repo.Prune(context.Background(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("pruned=%d; want 1 (revoked only)", n)
	}
	// Active row is still reachable.
	if _, err := repo.ByHash(context.Background(), sh2); err != nil {
		t.Errorf("active row gone after prune: %v", err)
	}
}
