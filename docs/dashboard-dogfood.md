# Dashboard Dogfood

> M2 closure(`comax-secrets-dashboard-m2-close.plan.md`)의 Task 2.6에서
> 신설. M2는 mccp 게이트(impeccable plan-shape + codex adversarial)와
> 본 문서의 **최소 live smoke 1회**로 closure를 검증한다. 정량 Flow
> A/B/C(클릭/초 budget)는 운영 단계의 별개 trigger로 분리된다.

## Scope

- **In**: dashboard-only flow. 임베드 빌드된 `bin/secret-server`를
  로컬에서 띄우고 브라우저로 진입한 SPA 동작 검증.
- **Out**: CLI flow는 [docs/dogfood.md](./dogfood.md)에서 별도 측정.
  본 문서는 dashboard surface에만 집중.

## Flow A/B/C (정량 측정, 운영 trigger로 분리)

PRD Success Metrics ("신규 envvar 1건 추가 → 전 환경 반영 시간",
"Local↔dev↔prod 누락 0건/월") 측정은 본인 운영 telemetry가 본질이고,
dashboard click count는 보조 지표다. M2 closure 시점에는 정의만 두고,
다음 dashboard 관련 PRD 갱신이 발생할 때 (예: M2 backlog craft, M3
GitHub Actions 통합으로 인한 dashboard 영향) trigger되어 측정한다.

| Flow | 시나리오 | Budget |
|---|---|---|
| **A** 새 envvar 추가 | project=api, env=local에 신규 `DEBUG_PORT=9999` 1개 등록 + 전 환경 반영 확인 | ≤ 30s / ≤ 12 clicks |
| **B** 잘못된 commit rollback | secret 1건의 직전 버전으로 되돌리기 (audit log → rollback 버튼 → confirm) | ≤ 30s / ≤ 8 clicks |
| **C** env-vs-env diff에서 누락 key 찾기 | local에만 존재하는 key를 env-vs-env diff 화면에서 발견 | ≤ 20s / ≤ 5 clicks |

### Record (Flow A/B/C)

| Flow | 측정일 (YYYY-MM-DD) | 소요(초) | 클릭 수 | Audit row id | Verdict |
|---|---|---|---|---|---|
| A | _placeholder_ | — | — | — | trigger 시 채움 |
| B | _placeholder_ | — | — | — | trigger 시 채움 |
| C | _placeholder_ | — | — | — | trigger 시 채움 |

## Minimal live smoke (M2 closure acceptance)

mccp 게이트는 routing/auth/API wiring/timeout/env propagation 같은
런타임 결합을 실측하지 못한다. closure 직전 1회 5-10분짜리 smoke로
회귀 게이트의 90%를 잠근다.

### Checklist

```powershell
# 1) 임베드 빌드
go build -tags embed_dashboard -trimpath -o bin/secret-server.exe ./cmd/server

# 2) 자동 키 생성 모드로 띄움 (별도 셸)
$env:COMAX_AUTO_GENERATE_KEY="1"; .\bin\secret-server.exe

# 3) 브라우저 진입 (default http://localhost:8080)
#    - bootstrap 토큰 또는 기존 service token으로 로그인
#    - /settings/sessions 진입 → SessionsPage 보임
#    - 자기 세션 행 1개 + "현재 세션" 라벨 + revoke 버튼 disabled
#    - audit log 화면에서 신규 login 이벤트 1행 추가 확인
```

### Record

| 측정일 (YYYY-MM-DD) | 소요(분) | Login 성공 | /settings/sessions 진입 | 자기 세션 보임 | Audit row 추가 | Verdict |
|---|---|---|---|---|---|---|
| 2026-06-15 | 2 | ✓ | ✓ | ✓ (현재 세션 라벨 + revoke disabled) | ✓ `session.create` token_id=1 session_id=1 @ 05:00:39 (선행 `auth.bootstrap` @ 05:00:25) | PASS |

> Audit 화면은 row 자체 id를 표시하지 않으므로 timestamp + action +
> token_id + session_id 조합으로 식별한다. login은 `session.create`
> action으로 기록되며, bootstrap token 최초 사용 시 직전에
> `auth.bootstrap` 행이 함께 남는다.

### Acceptance

- [x] 4개 step 모두 ✓.
- [x] Audit row 식별자 캡처 (`session.create` token_id=1 session_id=1 @ 2026-06-15 05:00:39).
- [x] 위 Record 표 한 줄에 측정값 기록.
- [x] 실패 시 closure plan을 revisit — 본 회차 PASS이므로 craft 추가 없음.

## D1 reversibility 메모

dashboard 관련 PRD 갱신(예: M3가 dashboard에 GitHub Actions tab 추가,
M2 backlog craft 등) 발생 시 `.claude/plans/codex-findings-backlog.md`에
"D1(최소 live smoke) 결정 revisit" 항목이 기록돼 있는지 확인하고,
필요하면 본 문서의 Flow A/B/C 측정을 trigger한다.
