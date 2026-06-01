package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/idenn207/comax-secrets/internal/auth"
	"github.com/idenn207/comax-secrets/internal/store"
)

// projectView is the JSON shape we return for projects. Internal IDs
// are exposed because dashboards / SDKs need a stable handle that
// survives a rename when we add one in M2; names alone are not enough.
//
// EnvCount is filled on the list endpoint via ListWithEnvCounts and left
// zero by single-project create/lookup paths — the dashboard's Projects
// grid is the only caller that needs the chip data, and joining on a
// hot-path single-row create adds noise without a reader.
type projectView struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	EnvCount  int64     `json:"env_count"`
}

func newProjectView(p store.Project) projectView {
	return projectView{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt}
}

func newProjectViewWithEnvCount(p store.ProjectWithEnvCount) projectView {
	return projectView{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt, EnvCount: p.EnvCount}
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := store.NewProjectRepo(s.db).ListWithEnvCounts(r.Context())
	if err != nil {
		s.logger.Error("list projects", slog.String("err", err.Error()))
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	views := make([]projectView, 0, len(projects))
	for _, p := range projects {
		views = append(views, newProjectViewWithEnvCount(p))
	}
	writeOK(w, http.StatusOK, views, s.logger)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
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

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		s.logger.Error("begin tx", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	defer func() { _ = tx.Rollback() }()

	p, err := store.NewProjectRepo(tx).Create(r.Context(), body.Name)
	if err != nil {
		if !errors.Is(err, store.ErrConflict) {
			s.logger.Error("create project", slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if err := appendAudit(r, tx, "project.create", fmt.Sprintf("project=%s", p.Name)); err != nil {
		s.logger.Error("audit project.create", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	if err := tx.Commit(); err != nil {
		s.logger.Error("commit project create", slog.String("err", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", s.logger)
		return
	}
	writeOK(w, http.StatusCreated, newProjectView(p), s.logger)
}

// appendAudit writes a row to audit_log under the actor stamped onto
// the request context by authMiddleware. Mutations call this from
// inside their transaction so the audit row commits atomically with
// the data change.
func appendAudit(r *http.Request, tx store.DBTX, action, target string) error {
	var actorID *int64
	if tok, ok := auth.FromContext(r.Context()); ok {
		id := tok.ID
		actorID = &id
	}
	_, err := store.NewAuditRepo(tx).Append(r.Context(), actorID, action, target, "")
	return err
}

// appendAuditForToken is appendAudit's escape hatch for endpoints that
// run *outside* authMiddleware — chiefly POST /dashboard/session, which
// arrives without an Authorization header (the bearer is in the body).
// Callers pass the verified token id explicitly so audit attribution
// stays correct without faking the context stamp.
func appendAuditForToken(r *http.Request, tx store.DBTX, tokenID int64, action, target string) error {
	id := tokenID
	_, err := store.NewAuditRepo(tx).Append(r.Context(), &id, action, target, "")
	return err
}
