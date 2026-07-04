# 구현 리포트: Webhooks (M4)

## 요약

시크릿 변경(upsert/rollback/delete) 시 등록된 URL로 **서명된 메타데이터 이벤트**를
POST하는 웹훅 서브시스템을 배송했다. Transactional outbox로 커밋된 변경만 배달하고,
원자적 lease claim으로 동시 워커 중복을 막으며, SSRF 하드닝(link-local/metadata 차단
+ DNS-rebinding 방어)과 HMAC-SHA256 서명을 갖춘다. 페이로드에 시크릿 평문은 구조적으로
없다. CLI(`secret webhook`), 클라이언트 메서드, 대시보드 Webhooks 화면까지 포함.

M4의 나머지 절반(secret referencing/overrides)은 M1/M2 resolver로 선-배송되어 본
마일스톤은 webhooks 단독 범위였다(재계획 시 중복).

## 계획 대비 실제

| 지표 | 계획 | 실제 |
|---|---|---|
| Complexity | Medium-Large | Medium-Large (일치) |
| 신규 패키지 | `internal/webhook` | `internal/webhook` (payload/signer/ssrf/worker) |
| 커버리지 목표 | 패키지별 ≥80% | store 82.4%, webhook 83.8% |

## 완료 태스크

| # | 태스크 | 상태 | 비고 |
|---|---|---|---|
| 1 | 스키마 + 타입 | 완료 | `webhooks`·`webhook_deliveries` + lease 컬럼(`claimed_at`) |
| 2 | `WebhookRepo` | 완료 | Create/List(secret 제외)/ByID(secret 포함)/Delete/SetEnabled/MatchForEvent |
| 3 | `DeliveryRepo` (원자적 claim) | 완료 | CAS claim + Mark* 가드 + ReclaimStale; 동시-claim 테스트 |
| 4 | payload + signer | 완료 | 값 필드 없는 Payload + HMAC 서명 |
| 5 | 배달 워커 + SSRF | 완료 | ssrf.go(ValidateURL/SafeClient) + worker.go |
| 6 | 웹훅 핸들러 + enqueue 헬퍼 | 완료 | admin-only 4 라우트 + `enqueueWebhooks` |
| 7 | 이벤트 소스 배선 | 완료 | 3개 시크릿 핸들러 tx에 enqueue |
| 8 | client 메서드 | 완료 | Create/List/Delete/ListDeliveries |
| 9 | `secret webhook` 서브커맨드 | 완료 | create/list/delete/deliveries |
| 10 | 워커 라이프사이클 배선 | 완료 | `--webhook-poll-interval`/`COMAX_WEBHOOK_POLL` |
| 11 | 배달 통합 테스트 | 완료 | HTTP 등록→upsert→워커→유효 HMAC·평문 부재 E2E |
| 12 | 대시보드 Webhooks 화면 | 완료 | 목록·1회노출 등록·삭제·배달 다이얼로그 |
| 13 | 대시보드 nav + 라우팅 | 완료 | `/integrations/webhooks` + axe e2e |
| 14 | 문서 + PRD 정합성 | 완료 | docs/webhooks.md·threat-model·README·PRD |

## 검증 결과

| 레벨 | 상태 | 비고 |
|---|---|---|
| 정적 분석 | Pass | `go build ./...` + `go vet ./...` clean; dashboard tsc/eslint clean |
| 단위 테스트 | Pass | store 82.4%, webhook 83.8%; server/client/cli green |
| 빌드 | Pass | `go build` + `vite build`(dashboard dist 임베드) |
| 통합 | Pass | 배달 E2E(유효 HMAC + 평문 부재) + 대시보드 axe e2e(WCAG 2.2 AA) |
| 엣지 | Pass | 동시 claim·stale reclaim·SSRF metadata 차단·retry→dead·ctx 취소 |

> **환경 제약**: `-race`는 cgo(C 툴체인)를 요구하나 로컬 gcc가 32비트 전용이라
> 로컬에서 불가. 동시성 로직은 결정론적 goroutine 테스트로 커버했고, `-race`
> 전수 검증은 CI(linux)에 위임한다.

## 계획 대비 편차 (WHAT / WHY)

- **`Payload.DeliveryID` 필드 제거** — outbox 행 PK는 enqueue 시점에 없어 서명 body에
  넣으려면 insert 후 재-UPDATE가 필요. GitHub webhook 관례(`X-*-Delivery` 헤더 + body만
  서명)를 따라 delivery id를 **`X-Comax-Delivery` 헤더**로 전달. 서명 body가 enqueue
  시점에 확정되고 tx 내 추가 UPDATE가 사라진다.
- **배달 상태 `failed` 미채택** — 상태 머신을 `pending→in_progress→{delivered|dead}`로
  설계. 재시도는 `in_progress→pending` 재진입이라 `failed`는 전이 없는 dead code가 됨.
  4-state로 축소(관측 동작 동일).
- **`ValidateURL`에 ctx 인자 추가** — 계획의 2-arg 시그니처에 context를 더해 요청
  context 전파. `SafeClient`에 timeout 인자 명시.
- **CLI `webhook deliveries` 서브커맨드 추가** — 계획의 create/list/delete에 더해,
  client `ListDeliveries`를 CLI에서도 도달 가능하게 함(운영자 배달 상태 확인).

## 이슈

- **impeccable design 게이트**: 대시보드 `.tsx` 편집이 design-quality 훅에 막혀
  `/impeccable layout`을 세션 내 호출해 해제. PRODUCT.md register=product + GitHub
  Settings 어휘 확인 후 기존 Tokens 패턴 그대로 미러링(색은 의미에만).

## 테스트 작성

| 파일 | 커버 |
|---|---|
| `internal/store/webhook_repo_test.go` | Create/List(민감필드 부재)/ByID/Delete/SetEnabled/MatchForEvent(필터·substring 가드) |
| `internal/store/delivery_repo_test.go` | Enqueue/due 필터/동시 claim/Mark* 가드/retry/dead/ReclaimStale |
| `internal/webhook/{signer,ssrf,worker}_test.go` | 서명 결정성·SSRF metadata/allow/deny·유효 HMAC·평문 부재·retry→dead·ctx 취소 |
| `internal/server/handlers_webhooks_test.go` | 1회노출·비admin 403·SSRF 거부·enqueue 매칭·롤백-무유령 |
| `internal/server/webhook_integration_test.go` | HTTP 등록→변경→워커 배달 E2E |
| `pkg/client/client_test.go` | 웹훅 메서드 라운드트립 + APIError |
| `cmd/cli/cmd_webhook_test.go` | create 1회출력·list·delete·SSRF 거부 |
| `web/dashboard/src/pages/Webhooks.test.tsx` | 목록·403·1회노출·삭제 |
| `web/dashboard/tests/e2e/a11y.spec.ts` | `/integrations/webhooks` axe WCAG 2.2 AA |

## Codex Adversarial 흡수 확인

- **F1 (동시 claim 중복)** → 원자적 CAS lease claim + Mark* `WHERE status='in_progress'`
  가드 + `ReclaimStale`. 동시 goroutine claim 테스트가 한 행 1회 배달 증명.
- **F2 (SSRF)** → link-local/metadata 기본 차단 + 리다이렉트 미추종 + DialContext IP
  재검증 + opt-in allow/deny. RFC1918/loopback 허용. metadata 거부 테스트 green.
- **F3 (서명 시크릿 회전)** → backlog 이연(delete+recreate 워크어라운드).

## 다음 단계

- [x] `/mccp:prp-commit` 로 커밋
- [x] `/mccp:pr` 로 PR 생성 → **PR #13 병합 완료** (2026-07-03, merge `27ef96b`).
- [x] `/mccp:code-review 13` → **APPROVE (with comments)**; CRITICAL/HIGH 0. 리뷰: [`../reviews/archive/pr-13-review.md`](../reviews/archive/pr-13-review.md).
- [x] PRD M4 → `complete` 확정.
- [ ] 후속(비차단): 배달 이력 prune/retention + `webhook_deliveries.webhook_id` 인덱스. `codex-findings-backlog.md`에 등록됨(F3 회전과 함께 M4 후속/v2 후보).
