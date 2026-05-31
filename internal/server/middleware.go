package server

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/idenn207/comax-secrets/internal/auth"
	"github.com/idenn207/comax-secrets/internal/store"
)

// statusRecorder wraps http.ResponseWriter so the log middleware can
// observe the status code. We default to 200 because WriteHeader is
// optional in Go's net/http — handlers that just call Write implicitly
// emit 200.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// recoverMiddleware turns a panic in a handler into a 500 response and a
// logged stack trace. Without this, a panic kills the goroutine and the
// client sees a torn TCP connection — much harder to debug than a
// proper 500.
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Error("panic recovered",
					slog.Any("panic", rec),
					slog.String("path", r.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// logMiddleware records one structured line per request. Bodies are
// never logged — the whole point of this service is to keep secrets out
// of logs — so we record only method, path, status, and duration.
func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("http",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("dur", time.Since(start)),
		)
	})
}

// authMiddleware enforces bearer-or-session auth. Paths returned by
// isPublicPath are exempt so /healthz, /bootstrap, and the session-
// creation endpoint can be reached without prior credentials.
//
// The auth flow has two arms:
//
//  1. Authorization: Bearer <token> — CLI / CI traffic. CSRF is NOT
//     enforced because there is no cookie ambiently attached to a
//     cross-origin request: the caller had to explicitly include the
//     bearer header.
//
//  2. Cookie: comax_session=<plain> — dashboard browser traffic. For
//     any mutating method (POST/PUT/DELETE/PATCH), the matching
//     X-CSRF-Token header must hash to the session row's csrf_hash.
//     This is the double-submit pattern: cookie + header together prove
//     the request originated from JS we shipped, not from a
//     cross-origin form post.
//
// On success the verified ServiceToken is stamped onto the context so
// handlers can attribute audit log rows to the same actor regardless of
// which arm authenticated them.
//
// last_used_at is bumped after a successful bearer Verify. A failure to
// touch the row is logged-and-continued: auth has already passed, so
// the request still succeeds. The alternative — failing the request —
// would turn a tiny SQL hiccup into an outage. The cookie arm doesn't
// touch last_used_at because a long-lived browser session would mask
// the bearer's actual usage pattern.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Bearer arm — preferred when both headers and cookies are
		// present so CLI requests with stale browser cookies still
		// behave as bearer auth.
		if hdr := r.Header.Get("Authorization"); hdr != "" {
			s.authBearer(w, r, next, hdr)
			return
		}

		// Cookie arm.
		if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
			s.authSession(w, r, next, cookie.Value)
			return
		}

		// Neither arm present → 401, same shape as a missing bearer.
		status, code, msg := httpError(auth.ErrMissingBearer)
		writeError(w, status, code, msg, s.logger)
	})
}

// authBearer handles the Authorization-header arm of authMiddleware.
// Split out so the cookie arm can mirror its shape without nesting four
// levels deep inside one closure.
func (s *Server) authBearer(w http.ResponseWriter, r *http.Request, next http.Handler, header string) {
	plain, err := auth.ParseBearer(header)
	if err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	tokens := store.NewTokenRepo(s.db)
	tok, err := auth.Verify(r.Context(), tokens, plain)
	if err != nil {
		if !errors.Is(err, auth.ErrUnknownToken) && !errors.Is(err, auth.ErrMissingBearer) {
			s.logger.Warn("verify bearer", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if err := tokens.TouchLastUsed(r.Context(), tok.ID); err != nil {
		s.logger.Warn("touch last_used_at", slog.Int64("token_id", tok.ID), slog.String("err", err.Error()))
	}
	next.ServeHTTP(w, r.WithContext(auth.WithToken(r.Context(), tok)))
}

// authSession handles the dashboard session-cookie arm of authMiddleware.
// Mutating methods must additionally present a matching X-CSRF-Token.
func (s *Server) authSession(w http.ResponseWriter, r *http.Request, next http.Handler, cookieValue string) {
	hash := auth.HashToken(cookieValue)
	sessRepo := store.NewSessionRepo(s.db)
	sess, err := sessRepo.ByHash(r.Context(), hash)
	if err != nil {
		// Unknown / expired / revoked session: 401, no body details.
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Warn("session lookup", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(auth.ErrUnknownToken)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if isMutating(r.Method) {
		if err := auth.VerifyCSRF(r.Header.Get(csrfHeader), sess.CSRFHash); err != nil {
			writeError(w, http.StatusForbidden, "csrf_mismatch", "missing or invalid CSRF token", s.logger)
			return
		}
	}
	// Re-load the underlying service token so audit attribution + any
	// per-token policy still works. If the row went away (token was
	// revoked while the session was live), treat it as 401 — the
	// dashboard should re-authenticate.
	tok, err := store.NewTokenRepo(s.db).ByID(r.Context(), sess.TokenID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Warn("session token lookup", slog.Int64("token_id", sess.TokenID), slog.String("err", err.Error()))
		}
		status, code, msg := httpError(auth.ErrUnknownToken)
		writeError(w, status, code, msg, s.logger)
		return
	}
	next.ServeHTTP(w, r.WithContext(auth.WithToken(r.Context(), tok)))
}

// isMutating reports whether method causes server-side state change and
// therefore needs CSRF protection when authenticated by cookie. GET /
// HEAD / OPTIONS read-only requests are exempt.
func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}

// isPublicPath returns true for endpoints that must work without a
// bearer token. /healthz is a docker-compose healthcheck; /bootstrap is
// the one-time admin token issuance flow (it gates itself on "no
// tokens exist yet" instead of on a bearer); POST /dashboard/session
// receives the bearer in the body, not the header, so it cannot be
// gated by authMiddleware without breaking the dashboard login flow.
func isPublicPath(path string) bool {
	switch path {
	case "/healthz", "/api/v1/bootstrap", "/api/v1/dashboard/session":
		return true
	}
	return false
}
