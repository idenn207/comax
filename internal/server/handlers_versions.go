package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/idenn207/comax-secrets/internal/crypto"
	"github.com/idenn207/comax-secrets/internal/store"
)

// versionView is the API shape. We expose ciphertext length (not the
// ciphertext itself) so dashboard/M2 callers can size diff UIs without
// re-pulling the full history. Decrypted historical values are only
// available via a future rollback endpoint (M2), not via this list.
type versionView struct {
	ID             int64     `json:"id"`
	SecretID       int64     `json:"secret_id"`
	Version        int64     `json:"version"`
	CiphertextSize int       `json:"ciphertext_size"`
	ActorTokenID   *int64    `json:"actor_token_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// secretVersionView is the decrypted historical-value shape returned by
// GET .../versions/{v}. The dashboard's diff viewer needs the plaintext
// of a specific past version side-by-side with the current value; the
// ciphertext stays inside the server.
type secretVersionView struct {
	Key          string    `json:"key"`
	Version      int64     `json:"version"`
	Value        string    `json:"value"`
	ActorTokenID *int64    `json:"actor_token_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// handleGetVersion returns the decrypted value of a specific historical
// version of a secret. Read-only — does not produce a new version. Works
// for soft-deleted secrets too, so the dashboard can still render the
// timeline of a removed key.
func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	_, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	keyName := r.PathValue("k")
	if err := validateName("key", keyName); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	verStr := r.PathValue("v")
	version, err := strconv.ParseInt(verStr, 10, 64)
	if err != nil || version <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "version must be a positive integer", s.logger)
		return
	}

	sec, err := store.NewSecretRepo(s.db).ByKeyAny(r.Context(), e.ID, keyName)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("lookup secret", slog.String("key", keyName), slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	v, err := store.NewVersionRepo(s.db).ByVersion(r.Context(), sec.ID, version)
	if err != nil {
		if !errors.Is(err, store.ErrVersionNotFound) {
			s.logger.Error("lookup version", slog.Int64("secret_id", sec.ID), slog.Int64("version", version), slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	master, err := s.keys.Key(r.Context())
	if err != nil {
		s.logger.Error("load master key", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "key unavailable", s.logger)
		return
	}
	plain, err := crypto.Open(master, v.Ciphertext)
	if err != nil {
		s.logger.Error("decrypt version", slog.Int64("secret_id", sec.ID), slog.Int64("version", version), slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "decrypt failed", s.logger)
		return
	}

	writeOK(w, http.StatusOK, secretVersionView{
		Key:          keyName,
		Version:      v.Version,
		Value:        string(plain),
		ActorTokenID: v.ActorToken,
		CreatedAt:    v.CreatedAt,
	}, s.logger)
}

// handleListVersions returns every historical version across every
// secret in the given env, including the timeline of soft-deleted keys
// so the dashboard's history view stays complete after a delete.
func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	_, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	secrets, err := store.NewSecretRepo(s.db).ListByEnvAny(r.Context(), e.ID)
	if err != nil {
		s.logger.Error("list secrets for versions", slog.String("err", err.Error()))
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	verRepo := store.NewVersionRepo(s.db)
	var out []versionView
	for _, sec := range secrets {
		vs, err := verRepo.ListBySecret(r.Context(), sec.ID)
		if err != nil {
			s.logger.Error("list versions", slog.String("err", err.Error()))
			status, code, msg := httpError(err)
			writeError(w, status, code, msg, s.logger)
			return
		}
		for _, v := range vs {
			out = append(out, versionView{
				ID:             v.ID,
				SecretID:       v.SecretID,
				Version:        v.Version,
				CiphertextSize: len(v.Ciphertext),
				ActorTokenID:   v.ActorToken,
				CreatedAt:      v.CreatedAt,
			})
		}
	}
	writeOK(w, http.StatusOK, out, s.logger)
}
