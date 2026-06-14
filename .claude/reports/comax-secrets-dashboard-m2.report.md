# Report: Comax Secrets — M2 (Dashboard UI) closure

**PRD**: [.claude/prds/comax-secrets.prd.md](../prds/comax-secrets.prd.md)
**Plans (모두 `completed/` 인덱스)**:
- 본체: [completed/comax-secrets-dashboard.plan.md](../plans/completed/comax-secrets-dashboard.plan.md)
- cleanup: [completed/comax-secrets-dashboard-m2-cleanup.plan.md](../plans/completed/comax-secrets-dashboard-m2-cleanup.plan.md)
- closure: [completed/comax-secrets-dashboard-m2-close.plan.md](../plans/completed/comax-secrets-dashboard-m2-close.plan.md)

**Status**: done (2026-06-15)

## Shipped

- **M2 본체** (PR #3, #5, #6, #7, #8) — dashboard SPA shell, 로그인/세션
  라이프사이클, 프로젝트/환경/시크릿 CRUD, 버전 타임라인·롤백,
  env-vs-env diff, 감사 로그 피드, env_count API, craft polish (Task 12).
- **M2 cleanup** (PR #9, Codex Round 2 흡수 `5a4ba45` 포함) — Sessions
  통제 UI(`/settings/sessions`), 위협 모델 "Browser sessions" 섹션,
  prune sweeper(`cmd/server`의 `runPruneSweeper` 별도 함수 + 단위
  테스트), CI size gate (raw bytes — dashboard payload 1.5MB / server
  binary 30MB).
- **M2 closure** (본 PR) — PRD M2 행 `done` 마감 + cleanup·closure plan
  archive + mccp 게이트 재검토 + closure report + dogfood 문서 신설 +
  최소 live smoke 1회.

## Gate review (옛 dogfood live 측정 자리를 게이트로 대체)

mccp v0.4.0 게이트 두 단계가 옛 cleanup plan의 "Phase 5.0 impeccable
live audit" + "Phase 5 codex adversarial review"를 자동 흡수했다.

- **Phase 5.0 impeccable critique (plan-shape)** — closure plan body
  `## Design Critique` 절에 인라인 보관. Health score 34/40 (Good).
  P1 #1/#2/P2 #3 finding 3개를 Tasks 2.3/2.4/2.5로 흡수.
- **Phase 5 plan-codex adversarial review** — Codex session
  `019ec6da-e2b1-7371-aac7-e4de92287094`. Verdict `needs-attention` →
  3 finding 모두 ACCEPT_NOW + R1에서 plan 본문 정정으로 해결 (cap=1).
  - F1 (HIGH 0.84) "live dogfood regression gate 제거됨" → D1 재정의 +
    Task 2.6 (최소 live smoke + dogfood placeholder) 신설.
  - F2 (HIGH 0.95) "finding이 task로 승격 안 됨" → Tasks 2.3/2.4/2.5 +
    Acceptance 체크박스 명시.
  - F3 (MEDIUM 0.78) "receipt가 reviewable artifact 아님" → Files to
    Change에 receipt 행 + Task 2.7 + PRD M2 셀 footer 인용.
- **Phase 2.5 implement-codex cross-gate dedupe** — implement 진입 시
  새로운 implement-time decision 없음(코드 변경 0) 확인 + Codex 재호출
  없이 dedupe path로 진행.

### Finding disposition

| ID | Severity | Verdict | Absorbed at |
|---|---|---|---|
| Phase 5.0 P1 #1 Summary 정직성 누락 | P1 | Task 2.3 | plan Summary footer |
| Phase 5.0 P1 #2 Scope 인용문 위치 | P1 | Task 2.4 | plan line 221 (Scope 인용) |
| Phase 5.0 P2 #3 D1 reversibility | P2 | Task 2.5 | plan Risks 표 신규 행 |
| Codex F1 Live gate 제거 | HIGH | Task 2.6 | `docs/dashboard-dogfood.md` + minimal live smoke |
| Codex F2 finding → task 미승격 | HIGH | Tasks 2.3/2.4/2.5 | Acceptance 체크박스 3개 |
| Codex F3 receipt 비-reviewable | MEDIUM | Task 2.7 + 본 report | receipt 인용 + PRD M2 셀 footer |

### Receipt chain (git-ignored)

- `mccp-plan-codex` — `.claude/receipts/mccp-plan-codex/comax-secrets-dashboard-m2-close.json` (round=1, converged=true)
- `mccp-implement-codex` — `.claude/receipts/mccp-implement-codex/comax-secrets-dashboard-m2-close.json` (cross-gate dedupe, no new implement-time decisions)

PRD M2 셀 footer에 `(gate: impeccable + codex 019ec6da; receipt: ...)` 한
줄로 인용. receipts는 `.gitignore`되어 PR diff에는 안 보이고 path로만
참조한다. backlog는 [.claude/plans/codex-findings-backlog.md](../plans/codex-findings-backlog.md)
에 D1 reversibility trigger 한 줄 기록.

## Not shipped (의도된 분리)

- `docs/dashboard-dogfood.md` **Flow A/B/C 정량 측정** (클릭 수 / wall-clock)
  — 운영 단계의 별개 trigger로 미룸. PRD Success Metrics ("신규 envvar
  1건 추가 → 전 환경 반영 시간", "Local↔dev↔prod 누락 0건/월")
  측정은 본인 운영 telemetry가 본질이고, dashboard click count는
  보조 지표라 M2 acceptance에서 빠뜨려도 무방. 다음 dashboard 관련
  PRD 갱신 시 backlog 항목을 따라 revisit.
- **Multi-token admin 권한** — M2 범위 밖. 다른 service token이 발급한
  세션은 회수 불가, 그 token 자체를 revoke해야 함. `docs/threat-model.md`
  Browser sessions 섹션에 명시.
- **`last_used_at` 컬럼** — schema에 없음. v1에서 "Created" 라벨만
  표시. M3 backlog로 분리 (impeccable P2 #5 결정).

## Live smoke (Task 2.6 결과)

`bin/secret-server.exe` 18.3 MB (size gate 30 MB 한도 안). 측정 결과는
[docs/dashboard-dogfood.md](../../docs/dashboard-dogfood.md) Record 표 참조.

## Next

- **M3 — GitHub Actions integration**. 별도 `/mccp:plan
  .claude/prds/comax-secrets.prd.md` 세션으로 시작. M3 plan 작성 시
  dashboard 영향 발생 여부 의식 — 발생 시 backlog의 D1 revisit 항목
  확인.
- **M2 backlog craft**: `last_used_at` 컬럼 추가 + Sessions 화면 "Last
  activity" 컬럼. 별도 trigger 시 진행.
