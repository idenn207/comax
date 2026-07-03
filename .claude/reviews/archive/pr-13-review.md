# PR Review: #13 — feat: M4 웹훅 (서명 이벤트 배달 + SSRF 하드닝 + 소프트 비활성화)

**Reviewed**: 2026-07-03
**Author**: idenn207 (박동민)
**Branch**: feat/m4-webhooks → master
**Decision**: APPROVE (with comments)
> self-review이므로 GitHub `--approve`는 거부됨 → `--comment`로 게시, 결정은 본 헤더에 보존.

## Summary

M4 웹훅 서브시스템(서명 배달 + transactional outbox + 원자적 lease + SSRF 이중 검증)은 설계·구현·테스트 모두 견고하다. CRITICAL/HIGH 없음. 코어 배달 로직(CAS lease claim, `WHERE status='in_progress'` 가드, ReclaimStale at-least-once, HMAC 타임스탬프 바인딩, SSRF DNS-rebinding TOCTOU 방어)은 정확하다. 지적 사항은 배달 이력 무한 증가(MEDIUM, 비차단)와 소수의 LOW 항목뿐이며, 모두 후속 사이클로 안전하게 이연 가능하다.

## Cross-gate dedupe

- **PR-Codex**: `mccp-pr-codex` 리시트(`comax-secrets-m4-webhooks`) 재사용 — round 1 수렴, findings/accepted/rejected/open_questions 전부 비어 있음. 재도전할 수렴 결정 없음 → 전 카테고리 fresh 리뷰.
- **Security Reviewer**: PR 본문 `### Security Reviewer` 재사용 — PASS(CRITICAL/HIGH 없음). SSRF/서명/authz/입력검증/시크릿 심층 패스 확인.
- **Design(impeccable critique+audit)**: PR 본문 `## Design Review` 재사용 — Nielsen ~36/40, AI-slop 아님. P2/P3 advisory.
- **Accessibility(a11y-architect)**: PR 본문 `## Accessibility Review` 재사용 — AA 미준수 advisory(비차단).
- **게이트 범위 밖 델타**: pr-codex 리시트 head=`8a1b2c7d`, 현재 HEAD=`4af29b2`. 추가 커밋 `4af29b2`(gosec G104: `_ = x.Close()` 5건)를 직접 확인 → 로직 무변경, 무해.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM

**M1. `webhook_deliveries` 무한 증가 — prune/retention 부재**
`internal/store/delivery_repo.go` / `cmd/server/main.go:160` / `internal/store/schema.sql:135`
- `dashboard_sessions`는 `SessionRepo.Prune` + `runPruneSweeper`(hourly)로 만료·폐기 행을 정리하지만, `webhook_deliveries`에는 대응 prune 경로가 전혀 없다(`DeliveryRepo`에 Prune 없음, 서버에 배달 스위퍼 배선 없음 — grep 확인).
- `delivered`/`dead` 종단 행이 영구 누적 → 장기 운영 시 DB 파일·테이블 무한 증가, `ListByWebhook` 및 cascade delete 비용 점증.
- **비차단**: 자가호스팅·저빈도 변경 특성상 slow-burn이며, F3(시크릿 회전)처럼 backlog 이연이 합리적. 다만 backlog(`codex-findings-backlog.md`)에 현재 미추적이라 명시적으로 남길 가치가 있음.
- 제안: `DeliveryRepo.PruneTerminal(before, keepPerWebhook?)` + 시간 기반(예: N일 경과한 delivered/dead 삭제) 스위퍼를 세션 스위퍼와 대칭으로 추가하거나, backlog에 정식 등록.

### LOW

**L1. `webhook_deliveries.webhook_id` 인덱스 부재** — `internal/store/schema.sql:150`
- 인덱스는 `idx_deliveries_due(status, next_attempt_at)`만 존재. `ListByWebhook`(`WHERE webhook_id=? ORDER BY id DESC`)과 `ON DELETE CASCADE(webhook_id)`가 풀스캔.
- M1의 무한 증가와 결합 시 열람·삭제가 점진적으로 느려짐. `CREATE INDEX idx_deliveries_webhook ON webhook_deliveries(webhook_id)` 추가 권장(M1과 함께 처리하면 자연스러움).

**L2. `handleListDeliveries` — 미존재 webhook id에 404 아닌 200 `[]`** — `internal/server/handlers_webhooks.go:362`
- 다른 핸들러(delete/toggle)는 unknown id에 `ErrNotFound`→404지만, deliveries는 존재 확인 없이 빈 배열 200 반환.
- 보안 영향 없음(admin-only). API 일관성 nit — 선택적으로 `WebhookRepo.ByID` 존재 확인 후 404 처리.

**L3. `copySecret` 조용한 실패 (Design P2 재확인)** — `web/dashboard/src/pages/Webhooks.tsx:298`
- `navigator.clipboard.writeText` 거부 시 `setCopied(false)`만 하고 사용자 피드백 없음.
- **데이터 손실 아님**: 시크릿은 readOnly `TextField`(`onFocus` 시 select)에 노출되어 수동 선택·복사가 가능. 다만 1회성 시크릿이라 복사 실패 시 폴백 안내(예: "수동 선택하여 복사하세요")가 있으면 안전. advisory.

## Reused advisory findings (비차단, PR 본문 기재됨)

이미 `/mccp:pr` 게이트가 PR 본문에 주입한 항목 — 별도 사이클(`/mccp:prp-implement`)에서 처리 권장:
- **A11y Critical**: WCAG 2.5.8 Target Size(`WebhookRow` 액션 버튼 `size="1"` <24px), 4.1.3 Status Messages("복사됨" `aria-live` 부재).
- **A11y Serious**: WCAG 1.3.1(이벤트 체크박스 그룹 `fieldset`/`legend`/`role="group"` 부재), 3.3.1(개별 필드 에러 미포커스).
- **Design P2/P3**: copySecret 폴백(=L3), URL hint와 서버 SSRF 정책 메시징 긴장(`Webhooks.tsx:386`), 긴 URL 셀 오버플로우(`WebhookRow.tsx:38`).

## 강점 (확인 사항)

- **동시성**: `ClaimDue`가 커서를 닫은 뒤 per-row CAS(`WHERE status='pending'`)로 claim, Mark*가 `WHERE status='in_progress'`로 가드 → stale worker의 finalize 무효화. `ReclaimStale`은 attempts 미증가(at-least-once). 정확.
- **SSRF**: `ValidateURL`(등록) + `SafeClient.DialContext`(dial 시 전 IP 재검증, 하나라도 차단 시 dial 실패)로 DNS-rebinding TOCTOU 폐쇄. redirect 거부, proxy nil, IPv4-mapped IPv6 정규화. loopback/RFC1918 의도 허용(문서화).
- **시크릿 비유출**: 페이로드 구조에 값 필드 부재(`payload.go`), List/MatchForEvent가 `secret_ciphertext` 미조회(ByID만 조회), `transportReason`이 `url.Error` unwrap으로 userinfo 포함 URL 로깅 회피, 워커가 키·URL 미로깅.
- **transactional outbox**: 3개 시크릿 핸들러가 변경+audit+enqueue를 단일 tx에 배선 → 롤백 시 유령 배달 없음.
- **HMAC**: `<ts>.<body>` 서명으로 replay 방어, 수신자 constant-time 비교 안내 주석 정확.

## Validation Results

| Check | Result |
|---|---|
| go build ./... | Pass |
| go vet ./... | Pass |
| go test ./... | Pass (전 패키지 green) |
| golangci-lint run | Pass (0 issues) |
| dashboard tsc typecheck | Pass |
| dashboard vitest (Webhooks.test.tsx) | Pass (5/5) |
| dashboard axe WCAG 2.2 (e2e) | Skipped (로컬 미실행 — a11y-architect advisory로 대체, CI 위임) |
| go test -race | Skipped (로컬 32-bit gcc 제약 — CI(linux) 위임) |

## Files Reviewed (핵심)

**Go 소스 (Modified/Added)**
- `internal/webhook/{ssrf,signer,payload,worker}.go` — SSRF 정책·HMAC·페이로드·배달 워커
- `internal/store/{delivery_repo,webhook_repo,env_repo}.go`, `schema.sql` — outbox/CAS/저장소/스키마
- `internal/server/{handlers_webhooks,handlers_secrets,router,server}.go` — 5개 admin 라우트 + enqueue 배선
- `cmd/server/main.go`, `cmd/cli/cmd_webhook.go` — 워커 라이프사이클 + CLI
- `pkg/client/client.go` — 클라이언트 메서드

**TS 소스 (Modified/Added)**
- `web/dashboard/src/pages/Webhooks.tsx`, `components/WebhookRow.tsx` — 목록/등록/삭제/배달 다이얼로그

**게이트 범위 밖 델타**
- `4af29b2` (gosec G104, `_ = x.Close()` ×5) — 직접 확인, 로직 무변경
