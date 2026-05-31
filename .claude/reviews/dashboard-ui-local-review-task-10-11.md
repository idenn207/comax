# 로컬 리뷰: dashboard-ui M2 Task 10·11 (미커밋)

**리뷰 일자**: 2026-05-31
**브랜치**: `feat/dashboard-ui` (worktree)
**범위**: 미커밋 변경 — `git diff --name-only HEAD`로 추출한 10개 파일
**결정**: **APPROVE with comments** — CRITICAL/HIGH 없음, 검증 모두 그린

## 요약

M2 Task 10 (env-vs-env diff 화면) + Task 11 (audit feed)을 클라이언트 단독으로 추가한 PR. 백엔드 엔드포인트(`GET /api/v1/projects/{p}/envs/{e}/diff`, `GET /api/v1/audit`)는 Task 1에서 이미 구현됨. envelope 풀기 계약을 `apiFetchEnvelope` / `apiFetch` 두 단계로 쪼개 cursor pagination을 자연스럽게 수용한 점이 가장 큰 구조 변경.

타입 안전성·인증·CSRF·URL 인코딩 모두 기존 패턴과 일관됨. 보안·데이터 노출 측면에서 새로 들어온 위험 없음.

## 검증 결과

| 단계 | 결과 | 비고 |
|---|---|---|
| 타입 체크 (`pnpm typecheck`) | 통과 | tsc 클린 |
| 린트 (`pnpm lint`) | 통과 | `--max-warnings=0` 무경고 |
| 단위 테스트 (`pnpm test`) | 통과 | 11 파일 / **82개 테스트 그린** (신규 EnvDiff 4 + Audit 7 포함) |
| Go 회귀 | 해당 없음 | 서버 코드 변경 없음 (보고서 기준 `go test ./...` 그린) |

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM

1. **[router.tsx:21-26](../../web/dashboard/src/router.tsx#L21-L26) — 라우트 docblock이 stale**
   주석에 `/projects/$project/envs/$env/diff` 와 `/audit` 가 빠져 있다. 새로 추가한 두 라우트 모두 누락. 라우트 정의 직접 옆에 있는 문서가 코드와 어긋나면 미래 변경자가 라우트 전수 조회를 grep으로만 하게 됨 — 댓글의 신뢰가 깨지는 시작점.
   - **Fix**: 두 라우트 추가 (1줄씩).

2. **[Audit.tsx:40-43](../../web/dashboard/src/pages/Audit.tsx#L40-L43) — 폼 state가 prop `filter`와 mount 시점 한 번만 동기화**
   ```ts
   const [projectInput, setProjectInput] = useState(filter.project ?? '');
   ```
   동일 컴포넌트가 마운트된 상태에서 URL이 외부에서 바뀌면(예: 다른 페이지의 Link `to="/audit" search={{project:'x'}}` deep-link, 또는 향후 `replace=false`로의 변경) 폼 입력값이 URL을 반영하지 못함. 현재는 자체 navigate가 폼을 변경한 직후에만 일어나므로 사용자가 체감하는 버그는 없지만, deep-link 시나리오가 들어오는 순간 깨진다.
   - **Fix**: `useEffect(() => { setProjectInput(filter.project ?? ''); ... }, [filter])` 한 줄로 prop→state 미러링, 또는 controlled URL state 패턴으로 전환.

3. **[EnvSecrets.tsx:99](../../web/dashboard/src/pages/EnvSecrets.tsx#L99) — `search={{ against: '' }}` 가 빈 쿼리스트링을 강제**
   "환경 비교" 버튼이 항상 `?against=` 를 URL에 붙임. `validateSearch` 가 빈 문자열을 그대로 통과시키므로 동작상 문제는 없지만, 공유 URL에 무의미한 파라미터가 끼고 `EnvDiffPage` 진입 시 select에 placeholder가 떠 있는 동안 URL은 이미 `?against=` 가 박힌 상태가 됨. 검색 파라미터를 아예 생략하는 게 더 깨끗함.
   - **Fix**: `search={{}}` 또는 Button을 직접 `onClick`+`navigate` 로 바꿔 빈 against 키를 보내지 않도록.

### LOW

1. **[EnvDiff.tsx:180](../../web/dashboard/src/pages/EnvDiff.tsx#L180) — 한국어 조사 "와/과" 미적용**
   `${envName}와 ${against}의 키와 값이...` — 환경 이름이 자음으로 끝나면 "와"가 비문이 됨 (`prod와` ❌ `prod과` ✓). 환경 이름은 사용자 정의라 어떤 종성도 가능. 운영자 내부 화면이라 우선순위 낮음.
   - **Fix(택1)**: (a) "동일합니다 — {envName} 환경과 {against} 환경의 키·값이 모두 일치합니다" 처럼 조사 의존성 제거. (b) `hangul-js` 같은 유틸 도입(과한 의존성). 보통 (a) 권장.

2. **[Audit.tsx:54](../../web/dashboard/src/pages/Audit.tsx#L54) — `entries` 매 렌더마다 재계산**
   ```ts
   const entries = (query.data?.pages ?? []).flatMap((p) => p.entries);
   ```
   PAGE_LIMIT=50 기준 페이지 한두 개에서는 영향 없음. 다만 운영자가 "더 보기"를 10번 누른 뒤 폼에 타이핑하면 500개 배열 flatMap이 키 입력마다 돌아감.
   - **Fix**: `useMemo`로 감싸기. `[query.data?.pages]` 의존성.

3. **[EnvDiff.tsx:124](../../web/dashboard/src/pages/EnvDiff.tsx#L124), [:188-217](../../web/dashboard/src/pages/EnvDiff.tsx#L188-L217) — `−` (U+2212) 와 ASCII `-` 혼용**
   요약 라인의 `−` 는 Unicode minus, badge prop의 문자열도 동일. 일관되긴 하지만 코드 grep 시 `-2` (ASCII)와 `−2` (Unicode)가 서로 매칭되지 않음. 의식적 결정이면 유지해도 무방.

4. **[api.ts:90-96](../../web/dashboard/src/lib/api.ts#L90-L96) — `apiFetch` 가 `env.data as T` 로 단언**
   기존 동작 유지지만, envelope이 `ok:true / data:undefined` (서버 버그 가능성)인 경우 `undefined` 가 `T` 로 무성코 흘러가서 호출부에서 NRE를 트리거할 수 있음. 현재 `listAudit` 만 `apiFetchEnvelope` 경로로 `?? []` 폴백을 가지고, 나머지 `apiFetch` 호출은 모두 데이터를 신뢰함. 서버를 신뢰하는 결정 자체는 types.ts 헤더 docblock에 명시되어 있어 컨벤션과 일치. 향후 cross-origin embed 단계에서 zod 도입 시 함께 정리되는 게 자연스러움.

5. **[Audit.tsx:217-219](../../web/dashboard/src/pages/Audit.tsx#L217-L219) — `toLocaleString('ko-KR')` 의 SSR/타임존 결정**
   현재는 클라이언트 전용 SPA라 사용자 로컬 타임존이 적용되어 OK. 향후 SSR 또는 서버 사이드 렌더로 갈 일이 생기면 hydration mismatch 위험. 명시적 `Intl.DateTimeFormat` 이 안전. 현 단계에서는 과잉.

## 보안 체크 (체크리스트)

- [x] 하드코딩된 시크릿/토큰 없음
- [x] 사용자 입력 검증 — 클라이언트(actor 양의 정수)+서버(name regex, action, before/limit 정수) 양쪽
- [x] CSRF 토큰 — 변이 메서드에서 `X-CSRF-Token` 자동 부착 (api.ts:119-122)
- [x] XSS — 모든 사용자 입력은 React Text 컴포넌트로 escape됨. `entry.metadata`, `entry.target` 같은 raw 문자열은 텍스트 노드로만 렌더됨, `dangerouslySetInnerHTML` 미사용
- [x] URL 주입 — `encodeURIComponent` 일관 적용, `URLSearchParams` 사용
- [x] 토큰 누출 — actor를 `token_id`(정수)로만 노출, 토큰 plaintext 절대 미노출 (백엔드 `auditView` 주석으로 의도 명시)
- [x] 시크릿 로그 — 시크릿 값을 콘솔/페이지 컨텍스트에 적재하지 않음

## 코드 품질 체크

- [x] 함수 < 50 lines — 가장 큰 `apiFetchEnvelope` 78줄이지만 fetch 경계 핸들러 특성상 한 함수 유지가 readability에 유리
- [x] 파일 < 800 lines — 최대 `Audit.tsx` 282줄
- [x] 중첩 < 4 — 최대 3단
- [x] 명시적 에러 핸들링 — ApiError + code 분기 일관
- [x] console.log 없음
- [x] 신규 코드 테스트 커버리지 — EnvDiff 4 + Audit 7 = 11개 케이스, 빈 상태/에러/성공/페이지네이션/폼 검증/리셋 망라

## 리뷰한 파일

| 파일 | 변경 | 비고 |
|---|---|---|
| `web/dashboard/src/lib/types.ts` | Modified (+33) | 4개 신규 인터페이스 (EnvDiff/EnvDiffChanged/AuditEntry/AuditMeta/AuditPage) — 서버 JSON 태그 그대로 미러 |
| `web/dashboard/src/lib/api.ts` | Modified (+17/-4) | `apiFetchEnvelope` 추가, `apiFetch`는 thin wrapper로 단순화. envelope 풀기·CSRF·401/403 핸들링 일관 |
| `web/dashboard/src/lib/queries.ts` | Modified (+58) | `AuditFilter` 인터페이스 + `diffEnvs`/`listAudit` fetcher + queryKeys 확장 |
| `web/dashboard/src/router.tsx` | Modified (+57) | 2개 신규 라우트, validateSearch로 URL 정규화. **MED-1: docblock stale** |
| `web/dashboard/src/components/AppShell.tsx` | Modified (+7) | 헤더 우측 "감사 로그" Link, aria-label 정상 |
| `web/dashboard/src/pages/EnvSecrets.tsx` | Modified (+10) | "환경 비교" actions 버튼. **MED-3: 빈 against 쿼리스트링** |
| `web/dashboard/src/pages/EnvDiff.tsx` | Added (+250) | 3-컬럼 카드, aria-live 카운트, 키 클릭 시 LHS/RHS 시크릿 테이블로 이동 |
| `web/dashboard/src/pages/EnvDiff.test.tsx` | Added (+105) | 4 cases — 빈 상태/3분류/동일/422 에러 |
| `web/dashboard/src/pages/Audit.tsx` | Added (+220) | useInfiniteQuery, URL 필터, 양의 정수 actor 검증. **MED-2: prop→state 한 번만 sync** |
| `web/dashboard/src/pages/Audit.test.tsx` | Added (+130) | 7 cases — 빈/렌더/마지막 페이지 비활성/cursor 전달/폼/actor 검증/리셋 |

## 다음 단계

- MED-1 (router.tsx docblock 보정) 은 줄 단위 수정이라 커밋 전 즉시 처리 권장
- MED-2/3 은 별도 작은 follow-up 커밋 또는 Task 12 (a11y/visual polish) 묶음에 흡수 가능
- LOW-1 (조사) 은 사용자 노출 카피이므로 같은 follow-up에서 한 줄로 정리
- 그 외 LOW는 차후 zod 도입(LOW-4)이나 SSR 검토(LOW-5)와 함께 자연스럽게 해결됨
- 본 변경은 그대로 `/ecc:prp-commit` → `/ecc:prp-pr` 흐름으로 진행 가능 (`M2 Task 10·11 — Env diff + Audit feed`)
