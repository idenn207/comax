package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DeliveryRepo is the transactional outbox for webhook deliveries.
//
// Enqueue runs inside the secret-change tx, so a delivery becomes durable only
// when that change commits. The delivery worker then claims due rows with an
// atomic compare-and-swap (pending -> in_progress + claimed_at lease) so two
// workers never take the same row, marks the outcome guarded by
// WHERE status='in_progress' (a lost race or a reclaim can't be overwritten),
// and ReclaimStale recovers rows a crashed worker left in_progress.
type DeliveryRepo struct{ db DBTX }

// NewDeliveryRepo wraps db.
func NewDeliveryRepo(db DBTX) *DeliveryRepo { return &DeliveryRepo{db: db} }

// Enqueue appends a pending delivery. It is meant to run inside the same tx as
// the triggering secret change (via a DeliveryRepo built over *sql.Tx), so a
// rolled-back change leaves no ghost delivery. next_attempt_at is now, making
// the row immediately due once the tx commits.
func (r *DeliveryRepo) Enqueue(ctx context.Context, webhookID int64, event, payload string) (WebhookDelivery, error) {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries
		   (webhook_id, event, payload, status, attempts, next_attempt_at, created_at)
		 VALUES (?, ?, ?, ?, 0, ?, ?)`,
		webhookID, event, payload, DeliveryPending, now, now,
	)
	if err != nil {
		return WebhookDelivery{}, fmt.Errorf("enqueue delivery: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return WebhookDelivery{}, fmt.Errorf("enqueue delivery: %w", err)
	}
	return WebhookDelivery{
		ID:            id,
		WebhookID:     webhookID,
		Event:         event,
		Payload:       payload,
		Status:        DeliveryPending,
		Attempts:      0,
		NextAttemptAt: unixSeconds(now),
		CreatedAt:     unixSeconds(now),
	}, nil
}

// ClaimDue atomically claims up to limit due deliveries and transitions each to
// in_progress with a claimed_at lease. Concurrency safety comes from a
// compare-and-swap per candidate: the row is selected only WHERE it is still
// pending, so if a competing worker grabbed it first our UPDATE affects zero
// rows and we skip it. Only rows this call actually won are returned, with
// their in-memory Status/ClaimedAt updated to reflect the claim.
//
// Candidates are read and the read cursor closed BEFORE any UPDATE, so the
// single-writer driver never has a live SELECT open across the CAS writes.
func (r *DeliveryRepo) ClaimDue(ctx context.Context, now time.Time, limit int) ([]WebhookDelivery, error) {
	nowU := now.UTC().Unix()

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, webhook_id, event, payload, status, attempts, next_attempt_at, claimed_at, last_status, last_error, created_at, delivered_at
		   FROM webhook_deliveries
		  WHERE status = ? AND next_attempt_at <= ?
		  ORDER BY id
		  LIMIT ?`,
		DeliveryPending, nowU, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claim due: select: %w", err)
	}
	var candidates []WebhookDelivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("claim due: scan: %w", err)
		}
		candidates = append(candidates, d)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("claim due: rows: %w", err)
	}
	rows.Close()

	var claimed []WebhookDelivery
	for _, d := range candidates {
		res, err := r.db.ExecContext(ctx,
			`UPDATE webhook_deliveries
			    SET status = ?, claimed_at = ?
			  WHERE id = ? AND status = ?`,
			DeliveryInProgress, nowU, d.ID, DeliveryPending,
		)
		if err != nil {
			return nil, fmt.Errorf("claim due: cas %d: %w", d.ID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("claim due: cas %d: %w", d.ID, err)
		}
		if n == 0 {
			continue // lost the race to a competing worker
		}
		d.Status = DeliveryInProgress
		ct := unixSeconds(nowU)
		d.ClaimedAt = &ct
		claimed = append(claimed, d)
	}
	return claimed, nil
}

// MarkDelivered transitions a claimed delivery to delivered. The
// WHERE status='in_progress' guard means only the worker holding the claim can
// finalise it; a reclaimed or already-terminal row affects zero rows and
// returns ErrNotFound. statusCode is the 2xx that confirmed delivery.
func (r *DeliveryRepo) MarkDelivered(ctx context.Context, id int64, statusCode int) error {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`UPDATE webhook_deliveries
		    SET status = ?, attempts = attempts + 1, last_status = ?, last_error = '',
		        claimed_at = NULL, delivered_at = ?
		  WHERE id = ? AND status = ?`,
		DeliveryDelivered, statusCode, now, id, DeliveryInProgress,
	)
	return affectedOne(res, err, id, "mark delivered")
}

// MarkRetry returns a claimed delivery to pending with attempts incremented and
// next_attempt_at pushed to nextAttemptAt (caller applies backoff). It is
// guarded by WHERE status='in_progress'. statusCode is the failing HTTP status
// or 0 for a transport error (stored as NULL); errMsg is a short reason.
func (r *DeliveryRepo) MarkRetry(ctx context.Context, id int64, nextAttemptAt time.Time, statusCode int, errMsg string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE webhook_deliveries
		    SET status = ?, attempts = attempts + 1, next_attempt_at = ?,
		        last_status = ?, last_error = ?, claimed_at = NULL
		  WHERE id = ? AND status = ?`,
		DeliveryPending, nextAttemptAt.UTC().Unix(), nullableStatus(statusCode), errMsg, id, DeliveryInProgress,
	)
	return affectedOne(res, err, id, "mark retry")
}

// MarkDead transitions a claimed delivery to the terminal dead state after the
// final attempt exhausted maxAttempts. Guarded by WHERE status='in_progress'.
func (r *DeliveryRepo) MarkDead(ctx context.Context, id int64, statusCode int, errMsg string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE webhook_deliveries
		    SET status = ?, attempts = attempts + 1, last_status = ?, last_error = ?, claimed_at = NULL
		  WHERE id = ? AND status = ?`,
		DeliveryDead, nullableStatus(statusCode), errMsg, id, DeliveryInProgress,
	)
	return affectedOne(res, err, id, "mark dead")
}

// ReclaimStale flips in_progress deliveries whose claimed_at predates before
// back to pending, so a worker that crashed mid-delivery does not strand the
// row forever. attempts is intentionally NOT incremented and next_attempt_at is
// left as-is: the row becomes due immediately for a fresh claim (at-least-once
// — the receiver must be idempotent). Returns the number of rows reclaimed.
func (r *DeliveryRepo) ReclaimStale(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE webhook_deliveries
		    SET status = ?, claimed_at = NULL
		  WHERE status = ? AND claimed_at IS NOT NULL AND claimed_at < ?`,
		DeliveryPending, DeliveryInProgress, before.UTC().Unix(),
	)
	if err != nil {
		return 0, fmt.Errorf("reclaim stale: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("reclaim stale: %w", err)
	}
	return n, nil
}

// ByID returns a single delivery by primary key, or ErrNotFound.
func (r *DeliveryRepo) ByID(ctx context.Context, id int64) (WebhookDelivery, error) {
	d, err := scanDelivery(r.db.QueryRowContext(ctx,
		`SELECT id, webhook_id, event, payload, status, attempts, next_attempt_at, claimed_at, last_status, last_error, created_at, delivered_at
		   FROM webhook_deliveries
		  WHERE id = ?`,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return WebhookDelivery{}, fmt.Errorf("delivery %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return WebhookDelivery{}, fmt.Errorf("delivery %d: %w", id, err)
	}
	return d, nil
}

// ListByWebhook returns the most recent deliveries for a webhook, newest first,
// capped at limit. Used by the dashboard / CLI to show delivery status.
func (r *DeliveryRepo) ListByWebhook(ctx context.Context, webhookID int64, limit int) ([]WebhookDelivery, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, webhook_id, event, payload, status, attempts, next_attempt_at, claimed_at, last_status, last_error, created_at, delivered_at
		   FROM webhook_deliveries
		  WHERE webhook_id = ?
		  ORDER BY id DESC
		  LIMIT ?`,
		webhookID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()

	var out []WebhookDelivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("list deliveries: scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list deliveries: rows: %w", err)
	}
	return out, nil
}

// affectedOne folds the RowsAffected guard shared by the Mark* transitions: a
// driver error wraps through, zero rows affected (lost claim / wrong state)
// surfaces ErrNotFound, exactly one row is success.
func affectedOne(res sql.Result, execErr error, id int64, op string) error {
	if execErr != nil {
		return fmt.Errorf("%s %d: %w", op, id, execErr)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s %d: %w", op, id, err)
	}
	if n == 0 {
		return fmt.Errorf("%s %d: %w", op, id, ErrNotFound)
	}
	return nil
}

// nullableStatus maps a zero HTTP status (transport error, no response) to a
// NULL last_status, and a real status code to itself.
func nullableStatus(statusCode int) any {
	if statusCode <= 0 {
		return nil
	}
	return statusCode
}

// scanDelivery scans a full webhook_deliveries row. The column order must match
// the SELECT lists above. Satisfied by both *sql.Row and *sql.Rows.
func scanDelivery(sc interface{ Scan(...any) error }) (WebhookDelivery, error) {
	var (
		d             WebhookDelivery
		nextAttemptAt int64
		claimedAt     sql.NullInt64
		lastStatus    sql.NullInt64
		lastError     sql.NullString
		createdAt     int64
		deliveredAt   sql.NullInt64
	)
	if err := sc.Scan(
		&d.ID, &d.WebhookID, &d.Event, &d.Payload, &d.Status, &d.Attempts,
		&nextAttemptAt, &claimedAt, &lastStatus, &lastError, &createdAt, &deliveredAt,
	); err != nil {
		return WebhookDelivery{}, err
	}
	d.NextAttemptAt = unixSeconds(nextAttemptAt)
	if claimedAt.Valid {
		t := unixSeconds(claimedAt.Int64)
		d.ClaimedAt = &t
	}
	if lastStatus.Valid {
		v := lastStatus.Int64
		d.LastStatus = &v
	}
	d.LastError = lastError.String
	d.CreatedAt = unixSeconds(createdAt)
	if deliveredAt.Valid {
		t := unixSeconds(deliveredAt.Int64)
		d.DeliveredAt = &t
	}
	return d, nil
}
