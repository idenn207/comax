> Archived from .claude/plans/ on 2026-06-15 — paths re-rooted at completed/.

# Plan: Comax Secrets — M2 Closure (mccp 게이트 기반 재검토)

**Source PRD**: [.claude/prds/comax-secrets.prd.md](../../prds/comax-secrets.prd.md)
**Selected Milestone**: #2 — Dashboard UI 마감 (`in-progress` → `done`)
**Complexity**: Small (정합성 회복 + 게이트 재검토. 게이트가 신규 finding을 내면 그 범위만 task 추가)
**Plugin**: mccp v0.4.0
**선행 plan**:
- 본체: [comax-secrets-dashboard.plan.md](./comax-secrets-dashboard.plan.md) (이미 archive)
- cleanup: [comax-secrets-dashboard-m2-cleanup.plan.md](./comax-secrets-dashboard-m2-cleanup.plan.md) (active — 본 plan에서 archive)

## Summary

M2 본체 + cleanup(PR #9 + Codex Round 2 정정 `5a4ba45`) 모두 master에 머지됐고 작업트리도 깨끗하다. 그러나 PRD M2 행이 여전히 `in-progress` 상태로 남아 있고, Plan 셀이 `comax-secrets-dashboard.plan.md`를 가리키는데 그 파일은 `completed/`로 이동돼 **incoming 링크가 깨진 상태**다. cleanup plan(`comax-secrets-dashboard-m2-cleanup.plan.md`)도 active 디렉터리에 남아 있어 PRD가 신뢰할 수 없는 인덱스가 됐다.

본 plan은 두 가지를 한다: (1) PRD-plan-archive 정합성 회복, (2) **옛 dogfood live 측정 + impeccable live 자리를 mccp 게이트로 대체**해 dashboard를 재검토. `/mccp:plan` 진입 시 Phase 5.0 impeccable 게이트가 design critique을 자동 흡수하고, Phase 5 plan-codex 게이트가 adversarial review를 자동 흡수한다. 게이트가 신규 HIGH/CRITICAL finding을 발견하면 본 plan의 Tasks 절에 task로 흡수하고, 아니면 정합성 회복 단일 PR로 M2를 마감하고 M3로 넘어간다.

성공 기준: ① PRD M2 행 상태 `done` + Plan 셀이 `completed/` 신경로, ② cleanup plan + 본 plan 모두 `completed/`로 이동 + incoming 링크 grep 잔여 0건, ③ Phase 5.0 / 5.5의 finding이 모두 흡수됐거나 백로그로 분리됨, ④ M3(GitHub Actions integration) plan이 별도 `/mccp:plan` 세션에서 시작 가능한 상태.

> M2는 mccp 게이트(impeccable plan-shape + codex adversarial)와 **Task 2.6의 최소 live smoke 1회로 closure**를 검증하며, Flow A/B/C 정량 측정(`docs/dashboard-dogfood.md`)은 운영 단계의 별개 trigger로 분리된다.

## master 사실 검증

| 항목 | Master 실제 상태 | 본 plan에서의 처리 |
|---|---|---|
| Sessions UI 라우트 | [web/dashboard/src/router.tsx:121-126,160](../../../web/dashboard/src/router.tsx#L121-L126) — code-based router에 `settingsSessionsRoute` 등록 | **검증됨** — task 없음 |
| Sessions 페이지 + 행 | [web/dashboard/src/pages/Sessions.tsx](../../../web/dashboard/src/pages/Sessions.tsx), [web/dashboard/src/components/SessionRow.tsx](../../../web/dashboard/src/components/SessionRow.tsx) | **검증됨** — design critique은 Phase 5.0이 수행 |
| threat-model "Browser sessions" 섹션 | [docs/threat-model.md:69-126](../../../docs/threat-model.md#L69-L126) — Mitigation 5개 + Honest limits 4개 한국어 | **검증됨** |
| CI size gate | [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) size gate step + 1.5MB 정식화 (Codex R2 흡수, 커밋 `5a4ba45`) | **검증됨** |
| Prune sweeper | `cmd/server/main.go`의 `runPruneSweeper` 별도 함수 + 단위 테스트 | **검증됨** |
| `docs/dashboard-dogfood.md` | **없음** — cleanup plan Phase 3.1 미수행 | **분리 결정** — 본 plan에서는 만들지 않음 (D1) |
| PRD M2 Plan 셀 | `.claude/plans/comax-secrets-dashboard.plan.md` — 그러나 파일은 `completed/`로 이동 → **깨진 링크** | **정정** — Task 1.1 |
| cleanup plan 위치 | `.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md` — active 디렉터리 | **정정** — Task 1.2 (`completed/`로 이동) |

## Decisions

| # | Decision | Choice | Rationale |
|---|---|---|---|
| **D1** | dogfood 실측 vs M2 done (Codex F1 흡수 후 재정의) | **최소 live smoke + dogfood placeholder.** 옛 결정("만들지 않고 미루기")은 reject. `docs/dashboard-dogfood.md` 신규 작성(Flow A/B/C 명세 + Record placeholder) + Task 2.6 신설(최소 live smoke 1회): `bin/secret-server.exe` 임베드 빌드 → 로그인 → `/settings/sessions` 진입(`SessionsPage` 확인) → 자기 세션 보임 확인 → audit log 1행 확인. budget(분 단위 시간) 기록만, 정량 클릭/초 budget은 안 잠금. Flow A/B/C 정량 측정은 운영 trigger로 분리. **명시적 충돌 처리 (Codex stop-time review 흡수, 2026-06-15)**: M2 본체 plan [Task 15 acceptance](./comax-secrets-dashboard.plan.md#L259) (`three flows ≤ 30s logged in docs/dogfood.md`)는 본 D1에 의해 **미충족 상태로 deferred** 처리되어 M2 `done` 선언. 본체 plan acceptance line은 `[~]` 마커 + DEFERRED footer로 잠금, backlog에 별 항목으로 기록. | Codex F1 (HIGH, 0.84): mccp 게이트는 routing/auth/API wiring/timeout/env propagation을 실측하지 못함. 최소 live smoke 1회는 5-10분 비용으로 회귀 게이트의 90%를 잠근다. Flow A/B/C 정량 budget(클릭/초)은 PRD Success Metrics가 본인 운영 telemetry 기반이라 별 trigger로 OK. |
| **D2** | cleanup plan archive 시점 | **본 plan과 같은 PR에서.** 본 plan 머지 PR이 cleanup plan을 `completed/`로 이동 + 본 plan 자체도 `completed/`로 이동 + PRD M2 행 `done` 갱신. | 옛 cleanup plan Phase 3.4가 의도했던 일회성 PR 패턴. 본 plan으로 그 의도를 이어받는다. |
| **D3** | 게이트 finding 처리 방식 | **본 plan에 task로 흡수.** Phase 5.0 impeccable이 design HIGH 결함을 내거나 Phase 5의 codex가 HIGH/CRITICAL finding을 내면, 본 plan의 "Tasks Phase 2"에 task로 추가하고 같은 PR에 포함. MEDIUM/LOW는 `.claude/plans/codex-findings-backlog.md`로 분리. CRITICAL은 Phase 5.5 stop 트리거로 사용자 결정 요청. | mccp Phase 5.4 severity-gated rerun 규칙. cap=1 안에서 끝낸다. |

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| Plan archive 이동 | [.claude/plans/completed/comax-secrets.plan.md](./comax-secrets.plan.md), [.claude/plans/completed/comax-secrets-dashboard.plan.md](./comax-secrets-dashboard.plan.md) | 첫 줄에 `> Archived from .claude/plans/ on YYYY-MM-DD — paths re-rooted at completed/.` 한 줄 추가. 내부 상대 경로(`../../../internal/...`)는 +1 깊이로 보정. |
| PRD Delivery Milestones 갱신 | [.claude/prds/comax-secrets.prd.md:132](../../prds/comax-secrets.prd.md#L132) (M1 행) | Plan 셀에 `[plan](./<name>.plan.md)` + report 링크 가능. 상태 셀 `done`. |
| Report 작성 | [.claude/reports/comax-secrets-report.md](../../reports/comax-secrets-report.md), [.claude/reports/comax-secrets-dashboard-task-10-11.report.md](../../reports/comax-secrets-dashboard-task-10-11.report.md) | 동일 톤·구조 한국어. "Shipped / Not shipped / Gate review / Next" 절. |
| Incoming 링크 grep | cleanup plan Task 3.4의 `Select-String` 패턴 | 옛 plan 경로를 master 전체에서 검색. 결과 0건이 acceptance. |

## Files to Change

| File | Action | Why |
|---|---|---|
| `.claude/prds/comax-secrets.prd.md` | UPDATE | Delivery Milestones M2 행: 상태 `in-progress` → `done`. Plan 셀에 `[plan](./comax-secrets-dashboard.plan.md) · [cleanup](./comax-secrets-dashboard-m2-cleanup.plan.md) · [closure](./comax-secrets-dashboard-m2-close.plan.md) · [report](../../reports/comax-secrets-dashboard-m2.report.md)`. |
| `.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md` | MOVE → `.claude/plans/completed/` | active 디렉터리에 남아 있던 cleanup plan을 archive. 첫 줄 archive 메타 추가. 내부 상대 경로 깊이 +1 보정. |
| `.claude/plans/comax-secrets-dashboard-m2-close.plan.md` (본 plan) | MOVE → `.claude/plans/completed/` | 같은 PR에서 본 plan도 archive. |
| `.claude/reports/comax-secrets-dashboard-m2.report.md` | CREATE | M2 closure report. 본체 + cleanup + closure 3개 작업 결과 + 게이트 critique 요약 + 최소 live smoke 1회 결과 + M3 hand-off. **Codex F3 absorb**: gate run identifiers(impeccable slug + Codex session `019ec6da-e2b1-7371-aac7-e4de92287094`) + finding disposition + receipt path inline. |
| `docs/dashboard-dogfood.md` | CREATE | Flow A/B/C 명세 + Record placeholder 표. Task 2.6에서 최소 live smoke 1회 결과(한 줄)만 채우고, 정량 Flow A/B/C 클릭/초 budget은 운영 trigger로 분리. **Codex F1 absorb**. |
| `README.md` | (확인 + 필요 시 UPDATE) | 본문에 옛 plan 경로가 인용돼 있는지 grep. 있으면 `completed/` 신경로로 정정. (없으면 변경 0.) |

## Tasks

### Phase 1 — PRD/archive 정합성 회복 (~30분)

#### Task 1.1: PRD M2 행 갱신
- **Action**: [.claude/prds/comax-secrets.prd.md:133](../../prds/comax-secrets.prd.md#L133)의 M2 행:
  - 상태: `in-progress` → `done`
  - Plan 셀: 다음 4개 링크로 갱신:
    ```
    [plan](./comax-secrets-dashboard.plan.md) ·
    [cleanup](./comax-secrets-dashboard-m2-cleanup.plan.md) ·
    [closure](./comax-secrets-dashboard-m2-close.plan.md) ·
    [report](../../reports/comax-secrets-dashboard-m2.report.md)
    ```
- **Mirror**: M1 행 ([.claude/prds/comax-secrets.prd.md:132](../../prds/comax-secrets.prd.md#L132)).
- **Validate**: PRD read 시 모든 링크 파일 존재해야 한다. closure plan과 report는 본 phase 안에서 생성됨.

#### Task 1.2: cleanup plan archive
- **Action**: `.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md` → `.claude/plans/completed/comax-secrets-dashboard-m2-cleanup.plan.md` 이동. 첫 줄 추가:
  ```
  > Archived from .claude/plans/ on YYYY-MM-DD — paths re-rooted at completed/.
  ```
  내부 상대 경로 깊이 +1 보정: `../../../internal/` → `../../../../internal/`, `../../prds/` → `../../../../prds/`, `../../../web/` → `../../../../web/`, `../../../.github/` → `../../../../.github/`, `./comax-secrets-dashboard.plan.md` → `./comax-secrets-dashboard.plan.md` (같은 `completed/` 내부이므로 유지).
- **Mirror**: 기존 [.claude/plans/completed/comax-secrets-dashboard.plan.md](./comax-secrets-dashboard.plan.md) 첫 줄 헤더 + 깊이 보정.
- **Validate**: archive 위치에서 plan 본문의 모든 상대 링크가 파일 시스템에서 resolve.

#### Task 1.3: 본 plan archive 자체
- **Action**: `.claude/plans/comax-secrets-dashboard-m2-close.plan.md` → `.claude/plans/completed/`로 이동. 첫 줄 archive 메타.
- **Validate**: PRD M2 Plan 셀의 closure 링크가 valid.

#### Task 1.4: 깨진 incoming 링크 grep
- **Action**:
  ```powershell
  Select-String -Path README.md, .claude\prds\*.md, .claude\reports\*.md, .claude\plans\**\*.md `
                -Pattern "plans/(?!completed/)comax-secrets-dashboard(\.plan)?\.md"
  ```
  결과 0건이거나, 0건이 아니면 모두 `completed/` 신경로로 정정.
- **Validate**: 위 명령 결과 0건.

### Phase 2 — 게이트 흡수 + M2 closure report 작성 (~30분)

#### Task 2.1: 게이트 finding 본 plan으로 흡수
- **Action**: Phase 5.0 impeccable 게이트와 Phase 5 codex 게이트가 종료되면 본 plan의 `## Design Critique` / `## Codex Adversarial Review` 섹션에 채워진 내용을 review해, HIGH/CRITICAL 항목이 있으면 본 plan의 Tasks Phase 2에 Task 2.x로 추가. (현재로서는 미지수 — 게이트 실행 후 확정.)
- **Validate**: Phase 5.5 auto-CRITICAL 체크가 stop을 트리거하지 않음. trigger되면 사용자 결정 요청.

#### Task 2.2: M2 closure report 작성
- **Action**: `.claude/reports/comax-secrets-dashboard-m2.report.md` 신규. 다음 구조:
  ````markdown
  # Report: Comax Secrets — M2 (Dashboard UI) closure

  **PRD**: [.claude/prds/comax-secrets.prd.md](../../prds/comax-secrets.prd.md)
  **Plans**: 본체 / cleanup / closure (3개, `completed/` 인덱스)
  **Status**: done (YYYY-MM-DD)

  ## Shipped
  - **M2 본체** (PR #3, #5, #6, #7, #8) — dashboard SPA shell, 로그인/세션 라이프사이클, 프로젝트/환경/시크릿 CRUD, 버전 타임라인·롤백, env-vs-env diff, 감사 로그 피드, env_count API, craft polish (Task 12).
  - **M2 cleanup** (PR #9, Codex Round 2 흡수 `5a4ba45` 포함) — Sessions 통제 UI(`/settings/sessions`), 위협 모델 "Browser sessions" 섹션, prune sweeper(`cmd/server`), CI size gate (raw bytes, dashboard payload 1.5MB / server binary 30MB).
  - **M2 closure** (본 PR) — PRD M2 행 마감 + cleanup·closure plan archive + mccp 게이트 재검토.

  ## Gate review (옛 dogfood live 측정 대체)
  - **Phase 5.0 impeccable critique**: <게이트 결과 요약 1-2줄>
  - **Phase 5 plan-codex adversarial review**: <결과 요약 1-2줄, finding 수 / 흡수 vs 백로그>

  ## Not shipped (의도된 분리)
  - `docs/dashboard-dogfood.md` Flow A/B/C 측정 — 운영 단계의 별개 trigger로 미룸. PRD Success Metrics ("신규 envvar 1건 추가 → 전 환경 반영 시간", "Local↔dev↔prod 누락 0건/월") 측정은 본인 운영 로그 기반이 본질적이고, dashboard click count는 보조 지표라 M2 acceptance에서 빠뜨려도 무방. 측정이 필요해지는 시점에 별도 trigger.

  ## Next
  - M3 — GitHub Actions integration. 별도 `/mccp:plan .claude/prds/comax-secrets.prd.md ...` 세션으로 시작.
  ````
- **Validate**: report 파일 존재 + PRD M2 Plan 셀의 report 링크 resolve.

#### Task 2.3: Summary 정직성 한 줄 추가 (Phase 5.0 P1#1 흡수)
- **Action**: 본 plan 파일의 Summary 마지막에 다음 한 줄 추가:
  > "M2는 mccp 게이트(impeccable plan-shape + codex adversarial)와 **Task 2.6의 최소 live smoke 1회**로 closure를 검증하며, Flow A/B/C 정량 측정(`docs/dashboard-dogfood.md`)은 운영 단계의 별개 trigger로 분리된다."
- **Validate**: `Select-String -Path .claude\plans\comax-secrets-dashboard-m2-close.plan.md -Pattern "최소 live smoke 1회로 closure"` 매치 1건.

#### Task 2.4: Scope 인용문 위치 보존 (Phase 5.0 P1#2 흡수)
- **Action**: 본 plan `## Design Critique` 섹션 직전의 Scope 인용 블록(`> **Scope**: closure plan의 판단 ...`)이 archive 후에도 유지되는지 grep 검증. 변경 없음 — 이미 추가됨, archive 시 보존만 확인.
- **Validate**: archive 후 `Select-String -Path .claude\plans\completed\comax-secrets-dashboard-m2-close.plan.md -Pattern "closure plan의 판단"` 매치 1건.

#### Task 2.5: Risks 표에 D1 reversibility 행 추가 (Phase 5.0 P2#3 흡수)
- **Action**: 본 plan Risks 표에 한 행 추가:
  > `| dogfood 측정 trigger 부재 → 무한 deferred | Low | Medium | M3 plan 작성 시 dashboard 관련 PRD 갱신 발생 여부 의식 — 발생 시 본 plan의 D1(최소 live smoke) 결정을 revisit하는 backlog로 .claude/plans/codex-findings-backlog.md에 기록 (Task 2.7 처리). |`
- **Validate**: Risks 표에서 "dogfood 측정 trigger 부재" 매치 1건.

#### Task 2.6: 최소 live smoke 1회 (Codex F1 흡수, 사용자 측정 필요)
- **Action**:
  1. `docs/dashboard-dogfood.md` 신규 작성. 헤더 + Flow A/B/C 명세(cleanup plan Phase 3.1과 동일 budget 인용) + Smoke checklist(아래 step) + Record placeholder 표.
  2. Smoke checklist 실행(사용자):
     ```powershell
     go build -tags embed_dashboard -trimpath -o bin/secret-server.exe ./cmd/server
     $env:COMAX_AUTO_GENERATE_KEY="1"; ./bin/secret-server.exe
     # 별도 셸/브라우저:
     # 1. http://localhost:8080 로그인 (bootstrap token)
     # 2. /settings/sessions 진입 → SessionsPage 보임 확인
     # 3. 자기 세션 행 1개 + "현재 세션" 라벨 + revoke 버튼 disabled 확인
     # 4. audit log 1행 신규 추가 확인 (login 이벤트)
     ```
  3. 결과를 `docs/dashboard-dogfood.md` Record 표에 한 줄(date, duration_seconds, audit_row_id) 기록.
- **Mirror**: 옛 [.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md](./comax-secrets-dashboard-m2-cleanup.plan.md) Task 3.2 임베드 빌드 절차.
- **Validate**: `Test-Path docs/dashboard-dogfood.md` true + Record 표 1행 채워짐(빈 placeholder 아님). Smoke 4개 step 모두 ✓.

#### Task 2.7: Receipt 인용 + backlog 기록 (Codex F3 흡수)
- **Action**:
  1. Phase 5.6 자동 작성된 receipt 경로를 캡처(예: `.claude/receipts/mccp-plan-codex/{decision-slug}.receipt.json`).
  2. `.claude/reports/comax-secrets-dashboard-m2.report.md`의 "Gate review" 절에 receipt path + Codex session ID + finding disposition 표를 인용.
  3. `.claude/plans/codex-findings-backlog.md` 신규 또는 기존 파일에 한 줄 추가:
     > `2026-06-15 | INFO | .claude/plans/comax-secrets-dashboard-m2-close.plan.md | D1 최소 live smoke 결정 — dashboard 관련 PRD 갱신 시 revisit`
  4. PRD M2 행 Plan 셀에 receipt 인용 한 줄 footer 추가:
     > `(gate: impeccable + codex 019ec6da; receipt: .claude/receipts/mccp-plan-codex/...)` — 단 receipts는 git-ignored이므로 경로만 인용, 내용은 commit 안 함.
- **Validate**: report에서 "019ec6da" 매치 + backlog 파일에 본 plan 경로 인용 1건.

## Validation

```powershell
# Phase 1
Test-Path .claude\plans\completed\comax-secrets-dashboard-m2-cleanup.plan.md
Test-Path .claude\plans\completed\comax-secrets-dashboard-m2-close.plan.md
Test-Path .claude\reports\comax-secrets-dashboard-m2.report.md
Select-String -Path README.md, .claude\prds\*.md, .claude\reports\*.md, .claude\plans\**\*.md `
              -Pattern "plans/(?!completed/)comax-secrets-dashboard(\.plan)?\.md"

# Phase 2 — 코드 변경 없으므로 빌드/테스트는 PR CI에 위임
git status --short
git diff --stat
```

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Phase 5.0 impeccable이 master dashboard에서 신규 P1 design 결함을 발견 | Medium | Medium | 본 plan의 Task 2.x로 흡수. CRITICAL이면 5.5 stop으로 사용자 결정 요청. |
| Phase 5 codex가 HIGH 새 finding을 발견 (M2 closure 결정 자체에 대한) | Medium | Medium | 본 plan에 task 추가 + 같은 PR에 포함. cap=1 초과면 DIVERGENT_UNRESOLVED 명시. |
| dogfood live 측정을 미루는 결정이 추후 PRD acceptance 미달로 판정 | Low | Medium | report에 정직 기록 + 운영 단계 trigger로 분리 명시. PRD Success Metrics는 본인 운영 로그가 본질. |
| cleanup plan archive 시 내부 상대 경로 깊이 보정 누락 | Low | Low | 이동 후 grep 검증 (`../../../internal/` 패턴이 그대로면 +1 보정). |
| PRD M2 Plan 셀이 너무 길어져 가독성 저하 | Low | Low | 4개 링크를 `(plan · cleanup · closure · report)` 단형으로 압축. |
| M3 plan 작성 시점이 본 plan 머지 전이라 PRD 상태와 어긋남 | Low | Low | 별도 `/mccp:plan` 세션은 본 plan 머지 후. 본 plan은 M2 closure에만 집중. |
| dogfood 측정 trigger 부재 → 무한 deferred | Low | Medium | M3 plan 작성 시 dashboard 관련 PRD 갱신 발생 여부 의식 — 발생 시 본 plan의 D1(최소 live smoke) 결정을 revisit하는 backlog로 `.claude/plans/codex-findings-backlog.md`에 기록 (Task 2.7에서 처리). |

## Acceptance

- [x] PRD M2 행 상태 `done` + Plan 셀이 `completed/` 신경로 3개 + report 1개로 갱신됨.
- [x] `.claude/plans/comax-secrets-dashboard-m2-cleanup.plan.md`이 `completed/`로 이동됨 + 첫 줄 archive 메타.
- [x] `.claude/plans/comax-secrets-dashboard-m2-close.plan.md`(본 plan)도 `completed/`로 이동됨 + 첫 줄 archive 메타.
- [x] `.claude/reports/comax-secrets-dashboard-m2.report.md` 신규 작성됨 + 게이트 결과 1-2줄 요약 포함.
- [x] `Select-String` 패턴으로 옛 plan 경로 잔여 0건.
- [x] Phase 5.0 impeccable + Phase 5 codex 게이트 모두 자동 실행됨. CRITICAL stop 없음 (R1에서 모든 finding converged + ACCEPT_NOW).
- [x] **Task 2.3** Summary 정직성 한 줄 추가됨 (Phase 5.0 P1#1 / Codex F2 흡수).
- [x] **Task 2.4** Scope 인용 보존 grep 통과 (Phase 5.0 P1#2 / Codex F2 흡수).
- [x] **Task 2.5** Risks 표에 dogfood deferred 행 추가됨 (Phase 5.0 P2#3 / Codex F2 흡수).
- [x] **Task 2.6** 최소 live smoke 1회 수행 + `docs/dashboard-dogfood.md` Record placeholder 한 줄 채움 (Codex F1 흡수). 2026-06-15 PASS, 2분.
- [x] **Task 2.7** Receipt 경로/Codex session ID가 closure report + backlog에 인용됨 (Codex F3 흡수).

## Next Steps (mccp 컨벤션)

1. **Phase 5 게이트 자동 실행** — 본 plan 작성 직후 inline에서 5.0 impeccable + 5.1-5.7 codex.
2. **Receipt 자동 기록** — Phase 5.6에서 `mccp-plan-codex` receipt 자동 작성.
3. **`/mccp:prp-implement .claude/plans/comax-secrets-dashboard-m2-close.plan.md`** — Phase 1 + Phase 2 + archive 이동을 단일 PR.
4. **`/mccp:pr`** — PR 작성. PR 머지 후 별도 `/mccp:plan` 세션에서 **M3 (GitHub Actions integration)** 시작.

## Design Critique

> **Scope**: closure plan의 판단(Decisions D1·D2·D3)에 대한 plan-shape critique. Dashboard surface(Sessions.tsx 등)는 [comax-secrets-dashboard-m2-cleanup.plan.md](./comax-secrets-dashboard-m2-cleanup.plan.md)의 `## Design Critique (impeccable, plan-shape)` 절에 보관되며 master 코드(`web/dashboard/src/pages/Sessions.tsx`, `SessionRow.tsx`)의 JSDoc-pinned 결정으로 흡수 완료. Detector(`detect.mjs`)는 markdown 대상 부적합으로 스킵, Assessment A만 수행.

### Design Health Score (plan-decision lens)

| # | Dimension | Score | Key finding |
|---|---|---|---|
| 1 | 상태 가시성 (PRD↔plan↔archive) | 4 | 깨진 incoming 링크 명시 + Tasks 1.1-1.4가 grep + Test-Path로 잠금. |
| 2 | 현실 일치 (master 사실 검증) | 4 | `router.tsx:121-126`, `threat-model.md:69-126` 등 정확한 라인 인용. |
| 3 | 사용자 통제 (PRD 상태↔acceptance) | 3 | M2 행 4개 링크 명확. 단 "dogfood live 미수행"이 PRD 본문에는 미반영, report에만 기록. |
| 4 | 일관성 (M1 closure 패턴과 정합) | 3 | plan+report 정합. 단 M1은 실측 dogfood가 있었고 본 plan은 없음 — 대조점 미언급. |
| 5 | 에러 예방 (마감 후 회귀 방지) | 2 | size gate + axe + Playwright 머지됨. 단 PRODUCT.md "90초 안에 안전한가" 신호 사용자 검증 0회. |
| 6 | 인지 부담 (read once → 행동) | 4 | Decisions 3개 + Files 5개 + Tasks 6개. 한 화면에 들어감. |
| 7 | 효율성 (단일 PR 흐름) | 4 | Phase 1+2 단일 PR. Phase 3 분리 깔끔. |
| 8 | 최소주의 (코드 변경 0) | 4 | 정합성 회복 + 게이트 흡수 + report. 최소. |
| 9 | 회수 (CRITICAL stop) | 4 | Phase 5.5 명시. D3 흡수 흐름 명확. |
| 10 | 정직함 (DESIGN.md "정직함") | 2 | dogfood 미수행을 운영 trigger로 미룸 → 부분 정직. 그러나 "M2가 dashboard live 측정 0회로 마감"이라는 한 줄이 plan Summary에 없음 — report에만 묻힘. |
| **Total** | | **34/40** | **Good** — 정직성 한 단락만 Summary로 끌어올리면 ship-ready. |

### Anti-Patterns Verdict

- **회피한 closure reflex**: 가짜 ship 체크리스트 없음, 빈 축하 toast 없음, "다음 PR로 미루기" 책임 회피 없음(self-archive 패턴).
- **남아 있는 안티패턴 — "report에 묻기"**: D1 사유("PRD Success Metrics는 본인 운영 telemetry가 본질")가 기술적으로 맞지만, Summary는 "M2를 마감한다"만 말하고 "dashboard live 측정 0회로 마감"이라는 비교 가능한 단어가 등장하지 않음. PRODUCT.md "정직함" 가치와 부분 충돌.

### Priority Issues (Phase 2 task 흡수 대상)

- **[P1] Summary 정직성 한 줄 누락** → Task 2.3 (신규).
  Summary 마지막에 한 줄: "M2는 mccp 게이트(impeccable plan-shape + codex adversarial)만으로 closure를 검증하며, dashboard live 측정(`docs/dashboard-dogfood.md` Flow A/B/C)은 한 번도 수행되지 않은 채 done 처리된다. 운영 단계에서 별개 trigger로 측정 가능."
- **[P1] 게이트 scope 한정 명시 누락** → Task 2.4 (신규).
  본 `## Design Critique` 섹션 직전에 Scope 인용문(이미 추가됨). closure plan 자체에서도 "본 critique은 plan-shape, surface critique은 cleanup plan에 보관"임을 PRD reader가 6개월 뒤 따라갈 수 있도록 위치 유지.
- **[P2] D1 reversibility 경로 부재** → Task 2.5 (신규).
  Risks 표에 한 행: "dogfood 측정 trigger 부재 → 무한 deferred | Low | Medium | M3 plan 작성 시 dashboard 관련 PRD 갱신 발생 여부 의식 — 발생 시 본 plan의 D1을 revisit하는 backlog로 `.claude/plans/codex-findings-backlog.md`에 기록(지금)."

### Persona Red Flags

- **Alex (본인, dashboard 빌더)**: ⚠️ Summary가 "live 측정 0회"라는 *행동 가능한 사실*을 강조 안 함 → "한번 띄워볼까"를 자동 판단하기 어렵다. P1 #1로 흡수.
- **Jordan (6개월 뒤 PR 추적자)**: ⚠️ 본 plan만 읽고는 "원래 측정 Flow가 있었고 본 plan이 미룸"을 이해 못함 — cleanup plan Phase 3까지 따라가야 함. P1 #1 + P1 #2로 부분 완화.

### Minor

- Files 표의 README 행("확인 + 필요 시 UPDATE")은 Task 1.4 grep에 흡수해 표에서 빼면 깔끔.
- P3 자체는 별도 task 불필요(같은 plan 안에서 흡수).

### Questions to Consider

- Phase 5.0 plan-shape critique 자동 흡수는 M3-M8 closure에도 동일 패턴으로 굳어질 것인가? `patterns/closure-plan-self-archive` 같은 메타 패턴 고려.
- "운영 trigger로 미룸"이 dashboard를 만지지 않는 시즌에 *정직성 검증 자체*가 길게 미뤄지는 위험 — backlog 기록(P2 #3)이 첫 안전망.

> **Trend**: First run for `comax-secrets-dashboard-m2-close.plan.md`, no prior critique. Snapshot persistence 스킵 — plan-shape critique의 trend는 surface critique trend와 양립 어려움.

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/0.4.0/scripts/lib/codex-invoke.js adversarial-review --impeccable-available` (fail-closed Bash wrapper, mccp v0.4.0)
- 라운드 수: 1 (cap=1, R1에서 모든 finding을 plan 본문에 직접 absorb)
- Codex session 참조: `019ec6da-e2b1-7371-aac7-e4de92287094`
- Verdict: `needs-attention` → "No-ship: M2 closure still has a real gate gap and the plan is not PRP-ready because absorbed findings are not executable tasks."
- 합치 결론: 3개 finding 모두 ACCEPT_NOW. D1을 재정의해 최소 live smoke를 Task 2.6으로 신설하고, P1/P2 design critique 항목과 receipt artifact를 모두 명시 Task + Acceptance 체크박스로 승격해 D3 absorption model을 실제로 잠금.

### YAGNI Triage

| Finding | Severity | Confidence | Verdict | Why |
|---|---|---|---|---|
| F1 Live dogfood regression gate is removed before marking M2 done | HIGH | 0.84 | **ACCEPT_NOW** | Plan/design review는 routing/auth/API wiring/timeout/env propagation을 실측 못 함. D1을 "skip + report에 묻기"에서 "최소 live smoke + dogfood placeholder"로 재정의 → Task 2.6 신설. |
| F2 Phase 5 findings are named as absorbed but not added to executable tasks | HIGH | 0.95 | **ACCEPT_NOW** | Design Critique이 Tasks 2.3-2.5 흡수를 명시했으나 Tasks 섹션엔 2.1/2.2만 존재. prp-implement가 이행 불가 — D3 absorption model 자체가 깨짐. Task 2.3/2.4/2.5 명시 추가 + Acceptance 체크박스. |
| F3 Receipt chain is asserted but not made reviewable in the closure artifacts | MEDIUM | 0.78 | **ACCEPT_NOW** | Receipt 경로/링크/output이 Files to Change / Acceptance 어디에도 요구사항 없음 — self-archive PR에서 reviewer가 chain을 못 본다. Report에 receipt 인용 + Files to Change에 추가 + Acceptance 체크. |

### 본문 흡수 위치

- F1 → Decisions D1 재정의 + Task 2.6 (최소 live smoke + dogfood placeholder).
- F2 → Tasks Phase 2에 Task 2.3 / Task 2.4 / Task 2.5 명시 신설 (Phase 5.0 impeccable critique P1#1/P1#2/P2#3) + Acceptance 체크박스 3개.
- F3 → Files to Change에 receipt artifact 행 + Task 2.7 신설 (report에 receipt 인용 + PRD 마감 시점 receipt 검증) + Acceptance 체크박스.

- Deferred to backlog: 0
- Open Questions: 없음 (모든 finding R1 plan body 정정으로 해결).

## Codex Implementation Review

decision-set already converged in mccp-plan-codex review. No new implement-time decisions detected. Cross-gate dedupe applied.

- Verification: `git diff --name-only origin/master..HEAD` empty + untracked = closure plan 자체 (⊆ Files to Change). 코드 변경 0줄이라 새로운 추상화/외부 deps/동시성 결정 없음.
- Plan-codex R1 결론(F1→Task 2.6, F2→Tasks 2.3/2.4/2.5, F3→Task 2.7) 그대로 적용.

