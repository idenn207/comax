# Plan: Webhooks (M4)

**Source PRD**: `.claude/prds/comax-secrets.prd.md`
**Selected Milestone**: #4 "Webhooks + Secret referencing/overrides" — "Secret 변경 → Docker 서비스 재시작 webhook. inline 참조와 env override 모델."
**Complexity**: Medium-Large

## Summary

M4의 절반(**Secret referencing/overrides**)은 이미 M1/M2에서 배송 완료다: `internal/secret/resolver.go`가 `inherits_from` 상속 오버레이 + `${{ env.KEY }}` 참조 확장 + 사이클 감지를 갖추고 있고, `handlers_secrets.go`/`handlers_envs.go`가 이를 pull/diff 경로에 배선했으며 `resolver_test.go`가 전수 커버한다. 따라서 이 계획은 **웹훅 서브시스템 단독**을 다룬다(재계획하면 중복).

웹훅은 시크릿 변경(`secret.upsert`/`secret.rollback`/`secret.delete`) 시 등록된 URL로 서명된 이벤트를 POST해서, 운영자가 Docker Swarm 서비스 재시작 같은 후속 동작을 트리거하도록 한다(PRD Open Q #2, `docker secret` 미사용 결정 → webhook 우선). 핵심 설계 결정:

1. **Transactional outbox**: 시크릿 변경 tx 안에서(`appendAudit`와 나란히) `webhook_deliveries` outbox 행을 enqueue한다. 커밋된 변경만 배달 대상이 되므로 롤백된 tx가 유령 배달을 만들지 않는다.
2. **배달 워커**: `runPruneSweeper` 패턴을 미러링한 `ctx`-취소 가능 goroutine이 due 배달을 폴링→서명→POST→재시도(지수 백오프)→dead-letter 처리. `cmd/server/main.go` 라이프사이클에 배선.
3. **서명 시크릿은 마스터키로 암호화 저장**(토큰 hash와 다름): 배달 시점 HMAC-SHA256 서명에 평문 키가 필요하므로 `crypto.Seal`로 봉인하고 생성 시 1회만 노출. `X-Comax-Signature: sha256=<hmac>` + `X-Comax-Timestamp`(replay 방지).
4. **페이로드에 시크릿 평문 없음**: 이벤트 메타데이터만(project/env/key/action/version/delivery_id/timestamp). 수신자는 트리거 수신 후 인증된 CLI로 재-pull한다. "시크릿은 로그/전송에 평문 없음" 규칙을 테스트로 강제.
5. **SSRF (Codex R1 F2 흡수)**: 웹훅의 목적이 내부 서비스 호출이므로 RFC1918 사설/loopback 전면 차단은 유스케이스를 깨뜨린다. 그러나 **link-local/cloud-metadata(169.254.0.0/16, IPv6 fe80::/10, 특히 169.254.169.254)는 정당한 웹훅 대상이 아니므로 기본 차단**한다(Docker overlay는 RFC1918을 쓰므로 무영향). 계층: (a) http/https scheme만, (b) 등록·배달 시 host DNS resolve 후 link-local/metadata 기본 거부, (c) 리다이렉트 미추종(`CheckRedirect`로 차단), (d) 운영자용 opt-in allowlist/denylist(`COMAX_WEBHOOK_ALLOW`/`_DENY` CIDR·host), (e) 등록 admin-only + soft-disable. 전면 사설 차단이 아닌 **metadata 경계 차단 + 명시적 정책**이 핵심.

또한 **대시보드 Webhooks 관리 화면**(등록/목록/삭제 + 최근 배달 상태)을 편입한다. 프로토타입 `Comax Prototype.dc.html`의 Integrations 섹션(GitHub Actions 옆)에 nav 추가, 새 디자인 없이 기존 `globals.css`·`Tokens.tsx`/`Actions.tsx` 패턴 재사용.

> **PRD 정합성 노트**: M3(GitHub Actions)은 PR #12로 병합 완료(`comax-secrets-m3-github-actions.report.md` 존재)이나 PRD 테이블은 아직 `in-progress`로 표기 — 드리프트. 본 계획은 mandate대로 M4 행만 갱신하고, M3 행 보정은 Open Question으로 남긴다.

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| Schema (신규 테이블) | `internal/store/schema.sql:1` | `CREATE TABLE IF NOT EXISTS` idempotent; INTEGER unix 초; BLOB ciphertext; FK `ON DELETE CASCADE`. 웹훅 두 테이블도 동일 |
| Migration (신규 테이블) | `internal/store/migrate.go:53` | 신규 테이블은 `schemaSQL`만으로 충족(ALTER 불필요). `migrate_test.go`가 신규 테이블 존재를 검증 |
| 암호화 저장 | `internal/server/handlers_secrets.go:170` (`crypto.Seal`) | 서명 시크릿을 마스터키로 Seal → `webhooks.secret_ciphertext`. 배달 시 `crypto.Open`(`aesgcm.go:71`) |
| Repo (hash 아닌 암호화 + soft-disable + List 민감필드 제외) | `internal/store/token_repo.go:79,186,232` | `Create`/`List`(ciphertext 제외)/`ByID`/`Delete`; soft-disable는 `Revoke`의 `revoked_at IS NULL` 가드 미러 |
| Repo (append-only 큐) | `internal/store/audit_repo.go:18` + `version_repo.go` | outbox `Enqueue`(tx 내), 커서 없는 due-poll `SELECT ... WHERE status='pending' AND next_attempt_at<=?` |
| HTTP handler (create, admin-only) | `internal/server/handlers_tokens.go` (`requireAdmin`+`handleCreateToken`) | decode→validate→BeginTx→repo→`appendAudit`→Commit→`writeOK(201)`; 목록은 민감필드 제외 view |
| tx 내 부수효과 헬퍼 | `internal/server/handlers_projects.go:101` (`appendAudit`) | 신규 `enqueueWebhooks(r, tx, evt)` 헬퍼가 매칭 웹훅에 outbox 행 삽입 |
| 이벤트 소스 | `handlers_secrets.go:202,327,381` | upsert/rollback/delete의 `appendAudit` 직후(같은 tx) `enqueueWebhooks` 호출 |
| 배경 워커 (ctx-취소, 테스트 추출) | `cmd/server/main.go:322` (`runPruneSweeper`) | `select { <-ctx.Done() / <-ticker.C }`, transient 에러는 로그만·다음 tick 재시도; `run()`에서 goroutine 기동 |
| 서버 의존성 시임 | `internal/server/server.go:63` (`NewServer`) | 워커는 서버 struct가 아닌 `cmd/server`에서 repo+keys로 조립(핸들러는 enqueue만) |
| Router | `internal/server/router.go:33` | `POST`/`GET /api/v1/webhooks`, `DELETE /api/v1/webhooks/{id}`, `GET /api/v1/webhooks/{id}/deliveries` |
| Client 메서드 | `pkg/client/client.go:307` (`ListTokens`/`CreateToken`/`RevokeToken`) | 추가형 typed 메서드, envelope `do()` 경유 |
| CLI 서브커맨드 | `cmd/cli/cmd_token.go` (create 1회노출/list/delete) | `secret webhook create/list/delete`; `main.go`에 `newWebhookCmd` 등록 |
| 시크릿 미로그 | `internal/server/middleware.go:52` | 워커·핸들러가 서명 시크릿/URL 자격증명 미로그; 페이로드 평문 부재 테스트 강제 |
| 대시보드 CRUD + 1회노출 | `web/dashboard/src/pages/Tokens.tsx` + `TokenRow.tsx` + `ConfirmDialog.tsx` | Webhooks 목록/등록(서명시크릿 1회 dialog)/삭제 |
| 대시보드 nav/route | `AppShell.tsx:135` (연동 섹션) + `router.tsx:137` (`integrationsActionsRoute`) | `active==='webhooks'`, `/integrations/webhooks` 추가 |
| 대시보드 API | `web/dashboard/src/lib/api.ts` (token fetch류) | `listWebhooks`/`createWebhook`/`deleteWebhook`/`listDeliveries` |

## Files to Change

| File | Action | Why |
|---|---|---|
| `internal/store/schema.sql` | UPDATE | `webhooks`, `webhook_deliveries` 두 테이블 추가(`CREATE TABLE IF NOT EXISTS`) |
| `internal/store/migrate_test.go` | UPDATE | 신규 테이블이 fresh/upgrade DB 모두에 존재함을 검증 |
| `internal/store/store.go` | UPDATE | `Webhook`, `WebhookDelivery` 타입 + 델리버리 상태 상수 |
| `internal/store/webhook_repo.go` | CREATE | `Create`(secret_ciphertext)/`List`(ciphertext 제외)/`ByID`(ciphertext 포함, 워커용)/`Delete`/`SetEnabled`/`MatchForEvent`(enqueue용) |
| `internal/store/webhook_repo_test.go` | CREATE | Create·List(민감필드 부재)·Delete·MatchForEvent(project/env/event 필터)·soft-disable |
| `internal/store/delivery_repo.go` | CREATE | `Enqueue`(tx 내)/`ClaimDue`(status=pending & next_attempt_at<=now)/`MarkDelivered`/`MarkRetry`(백오프)/`MarkDead`/`ListByWebhook` |
| `internal/store/delivery_repo_test.go` | CREATE | Enqueue·due 필터·백오프 next_attempt·dead 전이·목록 |
| `internal/webhook/payload.go` | CREATE | `Event` 타입 + `Payload{DeliveryID,Action,Project,Env,Key,Version,Timestamp}` (평문 값 필드 없음) + canonical JSON marshal |
| `internal/webhook/signer.go` | CREATE | `Sign(secret, body, ts) string` = `sha256=` + HMAC-SHA256(hex); 상수시간 아님(발신측) |
| `internal/webhook/signer_test.go` | CREATE | 서명 결정성·헤더 포맷·타임스탬프 포함 |
| `internal/webhook/ssrf.go` | CREATE | **(R1 F2)** `ValidateURL(raw, policy)`: http/https만, host resolve 후 link-local/metadata(169.254.0.0/16, fe80::/10) 기본 거부, opt-in allow/deny CIDR; `SafeClient(policy)`가 `CheckRedirect`로 리다이렉트 차단 + `DialContext`에서 resolve된 IP 재검증(DNS rebinding 방어) |
| `internal/webhook/ssrf_test.go` | CREATE | metadata/link-local 거부·loopback·RFC1918 허용·리다이렉트 차단·deny CIDR |
| `internal/webhook/worker.go` | CREATE | `Worker`: `ClaimDue`→`crypto.Open`(서명키)→서명 POST(bounded timeout)→2xx면 MarkDelivered, 아니면 MarkRetry/MarkDead. `Run(ctx, interval)`는 `runPruneSweeper` 미러 |
| `internal/webhook/worker_test.go` | CREATE | httptest 수신자: 2xx→delivered, 5xx→retry(백오프), max→dead, **페이로드 평문 시크릿 부재**, HMAC 검증, ctx 취소 종료 |
| `internal/server/handlers_webhooks.go` | CREATE | `requireAdmin` + `handleCreateWebhook`(1회노출)/`handleListWebhooks`/`handleDeleteWebhook`/`handleListDeliveries` + `enqueueWebhooks` 헬퍼 |
| `internal/server/handlers_webhooks_test.go` | CREATE | 발급 1회노출·비admin 403·목록 민감필드 부재·삭제 404·감사 actor·enqueue가 매칭 웹훅에만 |
| `internal/server/handlers_secrets.go` | UPDATE | upsert/rollback/delete tx에 `enqueueWebhooks` 호출(`appendAudit` 직후, 같은 tx) |
| `internal/server/handlers_secrets_test.go` | UPDATE | 변경 시 매칭 웹훅에 outbox 행 생성, 롤백된 tx는 미생성 |
| `internal/server/router.go` | UPDATE | 웹훅 4개 라우트 추가 |
| `pkg/client/client.go` | UPDATE | `CreateWebhook`/`ListWebhooks`/`DeleteWebhook`/`ListDeliveries` + 타입 |
| `pkg/client/client_test.go` | UPDATE | 신규 메서드 라운드트립 |
| `cmd/cli/cmd_webhook.go` | CREATE | `secret webhook create --project [--env] --url [--events]`(서명시크릿 1회)/`list`/`delete --id` |
| `cmd/cli/cmd_webhook_test.go` | CREATE | create 1회출력·list·delete |
| `cmd/cli/main.go` | UPDATE | `newWebhookCmd(st)` 등록 |
| `cmd/server/main.go` | UPDATE | `webhook.Worker` 조립 + `go worker.Run(ctx, interval)` (runPruneSweeper 옆); 배달 interval 플래그/env |
| `cmd/server/main_test.go` | UPDATE | 워커 기동/graceful-stop (기존 sweeper 테스트 미러) |
| `web/dashboard/src/pages/Webhooks.tsx` (+`.test.tsx`) | CREATE | 목록 + 등록(서명시크릿 1회 dialog) + 삭제(confirm) + 최근 배달 상태. admin 세션만 |
| `web/dashboard/src/components/WebhookRow.tsx` | CREATE | TokenRow 미러 — 웹훅 1행(url/scope/events/enabled/last-delivery/삭제) |
| `web/dashboard/src/lib/api.ts` (+`.test.ts`) | UPDATE | `listWebhooks`/`createWebhook`/`deleteWebhook`/`listDeliveries` |
| `web/dashboard/src/lib/types.ts` | UPDATE | `Webhook`/`WebhookDelivery` 타입 |
| `web/dashboard/src/router.tsx`, `src/components/AppShell.tsx` | UPDATE | 연동 섹션에 Webhooks nav + `/integrations/webhooks` route + `ActiveSection` 확장 |
| `web/dashboard/tests/e2e/*` | UPDATE | Webhooks 화면 axe(WCAG 2.2 AA) 커버 |
| `docs/webhooks.md` | CREATE | 등록/서명 검증 예제/페이로드 스키마/이벤트 종류/재시도·at-least-once/scope 한계 |
| `docs/threat-model.md` | UPDATE | 웹훅 admin-only, SSRF/사설 IP 의도적 허용, 페이로드 평문 부재, at-least-once |
| `README.md` | UPDATE | M4 상태 + docs 링크 |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | M4 행 pending→in-progress + Plan 셀 |

## Tasks

### Task 1: 스키마 + 타입 (`webhooks`, `webhook_deliveries`)
- **Action**: `schema.sql`에 `webhooks`(id, project_id FK CASCADE, env_id FK CASCADE nullable=전 env, url TEXT, secret_ciphertext BLOB, events TEXT=콤마조인 검증셋, enabled INTEGER DEFAULT 1, created_at, updated_at) + `webhook_deliveries`(id, webhook_id FK CASCADE, event TEXT, payload TEXT=JSON 메타데이터, status TEXT[pending|**in_progress**|delivered|failed|dead], attempts INTEGER, next_attempt_at INTEGER, **claimed_at INTEGER**, last_status INTEGER, last_error TEXT, created_at, delivered_at) + 인덱스(due poll용 `idx_deliveries_due (status, next_attempt_at)`). `store.go`에 두 타입 + 상태 상수. **(R1 F1)** `in_progress`+`claimed_at`는 원자적 claim/stale-claim 회수를 위한 lease 컬럼.
- **Mirror**: `schema.sql`의 IF NOT EXISTS·FK·INTEGER 초.
- **Validate**: `go test ./internal/store/ -run TestMigrate -race`

### Task 2: `WebhookRepo`
- **Action**: `Create(ctx, projectID, envID *int64, url, events string, secretCiphertext []byte)`; `List(ctx)`/`ListByProject`(secret_ciphertext 제외); `ByID`(ciphertext **포함** — 워커 서명용); `Delete`(부재 시 ErrNotFound); `SetEnabled`; `MatchForEvent(ctx, projectID, envID int64, event string) ([]Webhook, error)`(enabled=1 AND project 일치 AND (env_id IS NULL OR env_id=?) AND events LIKE event).
- **Mirror**: `token_repo.go` Create/List/ByID/Delete·nullable(`sql.NullInt64`)·`isUniqueViolation`.
- **Validate**: `go test ./internal/store/ -run TestWebhookRepo -race -cover` (≥80%)

### Task 3: `DeliveryRepo` (outbox, 원자적 lease claim — R1 F1)
- **Action**: `Enqueue(ctx, webhookID int64, event, payload string)`(status=pending, attempts=0, next_attempt_at=now). **`ClaimDue(ctx, now, limit)`는 원자적 compare-and-swap**: 후보 id를 `SELECT ... WHERE status='pending' AND next_attempt_at<=? ORDER BY id LIMIT ?`로 뽑되, 각 후보를 `UPDATE ... SET status='in_progress', claimed_at=? WHERE id=? AND status='pending'`로 전이하고 **RowsAffected=1인 행만 반환**(경쟁 워커가 이미 가져간 행은 0→건너뜀). `MarkDelivered`/`MarkRetry`/`MarkDead`는 모두 **`WHERE id=? AND status='in_progress'`** 가드로만 전이(성공/재시도/dead 경합 차단). 추가로 `ReclaimStale(ctx, before)`: `in_progress` 이지만 `claimed_at < before`(워커 크래시)인 행을 `pending`으로 되돌림. `ListByWebhook(ctx,webhookID,limit)`.
- **Mirror**: `token_repo.go` Revoke의 `WHERE ... IS NULL` + RowsAffected 가드(compare-and-swap 관용).
- **Validate**: `go test ./internal/store/ -run TestDeliveryRepo -race -cover` (≥80%) — **동시 claim이 한 행을 한 워커에만 준다**(두 goroutine claim race) + stale reclaim 포함

### Task 4: `internal/webhook` — payload + signer
- **Action**: `payload.go` — `Payload` struct(**값 필드 없음**: DeliveryID, Action, Project, Env, Key, Version, Timestamp) + deterministic JSON. `signer.go` — `Sign(secret []byte, body []byte, tsUnix int64) (sigHeader, tsHeader string)`, `sigHeader="sha256="+hex(HMAC-SHA256(secret, ts+"."+body))`.
- **Mirror**: `internal/cli/ghenv`류 순수 헬퍼 패키지 구조.
- **Validate**: `go test ./internal/webhook/ -run 'TestPayload|TestSign' -race -cover` (≥80%)

### Task 5: `internal/webhook` — 배달 워커
- **Action**: `worker.go` — `Worker{db, keys, client *http.Client(=ssrf.SafeClient, timeout 10s), maxAttempts, backoff func(attempt)}`. `deliverOne(ctx, d)`: `WebhookRepo.ByID`→`crypto.Open`(서명키)→**배달 직전 `ssrf.ValidateURL` 재검증**(등록 후 DNS 변경 방어)→payload+서명 헤더로 SafeClient POST→2xx면 `MarkDelivered`, 아니면 attempt+1: <max면 `MarkRetry(nextAt=now+backoff)`, ==max면 `MarkDead`. `Run(ctx, interval)`: `runPruneSweeper` 미러(`ClaimDue`→각 배달, tick 시작에 `ReclaimStale`, transient 에러 로그만). 서명키/URL 자격증명 미로그.
- **Mirror**: `cmd/server/main.go:322`.
- **Validate**: `go test ./internal/webhook/ -run TestWorker -race -cover` (≥80%)

### Task 6: 웹훅 핸들러 + enqueue 헬퍼 + 라우트 (admin-only)
- **Action**: `handlers_webhooks.go`. `handleCreateWebhook`(admin-only, body `{project, env?, url, events?}`→**`ssrf.ValidateURL`(scheme + metadata/link-local 거부 + 정책, R1 F2)**·events→랜덤 서명시크릿 생성→`crypto.Seal`→`Create`→`appendAudit("webhook.create")`→201 `{id, signing_secret(1회), ...}`; 거부 시 400 `bad_request`); `handleListWebhooks`/`handleDeleteWebhook`(admin-only)/`handleListDeliveries`. `enqueueWebhooks(r, tx, projectID, envID int64, event, payloadMeta)` 헬퍼: `MatchForEvent`→각 웹훅 `DeliveryRepo(tx).Enqueue`. router 4 라우트.
- **Mirror**: `handlers_tokens.go`(`requireAdmin`), `handlers_projects.go:101`(`appendAudit`).
- **Validate**: `go test ./internal/server/ -run 'TestWebhook' -race`

### Task 7: 이벤트 소스 배선 (outbox enqueue)
- **Action**: `handlers_secrets.go`의 `handlePutSecret`(:202), `handleRollbackSecret`(:327), `handleDeleteSecret`(:381)에서 `appendAudit` 직후 **같은 tx**로 `enqueueWebhooks(...)` 호출. payload는 메타데이터만.
- **Mirror**: 기존 `appendAudit` 호출부.
- **Validate**: `go test ./internal/server/ -run 'TestPutSecret|TestRollback|TestDeleteSecret' -race` — 매칭 웹훅에 outbox 생성, 커밋 실패 시 미생성

### Task 8: client 웹훅 메서드
- **Action**: `CreateWebhook`/`ListWebhooks`/`DeleteWebhook`/`ListDeliveries` + `Webhook`/`WebhookCreated`/`Delivery` 타입.
- **Mirror**: `pkg/client/client.go:307`.
- **Validate**: `go build ./... && go test ./pkg/client/ -race`

### Task 9: `secret webhook` 서브커맨드
- **Action**: `cmd_webhook.go`. `create --project [--env] --url [--events]`(서명시크릿 plaintext 1회 + "수신측 검증에 저장" 안내 stderr), `list`(표), `delete --id`. `main.go` 등록.
- **Mirror**: `cmd/cli/cmd_token.go`.
- **Validate**: `go test ./cmd/cli/ -run TestWebhook -race`

### Task 10: 워커 라이프사이클 배선
- **Action**: `cmd/server/main.go`에 `webhook.Worker` 조립 + `go worker.Run(ctx, interval)`(runPruneSweeper 옆). 배달 interval `--webhook-poll-interval`/`COMAX_WEBHOOK_POLL`(기본 10s), maxAttempts 상수. graceful-stop은 ctx 취소로.
- **Mirror**: `runPruneSweeper` 기동/배선.
- **Validate**: `go test ./cmd/server/ -race`

### Task 11: 배달 통합 테스트 (수용 증명)
- **Action**: `internal/webhook` 또는 `internal/server`에 통합 테스트: httptest 수신자 등록→시크릿 upsert→워커 tick→수신자가 **유효 HMAC** + **평문 시크릿 부재** 페이로드 수신 검증; 5xx 수신자로 retry→dead 전이; rollback/delete 이벤트도 커버.
- **Mirror**: `worker_test.go` httptest.
- **Validate**: `go test ./internal/webhook/ ./internal/server/ -race`

### Task 12: 대시보드 — Webhooks 관리 화면
- **Action**: `pages/Webhooks.tsx` + `WebhookRow.tsx`. 목록(url/scope/events/enabled/최근배달상태/삭제), 등록(project·env·url·events → 성공 시 **서명시크릿 1회 노출 dialog** copy), 삭제(`ConfirmDialog`). `lib/api.ts`/`types.ts` 확장. admin 세션만. 프로토타입 Integrations 참조·`globals.css` 재사용.
- **Mirror**: `Tokens.tsx`/`TokenRow.tsx`/`ConfirmDialog.tsx`.
- **Validate**: `cd web/dashboard && npm run typecheck && npm test -- Webhooks`

### Task 13: 대시보드 — nav + 라우팅
- **Action**: `AppShell.tsx` 연동 섹션에 Webhooks nav + `router.tsx` `/integrations/webhooks` route + `ActiveSection`에 `'webhooks'` 추가 + 아이콘. e2e axe 스펙에 화면 추가.
- **Mirror**: `integrationsActionsRoute`, `AppShell.tsx:135`.
- **Validate**: `cd web/dashboard && npm run lint && npm run build && npm run test:e2e`

### Task 14: 문서 + PRD 정합성
- **Action**: `docs/webhooks.md`(등록·서명 검증 예제·페이로드 스키마·이벤트·재시도/at-least-once·scope 한계), `docs/threat-model.md`(웹훅 admin-only·SSRF 의도적 허용·평문 부재·at-least-once), `README.md` M4, PRD M4 셀.
- **Validate**: 링크 grep 0 broken, `make build`.

## Validation

```bash
make build
make test          # -race, 신규 패키지 포함
make lint
go test ./internal/store/ ./internal/webhook/ ./internal/server/ ./cmd/cli/ ./cmd/server/ ./pkg/client/ -race -cover
# 대시보드(Webhooks 화면)
cd web/dashboard && npm run lint && npm run typecheck && npm test && npm run build && npm run test:e2e
# 통합: 등록→시크릿 변경→워커 배달(유효 HMAC·평문 부재)→retry→dead (M4 acceptance)
```

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| **서명 시크릿 저장 오설계** — hash로 저장하면 배달 시 서명 불가 | Medium | `crypto.Seal`로 암호화 저장, `ByID`만 ciphertext 반환·`List`는 제외; worker_test가 실제 HMAC 검증 |
| **페이로드로 시크릿 평문 유출** | Medium | `Payload`에 값 필드 자체를 두지 않음(구조로 강제) + 통합 테스트가 수신 페이로드에 canary 부재 grep |
| **SSRF** — 임의 URL로 metadata/내부 control-plane 접근 (R1 F2) | Medium | 전면 사설 차단 대신 **link-local/metadata(169.254.x, fe80::/10) 기본 거부** + http/https scheme + 리다이렉트 미추종 + `DialContext` IP 재검증(rebinding) + opt-in allow/deny CIDR. RFC1918/loopback은 허용(Docker 유스케이스). 등록 admin-only + soft-disable |
| **중복/경합 배달** — 동시 워커·crash-after-POST (R1 F1) | Medium | `ClaimDue` 원자적 compare-and-swap(`WHERE status='pending'`→`in_progress`) + Mark* `WHERE status='in_progress'` 가드 + `ReclaimStale`. payload `delivery_id` + docs "수신자 멱등 필수"; outbox는 tx 커밋 후에만 due |
| **배달 워커 stall** — 느린/무응답 수신자 | Medium | `http.Client` 10s timeout + attempt별 백오프 + maxAttempts→dead; 워커는 순차지만 tick마다 due batch |
| **outbox 무한 재시도 누적** | Low | maxAttempts 후 dead 전이(재시도 안 함); dead 행은 대시보드에 노출 |
| **트랜잭션 내 enqueue가 커밋 지연** | Low | MatchForEvent는 인덱스 조회, Enqueue는 INSERT N행(웹훅 수 소규모) — 무시 가능 |
| **DELETE CASCADE로 배달 이력 소실** | Low | 의도된 동작(웹훅 삭제 시 이력 정리); dead-letter는 삭제 전 대시보드에서 확인 가능 |

## Acceptance

- [ ] 모든 태스크 완료
- [ ] `make build`/`make test`(-race)/`make lint` green, 신규 패키지(`internal/webhook`, webhook/delivery repo) 커버리지 ≥80%
- [ ] 통합: 등록→시크릿 upsert/rollback/delete→워커가 **유효 HMAC 서명** POST + **페이로드에 평문 시크릿 부재** + 5xx→retry(백오프)→max→dead 를 green으로 증명
- [ ] Transactional outbox: 시크릿 변경 tx 커밋 시에만 배달 enqueue(롤백 tx는 미생성) 테스트
- [ ] **(R1 F1)** 동시 claim 테스트: 두 워커/goroutine이 같은 due 행을 claim해도 정확히 하나만 성공(POST 1회); stale `in_progress` 회수 동작
- [ ] **(R1 F2)** SSRF 테스트: `169.254.169.254`/link-local 등록·배달 모두 거부, 리다이렉트 미추종, RFC1918/loopback 허용
- [ ] admin-only 등록/삭제(비admin 403), 서명 시크릿 발급 1회 노출(재조회 불가), 목록에 secret_ciphertext 부재
- [ ] 배달 워커 graceful-stop(ctx 취소) + transient 에러가 프로세스 미종료
- [ ] 대시보드: Webhooks 화면(등록 1회노출·목록·삭제·배달상태, admin 세션만) 동작, 프로토타입 참조·`globals.css` 재사용, axe WCAG 2.2 AA green
- [ ] 시크릿이 서버/CLI/워커/대시보드 로그·페이로드·DOM 어디에도 평문으로 남지 않음(테스트로 강제; 서명 시크릿 발급 plaintext는 1회 dialog만)

## Open Questions

- **[해소됨, R1 F2] SSRF 정책**: v1은 link-local/metadata 기본 차단 + scheme + 리다이렉트 미추종 + DialContext IP 재검증 + opt-in allow/deny CIDR로 흡수(Task 4 `ssrf.go`, Task 5/6 배선). RFC1918/loopback은 Docker 유스케이스 위해 허용. 완전한 DNS-rebinding TOCTOU 봉쇄와 세밀한 정책 언어는 구현 시 acceptance로 검증.
- **[DEFER → backlog, R1 F3] 서명 시크릿 회전**: `RotateWebhookSecret`(old/new overlap) + 배달 이력 보존은 M4 MVP 범위 밖. v1은 delete+recreate 워크어라운드(웹훅=트리거라 유출 blast radius 제한적: 위조 트리거는 운영자 자신의 수신자에만 도달, 수신자는 인증 CLI로 재-pull). `.claude/plans/codex-findings-backlog.md`에 기록.
- **[LOW] 이벤트 scope 세분화**: v1은 project(필수)+env(옵션, NULL=전 env)+event-type 필터. per-key 필터는 YAGNI로 제외.
- **[LOW] PRD M3 행 드리프트**: M3은 PR #12로 병합 완료이나 PRD 테이블이 `in-progress` 표기. 본 계획 mandate 밖(M4 행만 갱신)이라 별도 정합성 커밋 또는 M4 PR에 housekeeping으로 흡수할지 사용자 결정 필요.

## Design Routing Guide

계획에 대시보드 UI(Webhooks 화면)가 편입되어 design_signal=true. 계획 단계는 렌더된 UI가 없으므로 impeccable을 **호출하지 않고** 아래를 구현 단계 체크리스트로 기록한다. routing mode: **auto**(구현 단계에서 발효). 구현 시 디자인 게이트가 diff 신호에 맞춰 stage별 impeccable 명령을 라우팅한다.

| Stage | Command |
|---|---|
| discovery | `/impeccable shape` |
| refine | `/impeccable layout` · `/impeccable typeset` · `/impeccable animate` · `/impeccable colorize` · `/impeccable bolder` · `/impeccable quieter` · `/impeccable overdrive` · `/impeccable delight` |
| simplify | `/impeccable adapt` · `/impeccable distill` · `/impeccable clarify` |
| evaluate | `/impeccable critique` · `/impeccable audit` |
| harden | `/impeccable harden` · `/impeccable optimize` · `/impeccable onboard` |
| polish | `/impeccable polish` |
| system | `/impeccable document` · `/impeccable extract` |

> 구현 화면은 새 디자인이 아니라 기존 `globals.css`/`Tokens.tsx`·`Actions.tsx` 패턴 재사용이 원칙. `Comax Prototype.dc.html` Integrations 섹션을 시각 레퍼런스로, evaluate 단계 `/impeccable critique`로 정합성 검증.

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2; `--impeccable-available`로 design-scope 제외 → 보안/정합성/성능 집중)
- 라운드 수: 1 (R1)
- 합치 결론: **needs-attention** — 배달 중복 claim·SSRF 완화 부족·서명 시크릿 회전 부재 3건.
- YAGNI Triage / 흡수:
  | Finding | Severity | Verdict | 해소 |
  |---|---|---|---|
  | F1 `ClaimDue` 비원자적 → 동일 delivery 중복 POST | HIGH | ACCEPT_NOW | 원자적 compare-and-swap claim(`in_progress`+`claimed_at` lease) + Mark* `WHERE status='in_progress'` 가드 + `ReclaimStale` (Task 1/3, 동시-claim 테스트를 acceptance에) |
  | F2 SSRF 완화가 admin-only+scheme에만 의존 | HIGH | ACCEPT_NOW | link-local/metadata(169.254.x, fe80::/10) 기본 차단 + 리다이렉트 미추종 + DialContext IP 재검증 + opt-in allow/deny CIDR (`internal/webhook/ssrf.go`, Task 4/5/6). RFC1918/loopback은 허용 |
  | F3 서명 시크릿 in-place 회전 경로 부재 | MEDIUM | DEFER_TO_BACKLOG | v1 delete+recreate 워크어라운드(웹훅=트리거, blast radius 제한). `RotateWebhookSecret`+이력 보존은 backlog |
- Deferred to backlog: 1 (F3) → `.claude/plans/codex-findings-backlog.md`
- Open Questions: 없음 — F1/F2는 계획에 직접 흡수(auto-CRITICAL 잔존 없음), F3는 backlog. 흡수된 수정의 **실코드 검증(동시 워커 1회 POST, metadata 차단, 페이로드 평문 부재)은 `/mccp:prp-implement`의 Implement-Codex 게이트**가 acceptance로 담당.
- 라운드 캡(default 1): F1/F2 흡수로 blocking 측면 해소 → Phase 5.4 escalation 미발동, R2 미실행.
- Codex session 참조: R1 threadId `019f223a-e4f8-71a0-97cb-6a9355706b9c`

## Codex Implementation Review

decision-set already converged in mccp-plan-codex review. No new implement-time decisions detected. Cross-gate dedupe applied.
