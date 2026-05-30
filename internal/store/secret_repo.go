package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SecretRepo persists encrypted secrets keyed by (env, key).
type SecretRepo struct{ db DBTX }

// NewSecretRepo wraps db.
func NewSecretRepo(db DBTX) *SecretRepo { return &SecretRepo{db: db} }

// UpsertResult is what Upsert returns. Created is true when the row was
// newly inserted, false when an existing row was updated.
type UpsertResult struct {
	Secret  Secret
	Created bool
}

// Upsert writes ciphertext for (envID, key). On insert, version starts
// at 1. On update, version is incremented. The returned Secret reflects
// post-write state.
//
// Atomicity note: this method performs only the secrets-table mutation.
// Writing a corresponding row to secret_versions and audit_log is the
// caller's responsibility — typically inside the same *sql.Tx — so that
// Task 5 handlers can compose Upsert with VersionRepo.Create and
// AuditRepo.Create in one BeginTx scope.
func (r *SecretRepo) Upsert(ctx context.Context, envID int64, key string, ciphertext []byte) (UpsertResult, error) {
	now := nowUnix()
	// ON CONFLICT DO UPDATE handles the race between two PUTs for the same
	// key. RETURNING gives us the post-write id + version + created_at so
	// we can detect insert vs update without a second roundtrip.
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO secrets (env_id, key, ciphertext, version, created_at, updated_at)
		 VALUES (?, ?, ?, 1, ?, ?)
		 ON CONFLICT(env_id, key) DO UPDATE SET
		     ciphertext = excluded.ciphertext,
		     version    = secrets.version + 1,
		     updated_at = excluded.updated_at
		 RETURNING id, version, created_at`,
		envID, key, ciphertext, now, now,
	)
	var (
		id, version, createdAt int64
	)
	if err := row.Scan(&id, &version, &createdAt); err != nil {
		return UpsertResult{}, fmt.Errorf("upsert secret %q in env %d: %w", key, envID, err)
	}
	return UpsertResult{
		Secret: Secret{
			ID:         id,
			EnvID:      envID,
			Key:        key,
			Ciphertext: ciphertext,
			Version:    version,
			CreatedAt:  unixSeconds(createdAt),
			UpdatedAt:  unixSeconds(now),
		},
		// version == 1 AND created_at == now is the unambiguous insert
		// signature: an update would either bump version above 1 or
		// preserve an older created_at.
		Created: version == 1 && createdAt == now,
	}, nil
}

// ByKey returns the current secret for (envID, key), or ErrNotFound.
func (r *SecretRepo) ByKey(ctx context.Context, envID int64, key string) (Secret, error) {
	var (
		s                    Secret
		createdAt, updatedAt int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, env_id, key, ciphertext, version, created_at, updated_at
		   FROM secrets
		  WHERE env_id = ? AND key = ?`,
		envID, key,
	).Scan(&s.ID, &s.EnvID, &s.Key, &s.Ciphertext, &s.Version, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Secret{}, fmt.Errorf("secret %q in env %d: %w", key, envID, ErrNotFound)
	}
	if err != nil {
		return Secret{}, fmt.Errorf("lookup secret %q in env %d: %w", key, envID, err)
	}
	s.CreatedAt = unixSeconds(createdAt)
	s.UpdatedAt = unixSeconds(updatedAt)
	return s, nil
}

// ListByEnv returns every current secret in envID, ordered by key for
// deterministic .env emission.
func (r *SecretRepo) ListByEnv(ctx context.Context, envID int64) ([]Secret, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, env_id, key, ciphertext, version, created_at, updated_at
		   FROM secrets
		  WHERE env_id = ?
		  ORDER BY key`,
		envID,
	)
	if err != nil {
		return nil, fmt.Errorf("list secrets for env %d: %w", envID, err)
	}
	defer rows.Close()

	var out []Secret
	for rows.Next() {
		var (
			s                    Secret
			createdAt, updatedAt int64
		)
		if err := rows.Scan(&s.ID, &s.EnvID, &s.Key, &s.Ciphertext, &s.Version, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan secret: %w", err)
		}
		s.CreatedAt = unixSeconds(createdAt)
		s.UpdatedAt = unixSeconds(updatedAt)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate secrets: %w", err)
	}
	return out, nil
}
