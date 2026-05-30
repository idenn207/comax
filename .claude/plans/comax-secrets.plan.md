# Plan: Comax Secrets — Milestone 1 (Self-host Server + CLI MVP)

**Source PRD**: [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md)
**Selected Milestone**: #1 — Self-host server + CLI MVP
**Complexity**: Large (foundation work; establishes conventions for all later milestones)

## Summary

Build the foundational pair — a single-binary Go server backed by SQLite and a single-binary Go CLI (`secret`) — so the operator can replace 12 hand-synced `.env` files with one source of truth. The server stores AES-256-GCM–encrypted secrets and exposes a small REST API; the CLI handles `init / login / pull / push / run / set / get / diff` with worktree-aware context resolution. **No dashboard, no GitHub Action, no SDK, no webhooks in this milestone** — those are Milestones 2–6. Success is binary: the operator's own 12 `.env` files run through `secret run -- <cmd>` end-to-end with `≤ 300ms p95` cold start and `docker compose up ≤ 2 min` on a clean VM.

## Resolved Open Questions (from PRD)

These three were blocking and are decided here so implementation is unblocked. Sections that touch them link back.

| PRD Q# | Decision | Rationale |
|---|---|---|
| **#1 Worktree CLI context** | **Hybrid**: `.secretrc` per-worktree (highest precedence) → git branch → `--env` flag → server-side default. `.secretrc` is gitignored. | Matches PRD's leaning ("권장(잠정): ③ 하이브리드"). `.secretrc` gives explicit override for worktrees; branch fallback gives zero-config for `main/dev/prod` branch naming; flag gives CI escape hatch. |
| **#3 Master key model** | **File on disk, mode `0600`, owned by server user**, path via `COMAX_MASTER_KEY_FILE` env or `--master-key-file` flag. **Refuse boot** if mode is wider than `0600` or owner mismatch. KMS/keyring is a *pluggable interface* (`crypto.KeyProvider`) but only the file provider ships in M1. | NAS-friendly (no KMS dep). The pluggable interface keeps Option #4 from PRD reachable without paying its cost now. Refuse-to-boot prevents the #1 misconfig risk in PRD. |
| **#6 CLI language** | **Go** for both server and CLI. Rust deferred. | Go cross-compiles cleanly to `linux/amd64`, `linux/arm64`, `linux/arm/v7` (typical NAS targets) without CGO when using `modernc.org/sqlite`. Cobra+Viper give CLI ergonomics for free. Cold-start budget (`≤ 300ms p95`) is achievable in Go with a thin command path. Rust would buy ~50ms at the cost of ~3× implementation time, and the PRD's success metric is operator throughput, not raw latency. |

Open Questions **#5 (infra config templating)** and **#7 (telemetry)** are out of scope for M1 — they belong to M7 and a later observability pass. **#2 (Swarm restart)** and **#4 (dashboard/site split)** are M3/M4/M6 concerns.

## Patterns to Mirror

**Greenfield repo — no existing patterns to mirror.** This milestone *establishes* the conventions later milestones must follow. Stated here so that later plans cite this file by path:

| Category | This milestone establishes | Where |
|---|---|---|
| Naming | Command verbs (`pull`, `push`, `run`, `set`, `get`, `diff`); package names lowercase single word (`store`, `crypto`, `auth`); type names PascalCase | `internal/*` packages |
| Errors | Wrapped errors via `fmt.Errorf("op: %w", err)`; sentinel errors for domain (`store.ErrNotFound`); CLI prints user-friendly messages, logs structured detail to stderr | `internal/store`, `cmd/cli` |
| Logging | `log/slog` (stdlib); JSON in server, text in CLI; secrets never logged (assert in tests) | `internal/server`, `cmd/cli` |
| Data access | Repository pattern per entity (`ProjectRepo`, `EnvRepo`, `SecretRepo`, `VersionRepo`) over `*sql.DB`; transactions explicit | `internal/store` |
| Tests | Table-driven `_test.go` colocated with source; integration tests use `httptest.Server` + temp SQLite file; `testdata/` for fixtures; coverage gate ≥ 80% per package | every package |

> **Common rule precedence**: This plan respects ECC `common/coding-style.md` (immutability, file size ≤ 800 lines, error handling explicit) and `common/testing.md` (TDD, 80% coverage). Go-idiomatic mutation via pointer receivers is allowed per the language-rule override pattern.

## Files to Change

Greenfield — every file is CREATE in M1. Path layout below is the contract later milestones depend on.

| File | Action | Why |
|---|---|---|
| `go.mod`, `go.sum` | CREATE | Go 1.22+ module `github.com/<owner>/comax-secrets` |
| `cmd/server/main.go` | CREATE | `secret-server` binary entrypoint |
| `cmd/cli/main.go` | CREATE | `secret` binary entrypoint (cobra root) |
| `internal/config/config.go` | CREATE | Server config load (env, file, flags); refuse boot on bad master-key permissions |
| `internal/crypto/aesgcm.go` | CREATE | AES-256-GCM seal/open; per-secret random nonce; HKDF for key derivation |
| `internal/crypto/provider.go` | CREATE | `KeyProvider` interface + `FileKeyProvider` impl (M1 only impl) |
| `internal/store/schema.sql` | CREATE | DDL: `projects`, `environments`, `secrets`, `secret_versions`, `audit_log`, `service_tokens` |
| `internal/store/migrate.go` | CREATE | Embedded schema via `embed.FS`; idempotent apply on boot |
| `internal/store/{project,env,secret,version,audit,token}_repo.go` | CREATE | Per-entity repository over `*sql.DB` |
| `internal/auth/token.go` | CREATE | Bearer token verify (HMAC); bootstrap admin token on first boot |
| `internal/server/router.go` | CREATE | `chi` router; mounts handlers; middleware (auth, request log, recover) |
| `internal/server/handlers_{project,env,secret,version}.go` | CREATE | REST handlers (see API spec below) |
| `internal/secret/reference.go` | CREATE | Inline `${{ shared.KEY }}` resolution (server-side at pull time) |
| `pkg/client/client.go` | CREATE | HTTP client shared by CLI now and SDK later (M5) |
| `cmd/cli/cmd_{login,init,pull,push,run,set,get,diff}.go` | CREATE | One file per command |
| `cmd/cli/context.go` | CREATE | Worktree-aware context resolution (`.secretrc` → branch → `--env`) |
| `cmd/cli/run.go` (in cmd_run.go) | CREATE | `os/exec` child process with env merged from pulled secrets |
| `deploy/docker/Dockerfile` | CREATE | Multi-stage; final image `gcr.io/distroless/static` for server |
| `deploy/compose/docker-compose.yml` | CREATE | Bind-mount `./data` (SQLite) and `./keys/master.key`; healthcheck |
| `docs/quickstart.md` | CREATE | `docker compose up` → `secret init` → first pull in ≤ 5 min |
| `docs/threat-model.md` | CREATE | "Self-host = operator owns DB and key" model spelled out |
| `Makefile` | CREATE | `make build / test / lint / bench / docker` targets |
| `.github/workflows/ci.yml` | CREATE | `go test -race -cover`, golangci-lint, build matrix `linux/{amd64,arm64,arm/v7}` |
| `.gitignore` | CREATE | Includes `.secretrc`, `data/`, `keys/`, `*.key`, `coverage.out` |
| `README.md` | UPDATE | Replace placeholder with project overview + link to docs |

## API Surface (M1 scope only)

Designed to be the contract Milestones 2 (dashboard), 3 (action), 5 (SDK) will consume. **Versioned at `/api/v1` from day one.** All endpoints require `Authorization: Bearer <service-token>` except `POST /api/v1/bootstrap`.

| Method | Path | Purpose | Used by M1 CLI |
|---|---|---|---|
| `POST` | `/api/v1/bootstrap` | One-time admin token issue when DB is empty | `secret init` (server-side) |
| `GET` | `/api/v1/projects` | List projects | `secret init` autocomplete |
| `POST` | `/api/v1/projects` | Create project | `secret init` |
| `GET` | `/api/v1/projects/{p}/envs` | List envs | `secret diff` |
| `POST` | `/api/v1/projects/{p}/envs` | Create env (e.g. `local`, `dev`, `prod`) | `secret init` |
| `GET` | `/api/v1/projects/{p}/envs/{e}/secrets` | Pull all secrets for env (decrypted, with `${{ ... }}` resolved) | `secret pull`, `secret run` |
| `PUT` | `/api/v1/projects/{p}/envs/{e}/secrets/{k}` | Upsert one secret (creates new version) | `secret set`, `secret push` |
| `GET` | `/api/v1/projects/{p}/envs/{e}/secrets/{k}` | Get one secret (current version) | `secret get` |
| `GET` | `/api/v1/projects/{p}/envs/{e}/versions` | List version history (for M2 diff/rollback — exposed now to lock the shape) | — (out-of-CLI; M2 consumer) |
| `GET` | `/healthz` | Liveness | docker-compose healthcheck |

Response envelope (per ECC `common/patterns.md`): `{ "ok": true, "data": ..., "error": null, "meta": { ... } }`.

## Tasks

Ordered for early CI green and late integration confidence. Each task ends with a runnable validation step.

### Task 1: Repo bootstrap & CI green
- **Action**: `go mod init`; commit minimal `cmd/server/main.go` and `cmd/cli/main.go` printing version; add `Makefile`, `.gitignore`, `.github/workflows/ci.yml` with build + lint + test matrix.
- **Establishes**: project layout, Go version, lint config (`.golangci.yml`).
- **Validate**: `make build` succeeds; CI green on push.

### Task 2: SQLite store layer with embedded migrations
- **Action**: Use `modernc.org/sqlite` (pure-Go, no CGO → arm/v7 cross-compile works). Write `schema.sql` for `projects`, `environments`, `secrets`, `secret_versions`, `audit_log`, `service_tokens`. Embed via `//go:embed`. Idempotent `Migrate(ctx, db) error`. Repos return `store.ErrNotFound` sentinel.
- **Establishes**: repository pattern, error sentinels, table-driven test style.
- **Validate**: `go test ./internal/store/... -race -cover` ≥ 90% (this layer is pure logic, target higher than gate).

### Task 3: Crypto layer + master key provider
- **Action**: `internal/crypto/aesgcm.go` (AES-256-GCM, 12-byte random nonce, ciphertext layout `nonce || ct || tag`). `KeyProvider` interface; `FileKeyProvider` loads 32-byte key from file, **refuses if `stat.Mode().Perm() != 0o600`** on Unix (Windows: warn-and-continue, documented). HKDF per-secret subkey is *optional* — defer unless TDD shows clear win.
- **Establishes**: encryption interface for KMS/keyring later.
- **Validate**: `go test ./internal/crypto/... -race`; round-trip property test (encrypt→decrypt over fuzzed input).

### Task 4: Auth + bootstrap flow
- **Action**: `service_tokens` table with `id`, `name`, `token_hash` (SHA-256 of opaque random token), `created_at`, `last_used_at`. `POST /bootstrap` works *only when no tokens exist* (race-safe via SQL guard). Middleware extracts bearer, looks up hash, attaches token meta to ctx.
- **Validate**: `httptest`-backed integration test: bootstrap succeeds once, second call returns `409`.

### Task 5: REST handlers — projects/envs/secrets/versions
- **Action**: Implement the API table above behind `chi` router. Encrypt on PUT, decrypt on GET, write `secret_versions` row on every change, write `audit_log` row in same tx.
- **Establishes**: handler style, request validation, response envelope, audit pattern.
- **Validate**: `go test ./internal/server/... -race -cover` ≥ 80%; full CRUD round-trip via `httptest.Server` in `test/integration/server_test.go`.

### Task 6: Inline secret references (`${{ shared.KEY }}`)
- **Action**: Resolution happens at pull time on the server (not at write time) so a referenced value reflects current state. Define syntax: `${{ <env>.<KEY> }}` where `<env>` is a sibling env name; `shared` is the conventional name for a cross-env env. Detect cycles → return 400 with the cycle path. Inherit/override: env `dev` may declare `inherits_from = "shared"`; explicit keys override.
- **Validate**: unit tests for parse, resolve, cycle detection; integration test that two envs sharing `shared.DB_HOST` pull consistent values; mutating `shared` propagates without writing in other envs.

### Task 7: CLI skeleton + login/init
- **Action**: Cobra root with subcommands stubbed. `secret login --server <url> --token <t>` writes to `~/.config/comax/credentials.json` (mode `0600`). `secret init` prompts for project, creates project + envs server-side, writes `.secretrc` (gitignored) with `{ "project": "...", "default_env": "..." }`.
- **Establishes**: CLI ergonomics, config file layout.
- **Validate**: `go test ./cmd/cli/... -race`; manual smoke: `secret login && secret init` against local docker-compose.

### Task 8: Worktree-aware context resolution
- **Action**: `cmd/cli/context.go` resolves env by precedence: `--env` flag > `COMAX_ENV` env var > `.secretrc.env` > git branch lookup (configurable mapping in `.secretrc.branches`, default `main→prod`, `dev→dev`, anything else → `local`) > server default. Detect worktree via `git rev-parse --git-common-dir` ≠ `--git-dir`.
- **Validate**: table-driven test covering all precedence permutations; integration test in temp git repo with two worktrees on different branches → each resolves correctly.

### Task 9: pull / push / set / get / diff
- **Action**:
  - `secret pull` writes `.env` to current dir (or `--out`); idempotent; preserves trailing newline; respects `--format=dotenv|json|yaml`.
  - `secret push --file .env` parses dotenv and PUTs each key.
  - `secret set KEY=VALUE`, `secret get KEY` for one-offs.
  - `secret diff [--against <env>]` shows added/removed/changed keys between two envs using the existing list endpoint.
- **Validate**: per-command tests; end-to-end script in `test/e2e/` boots compose, pushes a fixture `.env`, pulls it back, diffs against a second env.

### Task 10: `secret run -- <cmd>` (the headline command)
- **Action**: Resolve context (Task 8) → pull (in-memory, no disk write) → `exec.Command` child with `Env = append(os.Environ(), pulled...)`. Forward stdin/stdout/stderr. Exit with child's exit code. **Never write decrypted values to disk in `run` mode.**
- **Validate**: integration test asserts a child `printenv KEY` sees pulled value; assertion that no temp file with decrypted content is created (filesystem audit).

### Task 11: Cold-start benchmark + budget gate
- **Action**: `go test -bench BenchmarkSecretRunColdStart` in `cmd/cli/bench_test.go` that measures wall-clock from `exec` to first child env read. CI fails if p95 > 300 ms on the GH runner profile.
- **Validate**: bench passes locally and in CI on `ubuntu-latest`; record baseline in `docs/perf.md` (≤ one paragraph).

### Task 12: Docker packaging — `docker compose up ≤ 2 min`
- **Action**: Multi-stage Dockerfile (`golang:1.22` builder → `distroless/static` runner). `docker-compose.yml` bind-mounts `./data` (SQLite) and `./keys/master.key`; healthcheck on `/healthz`; auto-generates a master key on first boot if `keys/master.key` is missing (logs the path + a one-time bootstrap admin token to stdout — operator copies it).
- **Validate**: on a clean VM, `time docker compose up -d && wait-for-healthy.sh` ≤ 120 s; bootstrap token visible in logs.

### Task 13: Quickstart + threat model docs
- **Action**: `docs/quickstart.md` — clone → `docker compose up` → `secret login` (with bootstrap token from logs) → `secret init` → `secret push --file .env` → `secret run -- npm dev`. Total operator time ≤ 5 min. `docs/threat-model.md` — explicit statement of the PRD's threat model + the master key permission requirement.
- **Validate**: a fresh teammate (or operator self-test from a clean clone) completes the quickstart without questions in ≤ 5 min.

### Task 14: Operator dogfood — replace own 12 `.env` files
- **Action**: Take the operator's real `api/web/mq/infra × local/dev/prod` set. Migrate all 12 into the server. Run each service via `secret run -- <existing-start-cmd>`. Record before/after time for adding one new envvar across all envs.
- **Validate**: the PRD's headline success metric — adding one envvar to all 12 envs takes ≤ 1 minute (vs ≥ 5 minutes baseline). Logged in `docs/dogfood.md`.

## Validation

```bash
# Build & static checks
make build
make lint                    # golangci-lint run

# Test & coverage gate
go test ./... -race -coverprofile=coverage.out
go tool cover -func=coverage.out | awk '/total:/ { if ($3+0 < 80) exit 1 }'

# Benchmark gate
go test -bench=BenchmarkSecretRunColdStart -run=^$ -benchtime=10x ./cmd/cli

# Cross-compile sanity (NAS targets)
GOOS=linux GOARCH=arm64   go build -o /tmp/secret-arm64   ./cmd/cli
GOOS=linux GOARCH=arm GOARM=7 go build -o /tmp/secret-armv7 ./cmd/cli

# Compose smoke
docker compose -f deploy/compose/docker-compose.yml up -d
docker compose -f deploy/compose/docker-compose.yml ps   # all healthy
docker compose -f deploy/compose/docker-compose.yml down

# End-to-end
./test/e2e/run.sh            # push fixture, pull, diff, run child
```

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| **Scope creep into M2 dashboard** while M1 is unfinished | High | High | API for `/versions` is shipped but no UI work in M1. Dogfood (Task 14) is the gate. |
| **CGO needed for SQLite** breaks arm/v7 NAS cross-compile | Medium | High | Use `modernc.org/sqlite` (pure-Go). Verified by Task 12 cross-compile step. Fallback: ship per-arch images, not a static binary. |
| **`secret run` cold start > 300 ms** | Medium | Medium | Bench gate in CI (Task 11). Mitigations: small dep tree, no init-time HTTP, lazy cobra subcommand binding. |
| **Master key misconfig** (the PRD's #1 named risk) | Medium | High | Refuse-to-boot on mode > `0600` (Task 3). Docs explicit (Task 13). Auto-gen on empty-dir first boot (Task 12) avoids "operator copies an insecure example". |
| **Inline reference resolution turns into a template language** | Medium | Medium | Spec is fixed: `${{ <env>.<KEY> }}` only, no expressions, no functions. Reject anything else with parse error. |
| **Worktree resolution surprises operator** (wrong env injected silently) | Medium | High | `secret run` prints `→ resolved env=<x> via <source>` to stderr unless `--quiet`. Test in Task 10. |
| **Audit log grows unbounded on SQLite** | Low | Low | M1 acceptable. Rotation/retention deferred (track as M4-adjacent). |
| **Bootstrap token leaks via stdout** (Docker logs) | Low | Medium | Documented; recommend `docker compose logs` then redact. Acceptable in self-host threat model. |

## Acceptance

- [ ] All 14 tasks complete; each task's validation step passes.
- [ ] `go test ./... -race -cover` ≥ 80% per package; ≥ 85% overall.
- [ ] `docker compose up` on clean VM → healthy in ≤ 120 s.
- [ ] `secret run -- printenv` cold start p95 ≤ 300 ms (bench gate).
- [ ] Cross-compile for `linux/{amd64, arm64, arm/v7}` succeeds in CI.
- [ ] **Operator dogfood (Task 14): own 12 `.env` files replaced; adding one new envvar to all envs ≤ 1 min** — this is the PRD's gating success metric for M1.
- [ ] Quickstart doc walkthrough completes in ≤ 5 min on a clean clone.
- [ ] Threat model doc states master key model and refuse-to-boot rule.
- [ ] No decrypted secret written to disk during `secret run` (filesystem audit test).
- [ ] PRD Milestone 1 row updated to `done` with this plan linked.

---

*Generated by `/plan` on 2026-05-30. Source PRD: [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md).*
