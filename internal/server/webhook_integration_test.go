package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/idenn207/comax-secrets/internal/webhook"
)

// TestWebhookDelivery_EndToEnd is the M4 acceptance proof: it drives the full
// path — register a webhook over HTTP, change a secret over HTTP (which enqueues
// through the transactional outbox), run the real delivery worker, and assert
// the httptest receiver got a VALID HMAC signature over a payload that contains
// NO secret plaintext. The worker shares the server's master key so the sealed
// signing secret opens correctly.
func TestWebhookDelivery_EndToEnd(t *testing.T) {
	ts := newTestServer(t)
	seedWebhookProject(t, ts) // bootstrap + project "comax" + env "dev"

	var (
		mu            sync.Mutex
		hits          int
		lastBody      []byte
		lastEvent     string
		signingSecret string
		validSig      bool
	)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		hits++
		body, _ := io.ReadAll(r.Body)
		lastBody = body
		lastEvent = r.Header.Get(webhook.HeaderEvent)
		tsHeader := r.Header.Get(webhook.HeaderTimestamp)
		mac := hmac.New(sha256.New, []byte(signingSecret))
		mac.Write([]byte(tsHeader + "."))
		mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		validSig = hmac.Equal([]byte(r.Header.Get(webhook.HeaderSignature)), []byte(want))
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	// Register the webhook through the public HTTP API.
	_, secret := createWebhook(t, ts, map[string]any{
		"project": "comax", "env": "dev",
		"url": receiver.URL, "events": []string{"secret.upsert", "secret.delete"},
	})
	mu.Lock()
	signingSecret = secret // established before the worker (and any delivery) starts
	mu.Unlock()

	// Change a secret through the HTTP API → enqueues an upsert delivery.
	const canary = "postgres://user:super-secret-pw@db/app"
	if status, _ := ts.do(t, http.MethodPut,
		"/api/v1/projects/comax/envs/dev/secrets/DB_URL",
		map[string]string{"value": canary}); status != http.StatusCreated {
		t.Fatalf("put secret: status=%d", status)
	}

	// Run the real worker until the delivery lands.
	worker := webhook.NewWorker(webhook.Options{DB: ts.db, Keys: ts.keys})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx, 10*time.Millisecond)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := hits >= 1
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if hits < 1 {
		t.Fatal("worker never delivered to the receiver")
	}
	if !validSig {
		t.Error("delivered payload had an invalid HMAC signature")
	}
	if lastEvent != "secret.upsert" {
		t.Errorf("event header = %q; want secret.upsert", lastEvent)
	}
	if bytes.Contains(lastBody, []byte(canary)) {
		t.Errorf("delivered payload leaked the secret value: %s", lastBody)
	}
	if bytes.Contains(lastBody, []byte("super-secret-pw")) {
		t.Error("delivered payload leaked secret substring")
	}
}
