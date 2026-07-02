package webhook

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
)

// Default worker tuning. maxAttempts counts total POST attempts before a
// delivery is declared dead; staleAfter is how long an in_progress lease may
// sit before ReclaimStale treats the owning worker as crashed.
const (
	DefaultMaxAttempts = 5
	DefaultBatchSize   = 50
	DefaultStaleAfter  = 2 * time.Minute
	// DefaultPollInterval is the outbox poll cadence used when the caller
	// passes a non-positive interval to Run (and the cmd/server default).
	DefaultPollInterval = 10 * time.Second
	// maxResponseDrain bounds how much of a receiver's response body we read
	// (to reuse the keep-alive connection) — we never need the content.
	maxResponseDrain = 4 << 10
)

// Worker drains the webhook_deliveries outbox: each tick it reclaims stale
// leases, claims due rows atomically, signs each payload with the webhook's
// sealed HMAC key, and POSTs through the SSRF-hardened client. Success marks
// the row delivered; failure schedules a backoff retry or, past maxAttempts,
// dead-letters it.
type Worker struct {
	db          *sql.DB
	keys        crypto.KeyProvider
	client      *http.Client
	policy      Policy
	maxAttempts int
	batchSize   int
	staleAfter  time.Duration
	backoff     func(attempt int64) time.Duration
	logger      *slog.Logger
}

// Options configures NewWorker. DB, Keys are required; the rest default.
type Options struct {
	DB          *sql.DB
	Keys        crypto.KeyProvider
	Policy      Policy
	Client      *http.Client // defaults to SafeClient(Policy, 10s)
	MaxAttempts int
	BatchSize   int
	StaleAfter  time.Duration
	Backoff     func(attempt int64) time.Duration
	Logger      *slog.Logger
}

// NewWorker assembles a Worker, filling in defaults for anything the caller
// left zero. The default client is a SafeClient bound to the same policy, so
// delivery is SSRF-hardened even if the caller passes only DB + Keys.
func NewWorker(opts Options) *Worker {
	w := &Worker{
		db:          opts.DB,
		keys:        opts.Keys,
		client:      opts.Client,
		policy:      opts.Policy,
		maxAttempts: opts.MaxAttempts,
		batchSize:   opts.BatchSize,
		staleAfter:  opts.StaleAfter,
		backoff:     opts.Backoff,
		logger:      opts.Logger,
	}
	if w.client == nil {
		w.client = SafeClient(opts.Policy, 10*time.Second)
	}
	if w.maxAttempts <= 0 {
		w.maxAttempts = DefaultMaxAttempts
	}
	if w.batchSize <= 0 {
		w.batchSize = DefaultBatchSize
	}
	if w.staleAfter <= 0 {
		w.staleAfter = DefaultStaleAfter
	}
	if w.backoff == nil {
		w.backoff = DefaultBackoff
	}
	if w.logger == nil {
		w.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return w
}

// DefaultBackoff is exponential with a 5-minute ceiling: attempt 1 -> 10s,
// 2 -> 20s, 3 -> 40s, ... capped at 5m.
func DefaultBackoff(attempt int64) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := 10 * time.Second
	for i := int64(1); i < attempt; i++ {
		d *= 2
		if d >= 5*time.Minute {
			return 5 * time.Minute
		}
	}
	return d
}

// Run drives the worker until ctx is cancelled, mirroring the server's prune
// sweeper: a ticker fires each interval, ctx.Done stops the loop, and a
// transient error only logs (the next tick retries). This is launched as
// `go worker.Run(ctx, interval)` from the server lifecycle.
func (w *Worker) Run(ctx context.Context, interval time.Duration) {
	// time.NewTicker panics on a non-positive interval, so a misconfigured
	// --webhook-poll-interval 0 / COMAX_WEBHOOK_POLL=0s would crash the worker
	// goroutine (and the process). Clamp to the default and warn instead.
	if interval <= 0 {
		w.logger.Warn("webhook poll interval non-positive; using default",
			slog.Duration("requested", interval), slog.Duration("default", DefaultPollInterval))
		interval = DefaultPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick runs one sweep: reclaim stale leases, claim due rows, deliver each.
// Exported-ish via Run; separated so tests can drive a single sweep
// deterministically without spinning the ticker.
func (w *Worker) tick(ctx context.Context) {
	deliveries := store.NewDeliveryRepo(w.db)

	if n, err := deliveries.ReclaimStale(ctx, time.Now().Add(-w.staleAfter)); err != nil {
		w.logger.Warn("reclaim stale deliveries failed", slog.String("err", err.Error()))
	} else if n > 0 {
		w.logger.Info("reclaimed stale deliveries", slog.Int64("rows", n))
	}

	claimed, err := deliveries.ClaimDue(ctx, time.Now(), w.batchSize)
	if err != nil {
		w.logger.Warn("claim due deliveries failed", slog.String("err", err.Error()))
		return
	}
	// One resolver per tick: the master key is fetched once and each webhook's
	// row + opened signing key is memoized, so a batch of deliveries to the
	// same webhook resolves the signing material once instead of per delivery.
	rz := newDeliveryResolver(w.db, w.keys)
	for _, d := range claimed {
		w.deliverOne(ctx, deliveries, rz, d)
	}
}

// signingMaterial is a webhook's row plus its opened (plaintext) HMAC signing
// key, cached per tick by deliveryResolver.
type signingMaterial struct {
	wh  store.Webhook
	key []byte
}

// Resolution failures, matched by deliverOne to pick the right delivery
// outcome. errWebhookGone and errKeyUnavailable are transient (retry);
// errKeyUnsealable is terminal (dead-letter — a corrupt key never heals).
var (
	errWebhookGone    = errors.New("webhook not found")
	errKeyUnavailable = errors.New("signing key unavailable")
	errKeyUnsealable  = errors.New("signing key unsealable")
)

// deliveryResolver memoizes per-tick signing material. The master key is loaded
// at most once; each webhook's (row, opened key) is cached by id. Only
// successful resolutions are cached — the rare error paths re-derive so their
// per-delivery outcome (retry vs dead-letter) stays intact.
type deliveryResolver struct {
	db           *sql.DB
	keys         crypto.KeyProvider
	master       []byte
	masterLoaded bool
	hooks        map[int64]signingMaterial
}

func newDeliveryResolver(db *sql.DB, keys crypto.KeyProvider) *deliveryResolver {
	return &deliveryResolver{db: db, keys: keys, hooks: make(map[int64]signingMaterial)}
}

// masterKey returns the master key, fetching it once per tick.
func (rz *deliveryResolver) masterKey(ctx context.Context) ([]byte, error) {
	if rz.masterLoaded {
		return rz.master, nil
	}
	m, err := rz.keys.Key(ctx)
	if err != nil {
		return nil, err
	}
	rz.master, rz.masterLoaded = m, true
	return m, nil
}

// resolve returns the signing material for webhookID, memoizing the webhook row
// and opened key on success. It wraps failures in the sentinel that tells
// deliverOne how to record the outcome.
func (rz *deliveryResolver) resolve(ctx context.Context, webhookID int64) (signingMaterial, error) {
	if sm, ok := rz.hooks[webhookID]; ok {
		return sm, nil
	}
	wh, err := store.NewWebhookRepo(rz.db).ByID(ctx, webhookID)
	if err != nil {
		return signingMaterial{}, fmt.Errorf("%w: %v", errWebhookGone, err)
	}
	master, err := rz.masterKey(ctx)
	if err != nil {
		return signingMaterial{}, fmt.Errorf("%w: %v", errKeyUnavailable, err)
	}
	key, err := crypto.Open(master, wh.SecretCiphertext)
	if err != nil {
		return signingMaterial{}, fmt.Errorf("%w: %v", errKeyUnsealable, err)
	}
	sm := signingMaterial{wh: wh, key: key}
	rz.hooks[webhookID] = sm
	return sm, nil
}

// deliverOne signs and POSTs a single claimed delivery, then records the
// outcome. It never logs the signing key or the raw URL (which may carry
// userinfo) — only ids, the event, and the status/reason.
func (w *Worker) deliverOne(ctx context.Context, deliveries *store.DeliveryRepo, rz *deliveryResolver, d store.WebhookDelivery) {
	sm, err := rz.resolve(ctx, d.WebhookID)
	switch {
	case errors.Is(err, errWebhookGone):
		// The webhook was deleted after enqueue (its cascade normally removes
		// the delivery too; this is the belt-and-suspenders path).
		w.fail(ctx, deliveries, d, 0, "webhook not found")
		return
	case errors.Is(err, errKeyUnavailable):
		// Key temporarily unavailable — transient, retry.
		w.logger.Warn("delivery key unavailable", slog.Int64("delivery", d.ID), slog.String("err", err.Error()))
		w.fail(ctx, deliveries, d, 0, "signing key unavailable")
		return
	case errors.Is(err, errKeyUnsealable):
		// Ciphertext can't be opened (corrupt / wrong key). Retrying won't help
		// — dead-letter with a non-leaking reason (never echo key material).
		w.logger.Error("delivery signing key unsealable", slog.Int64("delivery", d.ID))
		if err := deliveries.MarkDead(ctx, d.ID, 0, "signing key unsealable"); err != nil {
			w.logger.Warn("mark dead failed", slog.Int64("delivery", d.ID), slog.String("err", err.Error()))
		}
		return
	}

	// Re-validate at delivery time: DNS may have changed since registration.
	if err := ValidateURL(ctx, sm.wh.URL, w.policy); err != nil {
		w.logger.Warn("delivery url blocked by policy", slog.Int64("delivery", d.ID), slog.String("err", err.Error()))
		w.fail(ctx, deliveries, d, 0, "url blocked by policy")
		return
	}

	ts := time.Now().Unix()
	body := []byte(d.Payload)
	sig, tsHeader := Sign(sm.key, body, ts)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sm.wh.URL, bytes.NewReader(body))
	if err != nil {
		w.fail(ctx, deliveries, d, 0, "request build failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "comax-webhook/1")
	req.Header.Set(HeaderEvent, d.Event)
	req.Header.Set(HeaderDelivery, strconv.FormatInt(d.ID, 10))
	req.Header.Set(HeaderSignature, sig)
	req.Header.Set(HeaderTimestamp, tsHeader)

	resp, err := w.client.Do(req)
	if err != nil {
		w.fail(ctx, deliveries, d, 0, transportReason(err))
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseDrain))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := deliveries.MarkDelivered(ctx, d.ID, resp.StatusCode); err != nil {
			w.logger.Warn("mark delivered failed", slog.Int64("delivery", d.ID), slog.String("err", err.Error()))
		}
		return
	}
	w.fail(ctx, deliveries, d, resp.StatusCode, fmt.Sprintf("HTTP %d", resp.StatusCode))
}

// fail advances a failed attempt: dead-letter once maxAttempts is reached,
// otherwise schedule a backoff retry. d.Attempts is the pre-attempt count, so
// the attempt just made is d.Attempts+1.
func (w *Worker) fail(ctx context.Context, deliveries *store.DeliveryRepo, d store.WebhookDelivery, status int, reason string) {
	attempt := d.Attempts + 1
	if attempt >= int64(w.maxAttempts) {
		if err := deliveries.MarkDead(ctx, d.ID, status, reason); err != nil {
			w.logger.Warn("mark dead failed", slog.Int64("delivery", d.ID), slog.String("err", err.Error()))
			return
		}
		w.logger.Warn("delivery dead-lettered",
			slog.Int64("delivery", d.ID), slog.Int64("attempts", attempt), slog.String("reason", reason))
		return
	}
	next := time.Now().Add(w.backoff(attempt))
	if err := deliveries.MarkRetry(ctx, d.ID, next, status, reason); err != nil {
		w.logger.Warn("mark retry failed", slog.Int64("delivery", d.ID), slog.String("err", err.Error()))
	}
}

// transportReason extracts a non-leaking failure reason from a transport
// error. *url.Error.Error() embeds the request URL (which may carry userinfo
// credentials), so we unwrap to the underlying cause and never store the URL.
func transportReason(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		if errors.Is(ue.Err, ErrRedirect) {
			return "redirect refused"
		}
		return "transport: " + ue.Err.Error()
	}
	return "transport error"
}
