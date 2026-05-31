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
//
// Soft-delete reactivation: when the existing row was previously
// soft-deleted (deleted_at IS NOT NULL), the conflict-update clears
// deleted_at, restoring the secret as part of the same write. This
// matches the operator's expectation that re-PUTting a deleted key
// brings it back (the original ciphertext is gone, but the new one
// becomes version N+1 in the same lineage so history is preserved).
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
		     updated_at = excluded.updated_at,
		     deleted_at = NULL
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

// ByKey returns the current live secret for (envID, key), or ErrNotFound.
// Soft-deleted rows are excluded — operators reading a deleted key see a
// 404 even though the row physically exists.
func (r *SecretRepo) ByKey(ctx context.Context, envID int64, key string) (Secret, error) {
	var (
		s                    Secret
		createdAt, updatedAt int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, env_id, key, ciphertext, version, created_at, updated_at
		   FROM secrets
		  WHERE env_id = ? AND key = ? AND deleted_at IS NULL`,
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

// ByKeyAny returns the secret for (envID, key) including soft-deleted
// rows. Used by the version-history endpoint so the dashboard can keep
// rendering a deleted secret's past versions; live reads must continue
// to use ByKey.
func (r *SecretRepo) ByKeyAny(ctx context.Context, envID int64, key string) (Secret, error) {
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

// ListByEnv returns every current live secret in envID, ordered by key
// for deterministic .env emission. Soft-deleted rows are excluded.
func (r *SecretRepo) ListByEnv(ctx context.Context, envID int64) ([]Secret, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, env_id, key, ciphertext, version, created_at, updated_at
		   FROM secrets
		  WHERE env_id = ? AND deleted_at IS NULL
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

// ListByEnvAny returns every secret in envID — including soft-deleted
// rows — ordered by key. Used by the env-wide versions endpoint so the
// dashboard can still render the timeline of a deleted key.
func (r *SecretRepo) ListByEnvAny(ctx context.Context, envID int64) ([]Secret, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, env_id, key, ciphertext, version, created_at, updated_at
		   FROM secrets
		  WHERE env_id = ?
		  ORDER BY key`,
		envID,
	)
	if err != nil {
		return nil, fmt.Errorf("list (incl deleted) secrets for env %d: %w", envID, err)
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

// Delete soft-deletes the secret at (envID, key). Returns ErrNotFound
// when no live row matches — either the key never existed or it is
// already deleted.
//
// History in secret_versions is intentionally untouched so the dashboard
// can still render the timeline of a removed key. A subsequent Upsert
// of the same key clears deleted_at and continues the version sequence.
func (r *SecretRepo) Delete(ctx context.Context, envID int64, key string) error {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`UPDATE secrets
		    SET deleted_at = ?, updated_at = ?
		  WHERE env_id = ? AND key = ? AND deleted_at IS NULL`,
		now, now, envID, key,
	)
	if err != nil {
		return fmt.Errorf("delete secret %q in env %d: %w", key, envID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete secret %q in env %d: %w", key, envID, err)
	}
	if n == 0 {
		return fmt.Errorf("secret %q in env %d: %w", key, envID, ErrNotFound)
	}
	return nil
}
