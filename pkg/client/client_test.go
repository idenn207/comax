package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// writeEnv writes a success envelope {"ok":true,"data":data}, matching the
// server's response.go shape that Client.do unwraps.
func writeEnv(t *testing.T, w http.ResponseWriter, status int, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	env := map[string]any{"ok": true, "data": json.RawMessage(raw)}
	if err := json.NewEncoder(w).Encode(env); err != nil {
		t.Fatalf("encode env: %v", err)
	}
}

func TestWebhookMethods_RoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/webhooks", func(w http.ResponseWriter, r *http.Request) {
		var in CreateWebhookInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if in.Project != "app" || in.URL != "http://10.0.0.1/h" {
			t.Errorf("unexpected create input: %+v", in)
		}
		writeEnv(t, w, http.StatusCreated, WebhookCreated{
			ID: 1, Project: in.Project, URL: in.URL, Events: in.Events,
			Enabled: true, SigningSecret: "whsec_once", CreatedAt: time.Now(),
		})
	})
	mux.HandleFunc("GET /api/v1/webhooks", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(t, w, http.StatusOK, []Webhook{
			{ID: 1, Project: "app", URL: "http://10.0.0.1/h", Events: []string{"secret.upsert"}, Enabled: true},
		})
	})
	var deletedID string
	mux.HandleFunc("DELETE /api/v1/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		deletedID = r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})
	var patchedID string
	var patchedEnabled bool
	mux.HandleFunc("PATCH /api/v1/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		patchedID = r.PathValue("id")
		var in map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Errorf("decode patch body: %v", err)
		}
		patchedEnabled = in["enabled"]
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/webhooks/{id}/deliveries", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(t, w, http.StatusOK, []Delivery{
			{ID: 9, Event: "secret.upsert", Status: "delivered", Attempts: 1},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cl, err := New(srv.URL, "admin-token", time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	created, err := cl.CreateWebhook(ctx, CreateWebhookInput{
		Project: "app", URL: "http://10.0.0.1/h", Events: []string{"secret.upsert"},
	})
	if err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}
	if created.SigningSecret != "whsec_once" || created.ID != 1 {
		t.Errorf("created = %+v", created)
	}

	list, err := cl.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks: %v", err)
	}
	if len(list) != 1 || list[0].URL != "http://10.0.0.1/h" {
		t.Errorf("list = %+v", list)
	}

	deliveries, err := cl.ListDeliveries(ctx, 1)
	if err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].Status != "delivered" {
		t.Errorf("deliveries = %+v", deliveries)
	}

	if err := cl.DeleteWebhook(ctx, 7); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}
	if deletedID != "7" {
		t.Errorf("server saw delete id %q; want 7", deletedID)
	}

	if err := cl.SetWebhookEnabled(ctx, 3, false); err != nil {
		t.Fatalf("SetWebhookEnabled: %v", err)
	}
	if patchedID != "3" || patchedEnabled {
		t.Errorf("server saw patch id=%q enabled=%v; want id=3 enabled=false", patchedID, patchedEnabled)
	}
}

func TestCreateWebhook_SurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": map[string]string{"code": "bad_request", "message": "invalid webhook url"},
		})
	}))
	defer srv.Close()

	cl, _ := New(srv.URL, "admin-token", time.Second)
	_, err := cl.CreateWebhook(context.Background(), CreateWebhookInput{Project: "app", URL: "ftp://x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Errorf("err = %v; want *APIError code=bad_request", err)
	}
}
