# Local Review: `feat/dashboard-ui` (M2 Tasks 1·2·3)

**Reviewed**: 2026-05-31
**Branch**: `feat/dashboard-ui` (worktree `.worktrees/dashboard-ui`)
**Commits ahead of master**: 2
- `36bd508` feat: M2 Task 1·2 — 대시보드용 read/write API 추가 (audit, diff, rollback, delete, get-version)
- `245a7d0` feat: M2 Task 3 — 대시보드 브라우저 세션 + CSRF 미들웨어

**Diffstat**: 33 files, +3,706 / −38 (≈3,150 LoC after subtracting tests)
**Decision**: **APPROVE WITH COMMENTS** — no blocker, but two MEDIUMs are likely to be felt on first dashboard boot and on the rollback-of-a-deleted-secret flow.

## Summary

Tasks 1·2·3 of the M2 plan land cleanly: the new read-side endpoints (`get-version`, `env diff`, `audit list`), write-side endpoints (`rollback`, `delete`), and the cookie/CSRF browser-session flow all follow the established M1 patterns (envelope, sentinel→status mapping, transactional audit). Tests are thorough — 6 new test files, coverage ≥ 80% on every changed package (auth 86.3%, server 80.4%, store 83.1%). The CSRF design (double-submit, SHA-256 stored, constant-time compare) and the IP-prefix privacy treatment are good. Soft-delete semantics are coherent and let the dashboard keep rendering deleted-key timelines.

The findings below are mostly operator-UX gaps, not security or correctness defects.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM

#### M-1 — `Secure: true` cookie will silently drop on the documented `docker compose up` localhost flow

[internal/server/handlers_sessions.go:121-130](.worktrees/dashboard-ui/internal/server/handlers_sessions.go#L121-L130) sets the session cookie with `Secure: true` unconditionally. Per spec, browsers refuse to store `Secure` cookies set over plain HTTP. The plan's own Validation block ([plan §Validation](.worktrees/dashboard-ui/.claude/plans/comax-secrets-dashboard.plan.md#L226-L229)) and `docs/quickstart.md` both walk the operator through `http://localhost:8080` — so the first dashboard login over the documented quickstart will return 201 + a CSRF token, but no cookie persists, and the next request 401s. That's a confusing first-run experience.

Two reasonable fixes:
1. Gate `Secure` on `r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"`.
2. Introduce a `COMAX_DASHBOARD_INSECURE_COOKIE=true` env (loud startup warning) and document the trade-off.

(1) is closer to standard cookie hygiene and matches how popular Go HTTP frameworks ship session cookies. (2) keeps the threat model conservative by default. Either way, the `docker compose up` smoke needs to actually exercise the cookie round-trip, not just `curl /healthz`.

#### M-2 — Rollback of a soft-deleted secret returns a generic 404 the dashboard cannot disambiguate

[internal/server/handlers_secrets.go:252-260](.worktrees/dashboard-ui/internal/server/handlers_secrets.go#L252-L260) uses `SecretRepo.ByKey` (live rows only) and returns the generic `not_found` envelope when the live row is missing. The dashboard's history/timeline view will be rendered from `handleListVersions` (which uses `ListByEnvAny` and so shows deleted keys' history), so the operator will see the rollback button on a deleted-key row, click it, and get the same 404 they'd get for a typo'd key name.

The handler's own docstring at lines 220-226 acknowledges this is a deferred deviation from the plan's `?undelete=true` flag. Even without re-introducing that flag, returning a distinct envelope (`409 secret_deleted` or `404 secret_deleted`) would let the dashboard surface a useful "Restore first, then roll back" prompt. Minimal change inside the handler, no schema work.

#### M-3 — `DELETE /api/v1/dashboard/session` is in `isPublicPath`, bypassing CSRF

[internal/server/middleware.go:200-206](.worktrees/dashboard-ui/internal/server/middleware.go#L200-L206) matches by path only. Both `POST` and `DELETE` on `/api/v1/dashboard/session` therefore skip the auth middleware — and so skip the CSRF check the plan's Task 3 explicitly calls for on mutating methods. `handleRevokeDashboardSession` itself does not re-verify CSRF.

Mostly mitigated by `SameSite=Strict` (the browser won't ship the session cookie on a cross-origin POST/DELETE), so the practical attack surface is small — a malicious page can't forge a logout. The reason to fix it anyway: it diverges from the plan's stated invariant ("CSRF enforced on every mutating method"), and a future reviewer reading `isPublicPath` won't notice the silent exception for DELETE. Either:
- Split `isPublicPath` into `(method, path)` pairs so only `POST /api/v1/dashboard/session` is exempt, and let the standard `authMiddleware` → `authSession` → CSRF flow handle DELETE.
- Or call `auth.VerifyCSRF` explicitly inside `handleRevokeDashboardSession` before revoking.

### LOW

- **`SessionRepo.Prune` is implemented but never scheduled.** The plan's risk table calls out "session table grows unbounded" with mitigation "Prune called on a 1-h goroutine". The repo method exists ([session_repo.go:139](.worktrees/dashboard-ui/internal/store/session_repo.go#L139)) but no goroutine in `cmd/server` or `Server.Run` invokes it. Acceptable as deferred work but worth a TODO comment near the repo method or a follow-up task in the plan.

- **`ByHash` defense-in-depth compare is misleading.** [session_repo.go:96-99](.worktrees/dashboard-ui/internal/store/session_repo.go#L96-L99) re-confirms with `ConstantTimeCompare` after `WHERE session_hash = ?` returned a row. The comment ("never trust the WHERE clause alone") implies a timing-side-channel mitigation, but the timing channel is the SELECT itself — adding a constant-time compare on the returned bytes doesn't close it. The compare is effectively dead code (it never returns `ErrNotFound` in practice). Either delete it and shorten the comment, or document it as a regression guard against a future driver short-circuit bug rather than a timing defense.

- **`clearSessionCookie` sets only `MaxAge=-1`, not `Expires` in the past.** Modern browsers accept this, but a belt-and-braces `Expires: time.Unix(0,0)` costs nothing.

- **`session.create` / `session.revoke` audit metadata is empty.** [plan §Patterns to Mirror](.worktrees/dashboard-ui/.claude/plans/comax-secrets-dashboard.plan.md#L43) calls for `user_agent` + truncated remote IP in audit metadata. The current implementation puts everything in the `target` field instead and passes `""` for metadata via `appendAuditForToken`. Both are persisted to the DB, just under a different column. Consider relocating to `metadata` (which is JSON-shaped) so the dashboard's audit view can render structured rows.

- **`audit` LIKE filter can match across `project=` / `env=` substrings.** [audit_repo.go:80-87](.worktrees/dashboard-ui/internal/store/audit_repo.go#L80-L87). The handler docstring already calls this out and `validateName` blocks LIKE wildcards, so there's no injection or false-positive on safe names. Documented limitation; flag for the M3 schema split.

- **`handleListVersions` returns flat versions without the `key` field.** [handlers_versions.go:127-136](.worktrees/dashboard-ui/internal/server/handlers_versions.go#L127-L136). The dashboard needs a secondary call (`ListByEnvAny` shape) to map `secret_id` → key. Deferred per plan; not blocking.

- **Race detector not run in this validation pass.** The project's policy is `CGO_ENABLED=0`, and the local Windows gcc on this checkout doesn't support 64-bit mode, so `go test -race` cannot run here. CI should still run `-race` on linux/amd64 before merge.

### INFO / Positive observations

- The CSRF crypto is clean: SHA-256 storage, constant-time compare, `crypto/rand` source, `base64.RawURLEncoding` for transport-safe headers. ([internal/auth/csrf.go](.worktrees/dashboard-ui/internal/auth/csrf.go))
- The double-arm `authMiddleware` (bearer first, cookie second) preserves CLI behavior and explicitly documents *why* bearer wins when both are present. The cookie arm rehydrates the underlying `ServiceToken` so audit attribution stays consistent. ([middleware.go:93-181](.worktrees/dashboard-ui/internal/server/middleware.go#L93-L181))
- `IPPrefix` (/24 v4, /48 v6) is a tasteful privacy choice — preserves enough for blast-radius investigation without persisting an exact operator location.
- The new sentinel `ErrVersionNotFound` (distinct from `ErrNotFound`) lets the dashboard distinguish "version 5 was never written" from "secret was never created". Small but useful.
- The `additiveColumns` migration dance is a thoughtful M1→M2 upgrade path that avoids gating a fresh DB on the duplicate-column error.
- Test discipline is strong — six new test files, deliberate negative-path coverage (revoked / expired / dup / CSRF missing / unknown actor / bad cursor), and the coverage gate is met without padding.

## Validation

| Check | Result | Notes |
|---|---|---|
| `go build ./...` | ✅ Pass | Clean. |
| `go vet ./...` | ✅ Pass | No diagnostics. |
| `gofmt -l` | ⚠️ Noisy | Working-tree CRLF (autocrlf=true). Committed blobs are LF-clean — verified via `git show`. Not a real defect. |
| `go test -count=1 ./...` | ✅ Pass | 12 packages, all green. |
| `go test -cover` (changed pkgs) | ✅ Pass | auth 86.3%, server 80.4%, store 83.1% — all ≥ 80% gate. |
| `go test -race` | ⏭ Skipped | `CGO_ENABLED=0` per project policy; local gcc cannot build 64-bit cgo. Run on linux/amd64 in CI before merge. |

## Files Reviewed (source — tests not listed individually)

- **Added**: `internal/auth/csrf.go`, `internal/server/handlers_audit.go`, `internal/server/handlers_sessions.go`, `internal/store/session_repo.go`
- **Modified**: `internal/server/middleware.go`, `internal/server/router.go`, `internal/server/response.go`, `internal/server/handlers_envs.go`, `internal/server/handlers_secrets.go`, `internal/server/handlers_versions.go`, `internal/server/handlers_projects.go`, `internal/store/secret_repo.go`, `internal/store/version_repo.go`, `internal/store/audit_repo.go`, `internal/store/token_repo.go`, `internal/store/migrate.go`, `internal/store/schema.sql`, `internal/store/store.go`

## Next steps

1. Decide on M-1 (Secure cookie / quickstart UX) before Task 5 starts — the SPA login flow lives or dies on the cookie round-trip.
2. Decide whether rollback-on-deleted is M-2 or stays a 404 for the milestone; align with whatever the dashboard's history-view UX assumes.
3. Add a follow-up TODO for M-3 + `SessionRepo.Prune` scheduling. Both are small but easy to lose by Task 12.
4. Run `go test -race ./...` in CI on linux/amd64.
