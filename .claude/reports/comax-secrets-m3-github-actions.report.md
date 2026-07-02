# 구현 리포트: GitHub Actions 통합 (M3)

**Plan**: `.claude/plans/comax-secrets-m3-github-actions.plan.md`
**Branch**: `feat/comax-secrets-m3-github-actions`
**Source PRD**: `.claude/prds/comax-secrets.prd.md` (Milestone #3)

## 요약

셀프호스트 서버의 시크릿을 GitHub Actions에 주입하는 composite action을 구현했다.
Codex R1/R2 적대 검토를 코드로 흡수: (1) 토큰 발급 **admin-only** + `is_admin`
migration, (2) `revoked_at` **soft revoke**(bearer·대시보드 세션 양쪽 차단),
(3) 주입 기본 **process-env**(자식 프로세스만) + **github-env** opt-in(job 전역),
(4) CI 자격증명 **RUNNER_TEMP one-shot + always() cleanup**, (5) smoke의 **비-echo
canary + 로그 평문 부재 grep**. 백엔드에 더해 대시보드 **Tokens 관리 화면**과
**GitHub Actions 레퍼런스 화면**을 편입했다.

## 예측 대비 실제

| 지표 | Plan 예측 | 실제 |
|---|---|---|
| Complexity | Medium-Large | Medium-Large (일치) |
| Files Changed | ~30 | 30 (신규 17 + 수정 13, gofmt 잔여 8 제외) |
| 신규 패키지 | `internal/cli/ghenv` | 동일, 커버리지 81.8% |

## 완료된 태스크

| # | 태스크 | 상태 | 비고 |
|---|---|---|---|
| 1 | 스키마 + migration (is_admin, revoked_at) | ✅ | admin backfill 멱등성 테스트 포함 |
| 2 | TokenRepo admin/revoke | ✅ | ByHash 필터 / ByID 노출 비대칭(R2-1) |
| 3 | 토큰 핸들러 + 라우트 + revoke 세션 차단 | ✅ | R2-1 authSession RevokedAt 401 |
| 4 | client 토큰 메서드 | ✅ | CLI 테스트로 간접 커버 |
| 5 | loadContext --project override | ✅ | flag 자동감지로 무-호출부-churn |
| 6 | internal/cli/ghenv 방출기 | ✅ | heredoc 충돌검사 + 줄단위 마스킹 |
| 7 | secret export | ✅ | dotenv/json/github-env |
| 8 | secret token 서브커맨드 | ✅ | create 1회노출 / list / revoke |
| 9 | secret run --project | ✅ | Task 5와 함께 구현 |
| 10 | composite action.yml | ✅ | 입력 전부 env 전달(injection 방지) |
| 11 | action-smoke.yml | ✅ | **로컬 통합 스모크로 사전 검증** |
| 12 | 문서 + PRD | ✅ | github-actions.md, threat-model M3절 |
| 13 | 대시보드 Tokens 화면 | ✅ | 1회노출 dialog + 403 admin-only 안내 |
| 14 | 대시보드 Actions 화면 | ✅ | 복사가능 YAML 스니펫 |
| 15 | 대시보드 nav + 라우팅 + e2e | ✅ | 연동/설정 nav + axe 2 라우트 |

## 검증 결과

| 레벨 | 상태 | 비고 |
|---|---|---|
| Go build / vet | ✅ | `go build ./...`, `go vet ./...` |
| Go 단위 테스트 | ✅ | 전 패키지 green (로컬은 `-race` 불가 — cgo 미지원, CI에 위임) |
| golangci-lint | ✅ | 0 issues (gosec G703 = 정당화 억제) |
| ghenv 커버리지 | ✅ | 81.8% (≥80%) |
| 대시보드 typecheck/lint/test/build | ✅ | vitest 128 통과, gzip 예산 이내 |
| 로컬 통합 스모크 | ✅ | server→bootstrap→token→run canary→export→revoke |
| action-smoke (CI) | ⏳ | GitHub Actions 러너 필요 (uses:./ · $GITHUB_ENV · ::add-mask::) |
| dashboard-e2e axe (CI) | ⏳ | Playwright 러너 필요 |

### 디자인 게이트

impeccable `critique`를 세션에서 호출(impeccable-guard 통과). UI는 새 디자인이
아니라 SessionRow/.sessions-table·Radix·PageHeader/AppShell **재사용**이며,
DESIGN.md 원칙(색은 의미에만·정직한 라벨·라벨로 상태 전달) 적용. mccp
design-grounding capture는 cross-gate dedupe로 skip됨(Phase 3.6/3.7 no-op).

## 계획 대비 편차

- **[minor] 대시보드 fetcher/타입 위치**: plan Files-to-Change는 `lib/api.ts`를
  지목했으나, 코드베이스 컨벤션상 resource fetcher는 `lib/queries.ts`, 타입은
  `lib/types.ts`에 산다(`api.ts`는 저수준 fetch seam). 컨벤션을 따라
  queries.ts + types.ts에 추가. 아키텍처(fetcher+타입+page+row)는 계획대로.
- **[minor] loadContext 시그니처 무변경**: param 추가 대신 `cmd.Flags().Lookup`
  로 선택적 --project를 자동감지 — diff/getset/push 호출부를 건드리지 않아
  Files-to-Change ⊆ 유지.
- **[운영] plan 물리 archive 보류**: 영수증(plan-codex/implement-codex)이 현재
  plan 경로를 앵커하므로, PR 게이트 chain 보전을 위해 completed/ 이동은
  머지 후로 이연.

## 겪은 이슈 / 해결

- **토큰 캡처 버그(사전 발견)**: smoke의 `grep -A1 'bootstrap admin token'`이
  stdout 배너 + stderr slog 두 줄 매칭 → `tail -1`이 오탐. 배너 전용 `shown ONCE`
  로 유일 매칭 수정. **로컬 스모크가 CI 전에 잡음.**
- **전체 트리 gofmt 실수**: `gofmt -w`를 트리 전체에 돌려 M3 무관 8개 파일에
  정렬 변경 유입. HEAD 복원은 GateGuard가 hard-block → **커밋 시 선택적 staging**
  으로 제외 예정(아래 Next Steps).
- **jsdom 204/무-href 링크**: 테스트에서 `new Response(null,{status:204})`,
  링크는 `getByText`+`to` 속성으로 조정.

## 작성한 테스트

| 파일 | 성격 |
|---|---|
| `internal/store/migrate_test.go` | upgrade admin backfill + 멱등성 |
| `internal/store/token_repo_test.go` | admin/revoke/List/ByID-sees-revoked |
| `internal/server/handlers_tokens_test.go` | 403·1회노출·revoke·**R2-1 세션 종료**·감사 |
| `internal/cli/ghenv/ghenv_test.go` | 마스킹·DELIM 충돌·순서 (81.8%) |
| `cmd/cli/cli_loadctx_test.go` | --project override/부재/에러 |
| `cmd/cli/cmd_export_test.go` | 포맷별·$GITHUB_ENV 미설정·마스크 |
| `cmd/cli/cmd_token_test.go` | create/list/revoke |
| `web/dashboard/src/pages/Tokens.test.tsx` | 목록·403·1회노출·회수 |
| `web/dashboard/src/pages/Actions.test.tsx` | 헤딩·스니펫·링크·한계 |

## Next Steps

- [ ] 커밋: **선택적 staging** — M3 파일만 add, gofmt 잔여 8개(provider.go,
      handlers_envs.go, handlers_m2_test.go, handlers_spa_test.go, server_test.go,
      session_repo.go, store_test.go, dotenv_test.go)는 제외.
- [ ] `/mccp:pr` 로 PR 생성 (plan-codex + implement-codex 영수증 chain 유효).
- [ ] CI에서 action-smoke(M3 acceptance) + dashboard-e2e(axe) green 확인.
- [ ] 머지 후 plan을 `.claude/plans/completed/`로 archive + PRD 행 done 전환.
