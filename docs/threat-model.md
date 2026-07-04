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

## Service tokens & CI (M3)

M3 adds a GitHub Actions composite action and the token-management surface
it needs. The M1 rule that "a stolen plaintext token is usable until
revoked" is now enforceable in-band, and token issuance is no longer flat:

1. **Admin-only issuance.** `service_tokens.is_admin` gates token
   management. Only an admin token (the bootstrap token, or one promoted
   by the migration backfill) may `POST/GET/DELETE /api/v1/tokens`. Issued
   CI tokens are always non-admin, so a leaked CI credential **cannot mint
   or revoke further tokens** — it escalates no privilege it did not
   already hold.
2. **Soft revoke on both arms.** `service_tokens.revoked_at` soft-revokes
   a credential. The bearer arm (`ByHash`) filters revoked rows, and the
   dashboard session arm (`ByID` + a `RevokedAt` check in `authSession`)
   401s any live session bound to a revoked token. Revoking a token thus
   kills both its CLI/CI use and any open browser tab (R2-1).
3. **No credential residue on the runner.** The action writes its
   credential to a one-shot `$RUNNER_TEMP/comax-creds.json`, never the
   default `~/.config/comax` path, and an `if: always()` cleanup step
   deletes it even on failure (R2-2). The token is passed via env, never a
   command line, so it is absent from process listings.
4. **Injection models.** The default `secret run` path injects secrets
   only into the child process's environment (process-env). The opt-in
   `export-to: github-env` path widens exposure to the whole job and is
   documented as such. Both mask values via `::add-mask::`, but masking is
   best-effort (R2-3): the `action-smoke` workflow proves the default path
   never prints the plaintext to a job log.

### Honest limits

- **CI 토큰에는 project/env read scope가 없다 (M4 이연).** non-admin CI
  토큰도 현재는 그 서버의 **모든 project/env 시크릿을 read**할 수 있다.
  scope 컬럼·미들웨어 인가는 위협모델 재정의가 필요해 M4로 명시 이연됐다
  (사용자 확정). M3에서 blast radius를 닫는 수단은 **발급 admin-only 제한 +
  soft revoke**다.
- **마스킹은 best-effort다.** `::add-mask::`는 로그에서 값을 가릴 뿐이며,
  짧거나 저엔트로피인 값은 부분적으로 새어나올 수 있다. process-env 기본
  모드는 값을 job 로그 경로에 올리지 않으므로 이 한계를 회피한다.
- **github-env opt-in은 job 전체로 노출을 넓힌다.** opt-in 이후의 모든
  스텝(서드파티 action 포함)이 시크릿을 env로 본다. 신뢰할 수 없는 후속
  스텝이 있다면 process-env를 쓴다.
- **revoke는 소급 방지가 아니다.** 회수 이전에 이미 read된 값은 되돌릴 수
  없다. 유출 의심 시 회수와 별개로 값 자체를 로테이션해야 한다.

## Webhooks (M4)

웹훅은 시크릿 변경 시 등록된 URL로 서명된 이벤트를 POST한다. 자세한 사용법은
[webhooks.md](webhooks.md).

- **등록·삭제는 admin 전용이다.** 비-admin 토큰은 403. 유출된 CI 토큰이 웹훅을
  심어 트래픽을 빼돌릴 수 없다.
- **페이로드에 시크릿 평문이 없다.** 이벤트는 메타데이터(project/env/key/
  version/action)만 담는다. `Payload` 구조체에 값 필드 자체가 없어 구조로
  강제되며, 통합 테스트가 수신 페이로드에 canary 부재를 검증한다.
- **서명 시크릿은 마스터키로 암호화 저장**한다(토큰처럼 hash 가 아님 — 배달
  시점 HMAC 서명에 평문이 필요하므로 `crypto.Seal`). 목록 API/CLI/대시보드는
  ciphertext 를 제외하고, 발급 plaintext 는 등록 시 1회만 노출된다.
- **SSRF: 사설 IP 는 의도적으로 허용**한다(웹훅의 목적이 내부 서비스 호출,
  Docker overlay 는 RFC1918). 대신 **link-local/클라우드 metadata
  (169.254.0.0/16, `fe80::/10`, 특히 `169.254.169.254`)를 기본 차단**한다.
  방어 계층: http/https 스킴 제한, 등록·배달 시 host resolve 후 metadata 거부,
  리다이렉트 미추종, 배달 시 `DialContext` IP 재검증(DNS rebinding), 운영자
  opt-in allow/deny CIDR(`COMAX_WEBHOOK_ALLOW`/`_DENY`).
- **at-least-once 배달.** 배달은 중복될 수 있으므로 수신자는 멱등해야 한다
  (`X-Comax-Delivery` id 를 멱등 키로). 배달 워커는 원자적 lease claim 으로
  동시 워커의 중복 POST 를 막고, 크래시한 워커의 in_progress 행은 lease 만료
  후 회수한다. outbox 행은 시크릿 변경 tx 커밋 후에만 배달 대상이 된다.
- **서명 시크릿 회전은 delete+recreate** 워크어라운드다(v1). 위조 트리거의
  blast radius 는 운영자 자신의 수신자에 한정되고, 수신자는 인증 CLI 로 값을
  재-pull 하므로 유출 파급이 제한적이다. in-place 회전은 backlog 이연.

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
