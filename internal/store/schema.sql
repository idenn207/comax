-- Comax Secrets — Milestone 1 schema.
--
-- Design notes:
--   * INTEGER timestamps are unix seconds (UTC). Cheap to compare/sort and
--     trivially portable across drivers; we wrap in Go time.Time at the
--     repo boundary.
--   * Encrypted values live in BLOB ciphertext columns. Plaintext never
--     touches the DB.
--   * service_tokens stores only token_hash (SHA-256). The plaintext token
--     is shown to the operator exactly once at issue time.
--   * secret_versions is append-only: every change to secrets writes a row
--     here so M2 can implement diff/rollback without schema changes.
--   * audit_log mirrors secret_versions for non-secret operations (env
--     create, token issue, etc.). Same append-only shape.
--   * FKs are enforced by setting PRAGMA foreign_keys=ON on every
--     connection (handled in store.Open via DSN pragma).
--   * All CREATE statements are IF NOT EXISTS so Migrate is idempotent.

CREATE TABLE IF NOT EXISTS projects (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS environments (
    id            INTEGER PRIMARY KEY,
    project_id    INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name          TEXT    NOT NULL,
    inherits_from TEXT,                      -- sibling env name; NULL = no inheritance
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS secrets (
    id         INTEGER PRIMARY KEY,
    env_id     INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    key        TEXT    NOT NULL,
    ciphertext BLOB    NOT NULL,
    version    INTEGER NOT NULL,             -- monotonic per-secret version counter
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER,                      -- NULL = live; non-NULL = soft-deleted (M2)
    UNIQUE (env_id, key)
);

CREATE INDEX IF NOT EXISTS idx_secrets_env_key ON secrets (env_id, key);

CREATE TABLE IF NOT EXISTS secret_versions (
    id          INTEGER PRIMARY KEY,
    secret_id   INTEGER NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    ciphertext  BLOB    NOT NULL,
    actor_token INTEGER REFERENCES service_tokens(id),
    created_at  INTEGER NOT NULL,
    UNIQUE (secret_id, version)
);

CREATE INDEX IF NOT EXISTS idx_versions_secret ON secret_versions (secret_id, version);

CREATE TABLE IF NOT EXISTS service_tokens (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    token_hash   BLOB NOT NULL UNIQUE,       -- SHA-256(plaintext token)
    created_at   INTEGER NOT NULL,
    last_used_at INTEGER
);

CREATE TABLE IF NOT EXISTS audit_log (
    id          INTEGER PRIMARY KEY,
    actor_token INTEGER REFERENCES service_tokens(id),
    action      TEXT    NOT NULL,            -- e.g. "project.create", "secret.upsert"
    target      TEXT    NOT NULL,            -- e.g. "project=app env=dev key=DB_URL"
    metadata    TEXT,                        -- optional JSON blob
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log (created_at);

-- M2: browser session for the dashboard UI. Each row hands a service
-- token a cookie-shaped credential plus a paired CSRF token. We store
-- only the SHA-256 hash of each plaintext; the dashboard sees plaintext
-- once at /dashboard/session creation time.
CREATE TABLE IF NOT EXISTS dashboard_sessions (
    id           INTEGER PRIMARY KEY,
    token_id     INTEGER NOT NULL REFERENCES service_tokens(id) ON DELETE CASCADE,
    session_hash BLOB    NOT NULL UNIQUE,     -- SHA-256(comax_session cookie)
    csrf_hash    BLOB    NOT NULL,            -- SHA-256(X-CSRF-Token header)
    user_agent   TEXT,                        -- best-effort, may be ""
    ip_prefix    TEXT,                        -- /24 (v4) or /48 (v6), see auth.IPPrefix
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,            -- unix seconds; 0 == never expires (unused in v1)
    revoked_at   INTEGER
);

CREATE INDEX IF NOT EXISTS idx_sessions_token ON dashboard_sessions (token_id);
