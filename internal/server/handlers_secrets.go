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
)

// secretView is the resolved-plaintext shape returned by GET endpoints.
// The ciphertext blob never leaves the server.
type secretView struct {
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
		Key:       up.Secret.Key,
		Value:     body.Value,
		Version:   up.Secret.Version,
		UpdatedAt: up.Secret.UpdatedAt,
	}, s.logger)
}
