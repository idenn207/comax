# 실행 보고서: Comax Secrets 대시보드 M2 Task 10·11

**원본 플랜**: [.claude/plans/comax-secrets-dashboard.plan.md](../plans/comax-secrets-dashboard.plan.md)
**범위**: M2 Task 10 (Env-vs-env diff 화면) + Task 11 (Audit feed)
**브랜치**: `feat/dashboard-ui` (worktree)
**상태**: 코드 완료, 검증 그린, 미커밋

## 요약

대시보드 SPA에 두 화면을 추가했다.

- **Env-vs-env diff 화면**: `/projects/$project/envs/$env/diff?against=<rhs>`. 다른 환경을 골라 키 셋·값 차이를 세 분류(`added` / `removed` / `changed`)로 보여준다. 각 행은 해당 환경의 시크릿 테이블로 연결된다.
- **Audit feed 화면**: `/audit?project=&env=&actor=&action=`. URL에 필터를 저장하고, `useInfiniteQuery` 기반 커서(`before=<id>`) 페이지네이션으로 "더 보기" 한 번에 50건씩 추가 로드한다.

서버 API(`GET .../diff`, `GET /audit`)는 Task 1에서 이미 구현되어 있어 이번 작업은 클라이언트 전용이다.

## 플랜 예측 vs 실제

| 항목 | 플랜 예측 | 실제 |
|---|---|---|
| 작업 분할 | 화면 1개씩 PR | UI만 묶어서 한 PR로 처리 (요청에 따름) |
| 신규 화면 수 | 2 | 2 |
| 신규 백엔드 변경 | 0 (Task 1에서 끝남) | 0 |
| 가벼운 클라이언트 라이브러리 변경 | api.ts 보조 함수 1개 | `apiFetchEnvelope` 1개 추가, `apiFetch`를 그 위에 얹어 DRY |

## 태스크 완료 현황

| # | 태스크 | 상태 | 비고 |
|---|---|---|---|
| 10 | Env-vs-env diff 화면 | 완료 | 3-컬럼 카드 + 키 클릭 시 시크릿 테이블 이동 |
| 11 | Audit feed | 완료 | URL 필터 + 커서 페이지네이션 + 토큰 ID 정규 표현 검증 |

## 검증 결과

| 단계 | 결과 | 비고 |
|---|---|---|
| 타입 체크 (`pnpm typecheck`) | 통과 | tsc 클린 |
| 린트 (`pnpm lint`) | 통과 | `--max-warnings=0` 무경고 |
| 단위 테스트 (`pnpm test`) | 통과 | 11 파일 / **82개 테스트 그린** (신규 EnvDiff 4개 + Audit 7개) |
| 빌드 (`pnpm build`) | 통과 | JS 442 KB raw / 138 KB gz, CSS 718 KB raw / 84 KB gz |
| Prettier 검사 | 통과 | 신규/변경 파일에만 적용 |
| Go 테스트 (`go test ./... -count=1`, CGO_ENABLED=0) | 통과 | 서버 회귀 없음. race 플래그는 로컬 Win에서 CGO 64-bit 컴파일러 부재로 보류 — CI 게이트에 위임 |

## 변경 파일

| 파일 | 동작 | 비고 |
|---|---|---|
| `web/dashboard/src/lib/types.ts` | UPDATE | `EnvDiff`, `EnvDiffChanged`, `AuditEntry`, `AuditMeta`, `AuditPage` 타입 추가 (+33 lines) |
| `web/dashboard/src/lib/api.ts` | UPDATE | `apiFetchEnvelope<T, M>` 추가, 기존 `apiFetch`는 그 래퍼로 단순화 (+17 / -4) |
| `web/dashboard/src/lib/queries.ts` | UPDATE | `queryKeys.envDiff` / `queryKeys.audit` + `diffEnvs` / `listAudit` fetcher + `AuditFilter` 인터페이스 추가 (+58) |
| `web/dashboard/src/router.tsx` | UPDATE | `/projects/$project/envs/$env/diff` (`validateSearch: against`) + `/audit` (`validateSearch: project,env,actor,action`) 라우트 추가 (+57) |
| `web/dashboard/src/components/AppShell.tsx` | UPDATE | 헤더 우측에 "감사 로그" 링크 추가 (+7) |
| `web/dashboard/src/pages/EnvSecrets.tsx` | UPDATE | actions에 "환경 비교" 버튼 추가 (+10) |
| `web/dashboard/src/pages/EnvDiff.tsx` | CREATE | 3-컬럼 diff 화면 (+250) |
| `web/dashboard/src/pages/EnvDiff.test.tsx` | CREATE | RTL 단위 테스트 4개 (+105) |
| `web/dashboard/src/pages/Audit.tsx` | CREATE | 필터·페이지네이션 가능한 감사 로그 (+220) |
| `web/dashboard/src/pages/Audit.test.tsx` | CREATE | RTL 단위 테스트 7개 (+130) |

## 플랜으로부터의 deviation

- **`apiFetchEnvelope` 헬퍼 추가**: Audit 응답의 `meta.next_before` 커서를 페이지네이션에 활용해야 해서, `apiFetch`가 항상 `data`만 풀어주던 기존 계약을 깨지 않고 envelope(`{ data, meta }`)를 반환하는 보조 함수를 추가했다. 기존 `apiFetch`는 이 함수의 얇은 래퍼로 재정의해 DRY를 지켰다.
- **검색 파라미터 검증**: 플랜은 검색 파라미터 처리법을 명시하지 않았다. TanStack Router의 `validateSearch`로 URL 들어올 때 `against`/`project`/`env`/`actor`/`action`을 정규화하여, 잘못된 URL에서도 안전하게 빈 객체로 떨어지도록 했다.

## 발견한 문제와 처리

- **prettier 글로벌 포맷 사고**: `pnpm format` 스크립트가 프로젝트 전체를 재포맷하기 때문에, 셸 cwd가 worktree에서 main repo로 미세하게 전환된 시점에 master 디렉터리의 52개 파일이 오염되었다. `git restore` 로 master를 원복하고, worktree의 의도된 파일만 `pnpm prettier --write <files...>` 로 명시 포맷했다.
- **race 플래그**: `go test -race`는 로컬 Windows에 64-bit C 컴파일러가 없어 빌드 실패. 프로젝트 약속(`CGO_ENABLED=0`)을 지키기 위해 race는 CI에서 검증되도록 두고, 로컬에서는 CGO 비활성으로 풀세트를 통과시켰다.

## 작성한 테스트

| 테스트 파일 | 케이스 수 | 다룬 영역 |
|---|---|---|
| `web/dashboard/src/pages/EnvDiff.test.tsx` | 4 | (1) 미선택 시 안내 카드 (2) added/removed/changed 3분류 렌더 (3) 동일 환경 안내 (4) 422 bad_reference 에러 메시지 |
| `web/dashboard/src/pages/Audit.test.tsx` | 7 | (1) 빈 상태 (2) 테이블 렌더 (3) `마지막 페이지` 비활성 (4) `더 보기` → `before=10` 커서 전달 (5) 폼 제출 → navigate 호출 (6) 음수 actor 검증 (7) 초기화 버튼 |

신규 테스트는 라우터 마운트 없이 `vi.mock('@tanstack/react-router')`로 `useNavigate` / `Link`를 스텁하는 기존 패턴을 그대로 따랐다.

## 남은 M2 태스크

이 보고서는 M2 마일스톤 일부만 다룬다. 플랜 파일은 archive 하지 않고 그대로 둔다.

| # | 태스크 | 상태 |
|---|---|---|
| 1 | API 추가 (read-side) | 완료 (이전 PR) |
| 2 | API 추가 (write-side) | 완료 (이전 PR) |
| 3 | 브라우저 세션 + CSRF | 완료 (이전 PR) |
| 4 | SPA embed 파이프라인 | 완료 (이전 PR) |
| 5 | Vite + React 스캐폴드 | 완료 (이전 PR) |
| 6 | 로그인 / 세션 라이프사이클 | 완료 (이전 PR) |
| 7 | Projects/Envs 화면 | 완료 (직전 커밋) |
| 8 | Secrets 테이블 | 완료 (직전 커밋) |
| 9 | 버전 타임라인 + 롤백 | 완료 (직전 커밋) |
| **10** | **Env-vs-env diff** | **본 보고서 완료** |
| **11** | **Audit feed** | **본 보고서 완료** |
| 12 | a11y + visual polish + 안티-템플릿 | 다음 |
| 13 | 임베드 바이너리 사이즈 게이트 | 다음 |
| 14 | cross-compile + docker compose smoke | 다음 |
| 15 | Operator dogfood | 다음 |

## 다음 단계 제안

- `/ecc:code-review` 또는 `/ecc:review-pr`로 변경 리뷰
- `/ecc:prp-commit`으로 커밋, `/ecc:prp-pr`로 PR 생성 (기존 흐름: `M2 Task 10·11 — Env diff + Audit feed`)
- 이후 Task 12 (a11y/visual polish) 진행
