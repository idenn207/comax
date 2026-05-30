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
type ServiceToken struct {
	ID         int64
	Name       string
	TokenHash  []byte
	CreatedAt  time.Time
	LastUsedAt *time.Time
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
