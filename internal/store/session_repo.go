package store

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SessionRepo persists browser sessions for the dashboard. Each row is
// a SHA-256 of the cookie value plus a SHA-256 of the matching CSRF
// token. The plaintexts of both are emitted to the dashboard exactly
// once, at /api/v1/dashboard/session creation time.
type SessionRepo struct{ db DBTX }

// NewSessionRepo wraps db.
func NewSessionRepo(db DBTX) *SessionRepo { return &SessionRepo{db: db} }

// SessionCreateInput is the bundle CreateInput consumes. The hashes are
// caller-computed (see internal/auth.HashToken / HashCSRF) so this repo
// never sees the plaintext credentials.
type SessionCreateInput struct {
	TokenID     int64
	SessionHash []byte
	CSRFHash    []byte
	UserAgent   string
	IPPrefix    string
	TTL         time.Duration
}

// Create inserts a new dashboard session and returns the persisted row.
// The expires_at is computed from time.Now() + TTL so callers don't have
// to thread a clock through their request handlers.
func (r *SessionRepo) Create(ctx context.Context, in SessionCreateInput) (DashboardSession, error) {
	now := nowUnix()
	exp := now + int64(in.TTL.Seconds())
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO dashboard_sessions
		   (token_id, session_hash, csrf_hash, user_agent, ip_prefix, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.TokenID, in.SessionHash, in.CSRFHash, in.UserAgent, in.IPPrefix, now, exp,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return DashboardSession{}, fmt.Errorf("create session: %w", ErrConflict)
		}
		return DashboardSession{}, fmt.Errorf("create session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return DashboardSession{}, fmt.Errorf("create session: %w", err)
	}
	return DashboardSession{
		ID:          id,
		TokenID:     in.TokenID,
		SessionHash: in.SessionHash,
		CSRFHash:    in.CSRFHash,
		UserAgent:   in.UserAgent,
		IPPrefix:    in.IPPrefix,
		CreatedAt:   unixSeconds(now),
		ExpiresAt:   unixSeconds(exp),
	}, nil
}

// ByHash returns the active session whose session_hash matches the
// given hash. "Active" means revoked_at IS NULL and expires_at > now.
// Returns ErrNotFound when no live row matches.
//
// A constant-time compare on the returned hash defends against a future
// driver regression where the WHERE clause might shortcut on a partial
// match.
func (r *SessionRepo) ByHash(ctx context.Context, sessionHash []byte) (DashboardSession, error) {
	now := nowUnix()
	var (
		s                                          DashboardSession
		userAgent, ipPrefix                        sql.NullString
		createdAt, expiresAt                       int64
		revokedAt                                  sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, token_id, session_hash, csrf_hash, user_agent, ip_prefix,
		        created_at, expires_at, revoked_at
		   FROM dashboard_sessions
		  WHERE session_hash = ? AND revoked_at IS NULL AND expires_at > ?`,
		sessionHash, now,
	).Scan(&s.ID, &s.TokenID, &s.SessionHash, &s.CSRFHash,
		&userAgent, &ipPrefix, &createdAt, &expiresAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DashboardSession{}, fmt.Errorf("session lookup: %w", ErrNotFound)
	}
	if err != nil {
		return DashboardSession{}, fmt.Errorf("session lookup: %w", err)
	}
	// Defence-in-depth: never trust the WHERE clause alone.
	if subtle.ConstantTimeCompare(s.SessionHash, sessionHash) != 1 {
		return DashboardSession{}, fmt.Errorf("session lookup: %w", ErrNotFound)
	}
	s.UserAgent = userAgent.String
	s.IPPrefix = ipPrefix.String
	s.CreatedAt = unixSeconds(createdAt)
	s.ExpiresAt = unixSeconds(expiresAt)
	if revokedAt.Valid {
		t := unixSeconds(revokedAt.Int64)
		s.RevokedAt = &t
	}
	return s, nil
}

// Revoke marks the session row as revoked. Returns ErrNotFound when no
// live row matches (either the id is unknown or it was already revoked);
// the caller treats both as a no-op for idempotency.
func (r *SessionRepo) Revoke(ctx context.Context, id int64) error {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`UPDATE dashboard_sessions
		    SET revoked_at = ?
		  WHERE id = ? AND revoked_at IS NULL`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("revoke session %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke session %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("revoke session %d: %w", id, ErrNotFound)
	}
	return nil
}

// Prune deletes rows that are either revoked or expired before cutoff.
// Intended for a background sweeper goroutine; callers pass time.Now()
// minus a grace window so a barely-expired session has a moment to be
// observed by audit tooling.
func (r *SessionRepo) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM dashboard_sessions
		  WHERE (revoked_at IS NOT NULL AND revoked_at < ?)
		     OR (expires_at < ?)`,
		cutoff.UTC().Unix(), cutoff.UTC().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("prune sessions: %w", err)
	}
	return res.RowsAffected()
}
