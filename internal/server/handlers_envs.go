package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/idenn207/comax-secrets/internal/store"
)

type envView struct {
	ID           int64     `json:"id"`
	ProjectID    int64     `json:"project_id"`
	Name         string    `json:"name"`
	InheritsFrom string    `json:"inherits_from,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func newEnvView(e store.Environment) envView {
	return envView{
		ID:           e.ID,
		ProjectID:    e.ProjectID,
		Name:         e.Name,
		InheritsFrom: e.InheritsFrom,
		CreatedAt:    e.CreatedAt,
	}
}

// resolveProject looks up the project by URL path name and writes a 404
// envelope if missing. The lookup happens on every env/secret endpoint
// because the URL is the only place the project is named.
func (s *Server) resolveProject(w http.ResponseWriter, r *http.Request) (store.Project, bool) {
	name := r.PathValue("p")
	if err := validateName("project", name); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return store.Project{}, false
	}
	p, err := store.NewProjectRepo(s.db).ByName(r.Context(), name)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("lookup project", slog.String("name", name), slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return store.Project{}, false
	}
	return p, true
}

func (s *Server) handleListEnvs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	envs, err := store.NewEnvRepo(s.db).ListByProject(r.Context(), p.ID)
	if err != nil {
		s.logger.Error("list envs", slog.String("err", err.Error()))
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	views := make([]envView, 0, len(envs))
	for _, e := range envs {
		views = append(views, newEnvView(e))
	}
	writeOK(w, http.StatusOK, views, s.logger)
}

func (s *Server) handleCreateEnv(w http.ResponseWriter, r *http.Request) {
	p, ok := s.resolveProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Name         string `json:"name"`
		InheritsFrom string `json:"inherits_from"`
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
	if body.InheritsFrom != "" {
		if err := validateName("inherits_from", body.InheritsFrom); err != nil {
			status, code, msg := httpError(err)
			writeError(w, status, code, msg, s.logger)
			return
		}
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	e, err := store.NewEnvRepo(tx).Create(r.Context(), p.ID, body.Name, body.InheritsFrom)
	if err != nil {
		if !errors.Is(err, store.ErrConflict) {
			s.logger.Error("create env", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if err := appendAudit(r, tx, "env.create", fmt.Sprintf("project=%s env=%s", p.Name, e.Name)); err != nil {
		s.logger.Error("audit env.create", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("commit env create", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	writeOK(w, http.StatusCreated, newEnvView(e), s.logger)
}
