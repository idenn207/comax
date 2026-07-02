package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/idenn207/comax-secrets/internal/auth"
	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
	"github.com/idenn207/comax-secrets/internal/webhook"
)

// maxDeliveryList caps how many recent deliveries GET .../deliveries returns.
const maxDeliveryList = 50

// validWebhookEvents is the set an operator may subscribe to. It mirrors the
// audit action strings emitted by the secret handlers.
var validWebhookEvents = map[string]bool{
	store.EventSecretUpsert:   true,
	store.EventSecretRollback: true,
	store.EventSecretDelete:   true,
}

// webhookView is the JSON listing shape. secret_ciphertext is NEVER included —
// the signing key is shown once at creation and never again.
type webhookView struct {
	ID        int64     `json:"id"`
	Project   string    `json:"project"`
	Env       *string   `json:"env,omitempty"` // nil = all environments
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// webhookCreatedView is returned once from POST /api/v1/webhooks: it is the
// only response carrying the plaintext signing secret. The operator must copy
// it now to configure the receiver's signature verification.
type webhookCreatedView struct {
	ID            int64     `json:"id"`
	Project       string    `json:"project"`
	Env           *string   `json:"env,omitempty"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	Enabled       bool      `json:"enabled"`
	SigningSecret string    `json:"signing_secret"`
	CreatedAt     time.Time `json:"created_at"`
}

// deliveryView is the JSON shape for GET .../deliveries. The payload column is
// deliberately omitted from the listing — it is metadata only, and the status
// fields are what an operator needs to see.
type deliveryView struct {
	ID            int64      `json:"id"`
	Event         string     `json:"event"`
	Status        string     `json:"status"`
	Attempts      int64      `json:"attempts"`
	LastStatus    *int64     `json:"last_status,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	CreatedAt     time.Time  `json:"created_at"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
}

// handleCreateWebhook registers a webhook. Admin-only. It resolves the target
// project/env, SSRF-validates the URL, generates a signing secret, seals it
// with the master key, and returns the plaintext secret exactly once.
func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Project string   `json:"project"`
		Env     string   `json:"env"`
		URL     string   `json:"url"`
		Events  []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", s.logger)
		return
	}
	if err := validateName("project", body.Project); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	proj, err := store.NewProjectRepo(s.db).ByName(r.Context(), body.Project)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("webhook create: lookup project", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	// env is optional — omit for "all environments in the project".
	var envID *int64
	var envName *string
	if body.Env != "" {
		if err := validateName("env", body.Env); err != nil {
			status, code, msg := httpError(err)
			writeError(w, status, code, msg, s.logger)
			return
		}
		env, err := store.NewEnvRepo(s.db).ByName(r.Context(), proj.ID, body.Env)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.logger.Error("webhook create: lookup env", slog.String("err", err.Error()))
			}
			status, code, msg := httpError(err)
			writeError(w, status, code, msg, s.logger)
			return
		}
		envID = &env.ID
		envName = &env.Name
	}

	// SSRF guard: only http/https, resolvable, non-metadata (R1 F2).
	if err := webhook.ValidateURL(r.Context(), body.URL, s.webhookPolicy); err != nil {
		// The error names the host/scheme but never secret material.
		writeError(w, http.StatusBadRequest, "bad_request", "invalid webhook url: "+err.Error(), s.logger)
		return
	}

	events, err := normalizeWebhookEvents(body.Events)
	if err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	// Signing secret: generated here, shown once, stored only as ciphertext.
	signingSecret, err := auth.GenerateToken()
	if err != nil {
		s.logger.Error("webhook create: generate secret", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	master, err := s.keys.Key(r.Context())
	if err != nil {
		s.logger.Error("webhook create: load master key", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "key unavailable", s.logger)
		return
	}
	sealed, err := crypto.Seal(master, []byte(signingSecret))
	if err != nil {
		s.logger.Error("webhook create: seal secret", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "encrypt failed", s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("webhook create: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	hook, err := store.NewWebhookRepo(tx).Create(r.Context(), proj.ID, envID, body.URL, events, sealed)
	if err != nil {
		s.logger.Error("webhook create: store", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "create failed", s.logger)
		return
	}
	if err := appendAudit(r, tx, "webhook.create", fmt.Sprintf("project=%s webhook_id=%d", proj.Name, hook.ID)); err != nil {
		s.logger.Error("webhook create: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("webhook create: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}

	writeOK(w, http.StatusCreated, webhookCreatedView{
		ID:            hook.ID,
		Project:       proj.Name,
		Env:           envName,
		URL:           hook.URL,
		Events:        strings.Split(events, ","),
		Enabled:       hook.Enabled,
		SigningSecret: signingSecret,
		CreatedAt:     hook.CreatedAt,
	}, s.logger)
}

// handleListWebhooks returns every webhook (metadata only). Admin-only.
func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	hooks, err := store.NewWebhookRepo(s.db).List(r.Context())
	if err != nil {
		s.logger.Error("webhook list", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}

	// Resolve project ids to names once; env names on demand (small N).
	projects, err := store.NewProjectRepo(s.db).List(r.Context())
	if err != nil {
		s.logger.Error("webhook list: projects", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	projName := make(map[int64]string, len(projects))
	for _, p := range projects {
		projName[p.ID] = p.Name
	}

	// Resolve env ids to names in one query, symmetric with projects above — a
	// per-webhook ByID would be an N+1 across the list.
	envs, err := store.NewEnvRepo(s.db).List(r.Context())
	if err != nil {
		s.logger.Error("webhook list: envs", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	envName := make(map[int64]string, len(envs))
	for _, e := range envs {
		envName[e.ID] = e.Name
	}

	out := make([]webhookView, 0, len(hooks))
	for _, h := range hooks {
		var envPtr *string
		if h.EnvID != nil {
			if name, ok := envName[*h.EnvID]; ok {
				n := name
				envPtr = &n
			}
		}
		out = append(out, webhookView{
			ID:        h.ID,
			Project:   projName[h.ProjectID],
			Env:       envPtr,
			URL:       h.URL,
			Events:    strings.Split(h.Events, ","),
			Enabled:   h.Enabled,
			CreatedAt: h.CreatedAt,
			UpdatedAt: h.UpdatedAt,
		})
	}
	writeOK(w, http.StatusOK, out, s.logger)
}

// handleDeleteWebhook removes a webhook by id. Admin-only. 204 on success;
// 404 when the id is unknown. Its delivery history is cascaded away.
func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid webhook id", s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("webhook delete: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if err := store.NewWebhookRepo(tx).Delete(r.Context(), id); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("webhook delete: store", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if err := appendAudit(r, tx, "webhook.delete", fmt.Sprintf("webhook_id=%d", id)); err != nil {
		s.logger.Error("webhook delete: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("webhook delete: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSetWebhookEnabled toggles a webhook's enabled flag (soft-disable).
// Admin-only. Body: {"enabled": bool}. A disabled webhook is skipped by
// MatchForEvent, so it stops receiving NEW deliveries without losing its
// registration or history; already-enqueued deliveries still drain. 204 on
// success; 404 when the id is unknown; 400 when the body omits "enabled".
func (s *Server) handleSetWebhookEnabled(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid webhook id", s.logger)
		return
	}
	// Pointer so a missing field is a 400, not a silent disable: the Go zero
	// value of a bool cannot distinguish absent from explicit false.
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", s.logger)
		return
	}
	if body.Enabled == nil {
		writeError(w, http.StatusBadRequest, "bad_request", `missing "enabled" field`, s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("webhook set-enabled: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if err := store.NewWebhookRepo(tx).SetEnabled(r.Context(), id, *body.Enabled); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("webhook set-enabled: store", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	action := "webhook.disable"
	if *body.Enabled {
		action = "webhook.enable"
	}
	if err := appendAudit(r, tx, action, fmt.Sprintf("webhook_id=%d", id)); err != nil {
		s.logger.Error("webhook set-enabled: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("webhook set-enabled: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListDeliveries returns the recent delivery attempts for a webhook.
// Admin-only.
func (s *Server) handleListDeliveries(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid webhook id", s.logger)
		return
	}
	deliveries, err := store.NewDeliveryRepo(s.db).ListByWebhook(r.Context(), id, maxDeliveryList)
	if err != nil {
		s.logger.Error("webhook deliveries", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	out := make([]deliveryView, 0, len(deliveries))
	for _, d := range deliveries {
		out = append(out, deliveryView{
			ID:            d.ID,
			Event:         d.Event,
			Status:        d.Status,
			Attempts:      d.Attempts,
			LastStatus:    d.LastStatus,
			LastError:     d.LastError,
			NextAttemptAt: d.NextAttemptAt,
			CreatedAt:     d.CreatedAt,
			DeliveredAt:   d.DeliveredAt,
		})
	}
	writeOK(w, http.StatusOK, out, s.logger)
}

// normalizeWebhookEvents validates and de-duplicates the requested event list.
// An empty request subscribes to all known events. An unknown event is a 400.
func normalizeWebhookEvents(in []string) (string, error) {
	if len(in) == 0 {
		return strings.Join([]string{
			store.EventSecretUpsert, store.EventSecretRollback, store.EventSecretDelete,
		}, ","), nil
	}
	seen := make(map[string]bool, len(in))
	var out []string
	for _, e := range in {
		e = strings.TrimSpace(e)
		if !validWebhookEvents[e] {
			return "", fmt.Errorf("unknown event %q: %w", e, errBadRequest)
		}
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return strings.Join(out, ","), nil
}

// enqueueWebhooks inserts an outbox row for every webhook matching
// (projectID, envID, event). It runs inside the caller's tx — the same tx as
// the secret change and its audit row — so a rolled-back change enqueues
// nothing (transactional outbox). Mirrors appendAudit's shape.
func enqueueWebhooks(r *http.Request, tx store.DBTX, projectID, envID int64, event string, payload webhook.Payload) error {
	hooks, err := store.NewWebhookRepo(tx).MatchForEvent(r.Context(), projectID, envID, event)
	if err != nil {
		return err
	}
	if len(hooks) == 0 {
		return nil
	}
	body, err := payload.Marshal()
	if err != nil {
		return err
	}
	deliveries := store.NewDeliveryRepo(tx)
	for _, h := range hooks {
		if _, err := deliveries.Enqueue(r.Context(), h.ID, event, string(body)); err != nil {
			return err
		}
	}
	return nil
}
