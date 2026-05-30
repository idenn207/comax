package server

import (
	"log/slog"
	"net/http"
	"time"

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

// handleListVersions returns every historical version across every
// secret in the given env. Used by dashboard/diff (M2); shipped now to
// lock the response shape before M2 starts.
func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	_, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	secrets, err := store.NewSecretRepo(s.db).ListByEnv(r.Context(), e.ID)
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
