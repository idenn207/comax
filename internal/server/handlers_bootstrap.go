package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/idenn207/comax-secrets/internal/auth"
)

// bootstrapResponse is what /bootstrap returns. token is shown to the
// operator exactly once — there is no endpoint to re-fetch it.
type bootstrapResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

// handleBootstrap mints the very first admin token. Idempotent only on
// the failure path: a second call when one already exists returns 409
// with code "already_bootstrapped".
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	res, err := auth.Bootstrap(r.Context(), s.db)
	if err != nil {
		if !errors.Is(err, auth.ErrAlreadyBootstrapped) {
			s.logger.Error("bootstrap", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	writeOK(w, http.StatusCreated, bootstrapResponse{
		Token: res.Plaintext,
		Name:  res.Token.Name,
	}, s.logger)
}

// handleHealth is the liveness probe used by docker-compose. No body
// needed; the 200 status is the signal.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, http.StatusOK, map[string]string{"status": "ok"}, s.logger)
}
