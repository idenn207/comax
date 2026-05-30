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

// authMiddleware enforces bearer auth. Paths under publicPrefixes are
// exempt so /healthz and /bootstrap can be reached without credentials.
//
// On success it stamps the verified ServiceToken onto the context via
// auth.WithToken so handlers can attribute audit log rows to the actor.
// On failure it writes a 401 envelope and short-circuits.
//
// last_used_at is bumped after a successful Verify. A failure to touch
// the row is logged-and-continued: auth has already passed, so the
// request still succeeds. The alternative — failing the request — would
// turn a tiny SQL hiccup into an outage.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		plain, err := auth.ParseBearer(r.Header.Get("Authorization"))
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
	})
}

// isPublicPath returns true for endpoints that must work without a
// bearer token. /healthz is a docker-compose healthcheck; /bootstrap is
// the one-time admin token issuance flow (it gates itself on "no
// tokens exist yet" instead of on a bearer).
func isPublicPath(path string) bool {
	switch path {
	case "/healthz", "/api/v1/bootstrap":
		return true
	}
	return false
}
