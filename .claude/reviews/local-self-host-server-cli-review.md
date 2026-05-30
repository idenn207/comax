# Local Review: feat/self-host-server-cli

**Reviewed**: 2026-05-30
**Branch**: `feat/self-host-server-cli` (worktree: `.worktrees/self-host-server-cli`)
**Scope**: 8 modified + ~30 untracked Go files (Tasks 4–10 of M1)
**Reviewers**: security-reviewer agent, go-reviewer agent, main thread cross-check
**Decision**: **REQUEST CHANGES** — block on HIGH findings before merge to `main`

## Summary

Solid M1 work: crypto, auth, server, secret resolver, and CLI all land in clean,
well-documented Go with 81.3% coverage and ~110 tests. The implementation report
is thorough and the design decisions documented in code are defensible.

Two blocking issues prevent a clean approval:
1. The server binary (`cmd/server/main.go`) is still the M1-Task-1 stub — it
   prints version and exits. The HTTP handlers it depends on are fully tested
   via `httptest.NewServer`, but no production listener exists.
2. `secret run -- <cmd>` does not forward signals to the child, so a SIGTERM'd
   parent CLI orphans a child process whose `/proc/PID/environ` still holds the
   plaintext secrets.

Everything else is MEDIUM or below and can be folded into the next pass.

---

## Findings

### CRITICAL
_None._

### HIGH

**H1. Server binary is a stub — no HTTP listener, no timeouts, no DB wiring.**
`cmd/server/main.go:1-26` still runs the Task-1 placeholder (`fmt.Fprintln(out, "secret-server", version.String())`). The `Server.Handler()` produced by `internal/server` is never served by any binary. Operators cannot run the M1 server as-is. Acknowledged as Task 12 scope in the report, but the binary cannot be merged in this state without misleading anyone who runs it.
**Fix**: Wire `cmd/server/main.go` to open the SQLite store, load the key provider, construct `server.NewServer(...)`, and start `http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, Handler: s.Handler()}.ListenAndServe()`. Signal-driven graceful shutdown via `srv.Shutdown(ctx)` on SIGINT/SIGTERM.

**H2. `secret run` does not forward signals to the child process.**
[cmd/cli/cmd_run.go:58-75](.worktrees/self-host-server-cli/cmd/cli/cmd_run.go#L58-L75) spawns via `exec.Command(args[0], args[1:]...).Run()` without `SysProcAttr` for process-group setup or `signal.Notify` to forward SIGTERM/SIGINT. If the parent CLI is killed by a supervisor (systemd, k8s, supervisord), the child is reparented to init and continues holding plaintext secrets in `/proc/PID/environ` (Linux) until it exits on its own. The "no plaintext on disk" invariant is preserved, but "no plaintext in an orphaned, possibly-immortal child" is not.
**Fix**: Set `SysProcAttr` to put the child in its own process group, register `signal.Notify(c, os.Interrupt, syscall.SIGTERM)`, and on signal call `child.Process.Signal(sig)` then wait. Add a regression test that asserts the child exits within N seconds of SIGTERM to the parent.

### MEDIUM

**M1. Coverage artifacts are untracked but unstaged — risk of accidental commit.**
`cov_auth/`, `cov_phase1/`, `cov_server/` appear in `git status` as untracked. They are coverage profile output from `go test -coverprofile`. A future `git add -A` will commit them.
**Fix**: Add `cov_*` (or `coverage*`) to `.gitignore` at the worktree root.

**M2. Line-ending churn: every modified Go file warns "LF will be replaced by CRLF on next git touch".**
This will produce noisy diffs on Windows and make `git blame` unstable.
**Fix**: Add `.gitattributes` with `*.go text eol=lf` (and similar for `.mod`, `.sum`, `.md`).

**M3. Validate.go allows `.` and `..` as names (charset-only validation).**
[internal/server/validate.go:14-17](.worktrees/self-host-server-cli/internal/server/validate.go#L14-L17): `_-.+` is in the allowed extra charset, so `.`, `..`, `.git` are accepted as project/env/secret names. URL safety is preserved (no `/`), but client-side `cmd_pull.go` writes `.env.<envname>` style files and other tooling may use these as filename fragments later. Defense-in-depth: reject pure dots.
**Fix**: After charset check, reject names that are `.`, `..`, or start with `.` (or whitelist a stricter pattern).

**M4. Bootstrap endpoint is unauthenticated and unrate-limited.**
`internal/server/middleware.go:111` exempts `/api/v1/bootstrap` from auth (correct — there's no token yet). The handler then guards via `auth.Bootstrap` which returns 409 once a token exists. Race-safe, but an attacker can hammer `/bootstrap` after the fact and the server will happily respond 409 forever, consuming CPU + audit-log rows (the audit insert only fires when `created=true`, so this is bounded).
**Fix**: Optional rate-limit on public paths; document the assumption in the threat model.

**M5. `cmd_login.go:75` discards `credentials.Path()` error.**
Flagged by the go-reviewer; verify the line and convert `p, _ := credentials.Path()` to capture-and-return.

**M6. `cmd_pull.go:43`-area: deferred `f.Close()` drops error before rename.**
Atomic-write pattern needs explicit `f.Close()` with error check **before** `os.Rename` — otherwise a buffered-write failure can be lost and the rename promotes a truncated file.
**Fix**: Move the close to an explicit call before rename; deferred Close only as a no-op safety net.

### LOW

**L1. Windows credentials file ACL is not enforced.**
[internal/cli/credentials/credentials.go:113-119](.worktrees/self-host-server-cli/internal/cli/credentials/credentials.go#L113-L119) skips `Chmod` on Windows by design. Documented in the package doc. Acceptable for a single-operator self-hosted tool; revisit if multi-user Windows machines become a target.

**L2. Subprocess plaintext lives in CLI heap.**
Inherent to env injection. Mitigated by "no disk" invariant. Document in threat model alongside the `/proc/PID/environ` note from H2.

**L3. `recoverMiddleware` logs `panic` value and `stack` via slog.**
[internal/server/middleware.go:33-46](.worktrees/self-host-server-cli/internal/server/middleware.go#L33-L46) logs the recovered panic argument verbatim. If a downstream library ever panics with a secret-bearing value (e.g. SQL driver echoing a parameterised query in a malformed-prepare error), it lands in logs. Low likelihood for the current dep set; worth a defensive `slog.String("panic", fmt.Sprint(rec))` with a length cap.

**L4. Token-lookup timing leak via SQL WHERE clause.**
The `subtle.ConstantTimeCompare` at [internal/auth/token.go:109](.worktrees/self-host-server-cli/internal/auth/token.go#L109) is correctly described as defense-in-depth. The actual timing leak is the WHERE clause itself. Bearer tokens are 256-bit random (`tokenBytes = 32`), so this is infeasible in practice. **No action — document only.**

---

## Verified Non-Issues (rejected agent findings)

- **Cross-project cache leak in resolver** (security-reviewer MEDIUM #3): False positive. `resolveCtx` is constructed per `Resolve()` call with `projectID` baked in ([resolver.go:71-79](.worktrees/self-host-server-cli/internal/secret/resolver.go#L71-L79)); the `envByNameID` cache cannot span calls or projects.
- **`exec.Command` command injection in `cmd_run.go`**: No shell is invoked. `exec.Command(args[0], args[1:]...)` is the injection-immune form. The `//nolint:gosec` comment is correct.
- **Audit metadata forgery**: All inputs flow through `validateName` (charset restricted, no newlines/commas), so audit lines cannot be forged. Defense-in-depth already in place.

---

## Validation Results

Author's report claims (per `.claude/reports/comax-secrets-m1-tasks-4-10.report.md`):

| Check | Result |
|---|---|
| `go test ./...` | Pass (per author report) |
| `go vet ./...` | Pass (per author report) |
| Coverage `./internal/...` | 81.3% (≥ 70% CI gate, ≥ 80% plan target met) |
| Cross-compile linux/amd64, arm64, arm/v7 | Pass |
| `-race` on Windows | Skipped (32-bit MinGW); deferred to CI |

**Not independently re-run** in this review pass. Recommend the next pass execute the test suite + `gosec` (Go security-focused linter) against the worktree before merge.

---

## Files Reviewed

**Modified (8)**: `cmd/cli/main.go`, `go.mod`, `go.sum`, `internal/store/env_repo.go`, `internal/store/env_repo_test.go`, `internal/store/store.go`, `internal/store/token_repo.go`, `internal/store/token_repo_test.go`

**Added — server (15)**: `cmd/server/main.go` (stub, see H1), `internal/auth/{token,bootstrap}.go` + tests, `internal/server/{server,router,middleware,response,validate,resolver}.go`, `internal/server/handlers_{bootstrap,projects,envs,secrets,versions}.go`, `internal/server/{server_test,error_paths_test}.go`, `internal/secret/{reference,resolver}.go` + tests

**Added — CLI (13)**: `cmd/cli/cmd_{login,init,pull,push,getset,diff,run}.go`, `cmd/cli/{cli_integration_test,cli_dataflow_test,cli_run_test}.go`, `pkg/client/client.go`, `internal/cli/{credentials,secretrc,envctx,dotenv}/*.go` + tests

**Untracked artifacts (should be gitignored)**: `cov_auth/`, `cov_phase1/`, `cov_server/`

---

## Recommended Action

Before merging to `main`:
1. **H1** — wire `cmd/server/main.go` to a real `http.Server` with timeouts and graceful shutdown.
2. **H2** — add signal forwarding to `cmd_run.go` and a regression test.
3. **M1** — gitignore `cov_*`.
4. **M3, M5, M6** — quick surgical fixes.

H1 could legitimately move to a follow-up "Task 12 prep" PR if this branch is meant to land the M1 *library* surface only — but the slash command target `feat/self-host-server-cli` strongly implies a self-host-runnable server, which today's binary cannot do.
