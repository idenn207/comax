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
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO service_tokens (name, token_hash, created_at)
		 SELECT ?, ?, ?
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
		CreatedAt: unixSeconds(now),
	}, true, nil
}

// Create inserts a new token. Both name and tokenHash must be unique;
// either collision returns ErrConflict.
func (r *TokenRepo) Create(ctx context.Context, name string, tokenHash []byte) (ServiceToken, error) {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO service_tokens (name, token_hash, created_at) VALUES (?, ?, ?)`,
		name, tokenHash, now,
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
		CreatedAt: unixSeconds(now),
	}, nil
}

// ByHash looks up the token whose SHA-256 hash equals tokenHash, or
// returns ErrNotFound.
func (r *TokenRepo) ByHash(ctx context.Context, tokenHash []byte) (ServiceToken, error) {
	var (
		t          ServiceToken
		createdAt  int64
		lastUsedAt sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, token_hash, created_at, last_used_at
		   FROM service_tokens
		  WHERE token_hash = ?`,
		tokenHash,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &createdAt, &lastUsedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServiceToken{}, fmt.Errorf("token lookup: %w", ErrNotFound)
	}
	if err != nil {
		return ServiceToken{}, fmt.Errorf("token lookup: %w", err)
	}
	t.CreatedAt = unixSeconds(createdAt)
	if lastUsedAt.Valid {
		lu := unixSeconds(lastUsedAt.Int64)
		t.LastUsedAt = &lu
	}
	return t, nil
}

// ByID looks up a service token by primary key. Used by the dashboard
// session arm of authMiddleware to re-hydrate the underlying actor from
// a session row's token_id without trusting the cookie further.
func (r *TokenRepo) ByID(ctx context.Context, id int64) (ServiceToken, error) {
	var (
		t          ServiceToken
		createdAt  int64
		lastUsedAt sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, token_hash, created_at, last_used_at
		   FROM service_tokens
		  WHERE id = ?`,
		id,
	).Scan(&t.ID, &t.Name, &t.TokenHash, &createdAt, &lastUsedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServiceToken{}, fmt.Errorf("token %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return ServiceToken{}, fmt.Errorf("token %d: %w", id, err)
	}
	t.CreatedAt = unixSeconds(createdAt)
	if lastUsedAt.Valid {
		lu := unixSeconds(lastUsedAt.Int64)
		t.LastUsedAt = &lu
	}
	return t, nil
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
