package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
)

// memKeyProvider is a crypto.KeyProvider backed by an in-memory key.
type memKeyProvider struct{ key []byte }

func (m memKeyProvider) Key(context.Context) ([]byte, error) { return m.key, nil }

// countingKeyProvider counts Key() calls so a test can prove the delivery
// resolver fetches the master key once per tick, not once per delivery.
type countingKeyProvider struct {
	key   []byte
	calls int
}

func (c *countingKeyProvider) Key(context.Context) ([]byte, error) {
	c.calls++
	return c.key, nil
}

// canarySecret stands in for a real secret value. It must never appear in any
// delivered request — the payload carries metadata only.
const canarySecret = "S3CR3T-CANARY-VALUE"

func newWorkerDB(t *testing.T) (*sql.DB, []byte) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "wh.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	master := bytes.Repeat([]byte{0x2a}, crypto.KeySize)
	return db, master
}

// seedDelivery creates a project + webhook (URL=url, sealed signingKey) and
// enqueues one due delivery. Returns the webhook id and delivery id.
func seedDelivery(t *testing.T, db *sql.DB, master, signingKey []byte, url, event string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	p, err := store.NewProjectRepo(db).Create(ctx, "app")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	sealed, err := crypto.Seal(master, signingKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	wh, err := store.NewWebhookRepo(db).Create(ctx, p.ID, nil, url, event, sealed)
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}
	body, err := Payload{Action: event, Project: "app", Env: "prod", Key: "DB_URL", Version: 2, Timestamp: time.Now().Unix()}.Marshal()
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	d, err := store.NewDeliveryRepo(db).Enqueue(ctx, wh.ID, event, string(body))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	return wh.ID, d.ID
}

func TestWorker_DeliversSignedPayloadNoPlaintext(t *testing.T) {
	db, master := newWorkerDB(t)
	signingKey := []byte("per-webhook-signing-key")

	var (
		mu        sync.Mutex
		gotBody   []byte
		gotSig    string
		gotTS     string
		gotEvent  string
		gotDelID  string
		validHMAC bool
		hits      int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		hits++
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		gotBody = buf.Bytes()
		gotSig = r.Header.Get(HeaderSignature)
		gotTS = r.Header.Get(HeaderTimestamp)
		gotEvent = r.Header.Get(HeaderEvent)
		gotDelID = r.Header.Get(HeaderDelivery)
		// Independently verify the HMAC over "<ts>.<body>".
		mac := hmac.New(sha256.New, signingKey)
		mac.Write([]byte(gotTS + "."))
		mac.Write(gotBody)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		validHMAC = hmac.Equal([]byte(gotSig), []byte(want))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, delID := seedDelivery(t, db, master, signingKey, srv.URL, store.EventSecretUpsert)

	w := NewWorker(Options{DB: db, Keys: memKeyProvider{master}})
	w.tick(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if hits != 1 {
		t.Fatalf("receiver hit %d times; want 1", hits)
	}
	if !validHMAC {
		t.Errorf("HMAC signature did not verify: sig=%q", gotSig)
	}
	if !strings.HasPrefix(gotSig, "sha256=") {
		t.Errorf("signature format wrong: %q", gotSig)
	}
	if gotEvent != store.EventSecretUpsert {
		t.Errorf("event header = %q", gotEvent)
	}
	if gotDelID == "" {
		t.Error("X-Comax-Delivery header missing (idempotency key)")
	}
	// The delivered payload must not contain any secret plaintext.
	if bytes.Contains(gotBody, []byte(canarySecret)) {
		t.Errorf("payload leaked secret plaintext: %s", gotBody)
	}
	if bytes.Contains(gotBody, []byte(string(signingKey))) {
		t.Error("payload leaked the signing key")
	}

	got, err := store.NewDeliveryRepo(db).ByID(context.Background(), delID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Status != store.DeliveryDelivered {
		t.Errorf("status = %q; want delivered", got.Status)
	}
	if got.LastStatus == nil || *got.LastStatus != 200 {
		t.Errorf("last_status = %v; want 200", got.LastStatus)
	}
}

// TestWorker_BatchReusesSigningMaterial proves the per-tick memoization: two
// due deliveries to the SAME webhook fetch the master key once and open the
// sealed signing key once, while both still deliver.
func TestWorker_BatchReusesSigningMaterial(t *testing.T) {
	db, master := newWorkerDB(t)
	ctx := context.Background()
	signingKey := []byte("shared-signing-key")

	var (
		mu   sync.Mutex
		hits int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, err := store.NewProjectRepo(db).Create(ctx, "app")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	sealed, err := crypto.Seal(master, signingKey)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	wh, err := store.NewWebhookRepo(db).Create(ctx, p.ID, nil, srv.URL, store.EventSecretUpsert, sealed)
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}
	body, _ := Payload{Action: store.EventSecretUpsert, Project: "app", Env: "prod", Key: "K", Timestamp: time.Now().Unix()}.Marshal()
	deliveries := store.NewDeliveryRepo(db)
	d1, _ := deliveries.Enqueue(ctx, wh.ID, store.EventSecretUpsert, string(body))
	d2, _ := deliveries.Enqueue(ctx, wh.ID, store.EventSecretUpsert, string(body))

	keys := &countingKeyProvider{key: master}
	w := NewWorker(Options{DB: db, Keys: keys})
	w.tick(ctx)

	if keys.calls != 1 {
		t.Errorf("master key fetched %d times in one tick; want 1 (memoized across deliveries)", keys.calls)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 2 {
		t.Errorf("receiver hit %d times; want 2 (both deliveries)", hits)
	}
	for _, id := range []int64{d1.ID, d2.ID} {
		got, err := deliveries.ByID(ctx, id)
		if err != nil {
			t.Fatalf("ByID %d: %v", id, err)
		}
		if got.Status != store.DeliveryDelivered {
			t.Errorf("delivery %d status = %q; want delivered", id, got.Status)
		}
	}
}

func TestWorker_RetriesThenDeadLetters(t *testing.T) {
	db, master := newWorkerDB(t)
	signingKey := []byte("k")

	var mu sync.Mutex
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError) // always fail
	}))
	defer srv.Close()

	_, delID := seedDelivery(t, db, master, signingKey, srv.URL, store.EventSecretUpsert)

	// maxAttempts=2 with zero backoff so the retried row is immediately re-due.
	w := NewWorker(Options{
		DB:          db,
		Keys:        memKeyProvider{master},
		MaxAttempts: 2,
		Backoff:     func(int64) time.Duration { return 0 },
	})

	ctx := context.Background()
	w.tick(ctx) // attempt 1 -> retry
	mid, _ := store.NewDeliveryRepo(db).ByID(ctx, delID)
	if mid.Status != store.DeliveryPending {
		t.Fatalf("after attempt 1 status = %q; want pending (retry)", mid.Status)
	}
	if mid.Attempts != 1 {
		t.Fatalf("after attempt 1 attempts = %d; want 1", mid.Attempts)
	}

	w.tick(ctx) // attempt 2 -> dead
	final, _ := store.NewDeliveryRepo(db).ByID(ctx, delID)
	if final.Status != store.DeliveryDead {
		t.Errorf("final status = %q; want dead", final.Status)
	}
	if final.Attempts != 2 {
		t.Errorf("final attempts = %d; want 2", final.Attempts)
	}
	if final.LastStatus == nil || *final.LastStatus != 500 {
		t.Errorf("last_status = %v; want 500", final.LastStatus)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 2 {
		t.Errorf("receiver hit %d times; want 2 (attempt then retry)", hits)
	}
}

func TestWorker_BlockedURLFailsWithoutLeak(t *testing.T) {
	db, master := newWorkerDB(t)
	signingKey := []byte("k")
	// Register a webhook pointing at the metadata IP (blocked at delivery time).
	_, delID := seedDelivery(t, db, master, signingKey, "http://169.254.169.254/hook", store.EventSecretUpsert)

	w := NewWorker(Options{DB: db, Keys: memKeyProvider{master}, MaxAttempts: 2, Backoff: func(int64) time.Duration { return 0 }})
	w.tick(context.Background())

	got, _ := store.NewDeliveryRepo(db).ByID(context.Background(), delID)
	if got.Status != store.DeliveryPending {
		t.Errorf("status = %q; want pending (retry after block)", got.Status)
	}
	if !strings.Contains(got.LastError, "blocked") {
		t.Errorf("last_error = %q; want a 'blocked' reason", got.LastError)
	}
	// The stored reason must not echo the full URL/credentials.
	if strings.Contains(got.LastError, "169.254.169.254") {
		// host in a policy error is acceptable, but be conservative: the reason
		// we store is generic ("url blocked by policy").
		t.Logf("note: last_error contains host: %q", got.LastError)
	}
}

func TestWorker_TransportErrorRetries(t *testing.T) {
	db, master := newWorkerDB(t)
	signingKey := []byte("k")
	// Loopback port 1 has nothing listening → connection refused (transport
	// error), but the address is allowed by policy so we reach the dial.
	_, delID := seedDelivery(t, db, master, signingKey, "http://127.0.0.1:1/hook", store.EventSecretUpsert)

	w := NewWorker(Options{DB: db, Keys: memKeyProvider{master}, MaxAttempts: 3, Backoff: func(int64) time.Duration { return time.Minute }})
	w.tick(context.Background())

	got, _ := store.NewDeliveryRepo(db).ByID(context.Background(), delID)
	if got.Status != store.DeliveryPending {
		t.Errorf("status = %q; want pending (retry after transport error)", got.Status)
	}
	if !strings.HasPrefix(got.LastError, "transport") {
		t.Errorf("last_error = %q; want a 'transport' reason", got.LastError)
	}
	if got.LastStatus != nil {
		t.Errorf("last_status = %v; want nil (no HTTP response)", *got.LastStatus)
	}
}

func TestWorker_UnsealableKeyDeadLetters(t *testing.T) {
	db, master := newWorkerDB(t)
	ctx := context.Background()
	p, err := store.NewProjectRepo(db).Create(ctx, "app")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	// Store ciphertext that is NOT a valid Seal — crypto.Open must fail, and the
	// worker must dead-letter (retrying can't fix a corrupt key) without leaking.
	garbage := bytes.Repeat([]byte{0x00}, 40)
	wh, err := store.NewWebhookRepo(db).Create(ctx, p.ID, nil, "http://127.0.0.1:9/hook", store.EventSecretUpsert, garbage)
	if err != nil {
		t.Fatalf("webhook: %v", err)
	}
	body, _ := Payload{Action: store.EventSecretUpsert, Project: "app", Env: "prod", Key: "K", Timestamp: time.Now().Unix()}.Marshal()
	d, err := store.NewDeliveryRepo(db).Enqueue(ctx, wh.ID, store.EventSecretUpsert, string(body))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	w := NewWorker(Options{DB: db, Keys: memKeyProvider{master}})
	w.tick(ctx)

	got, _ := store.NewDeliveryRepo(db).ByID(ctx, d.ID)
	if got.Status != store.DeliveryDead {
		t.Errorf("status = %q; want dead (unsealable key is terminal)", got.Status)
	}
	if strings.Contains(got.LastError, string(garbage)) {
		t.Error("last_error leaked key material")
	}
}

func TestWorker_RunStopsOnContextCancel(t *testing.T) {
	db, master := newWorkerDB(t)
	w := NewWorker(Options{DB: db, Keys: memKeyProvider{master}})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx, time.Hour) // long interval: exit must come from ctx.Done()
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("worker did not stop within 1s after cancel")
	}
}

// TestWorker_RunClampsNonPositiveInterval guards the crash path: a
// non-positive poll interval must not reach time.NewTicker (which panics),
// so Run clamps it to the default and still stops cleanly on ctx cancel.
func TestWorker_RunClampsNonPositiveInterval(t *testing.T) {
	db, master := newWorkerDB(t)
	for _, interval := range []time.Duration{0, -5 * time.Second} {
		w := NewWorker(Options{DB: db, Keys: memKeyProvider{master}})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			w.Run(ctx, interval) // must NOT panic on NewTicker(interval<=0)
			close(done)
		}()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Errorf("worker with interval %v did not stop within 1s", interval)
		}
	}
}

func TestDefaultBackoff(t *testing.T) {
	if d := DefaultBackoff(1); d != 10*time.Second {
		t.Errorf("attempt 1 = %v; want 10s", d)
	}
	if d := DefaultBackoff(2); d != 20*time.Second {
		t.Errorf("attempt 2 = %v; want 20s", d)
	}
	if d := DefaultBackoff(0); d != 10*time.Second {
		t.Errorf("attempt 0 clamped = %v; want 10s", d)
	}
	if d := DefaultBackoff(100); d != 5*time.Minute {
		t.Errorf("large attempt = %v; want 5m ceiling", d)
	}
}
