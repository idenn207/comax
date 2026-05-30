# Implementation Report: Comax Secrets — M1 Tasks 4–10

Plan: [.claude/plans/comax-secrets.plan.md](../plans/comax-secrets.plan.md)
Branch: `feat/self-host-server-cli`
Date: 2026-05-30

## Summary

Implemented Tasks 4–10 of the Comax Secrets Milestone 1 plan: the auth/bootstrap flow, the REST API behind a stdlib router, the inline-reference + inheritance resolver, and the full CLI surface (`login`, `init`, `pull`, `push`, `set`, `get`, `diff`, `run`). The previous session committed Tasks 1–3 (repo bootstrap, SQLite store, crypto layer); this session built on those primitives without modifying their public APIs except for two additive extensions (`TokenRepo.BootstrapIfEmpty`, `EnvRepo.ByID`).

Tasks 11 (bench gate), 12 (docker compose), 13 (quickstart/threat model docs), and 14 (operator dogfood) are intentionally out of scope per the user's session-scoping question.

## Assessment vs Reality

| Metric | Predicted (Plan) | Actual |
|---|---|---|
| Complexity | Large | Confirmed large; 7 tasks ≈ 8 hours of focused work |
| Files Changed | ~25 across 4 packages | 33 files across 11 packages |
| Coverage (overall) | ≥ 85% (plan) / ≥ 70% (CI) | **81.3% per `./internal/...`** — above CI gate, slightly below plan stretch |
| New tests | Unspecified | ~110 tests across 18 test files |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 4 | Auth + bootstrap flow | ✅ Complete | Added `TokenRepo.BootstrapIfEmpty` with single-statement race-safety; `internal/auth` package with `GenerateToken`, `HashToken`, `Verify`, `ParseBearer`, `Bootstrap` |
| 5 | REST handlers | ✅ Complete | stdlib `http.ServeMux` (Go 1.22+ pattern routing) instead of chi — documented deviation. 9 endpoints + middleware ring (auth, log, recover) + envelope shape |
| 6 | Inline `${{ env.KEY }}` + inheritance | ✅ Complete | `internal/secret` package; both cycle detectors (inheritance + reference); resolver interface refactored from per-string to per-env snapshot |
| 7 | CLI skeleton + login/init | ✅ Complete | Cobra (no viper, docs deviation); `pkg/client`; `internal/cli/credentials` with atomic-write + 0600; `internal/cli/secretrc` |
| 8 | Worktree context resolution | ✅ Complete | 7-step precedence chain; pure resolver + production `Load` wiring; shells out to `git branch --show-current` rather than pulling a git library |
| 9 | pull / push / set / get / diff | ✅ Complete | Plus `internal/cli/dotenv` parser/emitter with BOM stripping, escapes, single+double quoting, idempotent round-trip |
| 10 | `secret run -- <cmd>` | ✅ Complete | In-memory env merge; child sees pulled values; **directory-snapshot audit confirms no plaintext on disk**; parent env vars shadowed by secrets |

## Deviations from Plan

| # | What | Why |
|---|---|---|
| 1 | Used stdlib `http.ServeMux` instead of `chi` | Go 1.22's `"METHOD /path/{param}"` pattern routing covers both features we'd have used chi for (method matching + path params); removes a dep without losing capability |
| 2 | Used Cobra alone, dropped Viper | Plan said "Cobra+Viper". Config is two JSON files (credentials + .secretrc); a 40-line custom loader is lighter than Viper and cleaner to test. Cold-start budget concern (Task 11) was a factor |
| 3 | Added `_pragma=busy_timeout(5000)` to `store.Open` | The bootstrap concurrent test flaked under `-cover` instrumentation because all writers got SQLITE_BUSY before any could commit. Setting a busy timeout is the production-correct fix — operators shouldn't have to write retry loops for a single-writer DB. Production behavior change, documented in `store.go` |
| 4 | Resolver interface changed mid-Task-6 | Initial Task 5 design was `Resolve(plaintext) → plaintext`; Task 6 inheritance required a snapshot-shaped interface. Refactored: handlers delegate the full env-decryption to the resolver. Old `noopResolver` removed |
| 5 | Added 2 store-layer methods | `TokenRepo.BootstrapIfEmpty` (single-statement race-safe insert); `EnvRepo.ByID` (resolver walks inheritance chain by ID). Both purely additive |

## Validation Results

| Check | Result |
|---|---|
| `go test ./...` | ✅ All packages pass |
| `go vet ./...` | ✅ Clean |
| Per-internal coverage (CI gate ≥70%) | ✅ Total 81.3% |
| Cross-compile linux/amd64 | ✅ |
| Cross-compile linux/arm64 | ✅ |
| Cross-compile linux/arm/v7 | ✅ |
| Cross-compile server linux/amd64 | ✅ |
| CGO required? | ❌ All builds CGO-free (modernc.org/sqlite) |

Note: `-race` couldn't run locally on Windows (the user's MinGW is 32-bit and Go's race detector needs CGO + 64-bit C). CI on `ubuntu-latest` runs the race detector against the same test suite.

## Coverage by Package

| Package | Coverage | Plan Target (80%) |
|---|---|---|
| `internal/auth` | 83.0% | ✅ |
| `internal/cli/credentials` | 66.0% | ⚠️ Below target; remaining branches are filesystem error-paths (chmod/write/close failure injection) |
| `internal/cli/dotenv` | 90.9% | ✅ |
| `internal/cli/envctx` | 91.3% | ✅ |
| `internal/cli/secretrc` | 85.0% | ✅ |
| `internal/crypto` | 85.7% | ✅ |
| `internal/secret` | 80.6% | ✅ |
| `internal/server` | 71.9% | ⚠️ Below target; uncovered branches are internal-error paths needing DB fault injection |
| `internal/store` | 90.8% | ✅ |
| `internal/version` | 100% | ✅ |
| **Total (./internal/...)** | **81.3%** | ✅ |

CI is gated on the total (≥70%), which is met comfortably.

## Files Changed

### Server-side (Tasks 4–6)

| File | Action | Why |
|---|---|---|
| `internal/store/token_repo.go` | UPDATED | Added `BootstrapIfEmpty` (race-safe single-statement insert) |
| `internal/store/token_repo_test.go` | UPDATED | +2 tests for `BootstrapIfEmpty` |
| `internal/store/env_repo.go` | UPDATED | Added `ByID` for resolver use |
| `internal/store/env_repo_test.go` | UPDATED | +2 tests for `ByID` |
| `internal/store/store.go` | UPDATED | Added `busy_timeout=5000` pragma + `appendPragma` helper |
| `internal/auth/token.go` | CREATED | Token primitives, context plumbing |
| `internal/auth/token_test.go` | CREATED | 13 tests |
| `internal/auth/bootstrap.go` | CREATED | Bootstrap orchestrator |
| `internal/auth/bootstrap_test.go` | CREATED | 4 tests inc. concurrent race-safety test |
| `internal/server/response.go` | CREATED | Envelope shape + error mapping |
| `internal/server/validate.go` | CREATED | Name validation |
| `internal/server/resolver.go` | CREATED | Resolver interface (post-refactor: snapshot-shaped) |
| `internal/server/server.go` | CREATED | `Server` type, `NewServer` |
| `internal/server/middleware.go` | CREATED | recover, log, auth middleware |
| `internal/server/router.go` | CREATED | stdlib mux assembly |
| `internal/server/handlers_bootstrap.go` | CREATED | `/bootstrap`, `/healthz` |
| `internal/server/handlers_projects.go` | CREATED | projects CRUD + `appendAudit` helper |
| `internal/server/handlers_envs.go` | CREATED | envs CRUD + `resolveProject` |
| `internal/server/handlers_secrets.go` | CREATED | secrets CRUD (encrypt/decrypt via resolver) |
| `internal/server/handlers_versions.go` | CREATED | version history list |
| `internal/server/server_test.go` | CREATED | 11 integration tests |
| `internal/server/error_paths_test.go` | CREATED | 9 tests for error branches |
| `internal/secret/reference.go` | CREATED | `${{ env.KEY }}` regex + parser |
| `internal/secret/resolver.go` | CREATED | Inheritance + reference resolver with cycle detection |
| `internal/secret/reference_test.go` | CREATED | 8 parser tests |
| `internal/secret/resolver_test.go` | CREATED | 10 resolver tests |

### Client-side (Tasks 7–10)

| File | Action | Why |
|---|---|---|
| `pkg/client/client.go` | CREATED | HTTP client for CLI + future SDK |
| `internal/cli/credentials/credentials.go` | CREATED | Atomic-write, 0600 on Unix |
| `internal/cli/credentials/credentials_test.go` | CREATED | 10 tests |
| `internal/cli/secretrc/secretrc.go` | CREATED | `.secretrc` reader/writer |
| `internal/cli/secretrc/secretrc_test.go` | CREATED | 4 tests |
| `internal/cli/envctx/envctx.go` | CREATED | 7-step precedence resolver + production wiring |
| `internal/cli/envctx/envctx_test.go` | CREATED | 10 tests |
| `internal/cli/dotenv/dotenv.go` | CREATED | Parser + emitter |
| `internal/cli/dotenv/dotenv_test.go` | CREATED | 10 tests inc. BOM, escapes, quoting, round-trip |
| `cmd/cli/main.go` | UPDATED | Cobra root, 8 subcommands wired |
| `cmd/cli/cmd_login.go` | CREATED | login + load/save helpers |
| `cmd/cli/cmd_init.go` | CREATED | Idempotent project+env setup |
| `cmd/cli/cmd_pull.go` | CREATED | Pull + atomic .env write + `loadContext` helper |
| `cmd/cli/cmd_push.go` | CREATED | Parse .env, PUT each |
| `cmd/cli/cmd_getset.go` | CREATED | get (composable) + set (KEY=VALUE) |
| `cmd/cli/cmd_diff.go` | CREATED | Added/removed/changed report |
| `cmd/cli/cmd_run.go` | CREATED | In-memory env merge, child spawn, no disk plaintext |
| `cmd/cli/cli_integration_test.go` | CREATED | login + init tests (5) |
| `cmd/cli/cli_dataflow_test.go` | CREATED | push/pull/set/get/diff tests (6) |
| `cmd/cli/cli_run_test.go` | CREATED | `secret run` tests inc. no-plaintext-on-disk audit (4) |
| `go.mod`, `go.sum` | UPDATED | Added cobra (and transitives) |

## Issues Encountered

| # | Issue | Resolution |
|---|---|---|
| 1 | `-race` test runs failed locally on Windows (32-bit MinGW can't compile race runtime) | Documented; CI runs `-race` on `ubuntu-latest`. Local tests use `go test` without `-race` |
| 2 | Bootstrap concurrent test flaked under `-cover` instrumentation (all writers hit SQLITE_BUSY) | Added `busy_timeout=5000` pragma to `store.Open` — production-correct fix |
| 3 | Go parser rejects raw BOM inside string literals | Used `"\xEF\xBB\xBF"` escape sequence in both source and test |
| 4 | Struct with map field can't be compared with `!=` (secretrc Config has `Branches` map) | Switched test comparison to `reflect.DeepEqual` |
| 5 | Resolver interface changed mid-Task-6 (single-string → snapshot) | Refactored handlers to delegate to resolver; old `noopResolver` removed |
| 6 | Several non-existent method references (`EnvRepo.DB()`, `envctx.LoadSecretrc`) during cascading edits | Inline fixes; final compile clean |

## Architectural Decisions

1. **Resolver as a separate layer**: `internal/secret.Resolver` owns the entire decrypted-value pipeline (key load → DB fetch → decrypt → inherit merge → reference expand). The server's handlers just call `Resolve(projectID, envID) → Snapshot` and format. This is the only sane place to express inheritance + references together since they cross env boundaries.

2. **CI-covered logic in `internal/cli/*`, glue in `cmd/cli/*`**: CI's coverage gate only runs against `./internal/...`. Anything testable goes in `internal/cli/<feature>` packages. The cobra commands stay thin wrappers, exercised by integration tests that drive `rootCmd.SetArgs + Execute` — the same code path the real binary takes minus `os.Args` parsing.

3. **Stdlib `http.ServeMux` over chi**: Go 1.22+'s native pattern routing makes the chi dep redundant. One less dep means cleaner cross-compile and faster cold start.

4. **Cobra without Viper**: Two JSON files (credentials, .secretrc) don't justify Viper's surface. A 40-line custom loader for each is lighter and tests cleaner.

5. **Single-statement bootstrap**: `INSERT ... SELECT ... WHERE (SELECT COUNT(*)) = 0` is atomic at the SQLite statement level. Combined with the `name UNIQUE` constraint as defence-in-depth, race-safety is provable rather than hoped-for. Verified by an 8-goroutine concurrent test.

6. **Atomic file writes**: Both credentials and `.env` writes use the temp-file-then-rename pattern. A crash mid-write leaves the previous contents intact.

## Next Steps

The plan's remaining tasks (out of session scope):

- **Task 11**: Cold-start benchmark + CI bench gate. The CLI binary is already structured for fast start (lazy cobra subcommand binding, small dep tree, no init-time HTTP); the bench would just measure and gate it.
- **Task 12**: Docker packaging (multi-stage Dockerfile + compose with `./data` and `./keys/master.key` mounts, auto-bootstrap on empty key dir).
- **Task 13**: `docs/quickstart.md` + `docs/threat-model.md`.
- **Task 14**: Operator dogfood — migrating the 12 real `.env` files and timing the "add one envvar to all envs" workflow. This is the PRD's gating success metric.

Recommended order for a follow-up session: 12 → 13 → 11 → 14. Docker first because everything else depends on having a deployable artifact for the operator to point at.
