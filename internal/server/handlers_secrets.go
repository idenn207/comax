package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/idenn207/comax-secrets/internal/auth"
	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/secret"
	"github.com/idenn207/comax-secrets/internal/store"
	"github.com/idenn207/comax-secrets/internal/webhook"
)

// secretView is the resolved-plaintext shape returned by GET endpoints.
// The ciphertext blob never leaves the server.
//
// SecretID is the underlying secrets row id (parent env's row for
// inherited entries). The dashboard needs it to correlate this view
// with its version timeline: two keys in the same env can share a
// current version number, and matching on (key, version) alone is
// ambiguous.
type secretView struct {
	SecretID  int64     `json:"secret_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// resolveEnv loads {project, env} from the URL path. Writes a 404
// envelope to w and returns ok=false if either is missing.
func (s *Server) resolveEnv(w http.ResponseWriter, r *http.Request) (store.Project, store.Environment, bool) {
	p, ok := s.resolveProject(w, r)
	if !ok {
		return store.Project{}, store.Environment{}, false
	}
	envName := r.PathValue("e")
	if err := validateName("env", envName); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return store.Project{}, store.Environment{}, false
	}
	e, err := store.NewEnvRepo(s.db).ByName(r.Context(), p.ID, envName)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("lookup env", slog.String("env", envName), slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return store.Project{}, store.Environment{}, false
	}
	return p, e, true
}

// resolverErrorToHTTP maps resolver errors to envelope shapes. Cycles
// and unknown references are operator-facing 400s; everything else is
// a 500.
func resolverErrorToHTTP(err error) (status int, code, msg string) {
	var cyc *secret.CycleError
	if errors.As(err, &cyc) {
		return http.StatusBadRequest, "bad_reference", err.Error()
	}
	if errors.Is(err, secret.ErrUnknownReference) {
		return http.StatusBadRequest, "bad_reference", err.Error()
	}
	return http.StatusInternalServerError, "internal", "resolve failed"
}

// handleListSecrets returns the decrypted, reference-resolved secrets
// for the given env. This is what `secret pull` and `secret run` call.
func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	p, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	snap, err := s.resolver.Resolve(r.Context(), p.ID, e.ID)
	if err != nil {
		s.logger.Warn("resolve env",
			slog.String("env", e.Name), slog.String("err", err.Error()))
		status, code, msg := resolverErrorToHTTP(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	views := make([]secretView, 0, len(snap))
	for k, sec := range snap {
		views = append(views, secretView{
			SecretID:  sec.SecretID,
			Key:       k,
			Value:     sec.Value,
			Version:   sec.Version,
			UpdatedAt: sec.UpdatedAt,
		})
	}
	// Map iteration order is randomised; sort for deterministic output
	// (pull/run consumers diff client-side and need stable ordering).
	sort.Slice(views, func(i, j int) bool { return views[i].Key < views[j].Key })
	writeOK(w, http.StatusOK, views, s.logger)
}

func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	p, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	keyName := r.PathValue("k")
	if err := validateName("key", keyName); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	snap, err := s.resolver.Resolve(r.Context(), p.ID, e.ID)
	if err != nil {
		s.logger.Warn("resolve env",
			slog.String("env", e.Name), slog.String("err", err.Error()))
		status, code, msg := resolverErrorToHTTP(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	sec, ok := snap[keyName]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "resource not found", s.logger)
		return
	}
	writeOK(w, http.StatusOK, secretView{
		SecretID:  sec.SecretID,
		Key:       keyName,
		Value:     sec.Value,
		Version:   sec.Version,
		UpdatedAt: sec.UpdatedAt,
	}, s.logger)
}

// handlePutSecret is the upsert endpoint. It encrypts the value, writes
// the secrets row, appends a row to secret_versions, and appends an
// audit_log entry — all inside one transaction so a half-applied write
// never appears in history.
//
// Encryption uses Server.keys directly (not the resolver) because the
// write path operates on the raw, non-inherited secret. The resolver is
// strictly a read-side concern.
func (s *Server) handlePutSecret(w http.ResponseWriter, r *http.Request) {
	p, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	keyName := r.PathValue("k")
	if err := validateName("key", keyName); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", s.logger)
		return
	}

	master, err := s.keys.Key(r.Context())
	if err != nil {
		s.logger.Error("load master key", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "key unavailable", s.logger)
		return
	}
	ct, err := crypto.Seal(master, []byte(body.Value))
	if err != nil {
		s.logger.Error("seal secret", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "encrypt failed", s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	up, err := store.NewSecretRepo(tx).Upsert(r.Context(), e.ID, keyName, ct)
	if err != nil {
		s.logger.Error("upsert secret", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "upsert failed", s.logger)
		return
	}

	var actorID *int64
	if tok, ok := auth.FromContext(r.Context()); ok {
		id := tok.ID
		actorID = &id
	}
	if _, err := store.NewVersionRepo(tx).Create(r.Context(), up.Secret.ID, up.Secret.Version, ct, actorID); err != nil {
		s.logger.Error("append version", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "version append failed", s.logger)
		return
	}
	if err := appendAudit(r, tx, "secret.upsert", fmt.Sprintf("project=%s env=%s key=%s version=%d", p.Name, e.Name, keyName, up.Secret.Version)); err != nil {
		s.logger.Error("audit secret.upsert", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	// Transactional outbox: enqueue deliveries for matching webhooks in the
	// SAME tx as the change + audit, so a rolled-back write leaves no delivery.
	if err := enqueueWebhooks(r, tx, p.ID, e.ID, store.EventSecretUpsert, webhook.Payload{
		Action:    store.EventSecretUpsert,
		Project:   p.Name,
		Env:       e.Name,
		Key:       keyName,
		Version:   up.Secret.Version,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		s.logger.Error("enqueue webhooks: upsert", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "webhook enqueue failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("commit secret upsert", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}

	status := http.StatusOK
	if up.Created {
		status = http.StatusCreated
	}
	writeOK(w, status, secretView{
		SecretID:  up.Secret.ID,
		Key:       up.Secret.Key,
		Value:     body.Value,
		Version:   up.Secret.Version,
		UpdatedAt: up.Secret.UpdatedAt,
	}, s.logger)
}

// handleRollbackSecret writes a new version of a secret using the
// ciphertext of a previously-stored target_version. The historical
// blob is re-used as-is (no re-encryption) — it was sealed with the
// same master key M2 cannot rotate without invalidating the chain, so
// the resolver will open it cleanly. A new audit row records the
// transition.
//
// Refuses with 404 when the current secrets row is soft-deleted; the
// operator should re-PUT to restore the key first. (The deviation from
// the plan's `?undelete=true` flag is intentional for this slice — PUT
// already clears deleted_at via the Upsert path.)
func (s *Server) handleRollbackSecret(w http.ResponseWriter, r *http.Request) {
	p, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	keyName := r.PathValue("k")
	if err := validateName("key", keyName); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	var body struct {
		TargetVersion int64 `json:"target_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", s.logger)
		return
	}
	if body.TargetVersion <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "target_version must be a positive integer", s.logger)
		return
	}

	// Current live secret — sets the lineage and proves the key is not
	// soft-deleted.
	current, err := store.NewSecretRepo(s.db).ByKey(r.Context(), e.ID, keyName)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("rollback: lookup current", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if body.TargetVersion == current.Version {
		writeError(w, http.StatusBadRequest, "bad_request", "target_version is already the current version", s.logger)
		return
	}
	target, err := store.NewVersionRepo(s.db).ByVersion(r.Context(), current.ID, body.TargetVersion)
	if err != nil {
		if !errors.Is(err, store.ErrVersionNotFound) {
			s.logger.Error("rollback: lookup target version", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	// Decrypt now so the response body can echo the resolved value back
	// to the dashboard; the ciphertext we write is unchanged from the
	// historical row.
	master, err := s.keys.Key(r.Context())
	if err != nil {
		s.logger.Error("rollback: load master key", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "key unavailable", s.logger)
		return
	}
	plain, err := crypto.Open(master, target.Ciphertext)
	if err != nil {
		s.logger.Error("rollback: decrypt target", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "decrypt failed", s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("rollback: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	up, err := store.NewSecretRepo(tx).Upsert(r.Context(), e.ID, keyName, target.Ciphertext)
	if err != nil {
		s.logger.Error("rollback: upsert", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "upsert failed", s.logger)
		return
	}
	var actorID *int64
	if tok, ok := auth.FromContext(r.Context()); ok {
		id := tok.ID
		actorID = &id
	}
	if _, err := store.NewVersionRepo(tx).Create(r.Context(), up.Secret.ID, up.Secret.Version, target.Ciphertext, actorID); err != nil {
		s.logger.Error("rollback: append version", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "version append failed", s.logger)
		return
	}
	auditTarget := fmt.Sprintf("project=%s env=%s key=%s from_version=%d to_version=%d",
		p.Name, e.Name, keyName, current.Version, body.TargetVersion)
	if err := appendAudit(r, tx, "secret.rollback", auditTarget); err != nil {
		s.logger.Error("rollback: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := enqueueWebhooks(r, tx, p.ID, e.ID, store.EventSecretRollback, webhook.Payload{
		Action:    store.EventSecretRollback,
		Project:   p.Name,
		Env:       e.Name,
		Key:       keyName,
		Version:   up.Secret.Version,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		s.logger.Error("enqueue webhooks: rollback", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "webhook enqueue failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("rollback: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}

	writeOK(w, http.StatusOK, secretView{
		SecretID:  up.Secret.ID,
		Key:       up.Secret.Key,
		Value:     string(plain),
		Version:   up.Secret.Version,
		UpdatedAt: up.Secret.UpdatedAt,
	}, s.logger)
}

// handleDeleteSecret soft-deletes the secret at (env, key). The row in
// secrets is flagged with deleted_at; secret_versions is untouched so
// the dashboard can keep rendering the timeline. A subsequent PUT
// clears the flag (via SecretRepo.Upsert) and continues the version
// sequence — operator never has to think about it.
func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	p, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	keyName := r.PathValue("k")
	if err := validateName("key", keyName); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("delete: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if err := store.NewSecretRepo(tx).Delete(r.Context(), e.ID, keyName); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("delete: store delete", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	auditTarget := fmt.Sprintf("project=%s env=%s key=%s", p.Name, e.Name, keyName)
	if err := appendAudit(r, tx, "secret.delete", auditTarget); err != nil {
		s.logger.Error("delete: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := enqueueWebhooks(r, tx, p.ID, e.ID, store.EventSecretDelete, webhook.Payload{
		Action:    store.EventSecretDelete,
		Project:   p.Name,
		Env:       e.Name,
		Key:       keyName,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		s.logger.Error("enqueue webhooks: delete", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "webhook enqueue failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("delete: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
