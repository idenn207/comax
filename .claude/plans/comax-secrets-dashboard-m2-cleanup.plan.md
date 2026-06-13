# Plan: Comax Secrets — M2 Cleanup (Sessions 통제 + 위협 모델 정정)

**Source PRD**: [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md)
**Source Review**: Codex adversarial review session `019e8677-15d6-7ce1-a5ce-468c9c5831f9` (verdict: `needs-attention`, 4 high + 2 medium)
**M2 본체 plan**: [comax-secrets-dashboard.plan.md](./comax-secrets-dashboard.plan.md) (PR #3·#5·#6·#7·#8 머지 완료)
**Selected Milestone**: #2 — Dashboard UI cleanup/hardening pass
**Complexity**: Medium (옛 plan 대비 ~30% 축소 — master 사실 검증으로 2개 결함이 이미 해결된 상태 확인)
**Plugin**: mccp (옛 plan은 ECC 기반이었음. 명령·CLI 모두 mccp로 매핑)

## Summary

M2 본체는 master에 머지됐지만 Codex adversarial review가 6개 결함을 지적했다. 사용자 결정은 **선택지 D — 세션 통제 완전 구현 + 위협 모델 정정**. master 사실 검증 결과 6개 결함 중 2개(pnpm-workspace placeholder, Dockerfile 공급망 우회)는 이미 master에서 해결된 상태이므로 본 plan은 **남은 4개 + Sessions 통제 신규 구현 + 위협 모델 신규 섹션**에 집중한다.

성공은 binary: ① 운영자가 `/settings/sessions`에서 자신의 token으로 만들어진 모든 dashboard 세션을 보고 임의 세션을 회수할 수 있다, ② `cmd/server/main.go`의 server 라이프사이클이 1시간 주기 prune sweeper를 띄워 만료 세션을 정리한다, ③ `docs/threat-model.md`에 Browser sessions 섹션이 신규 추가되어 CSRF 한계(mutation only, GET은 cookie 단독)와 cookie 탈취 시 read exfiltration 가능성이 명시된다, ④ CI에 size gate가 신규 추가되어 dashboard JS payload + secret-server binary 크기가 자동 검증된다(fail-closed), ⑤ M2 본체 plan이 `completed/`로 archive 이동하고 PRD M2 행이 `done`으로 마감.

## master 사실 검증 (옛 plan과의 차이)

| 옛 plan 항목 | Master 실제 상태 | 본 plan에서의 처리 |
|---|---|---|
| Task 1.1 pnpm-workspace placeholder 제거 | [web/dashboard/pnpm-workspace.yaml](../../web/dashboard/pnpm-workspace.yaml) 5줄, `allowBuilds: esbuild: true`만. placeholder 없음 | **드롭** — 이미 해결됨 |
| Task 1.2 Dockerfile `PNPM_CONFIG_DANGEROUSLY_ALLOW_ALL_BUILDS=true` 제거 | [deploy/docker/Dockerfile](../../deploy/docker/Dockerfile)은 Go binary만 빌드. pnpm/dashboard 임베드 step 자체가 없음 | **드롭** — 해당 ENV 자체가 master에 존재하지 않음 |
| Task 1.3 CI size gate fail-closed | [.github/workflows/ci.yml](../../.github/workflows/ci.yml)에 size gate **자체가 없음** | **재정의** — size gate를 신규 추가하면서 처음부터 fail-closed로 |
| Task 1.4 plan archive 링크 그래프 복원 | M2 본체 plan은 [.claude/plans/comax-secrets-dashboard.plan.md](./comax-secrets-dashboard.plan.md)에 아직 활성. archive 미수행 | **유지 + 단순화** — 본 cleanup 완료 시점에 한꺼번에 archive |
| Task 2.D1~D7 Sessions + threat-model | [internal/server/handlers_sessions.go](../../internal/server/handlers_sessions.go)는 단수형(`POST/DELETE /api/v1/dashboard/session`)만 존재. ListBySubject/ListByTokenID 없음. `/settings/sessions` 라우트 없음. `Server.Run()` 메서드 없음. `docs/threat-model.md`에 Browser sessions 섹션 없음 | **유지 + 정정** — D1을 token_id 스코프로, D2 위치를 main.go로, D7을 신규 섹션 추가로 |
| Phase 3 dogfood 실측 + archive | [docs/dogfood.md](../../docs/dogfood.md)는 M1 CLI 측정용. dashboard-only Flow는 master에 없음 | **유지 + 분리** — `docs/dashboard-dogfood.md` 신규 작성 |

## Decisions (선택지 D, master 시점)

| # | Decision | Choice | Rationale |
|---|---|---|---|
| **D1** | Sessions REST API scope | **token_id scope, no oracle**: `GET /api/v1/dashboard/sessions`는 현재 token이 발급한 세션만 반환. `DELETE /api/v1/dashboard/sessions/{id}`는 SQL WHERE에 token_id를 원자적으로 결합 — `UPDATE dashboard_sessions SET revoked_at=? WHERE id=? AND token_id=? AND revoked_at IS NULL`. **affected rows == 0**(다른 token의 id이거나, 존재하지 않거나, 이미 revoked)인 모든 경우를 **동일 응답으로 통일**: 204 (idempotent). 핸들러는 actor token_id와 target id의 관계를 응답으로 노출하지 않는다. 둘 다 cookie + CSRF 필요. | M1 schema는 `token_id`로만 묶임 + multi-token admin이 PRD 범위 밖. **Codex F2 (MEDIUM, 0.88)**: id PK가 정수라 다른 token_id 시도 시 403/404 분리는 brute-force oracle. 0 rows를 모두 204로 통일하면 oracle 닫힘 + revoke 자체가 idempotent여서 UX 자연. |
| **D2** | Prune sweeper 위치·주기·cutoff | **`cmd/server/main.go`의 `run()` 함수**에 `time.NewTicker(time.Hour)` 고루틴 추가. tick마다 **`store.NewSessionRepo(db).Prune(ctx, time.Now())`** — 만료 즉시 1시간 이내 정리. `<-ctx.Done()`에 ticker.Stop(). | M1 컨벤션 (별도 daemon 없음). `httpSrv.ListenAndServe()` 같은 자리의 goroutine 패턴([main.go:141-147](../../cmd/server/main.go#L141-L147)). **Codex F3 (MEDIUM, 0.84)**: 옛 plan은 `cutoff = time.Now() - sessionTTL` (30일 추가 grace) — threat model의 "1시간 주기 만료 정리" 약속과 어긋났음. cutoff를 `time.Now()`로 통일해 row가 `expires_at < now` 즉시 다음 tick에 삭제. session TTL(30일)은 expires_at 발급 시점 결정. |
| **D3** | Sessions UI 라우트·위치 | **Code-based router** ([web/dashboard/src/router.tsx](../../web/dashboard/src/router.tsx))에 `settingsSessionsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/settings/sessions', beforeLoad: requireAuth, component: SessionsPage })` 추가 + `rootRoute.addChildren([...기존..., settingsSessionsRoute])`. 페이지 컴포넌트는 `web/dashboard/src/pages/Sessions.tsx`(다른 라우트의 `Projects.tsx`/`Audit.tsx`와 동일). 행 컴포넌트는 `web/dashboard/src/components/SessionRow.tsx`. revoke 확인은 기존 [`ConfirmDialog.tsx`](../../web/dashboard/src/components/ConfirmDialog.tsx) 재사용. 헤더에 "Sessions" 링크. | **Codex F1 (HIGH, 0.96)**: master는 code-based router. file-based 라우트 파일만 만들면 도달 불가. 또한 master에 `src/features/` 디렉터리 자체가 없음 — page는 `src/pages/`, 재사용 컴포넌트는 `src/components/`. 기존 `ConfirmDialog`는 destructive confirm 패턴을 이미 가짐. |
| **D4** | 위협 모델 신규 섹션 | `docs/threat-model.md`에 **신규 "Browser sessions" 섹션 추가** (옛 plan은 "정정"이라 했으나 master에 섹션 자체가 없음). 두 단락 구성: (a) **Mitigation**: "운영자는 `/settings/sessions`에서 자신의 token이 발급한 dashboard 세션을 보고 임의 세션을 회수할 수 있다. 만료 세션 row는 `cmd/server`의 1시간 주기 sweeper가 정리한다", (b) **Limit**: "CSRF 토큰은 mutation에만 요구되며 GET API는 cookie 단독으로 인증된다. cookie 탈취는 read exfiltration에 충분하므로 cookie 보호(httpOnly + Secure + SameSite=Strict)가 보안 경계다. revoke는 회수 수단이지 탈취 방지 수단이 아니다." | Codex 지적의 핵심. 보안 약속을 부풀리지 않고 정직히 표기. |
| **D5** *(신규)* | CI size gate | `.github/workflows/ci.yml`의 `dashboard-e2e` job 끝에 size gate step 추가. **fail-closed**로: dist 부재 시 명시적 실패, JS asset 매치 0건이면 실패, **raw** total payload(JS+CSS raw bytes) > 500 KB면 실패, `bin/secret-server` > 30 MB면 실패. baseline은 **raw bytes**로 통일(gzip 측정 안 함). | M2 plan의 acceptance에 size budget이 있는데 master CI에 강제 step이 없음. **Codex F4 (MEDIUM, 0.92)**: 옛 plan은 "gz 500KB" 약속 + 스니펫은 raw bytes — baseline 혼동. 가장 단순한 정정은 raw byte로 통일 (CDN gzip은 transport 최적화이고, repo CI에서 측정 일관성이 우선). |

### Cross-gate dedupe (Codex 합치 항목 — 재호출 불필요)

mccp Command Gates에 따라 이미 첫 round에서 합치된 결정은 본 plan에서는 한 줄로만 기록한다:

- **단일 키 pnpm 설정** — master에 이미 적용된 상태 (dedupe 불필요)
- **Docker 공급망 보호** — master에 해당 ENV가 없는 상태 (dedupe 불필요)
- **plan archive 이동 + incoming 링크 갱신** — 합치됨. Phase 3에서 단일 PR로 처리.

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| 라우터 등록 | [internal/server/router.go:23-24](../../internal/server/router.go#L23-L24) | D1 신규 라우트는 동일 `mux.HandleFunc("METHOD /path/{param}", s.handlerXxx)` 스타일. chi/gorilla 도입 금지. |
| Repository pattern | [internal/store/session_repo.go:36-65](../../internal/store/session_repo.go#L36-L65) (`Create`) | D1 list endpoint를 위해 `SessionRepo.ListByTokenID(ctx, tokenID int64) ([]DashboardSession, error)` 신규. `expires_at > now AND revoked_at IS NULL` 필터. `token_id` 인덱스 활용. |
| Prune 호출 | [internal/store/session_repo.go:139-150](../../internal/store/session_repo.go#L139-L150) | D2 ticker는 기존 `Prune(ctx, cutoff time.Time)` 시그니처 그대로. `time.Now().Add(-sessionTTL)`를 인수로. |
| 라이프사이클 goroutine | [cmd/server/main.go:140-147](../../cmd/server/main.go#L140-L147) (`httpSrv.ListenAndServe()`) | D2 prune ticker는 같은 위치에서 동일하게 `errCh chan error` + `select` 패턴으로. 종료 시 `<-ctx.Done()`. |
| audit append | [internal/server/handlers_projects.go:91-99](../../internal/server/handlers_projects.go#L91-L99) | D1 `DELETE /sessions/{id}`는 mutation transaction 안에서 `appendAudit(...)`. action `session.revoke_by_id`. metadata에 `target_session_id` (token 본문 절대 금지), `actor_token_id`, truncated remote IP. |
| Middleware 체인 | [internal/server/router.go:52](../../internal/server/router.go#L52) (`recover ← log ← auth ← mux`) | D1 신규 라우트는 동일 체인 통과. CSRF 검사는 `DELETE`에 자동 적용(기존 middleware가 method 기반). 새 미들웨어 추가 금지. |
| 핸들러 테스트 | [internal/server/handlers_sessions_test.go](../../internal/server/handlers_sessions_test.go), [internal/server/server_test.go](../../internal/server/server_test.go) | D1 신규 핸들러는 `httptest.Server` + 임시 SQLite + 테이블 드리븐. 다른 token_id 시도 → 403, 자기 token_id → 200/204, 존재하지 않는 id → 404. |
| Repo 테스트 | [internal/store/session_repo_test.go](../../internal/store/session_repo_test.go) | `ListByTokenID` 테스트는 다른 token_id row가 결과에 포함 안 됨을 명시적으로 검증. 만료/회수된 row 제외. |
| Frontend 파일 layout | [web/dashboard/src/features/projects/](../../web/dashboard/src/features/projects/), [web/dashboard/src/features/secrets/](../../web/dashboard/src/features/secrets/) | D3 Sessions 화면을 `src/features/sessions/`로 신설. `SessionsPage.tsx`, `SessionRow.tsx`, `useSessions.ts`. |
| TanStack Router 라우트 | 기존 `web/dashboard/src/routes/` 라우트들 | `/settings/sessions` 파일 기반 라우트 추가. search params 사용 안 함(목록뿐). |
| Anti-template (Sessions UI) | mccp `frontend-design-direction` skill (있을 시) + 옛 plan 정신 | Radix `AlertDialog` + 의도된 hover/focus 상태. 기본 회색 카드 그리드 금지. Sessions 행은 device/IP/last-seen을 typographic hierarchy로 구분. |
| docs 톤 | [docs/threat-model.md](../../docs/threat-model.md) 기존 한국어 톤·표 구조 | D4 신규 섹션은 기존 "What the system protects" / "What the system does NOT protect" 표와 동일한 마크다운 톤. **Mitigation** / **Limit** 소제목으로 분리. |
| CI 단계 작성 | [.github/workflows/ci.yml:178-189](../../.github/workflows/ci.yml#L178-L189) (build + embed + playwright) | D5 size gate는 `dashboard-e2e` job의 마지막 step. bash + `set -euo pipefail`. fail-closed 가드 인라인. |

## Files to Change

### 백엔드 (Go)

| File | Action | Why |
|---|---|---|
| `internal/store/session_repo.go` | UPDATE | `ListByTokenID(ctx, tokenID int64) ([]DashboardSession, error)` 신규. `expires_at > now AND revoked_at IS NULL`. |
| `internal/store/session_repo_test.go` | UPDATE | `ListByTokenID` 테이블 드리븐 테스트. 다른 token_id row 격리, 만료·revoked row 제외 검증. |
| `internal/server/handlers_sessions.go` | UPDATE | `handleListDashboardSessions` (현재 actor token_id scope), `handleRevokeDashboardSessionByID` (target session의 token_id가 actor와 일치할 때만). 둘 다 audit append. |
| `internal/server/handlers_sessions_test.go` | UPDATE | 다른 token_id 시도 → 403, 자기 token_id → 200/204, 존재하지 않는 id → 404. |
| `internal/server/router.go` | UPDATE | `GET /api/v1/dashboard/sessions`, `DELETE /api/v1/dashboard/sessions/{id}` 라우트 등록. 기존 단수형 라우트(`POST/DELETE /api/v1/dashboard/session`)는 유지. |
| `cmd/server/main.go` | UPDATE | `run()` 함수의 `httpSrv.ListenAndServe()` goroutine 아래에 prune sweeper goroutine 추가. `time.NewTicker(time.Hour)`, `<-ctx.Done()` 종료. session TTL은 const 또는 별도 sweepInterval flag 도입은 보류(기본 30일 가정). |
| `cmd/server/main_test.go` (또는 별도 `prune_sweeper_test.go`) | CREATE/UPDATE | 짧은 interval(10ms) 주입 가능한 형태로 sweeper를 함수로 추출(`runPruneSweeper(ctx, repo, interval, ttl)`) 후 단위 테스트. integration은 만료 row가 N tick 후 삭제됨을 확인. |

### 프론트엔드 (TypeScript) — Codex F1 absorb: code-based router + `src/pages/` 구조

| File | Action | Why |
|---|---|---|
| `web/dashboard/src/pages/Sessions.tsx` | CREATE | 목록 + revoke 버튼. ConfirmDialog. 다른 page와 동일 layout. |
| `web/dashboard/src/components/SessionRow.tsx` | CREATE | 개별 행. device(user_agent 요약) / IP / created typographic hierarchy. |
| `web/dashboard/src/lib/sessions.ts` (또는 `lib/api.ts` 확장) | CREATE/UPDATE | TanStack Query 훅 + `listSessions()` / `revokeSession(id)` API 래퍼. envelope unwrap + CSRF 자동 첨부 기존 패턴. |
| `web/dashboard/src/router.tsx` | UPDATE | `settingsSessionsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/settings/sessions', beforeLoad: requireAuth, component: SessionsPage })` 추가 + `rootRoute.addChildren([..., settingsSessionsRoute])`. |
| `web/dashboard/src/components/AppShell.tsx` (또는 헤더 영역) | UPDATE | "Sessions" 링크를 헤더 또는 사용자 메뉴에 노출. |
| `web/dashboard/src/components/ConfirmDialog.tsx` | (재사용) | 기존 destructive confirm 패턴 그대로 사용. 새 dialog 만들지 않음. |
| `web/dashboard/src/pages/Sessions.test.tsx` | CREATE | vitest + RTL. 다른 페이지 테스트(`Projects.test.tsx` 등)와 동일 스타일. |
| `web/dashboard/tests/sessions.spec.ts` | CREATE | Playwright + axe. 두 BrowserContext로 두 세션 만들고 ① context1에서 `/settings/sessions` 진입 → 두 세션 보임, ② context2 세션 revoke → 204, ③ context2에서 401 확인, ④ context1이 자기 자신 revoke 시도 → 버튼 disabled (UI level) + 백엔드도 idempotent 204. |

### 문서

| File | Action | Why |
|---|---|---|
| `docs/threat-model.md` | UPDATE | **신규 "Browser sessions" 섹션 추가**. D4 두 단락(Mitigation / Limit). 기존 "Reporting a vulnerability" 섹션 앞에 삽입. |
| `docs/dashboard.md` | UPDATE | Sessions 화면 사용법 단락 추가. |
| `docs/dashboard-dogfood.md` | CREATE | Phase 3 측정용. dashboard-only Flow A/B/C 정의 + 사용자 측정 placeholder. (master `docs/dogfood.md`는 M1 CLI flow 전용이라 분리.) |

### CI

| File | Action | Why |
|---|---|---|
| `.github/workflows/ci.yml` | UPDATE | `dashboard-e2e` job 끝에 size gate step **신규 추가** (fail-closed). dist 부재 시 명시적 실패 / JS asset 매치 0건 시 실패 / total payload > 500 KB 시 실패 / `bin/secret-server` > 30 MB 시 실패. |

### plan / PRD

| File | Action | Why |
|---|---|---|
| `.claude/plans/comax-secrets-dashboard.plan.md` | MOVE → `.claude/plans/completed/` | Phase 3 마지막에 M2 본체 plan을 archive. 첫 줄에 archive 메타 한 줄. |
| `.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md` (본 plan) | MOVE → `.claude/plans/completed/` | 같은 PR에서 본 plan도 archive. |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | `Delivery Milestones` M2 행: Plan 셀을 `completed/` 신경로로, 상태 `in-progress` → `done`. 옛 cleanup 경로(있다면)는 정리. |
| `README.md` | UPDATE | M2 plan 경로가 본문에 인용돼 있다면 archive 경로로 갱신. (Phase 1.2에서 grep으로 확인.) |

## Tasks

### Phase 1 — Cross-gate dedupe 항목 + 신규 size gate (~1시간)

#### Task 1.1: CI size gate 신규 추가 (fail-closed)
- **Action**: [.github/workflows/ci.yml:178-189](../../.github/workflows/ci.yml#L178-L189) `dashboard-e2e` job 끝에 `Size budget gate` step 추가. bash + `set -euo pipefail`:
  ```bash
  DIST="internal/server/dashboard/dist"
  BIN="bin/secret-server"
  [ -d "$DIST" ] || { echo "::error::dashboard dist missing at $DIST"; exit 1; }
  [ -f "$BIN" ]  || { echo "::error::server binary missing at $BIN"; exit 1; }

  shopt -s nullglob
  JS_FILES=("$DIST"/assets/*.js)
  CSS_FILES=("$DIST"/assets/*.css)
  [ "${#JS_FILES[@]}" -gt 0 ] || { echo "::error::no JS assets matched in $DIST/assets/*.js"; exit 1; }

  JS_BYTES=$(du -bc "${JS_FILES[@]}" | tail -1 | awk '{print $1}')
  CSS_BYTES=0
  if [ "${#CSS_FILES[@]}" -gt 0 ]; then
    CSS_BYTES=$(du -bc "${CSS_FILES[@]}" | tail -1 | awk '{print $1}')
  fi
  BIN_BYTES=$(du -b "$BIN" | awk '{print $1}')

  TOTAL_PAYLOAD_MAX=$((500 * 1024))
  BIN_MAX=$((30 * 1024 * 1024))

  echo "js=$JS_BYTES css=$CSS_BYTES total=$((JS_BYTES+CSS_BYTES)) bin=$BIN_BYTES"
  [ $((JS_BYTES + CSS_BYTES)) -le "$TOTAL_PAYLOAD_MAX" ] \
    || { echo "::error::dashboard payload $((JS_BYTES+CSS_BYTES)) > $TOTAL_PAYLOAD_MAX bytes"; exit 1; }
  [ "$BIN_BYTES" -le "$BIN_MAX" ] \
    || { echo "::error::secret-server $BIN_BYTES > $BIN_MAX bytes"; exit 1; }
  ```
- **Mirror**: 기존 `dashboard-e2e` step들의 bash + `set -euo pipefail` 패턴.
- **Validate**:
  ```powershell
  Get-Content .github/workflows/ci.yml | Select-String "Size budget gate"
  ```
  CI 실행 후 step PASS 확인.

#### Task 1.2: 옛 cleanup plan 잔재 정리
- **Action**: `.worktrees/dashboard-ui/.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md`이 worktree에만 있어 master에서 추적 안 됨. 본 plan으로 대체되므로 worktree 정리는 사용자 판단(자동 삭제 안 함). `grep -r "comax-secrets-dashboard-m2-cleanup"`로 master 트리 내 옛 cleanup 경로 인용 0건 확인.
- **Validate**:
  ```powershell
  Select-String -Path README.md, .claude/prds/*.md, .claude/reports/*.md, .claude/plans/**/*.md `
                -Pattern "comax-secrets-dashboard-m2-cleanup" -ErrorAction SilentlyContinue
  # 본 plan 파일 자체 외 잔여 0건이어야 함.
  ```

### Phase 2 — 선택지 D: Sessions 통제 + 위협 모델 (~6-10시간)

#### Task 2.D1: SessionRepo.ListByTokenID
- **Action**: [internal/store/session_repo.go](../../internal/store/session_repo.go)에 `ListByTokenID(ctx context.Context, tokenID int64) ([]DashboardSession, error)` 추가. SQL: `SELECT ... FROM dashboard_sessions WHERE token_id = ? AND revoked_at IS NULL AND expires_at > ? ORDER BY created_at DESC`. token_id 인덱스가 schema에 이미 있는지 [internal/store/schema.sql](../../internal/store/schema.sql) 확인 후 없으면 마이그레이션 추가.
- **Mirror**: 기존 `ByHash` 패턴(`unixSeconds` 등). list는 constant-time compare 불필요.
- **Validate**:
  ```powershell
  go test -race -run "TestSessionRepo_ListByTokenID" ./internal/store
  ```
  통과 + 커버리지 ≥ 80%. 다른 token_id row가 결과에 포함 안 됨을 명시 검증.

#### Task 2.D2: Sessions 핸들러 + 라우터 (Codex F2 absorb)
- **Action**:
  1. [internal/store/session_repo.go](../../internal/store/session_repo.go)에 **`RevokeByIDAndTokenID(ctx, id int64, tokenID int64) error`** 신규. SQL: `UPDATE dashboard_sessions SET revoked_at=? WHERE id=? AND token_id=? AND revoked_at IS NULL`. **affected rows == 0**이면 그냥 `nil` (idempotent — id가 다른 token 소유, 미존재, 이미 revoked를 구분하지 않음). 따라서 oracle 없음.
  2. [internal/server/handlers_sessions.go](../../internal/server/handlers_sessions.go)에 `handleListDashboardSessions`, `handleRevokeDashboardSessionByID` 추가. 둘 다 context에서 actor의 token_id 추출.
  3. revoke 핸들러: **target을 미리 fetch하지 않는다**. transaction 안에서 `SessionRepo.RevokeByIDAndTokenID(ctx, targetID, actor.TokenID)` 호출 → 항상 204. (affected rows == 1인 경우만 `appendAudit(ctx, tx, ...)`로 `session.revoke_by_id` 기록. 0이면 audit row 만들지 않음 — 다른 token 소유 id를 시도한 흔적이 audit에도 남지 않게 함.)
  4. [internal/server/router.go:24](../../internal/server/router.go#L24) 아래에:
     ```go
     mux.HandleFunc("GET /api/v1/dashboard/sessions", s.handleListDashboardSessions)
     mux.HandleFunc("DELETE /api/v1/dashboard/sessions/{id}", s.handleRevokeDashboardSessionByID)
     ```
  5. [internal/server/handlers_sessions_test.go](../../internal/server/handlers_sessions_test.go) 확장: ① 자기 token 세션 revoke → 204 + audit row 1개, ② **다른 token_id 시도 → 204 + audit row 0개** (oracle 닫힘), ③ 미존재 id → 204 + audit 0개, ④ 이미 revoked id → 204 idempotent, ⑤ CSRF 없는 DELETE → 403 (기존 미들웨어가 처리).
- **Mirror**: [handlers_projects.go:91-99](../../internal/server/handlers_projects.go#L91-L99) audit 패턴 (단, audit append를 affected rows 조건부로).
- **Validate**:
  ```powershell
  go test -race -cover -coverpkg=./internal/server,./internal/store `
          -run "TestHandle(List|Revoke).*Dashboard" ./internal/server
  ```
  커버리지 ≥ 80%. 핵심 assertion: cross-token 시도가 204 응답 + audit row 없음 (oracle 닫혔는지 검증).

#### Task 2.D3: Prune sweeper (cmd/server/main.go) — Codex F3 absorb
- **Action**:
  1. [cmd/server/main.go](../../cmd/server/main.go)에 helper 함수 `runPruneSweeper(ctx context.Context, repo *store.SessionRepo, interval time.Duration, logger *slog.Logger)` 추가. ttl 인자 **제거** — cutoff는 항상 `time.Now()`. ticker 루프 + `<-ctx.Done()` 종료 + 매 tick 에러는 warn 로그.
  2. `run()` 함수의 `httpSrv` goroutine 직후([main.go:148](../../cmd/server/main.go#L148))에:
     ```go
     go runPruneSweeper(ctx, store.NewSessionRepo(db), time.Hour, logger)
     ```
     함수 본문은:
     ```go
     func runPruneSweeper(ctx context.Context, repo *store.SessionRepo, interval time.Duration, logger *slog.Logger) {
         ticker := time.NewTicker(interval)
         defer ticker.Stop()
         for {
             select {
             case <-ctx.Done():
                 return
             case <-ticker.C:
                 if n, err := repo.Prune(ctx, time.Now()); err != nil {
                     logger.Warn("session prune failed", slog.Any("err", err))
                 } else if n > 0 {
                     logger.Info("sessions pruned", slog.Int64("rows", n))
                 }
             }
         }
     }
     ```
  3. 종료 흐름은 ctx가 동일하므로 자동 drain.
- **Mirror**: [main.go:140-147](../../cmd/server/main.go#L140-L147) goroutine 패턴.
- **Validate**:
  ```powershell
  go test -race -run "TestRunPruneSweeper" ./cmd/server
  ```
  10ms interval 주입으로 ① `expires_at < now`인 row가 1 tick 후 삭제됨 (cutoff = `time.Now()`이므로 만료 즉시 다음 tick에 정리됨을 검증), ② ctx 취소 시 sweeper가 즉시 종료됨.

#### Task 2.D4: Sessions UI 라우트 + 페이지 — Codex F1 absorb (code-based router + master 디렉터리 구조)
- **Action**:
  1. `web/dashboard/src/lib/sessions.ts` — TanStack Query 훅 + API 래퍼. `useSessionsList(): Query<Session[]>`, `useRevokeSession(): Mutation<id, void>`. invalidate on success. `listSessions()` / `revokeSession(id)` 함수.
  2. `web/dashboard/src/components/SessionRow.tsx` — props: `session`, `isCurrent`, `onRevoke`. typographic hierarchy(device 굵게 + IP 보조 + created 약하게). 현재 세션은 revoke 버튼 비활성화. Design Critique의 7가지 결정(P1+P2) 모두 반영.
  3. `web/dashboard/src/pages/Sessions.tsx` — table(role="table") + 기존 [`ConfirmDialog`](../../web/dashboard/src/components/ConfirmDialog.tsx) 재사용. empty state("다른 활성 세션이 없습니다"). loading skeleton. ConfirmDialog 카피는 Design Critique P1 #3의 honest warning 문구 그대로.
  4. `web/dashboard/src/router.tsx` UPDATE — **code-based router**. `Sessions` import + 신규 라우트:
     ```ts
     import { SessionsPage } from './pages/Sessions';

     const settingsSessionsRoute = createRoute({
       getParentRoute: () => rootRoute,
       path: '/settings/sessions',
       beforeLoad: requireAuth,
       component: SessionsPage,
     });
     // ...
     const routeTree = rootRoute.addChildren([
       indexRoute, projectRoute, envRoute, envDiffRoute,
       auditRoute, loginRoute,
       settingsSessionsRoute,  // 신규
     ]);
     ```
  5. `web/dashboard/src/components/AppShell.tsx` — 헤더 또는 사용자 메뉴에 `/settings/sessions` 링크 노출.
- **Mirror**: 다른 page(`Projects.tsx`, `Audit.tsx`) + 기존 `ConfirmDialog`. `src/features/` 디렉터리 만들지 않음 (master 컨벤션 아님).
- **Validate**:
  ```powershell
  cd web/dashboard
  pnpm typecheck
  pnpm lint
  pnpm test -- Sessions
  pnpm build
  ```
  브라우저 수동 확인 — `/settings/sessions` 진입 가능 (라우트 등록 검증).

#### Task 2.D5: e2e + axe gate
- **Action**: `web/dashboard/tests/sessions.spec.ts` 신규.
  - 두 BrowserContext로 두 세션 생성.
  - context1에서 `/settings/sessions` 진입 → 두 세션 모두 보임.
  - context2 세션을 revoke.
  - context2에서 API 호출 → 401 envelope 확인.
  - axe gate 진입 시 자동.
- **Mirror**: 기존 `dashboard-e2e` job의 Playwright 패턴.
- **Validate**:
  ```powershell
  cd web/dashboard
  pnpm exec playwright test sessions
  ```
  axe violation 0.

#### Task 2.D6: 위협 모델 신규 섹션
- **Action**: [docs/threat-model.md](../../docs/threat-model.md)의 "Audit log retention" 섹션 직후, "Reporting a vulnerability" 직전에 신규 섹션 삽입:
  ```markdown
  ## Browser sessions

  M2 dashboard는 service token을 cookie + CSRF 토큰 쌍으로 감싸 브라우저
  세션을 만든다. 보안 경계와 한계는 다음과 같다.

  **Mitigation (구현됨)**
  - 운영자는 `/settings/sessions`에서 자신의 service token이 발급한
    dashboard 세션을 모두 보고 임의 세션을 회수할 수 있다.
  - cookie는 `HttpOnly + Secure + SameSite=Strict + Path=/`로 발급되어
    JS에서 읽히지 않고 cross-site 요청에 동반되지 않는다.
  - 만료된 세션 row는 `cmd/server`가 1시간 주기로 돌리는 prune sweeper가
    삭제한다(기본 TTL 30일).
  - 모든 세션 mutation(rollback, delete, revoke 등)은 `audit_log`에 기록되고
    actor의 `token_id`와 truncated remote IP를 함께 남긴다.

  **Limit (정직히 표기)**
  - CSRF 토큰은 **mutation(POST/PUT/DELETE/PATCH)에만** 요구된다.
    `GET /api/v1/...` 요청은 cookie만으로 인증된다. cookie 단독 탈취는
    secret 값 read exfiltration에 충분하다.
  - 따라서 revoke는 **회수 수단**이지 **탈취 방지 수단이 아니다**.
    cookie 자체의 보호(HttpOnly+Secure+SameSite=Strict)와 운영자의
    workstation 위생이 보안 경계다.
  - M2는 multi-token admin 권한을 지원하지 않는다. 다른 service token이
    발급한 세션을 회수할 수 없다 — 그 token 자체를 revoke해야 한다.
  ```
- **Mirror**: 기존 표 구조 + 한국어 톤.
- **Validate**: 마크다운 렌더 확인 + `Select-String -Path docs/threat-model.md -Pattern "Browser sessions"`로 섹션 존재 확인.

#### Task 2.D7: dashboard 사용 문서 갱신
- **Action**: [docs/dashboard.md](../../docs/dashboard.md)에 "Sessions" 단락 추가. URL `/settings/sessions`, 표시되는 컬럼, revoke가 즉시 적용됨을 설명. CSRF 한계는 threat-model로 링크.
- **Validate**: 단락 존재 확인 + threat-model 링크 깨짐 없음.

### Phase 3 — Dogfood 실측 + Archive (사용자 측정 필요)

#### Task 3.1: dashboard-dogfood 문서 신규
- **Action**: `docs/dashboard-dogfood.md` 작성. 세 Flow를 명세로 정의:
  - **Flow A**: 새 envvar 추가 (project=api, env=local) — budget ≤ 30s / ≤ 12 clicks
  - **Flow B**: 잘못된 commit rollback (특정 version으로 되돌리기) — budget ≤ 30s / ≤ 8 clicks
  - **Flow C**: env-vs-env diff에서 "local에만 있는 key" 찾기 — budget ≤ 20s / ≤ 5 clicks
  - 각 Flow 아래 Record 테이블(clicks / wall-clock / audit row), Acceptance 체크리스트.
- **Validate**: 문서 존재 + Record placeholder 표시 명확(`<측정 후 채움>`).

#### Task 3.2: 측정용 임베드 빌드
- **Action**:
  ```powershell
  go build -tags embed_dashboard -trimpath -o bin/secret-server.exe ./cmd/server
  # 별도 셸에서:
  $env:COMAX_AUTO_GENERATE_KEY="1"; ./bin/secret-server.exe
  ```
- **Validate**: 브라우저에서 SPA + `/settings/sessions` 노출.

#### Task 3.3: Flow A/B/C 실측 (사용자 직접)
- **Action**: docs/dashboard-dogfood.md의 세 Flow를 dashboard-only로 수행. 클릭 수 / wall-clock / audit row를 측정해 Record 표 채움.
- **Validate**: 각 Flow가 budget 통과. 미달 시 cleanup 미완 판정 → craft 라운드 추가.

#### Task 3.4: PRD 마감 + plan archive
- **Action**:
  1. [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md)의 `Delivery Milestones` M2 행:
     - Plan 셀: `.claude/plans/completed/comax-secrets-dashboard.plan.md` (M2 본체) + `.claude/plans/completed/comax-secrets-dashboard-m2-cleanup.plan.md` (본 plan, cleanup)
     - 상태: `in-progress` → `done`
  2. M2 본체 plan ([comax-secrets-dashboard.plan.md](./comax-secrets-dashboard.plan.md)) → `.claude/plans/completed/`로 이동. 첫 줄에 `> Archived from .claude/plans/ on YYYY-MM-DD — paths re-rooted at completed/.`
  3. 본 cleanup plan도 같은 PR에서 `.claude/plans/completed/`로 이동. 첫 줄 archive 메타.
  4. 내부 상대 경로 깊이 +1 보정 (`../../internal/` → `../../../internal/` 등).
  5. README/PRD/reports에서 옛 경로 인용을 `completed/` 신경로로 갱신.
- **Validate**:
  ```powershell
  Select-String -Path README.md, .claude/prds/*.md, .claude/reports/*.md, .claude/plans/**/*.md `
                -Pattern "plans/comax-secrets-dashboard(\.plan)?\.md(?!.completed)"
  ```
  옛 경로 잔여 0건.

## Validation

```powershell
# Phase 1 — size gate
Get-Content .github/workflows/ci.yml | Select-String "Size budget gate"

# Phase 2 — Go side
go test -race -cover -coverpkg=./internal/server,./internal/store,./cmd/server ./...
go vet ./...
make lint   # or golangci-lint run --timeout=5m

# Phase 2 — Dashboard side
cd web/dashboard
pnpm typecheck
pnpm lint
pnpm test
pnpm build
pnpm exec playwright test sessions

# Phase 3 — 임베드 빌드 + 수동 측정
cd ..\..
go build -tags embed_dashboard -trimpath -o bin/secret-server.exe ./cmd/server

# 링크 무결성 (Phase 3 archive 후)
Select-String -Path README.md, .claude/prds/*.md, .claude/reports/*.md, .claude/plans/**/*.md `
              -Pattern "plans/comax-secrets-dashboard(\.plan)?\.md(?!.completed)"
```

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `ListByTokenID`에 `token_id` 인덱스가 없으면 full scan | Low | Low | schema.sql 확인 후 없으면 같은 PR에서 idempotent `CREATE INDEX IF NOT EXISTS`. session 수가 적어 영향은 미미하지만 옳은 일. |
| Prune sweeper가 SQLite 동시성과 충돌(writer lock 경합) | Low | Medium | Prune은 단일 DELETE. busy_timeout이 이미 설정돼 있을 것. `context.WithTimeout(ctx, 30*time.Second)`로 hang 방지. |
| Sweeper goroutine 누수(testing) | Low | Low | `runPruneSweeper`를 함수로 분리하고 ctx cancel 시 즉시 return 확인 단위 테스트. |
| Sessions UI가 anti-template (회색 카드 그리드) | Medium | Medium | Phase 5.0 impeccable design gate 호출 시 design signal 잡힘. Task 2.D4 마무리 시 impeccable polish 추가. |
| 위협 모델 신규 섹션이 marketing(M6) 카피와 충돌 | Low | Low | M6 마케팅은 별도 worktree. cross-link만 명확히. |
| Phase 3 실측 budget 미달 | Low-Medium | High | 미달 시 본 PR을 Phase 1+2 한정으로 머지, Phase 3는 별도 craft PR로 분리 옵션 보유. |
| CI size gate가 `bin/secret-server` 경로를 못 찾음 | Low | Medium | dashboard-e2e job의 `go build -o bin/secret-server` 줄이 이미 있음([ci.yml:181](../../.github/workflows/ci.yml#L181)). 동일 working-dir에서 size gate 실행. |
| Plan-Codex 1회 호출 시 D1~D5에서 상이(Divergent) | Medium | Medium | mccp Phase 5.4 severity-gated rerun으로 최대 cap 라운드. 합치 실패 시 Open Question으로 명시. |

## Acceptance

- [ ] Phase 1 (1.1 size gate + 1.2 dedupe 확인) 적용 + CI 통과.
- [ ] Phase 2 D1~D7 모두 적용. Go 커버리지 ≥ 80%. Playwright + axe 통과.
- [ ] `cmd/server`의 prune sweeper가 단위 테스트로 검증됨 (ctx cancel + 만료 row 삭제).
- [ ] `docs/threat-model.md`에 "Browser sessions" 섹션 존재 (Mitigation + Limit 양쪽).
- [ ] Plan-Codex 1회 호출 (Phase 5 자동) + 합치 또는 라운드 cap 도달. 본 plan 하단 `## Codex Adversarial Review` 섹션 채워짐.
- [ ] Phase 3 dashboard-dogfood Flow A/B/C 실측 완료. 모든 Record/Acceptance 채워짐.
- [ ] M2 본체 plan + 본 cleanup plan 모두 `.claude/plans/completed/`로 이동. PRD M2 행 `done`.
- [ ] 모든 incoming 링크가 `completed/` 신경로로 정확히 향함 (grep 잔여 0건).

## Next Steps (mccp 컨벤션)

1. **Phase 5 — Plan-Codex 게이트 (자동)** — `/mccp:plan` 진입 시 자동 실행됨. 본 plan에 placeholder 섹션이 곧 채워진다.
2. **Receipt 자동 기록** — Phase 5.6에서 `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/0.4.0/scripts/receipt/cli.js write --gate mccp-plan-codex` 자동 호출.
3. **`/mccp:prp-implement` 실행**:
   ```
   /mccp:prp-implement .claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md
   ```
   Phase 1 → Phase 2 D1~D7 → Phase 3 순서. Phase 3는 사용자 측정 단계에서 일시 중지.
4. **`/mccp:pr`** — Phase 2 종료 시점에 PR 옵션 평가. Phase 3 dogfood 실측 완료 후 최종 PR.

## Design Critique (impeccable, plan-shape)

> Pre-implementation critique. Code doesn't exist yet, so this is a shape critique
> against [PRODUCT.md](../../PRODUCT.md) / [DESIGN.md](../../DESIGN.md), not a
> visual audit. The implementer must absorb these decisions before writing Task 2.D4.

**Strengths the plan already gets right**
- Typographic hierarchy(device 굵게 → IP 보조 → last-seen 약하게) aligns with
  DESIGN.md 원칙 1 ("색은 의미에만, 신호는 굵기·표면·간격").
- Radix `AlertDialog`로 revoke 명시 confirm — 원칙 4 ("비가역 액션은 명시 confirm").
- 옛 cleanup plan의 "회색 카드 그리드 금지" 가드를 본 plan이 계승.

**Shape decisions the plan must commit to before Task 2.D4 (P1)**

1. **Sessions는 table, not cards.** GitHub Settings → Personal Access Tokens가
   가장 가까운 어휘(PRODUCT.md 핵심 레퍼런스). 카드 그리드 자리에 plan은 명시적으로
   `<table>` (또는 `role="table"` semantic grid)을 둔다. 컬럼: Device / IP / Created /
   Status / Actions.
2. **Current session 식별은 색이 아닌 표면 elevation + 라벨.** 운영자의 1차 질문은
   "어느 게 내 세션이지?". `--color-surface-elevated` + `--color-border-strong` +
   `font-weight 600` + "현재 세션" 텍스트 chip. 채도 액센트 금지(원칙 1).
3. **AlertDialog 카피는 honest warning 포함.** plan의 D4 위협 모델 "Limit" 단락과
   카피 의미가 일치해야 한다. 예시:
   > **세션 회수**
   >
   > 이 세션의 cookie는 더 이상 인증에 사용되지 않습니다. 다만 cookie가 이미 탈취된
   > 상태라면 그 시점까지의 read는 막을 수 없습니다. cookie 자체가 의심되면 이 token을
   > 통째로 revoke하세요.
   >
   > [회수] [취소]

   PRODUCT.md "정직함" + General rules "위험과 비가역성은 라벨에서 미리 보인다."
4. **device 라벨은 raw user_agent 금지, 파싱된 사람-가독 형태.** `Mozilla/5.0 (Windows NT
   10.0; ...)` 그대로 노출은 General rules "Recognition rather than recall" 위반. `ua-parser-js`
   또는 가벼운 자체 파서로 `Chrome on Windows` 수준의 라벨로 변환. 파싱 실패 시
   `Unknown device` fallback.

**Shape decisions the plan should commit to (P2)**

5. **last-seen 라벨의 정확한 의미를 plan에 명시.** master schema의 `dashboard_sessions`는
   `created_at` / `expires_at`만 있고 `last_used_at`이 없다. plan은 v1에서
   "Created" 라벨로 `created_at`을 표시하고, "Last activity" 컬럼은 M3 backlog로
   분리한다는 결정을 명시해야 한다. 아니면 사용자가 "이 세션이 마지막으로 활동한 시점"으로
   오해한다.
6. **Empty state는 1개 액션만.** 현재 세션 하나뿐인 정상 상태에서 빈 행이 아니라
   "다른 활성 세션이 없습니다" + 부가 안내 0줄. DESIGN.md 원칙 4.
7. **table은 mobile에서 stacked row로 붕괴.** 데스크탑 앱 포팅이 예정돼 있어 viewport
   100vw fluid는 피하되, 880px 이하 모바일 브라우저 접속 시 행이 stacked card로
   reflow되어야 한다. 가로 스크롤 금지.

**AI slop test**
- **first-order**: "Sessions 페이지 = 인디고 카드 그리드"는 SaaS reflex. plan이 명시적으로
  반대 선언했고 PRODUCT.md anti-reference("Doppler / Linear-copy SaaS 다크")에 부합.
- **second-order**: "AWS IAM 콘솔 stuck-in-2014" 회색 dense table은 그 반대 trap.
  GitHub PAT 화면의 절제된 monochrome table을 reference로 명시하면 양 reflex 모두 피한다.

**Implementer 액션 요약**
- Task 2.D4 시작 직전, 위 1~4번 결정(P1)을 commit해 코드로 반영.
- Task 2.D5 e2e에서 AlertDialog 카피의 honest warning 문구를 assertion으로 잠금.
- Task 2.D6 위협 모델 "Limit" 단락과 AlertDialog 카피 사이 cross-link을 docs에 추가.

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/0.4.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, mccp v0.4.0)
- 라운드 수: 1 (cap=1, R1에서 모든 finding을 plan 본문에 직접 absorb)
- 합치 결론: **needs-attention** verdict 수용. D3 router mismatch / D1 cross-token oracle / D2 prune grace / D5 raw vs gzip 4개 finding 모두 ACCEPT_NOW + plan 본문 정정으로 해결.
- YAGNI Triage:

  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | F1 D3 router mismatch | HIGH | ACCEPT_NOW | confidence 0.96. master는 code-based router(`src/router.tsx`의 `createRoute` + `rootRoute.addChildren`). plan대로 file-based 라우트 파일만 만들면 도달 불가. |
  | F2 D1 cross-token id oracle | MEDIUM | ACCEPT_NOW | confidence 0.88. 다른 token_id의 id 추측 시 403/404 분리는 존재 누출. SQL WHERE predicate에 token_id 원자적 결합 + 0 rows = 동일 응답. |
  | F3 D2 prune grace 오류 | MEDIUM | ACCEPT_NOW | confidence 0.84. cutoff를 `time.Now()`로 변경 — "1시간 주기 만료 정리" 약속과 일치. |
  | F4 D5 raw vs gzip 혼동 | MEDIUM | ACCEPT_NOW | confidence 0.92. plan과 스니펫 모두 **raw byte** 기준으로 통일(gz 표현 제거). |

- Deferred to backlog: 0
- Open Questions: 없음. 4개 finding 모두 R1에서 plan body 정정으로 해결.
- Codex session 참조: `019ec081-c2cc-74f0-b78b-cfefd12eabc4`
- 정정 반영 위치: Decisions(D1/D2/D3/D5), Files to Change(router.tsx), Tasks(2.D1/2.D2/2.D3/2.D4 + 1.1).
