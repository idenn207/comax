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

const (
	// sessionCookieName is the cookie the dashboard sets at login. The
	// CLI never reads or writes this cookie; bearer-auth requests stay
	// completely separate from the browser session flow.
	sessionCookieName = "comax_session"

	// sessionTTL is the lifetime of a dashboard session. 30 days matches
	// the plan; operators can revoke earlier via DELETE
	// /api/v1/dashboard/session.
	sessionTTL = 30 * 24 * time.Hour

	// csrfHeader is the header the dashboard must echo on every mutating
	// request. The double-submit invariant: cookie value's session row
	// holds csrf_hash; the header carries the matching plaintext.
	csrfHeader = "X-CSRF-Token"
)

// sessionCreateRequest is the JSON body POSTed to /dashboard/session. We
// take the bearer in the body — not the header — so the request matches
// the dashboard's form-fetch ergonomics (paste-token → submit) and never
// caches a token in the proxy's Authorization log.
type sessionCreateRequest struct {
	Token string `json:"token"`
}

// sessionCreateResponse echoes the CSRF plaintext exactly once. The
// session cookie itself is set via Set-Cookie; only the CSRF token needs
// to live in JS memory (the cookie is HttpOnly by design).
type sessionCreateResponse struct {
	CSRF      string    `json:"csrf"`
	ExpiresAt time.Time `json:"expires_at"`
}

// handleCreateDashboardSession exchanges a bearer token for a browser
// session cookie + CSRF token. This endpoint is exempt from the auth
// middleware (see isPublicPath) because the bearer arrives in the body,
// not the Authorization header.
func (s *Server) handleCreateDashboardSession(w http.ResponseWriter, r *http.Request) {
	var body sessionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", s.logger)
		return
	}

	// Verify the bearer against service_tokens. Auth failures look the
	// same as missing/unknown bearer in the rest of the API for shape
	// consistency.
	tok, err := auth.Verify(r.Context(), store.NewTokenRepo(s.db), body.Token)
	if err != nil {
		if !errors.Is(err, auth.ErrUnknownToken) && !errors.Is(err, auth.ErrMissingBearer) {
			s.logger.Warn("session create: verify bearer", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	sessionPlain, err := auth.GenerateToken()
	if err != nil {
		s.logger.Error("session create: generate session", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	csrfPlain, err := auth.GenerateCSRF()
	if err != nil {
		s.logger.Error("session create: generate csrf", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("session create: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	in := store.SessionCreateInput{
		TokenID:     tok.ID,
		SessionHash: auth.HashToken(sessionPlain),
		CSRFHash:    auth.HashCSRF(csrfPlain),
		UserAgent:   truncateUserAgent(r.UserAgent()),
		IPPrefix:    auth.IPPrefix(r.RemoteAddr),
		TTL:         sessionTTL,
	}
	sess, err := store.NewSessionRepo(tx).Create(r.Context(), in)
	if err != nil {
		s.logger.Error("session create: insert", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	// Audit attribution: the underlying service token is the actor; the
	// session is just a vehicle for the same identity.
	auditTarget := fmt.Sprintf("token_id=%d session_id=%d", tok.ID, sess.ID)
	if err := appendAuditForToken(r, tx, tok.ID, "session.create", auditTarget); err != nil {
		s.logger.Error("session create: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("session create: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionPlain,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	writeOK(w, http.StatusCreated, sessionCreateResponse{
		CSRF:      csrfPlain,
		ExpiresAt: sess.ExpiresAt,
	}, s.logger)
}

// handleRevokeDashboardSession revokes the session bound to the cookie
// on the incoming request. Always returns 204 on success; idempotent so
// a double-logout doesn't 4xx.
func (s *Server) handleRevokeDashboardSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		// No cookie → nothing to revoke. Treat as success so logout-when-
		// already-logged-out doesn't surface as an error.
		clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	hash := auth.HashToken(cookie.Value)

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("session revoke: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	repo := store.NewSessionRepo(tx)
	sess, err := repo.ByHash(r.Context(), hash)
	if err != nil {
		// Unknown / already-revoked session: clear the cookie on the client
		// side and return 204. We avoid a 404 here so that a stale cookie
		// after a server-side prune still produces a clean logout UX.
		clearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := repo.Revoke(r.Context(), sess.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("session revoke: store", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	auditTarget := fmt.Sprintf("token_id=%d session_id=%d", sess.TokenID, sess.ID)
	if err := appendAuditForToken(r, tx, sess.TokenID, "session.revoke", auditTarget); err != nil {
		s.logger.Error("session revoke: audit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("session revoke: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}

	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// clearSessionCookie sets a same-named cookie with MaxAge=-1 so the
// browser drops the session cookie regardless of why logout was hit.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// sessionListItem is the per-row JSON shape for GET /dashboard/sessions.
// Hashes (session_hash, csrf_hash) are deliberately omitted: the operator
// never needs them and exposing them would weaken the "plaintext only
// leaves the server once" invariant.
//
// IsCurrent flags the session whose cookie carried this request so the
// UI can disable revoke on it (revoking the active session would land
// the operator on /login mid-action, which is a poor pattern even if
// technically idempotent on the backend).
type sessionListItem struct {
	ID        int64     `json:"id"`
	UserAgent string    `json:"user_agent"`
	IPPrefix  string    `json:"ip_prefix"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IsCurrent bool      `json:"is_current"`
}

// handleListDashboardSessions returns the actor token's own live sessions.
// Cross-token discovery is impossible by design — the WHERE clause in
// SessionRepo.ListByTokenID binds the actor's token id and there is no
// admin-scope variant of this endpoint.
//
// The endpoint is GET so the cookie + bearer arms of authMiddleware both
// authenticate it; CSRF only kicks in on mutating methods.
func (s *Server) handleListDashboardSessions(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing actor", s.logger)
		return
	}
	repo := store.NewSessionRepo(s.db)
	sessions, err := repo.ListByTokenID(r.Context(), actor.ID)
	if err != nil {
		s.logger.Error("sessions list", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}

	// Resolve the request's own session id (if any) so IsCurrent lights
	// up the row representing this very request. A failed lookup leaves
	// currentID at zero and silently produces is_current=false for all
	// rows — that's the right shape for bearer-arm callers like the CLI
	// which have no cookie to begin with.
	var currentID int64
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		if cur, err := repo.ByHash(r.Context(), auth.HashToken(cookie.Value)); err == nil {
			currentID = cur.ID
		}
	}

	out := make([]sessionListItem, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, sessionListItem{
			ID:        sess.ID,
			UserAgent: sess.UserAgent,
			IPPrefix:  sess.IPPrefix,
			CreatedAt: sess.CreatedAt,
			ExpiresAt: sess.ExpiresAt,
			IsCurrent: sess.ID == currentID,
		})
	}
	writeOK(w, http.StatusOK, out, s.logger)
}

// handleRevokeDashboardSessionByID revokes a single session row by id,
// scoped to the actor's own token. Always 204 (or 4xx for bad input);
// the body is never used to distinguish "you don't own this id" from
// "no such id" from "already revoked" — see SessionRepo.RevokeByID-
// AndTokenID for why those three cases collapse into one response.
//
// An audit row is appended only when affected == 1 so that probes of
// foreign / unknown ids leave no forensic trail the prober could later
// read back. The middleware enforces CSRF on this DELETE automatically.
func (s *Server) handleRevokeDashboardSessionByID(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing actor", s.logger)
		return
	}
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || targetID <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid session id", s.logger)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("revoke by id: begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	n, err := store.NewSessionRepo(tx).RevokeByIDAndTokenID(r.Context(), targetID, actor.ID)
	if err != nil {
		s.logger.Error("revoke by id: store", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	if n > 0 {
		target := fmt.Sprintf("token_id=%d session_id=%d", actor.ID, targetID)
		if err := appendAuditForToken(r, tx, actor.ID, "session.revoke_by_id", target); err != nil {
			s.logger.Error("revoke by id: audit", slog.String("err", err.Error()))
			writeError(w, http.StatusInternalServerError, "internal", "audit failed", s.logger)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("revoke by id: commit", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "commit failed", s.logger)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// truncateUserAgent caps the persisted UA at a reasonable length so a
// hostile client can't blow up the dashboard_sessions row. 512 is
// generous for legitimate UAs (the longest real-world Chrome UA is
// ~150 chars) without being so loose it invites abuse.
func truncateUserAgent(ua string) string {
	const max = 512
	if len(ua) <= max {
		return ua
	}
	return ua[:max]
}
