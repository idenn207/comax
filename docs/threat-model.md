# Threat model

This document is the explicit statement of what Comax Secrets protects
against, what it does not, and the operator obligations that follow.
The self-host model assumes the operator owns the host, the database,
and the master key — Comax is **not** a managed service, and trust
boundaries are drawn accordingly.

## Who this is for

A self-host operator running the server on a NAS, a small VPS, or a
homelab VM. The CLI is used from one or more developer workstations
and possibly CI runners. There is no multi-tenant story in Milestone 1.

## What the system protects

| Threat | Mitigation |
|---|---|
| Casual disk-image leak of the database (e.g. backup falls off a USB stick) | All secret values are encrypted at rest with AES-256-GCM. Without the master key, the DB is opaque. |
| Casual disk-image leak of the **key file** | The file is mode `0600`, owned by the server user. The server **refuses to boot** on Unix if the mode is wider than `0600`. |
| Token theft from a developer machine | Tokens are random opaque strings (32 bytes, base64url). Only the SHA-256 hash is stored server-side. A stolen plaintext token is usable until revoked; this is the price of an operator-friendly bootstrap. |
| Accidental token logging | The server never logs the plaintext token after issuance. The `secret` CLI never echoes the token. The bootstrap plaintext is emitted to **stdout exactly once** on first boot. |
| Replay of stale ciphertext | Each secret stores a monotonic version + an append-only `secret_versions` history. `audit_log` records every state change. |
| Cross-env contamination via inline references (`${{ shared.KEY }}`) | Resolved at pull time, not at write time. Cycle detection rejects `shared → app → shared` chains with a parse error. |
| Decrypted secrets touching disk during `secret run` | The CLI pulls in memory and forks the child with `exec.Command`'s `Env`. The integration test asserts no new file is written during a run. |

## What the system does NOT protect

| Threat | Why not (yet) |
|---|---|
| Compromise of the master key | The key is the entire trust anchor. If the operator exfiltrates `keys/master.key`, all secrets are decryptable. M1 is file-on-disk; a pluggable `KeyProvider` interface keeps OS keyring / cloud KMS reachable in later milestones. |
| Compromise of the server host | An attacker with root on the server host can read the DB + the key file. M1 is intentionally NOT a HSM-fronted design. |
| Compromise of a developer workstation | A stolen `~/.config/comax/credentials.json` (mode `0600`) lets the attacker pull all secrets the token can read. Revoke the token (server-side; out of M1 scope — manual SQL for now) and bootstrap a replacement. |
| Coercion / legal compulsion of the operator | Self-host means the operator is the custodian. There is no third-party escrow. |
| Side-channel attacks on the host (Spectre, Rowhammer) | Out of scope. |
| Timing attacks against the auth comparison | The HMAC compare uses `subtle.ConstantTimeCompare`; the SQL lookup is not constant-time but the surface area is small (one indexed lookup per request). |

## Operator obligations

The system depends on these. Failure to honour them voids the protections
above:

1. **Master key permissions.** The file at `--master-key-file` (default
   `./keys/master.key`) MUST be mode `0600` and owned by the server's
   UID. On Unix the server refuses to boot otherwise. On Windows the
   server logs a warning and continues — protect the file via NTFS ACLs.
2. **Backups.** Back up both `data/secrets.db` AND `keys/master.key`,
   to the same place and with the same frequency. They are useless
   without each other.
3. **Bootstrap token capture.** The plaintext bootstrap token is
   printed to stdout exactly once. Capture it from
   `docker compose logs secret-server` before the log rotates and store
   it in your password manager (not in a `.env` file — that would
   defeat the purpose).
4. **Token rotation.** When a developer leaves, revoke their token.
   M1 has no UI for this — manual SQL is the escape hatch until M2
   ships a dashboard. The schema is stable; rows are safe to delete.
5. **TLS termination.** This binary speaks plain HTTP. Put a reverse
   proxy (Caddy / nginx / Traefik) in front for any deployment that
   leaves localhost. Tokens are bearer credentials — they MUST traverse
   TLS.
6. **No source-control commits of secrets or keys.** `.gitignore`
   already excludes `data/`, `keys/`, `*.key`, `.secretrc`, and `.env`.
   Confirm the rules apply to your repo's overrides.
7. **Master key rotation.** Out of M1 scope — rotation requires
   re-encrypting every row. Plan it before you have so many rows that
   the operation becomes a project.

## Browser sessions (M2)

The dashboard reuses bearer tokens, but wraps them in an HTTP cookie so
the browser security model can do its job. Adopting the M2 dashboard
does not weaken the M1 threat model; it adds three new surfaces and
hardens each one:

1. **Cookie**: `Set-Cookie: comax_session=<random>; HttpOnly; Secure;
   SameSite=Strict; Path=/`. The cookie value is the session token; the
   server stores `SHA-256(token)` exactly as it does bearer tokens. No
   client-side JavaScript can read it (HttpOnly) and no cross-site
   request can carry it (SameSite=Strict). Recovery from a stolen
   cookie requires both the cookie and the matching CSRF secret stored
   server-side per session.
2. **CSRF**: double-submit token. `POST /api/v1/dashboard/session`
   returns a CSRF token in the JSON body; the SPA echoes it back in
   `X-CSRF-Token` on every mutating call. The server compares with
   `subtle.ConstantTimeCompare`. Bearer-auth requests skip the check —
   the CLI does not carry a cookie, so the cookie+CSRF rail does not
   apply to it.
3. **CSP**: every SPA response carries a per-request nonce
   (`Content-Security-Policy: ... 'nonce-<random>' ...`). Inline scripts
   without the matching nonce are blocked. React renders text by
   default, but the CSP gate is what stops a future templating mistake
   from becoming an XSS escape from the cookie's HttpOnly bound.
4. **Session lifetime**: 30 days default. The dashboard's Sessions
   page lets the operator revoke any session (its `revoked_at` flips
   server-side and the cookie no longer authenticates). Expired and
   revoked rows are pruned hourly.
5. **Logout**: `DELETE /api/v1/dashboard/session` revokes the session
   row and clears the cookie. Closing the tab does **not** revoke the
   session — the cookie still works until TTL or explicit revocation.

### Honest limits

We list these so the operator can build their workstation hygiene
around what the dashboard actually protects, not the protection they
might assume.

- **CSRF 토큰은 mutation에만 요구된다.** `GET /api/v1/...` 요청은
  cookie 단독으로 인증된다. cookie 한 장만 빠져나가도 그 시점부터의
  **모든 read (시크릿 값 포함) 가 가능**해진다. CSRF는 cross-site write
  를 막을 뿐 cookie 자체의 보호 수단이 아니다.
- **Revoke는 회수 수단이지 탈취 방지 수단이 아니다.** `/settings/sessions`
  에서 임의 세션을 회수해도, **회수 이전에 이미 read 된 값은 되돌릴 수
  없다**. cookie 자체가 의심된다면 해당 service token 을 통째로
  revoke 해 그 token 으로 발급된 모든 세션을 무력화하는 게 옳다.
- **보안 경계는 cookie 보호다.** HttpOnly + Secure + SameSite=Strict +
  Path=/ 가 cookie 가 JS / cross-site / 평문 채널로 새지 않도록 한다.
  운영자의 workstation 위생 (디바이스 잠금, 신뢰할 수 있는 브라우저,
  공용 단말에서 로그인하지 않기) 이 그 위에 올라가는 마지막 한 줄이다.
- **다른 token이 발급한 세션은 회수할 수 없다.** v1 는 multi-token
  admin 권한을 지원하지 않는다. 다른 service token 이 만든 세션을
  무력화하려면 그 token 자체를 revoke 해야 한다.

What's still out of scope: per-user identity. v1 has a single privilege
level; "logged-in operator" means "anyone with a bearer token". RBAC
is deferred to a later milestone and is called out in the PRD.

## Audit log retention

Every state-changing operation writes a row to `audit_log` with the
acting token's ID and an opaque target string. M1 does not rotate the
audit log; for high-volume deployments, monitor the table size and
truncate / archive on a schedule appropriate to your compliance
requirements.

## Reporting a vulnerability

Open a private security advisory on the repository, or email the
operator. This is a single-maintainer project; CVE disclosure
co-ordination will be best-effort.
