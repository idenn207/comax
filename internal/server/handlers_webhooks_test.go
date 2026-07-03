package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/idenn207/comax-secrets/internal/store"
	"github.com/idenn207/comax-secrets/internal/webhook"
)

// seedWebhookProject bootstraps + creates project "comax" with env "dev".
func seedWebhookProject(t *testing.T, ts *testServer) {
	t.Helper()
	ts.bootstrap(t)
	mustCreate(t, ts, "/api/v1/projects", map[string]string{"name": "comax"})
	mustCreate(t, ts, "/api/v1/projects/comax/envs", map[string]string{"name": "dev"})
}

// createWebhook POSTs a webhook and returns its id + the one-time signing secret.
func createWebhook(t *testing.T, ts *testServer, body map[string]any) (int64, string) {
	t.Helper()
	status, env := ts.do(t, http.MethodPost, "/api/v1/webhooks", body)
	if status != http.StatusCreated {
		t.Fatalf("create webhook: status=%d env=%+v", status, env)
	}
	m := env.Data.(map[string]any)
	secret, _ := m["signing_secret"].(string)
	if secret == "" {
		t.Fatalf("create webhook did not return signing_secret: %+v", m)
	}
	idf, _ := m["id"].(float64)
	return int64(idf), secret
}

func TestWebhooks_CreateExposesSigningSecretOnce(t *testing.T) {
	ts := newTestServer(t)
	seedWebhookProject(t, ts)

	id, secret := createWebhook(t, ts, map[string]any{
		"project": "comax", "env": "dev",
		"url": "http://10.1.2.3/hook", "events": []string{"secret.upsert"},
	})
	if id == 0 {
		t.Fatal("zero webhook id")
	}
	if len(secret) < 16 {
		t.Errorf("signing secret looks too short: %q", secret)
	}

	// The listing never re-exposes the secret (nor the ciphertext).
	_, env := ts.do(t, http.MethodGet, "/api/v1/webhooks", nil)
	list := env.Data.([]any)
	if len(list) != 1 {
		t.Fatalf("list len=%d; want 1", len(list))
	}
	row := list[0].(map[string]any)
	for _, forbidden := range []string{"signing_secret", "secret_ciphertext", "secret"} {
		if _, present := row[forbidden]; present {
			t.Errorf("listing leaked %q", forbidden)
		}
	}
	if row["url"].(string) != "http://10.1.2.3/hook" {
		t.Errorf("url = %v", row["url"])
	}
}

func TestWebhooks_NonAdminForbidden(t *testing.T) {
	ts := newTestServer(t)
	seedWebhookProject(t, ts)
	id, _ := createWebhook(t, ts, map[string]any{"project": "comax", "url": "http://10.0.0.1/h"})

	plain, _ := mustIssueToken(t, ts, "ci")
	ts.bearer = plain
	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/webhooks", map[string]any{"project": "comax", "url": "http://10.0.0.2/h"}},
		{http.MethodGet, "/api/v1/webhooks", nil},
		{http.MethodPatch, fmt.Sprintf("/api/v1/webhooks/%d", id), map[string]any{"enabled": false}},
		{http.MethodDelete, fmt.Sprintf("/api/v1/webhooks/%d", id), nil},
		{http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d/deliveries", id), nil},
	}
	for _, c := range cases {
		status, env := ts.do(t, c.method, c.path, c.body)
		if status != http.StatusForbidden {
			t.Errorf("%s %s: status=%d; want 403", c.method, c.path, status)
		}
		if env.Error == nil || env.Error.Code != "forbidden" {
			t.Errorf("%s %s: error=%+v; want forbidden", c.method, c.path, env.Error)
		}
	}
}

func TestWebhooks_CreateRejectsSSRFAndBadInput(t *testing.T) {
	ts := newTestServer(t)
	seedWebhookProject(t, ts)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"metadata IP", map[string]any{"project": "comax", "url": "http://169.254.169.254/latest"}},
		{"link-local", map[string]any{"project": "comax", "url": "http://169.254.1.2/h"}},
		{"bad scheme", map[string]any{"project": "comax", "url": "ftp://10.0.0.1/h"}},
		{"unknown event", map[string]any{"project": "comax", "url": "http://10.0.0.1/h", "events": []string{"secret.explode"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, env := ts.do(t, http.MethodPost, "/api/v1/webhooks", c.body)
			if status != http.StatusBadRequest {
				t.Errorf("status=%d; want 400 (env=%+v)", status, env)
			}
		})
	}
}

func TestWebhooks_UnknownProject404(t *testing.T) {
	ts := newTestServer(t)
	ts.bootstrap(t)
	status, _ := ts.do(t, http.MethodPost, "/api/v1/webhooks",
		map[string]any{"project": "ghost", "url": "http://10.0.0.1/h"})
	if status != http.StatusNotFound {
		t.Errorf("status=%d; want 404", status)
	}
}

func TestWebhooks_DeleteAndNotFound(t *testing.T) {
	ts := newTestServer(t)
	seedWebhookProject(t, ts)
	id, _ := createWebhook(t, ts, map[string]any{"project": "comax", "url": "http://10.0.0.1/h"})

	if status, _ := ts.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/webhooks/%d", id), nil); status != http.StatusNoContent {
		t.Errorf("delete status=%d; want 204", status)
	}
	if status, _ := ts.do(t, http.MethodDelete, fmt.Sprintf("/api/v1/webhooks/%d", id), nil); status != http.StatusNotFound {
		t.Errorf("second delete status=%d; want 404", status)
	}
}

// TestWebhooks_SetEnabledToggle proves the soft-disable path end-to-end: a
// disabled webhook is skipped by MatchForEvent (no new delivery enqueued),
// re-enabling restores delivery, and the listing reflects the flag. Missing
// body field is a 400; an unknown id is a 404.
func TestWebhooks_SetEnabledToggle(t *testing.T) {
	ts := newTestServer(t)
	seedWebhookProject(t, ts)
	id, _ := createWebhook(t, ts, map[string]any{
		"project": "comax", "env": "dev",
		"url": "http://10.1.2.3/hook", "events": []string{"secret.upsert"},
	})
	path := fmt.Sprintf("/api/v1/webhooks/%d", id)

	// Disable → 204, and the listing shows enabled=false.
	if status, env := ts.do(t, http.MethodPatch, path, map[string]any{"enabled": false}); status != http.StatusNoContent {
		t.Fatalf("disable status=%d env=%+v", status, env)
	}
	_, env := ts.do(t, http.MethodGet, "/api/v1/webhooks", nil)
	if row := env.Data.([]any)[0].(map[string]any); row["enabled"].(bool) {
		t.Error("listing shows enabled=true after disable")
	}

	// A subscribed upsert now enqueues NOTHING (disabled is skipped).
	if status, _ := ts.do(t, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/K",
		map[string]string{"value": "v1"}); status != http.StatusCreated {
		t.Fatalf("put while disabled: status=%d", status)
	}
	_, env = ts.do(t, http.MethodGet, path+"/deliveries", nil)
	if n := len(env.Data.([]any)); n != 0 {
		t.Errorf("deliveries while disabled = %d; want 0", n)
	}

	// Re-enable → a subsequent upsert enqueues one delivery. A fresh key keeps
	// this a create (201) rather than an update of K above.
	if status, _ := ts.do(t, http.MethodPatch, path, map[string]any{"enabled": true}); status != http.StatusNoContent {
		t.Fatalf("enable status=%d", status)
	}
	if status, _ := ts.do(t, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/K2",
		map[string]string{"value": "v2"}); status != http.StatusCreated {
		t.Fatalf("put after enable: status=%d", status)
	}
	_, env = ts.do(t, http.MethodGet, path+"/deliveries", nil)
	if n := len(env.Data.([]any)); n != 1 {
		t.Errorf("deliveries after re-enable = %d; want 1", n)
	}

	// Missing "enabled" field is a 400 (pointer decode guards silent disable).
	if status, _ := ts.do(t, http.MethodPatch, path, map[string]any{}); status != http.StatusBadRequest {
		t.Errorf("empty body status=%d; want 400", status)
	}
	// Unknown id is a 404.
	if status, _ := ts.do(t, http.MethodPatch, "/api/v1/webhooks/99999", map[string]any{"enabled": false}); status != http.StatusNotFound {
		t.Errorf("unknown id status=%d; want 404", status)
	}
}

// TestWebhooks_EnqueuedOnMatchingChange proves the outbox wiring: a secret
// upsert on a subscribed (project, env) enqueues exactly one delivery, and a
// non-subscribed event enqueues none.
func TestWebhooks_EnqueuedOnMatchingChange(t *testing.T) {
	ts := newTestServer(t)
	seedWebhookProject(t, ts)
	// Subscribe to upsert only, env dev.
	id, _ := createWebhook(t, ts, map[string]any{
		"project": "comax", "env": "dev",
		"url": "http://10.1.2.3/hook", "events": []string{"secret.upsert"},
	})

	// An upsert enqueues one delivery.
	if status, _ := ts.do(t, http.MethodPut, "/api/v1/projects/comax/envs/dev/secrets/DB_URL",
		map[string]string{"value": "postgres://x/y"}); status != http.StatusCreated {
		t.Fatalf("put: status=%d", status)
	}
	_, env := ts.do(t, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d/deliveries", id), nil)
	deliveries := env.Data.([]any)
	if len(deliveries) != 1 {
		t.Fatalf("deliveries after upsert = %d; want 1", len(deliveries))
	}
	if got := deliveries[0].(map[string]any)["event"].(string); got != store.EventSecretUpsert {
		t.Errorf("delivery event = %q; want %q", got, store.EventSecretUpsert)
	}
	// The delivery payload (checked at the store layer) must never carry the
	// value — here we assert the listing does not surface it either.
	if _, present := deliveries[0].(map[string]any)["payload"]; present {
		t.Error("delivery listing leaked payload column")
	}

	// A delete is NOT subscribed → no new delivery.
	if status, _ := ts.do(t, http.MethodDelete, "/api/v1/projects/comax/envs/dev/secrets/DB_URL", nil); status != http.StatusNoContent {
		t.Fatalf("delete: status=%d", status)
	}
	_, env = ts.do(t, http.MethodGet, fmt.Sprintf("/api/v1/webhooks/%d/deliveries", id), nil)
	if got := len(env.Data.([]any)); got != 1 {
		t.Errorf("deliveries after non-subscribed delete = %d; want 1 (unchanged)", got)
	}
}

// TestEnqueueWebhooks_RolledBackTxLeavesNoDelivery proves the transactional
// outbox guarantee directly: an enqueue inside a tx that rolls back leaves no
// delivery row, while the same enqueue committed leaves exactly one.
func TestEnqueueWebhooks_RolledBackTxLeavesNoDelivery(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(t.TempDir() + "/wh.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	p, _ := store.NewProjectRepo(db).Create(ctx, "app")
	hook, _ := store.NewWebhookRepo(db).Create(ctx, p.ID, nil, "http://10.0.0.1/h", store.EventSecretUpsert, []byte("k"))
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	payload := webhook.Payload{Action: store.EventSecretUpsert, Project: "app", Env: "prod", Key: "K"}

	// Rolled-back tx → no delivery.
	tx, _ := db.BeginTx(ctx, nil)
	if err := enqueueWebhooks(req, tx, p.ID, 1, store.EventSecretUpsert, payload); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	_ = tx.Rollback()
	if got, _ := store.NewDeliveryRepo(db).ListByWebhook(ctx, hook.ID, 10); len(got) != 0 {
		t.Errorf("rolled-back tx left %d deliveries; want 0 (ghost delivery)", len(got))
	}

	// Committed tx → exactly one.
	tx2, _ := db.BeginTx(ctx, nil)
	if err := enqueueWebhooks(req, tx2, p.ID, 1, store.EventSecretUpsert, payload); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if got, _ := store.NewDeliveryRepo(db).ListByWebhook(ctx, hook.ID, 10); len(got) != 1 {
		t.Errorf("committed tx left %d deliveries; want 1", len(got))
	}
}
