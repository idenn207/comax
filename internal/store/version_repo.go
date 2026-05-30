package store

import (
	"context"
	"fmt"
)

// VersionRepo persists the append-only history of secret values.
type VersionRepo struct{ db DBTX }

// NewVersionRepo wraps db.
func NewVersionRepo(db DBTX) *VersionRepo { return &VersionRepo{db: db} }

// Create appends a version row for secretID. actorToken is the ID of the
// service_token that triggered the change; pass nil for system-initiated
// writes.
//
// version should equal the post-Upsert value returned by
// SecretRepo.Upsert so the rows agree.
func (r *VersionRepo) Create(ctx context.Context, secretID int64, version int64, ciphertext []byte, actorToken *int64) (SecretVersion, error) {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO secret_versions (secret_id, version, ciphertext, actor_token, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		secretID, version, ciphertext, actorToken, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return SecretVersion{}, fmt.Errorf("append version %d for secret %d: %w", version, secretID, ErrConflict)
		}
		return SecretVersion{}, fmt.Errorf("append version for secret %d: %w", secretID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return SecretVersion{}, fmt.Errorf("append version for secret %d: %w", secretID, err)
	}
	return SecretVersion{
		ID:         id,
		SecretID:   secretID,
		Version:    version,
		Ciphertext: ciphertext,
		ActorToken: actorToken,
		CreatedAt:  unixSeconds(now),
	}, nil
}

// ListBySecret returns every historical version for secretID, newest
// first. Empty slice (not ErrNotFound) when there are no rows yet.
func (r *VersionRepo) ListBySecret(ctx context.Context, secretID int64) ([]SecretVersion, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, secret_id, version, ciphertext, actor_token, created_at
		   FROM secret_versions
		  WHERE secret_id = ?
		  ORDER BY version DESC`,
		secretID,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions for secret %d: %w", secretID, err)
	}
	defer rows.Close()

	var out []SecretVersion
	for rows.Next() {
		var (
			v          SecretVersion
			createdAt  int64
			actorToken *int64
		)
		if err := rows.Scan(&v.ID, &v.SecretID, &v.Version, &v.Ciphertext, &actorToken, &createdAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		v.ActorToken = actorToken
		v.CreatedAt = unixSeconds(createdAt)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate versions: %w", err)
	}
	return out, nil
}
