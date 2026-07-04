# Implementation Report: M5 — Node/TS SDK + npm publish

**Plan**: `.claude/plans/comax-secrets-m5-node-ts-sdk.plan.md`
**Branch**: `feat/comax-secrets-m5-node-ts-sdk`
**Date**: 2026-07-04

## Summary

`@comax-secrets/sdk` 패키지를 `sdk/`에 신설했다. Next.js/Node 18+/Edge 앱이 런타임에 시크릿을 fetch → 캐시 → reload 하는 zero-dep 단일 패키지이며, 기존 Go 서버의 HTTP 표면(고정 envelope·해석완료 `Secret`·M3 Bearer 서비스 토큰)과 M4 웹훅 서명(`signer.go`)을 소비하는 클라이언트 계층이다. 신규 서버 기능은 없다.

## Assessment vs Reality

| Metric | Plan | Actual |
|---|---|---|
| Complexity | Large | Large (일치) |
| Files created | ~15 | 20 created + 5 modified |
| Codex 게이트 | plan-codex(needs-attention 4건) → 전부 흡수, implement cross-gate dedupe | 일치 |
| Coverage 목표 | ≥80% line/func | 95.71% line / 100% func / 87.59% branch |

## Tasks Completed

| # | Task | Status | Notes |
|---|---|---|---|
| 1 | 패키지 스캐폴딩 | ✅ | package.json(exports 맵)·tsconfig·tsup·vitest·eslint·prettier·gitignore |
| 2 | 에러 타입 + HTTP 코어 | ✅ | `errors.ts`·`http.ts` — `client.go do()` 미러, envelope→타입 에러 |
| 3 | SecretsClient (캐시·취소) | ✅ | 단일-키 기본(F2), timeout+AbortSignal+실패 flight 정리(F1), single-flight |
| 4 | 팩토리 + 환경변수 | ✅ | `createClient`/`createClientFromEnv`, README(Next.js/Edge/singleton) |
| 5 | Webhook 검증 서브모듈 | ✅ | `webhook.ts`(`./webhook`), Web Crypto 상수시간, Go signer golden 벡터 |
| 6 | Build·CI 배선 | ✅ | Makefile `sdk` 타깃, `ci.yml` sdk 잡(required), `sdk-publish.yml`(provenance) |
| 7 | 커버리지 + 시크릿 가드 | ✅ | 95.71% cov, `no-secret-leak.test.ts`(에러/console 누출 0) |
| 8 | PRD/plan 부기 | ✅ | PRD M5 complete + report 링크 (plan 아카이브는 연기 — Deviations 참조) |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static (typecheck) | ✅ Pass | `tsc --noEmit` 0 errors (strict + noUncheckedIndexedAccess) |
| Static (lint) | ✅ Pass | `eslint .` 0 warnings |
| Unit Tests | ✅ Pass | 40 tests / 5 files green |
| Coverage | ✅ Pass | 95.71% line, 100% func, 87.59% branch (bar 80/80/75) |
| Build | ✅ Pass | tsup → dist {index,webhook} × {js,cjs,d.ts,d.cts} |
| Dual-format smoke | ✅ Pass | ESM `import` + CJS `require` 양쪽 export 해석 |
| Pack dry-run | ✅ Pass | 10 files, 13.6kB — dist+README+package.json만(소스·소스맵·시크릿 누출 0) |
| Integration (live smoke) | ✅ Pass | 실 Go 서버 상대: get/getAll/has, 잘못된 토큰→ComaxAuthError(401), 없는 키→ComaxNotFoundError |

### Design Grounding

N/A — 디자인 트리거 미발동(SDK는 UI 아님, `impeccable-detect` design_signal=false).

## Files Changed

**Created (20)**: `sdk/{package.json,package-lock.json,tsconfig.json,tsup.config.ts,vitest.config.ts,eslint.config.js,.prettierrc.json,.gitignore,README.md}`, `sdk/src/{errors,http,secrets,index,webhook}.ts`, `sdk/src/{http,secrets,index,webhook,no-secret-leak}.test.ts`, `sdk/test/fixtures/webhook-vectors.json`, `.github/workflows/sdk-publish.yml`, `.claude/plans/comax-secrets-m5-node-ts-sdk.plan.md`, 본 report.

**Modified (5)**: `Makefile`(sdk 타깃), `.github/workflows/ci.yml`(sdk 잡), `.claude/prds/comax-secrets.prd.md`(M5 complete), `.claude/plans/codex-findings-backlog.md`(F3 잔여), `.claude/state/STATE.md`.

## Codex Findings Absorption (plan-codex, needs-attention)

| Finding | Sev | 반영 |
|---|---|---|
| F1 single-flight timeout/abort 부재 | HIGH | D8 — `timeoutMs`(10s) + `AbortSignal` + 실패 flight `finally` 정리. `secrets.test.ts` 정리 테스트 green |
| F2 단일 키가 whole-env 적재로 확대 | HIGH | D5 — `get(key)` 단일-키 엔드포인트 기본, whole-env는 `getAll()`/`preload()` opt-in |
| F3 mocked-only 계약 드리프트 | MED | Go signer golden 벡터 + live 스모크(필수). 전체 CI 서버-매트릭스는 backlog |
| F4 SDK가 repo gate 밖 | MED | `ci.yml` sdk 잡 required. `make test` fold-in은 Node-optional 보존 위해 거부 |

## Post-implementation Fix (Codex stop-time review)

- **reload-race (correctness)**: `reload()`가 in-flight fetch가 나중에 완료되며 `cache.set`으로 stale 값을 되살릴 수 있었음(웹훅 기반 reload 시나리오의 TOCTOU). 세대(epoch) 카운터 도입 — fetch는 시작 시 epoch 캡처, `reload()`는 epoch 증가 + in-flight 슬롯 정리, fetch 완료 시 epoch 불변일 때만 캐시 기록. in-flight 맵은 identity-guard 정리로 post-reload 재요청 clobber 방지. 회귀 테스트 추가(`secrets.test.ts`, 41 tests). typecheck 0.

## Deviations from Plan

1. **CI 파일 구조**: 계획의 단일 `sdk.yml` 대신, 저장소 관행(모든 품질 게이트가 `ci.yml`의 잡)에 맞춰 `ci.yml`에 `sdk` 잡을 추가하고 publish는 `sdk-publish.yml`로 분리. 기능 동일, 위치만 관행 정렬.
2. **plan 아카이브 연기**: mccp Phase 5는 plan을 `completed/`로 이동하나, receipt `plan_hash`가 경로 기준이라 남은 auto-chain(commit→PR) 재검증이 깨질 위험이 있어 **PR 머지 후 housekeeping으로 연기**(M2 closure와 동일).

## Issues Encountered

- **devDep 취약점 6건**(3 moderate/1 high/2 critical): 전부 빌드 툴체인(tsup/esbuild/vitest 트랜지티브). **SDK 런타임 의존성은 0**이라 published 패키지 소비자 미영향. `audit fix --force`는 툴체인 파손 위험으로 미적용 — 툴 버전 상향 시 재검토.
- implement 게이트가 dedupe 섹션을 plan에 주석하면서 plan-codex `plan_hash`가 드리프트 → plan-codex receipt를 현재 해시로 re-stamp해 해소(정상 흐름).

## Tests Written

| Test File | Tests | Coverage |
|---|---|---|
| `src/http.test.ts` | 12 | envelope·에러 매핑·timeout·network·bearer |
| `src/secrets.test.ts` | 11 | 단일-키·캐시·TTL·single-flight·flight 정리·getAll·auto-refresh |
| `src/index.test.ts` | 5 | 팩토리·env 해석·override |
| `src/webhook.test.ts` | 10 | Go golden 벡터 3 + 변조/replay/tolerance/누락 |
| `src/no-secret-leak.test.ts` | 2 | 에러·console에 토큰/시크릿 누출 0 |

## Next Steps

- [ ] `/mccp:prp-commit` → `/mccp:pr` (auto-chain)
- [ ] PR 머지 후: plan을 `.claude/plans/completed/`로 아카이브 + PRD Plan 셀 경로 갱신
- [ ] npm 발행: `@comax-secrets` 스코프 소유 확인 후 `sdk-publish.yml` operator dispatch (D7)
