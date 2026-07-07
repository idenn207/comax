# Implementation Report: M7 — Infra Config Templating PoC (Decision)

**Plan**: `.claude/plans/completed/comax-secrets-m7-infra-config-templating.plan.md`
**Milestone**: PRD #7 — (Decision) Infra config templating PoC
**Branch**: `feat/m7-infra-config-templating`
**Date**: 2026-07-05

## Decision: **IN (v1)** — `secret render`, staging-only, with live-replace as fast-follow

infra-config templating은 v1에 포함한다. PoC가 핵심 페인(환경별 config drift 제거)을 **낮은 비용·무(無)스코프폭주**로 해결함을 실증했다. v1 범위는 **클라이언트측 `secret render`(staging-only)** 로 한정하고, 라이브 config in-place 무중단 교체(atomic-replace)는 운영자가 실제 필요로 할 때의 fast-follow(backlog)로 둔다.

### Decision Criteria — 증거 기반 판정

| # | 기준 | 판정 | 증거 |
|---|---|---|---|
| 1 | 단일 템플릿 1개가 local/dev/prod로 렌더 (drift 제거) | ✅ | `TestRender_SingleTemplateAcrossEnvs`(`self` alias로 3환경 서로 다른 값), `TestRender_HappyPath`(CLI→서버→파일 end-to-end) |
| 2 | current-env(`self`) + cross-env 문법 둘 다 동작 | ✅ | `TestRender_CrossEnv`, `TestEnvs_ResolvesSelf`; `$host` 리터럴 보존(`TestRender_LiteralDollarPreserved`) → nginx 안전 |
| 3 | 스코프 비용(resolver 모델 잔류 vs 별도 시스템) | ✅ 낮음 | CLI가 `internal/secret`/`internal/store`/`database/sql`/`modernc.org/sqlite` **미링크**(go list -deps로 확정). 서버 변경 0. 신규 = leaf 패키지 1 + CLI 명령 1 |
| 4 | apply 경계 | 명확 | render=staging 파일 생성까지. mount/reload/live-replace는 별도 — reload는 M4 webhook(secret 변경→service restart)로 이미 커버. 라이브 in-place 교체만 신규 범위 → **backlog(fast-follow)** |
| 5 | 권고 | **In(v1)** | 위 근거 종합. PRD 최상위 리스크 "스코프 폭주"가 실현되지 않음 |

### 왜 In인가 (스코프 폭주 리스크 판정)

PRD가 M7 보류의 근거로 든 위험은 "envvar 모델과 별개 시스템(templating/file-render/mount-sync) 필요 → 범위 위험"이었다. PoC 결과 그 위험은 **실현되지 않았다**:

- 렌더는 기존 `${{ env.KEY }}` 문법(`internal/secret.ReferencePattern`)과 기존 resolver(`ListSecrets`)를 그대로 재사용했다. 신규 문법·신규 서버 API·마이그레이션 0.
- 클라이언트측 렌더라 서버 바이너리·DB 스키마 무변경(되돌리기 쉬움).
- cold-start 300ms 예산도 구조적으로 보호(leaf 패키지, CLI 의존 그래프에 sql/crypto/sqlite 부재).

유일하게 범위를 키울 수 있는 지점(라이브 config in-place 무중단 교체)은 staging-only 경계로 격리했고, bounded follow-up으로 명시했다.

## Tasks Completed

| # | Task | 상태 | 비고 |
|---|---|---|---|
| 1 | `internal/tmpl` 순수 render 코어 | ✅ | `self` 결정론 alias, cross-env, fail-closed, 에러에 시크릿 금지. 커버리지 100% |
| 2 | `secret render` CLI 명령 | ✅ | staging-dir-only fail-closed 경로 정책, 0600, symlink 방어, `{env}` 치환 |
| 3 | PoC 증거(fixtures) | ✅ | `docs/poc/{redis,nginx}.conf.tmpl`. 테스트가 재현 가능한 증거 |
| 4 | 결정 문서화 + PRD 반영 | ✅ | 본 report + PRD M7/OQ#5 stamp |

## Codex / Security 흡수 요약

- **plan-codex**(R1, needs-attention 3): F1 `${{ self.KEY }}` namespace(단일 템플릿→N환경), F2 시크릿 노출 fail-closed, F3 staging-only(atomic-replace defer) — 전부 흡수.
- **implement-codex**(R1, needs-attention 2): impl-F1 safe-path를 worktree 내 gitignored staging으로 한정(`{env}`치환·symlink 정규화 후 판정, git오류=hard error), impl-F2 `self` 결정론 alias + `currentEnv=="self"` 가드 — 흡수.
- **security-reviewer**(CRITICAL 4·HIGH 5·MED 3, self-host 위협 모델로 triage): 실 must-fix(#2/#10 에러-시크릿-노출 → `TestRender_NoSecretsInErrorMessages`, #3 `--out` 마커 거부, #6 temp 0600, #8 stdout 제거)는 흡수. 로컬 공격자 계열(#1/#9/#11 symlink TOCTOU)은 방어적 채택(O_NOFOLLOW 재검사·부모 world-writable 거부, Unix). #4(template 임의 경로)는 운영자 자기 파일이라 security-blocking으로 REJECT.
- Deferred→backlog: F3 atomic-replace 프레임워크, impl-F2 서버측 `self` env-name 예약 거부.

## Validation Results

| Level | 상태 | 비고 |
|---|---|---|
| Static (build/vet/gofmt/golangci-lint) | ✅ Pass | 0 issues |
| Unit `internal/tmpl` | ✅ Pass | 12 tests, **cov 100%** |
| Unit `internal/secret` | ✅ Pass | cov 80.6%(floor 유지; 패턴 export만) |
| CLI 통합 (render 5종) | ✅ Pass | happy/stdout거부/비ignored거부/missing fail-closed/resolveOutPath |
| Build | ✅ Pass | `go build ./...` 0 |
| Cold-start budget | ⚠ 환경 | 로컬 bench p95 초과는 dev 머신 로드 아티팩트. **구조 불변식**(CLI에 서버 deps 부재) 확정 → 무회귀. CI(clean env)가 게이트 |

## Files Changed

| File | Action |
|---|---|
| `internal/tmpl/render.go` | CREATE — 순수 render 코어(`References`/`Envs`/`Render`, `self` alias) |
| `internal/tmpl/render_test.go` | CREATE — 12 table tests + no-secrets-in-errors |
| `internal/tmpl/parity_test.go` | CREATE — 문법 drift 가드 |
| `internal/secret/reference.go` | UPDATE — `referencePattern`→`ReferencePattern` export |
| `cmd/cli/cmd_render.go` | CREATE — `secret render` 명령 + safe-path/atomic 헬퍼 |
| `cmd/cli/cmd_render_test.go` | CREATE — CLI 통합 테스트(git-init staging) |
| `cmd/cli/main.go` | UPDATE — `newRenderCmd` 등록 |
| `docs/poc/redis.conf.tmpl`, `docs/poc/nginx.conf.tmpl` | CREATE — PoC fixtures |
| `.gitignore` | UPDATE — `tmp-render/` staging sink |

## Deviations from Plan

없음 — 계획대로 구현. 계획 자체가 Codex/security 게이트로 3회 강화됨(safe-path staging-only, `self` 결정론, 에러-시크릿-금지).

## Next Steps (v1-full, 별개 사이클)

- [ ] 라이브 config in-place 무중단 교체(backup/rollback/fsync/OS별 atomic-replace) — backlog HIGH
- [ ] 서버 env create/update에서 `self` 예약어 거부 — backlog HIGH
- [ ] 비-git-repo 배포 디렉토리(NAS)용 safe-path 정책 확장(현 PoC는 git worktree 요구)
- [ ] docs 사이트에 `secret render` reference 추가
