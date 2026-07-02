package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/idenn207/comax-secrets/internal/auth"
	"github.com/idenn207/comax-secrets/internal/store"
)

// tokenView is the JSON shape for GET /api/v1/tokens. token_hash is never
// included — a listing exposes only non-secret metadata. RevokedAt is nil
// for live tokens so the dashboard can badge revoked rows distinctly.
type tokenView struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	IsAdmin    bool       `json:"is_admin"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func newTokenView(t store.ServiceToken) tokenView {
	return tokenView{
		ID:         t.ID,
		Name:       t.Name,
		IsAdmin:    t.IsAdmin,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		RevokedAt:  t.RevokedAt,
	}
}

// tokenCreatedView is returned exactly once, from POST /api/v1/tokens: it
// is the only response that carries the plaintext token. The operator must
// copy it now — the server keeps only the SHA-256 hash.
type tokenCreatedView struct {
	Token     string    `json:"token"`
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

// requireAdmin returns the actor token when it is an admin, otherwise it
// writes a 403 (or 401 when no actor is stamped) and returns ok=false.
//
// Token management (issue / list / revoke) is admin-only. Because issued
// CI tokens are non-admin (see handleCreateToken), a leaked CI credential
// cannot escalate by minting or revoking tokens — the blast radius stays
// bounded to secret reads until an admin revokes it.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (store.ServiceToken, bool) {
	actor, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing actor", s.logger)
		return store.ServiceToken{}, false
	}
	if !actor.IsAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin token required", s.logger)
		return store.ServiceToken{}, false
	}
	return actor, true
}

// handleCreateToken issues a new NON-admin service token. Admin-only. The
// plaintext is returned once in the response body and never persisted.
func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", s.logger)
		return
	}
	if err := validateName("name", body.Name); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	plain, err := auth.GenerateToken()
	if err != nil {
		s.logger.Error("token create: generate", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	hash := auth.HashToken(plain)

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("token create: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Issued tokens are always non-admin (isAdmin=false): only the bootstrap
	// token and the migration backfill ever mint admins.
	tok, err := store.NewTokenRepo(tx).Create(r.Context(), body.Name, hash, false)
	if err != nil {
		if !errors.Is(err, store.ErrConflict) {
			s.logger.Error("token create", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if err := appendAudit(r, tx, "auth.token.create", fmt.Sprintf("token=%s", tok.Name)); err != nil {
		s.logger.Error("token create: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("token create: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}

	writeOK(w, http.StatusCreated, tokenCreatedView{
		Token:     plain,
		ID:        tok.ID,
		Name:      tok.Name,
		IsAdmin:   tok.IsAdmin,
		CreatedAt: tok.CreatedAt,
	}, s.logger)
}

// handleListTokens returns every service token (metadata only). Admin-only.
func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	tokens, err := store.NewTokenRepo(s.db).List(r.Context())
	if err != nil {
		s.logger.Error("token list", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	out := make([]tokenView, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, newTokenView(t))
	}
	writeOK(w, http.StatusOK, out, s.logger)
}

// handleRevokeToken soft-revokes a token by id. Admin-only. 204 on
// success; 404 when the id is unknown or already revoked (TokenRepo.Revoke
// returns ErrNotFound in both cases).
func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || targetID <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid token id", s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("token revoke: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	repo := store.NewTokenRepo(tx)

	// Refuse to revoke the last live admin. A soft revoke leaves is_admin=1
	// on the row, so neither the migration backfill (NOT EXISTS is_admin=1)
	// nor BootstrapIfEmpty (COUNT(*)=0) could restore issuing rights — the
	// operator would be locked out of token management with no API path back.
	// ByID deliberately returns revoked rows, so an already-revoked or
	// unknown id falls through to Revoke below and yields the same 404.
	target, err := repo.ByID(r.Context(), targetID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("token revoke: lookup", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if target.IsAdmin && target.RevokedAt == nil {
		liveAdmins, err := repo.CountLiveAdmins(r.Context())
		if err != nil {
			s.logger.Error("token revoke: count admins", slog.String("err", err.Error()))
			writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
			return
		}
		if liveAdmins <= 1 {
			writeError(w, http.StatusConflict, "conflict", "cannot revoke the last live admin token", s.logger)
			return
		}
	}

	if err := repo.Revoke(r.Context(), targetID); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("token revoke: store", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if err := appendAudit(r, tx, "auth.token.revoke", fmt.Sprintf("token_id=%d", targetID)); err != nil {
		s.logger.Error("token revoke: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("token revoke: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
