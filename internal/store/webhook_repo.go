package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// WebhookRepo persists webhook subscriptions.
//
// The HMAC signing secret is stored ENCRYPTED (crypto.Seal), not hashed: the
// delivery worker must recover the plaintext to sign each payload. To keep
// that plaintext from leaking through a listing, List and MatchForEvent
// deliberately omit the secret_ciphertext column — only ByID (the worker's
// signing path) returns it.
type WebhookRepo struct{ db DBTX }

// NewWebhookRepo wraps db.
func NewWebhookRepo(db DBTX) *WebhookRepo { return &WebhookRepo{db: db} }

// Create inserts a webhook. envID is nil to subscribe to every environment in
// the project. secretCiphertext is the master-key-sealed HMAC signing key.
// events is the caller-validated comma-joined event set. New webhooks are
// enabled.
func (r *WebhookRepo) Create(ctx context.Context, projectID int64, envID *int64, url, events string, secretCiphertext []byte) (Webhook, error) {
	now := nowUnix()
	var envArg any
	if envID != nil {
		envArg = *envID
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO webhooks (project_id, env_id, url, secret_ciphertext, events, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		projectID, envArg, url, secretCiphertext, events, now, now,
	)
	if err != nil {
		return Webhook{}, fmt.Errorf("create webhook: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Webhook{}, fmt.Errorf("create webhook: %w", err)
	}
	return Webhook{
		ID:               id,
		ProjectID:        projectID,
		EnvID:            envID,
		URL:              url,
		SecretCiphertext: secretCiphertext,
		Events:           events,
		Enabled:          true,
		CreatedAt:        unixSeconds(now),
		UpdatedAt:        unixSeconds(now),
	}, nil
}

// List returns every webhook ordered by id. secret_ciphertext is deliberately
// NOT selected — a listing never needs the signing key, so it never leaves the
// store. The returned Webhook.SecretCiphertext is therefore always nil.
func (r *WebhookRepo) List(ctx context.Context) ([]Webhook, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, env_id, url, events, enabled, created_at, updated_at
		   FROM webhooks
		  ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var out []Webhook
	for rows.Next() {
		w, err := scanWebhookNoSecret(rows)
		if err != nil {
			return nil, fmt.Errorf("list webhooks: scan: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list webhooks: rows: %w", err)
	}
	return out, nil
}

// ByID returns a single webhook INCLUDING its secret_ciphertext. This is the
// worker's signing path: it must recover the sealed HMAC key to compute the
// delivery signature. Handlers that serve listings must never use ByID for
// that purpose. Returns ErrNotFound when id does not exist.
func (r *WebhookRepo) ByID(ctx context.Context, id int64) (Webhook, error) {
	var (
		w         Webhook
		envID     sql.NullInt64
		enabled   int64
		createdAt int64
		updatedAt int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, env_id, url, secret_ciphertext, events, enabled, created_at, updated_at
		   FROM webhooks
		  WHERE id = ?`,
		id,
	).Scan(&w.ID, &w.ProjectID, &envID, &w.URL, &w.SecretCiphertext, &w.Events, &enabled, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Webhook{}, fmt.Errorf("webhook %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Webhook{}, fmt.Errorf("webhook %d: %w", id, err)
	}
	if envID.Valid {
		v := envID.Int64
		w.EnvID = &v
	}
	w.Enabled = enabled != 0
	w.CreatedAt = unixSeconds(createdAt)
	w.UpdatedAt = unixSeconds(updatedAt)
	return w, nil
}

// Delete removes a webhook by id. The ON DELETE CASCADE on
// webhook_deliveries.webhook_id purges its delivery history in the same
// statement. Returns ErrNotFound when id matched no row.
func (r *WebhookRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete webhook %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete webhook %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("webhook %d: %w", id, ErrNotFound)
	}
	return nil
}

// SetEnabled toggles a webhook's enabled flag (soft-disable). A disabled
// webhook is skipped by MatchForEvent, so it stops receiving deliveries
// without losing its registration or history. Returns ErrNotFound when id
// matched no row.
func (r *WebhookRepo) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`UPDATE webhooks SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), now, id,
	)
	if err != nil {
		return fmt.Errorf("set webhook %d enabled: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set webhook %d enabled: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("webhook %d: %w", id, ErrNotFound)
	}
	return nil
}

// MatchForEvent returns the ENABLED webhooks that should receive a delivery
// for (projectID, envID, event): project must match, the webhook's env must be
// NULL (all-envs) or equal envID, and event must be one of the webhook's
// comma-joined events. secret_ciphertext is omitted — the enqueue path only
// needs webhook ids; the worker fetches the key via ByID at delivery time.
//
// The event match wraps both the stored list and the probe in commas
// (",upsert," LIKE "%,upsert,%") so a token matches exactly and never as a
// substring of a longer event name.
func (r *WebhookRepo) MatchForEvent(ctx context.Context, projectID, envID int64, event string) ([]Webhook, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, env_id, url, events, enabled, created_at, updated_at
		   FROM webhooks
		  WHERE enabled = 1
		    AND project_id = ?
		    AND (env_id IS NULL OR env_id = ?)
		    AND (',' || events || ',') LIKE ?
		  ORDER BY id`,
		projectID, envID, "%,"+event+",%",
	)
	if err != nil {
		return nil, fmt.Errorf("match webhooks: %w", err)
	}
	defer rows.Close()

	var out []Webhook
	for rows.Next() {
		w, err := scanWebhookNoSecret(rows)
		if err != nil {
			return nil, fmt.Errorf("match webhooks: scan: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("match webhooks: rows: %w", err)
	}
	return out, nil
}

// scanWebhookNoSecret scans a webhooks row from the non-secret column set
// (id, project_id, env_id, url, events, enabled, created_at, updated_at). The
// scanner is satisfied by both *sql.Row and *sql.Rows. SecretCiphertext is
// left nil by design — this column set never includes it.
func scanWebhookNoSecret(sc interface{ Scan(...any) error }) (Webhook, error) {
	var (
		w         Webhook
		envID     sql.NullInt64
		enabled   int64
		createdAt int64
		updatedAt int64
	)
	if err := sc.Scan(&w.ID, &w.ProjectID, &envID, &w.URL, &w.Events, &enabled, &createdAt, &updatedAt); err != nil {
		return Webhook{}, err
	}
	if envID.Valid {
		v := envID.Int64
		w.EnvID = &v
	}
	w.Enabled = enabled != 0
	w.CreatedAt = unixSeconds(createdAt)
	w.UpdatedAt = unixSeconds(updatedAt)
	return w, nil
}
