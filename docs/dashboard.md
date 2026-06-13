# Operator dashboard

The dashboard is a browser UI for the same operations the `secret` CLI
exposes — projects, environments, secret keys, versions, audit log —
plus three things the CLI cannot do well: version-by-version diff,
single-secret rollback, and env-vs-env diff (which keys exist in `local`
but not in `prod`).

It ships as a static React+Vite bundle embedded directly into the
`secret-server` binary via `//go:embed all:dist`. There is no separate
service to deploy: the same `docker compose up` that boots the API
also serves the dashboard at `/`.

## Opening it

After `docker compose -f deploy/compose/docker-compose.yml up -d`,
visit:

```
http://localhost:8080/
```

(Or the host:port your reverse proxy maps onto the server — see the
TLS note in [threat-model.md](threat-model.md).)

You will land on `/login`. Paste the same bearer token you would pass
to `secret login`. The bootstrap admin token is fine; so is any
service token issued from the dashboard's Tokens page.

## How auth works in the browser

1. `POST /api/v1/dashboard/session` verifies the bearer in the request
   body.
2. The server returns a `Set-Cookie: comax_session=...; HttpOnly;
   Secure; SameSite=Strict; Path=/` plus a CSRF token in the JSON body.
3. The SPA stores the CSRF token in memory + `sessionStorage` and
   attaches it as `X-CSRF-Token` on every mutating request.
4. `DELETE /api/v1/dashboard/session` revokes the cookie + the server-
   side session row. The logout button does this; closing the tab does
   not (cookie TTL is 30 days by default).

Bearer-auth requests (the CLI, CI runners, anything that sends
`Authorization: Bearer …`) are exempt from CSRF — the cookie carrier
is what raises the bar, and the CLI does not carry cookies.

## Disabling the dashboard

If the dashboard is not desired (for example, an API-only deployment
where only the CLI and SDK should reach the server), boot with:

```
COMAX_DASHBOARD_ENABLED=0
```

or pass `--dashboard-enabled=false` on the server command line. The
SPA route returns the standard 404 envelope; `/api/*` and `/healthz`
keep working.

The binary is the same in either case; this flag only gates whether
the embedded SPA is served. The size budget (≤ 25 MB) and the embed
build tag (`embed_dashboard`) are independent.

## Threat-model implications

A short version of [threat-model.md § Browser sessions](threat-model.md#browser-sessions-m2):

- **Cookie hardening**: `HttpOnly`, `Secure`, `SameSite=Strict`, scoped
  to `/`. No JS can read the cookie; no cross-site request can carry
  it.
- **CSRF**: double-submit token (cookie + `X-CSRF-Token` header).
  Bearer requests skip the CSRF check; cookie requests do not.
- **CSP**: every SPA response carries a per-request nonce; inline
  scripts without the matching nonce are blocked by the browser.
- **Session lifetime**: 30 days by default. Revoke via the dashboard's
  Sessions page or by `DELETE /api/v1/dashboard/session`. Sessions are
  pruned hourly server-side; the schema's `revoked_at` is the
  authoritative state.
- **No multi-tenant model**: a single privilege level; any logged-in
  operator can do anything the underlying token can do.

## Managing sessions

`/settings/sessions` lists every live dashboard session your service
token has issued — yours included. Use it to revoke a session from a
device you no longer trust without rotating the underlying bearer.

Columns: device (parsed UA), IP prefix (the `/24` truncated remote
address the cookie was minted from), and Created. The row carrying
the cookie that authenticated this request is flagged "현재 세션" and
its 회수 button is disabled — to log out, use the sidebar's 로그아웃.

Revoke is **only a recall mechanism**. If the cookie was already
exfiltrated, every read the attacker performed before revocation is
unrecoverable. When in doubt, revoke the service token itself (it
invalidates every session that token issued, all at once). See the
"Honest limits" subsection of [threat-model.md](threat-model.md) for
the full boundary.

## Build artefacts and budgets

| Asset | Budget | Source |
|---|---|---|
| `bin/secret-server` (linux/amd64, `-s -w`, embed) | ≤ 25 MB | [perf.md](perf.md) |
| Dashboard JS (gzip) | ≤ 400 KB | [perf.md](perf.md) |
| Dashboard CSS (gzip) | ≤ 100 KB | [perf.md](perf.md) |

CI gates each one. A regression fails the build before merge.

## Dogfood

The three CLI-painful flows the dashboard is supposed to fix (add a
new env var across envs, roll one secret back, find a `local`-only
key) are measured in [dogfood.md § Milestone 2](dogfood.md#milestone-2--dashboard-dogfood)
with click + time budgets.
