package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// AuditRepo persists immutable audit log entries.
type AuditRepo struct{ db DBTX }

// NewAuditRepo wraps db.
func NewAuditRepo(db DBTX) *AuditRepo { return &AuditRepo{db: db} }

// Append writes a new audit row. action / target are required;
// actorToken is nil for system events, metadata is "" when unused.
func (r *AuditRepo) Append(ctx context.Context, actorToken *int64, action, target, metadata string) (AuditEntry, error) {
	now := nowUnix()
	var metaArg any
	if metadata != "" {
		metaArg = metadata
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor_token, action, target, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		actorToken, action, target, metaArg, now,
	)
	if err != nil {
		return AuditEntry{}, fmt.Errorf("append audit %q: %w", action, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return AuditEntry{}, fmt.Errorf("append audit %q: %w", action, err)
	}
	return AuditEntry{
		ID:         id,
		ActorToken: actorToken,
		Action:     action,
		Target:     target,
		Metadata:   metadata,
		CreatedAt:  unixSeconds(now),
	}, nil
}

// ListRecent returns the most recent limit entries, newest first. limit
// must be > 0; callers cap this at the handler layer.
func (r *AuditRepo) ListRecent(ctx context.Context, limit int) ([]AuditEntry, error) {
	return r.List(ctx, AuditFilter{}, limit)
}

// AuditFilter narrows AuditRepo.List results. Empty / zero fields are
// ignored — passing AuditFilter{} matches every row, recreating the
// ListRecent shape.
//
// Project and Env match against the target string via substring (target
// rows look like "project=app env=dev key=DB_URL"). Action is matched
// exactly. ActorToken matches the FK column directly; pass nil to skip.
// BeforeID is the pagination cursor: rows with id < BeforeID are
// returned, oldest-cursor-rows first. Pass 0 to start from the newest.
type AuditFilter struct {
	Project    string
	Env        string
	Action     string
	ActorToken *int64
	BeforeID   int64
}

// List returns audit entries narrowed by f, newest first, capped at
// limit. limit must be > 0; the handler layer applies the public cap
// (e.g. 200) before calling.
//
// Cursor pagination: pass the smallest id from the previous page as
// f.BeforeID to fetch the next slice. The ordering is "created_at DESC,
// id DESC", so a strict "id < BeforeID" cursor is monotonic without
// needing the created_at column on the cursor.
func (r *AuditRepo) List(ctx context.Context, f AuditFilter, limit int) ([]AuditEntry, error) {
	clauses := []string{}
	args := []any{}
	if f.Project != "" {
		clauses = append(clauses, "target LIKE ?")
		args = append(args, "%project="+f.Project+"%")
	}
	if f.Env != "" {
		clauses = append(clauses, "target LIKE ?")
		args = append(args, "%env="+f.Env+"%")
	}
	if f.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, f.Action)
	}
	if f.ActorToken != nil {
		clauses = append(clauses, "actor_token = ?")
		args = append(args, *f.ActorToken)
	}
	if f.BeforeID > 0 {
		clauses = append(clauses, "id < ?")
		args = append(args, f.BeforeID)
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit)

	q := fmt.Sprintf(
		`SELECT id, actor_token, action, target, metadata, created_at
		   FROM audit_log
		   %s
		  ORDER BY created_at DESC, id DESC
		  LIMIT ?`,
		where,
	)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var (
			e          AuditEntry
			actorToken sql.NullInt64
			metadata   sql.NullString
			createdAt  int64
		)
		if err := rows.Scan(&e.ID, &actorToken, &e.Action, &e.Target, &metadata, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		if actorToken.Valid {
			id := actorToken.Int64
			e.ActorToken = &id
		}
		e.Metadata = metadata.String
		e.CreatedAt = unixSeconds(createdAt)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit: %w", err)
	}
	return out, nil
}
