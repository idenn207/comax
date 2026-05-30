package store

import (
	"context"
	"database/sql"
	"fmt"
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
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, actor_token, action, target, metadata, created_at
		   FROM audit_log
		  ORDER BY created_at DESC, id DESC
		  LIMIT ?`,
		limit,
	)
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
