package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
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

// envDiff is the response payload for handleDiffEnvs. lhs is the env in
// the URL path; rhs is the env named by `?against=<name>`. Each list is
// sorted alphabetically so the dashboard renders deterministically.
type envDiff struct {
	Lhs     string           `json:"lhs"`
	Rhs     string           `json:"rhs"`
	Added   []string         `json:"added"`   // keys present in lhs but not rhs
	Removed []string         `json:"removed"` // keys present in rhs but not lhs
	Changed []envDiffChanged `json:"changed"` // keys present in both with different resolved plaintext
}

// envDiffChanged carries the per-key version numbers on each side so the
// dashboard can drill into the specific historical version of either
// side via GET .../versions/{v}. Plaintext is intentionally omitted —
// fetching values is a separate, per-row click.
type envDiffChanged struct {
	Key        string `json:"key"`
	LhsVersion int64  `json:"lhs_version"`
	RhsVersion int64  `json:"rhs_version"`
}

// handleDiffEnvs returns the set-and-value diff between two envs in the
// same project, with inheritance applied to both sides before the
// comparison. "Changed" is judged by the post-resolver plaintext so two
// envs holding the same key with re-encrypted ciphertext (different
// nonce, same plaintext) correctly diff as equal.
func (s *Server) handleDiffEnvs(w http.ResponseWriter, r *http.Request) {
	p, e, ok := s.resolveEnv(w, r)
	if !ok {
		return
	}
	againstName := r.URL.Query().Get("against")
	if err := validateName("against", againstName); err != nil {
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	if againstName == e.Name {
		writeError(w, http.StatusBadRequest, "bad_request", "against env must differ from path env", s.logger)
		return
	}
	e2, err := store.NewEnvRepo(s.db).ByName(r.Context(), p.ID, againstName)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Error("lookup against env", slog.String("env", againstName), slog.String("err", err.Error()))
		}
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	lhsSnap, err := s.resolver.Resolve(r.Context(), p.ID, e.ID)
	if err != nil {
		s.logger.Warn("resolve lhs env", slog.String("env", e.Name), slog.String("err", err.Error()))
		status, code, msg := resolverErrorToHTTP(err)
		writeError(w, status, code, msg, s.logger)
		return
	}
	rhsSnap, err := s.resolver.Resolve(r.Context(), p.ID, e2.ID)
	if err != nil {
		s.logger.Warn("resolve rhs env", slog.String("env", e2.Name), slog.String("err", err.Error()))
		status, code, msg := resolverErrorToHTTP(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	added := make([]string, 0)
	changed := make([]envDiffChanged, 0)
	for k, lhsV := range lhsSnap {
		rhsV, ok := rhsSnap[k]
		if !ok {
			added = append(added, k)
			continue
		}
		if rhsV.Value != lhsV.Value {
			changed = append(changed, envDiffChanged{
				Key:        k,
				LhsVersion: lhsV.Version,
				RhsVersion: rhsV.Version,
			})
		}
	}
	removed := make([]string, 0)
	for k := range rhsSnap {
		if _, ok := lhsSnap[k]; !ok {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Slice(changed, func(i, j int) bool { return changed[i].Key < changed[j].Key })

	writeOK(w, http.StatusOK, envDiff{
		Lhs:     e.Name,
		Rhs:     e2.Name,
		Added:   added,
		Removed: removed,
		Changed: changed,
	}, s.logger)
}
