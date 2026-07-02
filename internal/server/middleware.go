package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
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
	// R2-1: a soft revoke must terminate live dashboard sessions too, not
	// just the bearer arm. ByHash already excludes revoked tokens, but the
	// session arm re-hydrates via ByID (which deliberately returns revoked
	// rows) — so we check RevokedAt here and 401 to force re-auth. Without
	// this, a revoked admin could keep operating through an open browser tab.
	if tok.RevokedAt != nil {
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
// bearer token.
//
// Two arms:
//
//  1. Any path outside the /api/ tree is public. This covers the SPA
//     shell (/, /login, /assets/...) and /healthz — none of which can
//     require auth without breaking the login flow itself or the
//     compose healthcheck.
//
//  2. Inside /api/, only the bootstrap and session-create endpoints
//     are public. Bootstrap gates itself on "no tokens exist yet";
//     POST /dashboard/session receives the bearer in the body, not
//     the header, so it cannot pass through authMiddleware without
//     breaking the dashboard login flow.
//
// Note: SPA paths are intentionally public; the /api endpoints they
// invoke from JS are not. The SPA holds nothing sensitive until the
// dashboard cookie is set.
func isPublicPath(path string) bool {
	if !strings.HasPrefix(path, "/api/") {
		return true
	}
	switch path {
	case "/api/v1/bootstrap", "/api/v1/dashboard/session":
		return true
	}
	return false
}

// nonceCtxKey is the typed key cspMiddleware uses to stamp the per-
// request CSP nonce onto the context. handlers_spa.go reads it to
// substitute the placeholder in index.html.
type nonceCtxKey struct{}

// cspMiddleware generates a per-request nonce, sets the
// Content-Security-Policy header, and stamps the nonce onto the request
// context. It is intentionally narrow in scope: the router wraps it
// only around the SPA handler (the /api/* JSON responses do not need
// CSP because they are not HTML and never execute scripts in a
// browsing context).
//
// Failing nonce generation is treated as fatal for the request: the
// browser would otherwise execute scripts the dashboard did not stamp.
func (s *Server) cspMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce, err := generateCSPNonce()
		if err != nil {
			s.logger.Error("csp nonce", slog.String("err", err.Error()))
			writeError(w, http.StatusInternalServerError, "internal", "csp nonce", s.logger)
			return
		}
		w.Header().Set("Content-Security-Policy", buildCSP(nonce))
		ctx := context.WithValue(r.Context(), nonceCtxKey{}, nonce)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateCSPNonce returns a 128-bit base64 nonce. base64.RawStdEncoding
// avoids the trailing "=" padding so the nonce is safe to drop into an
// HTML attribute without escaping.
func generateCSPNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

// buildCSP renders the full CSP header value for an SPA response.
//
// Allowances:
//
//   - script-src: 'self' + per-request nonce. Vite emits module scripts
//     from /assets/ which 'self' covers; the nonce lets the SPA shell
//     bootstrap from index.html without resorting to 'unsafe-inline'.
//   - style-src: 'self' 'unsafe-inline' — Radix and many CSS-in-JS
//     libraries inject inline style attributes. v1 trades strict style
//     CSP for ergonomic component libraries; style-based exfiltration
//     attacks are orders of magnitude weaker than script-based ones.
//   - img-src includes data: so SVG/PNG data URLs in the SPA work.
//   - connect-src 'self' restricts fetch/XHR back to this origin
//     (the dashboard never talks to a 3rd-party API).
//   - frame-src 'none' + object-src 'none' + base-uri 'self' close
//     common XSS escape hatches.
func buildCSP(nonce string) string {
	return "default-src 'self'; " +
		"script-src 'self' 'nonce-" + nonce + "'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"frame-src 'none'; " +
		"object-src 'none'; " +
		"base-uri 'self'"
}
