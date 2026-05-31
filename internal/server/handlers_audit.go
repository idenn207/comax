package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/idenn207/comax-secrets/internal/store"
)

// auditView is the JSON shape returned by handleListAudit. We expose
// actor_token_id (not the bearer itself) so the dashboard can show "who
// did this" without ever surfacing a token plaintext.
type auditView struct {
	ID           int64     `json:"id"`
	Action       string    `json:"action"`
	Target       string    `json:"target"`
	Metadata     string    `json:"metadata,omitempty"`
	ActorTokenID *int64    `json:"actor_token_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// auditMeta carries the cursor for the next page. Empty when the
// current page filled fewer rows than the limit (= no more pages).
type auditMeta struct {
	NextBefore int64 `json:"next_before,omitempty"`
	Limit      int   `json:"limit"`
}

const (
	auditDefaultLimit = 50
	auditMaxLimit     = 200
)

// handleListAudit returns paginated audit entries newest-first, with
// optional project/env/actor/action filters and an `before=<id>` cursor.
//
// Project/env match the canonical target format ("project=X env=Y ...")
// via substring LIKE — names that share a prefix (e.g. "foo" and
// "foobar") can collide. M2 v1 accepts that; a future schema split into
// dedicated target_project/target_env columns will tighten this.
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	qp := r.URL.Query()

	project := qp.Get("project")
	if project != "" {
		if err := validateName("project", project); err != nil {
			status, code, msg := httpError(err)
			writeError(w, status, code, msg, s.logger)
			return
		}
	}
	env := qp.Get("env")
	if env != "" {
		if err := validateName("env", env); err != nil {
			status, code, msg := httpError(err)
			writeError(w, status, code, msg, s.logger)
			return
		}
	}
	// Action is dotted ("secret.upsert"), not a name — skip validateName.

	filter := store.AuditFilter{
		Project: project,
		Env:     env,
		Action:  qp.Get("action"),
	}
	if a := qp.Get("actor"); a != "" {
		v, err := strconv.ParseInt(a, 10, 64)
		if err != nil || v <= 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "actor must be a positive integer", s.logger)
			return
		}
		filter.ActorToken = &v
	}
	if b := qp.Get("before"); b != "" {
		v, err := strconv.ParseInt(b, 10, 64)
		if err != nil || v <= 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "before must be a positive integer", s.logger)
			return
		}
		filter.BeforeID = v
	}

	limit := auditDefaultLimit
	if l := qp.Get("limit"); l != "" {
		v, err := strconv.Atoi(l)
		if err != nil || v <= 0 {
			writeError(w, http.StatusBadRequest, "bad_request", "limit must be a positive integer", s.logger)
			return
		}
		limit = v
	}
	if limit > auditMaxLimit {
		limit = auditMaxLimit
	}

	entries, err := store.NewAuditRepo(s.db).List(r.Context(), filter, limit)
	if err != nil {
		s.logger.Error("list audit", slog.String("err", err.Error()))
		status, code, msg := httpError(err)
		writeError(w, status, code, msg, s.logger)
		return
	}

	views := make([]auditView, 0, len(entries))
	for _, e := range entries {
		views = append(views, auditView{
			ID:           e.ID,
			Action:       e.Action,
			Target:       e.Target,
			Metadata:     e.Metadata,
			ActorTokenID: e.ActorToken,
			CreatedAt:    e.CreatedAt,
		})
	}

	meta := auditMeta{Limit: limit}
	if len(views) == limit && len(views) > 0 {
		meta.NextBefore = views[len(views)-1].ID
	}

	writeJSON(w, http.StatusOK, Envelope{OK: true, Data: views, Meta: meta}, s.logger)
}
