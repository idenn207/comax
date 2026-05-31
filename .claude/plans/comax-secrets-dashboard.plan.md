# Plan: Comax Secrets — Milestone 2 (Dashboard UI — Doppler 스타일)

**Source PRD**: [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md)
**Selected Milestone**: #2 — Dashboard UI (Doppler 스타일)
**Complexity**: Medium-Large (greenfield SPA + ~6 new API endpoints; conventions inherited from M1)

## Summary

Add a single-page web dashboard that lets the operator manage projects / environments / secrets through the same `/api/v1` server M1 already ships, plus the additive endpoints required for the dashboard's headline screens (version diff, rollback, env-vs-env diff, audit log, secret delete, browser session). The dashboard ships as a static asset embedded into the `secret-server` binary via `embed.FS`, preserving M1's `docker compose up ≤ 2 min` and single-binary deploy promises. The marketing/docs site (PRD Milestone 6) stays a separate Next.js+Vercel codebase — this milestone is dashboard-only.

Success is binary: the operator can do **everything `secret` CLI already does** through the browser, plus the three things the CLI cannot do well — see version-by-version diff, roll back one secret to a prior version, and visually spot "key X exists in `local` but not `prod`".

## Resolved Open Questions (from PRD)

PRD Open Question **#4 — Dashboard vs Website codebase** is the load-bearing M2 decision. The other PRD questions belong to other milestones.

| PRD Q# | Decision | Rationale |
|---|---|---|
| **#4 Dashboard / Website split** | **Two codebases**: dashboard is a Vite+React SPA embedded into `secret-server` via `embed.FS`; marketing site (M6) is a separate Next.js+Vercel app sharing only a small `@comax/ui` design-token package. | The dashboard sits behind auth — SSR adds no SEO value and the static build is trivial to embed. Keeping it inside the server binary preserves the "single-binary, `docker compose up` is the deploy" UX that the PRD's Success Metrics depend on (≤ 2 min boot, NAS-friendly). The marketing site needs SSR/SEO + Vercel preview deploys, which is a different operational model — coupling them would force the dashboard to either pull in Next or force the marketing site to ship from the server binary; both lose. |

### M2-internal decisions (not in PRD; needed to unblock implementation)

| Decision | Choice | Rationale |
|---|---|---|
| **SPA stack** | Vite 5 + React 18 + TypeScript + TanStack Query v5 + Tailwind + Radix UI primitives | PRD locks Tailwind+Radix. Vite gives a pure-static `dist/` output that drops cleanly into `embed.FS`. TanStack Query is the right shape for an API-driven dashboard with optimistic mutations (per ECC `web/patterns.md` Stale-While-Revalidate). React stays because Radix primitives are React-first. |
| **Routing** | TanStack Router (file-based) | Type-safe params (project/env/key in URL — see ECC `web/patterns.md` "URL As State") with zero SSR runtime. Search-params validation lands free. |
| **Browser auth** | Service tokens, wrapped in an httpOnly Secure SameSite=Strict session cookie by new `POST /api/v1/dashboard/session`. No user accounts in v1. | The operator is the only user. Service tokens already exist; inventing a user/password/MFA model belongs to a multi-tenant SaaS scope the PRD explicitly excludes. Cookie + CSRF token gives the browser security model the CLI doesn't need. |
| **CSRF** | Double-submit token: cookie + `X-CSRF-Token` header required on mutating requests. CLI traffic (bearer header, no cookie) is exempt. | Per ECC `web/security.md`. Distinguishes browser vs CLI cleanly without a user-agent check. |
| **State** | Server state in TanStack Query; URL state in the route; minimal client state in component-local hooks. No global store. | Per ECC `web/patterns.md`: "Do not duplicate server state into client stores." |
| **Build artifact location** | `web/dashboard/dist/` → embedded by `internal/server/dashboard/embed.go` (//go:embed all:dist) → served at `/` for any non-`/api/`, non-`/healthz` path. SPA fallback (any unknown path → `index.html`). | Single source-of-truth artifact. CI builds `web/dashboard` first, then `go build` picks up the embedded files. |
| **Toolchain** | pnpm + corepack pinned in `web/dashboard/package.json`. Node 20 LTS in CI. | pnpm is the ECC default. Pinned via `packageManager` field so contributors don't pick the wrong package manager. |
| **Testing** | Vitest (unit) + React Testing Library (component) + Playwright (e2e against a real `secret-server` instance) | ECC `web/testing.md` priority order: visual + a11y + cross-browser, with Playwright handling e2e against the actual server (not mocks). |
| **Accessibility** | Radix primitives + automated `axe-playwright` checks in CI on every page. WCAG 2.2 AA per ECC `a11y` skill. | Radix gives keyboard + ARIA for free; CI enforces. |

## Patterns to Mirror

Most patterns inherit from M1. For frontend, the rules ship via ECC `web/*.md` — see the **Common rule precedence** note in M1's plan.

| Category | Source | Pattern |
|---|---|---|
| API response envelope | [`internal/server/response.go:23-35`](../../internal/server/response.go#L23-L35) | Every new M2 endpoint returns `{ ok, data, error, meta }`. The TypeScript client unwraps once at the fetch layer. |
| Error mapping | [`internal/server/response.go:66-83`](../../internal/server/response.go#L66-L83) | New M2 sentinels (`store.ErrVersionNotFound`, `secret.ErrCycleInRollback`, `auth.ErrCSRFMismatch`) extend the `switch` here, not a parallel system. |
| Auth + audit | [`internal/server/middleware.go:76-103`](../../internal/server/middleware.go#L76-L103), [`handlers_projects.go:91-99`](../../internal/server/handlers_projects.go#L91-L99) | New mutating endpoints (`rollback`, `delete secret`, `session create`) MUST call `appendAudit(...)` inside the same transaction as the mutation. Browser session create gets its own audit action `session.create` with `user_agent` + truncated remote IP in metadata. |
| Repository pattern | [`internal/store/secret_repo.go`](../../internal/store/secret_repo.go), [`internal/store/version_repo.go`](../../internal/store/version_repo.go) | New per-entity helpers (e.g. `VersionRepo.ByVersion`, `SecretRepo.Delete`, `SessionRepo`) follow the same `New<Repo>(DBTX)` shape so they compose with `tx`. |
| Slog logging | [`internal/server/middleware.go:51-63`](../../internal/server/middleware.go#L51-L63) | Bodies still never logged. Browser session cookies treated as secrets — `Authorization` and `Cookie` headers are dropped from any future request log expansion. |
| Test style | [`internal/server/server_test.go`](../../internal/server/server_test.go), [`internal/server/error_paths_test.go`](../../internal/server/error_paths_test.go) | New handler tests reuse `httptest.Server` + temp SQLite. Table-driven. Coverage gate ≥ 80% per package (CI floor 70% during scaffolding, raise on completion). |
| Frontend file layout | ECC `web/coding-style.md` | Organize by feature: `src/features/projects/`, `src/features/secrets/`, `src/features/audit/`, `src/components/ui/` for Radix wrappers, `src/lib/` for the API client + design tokens. |
| Anti-template policy | ECC `web/design-quality.md` | "Doppler-style" reference, but the dashboard is not a stock template. Bento layout for the project home, editorial typography contrast, real hover/focus/active states. Diff viewer treats data viz as part of the design system. |

## Files to Change

### Server (Go)

| File | Action | Why |
|---|---|---|
| `internal/store/schema.sql` | UPDATE | Add `dashboard_sessions` table (id, token_id FK, session_hash, csrf_hash, ua, ip_prefix, created_at, expires_at, revoked_at). Migrate.go's idempotent apply already handles the new `CREATE TABLE IF NOT EXISTS`. |
| `internal/store/session_repo.go` | CREATE | `SessionRepo.Create / ByHash / Revoke / Prune`. Stores `SHA-256(session_token)`, not the token. |
| `internal/store/session_repo_test.go` | CREATE | Table-driven + sentinel error paths. |
| `internal/store/secret_repo.go` | UPDATE | Add `Delete(ctx, envID, key) error` — soft semantics: rows in `secret_versions` retained; `secrets` row removed. Sentinel: `store.ErrNotFound` on unknown key. |
| `internal/store/version_repo.go` | UPDATE | Add `ByVersion(ctx, secretID, version int64) (SecretVersion, error)` so rollback/diff can fetch one row without scanning. |
| `internal/server/handlers_sessions.go` | CREATE | `POST /api/v1/dashboard/session` (accept bearer in body → set cookie + return CSRF token), `DELETE /api/v1/dashboard/session` (revoke). Public-prefix-aware because the *create* path takes a bearer in the body, not the header. |
| `internal/server/handlers_versions.go` | UPDATE | Add `handleGetVersion` (decrypted historical value — read only, no rollback side effect). Path `GET /api/v1/projects/{p}/envs/{e}/secrets/{k}/versions/{v}`. |
| `internal/server/handlers_secrets.go` | UPDATE | Add `handleRollbackSecret` (`POST .../secrets/{k}/rollback {target_version: N}` → fetch historical ciphertext, write as new `version+1`, audit `secret.rollback`). Add `handleDeleteSecret` (`DELETE .../secrets/{k}` → remove from `secrets`, keep `secret_versions`, audit `secret.delete`). |
| `internal/server/handlers_envs.go` | UPDATE | Add `handleDiffEnvs` (`GET .../envs/{e}/diff?against=<e2>` → returns `{ added: [...], removed: [...], changed: [{key, lhs_version, rhs_version}] }`. Resolver inheritance applied to both sides before diffing). |
| `internal/server/handlers_audit.go` | CREATE | `GET /api/v1/audit?project=&env=&actor=&action=&before=&limit=` paginated, default 50 / max 200. Sorted `created_at DESC`. Cursor pagination via `before=<id>`. |
| `internal/server/router.go` | UPDATE | Register the 6 new routes. Add SPA mount: `mux.Handle("GET /", http.HandlerFunc(s.handleSPA))` *as the last fallthrough*, swapping out the existing 404 catch-all. |
| `internal/server/handlers_spa.go` | CREATE | Serve from embedded `dist` FS. Any unknown path → `index.html` (SPA fallback). `Cache-Control: public, max-age=31536000, immutable` on hashed assets; `no-store` on `index.html`. CSP nonce middleware (see middleware update below). |
| `internal/server/middleware.go` | UPDATE | Add `csrfMiddleware` — for browser sessions (cookie-auth), require `X-CSRF-Token` header equal to the stored csrf token on POST/PUT/DELETE/PATCH. Bearer-auth requests skip CSRF. Add `cspMiddleware` — per-request nonce, applied only on SPA responses. |
| `internal/server/dashboard/embed.go` | CREATE | `//go:embed all:dist` + `func DistFS() fs.FS`. Empty in dev mode (build tag) so contributors don't need to build the frontend to run server tests. |
| `internal/server/dashboard/embed_dev.go` | CREATE | Build tag `!embed_dashboard` — returns an empty FS. Server logs `"dashboard: dev mode, /api only"` on boot. Documented in `docs/quickstart.md`. |
| `internal/server/dashboard/dist/` | CREATE (build output) | Vite output target. Symlinked or copied from `web/dashboard/dist/` by `make dashboard`. |
| `internal/auth/csrf.go` | CREATE | Generate + compare CSRF tokens. Constant-time compare. |
| `cmd/server/main.go` | UPDATE | Wire the new middlewares. Surface `--dashboard-enabled / COMAX_DASHBOARD_ENABLED` (default true). |
| `Makefile` | UPDATE | `make dashboard` (pnpm install + build + sync dist into Go embed dir). `make build` depends on `dashboard`. `make dev` runs Go + Vite dev server side-by-side with Vite proxying `/api` to `:8080`. |
| `.github/workflows/ci.yml` | UPDATE | Add a `dashboard` job (pnpm install, lint, typecheck, vitest, vite build, axe-playwright e2e against `make dev`). The Go build matrix depends on it. |

### Frontend (TypeScript)

| File | Action | Why |
|---|---|---|
| `web/dashboard/package.json` | CREATE | `vite`, `react`, `react-dom`, `@tanstack/react-router`, `@tanstack/react-query`, `tailwindcss`, `@radix-ui/react-*`, `zod`, `clsx`, `axe-playwright`. pnpm. |
| `web/dashboard/vite.config.ts` | CREATE | Output to `../../internal/server/dashboard/dist`. Proxy `/api` → `http://localhost:8080` in dev. Asset hashing on. |
| `web/dashboard/tailwind.config.ts` + `tokens.css` | CREATE | Design tokens per ECC `web/coding-style.md`. |
| `web/dashboard/src/main.tsx` + `routeTree.ts` | CREATE | Router root, TanStack Query client provider, Radix theme provider. |
| `web/dashboard/src/lib/api.ts` | CREATE | `fetch` wrapper that: includes credentials, attaches CSRF header from cookie on mutations, unwraps `{ ok, data, error }` once, throws typed `ApiError`. |
| `web/dashboard/src/lib/auth.ts` | CREATE | Login screen state machine: paste bearer → `POST /dashboard/session` → cookie set + CSRF returned → store CSRF in memory + sessionStorage. Logout flushes both. |
| `web/dashboard/src/features/projects/` | CREATE | List, create, navigate. |
| `web/dashboard/src/features/envs/` | CREATE | Env list per project, create env, inheritance picker, **env-vs-env diff view** consuming `GET .../diff`. |
| `web/dashboard/src/features/secrets/` | CREATE | Table of keys per env with masked values (toggle reveal), inline edit (creates new version), delete with confirm, copy-as-dotenv. |
| `web/dashboard/src/features/secret-history/` | CREATE | Per-key version timeline + side-by-side diff viewer (current vs selected historical), **rollback button** with confirm. |
| `web/dashboard/src/features/audit/` | CREATE | Paginated audit feed; filters for project/env/actor/action. |
| `web/dashboard/src/components/ui/` | CREATE | Radix-wrapped Button, Dialog, Toast, Tooltip, Table, Tabs, Combobox. Design-system file count modest (~10 files). |
| `web/dashboard/tests/e2e/*.spec.ts` | CREATE | Playwright: login → create project → push secret → see audit row → version timeline → rollback → verify env diff. axe-playwright per route. |
| `web/dashboard/.eslintrc.cjs`, `tsconfig.json`, `prettierrc` | CREATE | ECC defaults. |

### Docs

| File | Action | Why |
|---|---|---|
| `docs/dashboard.md` | CREATE | "Opening the dashboard" (URL = same as server, `/`), how to log in with a service token, what `--dashboard-enabled=false` does, threat-model implications (cookie scope, CSRF). |
| `docs/threat-model.md` | UPDATE | Add a "Browser sessions" section: cookie hardening, CSRF, session lifetime (default 30 days; configurable; revocable from the dashboard's own Sessions page), no XSS escape because CSP nonce is mandatory and React only renders text by default. |
| `docs/quickstart.md` | UPDATE | After the `secret login` step, add "Or open `http://localhost:8080` in a browser and paste the same token." |
| `README.md` | UPDATE | Add Dashboard line under "Quickstart". |

## API Additions (M2 scope)

All additive. Versioned under `/api/v1`. All require bearer **or** dashboard cookie except `POST /api/v1/dashboard/session`.

| Method | Path | Purpose | Consumer |
|---|---|---|---|
| `POST` | `/api/v1/dashboard/session` | Body: `{token}` → sets `comax_session` httpOnly cookie, returns `{csrf}` | Dashboard login |
| `DELETE` | `/api/v1/dashboard/session` | Revokes current session | Dashboard logout |
| `GET` | `/api/v1/projects/{p}/envs/{e}/secrets/{k}/versions/{v}` | Decrypted historical value (read-only) | Diff viewer |
| `POST` | `/api/v1/projects/{p}/envs/{e}/secrets/{k}/rollback` | Body: `{target_version}` → writes new version with that ciphertext, audit `secret.rollback` | Rollback button |
| `DELETE` | `/api/v1/projects/{p}/envs/{e}/secrets/{k}` | Removes from `secrets`; `secret_versions` retained | Secret delete |
| `GET` | `/api/v1/projects/{p}/envs/{e}/diff?against=<e2>` | `{ added, removed, changed: [...] }` | Env diff view |
| `GET` | `/api/v1/audit?project=&env=&actor=&action=&before=&limit=` | Paginated audit log | Audit feed |

> The existing `GET /api/v1/projects/{p}/envs/{e}/versions` already ships from M1 and is reused — no shape change.

## Tasks

Ordered so the API ships before the screens that consume it, the auth model lands before any screen that mutates, and the embedded-deploy story is proven before any operator-visible polish.

### Task 1: API additions — read-side (versions, env diff, audit, get historical value)

- **Action**: Implement `handleGetVersion`, `handleDiffEnvs`, `handleListAudit`. Reuse the existing resolver for diff inputs so inheritance is honored. Cursor-paginated audit. Add `store.VersionRepo.ByVersion`, `store.AuditRepo.List(ctx, filter, limit)`.
- **Mirror**: handler style + error sentinel pattern from M1 `handlers_secrets.go`.
- **Validate**: `go test ./internal/server/... -race -cover` ≥ 80% on changed files. Manual: `curl` each endpoint against a seeded compose instance.

### Task 2: API additions — write-side (rollback, delete)

- **Action**: `handleRollbackSecret` (in a tx: fetch `secret_versions` row by version → re-insert ciphertext into `secrets` as version+1 → append `secret_versions` row → audit `secret.rollback`). `handleDeleteSecret` (tx: remove `secrets` row → append audit `secret.delete`; `secret_versions` retained per schema FK). Refuse rollback to a deleted secret (404 — `secrets` row gone, even if versions exist) unless `?undelete=true` is passed (re-inserts and audits as `secret.undelete`).
- **Mirror**: tx + audit pattern from `handlePutSecret`.
- **Validate**: e2e — push v1, push v2, rollback to v1 → GET returns v1 value at version 3, audit log shows `secret.rollback`. Delete a key → GET returns 404, versions endpoint still lists the history.

### Task 3: Browser session + CSRF middleware

- **Action**: `dashboard_sessions` table; `SessionRepo`; `POST /api/v1/dashboard/session` (verify bearer in body → create session row with `SHA-256(session)` + CSRF token → set `Set-Cookie: comax_session=...; HttpOnly; Secure; SameSite=Strict; Path=/` → return `{csrf}` in body). `DELETE` revokes. `authMiddleware` extended: if no `Authorization`, look for `comax_session` cookie → verify → require `X-CSRF-Token` to match for mutating methods. Constant-time compares.
- **Mirror**: `auth.ParseBearer` + `auth.Verify` shape; same context stamping with the underlying service token.
- **Validate**: integration test — bearer-only requests unchanged; cookie request without CSRF on PUT → 403; cookie + correct CSRF → mutation succeeds and audit attributes to the underlying token.

### Task 4: SPA embed pipeline + dev-mode build tag

- **Action**: `internal/server/dashboard/embed.go` + `embed_dev.go` (build tag `embed_dashboard`). `handleSPA` serves `index.html` for unknown paths under `/`, but only when the path does not start with `/api/` or `/healthz`. `Makefile` targets `make dashboard` and `make build` (depends on dashboard). CI: build dashboard before Go.
- **Validate**: `go test ./...` works without building the dashboard (build-tag default). `make build` produces a single binary with the dashboard inside; `curl http://localhost:8080/` returns the React shell.

### Task 5: Vite + React scaffold + design tokens

- **Action**: `pnpm create vite`, configure Tailwind + Radix Theme provider, set tokens in `tokens.css` per ECC `web/coding-style.md`. Type-safe router skeleton with two routes: `/login`, `/`. Wire `lib/api.ts` (fetch wrapper unwrapping the envelope). Wire TanStack Query client.
- **Validate**: `pnpm build` outputs to the embed dir. `make dev` starts Vite + server with proxy; `/login` renders.

### Task 6: Login flow + session lifecycle

- **Action**: Login page: paste bearer → `POST /dashboard/session` → store CSRF in memory + sessionStorage → redirect to `/`. App shell adds CSRF header to every mutating fetch via the `lib/api.ts` interceptor. Logout calls `DELETE`. Auto-logout on 401.
- **Mirror**: ECC `web/security.md` cookie + CSRF posture.
- **Validate**: Playwright e2e — log in, refresh page, still authed; logout, refresh, redirected to `/login`.

### Task 7: Projects + Envs screens

- **Action**: `/` lists projects (TanStack Query); create-project dialog; project page lists envs and "Add environment" with inheritance picker. URL is the state.
- **Validate**: vitest unit tests per component; Playwright covers create-and-navigate flow.

### Task 8: Secrets table + inline edit + delete

- **Action**: Per-env page: searchable + sortable table; values masked by default (eye toggle per row); inline edit creates a new version via PUT; delete with confirm dialog; bulk copy-as-dotenv to clipboard.
- **Validate**: e2e — set, see new value, delete, see audit row.

### Task 9: Version timeline + diff viewer + rollback

- **Action**: Side panel on each secret row → `GET .../versions` (filtered to this key client-side until/unless a per-key list endpoint is added in M3) + `GET .../versions/{v}` on demand. Side-by-side diff (current vs selected). Rollback button → confirm → `POST .../rollback`. Toast + invalidate query.
- **Validate**: e2e — rollback flow end-to-end; assertion that diff highlights changed lines.

### Task 10: Env-vs-env diff screen

- **Action**: `/projects/{p}/envs/{e}/diff?against=<e2>` consumes the new `/diff` endpoint. Three columns: added in `e`, removed vs `e2`, changed. Click-through to the secrets table with the row scrolled into view.
- **Validate**: e2e — push a key in `local`, run diff against `prod`, see it in "added".

### Task 11: Audit feed

- **Action**: `/audit` route. Filters in URL. Cursor pagination. Empty state. Reset filters.
- **Validate**: e2e — perform two mutations, see both rows newest-first.

### Task 12: A11y + visual polish + anti-template pass

- **Action**: axe-playwright on every route in CI. Keyboard nav on all interactive elements. Reduced-motion respected. Apply ECC `web/design-quality.md` checklist — bento home, type scale, deliberate hover/focus, depth via surfaces (not just borders), color used semantically (diff add/remove, env coloring). Dark+light both intentional.
- **Validate**: axe checks pass; visual regression screenshots at 320/768/1024/1440; Lighthouse a11y ≥ 95 on the seeded dashboard.

### Task 13: Embedded-binary smoke + size budget

- **Action**: After `make build`, the `secret-server` binary size budget: ≤ 25 MB stripped (dashboard `dist/` after gzip ≤ 400 KB JS + 30 KB CSS — per ECC `web/performance.md` app-page budget × app-not-landing). CI gate.
- **Validate**: CI step: `du -b bin/secret-server` < 25 MB. Bundle size report posted to PR.

### Task 14: Cross-compile + docker compose smoke (regression of M1)

- **Action**: Re-run M1's cross-compile matrix to confirm `linux/{amd64,arm64,arm/v7}` still build with the dashboard embedded. Re-run `docker compose up` smoke (≤ 120 s healthy). Document any size delta in `docs/perf.md`.
- **Validate**: same Makefile targets as M1.

### Task 15: Operator dogfood — replace CLI-only workflows

- **Action**: Operator uses *only the dashboard* (no CLI for one day) for the operations that drove M1: add a new env var, roll back a wrong commit, find the key that only exists in local. Record click count and time per task in `docs/dogfood.md`.
- **Validate**: each of the three flows ≤ 30 s. Audit log on the dashboard shows the work attribute to the operator's token.

## Validation

```bash
# Server (extends M1)
make build                       # builds dashboard, then secret-server with dist embedded
make lint                        # golangci-lint
go test ./... -race -coverprofile=coverage.out
go tool cover -func=coverage.out | awk '/total:/ { if ($3+0 < 80) exit 1 }'

# Frontend
pnpm --filter @comax/dashboard install
pnpm --filter @comax/dashboard lint
pnpm --filter @comax/dashboard typecheck
pnpm --filter @comax/dashboard test            # vitest unit
pnpm --filter @comax/dashboard build           # vite build
pnpm --filter @comax/dashboard test:e2e        # playwright + axe-playwright

# Cross-compile regression (M1 contract)
GOOS=linux GOARCH=arm64   go build -tags embed_dashboard -o /tmp/secret-arm64   ./cmd/server
GOOS=linux GOARCH=arm GOARM=7 go build -tags embed_dashboard -o /tmp/secret-armv7 ./cmd/server

# Compose smoke
docker compose -f deploy/compose/docker-compose.yml up -d
curl -fsSL http://localhost:8080/healthz
curl -fsSL http://localhost:8080/ | head -c 200   # SPA shell renders
docker compose -f deploy/compose/docker-compose.yml down

# Bundle / binary budget gates
du -b bin/secret-server          # < 25 MB
du -b internal/server/dashboard/dist/assets/*.js | awk '{ s+=$1 } END { exit (s > 400*1024) }'
```

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| **Scope creep into M3 (GitHub Action) or M5 (SDK)** because the dashboard exposes "obvious next features" (issue tokens UI, webhook UI, SDK examples) | High | High | Token-issue UI **is in M2** because the dashboard cannot bootstrap itself without it; webhook UI is **out** (M4). Decision gate: any new screen must trace to a PRD M2 outcome. |
| **CSRF + cookie auth bugs** silently let one operator's session be hijacked by an XSS payload | Medium | High | CSP nonce mandatory on the SPA response. `HttpOnly Secure SameSite=Strict` cookie. CSRF double-submit. Constant-time compare. Test in Task 3 includes a "CSRF missing → 403" assertion. |
| **Bundle size pushes binary past the operator-friendly threshold** for NAS targets | Medium | Medium | Budget gate in Task 13 (≤ 25 MB binary, ≤ 400 KB JS gz). Vite code-splitting; lazy-load Audit + Diff routes; no Lottie/heavy charts in v1. |
| **Embed pipeline breaks `go test` for contributors without Node** | Medium | Medium | Build tag `embed_dashboard` (default off in tests). Server runs `/api` only when embed not present, logs a clear message. |
| **Doppler-style ends up "stock template"** (PRD anti-pattern via ECC `web/design-quality.md`) | Medium | Medium | Anti-template pass is its own task (Task 12). Bento home, intentional motion, real hover states, dark+light both. Reviewed by ECC `a11y-architect` + design judgment at PR. |
| **Rollback semantics surprise the operator** (does it overwrite or append?) | Low | High | Rollback always appends a new version with the old ciphertext. Surface the version chain in the UI so the operator sees v1 → v2 → v3(=v1). Tested in Task 2. |
| **Audit endpoint becomes an exfiltration channel** if a low-privilege token sees too much | Low | Medium | v1 has only one privilege level. Documented limitation in `docs/threat-model.md`. RBAC explicitly deferred (not in PRD). |
| **Session table grows unbounded** | Low | Low | `SessionRepo.Prune(ctx, olderThan)` called on a 1-h goroutine. Default session TTL 30 days. |
| **Dev-mode CSP nonce mismatch** between Vite HMR + server middleware blocks the dev loop | Low | Medium | CSP applied only on embedded SPA path. In `make dev` the SPA is served by Vite (no CSP middleware on that origin), and `/api` does not return HTML. |

## Acceptance

- [ ] All 15 tasks complete; each task's validation step passes.
- [ ] `go test ./... -race -cover` ≥ 80% per package; ≥ 85% overall (regression of M1 gate).
- [ ] Dashboard CI job: lint + typecheck + vitest + vite build + playwright + axe — all green.
- [ ] `secret-server` binary ≤ 25 MB; dashboard JS ≤ 400 KB gz; CSS ≤ 50 KB.
- [ ] Cross-compile `linux/{amd64,arm64,arm/v7}` with `-tags embed_dashboard` succeeds in CI.
- [ ] `docker compose up` ≤ 120 s healthy with dashboard reachable at `/`.
- [ ] Browser session security: HttpOnly+Secure+SameSite=Strict cookie; CSRF enforced on mutations; CSP nonce on SPA HTML.
- [ ] **Operator dogfood (Task 15): three M1-painful flows completed via dashboard in ≤ 30 s each, logged in `docs/dogfood.md`.**
- [ ] `docs/dashboard.md`, `docs/threat-model.md`, `docs/quickstart.md`, `README.md` updated.
- [ ] PRD Milestone 2 row updated to `done` with this plan linked.

---

*Generated by `/ecc:plan` on 2026-05-31. Source PRD: [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md). Builds on Milestone 1 plan: [.claude/plans/comax-secrets.plan.md](./comax-secrets.plan.md).*
