package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// TokenRepo persists service tokens (bearer credentials).
//
// Plaintext tokens are NEVER stored. Callers hash with SHA-256 (or
// equivalent) before invoking Create / ByHash. The hash, not the
// plaintext, is what indexes the table and what auth middleware compares
// against on every request.
type TokenRepo struct{ db DBTX }

// NewTokenRepo wraps db.
func NewTokenRepo(db DBTX) *TokenRepo { return &TokenRepo{db: db} }

// Count returns the total number of service tokens. Used by the bootstrap
// flow to guard "POST /bootstrap works only when no tokens exist".
func (r *TokenRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM service_tokens`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count tokens: %w", err)
	}
	return n, nil
}

// BootstrapIfEmpty inserts a token only when the table is empty. The
// check and the insert are a single statement, so concurrent /bootstrap
// calls cannot both succeed: SQLite serialises writes, and the
// WHERE (SELECT COUNT(*) ...) = 0 guard re-evaluates against the
// committed state. Returns (token, true, nil) when the row was created,
// (zero, false, nil) when the table already had tokens, or
// (zero, false, err) on a real driver error. A UNIQUE collision on name
// is folded into the "already bootstrapped" branch as a defensive net.
func (r *TokenRepo) BootstrapIfEmpty(ctx context.Context, name string, tokenHash []byte) (ServiceToken, bool, error) {
	now := nowUnix()
	// The bootstrap token is admin (is_admin=1): it is the only token on a
	// fresh deployment, so it must be able to issue and revoke others.
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO service_tokens (name, token_hash, is_admin, created_at)
		 SELECT ?, ?, 1, ?
		 WHERE (SELECT COUNT(*) FROM service_tokens) = 0`,
		name, tokenHash, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ServiceToken{}, false, nil
		}
		return ServiceToken{}, false, fmt.Errorf("bootstrap token %q: %w", name, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return ServiceToken{}, false, fmt.Errorf("bootstrap token %q: %w", name, err)
	}
	if n == 0 {
		return ServiceToken{}, false, nil
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ServiceToken{}, false, fmt.Errorf("bootstrap token %q: %w", name, err)
	}
	return ServiceToken{
		ID:        id,
		Name:      name,
		TokenHash: tokenHash,
		IsAdmin:   true,
		CreatedAt: unixSeconds(now),
	}, true, nil
}

// Create inserts a new token. Both name and tokenHash must be unique;
// either collision returns ErrConflict. isAdmin grants token-management
// rights — the admin-only issue path (POST /api/v1/tokens) always passes
// false, so an issued CI token can never mint further tokens.
func (r *TokenRepo) Create(ctx context.Context, name string, tokenHash []byte, isAdmin bool) (ServiceToken, error) {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO service_tokens (name, token_hash, is_admin, created_at) VALUES (?, ?, ?, ?)`,
		name, tokenHash, boolToInt(isAdmin), now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ServiceToken{}, fmt.Errorf("create token %q: %w", name, ErrConflict)
		}
		return ServiceToken{}, fmt.Errorf("create token %q: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ServiceToken{}, fmt.Errorf("create token %q: %w", name, err)
	}
	return ServiceToken{
		ID:        id,
		Name:      name,
		TokenHash: tokenHash,
		IsAdmin:   isAdmin,
		CreatedAt: unixSeconds(now),
	}, nil
}

// ByHash looks up the LIVE token whose SHA-256 hash equals tokenHash, or
// returns ErrNotFound. Revoked tokens (revoked_at IS NOT NULL) are
// excluded here: this is the bearer-auth arm, so a revoked credential
// fails authentication exactly as if it never existed.
func (r *TokenRepo) ByHash(ctx context.Context, tokenHash []byte) (ServiceToken, error) {
	var (
		t          ServiceToken
		isAdmin    int64
		createdAt  int64
		lastUsedAt sql.NullInt64
		revokedAt  sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, token_hash, is_admin, created_at, last_used_at, revoked_at
		   FROM service_tokens
		  WHERE token_hash = ? AND revoked_at IS NULL`,
		tokenHash,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &isAdmin, &createdAt, &lastUsedAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServiceToken{}, fmt.Errorf("token lookup: %w", ErrNotFound)
	}
	if err != nil {
		return ServiceToken{}, fmt.Errorf("token lookup: %w", err)
	}
	t.IsAdmin = isAdmin != 0
	t.CreatedAt = unixSeconds(createdAt)
	if lastUsedAt.Valid {
		lu := unixSeconds(lastUsedAt.Int64)
		t.LastUsedAt = &lu
	}
	if revokedAt.Valid {
		rv := unixSeconds(revokedAt.Int64)
		t.RevokedAt = &rv
	}
	return t, nil
}

// ByID looks up a service token by primary key. Used by the dashboard
// session arm of authMiddleware to re-hydrate the underlying actor from
// a session row's token_id without trusting the cookie further.
//
// Unlike ByHash, ByID does NOT filter revoked tokens: the session arm
// must be able to observe RevokedAt so it can terminate a live dashboard
// session whose underlying token was revoked (R2-1). Callers inspect
// tok.RevokedAt themselves.
func (r *TokenRepo) ByID(ctx context.Context, id int64) (ServiceToken, error) {
	var (
		t          ServiceToken
		isAdmin    int64
		createdAt  int64
		lastUsedAt sql.NullInt64
		revokedAt  sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, token_hash, is_admin, created_at, last_used_at, revoked_at
		   FROM service_tokens
		  WHERE id = ?`,
		id,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &isAdmin, &createdAt, &lastUsedAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServiceToken{}, fmt.Errorf("token %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return ServiceToken{}, fmt.Errorf("token %d: %w", id, err)
	}
	t.IsAdmin = isAdmin != 0
	t.CreatedAt = unixSeconds(createdAt)
	if lastUsedAt.Valid {
		lu := unixSeconds(lastUsedAt.Int64)
		t.LastUsedAt = &lu
	}
	if revokedAt.Valid {
		rv := unixSeconds(revokedAt.Int64)
		t.RevokedAt = &rv
	}
	return t, nil
}

// List returns every service token ordered by id. The token_hash column
// is deliberately NOT selected — a listing never needs the credential
// digest, so it never leaves the store. Live and revoked tokens are both
// returned; callers render RevokedAt to show status.
func (r *TokenRepo) List(ctx context.Context) ([]ServiceToken, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, is_admin, created_at, last_used_at, revoked_at
		   FROM service_tokens
		  ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()

	var out []ServiceToken
	for rows.Next() {
		var (
			t          ServiceToken
			isAdmin    int64
			createdAt  int64
			lastUsedAt sql.NullInt64
			revokedAt  sql.NullInt64
		)
		if err := rows.Scan(&t.ID, &t.Name, &isAdmin, &createdAt, &lastUsedAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("list tokens: scan: %w", err)
		}
		t.IsAdmin = isAdmin != 0
		t.CreatedAt = unixSeconds(createdAt)
		if lastUsedAt.Valid {
			lu := unixSeconds(lastUsedAt.Int64)
			t.LastUsedAt = &lu
		}
		if revokedAt.Valid {
			rv := unixSeconds(revokedAt.Int64)
			t.RevokedAt = &rv
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tokens: rows: %w", err)
	}
	return out, nil
}

// Revoke soft-revokes a token by stamping revoked_at=now. It is
// deliberately NOT idempotent: revoking an already-revoked or absent
// token returns ErrNotFound. The WHERE revoked_at IS NULL guard is what
// makes a second revoke affect zero rows and surface ErrNotFound, so the
// caller can distinguish "revoked just now" from "nothing to revoke".
func (r *TokenRepo) Revoke(ctx context.Context, id int64) error {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`UPDATE service_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("revoke token %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke token %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("token %d: %w", id, ErrNotFound)
	}
	return nil
}

// CountLiveAdmins returns the number of admin tokens that are not revoked.
// Callers use it to refuse revoking the last live admin: a soft-revoked
// admin keeps is_admin=1, so neither the migration backfill nor
// BootstrapIfEmpty could restore issuing rights afterward.
func (r *TokenRepo) CountLiveAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_tokens WHERE is_admin = 1 AND revoked_at IS NULL`,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count live admins: %w", err)
	}
	return n, nil
}

// TouchLastUsed updates the token's last_used_at to now. Returns
// ErrNotFound if id does not match a row.
func (r *TokenRepo) TouchLastUsed(ctx context.Context, id int64) error {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`UPDATE service_tokens SET last_used_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("touch token %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("touch token %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("token %d: %w", id, ErrNotFound)
	}
	return nil
}
