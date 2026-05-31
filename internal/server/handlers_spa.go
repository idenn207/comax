package server

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

const (
	// spaIndexFile is the SPA shell that the React Router falls back to
	// for any client-side route.
	spaIndexFile = "index.html"

	// cspNoncePlaceholder is the literal string Vite emits inside
	// <script nonce="…"> tags. handleSPA swaps it for the per-request
	// nonce produced by cspMiddleware before writing the response body.
	cspNoncePlaceholder = "__CSP_NONCE__"

	// cacheImmutable is for content-hashed assets (Vite emits
	// /assets/<name>-<hash>.{js,css}); they can be cached forever
	// because any change produces a different filename.
	cacheImmutable = "public, max-age=31536000, immutable"

	// cacheNoStore is for the SPA shell itself so a deploy is picked up
	// on the next refresh without waiting for a CDN TTL.
	cacheNoStore = "no-store"

	// cacheShortTTL is a conservative default for unhashed root files
	// like favicon.ico that change rarely but should not be cached
	// "forever".
	cacheShortTTL = "public, max-age=3600"
)

// handleSPA is the router's last fallthrough: any path that did not
// match a registered /api/v1/* or /healthz pattern lands here.
//
// Routing decisions:
//
//   - Only GET / HEAD. Other methods get the same envelope-shape 404 as
//     any unknown route (POST /unknown is not a useful surface to expose,
//     and Method-Not-Allowed would imply a route exists at that path).
//   - If no dashboard is embedded (s.spaFS == nil, either dev mode or
//     --dashboard-enabled=false), the response is the same envelope 404.
//     /api routes still work because they were registered as explicit
//     patterns earlier.
//   - If the requested path resolves to a real file, serve it with the
//     long-lived cache header (Vite hashes asset filenames).
//   - Otherwise the request is a client-side route (e.g. /login) —
//     fall back to index.html and let React Router pick the view.
//   - index.html is always served with no-store + the per-request CSP
//     nonce substituted in, so a deploy lands on the next refresh and
//     the browser never executes a script Vite did not stamp with our
//     nonce.
func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusNotFound, "not_found", "no such route", s.logger)
		return
	}
	if s.spaFS == nil {
		writeError(w, http.StatusNotFound, "not_found", "no such route", s.logger)
		return
	}

	// path.Clean collapses "//" and "/./" so the FS lookup is canonical.
	cleaned := path.Clean(r.URL.Path)
	if cleaned == "/" || cleaned == "." {
		s.serveSPAIndex(w, r)
		return
	}
	if strings.HasPrefix(cleaned, "..") {
		// Defense-in-depth — io/fs would reject this too, but we'd
		// rather not exercise its error path.
		writeError(w, http.StatusNotFound, "not_found", "no such route", s.logger)
		return
	}

	fsPath := strings.TrimPrefix(cleaned, "/")
	f, err := s.spaFS.Open(fsPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.serveSPAIndex(w, r)
			return
		}
		s.logger.Warn("spa open", slog.String("path", fsPath), slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "spa", s.logger)
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		s.logger.Warn("spa stat", slog.String("path", fsPath), slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "spa", s.logger)
		return
	}
	if info.IsDir() {
		// Directory hits without an explicit file → SPA fallback so the
		// client-side router handles trailing-slash routes.
		s.serveSPAIndex(w, r)
		return
	}

	if strings.HasPrefix(fsPath, "assets/") {
		w.Header().Set("Cache-Control", cacheImmutable)
	} else {
		w.Header().Set("Cache-Control", cacheShortTTL)
	}
	http.ServeFileFS(w, r, s.spaFS, fsPath)
}

// serveSPAIndex writes the SPA shell with no-store + the per-request
// CSP nonce. The nonce comes from cspMiddleware via the request context;
// when the middleware is bypassed (e.g. tests) the placeholder is left
// in place — the browser would refuse to execute it, which is the
// fail-closed behavior we want.
func (s *Server) serveSPAIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.spaFS, spaIndexFile)
	if err != nil {
		// No index.html present — happens in the .gitkeep-only state
		// before `make dashboard` has run. Behave as "no dashboard
		// available" so the operator gets the same envelope-shape 404
		// as a stale link.
		writeError(w, http.StatusNotFound, "not_found", "no such route", s.logger)
		return
	}
	nonce, _ := r.Context().Value(nonceCtxKey{}).(string)
	body := strings.ReplaceAll(string(data), cspNoncePlaceholder, nonce)

	w.Header().Set("Cache-Control", cacheNoStore)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(w, body)
}
