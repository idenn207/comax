package server

import "net/http"

// Handler returns the assembled http.Handler. Wrapping order matters:
//
//   recover ← log ← auth ← mux
//
// Innermost is the mux. Auth runs next so handlers see a stamped
// context, log records the response status after both succeed, and
// recover is the outermost ring so a panic anywhere still gets a 500
// envelope.
//
// Go 1.22+'s ServeMux supports "METHOD /path/{param}" patterns, so we
// don't pull in chi or gorilla/mux. PathValue lookups happen in the
// handlers.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /api/v1/bootstrap", s.handleBootstrap)

	mux.HandleFunc("POST /api/v1/dashboard/session", s.handleCreateDashboardSession)
	mux.HandleFunc("DELETE /api/v1/dashboard/session", s.handleRevokeDashboardSession)
	mux.HandleFunc("GET /api/v1/dashboard/sessions", s.handleListDashboardSessions)
	mux.HandleFunc("DELETE /api/v1/dashboard/sessions/{id}", s.handleRevokeDashboardSessionByID)

	mux.HandleFunc("GET /api/v1/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/v1/projects", s.handleCreateProject)

	// Service-token management (admin-only; enforced inside each handler
	// via requireAdmin). POST returns the plaintext exactly once.
	mux.HandleFunc("POST /api/v1/tokens", s.handleCreateToken)
	mux.HandleFunc("GET /api/v1/tokens", s.handleListTokens)
	mux.HandleFunc("DELETE /api/v1/tokens/{id}", s.handleRevokeToken)

	mux.HandleFunc("GET /api/v1/projects/{p}/envs", s.handleListEnvs)
	mux.HandleFunc("POST /api/v1/projects/{p}/envs", s.handleCreateEnv)
	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/diff", s.handleDiffEnvs)

	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/secrets", s.handleListSecrets)
	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/secrets/{k}", s.handleGetSecret)
	mux.HandleFunc("PUT /api/v1/projects/{p}/envs/{e}/secrets/{k}", s.handlePutSecret)
	mux.HandleFunc("DELETE /api/v1/projects/{p}/envs/{e}/secrets/{k}", s.handleDeleteSecret)
	mux.HandleFunc("POST /api/v1/projects/{p}/envs/{e}/secrets/{k}/rollback", s.handleRollbackSecret)
	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/secrets/{k}/versions/{v}", s.handleGetVersion)

	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/versions", s.handleListVersions)

	mux.HandleFunc("GET /api/v1/audit", s.handleListAudit)

	// SPA fallthrough. Go's ServeMux (1.22+) chooses the more specific
	// pattern, so explicit /api/* and /healthz patterns above still win.
	// cspMiddleware wraps only this handler — /api responses are JSON,
	// never executed in a browsing context, and don't need the CSP
	// nonce. handleSPA returns the envelope-shape 404 when the dashboard
	// is not embedded so unknown paths still look like every other 404.
	mux.Handle("/", s.cspMiddleware(http.HandlerFunc(s.handleSPA)))

	return s.recoverMiddleware(s.logMiddleware(s.authMiddleware(mux)))
}
