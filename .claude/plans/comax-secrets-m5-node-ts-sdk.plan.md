# Plan: M5 — Node/TS SDK + npm publish

**Source PRD**: `.claude/prds/comax-secrets.prd.md`
**Selected Milestone**: #5 — Node/TS SDK + npm publish
**Complexity**: Large

## Summary

Next.js(및 임의의 Node 18+/Edge) 앱이 런타임에 Comax Secrets 서버에서 시크릿을 fetch → 캐시 → reload 할 수 있는 **단일 npm 패키지**를 만든다. 서버는 이미 SDK를 1급 소비자로 설계(고정 envelope, 해석-완료 `Secret` 반환, M3 Bearer 서비스 토큰 인증)했으므로 **신규 서버 기능은 없다** — 기존 HTTP 표면 위의 클라이언트 계층 + 인메모리 캐시 + reload 트리거(수동/TTL/웹훅)만 구현한다. 시크릿 blast radius 최소화를 위해 **`get(key)`는 단일-키 엔드포인트 기본**, whole-env 적재는 `getAll()`/`preload()` opt-in으로 제한한다. 모든 fetch는 timeout/AbortSignal로 취소 가능하며 실패한 in-flight는 즉시 정리한다. published 라이브러리 제약(zero-dep, dual ESM/CJS, Edge 호환)을 지킨다.

## Selected Decisions (확정 제안 — CONFIRM 시 고정)

| # | 결정 | 제안 | 근거 |
|---|---|---|---|
| D1 | 패키지 위치 | `sdk/` (repo 루트) | `pkg/client`(Go 외부 소비자용)의 TS 대응물. `web/`는 브라우저 SPA 전용. published 라이브러리라 앱 트리와 분리. |
| D2 | 패키지 이름 | `@comax-secrets/sdk` | org/repo `comax-secrets`, action `comax-secrets/load-action`와 정렬. 스코프로 향후 패키지(sdk/cli-helpers) 그룹화. |
| D3 | 런타임 의존성 | **zero-dep** (global `fetch` + 손수 타입가드) | SSR/Edge/Workers 호환 + 번들 최소화. zod는 앱(dashboard)용이지 좁은 서버-제어 응답엔 과함. |
| D4 | 모듈 포맷 | dual ESM+CJS+`.d.ts` (`tsup`) | npm TS 라이브러리 표준. `exports` 조건부 맵으로 import/require/types 분기. |
| D5 | 캐시 모델 | **`get(key)`=단일-키 fetch + per-key TTL(default)**, whole-env는 `getAll()`/`preload()` opt-in, 둘 다 single-flight | **Codex F2**: 단일 키 조회가 env 전체 평문을 메모리에 적재하는 blast radius 제거. 서버에 단일-키 엔드포인트 존재(`client.go:264`). bulk-inject 용도만 whole-env. |
| D6 | reload 트리거 | 수동 `reload()` + lazy TTL + (opt-in) interval + webhook-verify 서브모듈 | PRD "cache + reload"의 실질 페이오프. interval은 long-running Node 한정, Edge 가드. |
| D7 | npm publish 실제 발행 | 패키징+워크플로만 준비, 실제 `npm publish`는 operator 수동 트리거 | npm 토큰/org 등록은 사람 액션. provenance 워크플로는 `workflow_dispatch`로 게이트. |
| D8 | fetch 복원력 | `timeoutMs` 기본(10s, `client.go:51` 미러) + per-call `AbortSignal` + `AbortController` 취소 + 실패/취소 flight `finally` 정리 | **Codex F1**: single-flight 공유 promise가 timeout 없이 걸리면 모든 `get/reload`가 동반 정지 → SSR/Edge 장애 증폭. 실패한 flight는 캐시를 오염시키지 않고 즉시 버림. |
| D9 | repo gate 통합 | `sdk.yml`을 **required PR check**로. top-level `make test`/`lint`에는 **의도적 미포함** | **Codex F4** 부분수용: SDK 회귀를 CI required-check로 잠금. 단 `make test`가 Node 툴체인을 강제하면 "Node 없이 go test 가능"(Makefile:24-26) 속성이 깨지므로 fold-in은 거부. |

> npm 실제 발행(D7)은 M8 Public release와 겹치는 operator 액션이므로 본 마일스톤은 "발행 가능한 패키지 + CI 검증"까지를 acceptance로 잡는다.

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| HTTP client | `pkg/client/client.go:113` `do()` | URL 조립 → Bearer 헤더 → `{ok,data,error}` 파싱 → status≥400 시 `APIError{Status,Code,Message}` |
| Error 코드 → 타입 | `internal/server/response.go:66` `httpError` | `not_found`/`conflict`/`unauthorized`/`bad_request`/`forbidden`/`version_not_found` 코드 집합 |
| Sentinel 매핑 | `pkg/client/client.go:92` `Err*` + `Is()` | 코드 문자열 → 타입 에러(`errors.Is` 동치) — TS는 `ComaxError` 서브클래스 + `code` 필드로 미러 |
| Secret shape | `pkg/client/client.go:246` `Secret` | `{key,value,version,updated_at}` 해석-완료 평문 |
| Fetch 엔드포인트 | `internal/server/router.go:49-50` | `GET /api/v1/projects/{p}/envs/{e}/secrets`, `.../secrets/{k}` |
| Auth arm | `internal/server/middleware.go:127` `authBearer` | `Authorization: Bearer <service-token>`, CSRF 없음, non-admin 토큰도 read 가능 |
| Webhook 서명 | `internal/webhook/signer.go:28` `Sign` | `X-Comax-Signature: sha256=hex(HMAC-SHA256(secret,"<ts>.<body>"))` + `X-Comax-Timestamp`, 상수시간 비교 필수 |
| TS 툴링 | `web/dashboard/package.json`, `tsconfig.app.json` | TS 5.6 / ES2022 / strict / vitest / eslint / prettier, `@comax` 스코프 |
| Build 배선 | `Makefile:59` `dashboard` 타깃 | `cd <dir> && npm ci && npm run build` — SDK도 동일 형태 `sdk` 타깃 |
| 시크릿 로그 금지 | CLAUDE.md 작업규칙, `middleware.go:53` logMiddleware | 값은 절대 로그/에러 문자열에 넣지 않음 — 테스트로 강제 |

## Files to Change

| File | Action | Why |
|---|---|---|
| `sdk/package.json` | CREATE | `@comax-secrets/sdk`, exports 맵, scripts(build/typecheck/test/lint/format), engines node>=18, zero deps |
| `sdk/tsconfig.json` | CREATE | dashboard 베이스라인 미러(ES2022/strict) + `declaration`/emit는 tsup가 담당 |
| `sdk/tsup.config.ts` | CREATE | dual ESM+CJS + dts, entry `src/index.ts` + `src/webhook.ts` |
| `sdk/eslint.config.js`, `sdk/.prettierrc` | CREATE | dashboard 설정 재사용/미러 |
| `sdk/.gitignore`, `sdk/.npmignore` (또는 `files`) | CREATE | `dist/`, `node_modules/` 무시; 배포 산출물만 포함 |
| `sdk/src/errors.ts` | CREATE | `ComaxError`/`ComaxAuthError`/`ComaxNotFoundError`/`ComaxConflictError` + `code` |
| `sdk/src/http.ts` | CREATE | envelope 파싱 + Bearer + 에러코드→타입 (client.go `do()` 미러) |
| `sdk/src/secrets.ts` | CREATE | `SecretsClient`: TTL 캐시 + single-flight + `get/getAll/has/reload` |
| `sdk/src/index.ts` | CREATE | `createClient`/`createClientFromEnv` 팩토리 + 타입 re-export |
| `sdk/src/webhook.ts` | CREATE | `verifyWebhookSignature()` (signer.go 미러, 상수시간 비교) — 서브패스 export |
| `sdk/src/*.test.ts` + `sdk/test/fixtures/` | CREATE | vitest: http/캐시/에러/webhook + timeout·abort + Go signer golden 벡터 + envelope 계약 fixture, mocked fetch, ≥80% |
| `sdk/README.md` | CREATE | Next.js SSR/Edge/Route-Handler 사용법 + singleton 패턴 + reload-on-webhook 예제 |
| `Makefile` | UPDATE | `sdk` 타깃(npm ci + build + test) 추가, `.PHONY`에 등록 |
| `.github/workflows/sdk.yml` | CREATE | PR: typecheck+test+build; `workflow_dispatch`: `npm publish --provenance`(토큰 시크릿 게이트) |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | Milestone #5 행 `pending`→`in-progress`, Plan 셀에 본 plan 경로 |

## Tasks

### Task 1: `sdk/` 패키지 스캐폴딩
- **Action**: `sdk/`에 package.json(`@comax-secrets/sdk`, zero deps, exports 맵 `"."`+`"./webhook"`, engines node>=18), tsconfig(dashboard 미러 + emit는 tsup), tsup.config.ts(dual ESM/CJS/dts), eslint/prettier/.gitignore 생성. tsup/vitest/typescript/eslint는 devDeps.
- **Mirror**: `web/dashboard/package.json` scripts·`tsconfig.app.json` compilerOptions
- **Validate**: `cd sdk && npm i && npm run typecheck` (빈 index로 통과)

### Task 2: 에러 타입 + HTTP 코어
- **Action**: `errors.ts`에 `ComaxError`(base, `code`/`status`) + 서브클래스. `http.ts`에 `request()` — baseUrl 조립, `Authorization: Bearer`, `Accept: application/json`, `{ok,data,error}` 파싱, status≥400 → 코드별 타입 에러. 주입 가능한 `fetch` 옵션.
- **Mirror**: `pkg/client/client.go:113` `do()`, `response.go:66` 코드 집합
- **Validate**: `vitest run src/http.test.ts` — 200/401/404/409/malformed-envelope 케이스

### Task 3: SecretsClient — 캐시 + reload + 취소
- **Action**: `secrets.ts`에 `SecretsClient`. **`get(key)`=단일-키 fetch + per-key `Map<key,{secret,fetchedAt}>` (default)**; `getAll()`/`preload()`=whole-env fetch(bulk-inject opt-in). per-key TTL 만료 판정, in-flight promise 공유(single-flight)하되 **실패/취소 시 `finally`로 flight 제거(캐시 미오염, Codex F1)**. 모든 fetch에 `timeoutMs`(기본 10s) + `AbortSignal` 결합(`AbortSignal.timeout`/외부 signal merge). `has(key)`/`reload(key?)`. opt-in `refreshIntervalMs`(Node 한정, `unref()` 가드).
- **Mirror**: `client.go:264` `GetSecret`(단일 키 기본), `client.go:255` `ListSecrets`(whole-env), `client.go:51` timeout 기본(10s)
- **Validate**: `vitest run src/secrets.test.ts` — 단일키 cache hit/miss, per-key TTL 만료 재fetch, 동시 get single-flight(단일 fetch 호출), **timeout→AbortError→다음 호출 정상 재fetch(flight 정리 확인)**, reload 강제 refresh, `get(key)`가 whole-env 엔드포인트를 치지 않음(단일-키만)

### Task 4: 팩토리 + 환경변수 편의 + Next.js 문서
- **Action**: `index.ts`에 `createClient({baseUrl,token,project,env,ttlMs,fetch?})` + `createClientFromEnv()`(`COMAX_URL`/`COMAX_TOKEN`/`COMAX_PROJECT`/`COMAX_ENV`). README에 Next.js Route Handler/Server Component/Edge 사용법 + 모듈-스코프 singleton 주의(서버리스 콜드스타트 캐시 리셋).
- **Mirror**: `client.go:40` `New()` 시그니처(baseUrl/token/timeout 검증)
- **Validate**: `vitest run src/index.test.ts` — env 해석/누락 시 명확한 에러; README 예제 tsc 통과

### Task 5: Webhook 서명 검증 서브모듈
- **Action**: `webhook.ts`(서브패스 `./webhook`)에 `verifyWebhookSignature({secret, signatureHeader, timestampHeader, body, toleranceSec})` — `sha256=hex(HMAC-SHA256(secret,"<ts>.<body>"))` 재계산, `crypto.timingSafeEqual`(Node) / Web Crypto `subtle`(Edge) 상수시간 비교, timestamp tolerance replay 가드. reload-on-event 예제. **golden 벡터 fixture는 Go `signer.Sign`으로 1회 생성해 `sdk/test/fixtures/webhook-vectors.json`에 커밋**(byte material 계약 고정).
- **Mirror**: `internal/webhook/signer.go:28` `Sign` (동일 서명 material·헤더 이름)
- **Validate**: `vitest run src/webhook.test.ts` — 유효/변조body/변조ts/tolerance초과 4케이스 + **Go signer golden 벡터 일치(계약 드리프트 차단, Codex F3)**

### Task 6: Build·pack·CI 배선 + repo gate 통합
- **Action**: Makefile `sdk` 타깃(npm ci + typecheck + lint + test + build) + `.PHONY` 등록. `.github/workflows/sdk.yml`: PR 트리거 typecheck+lint+test+build를 **required check로**(D9, Codex F4); `workflow_dispatch` publish 잡(`--provenance`, `NPM_TOKEN` 시크릿 게이트, `if` 조건). `npm pack --dry-run`로 `files`/exports 검증. **ESM `import`/CJS `require` 각각 로드 스모크** + **envelope/에러코드 fixture 계약 테스트**(Codex F3). `make test`에는 의도적 미포함(Makefile:24-26 Node-optional 속성 보존).
- **Mirror**: `Makefile:59` `dashboard` 타깃 형태, 기존 `.github/workflows/*.yml`(M3 산출물)
- **Validate**: `cd sdk && npm run build && npm pack --dry-run` (dist에 ESM/CJS/dts 포함 확인), `make sdk`, dual-format 로드 스모크 green

### Task 7: 커버리지 + 시크릿-로그-금지 가드
- **Action**: vitest coverage ≥80% 달성. 별도 테스트: 던져지는 에러 메시지/`console` 출력에 시크릿 **값**이 절대 포함되지 않음을 assert(잘못된 토큰/실패 응답 경로 포함).
- **Mirror**: CLAUDE.md "시크릿은 절대 로그에 남기지 않는다(테스트로 강제)", 패키지 80% 룰
- **Validate**: `cd sdk && npm run test:coverage` — 총 라인/함수 ≥80%, no-secret 테스트 green

### Task 8: PRD/plan 부기 + report 준비
- **Action**: PRD Milestone #5 행 `pending`→`in-progress`, Plan 셀 갱신(본 plan에서 수행). 구현 완료 시 `.claude/reports/comax-secrets-m5-node-ts-sdk.report.md` 작성(구현/커밋/PR 단계).
- **Mirror**: 선행 마일스톤(M3/M4)의 report + PRD 행 갱신 패턴
- **Validate**: PRD 테이블 행이 in-progress + plan 링크 resolvable

## Validation

```bash
# SDK 로컬 검증 (POSIX shell / Git Bash)
cd sdk
npm ci
npm run typecheck          # tsc strict, 0 errors
npm run lint               # eslint --max-warnings=0
npm run test:coverage      # vitest, ≥80% line+func
npm run build              # tsup → dist/{index,webhook}.{js,cjs,d.ts}
node -e "import('./dist/index.js').then(m=>console.log('esm ok', typeof m.createClient))"  # ESM 로드 스모크
node -e "console.log('cjs ok', typeof require('./dist/index.cjs').createClient)"            # CJS 로드 스모크
npm pack --dry-run         # files/exports 검증, 시크릿·소스맵 누출 없음

# 저장소 통합
cd ..
make sdk                   # npm ci + build + test 일괄
go build ./...             # 서버/CLI 회귀 없음(SDK는 Go 빌드 무관)
```

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Edge 런타임에서 Node `crypto` 미가용 → webhook-verify 깨짐 | Medium | Web Crypto(`crypto.subtle`) 우선 + Node fallback, 두 경로 모두 테스트. 서브패스 export로 미사용 시 tree-shake |
| 서버리스 콜드스타트가 모듈-스코프 캐시를 매번 리셋 → 캐시 무의미 | Medium | 캐시는 warm 인스턴스 내 round-trip 절감이 목적임을 README에 명시. 영속 캐시는 v1 범위 밖 |
| dual ESM/CJS `exports` 오구성 → Next.js import 실패 | Medium | `npm pack` + 실제 import 스모크(ESM `import`/CJS `require`) 테스트로 게이트 |
| 시크릿 값이 에러 메시지/스택에 유출 | Low | Task 7 전용 가드 테스트. 에러는 code/status만 노출(client.go `APIError` 미러) |
| npm 스코프 `@comax-secrets` 미소유 → publish 불가 | Low | D7대로 실제 발행은 operator 게이트. 이름 미확정 시 CONFIRM에서 조정 가능 |
| **single-flight stall이 전 요청 블록 (Codex F1)** | Medium | D8: `timeoutMs`+`AbortSignal`+실패 flight `finally` 정리. timeout→재fetch 테스트를 acceptance로 |
| **whole-env 적재 blast radius (Codex F2)** | Medium | D5: `get(key)` 단일-키 기본, whole-env는 명시 opt-in, per-key TTL |
| **서버 계약 드리프트(path encoding·envelope·webhook byte material) (Codex F3)** | Medium | Go signer golden 벡터 + envelope fixture 계약 테스트를 required CI로. live 스모크는 report 단계 **필수** 1회로 승격. 전체 CI 서버-기동 매트릭스는 backlog |

## Acceptance

- [ ] All tasks complete
- [ ] `npm run build` → dist에 ESM+CJS+`.d.ts` 산출, `npm pack --dry-run` 클린
- [ ] 커버리지 line/func ≥80%, no-secret-in-logs 가드 green
- [ ] Next.js Route Handler/Server Component 사용 예제 README 존재 + 컴파일
- [ ] webhook-verify 유효/변조/replay 케이스 green (signer.go 서명 스킴과 동치)
- [ ] **`get(key)`=단일-키 엔드포인트만 사용, whole-env는 `getAll()`/`preload()` opt-in, per-key TTL (Codex F2)**
- [ ] **`timeoutMs`+`AbortSignal` 취소 + 실패 flight 정리(다음 호출 정상 재fetch) 테스트 green (Codex F1)**
- [ ] **Go signer golden 벡터 + envelope fixture 계약 테스트 green, live 스모크 report 1회 (Codex F3)**
- [ ] **`sdk.yml` required PR check 등록, `make test`엔 미포함(Node-optional 보존) (Codex F4)**
- [ ] `make sdk` green, `go build ./...` 회귀 없음
- [ ] 패턴 미러(client.go/response.go/signer.go), 재발명 아님

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2)
- 라운드 수: 1 (R1에서 4 findings 전부 계획 수정으로 흡수 → R2 불필요)
- 합치 결론: `needs-attention` — SDK가 (1) fetch stall을 전 요청 장애로 증폭, (2) 단일 키 조회를 env 전체 평문 적재로 확대, (3) mocked-only 테스트로 서버 계약 드리프트 미검출할 수 있다는 3축 지적. 모두 수용해 D5/D8/D9 + Task 3/5/6 + acceptance에 반영.
- YAGNI Triage:
  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | F1 single-flight timeout/abort 부재 | HIGH | ACCEPT_NOW | production SDK 필수. `client.go:51`의 10s timeout을 캐시/single-flight까지 확장 → D8 + Task 3 |
  | F2 단일 키가 whole-env 적재로 확대 | HIGH | ACCEPT_NOW | 시크릿 blast radius. 서버에 단일-키 엔드포인트 존재(`client.go:264`) → D5 `get(key)` 기본 단일-키 |
  | F3 mocked-only가 계약 드리프트 미검출 | MEDIUM | ACCEPT_NOW (scoped) | Go signer golden 벡터 + envelope fixture를 required CI로, live 스모크 필수 승격 → Task 5/6 |
  | F4 SDK가 repo gate 밖의 섬 | MEDIUM | ACCEPT_NOW (partial) | `sdk.yml` required check로 잠금(D9). 단 `make test` fold-in은 Node-optional 속성(Makefile:24-26) 위배라 거부 |
- Deferred to backlog: 1 → `.claude/plans/codex-findings-backlog.md` (F3 잔여: 전체 CI 서버-기동 계약 매트릭스)
- Open Questions: 없음 (auto-CRITICAL 0 — 최고 severity HIGH, 모두 흡수)
- Codex session 참조: threadId `019f2b26-cb2d-7a61-95bd-784b94f84455`

## Codex Implementation Review

decision-set already converged in mccp-plan-codex review. No new implement-time decisions detected. Cross-gate dedupe applied.
