# Plan: Comax Secrets — Milestone 2 Cleanup (Codex Adversarial Review 후속)

**Source PRD**: [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md)
**Source Reviews**:
- Claude 로컬 리뷰: [.claude/reviews/dashboard-ui-local-review-m2-cleanup.md](../reviews/dashboard-ui-local-review-m2-cleanup.md)
- Codex adversarial review: session `019e8677-15d6-7ce1-a5ce-468c9c5831f9`, verdict `needs-attention`, 4 high + 2 medium
- 이전 M2 plan: [.claude/plans/completed/comax-secrets-dashboard.plan.md](completed/comax-secrets-dashboard.plan.md)
**Selected Milestone**: #2 — Dashboard UI (Doppler 스타일) — **cleanup/hardening pass**
**Branch**: `feat/dashboard-ui`
**Complexity**: **Medium-Large** (선택지 D 채택 — Phase 1 인프라 정합 + Phase 2 D 세션 통제 완전 구현 + 위협 모델 정정 + Phase 3 dogfood 실측)
**Design 영향**: Phase 2 D의 Sessions 화면 1개에 한정 (Plan-Impeccable shape 필요)

## Summary

M2 본체(#7, #8 PR)는 머지됐지만 마무리 cleanup(이 worktree의 현재 staged 변경)에서 Codex가 6개 결함을 지적했다. 사용자 결정으로 **선택지 D — 세션 통제 완전 구현 + 위협 모델 정정**을 채택해 docs/threat-model.md가 약속한 "임의 세션 revoke + hourly prune"을 실제 코드로 구현하고, CSRF의 한계(read 보호 아님)와 cookie 탈취 시 read exfiltration 가능성을 위협 모델에 정직히 적는다. 동시에 pnpm 11 설정·Docker 공급망 보호 우회·CI size gate fail-closed·plan archive 링크 그래프·Task 15 실측까지 모두 해소한다.

성공은 binary: ① clean checkout pnpm install + `docker build` 통과, ② CI dashboard payload gate가 산출물 부재 시 명시적 실패, ③ 운영자가 Sessions 화면에서 임의 세션을 회수할 수 있고 prune이 1시간 주기로 동작함, ④ 위협 모델 문서가 구현과 일치, ⑤ docs/dogfood.md 세 Flow의 Record/Acceptance가 실측으로 채워짐, ⑥ 모든 incoming 링크가 `plans/completed/` 신경로를 가리킴.

## Decisions

선택지 D 채택으로 발생하는 **새 결정** 4건. Codex 합치 항목은 별도 표(아래 "Cross-gate dedupe")로 분리.

| # | Decision | Choice | Rationale |
|---|---|---|---|
| D1 | Sessions REST API shape | `GET /api/v1/dashboard/sessions` (현재 토큰의 subject가 소유한 모든 세션 행), `DELETE /api/v1/dashboard/sessions/{id}` (target session 회수). 두 endpoint 모두 cookie + CSRF 토큰 필요. | router.go의 기존 `POST /api/v1/dashboard/session` (current create) / `DELETE /api/v1/dashboard/session` (current logout) 짝을 깨지 않고 복수형 path로 분리. subject scope 강제로 다른 사용자의 세션을 노출/회수 불가. |
| D2 | Prune scheduler 위치·주기 | `internal/server/server.go`의 `Server.Run(ctx)` 내부에 `time.NewTicker(time.Hour)` 고루틴 추가. tick마다 `SessionRepo.Prune(ctx, time.Now().Add(-sessionTTL))` 호출. 세션 TTL 30일은 기존 plan 결정 유지. | M1 컨벤션(별도 daemon 만들지 않음). `ctx.Done()`으로 깨끗하게 종료. fake clock 없이 짧은 interval로 통합 테스트 가능. |
| D3 | Sessions UI 라우트·위치 | TanStack Router 라우트 `/settings/sessions` 신설. `web/dashboard/src/features/sessions/SessionsPage.tsx` + `SessionRow.tsx`. 헤더에서 "Sessions" 링크 노출. Radix `AlertDialog`로 revoke 확인. | 다른 feature와 동일한 `src/features/<name>/` 구조. `/settings/*` 그룹을 만들면 향후 운영자 설정 페이지가 합류 가능. |
| D4 | 위협 모델 정정 문구 | `docs/threat-model.md`의 세션 통제 약속을 **두 부분으로 분리**: (a) "운영자는 Sessions 페이지에서 임의 세션을 회수할 수 있다" (구현 완료 후 유지), (b) **신규 명시**: "CSRF 토큰은 mutation에만 요구되며 GET API는 cookie만으로 인증된다. cookie 단독 탈취는 read exfiltration에 충분하므로 cookie 보호(httpOnly + Secure + SameSite=Strict) 자체가 보안 경계다. revoke는 회수 수단이지 탈취 방지 수단이 아니다." | Codex 지적의 핵심. 보안 약속을 부풀리지 않고 정직히 표기. |

### Cross-gate dedupe (Codex 합치 항목, 재호출 불필요)

ECC Command Gates §5에 따라 다음 결정은 adversarial review에서 이미 합치되어 재검증하지 않는다. plan 본문에 한 줄로 기록한다.

- **pnpm 11 설정 단일 키 정합** — already converged in adversarial review (round 1). Phase 1.1에서 그대로 수용.
- **Docker `PNPM_CONFIG_DANGEROUSLY_ALLOW_ALL_BUILDS` 제거 + workspace yaml allowlist** — already converged in adversarial review (round 1). Phase 1.2.
- **CI size gate fail-closed** — already converged. Phase 1.3.
- **plan archive 내부 상대 경로 보정 + incoming 링크 갱신** — already converged. Phase 1.4.
- **Task 15 dogfood 실측 후에만 archive/`done` 이동** — already converged. Phase 3.

새 결정 D1~D4만 Phase 2 진입 직전에 Plan-Codex 1회 호출 대상이다 (본문 마지막 `## Plan-Codex Gate` 섹션 참고).

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| 라우터 등록 | [internal/server/router.go:23-24](../../internal/server/router.go#L23-L24) | D1 신규 라우트는 동일한 `mux.HandleFunc("METHOD /path/{param}", s.handlerXxx)` 스타일. chi/gorilla 도입 금지. |
| Repository pattern | [internal/store/session_repo.go](../../internal/store/session_repo.go) | D1 list endpoint를 위해 `SessionRepo.ListBySubject(ctx, subject)` 신규 메서드 추가. `New<Repo>(DBTX)` shape 유지. |
| Prune 호출 | [internal/store/session_repo_test.go:174](../../internal/store/session_repo_test.go#L174) | 기존 `Prune` 시그니처(`(ctx, olderThan time.Time)`) 그대로 사용. D2 ticker는 `time.Now().Add(-sessionTTL)`를 인수로. |
| audit append | [internal/server/handlers_projects.go:91-99](../../internal/server/handlers_projects.go#L91-L99) | D1 `DELETE /sessions/{id}`는 mutation transaction 안에서 `appendAudit(...)`. action `session.revoke_by_id`. metadata에 `target_session_id`(절대 token 본문 금지), `actor_session_id`, truncated remote IP. |
| Middleware 체인 | [internal/server/middleware.go:76-103](../../internal/server/middleware.go#L76-L103) | D1 신규 라우트는 `recover ← log ← auth ← mux` 체인 그대로 통과. CSRF 검사는 `DELETE`에 자동 적용(기존 middleware가 method 기반). 새 미들웨어 추가하지 않는다. |
| Background job | M1 컨벤션 (별도 daemon 없음) | D2 ticker는 `Server.Run(ctx context.Context) error` 내부에서 `errgroup` 또는 `go func()`로 띄우고 `<-ctx.Done()`에 종료. shutdown drain 보장. |
| 테스트 스타일 | [internal/server/server_test.go](../../internal/server/server_test.go), [internal/store/session_repo_test.go](../../internal/store/session_repo_test.go) | D1 신규 핸들러는 `httptest.Server` + 임시 SQLite + 테이블 드리븐. D2 ticker 테스트는 `time.NewTicker(10*time.Millisecond)` 주입 가능한 형태로 분리(또는 직접 `Prune` 단위 테스트로 분할). |
| Frontend 파일 layout | [web/dashboard/src/features/projects/](../../web/dashboard/src/features/projects/), [web/dashboard/src/features/secrets/](../../web/dashboard/src/features/secrets/) | D3 Sessions 화면을 `src/features/sessions/`로 신설. `SessionsPage.tsx`(라우트 엔트리), `SessionRow.tsx`(개별 행), `useSessions.ts`(TanStack Query 훅). |
| TanStack Router 라우트 | 기존 `web/dashboard/src/routes/`의 라우트 파일들 | `/settings/sessions` 라우트를 file-based로 추가. search params는 사용하지 않음(목록뿐). |
| Anti-template (Sessions UI) | ECC `web/design-quality.md` | Radix `AlertDialog` + 의도된 hover/focus 상태. 기본 회색 카드 그리드 금지. Sessions 행은 device/IP/last-seen을 typographic hierarchy로 구분. |
| docs 정정 톤 | [docs/threat-model.md](../../docs/threat-model.md) 기존 한국어 톤 | D4 정정은 "Mitigation"/"Limit" 소제목으로 구분해 mitigations vs honest limits를 가른다. |
| CI 단계 작성 | [.github/workflows/ci.yml:191-214](../../.github/workflows/ci.yml#L191-L214) | bash heredoc + `set -euo pipefail`. fail-closed 가드는 같은 step 안에 인라인. 새 step 만들지 않는다. |

## Files to Change

| File | Action | Why |
|---|---|---|
| `web/dashboard/pnpm-workspace.yaml` | UPDATE | placeholder 문자열 제거. Context7 검증된 pnpm 11.5 단일 키만 남김. |
| `deploy/docker/Dockerfile` | UPDATE | `PNPM_CONFIG_DANGEROUSLY_ALLOW_ALL_BUILDS` ENV 제거. `pnpm-workspace.yaml`을 `package.json` 옆에 함께 COPY. |
| `.github/workflows/ci.yml` | UPDATE | size gate에 DIST 존재 검사 + 최소 1 JS asset 매치 강제 + total payload gate. |
| `.claude/plans/completed/comax-secrets-dashboard.plan.md` | UPDATE | 내부 상대 경로 깊이 +1 보정. archive 이동 사실 첫 줄에 명시. |
| `README.md` | UPDATE | line 13 incoming 링크를 `plans/completed/` 신경로로. |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | line 133 `Delivery Milestones` 표 Plan 셀: 옛 경로 → 이 cleanup plan 경로. 본 plan 완료 후 다시 archive 경로로. |
| `.claude/reports/comax-secrets-dashboard-task-10-11.report.md` | UPDATE | line 3 incoming 링크 갱신. |
| `internal/store/session_repo.go` | UPDATE | `ListBySubject(ctx, subject string) ([]Session, error)` 신규. |
| `internal/store/session_repo_test.go` | UPDATE | `ListBySubject` 테이블 드리븐 테스트 추가. |
| `internal/server/handlers_sessions.go` | CREATE | `handleListDashboardSessions`, `handleRevokeDashboardSessionByID`. |
| `internal/server/handlers_sessions_test.go` | CREATE | 핸들러 테스트. 다른 subject의 세션 노출/회수 시도 차단 확인. |
| `internal/server/router.go` | UPDATE | `GET /api/v1/dashboard/sessions`, `DELETE /api/v1/dashboard/sessions/{id}` 라우트 등록. |
| `internal/server/server.go` | UPDATE | `Server.Run`에 prune ticker 고루틴 추가. 종료 시 drain. |
| `internal/server/server_test.go` (또는 별도 `prune_scheduler_test.go`) | CREATE/UPDATE | ticker 통합 테스트. 짧은 interval로 검증. |
| `web/dashboard/src/features/sessions/SessionsPage.tsx` | CREATE | 목록 + revoke 버튼. |
| `web/dashboard/src/features/sessions/SessionRow.tsx` | CREATE | 개별 행. device/IP/last-seen typographic hierarchy. |
| `web/dashboard/src/features/sessions/useSessions.ts` | CREATE | TanStack Query 훅 (목록 + revoke mutation). |
| `web/dashboard/src/routes/settings/sessions.tsx` | CREATE | TanStack Router 라우트 파일. |
| `web/dashboard/src/components/layout/Header.tsx` (또는 동등 위치) | UPDATE | "Sessions" 링크 노출. |
| `web/dashboard/src/lib/api.ts` (또는 등가 클라이언트) | UPDATE | sessions list/revoke 호출 추가. |
| `web/dashboard/tests/sessions.spec.ts` | CREATE | Playwright + axe gate. 다른 device 시뮬레이션으로 revoke 확인. |
| `docs/threat-model.md` | UPDATE | D4 정정 문구. 세션 통제 약속과 CSRF write-only 한계 명시. |
| `docs/dashboard.md` | UPDATE | Sessions 화면 사용법 단락 추가. |
| `docs/dogfood.md` | UPDATE | 세 Flow의 Record 블록 + Acceptance 5개 체크박스를 실측 결과로 채움 (Phase 3, **사용자 측정 필요**). |
| `.claude/design/dashboard-m2-sessions-page.design.plan.md` | CREATE | Plan-Impeccable shape 산출물 (Phase 2 D3 시작 직전). |

## Tasks

### Phase 1 — 즉시 수정 (Codex 합치 영역, ~30분)

#### Task 1.1: pnpm 워크스페이스 정합
- **Action**: Context7 또는 `mcp__plugin_ecc_context7__query-docs`로 pnpm 11.5 공식 키(`allowBuilds` vs `onlyBuiltDependencies`) 확인. `web/dashboard/pnpm-workspace.yaml`에서 placeholder 문자열(`set this to true or false`) 제거. 단일 키만 남기고 esbuild를 명시적으로 허용.
- **Mirror**: 없음 (단일 파일 정합).
- **Validate**:
  ```powershell
  cd web/dashboard
  Remove-Item -Recurse -Force node_modules, .pnpm-store -ErrorAction SilentlyContinue
  pnpm install --frozen-lockfile
  ```
  exit 0 + pnpm warning 0건 + esbuild postinstall 실행 로그 확인.

#### Task 1.2: Dockerfile allowlist 좁히기
- **Action**: [deploy/docker/Dockerfile:16-27](../../deploy/docker/Dockerfile#L16-L27)에서 `PNPM_CONFIG_DANGEROUSLY_ALLOW_ALL_BUILDS=true` 줄 삭제. `COPY web/dashboard/package.json web/dashboard/pnpm-lock.yaml ./` 라인을 `COPY web/dashboard/package.json web/dashboard/pnpm-lock.yaml web/dashboard/pnpm-workspace.yaml ./`로 변경.
- **Mirror**: 기존 multi-stage 구조 유지.
- **Validate**:
  ```powershell
  docker build -t comax-secrets:cleanup-test (Resolve-Path ../..)
  ```
  성공 + 빌드 로그에 esbuild 외 다른 postinstall 실행이 없음.

#### Task 1.3: CI size gate fail-closed
- **Action**: [.github/workflows/ci.yml:191-214](../../.github/workflows/ci.yml#L191-L214) bash 블록에서:
  1. `[ -d "$DIST" ] || { echo "::error::dashboard dist missing at $DIST"; exit 1; }` 추가
  2. JS/CSS 매치 카운터(`JS_COUNT`, `CSS_COUNT`) 도입, 최소 1개 JS 매치 강제
  3. `TOTAL_MAX=$((500 * 1024))` 도입, `JS_BYTES + CSS_BYTES > TOTAL_MAX` 시 실패
- **Mirror**: 기존 `set -euo pipefail` + heredoc 패턴.
- **Validate**: PR push 후 GitHub Actions에서 정상 빌드 통과 + (선택) 빈 dist 시뮬레이션으로 실패 확인.

#### Task 1.4: plan archive 링크 그래프 복원
- **Action**:
  1. [.claude/plans/completed/comax-secrets-dashboard.plan.md](completed/comax-secrets-dashboard.plan.md) 내부 모든 상대 경로 깊이 +1 보정 (`../prds/` → `../../prds/`, `../../internal/` → `../../../internal/` 등).
  2. archive 이동 사실을 plan 첫 줄에 `> Archived from .claude/plans/ on 2026-06-02 — paths re-rooted at completed/.` 형태로 명시.
  3. incoming 링크 갱신:
     - [README.md:13](../../README.md#L13) → `[`.claude/plans/completed/comax-secrets-dashboard.plan.md`](.claude/plans/completed/comax-secrets-dashboard.plan.md)`
     - [.claude/prds/comax-secrets.prd.md:133](../prds/comax-secrets.prd.md#L133) → Plan 셀을 **임시로** 본 cleanup plan(`../plans/comax-secrets-dashboard-m2-cleanup.plan.md`)으로. 본 plan 완료 시 Task 3.6에서 다시 archive 경로로.
     - [.claude/reports/comax-secrets-dashboard-task-10-11.report.md:3](../reports/comax-secrets-dashboard-task-10-11.report.md#L3) → `../plans/completed/comax-secrets-dashboard.plan.md`
  4. (옵션) `.github/workflows/ci.yml`에 markdown link check step 추가 — 단순 grep으로도 가능.
- **Mirror**: 없음.
- **Validate**:
  ```powershell
  Select-String -Path README.md, .claude/prds/*.md, .claude/reports/*.md, .claude/plans/completed/*.md -Pattern "plans/comax-secrets-dashboard\.plan\.md"
  ```
  옛 경로 잔여 0건 (cleanup plan 자체의 인용 라인 제외).

### Phase 2 — 선택지 D: 세션 통제 완전 구현 + 위협 모델 정정 (~6-10시간)

#### Task 2.D1: Plan-Impeccable shape (Sessions 화면)
- **Action**: `Skill(impeccable, "shape Sessions page — operator can list and revoke their other dashboard sessions")` 호출. 결과를 `.claude/design/dashboard-m2-sessions-page.design.plan.md`에 저장. 본 plan 본문 상단에 `> Design: [dashboard-m2-sessions-page.design.plan.md](../design/dashboard-m2-sessions-page.design.plan.md)` 링크 삽입.
- **Mirror**: ECC `web/design-quality.md` Sessions 행은 device/IP/last-seen typographic hierarchy. Radix `AlertDialog`로 revoke 확인. 회색 카드 그리드 금지.
- **Validate**: 디자인 plan 파일 존재 + 본 plan 본문 링크 살아있음 + impeccable 결과의 anti-SLOP 체크 통과.

#### Task 2.D2: SessionRepo.ListBySubject 추가
- **Action**: [internal/store/session_repo.go](../../internal/store/session_repo.go)에 `ListBySubject(ctx context.Context, subject string) ([]Session, error)` 추가. 만료 안 된 row만 반환(`expires_at > now`). subject 인덱스 활용.
- **Mirror**: 기존 repo 패턴(`SessionRepo` 메서드 시그니처).
- **Validate**:
  ```powershell
  go test -race -run TestSessionRepo_ListBySubject ./internal/store
  ```
  통과 + 다른 subject row가 결과에 포함 안 됨을 명시적으로 검증.

#### Task 2.D3: Sessions 핸들러 + 라우터
- **Action**:
  1. `internal/server/handlers_sessions.go` 생성. `handleListDashboardSessions`(현재 토큰 subject scope), `handleRevokeDashboardSessionByID`(target id의 subject가 actor subject와 일치하는 경우에만 회수).
  2. 라우트 등록: [router.go:23-24](../../internal/server/router.go#L23-L24) 아래에 `GET /api/v1/dashboard/sessions`, `DELETE /api/v1/dashboard/sessions/{id}` 추가.
  3. revoke 시 `appendAudit(...)`로 `session.revoke_by_id` action 기록. metadata에 `target_session_id`, `actor_session_id`, truncated remote IP. **token 본문 절대 기록 금지**.
  4. `internal/server/handlers_sessions_test.go`: 다른 subject 시도 → 403, 자기 자신 → 200/204, 존재하지 않는 id → 404.
- **Mirror**: [handlers_projects.go:91-99](../../internal/server/handlers_projects.go#L91-L99) audit 패턴, 기존 middleware 체인.
- **Validate**:
  ```powershell
  go test -race -cover -coverpkg=./internal/server,./internal/store -run "TestSessions" ./internal/server
  ```
  커버리지 ≥ 80%.

#### Task 2.D4: Prune scheduler
- **Action**: [internal/server/server.go](../../internal/server/server.go)의 `Server.Run(ctx)`에 ticker 고루틴 추가. `time.NewTicker(time.Hour)`. tick마다 `SessionRepo.Prune(ctx, time.Now().Add(-sessionTTL))`. `<-ctx.Done()` 시 ticker.Stop()하고 빠져나옴. shutdown drain은 기존 `Server.Run` 종료 흐름이 처리.
- **Mirror**: M1 컨벤션 (별도 daemon 없음).
- **Validate**:
  ```powershell
  go test -race -run "TestServer_PruneScheduler" ./internal/server
  ```
  10ms ticker 주입 테스트로 N tick 후 만료 row가 삭제됨을 확인.

#### Task 2.D5: Sessions UI 라우트 + 페이지
- **Action**:
  1. TanStack Router 라우트 파일 `web/dashboard/src/routes/settings/sessions.tsx` 생성. lazy loader.
  2. `src/features/sessions/SessionsPage.tsx` + `SessionRow.tsx` + `useSessions.ts`. shape plan(`dashboard-m2-sessions-page.design.plan.md`) 결정을 그대로 따른다.
  3. Header에 "Sessions" 링크 추가.
  4. `Skill(impeccable, "polish src/features/sessions and the SessionsPage route per the shape plan")` 호출로 마크업/스타일/모션 polish.
- **Mirror**: 다른 feature(`projects`, `secrets`)의 src/features 구조.
- **Validate**:
  ```powershell
  cd web/dashboard
  pnpm test -- sessions
  pnpm build
  ```
  + 브라우저 수동 확인 + axe (Task 2.D6에서 자동화).

#### Task 2.D6: e2e + axe gate
- **Action**: `web/dashboard/tests/sessions.spec.ts` 생성. ① 두 번째 세션을 별도 BrowserContext로 생성, ② 첫 번째 세션에서 Sessions 화면 진입, ③ 두 번째 세션을 revoke, ④ 두 번째 BrowserContext에서 401 확인. axe gate는 page 진입 시 자동.
- **Mirror**: 기존 `dashboard-e2e` job의 Playwright 패턴.
- **Validate**:
  ```powershell
  cd web/dashboard
  pnpm exec playwright test sessions
  ```
  통과 + axe violation 0.

#### Task 2.D7: 위협 모델 정정
- **Action**: [docs/threat-model.md:76-100](../../docs/threat-model.md#L76-L100) 갱신. 다음 두 단락을 추가:
  - "**Mitigation (구현됨)**: 운영자는 `/settings/sessions`에서 자신의 모든 세션을 보고 임의 세션을 회수할 수 있다. 만료된 세션 row는 `Server.Run`의 1시간 주기 `SessionRepo.Prune`이 정리한다."
  - "**Limit (정직히 표기)**: CSRF 토큰은 mutation에만 요구되며 GET API는 cookie 단독으로 인증된다. cookie 단독 탈취는 read exfiltration에 충분하다. revoke는 회수 수단이지 탈취 방지 수단이 아니다. cookie 보호(httpOnly + Secure + SameSite=Strict) 자체가 보안 경계다."
- **Mirror**: docs/threat-model.md 한국어 톤.
- **Validate**: 문서 본문에 두 단락이 명시적으로 존재. `Mitigation` / `Limit` 소제목으로 구분.

### Phase 3 — Task 15 dogfood 실측 + archive 확정 (사용자 측정 필요)

#### Task 3.1: 임베드 빌드 + 서버 기동
- **Action**: `make build BUILD_TAGS=embed_dashboard` → 임시 master key 생성 → `secret-server` 실행.
- **Validate**: `/healthz` 200 + 브라우저에서 SPA + Sessions 라우트 노출.

#### Task 3.2~3.4: Flow A/B/C 실측
- **Action**: docs/dogfood.md:124-186의 세 Flow를 dashboard-only로 수행하며 클릭 수/wall-clock/audit 행을 측정.
- **Validate**: 각 Flow가 명시된 budget(≤30s/≤12 clicks, ≤30s/≤8 clicks, ≤20s/≤5 clicks)을 통과.

#### Task 3.5: Acceptance 갱신
- **Action**: docs/dogfood.md:188-204의 5개 체크박스를 실측 결과로 체크. 세 Record 블록(line 138-142, 162-165, 183-186)을 placeholder에서 실측 값으로 교체.
- **Validate**: 체크박스 5개 모두 `[x]` + Record placeholder(`…`) 잔여 0.

#### Task 3.6: archive 이동 + PRD 표 갱신
- **Action**:
  1. PRD `Delivery Milestones` M2 행: Plan 셀을 다시 `../plans/completed/comax-secrets-dashboard.plan.md`로, 상태 `in-progress` → `done`.
  2. 본 cleanup plan(`.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md`)을 `.claude/plans/completed/`로 이동하고 본 plan 첫 줄에 archive 메타 한 줄 추가.
  3. (선택) PRD 표의 Plan 셀에 cleanup plan archive 경로도 함께 기록.
- **Validate**: 모든 incoming 링크가 `plans/completed/` 신경로로 정확히 향함. `grep -r "plans/comax-secrets-dashboard"`로 잔여 옛 경로 0건 확인.

## Validation

```powershell
# Phase 1 — 즉시 수정
cd web/dashboard
Remove-Item -Recurse -Force node_modules, .pnpm-store -ErrorAction SilentlyContinue
pnpm install --frozen-lockfile
docker build -t comax-secrets:cleanup-test (Resolve-Path ../..)

# Phase 2 — Go side
go test -race -cover -coverpkg=./internal/server,./internal/store ./...
go vet ./...
make lint

# Phase 2 — Dashboard side
cd web/dashboard
pnpm test
pnpm build
pnpm exec playwright test sessions

# Phase 3 — 빌드 + 수동 측정
cd ../..
make build
./bin/secret-server  # 별도 셸에서 브라우저로 측정

# 링크 무결성 (Phase 1.4 + 3.6)
Select-String -Path README.md, .claude/prds/*.md, .claude/reports/*.md, .claude/plans/**/*.md `
              -Pattern "plans/comax-secrets-dashboard\.plan\.md(?!.completed)"
```

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Context7로 확인한 pnpm 키 결과가 dual-key를 모두 인정하면 단일 키 정합 결정의 근거가 약해진다 | Medium | Low | 그 경우에도 placeholder 문자열은 무조건 제거. dual-support면 `onlyBuiltDependencies`(array)를 유지하고 `allowBuilds` 블록 제거. |
| Dockerfile에서 ENV 제거 후 esbuild 외 다른 transitive postinstall이 정말로 필요한 dep 발견 | Low | Medium | `pnpm rebuild --dry-run`으로 사전 확인. 필요 시 `pnpm-workspace.yaml`에 명시적으로 추가. |
| D2 prune ticker가 SQLite 동시성 이슈로 transaction 충돌 | Low | Medium | Prune은 단일 DELETE 쿼리이므로 충돌 가능성 낮음. 그래도 `context.WithTimeout(ctx, 30*time.Second)`로 감싸 hang 방지. |
| D3/D5 Sessions UI가 anti-template policy를 위반(기본 카드 그리드 등) | Medium | Medium | Task 2.D1 Plan-Impeccable + Task 2.D5 polish 호출 강제. shape plan 결정을 코드 리뷰 시 대조. |
| D7 위협 모델 정정이 기존 PRD/marketing 문구와 충돌 | Low | Low | M2 시점 docs/threat-model.md만 갱신. marketing(M6) 카피는 별도 작업으로 분리. |
| Phase 3 실측 budget 미달 (≤30s, ≤12 clicks 등) | Low-Medium | High | 사전에 docs/perf.md 가이드 적용. budget 미달 시 M2 미완 판정 → 추가 craft 라운드 필요. PR을 Phase 2까지로 끊고 Phase 3는 별도 PR로 분리 옵션도 보유. |
| Plan-Codex가 D1~D4 새 결정에서 상이(Divergent) 응답 | Medium | Medium | 본 plan 본문 `## Plan-Codex Gate`에서 3 라운드 한계 내 해소. 합치 실패 시 Open Question으로 명시하고 사용자 판단 요청. |
| `/ecc:prp-implement`가 Sessions UI 작업에서 impeccable polish를 건너뛸 위험 | Low | Medium | 본 plan의 Files to Change 표가 디자인 plan 파일을 명시적으로 요구. prp-implement 진입 전 본 plan + design plan 두 파일을 모두 입력에 포함. |

## Acceptance

- [ ] Phase 1 (Task 1.1~1.4) 모두 적용 및 validation 통과.
- [ ] Phase 2 D1 design plan 존재 + 본 plan에 링크.
- [ ] Phase 2 D2~D7 모두 적용. Go 커버리지 ≥ 80%. Playwright + axe 통과.
- [ ] Plan-Codex 1회 호출 + 합치 또는 3R 도달. `## Codex Adversarial Review (Plan-side)` 섹션이 본 plan 하단에 추가됨.
- [ ] Phase 3.1~3.5 사용자 실측 완료. Record/Acceptance 모두 채워짐.
- [ ] Phase 3.6 PRD 표 갱신 + cleanup plan archive 이동.
- [ ] 모든 incoming 링크가 정확히 향함 (grep 잔여 0건).

## Plan-Codex Gate (다음 단계)

본 plan은 Codex adversarial review(`019e8677-15d6-7ce1-a5ce-468c9c5831f9`)의 6개 finding 중 5개를 그대로 수용(`Cross-gate dedupe` 표 참고)하고, 선택지 D 채택으로 **4개 새 결정(D1~D4)**을 추가한다. 이 새 결정에 대해 ECC Command Gates §2 Plan-Codex 게이트가 다음 호출을 요구한다:

```
/codex:adversarial-review challenge whether the new sessions REST API shape (D1),
prune scheduler ticker placement (D2), Sessions UI route under /settings/sessions (D3),
and threat-model rewrite with explicit CSRF write-only limit (D4) are the right approach.
Reuse cross-gate dedupe for findings already converged in the previous adversarial review.
```

호출 결과를 합치(Converged) / 상이(Divergent)로 분류하고, 본 plan 하단에 `## Codex Adversarial Review (Plan-side)` 섹션을 추가한다(라운드 수 / 합치 결론 / 수용·거부 + 근거 / Open Questions / session 참조).

이 게이트가 끝난 후에만 `/ecc:prp-implement`를 실행한다.

## Codex Adversarial Review (Plan-side)

<!-- placeholder: filled by ECC Autonomy Contract Phase 7.3 -->

## Next Steps

1. **Plan-Codex 호출** — 위 `## Plan-Codex Gate` 명령 실행. 합치까지 또는 3R 도달.
2. **Receipt 기록** — ECC Command Gates §1.5에 따라:
   ```powershell
   node ~/.claude/scripts/ecc-receipt/cli.js write `
     --gate plan-codex `
     --decision m2-cleanup-d `
     --plan .claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md
   ```
3. **/ecc:prp-implement 실행** — 본 plan 경로를 인수로:
   ```
   /ecc:prp-implement .claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md
   ```
   prp-implement는 Phase 1 → Phase 2 D1(shape) → 2.D2~2.D7 → Phase 3 순서로 진행. Phase 3는 사용자 측정 단계에서 일시 중지하고 사용자 입력 대기.
4. **/ecc:pr 또는 /ecc:prp-pr** — Phase 2 종료 시점에 PR 옵션을 평가. Phase 3 dogfood 실측 완료 후 최종 PR.
