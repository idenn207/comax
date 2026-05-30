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

	mux.HandleFunc("GET /api/v1/projects", s.handleListProjects)
	mux.HandleFunc("POST /api/v1/projects", s.handleCreateProject)

	mux.HandleFunc("GET /api/v1/projects/{p}/envs", s.handleListEnvs)
	mux.HandleFunc("POST /api/v1/projects/{p}/envs", s.handleCreateEnv)

	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/secrets", s.handleListSecrets)
	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/secrets/{k}", s.handleGetSecret)
	mux.HandleFunc("PUT /api/v1/projects/{p}/envs/{e}/secrets/{k}", s.handlePutSecret)

	mux.HandleFunc("GET /api/v1/projects/{p}/envs/{e}/versions", s.handleListVersions)

	// 404 for everything else, in our envelope shape.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such route", s.logger)
	})

	return s.recoverMiddleware(s.logMiddleware(s.authMiddleware(mux)))
}
