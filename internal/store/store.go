// Package store contains the SQLite persistence layer for Comax Secrets.
//
// Conventions established here (mirrored by later milestones):
//
//   - Each entity has a dedicated *Repo type whose methods accept a context
//     and operate over a DBTX interface that is satisfied by *sql.DB and
//     *sql.Tx alike. Repos therefore work transparently inside or outside a
//     transaction; the caller manages BeginTx / Commit / Rollback.
//   - Lookups that miss return ErrNotFound; unique-constraint conflicts
//     surface as ErrConflict. Other failures are wrapped via
//     fmt.Errorf("op: %w", err).
//   - Timestamps are unix seconds (UTC) at rest and time.Time in Go.
//
// The package depends on modernc.org/sqlite (pure Go) so the build stays
// CGO-free and cross-compiles to linux/{amd64,arm64,arm/v7}.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// DBTX is the minimal database/sql surface used by every repository. Both
// *sql.DB and *sql.Tx satisfy it, which is how repo methods participate in
// the caller's transaction without overload variants.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Sentinel errors. Tests and HTTP handlers compare with errors.Is.
var (
	// ErrNotFound is returned when a lookup matches zero rows.
	ErrNotFound = errors.New("store: not found")
	// ErrConflict is returned when a UNIQUE constraint blocks a write.
	ErrConflict = errors.New("store: conflict")
	// ErrVersionNotFound is returned by VersionRepo.ByVersion when the
	// (secret_id, version) pair does not exist. Distinct from ErrNotFound
	// so the dashboard can show "version 5 was deleted" vs "secret was
	// never created".
	ErrVersionNotFound = errors.New("store: version not found")
)

// Project is a top-level scope: typically one repo / one customer.
type Project struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Environment is a named bucket within a project (e.g. "local", "dev",
// "prod", "shared"). InheritsFrom is the name of a sibling env whose
// secrets are merged before this env's overrides; empty = no inheritance.
type Environment struct {
	ID           int64
	ProjectID    int64
	Name         string
	InheritsFrom string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Secret is the current encrypted value of a key inside an environment.
// Version is the monotonic counter; every Upsert bumps it.
type Secret struct {
	ID         int64
	EnvID      int64
	Key        string
	Ciphertext []byte
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SecretVersion is one immutable snapshot in a secret's history.
type SecretVersion struct {
	ID         int64
	SecretID   int64
	Version    int64
	Ciphertext []byte
	ActorToken *int64 // nullable: NULL when system-initiated
	CreatedAt  time.Time
}

// ServiceToken is a bearer credential issued to a CI runner, CLI install
// or dashboard session. Only the SHA-256 hash is persisted; the plaintext
// is shown to the operator exactly once at creation time.
//
// IsAdmin gates token management: only admin tokens may issue or revoke
// other tokens (M3). RevokedAt is nil for live tokens; a non-nil value
// soft-revokes the credential — the bearer auth arm rejects it, and the
// dashboard session arm terminates any live session bound to it.
type ServiceToken struct {
	ID         int64
	Name       string
	TokenHash  []byte
	IsAdmin    bool
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// AuditEntry records a state-changing operation. Append-only.
type AuditEntry struct {
	ID         int64
	ActorToken *int64
	Action     string
	Target     string
	Metadata   string // free-form JSON; empty string when unused
	CreatedAt  time.Time
}

// DashboardSession is one browser session for the dashboard UI. It binds
// a service token to a cookie-shaped credential plus a paired CSRF token;
// only the SHA-256 hash of each plaintext is persisted.
type DashboardSession struct {
	ID          int64
	TokenID     int64
	SessionHash []byte
	CSRFHash    []byte
	UserAgent   string
	IPPrefix    string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

// Webhook is an operator-registered HTTP endpoint that receives a signed POST
// when a matching secret change commits. EnvID is nil to match every
// environment in the project. SecretCiphertext holds the master-key-sealed
// HMAC signing key; List never populates it — only ByID (the delivery-worker
// signing path) returns it.
type Webhook struct {
	ID               int64
	ProjectID        int64
	EnvID            *int64 // nil = all envs in the project
	URL              string
	SecretCiphertext []byte // populated only by ByID (worker signing path); nil in List
	Events           string // comma-joined event kinds
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// WebhookDelivery is one outbox row: a pending, in-flight, or terminal attempt
// to POST an event to a webhook. Payload is JSON metadata only — a secret's
// plaintext value never appears here. NextAttemptAt gates when a pending row
// becomes due; ClaimedAt is the worker lease stamp (nil unless in_progress);
// LastStatus/DeliveredAt are nil until the relevant transition.
type WebhookDelivery struct {
	ID            int64
	WebhookID     int64
	Event         string
	Payload       string
	Status        string
	Attempts      int64
	NextAttemptAt time.Time
	ClaimedAt     *time.Time
	LastStatus    *int64
	LastError     string
	CreatedAt     time.Time
	DeliveredAt   *time.Time
}

// Webhook delivery lifecycle states. A delivery starts pending, is claimed
// into in_progress, then reaches a terminal delivered or dead. A retry moves
// in_progress back to pending with next_attempt_at pushed into the future.
const (
	DeliveryPending    = "pending"
	DeliveryInProgress = "in_progress"
	DeliveryDelivered  = "delivered"
	DeliveryDead       = "dead"
)

// Webhook event kinds. These are the secret-mutation actions that trigger a
// delivery; they are the values stored in webhooks.events (comma-joined) and
// webhook_deliveries.event, and match the audit_log action strings.
const (
	EventSecretUpsert   = "secret.upsert"
	EventSecretRollback = "secret.rollback"
	EventSecretDelete   = "secret.delete"
)

// Open returns a *sql.DB pointed at dsn with foreign keys enforced and
// a busy-timeout configured on every pooled connection.
//
// dsn may be any modernc.org/sqlite-compatible path or URI. Bare paths
// are normalised to file: URIs so pragma appending stays predictable on
// Windows (where paths begin with "C:" and would otherwise collide with
// URI query parsing). The caller's existing pragmas (if any) are
// preserved; defaults are appended only when not already present.
//
// Defaults applied:
//
//   - foreign_keys=1: FK constraints are enforced per-connection (SQLite
//     does not enforce them globally).
//   - busy_timeout=5000: when a writer is blocked on the database lock,
//     wait up to 5 s instead of returning SQLITE_BUSY immediately.
//     SQLite is single-writer at the file level; without this, every
//     contending caller would need to implement its own retry loop.
//
// Examples:
//
//	store.Open("./data/secrets.db")               // bare path
//	store.Open("file:./data/secrets.db")          // file URI
//	store.Open(":memory:")                        // in-memory
//	store.Open("file::memory:?cache=shared")      // in-memory shared cache
func Open(dsn string) (*sql.DB, error) {
	if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
		dsn = "file:" + filepath.ToSlash(dsn)
	}
	dsn = appendPragma(dsn, "foreign_keys", "foreign_keys(1)")
	dsn = appendPragma(dsn, "busy_timeout", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

// appendPragma adds a _pragma=<value> query parameter to dsn iff a pragma
// matching probe is not already present. probe is the bare pragma name
// (e.g. "busy_timeout"); value is the full pragma expression including
// the argument (e.g. "busy_timeout(5000)"). This keeps caller overrides
// intact: store.Open("file:db?_pragma=busy_timeout(10000)") wins.
func appendPragma(dsn, probe, value string) string {
	if strings.Contains(dsn, "_pragma="+probe) {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=" + value
}

// isUniqueViolation returns true when err comes from a SQLite UNIQUE
// constraint failure. modernc.org/sqlite phrases these as
// "constraint failed: UNIQUE constraint failed: ..." so we substring-match
// rather than depend on a driver-specific error type.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE")
}

// unixSeconds converts a Unix-seconds integer (as stored in SQLite) to a
// time.Time in UTC. The zero value round-trips to time.Time{}.
func unixSeconds(secs int64) time.Time {
	if secs == 0 {
		return time.Time{}
	}
	return time.Unix(secs, 0).UTC()
}

// nowUnix returns the current time as Unix seconds, the on-disk format.
func nowUnix() int64 { return time.Now().UTC().Unix() }

// boolToInt maps a Go bool to SQLite's 0/1 integer representation. Used
// when binding boolean columns (e.g. service_tokens.is_admin) so we never
// depend on driver-specific bool coercion.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
