// Package server is the HTTP API for secret-server.
//
// The package establishes the Milestone-1 conventions later milestones
// must follow: response envelope shape (per ECC common/patterns.md),
// error → status mapping, bearer auth middleware, and the
// audit-on-every-mutation pattern.
package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/idenn207/comax-secrets/internal/auth"
	"github.com/idenn207/comax-secrets/internal/store"
)

// Envelope is the shape every JSON response carries. Either Data or
// Error is populated; meta is optional pagination/context. Locking this
// down in M1 lets the dashboard (M2), GitHub Action (M3), and SDK (M5)
// parse responses without each inventing a private convention.
type Envelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
	Meta  any    `json:"meta,omitempty"`
}

// Error is the structured failure payload. Code is a machine-readable
// identifier (e.g. "not_found", "conflict"); Message is human-readable.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serialises body as JSON with the given status. Encoding
// errors are logged and silently dropped: the response headers are
// already flushed at this point so we cannot recover by writing a
// different status.
func writeJSON(w http.ResponseWriter, status int, body any, logger *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil && logger != nil {
		logger.Error("encode response", slog.String("err", err.Error()))
	}
}

// writeOK wraps data in a success envelope and writes status.
func writeOK(w http.ResponseWriter, status int, data any, logger *slog.Logger) {
	writeJSON(w, status, Envelope{OK: true, Data: data}, logger)
}

// writeError writes an error envelope with the given status.
func writeError(w http.ResponseWriter, status int, code, message string, logger *slog.Logger) {
	writeJSON(w, status, Envelope{OK: false, Error: &Error{Code: code, Message: message}}, logger)
}

// httpError maps an internal error to (status, code, message). The
// router calls this from every handler so the response shape stays
// consistent across resources.
//
// The mapping is intentionally narrow: known sentinel → known status,
// everything else → 500. We log the underlying error inside the
// handler before calling httpError; the client only sees the code.
func httpError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound, "not_found", "resource not found"
	case errors.Is(err, store.ErrConflict):
		return http.StatusConflict, "conflict", "resource already exists"
	case errors.Is(err, auth.ErrAlreadyBootstrapped):
		return http.StatusConflict, "already_bootstrapped", "server is already bootstrapped"
	case errors.Is(err, auth.ErrMissingBearer), errors.Is(err, auth.ErrInvalidBearer):
		return http.StatusUnauthorized, "unauthorized", "missing or malformed bearer token"
	case errors.Is(err, auth.ErrUnknownToken):
		return http.StatusUnauthorized, "unauthorized", "unknown bearer token"
	case errors.Is(err, errBadRequest):
		return http.StatusBadRequest, "bad_request", err.Error()
	default:
		return http.StatusInternalServerError, "internal", "internal server error"
	}
}

// errBadRequest is the sentinel returned by handlers when input
// validation fails. Callers do `fmt.Errorf("name: %w", errBadRequest)`
// so the message carries the field-specific detail without exposing
// internal error chains.
var errBadRequest = errors.New("bad request")
