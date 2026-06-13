# Local Review: dashboard-ui — M2 마감 정리 변경분

**Reviewed**: 2026-06-01
**Mode**: Local (uncommitted changes on `feat/dashboard-ui`)
**Branch**: `feat/dashboard-ui` (HEAD는 이미 master에 머지된 `eb2d340` — 이번 변경은 그 위에 쌓인 uncommitted 작업)
**Worktree**: `c:\_project\my\comax\.worktrees\dashboard-ui`
**Reviewer**: Claude (Opus 4.7)
**Decision**: **REQUEST CHANGES** — HIGH 1건 + MEDIUM 1건 수정 후 머지 권장.

---

## Summary

이번 변경분은 신규 기능 코드가 아니라 **M2(대시보드) 마감 정리 작업**입니다 — plan 파일 아카이브, 새 운영자 문서(`docs/dashboard.md`) 추가, README/quickstart/threat-model/perf/dogfood 갱신, CI에 dashboard 빌드·페이로드 사이즈 게이트·compose smoke 도입, Dockerfile 멀티스테이지 빌드 도입(`dashboard-builder` → `go builder`), `pnpm-workspace.yaml` pnpm 11.5 호환 키 추가.

정적 코드 결함은 없습니다. 핵심 결함은 (1) plan 파일을 `.claude/plans/completed/`로 옮겼는데 **참조 4곳을 갱신하지 않은 broken link**, (2) `pnpm-workspace.yaml`에 **placeholder 텍스트 잔존** 두 가지입니다. 두 건 모두 30분 안에 수정 가능합니다.

CI 측 새 게이트(JS 400KB / CSS 100KB gzip / 바이너리 25MB / docker-compose ≤120s healthy + SPA reachable)는 설계가 견고하고, `set -euo pipefail` + 글로브 가드 + `working-directory: web/dashboard` defaults 까지 일관되게 잡혀 있어 그대로 통과시킬 만합니다.

---

## Findings

### CRITICAL

없음.

### HIGH

#### H1. plan 파일을 `completed/`로 옮겼는데 참조 4곳이 옛 경로 그대로 — broken link

**증상**

- 변경:
  - `D  .claude/plans/comax-secrets-dashboard.plan.md`
  - `?? .claude/plans/completed/comax-secrets-dashboard.plan.md`
- working tree의 `.claude/plans/` 디렉토리에는 `completed/` 하나만 남음. `comax-secrets.plan.md`도 이전 작업에서 이미 `completed/`로 이동된 상태(git ls-files 확인).
- 그러나 다음 4곳의 마크다운 링크는 모두 **옛 경로**를 가리켜 깨짐:

| 위치 | 링크 텍스트 | 실제 파일 |
|---|---|---|
| `README.md:13` | `.claude/plans/comax-secrets.plan.md` (M1 plan) | `.claude/plans/completed/comax-secrets.plan.md` |
| `README.md:13` | `.claude/plans/comax-secrets-dashboard.plan.md` (M2 plan) | `.claude/plans/completed/comax-secrets-dashboard.plan.md` |
| `.claude/prds/comax-secrets.prd.md:133` | `../plans/comax-secrets-dashboard.plan.md` | `../plans/completed/comax-secrets-dashboard.plan.md` |
| `.claude/reports/comax-secrets-dashboard-task-10-11.report.md:3` | `../plans/comax-secrets-dashboard.plan.md` | `../plans/completed/comax-secrets-dashboard.plan.md` |

**영향**

README는 사용자가 가장 먼저 보는 surface입니다. M2 마감 커밋에서 navigation 그래프가 깨지면 dogfood/onboarding 경험이 곧장 무너지고, 후속 작업자가 "plan이 사라진 것 같다"고 오해할 수 있습니다.

**수정안 (택1)**

1. **(권장)** 4곳의 참조를 모두 `.claude/plans/completed/...`로 갱신.
2. plan을 `completed/`로 옮기지 않고 파일 상단에 "✅ Closed (2026-06-01)" 헤더만 추가. (이 경우 변경 자체를 되돌려야 함.)

### MEDIUM

#### M1. `web/dashboard/pnpm-workspace.yaml` placeholder 텍스트 잔존

**증상**

현재 파일:

```yaml
allowBuilds:
  esbuild: set this to true or false   # ← placeholder 그대로
# Not a workspace; this file exists because pnpm 10+ requires explicit
# opt-in for transitive postinstall scripts. esbuild ships its native
# binary via postinstall and Vite refuses to build without it.
#
# Key name moved in pnpm 11.5: the legacy `allowBuilds:` map is no
# longer read; the supported setting lives under `onlyBuiltDependencies:`
# in pnpm-workspace.yaml (per https://pnpm.io/settings). Same intent,
# new spelling — esbuild is the only allowed builder.
onlyBuiltDependencies:
  - esbuild
```

**검증**

- 실제로 worktree에서 `pnpm install --frozen-lockfile` (pnpm 11.5.0) 실행 → 경고/에러 없이 통과 (`Done in 241ms`).
- pnpm 11.5는 legacy `allowBuilds:` 키를 **조용히 무시**합니다. 즉 placeholder 값 자체는 동작에 영향 없음.

**문제**

- "set this to true or false"는 누군가 채워야 할 자리표시자처럼 보이고, 후임자가 "이거 어디 값 채워야 하지?"라고 오해할 수 있습니다.
- legacy 키 블록 + 그 위에 "이 키는 더 이상 읽히지 않는다"는 주석이 동시에 존재해 의도가 모순적으로 읽힙니다.

**수정안**

legacy `allowBuilds:` 블록(2줄) 삭제. 주석에서 "Key name moved..." 부분만 남겨 마이그레이션 history를 기록.

```yaml
# Not a workspace; this file exists because pnpm 10+ requires explicit
# opt-in for transitive postinstall scripts. esbuild ships its native
# binary via postinstall and Vite refuses to build without it.
#
# Key name in pnpm 11.5+ is `onlyBuiltDependencies:` — legacy
# `allowBuilds:` map (used in pnpm 10) is no longer read.
# See https://pnpm.io/settings
onlyBuiltDependencies:
  - esbuild
```

### LOW

#### L1. CI 매트릭스에서 vite build 4회 중복 실행

`.github/workflows/ci.yml`의 `test` job이 GOOS×GOARCH 4셀(linux/amd64, linux/arm64, linux/arm, darwin/amd64) 각각에서 `pnpm install + pnpm build`를 다시 실행합니다. pnpm/node 캐시가 잘 잡혀 실측 비용은 크지 않지만, 정석은 별도 `dashboard-bundle` job에서 한 번 빌드 후 `actions/upload-artifact`로 전달하는 패턴입니다.

**결정에는 영향 없음** — 향후 빌드 시간이 신경 쓰일 때 분리해도 충분합니다.

#### L2. Dockerfile `dashboard-builder`의 install 잠재 재트리거

`deploy/docker/Dockerfile`의 `dashboard-builder` 스테이지에서 `COPY web/dashboard ./`가 `pnpm install` 다음에 와서 `package.json/pnpm-lock.yaml`도 다시 덮어씌웁니다. `pnpm build`가 무결성 재검사를 트리거할 수 있으나 `CI=true` + `PNPM_CONFIG_DANGEROUSLY_ALLOW_ALL_BUILDS=true`로 silent OK.

가독성 차원에서 install을 src copy 뒤로 일원화할 수도 있으나, src-only 변경 시 install layer 캐시가 무효화되는 트레이드오프가 있습니다. **현 구조가 캐시 효율 우선 — 그대로 둬도 OK.**

---

## Validation Results

| Check | Result | Note |
|---|---|---|
| `pnpm install --frozen-lockfile` | ✅ Pass | `Done in 241ms`, pnpm 11.5.0 |
| `git diff --check` (whitespace) | ✅ Pass | LF/CRLF 경고는 `web/dashboard/pnpm-workspace.yaml`만 — Windows 작업 환경 영향 |
| typecheck / lint / vitest / build | ⏭ Skipped | 변경 파일에 src 없음. CI 게이트에서 다시 실행됨 |
| Go vet / test / build | ⏭ Skipped | `.go` 파일 변경 없음 |

CI 게이트 자체 검토:

- `defaults.run.working-directory: web/dashboard` job-level로 잡혀서 `DIST="../../internal/server/dashboard/dist/assets"` 상대경로가 정확히 repo root의 embed 디렉토리를 가리킴.
- `set -euo pipefail` + `[ -f "$f" ] || continue` 글로브 가드로 산출물 0개일 때 silent pass 아니고 명시적 0 B vs budget 비교.
- `compose-smoke` job: `mkdir -p deploy/compose/data deploy/compose/keys` → `up -d` 순서 정확. 2초 간격 60회 폴링으로 docker-compose.yml의 10s healthcheck interval에 종속되지 않음. SPA reachable 체크는 `<div id="root"` + `/assets/index-` 두 가지를 동시에 검증해서 embed FS resolve가 실제로 SPA bundle로 갔는지 확인.
- 바이너리 size budget은 `matrix.target.goarch == 'amd64'` 조건으로만 실행 — arm/arm64 단면 차이를 의도적으로 분리한 점, 가드가 적절.

---

## Files Reviewed

| 파일 | 변경 | 평가 |
|---|---|---|
| `.github/workflows/ci.yml` | Modified | 신규 dashboard/size-budget/compose-smoke job 정확하고 안전 |
| `deploy/docker/Dockerfile` | Modified | 멀티스테이지 의도/주석 명확, vite output 경로 일관, ENV 가드 명시적 |
| `README.md` | Modified | M2 status 갱신 OK, **plan 링크 2건 깨짐 (H1)** |
| `docs/dashboard.md` | Added | 운영자 가이드 완결. auth flow, disable flag, threat-model 요약, 예산, dogfood 포인터 일관 |
| `docs/dogfood.md` | Modified | M2 click/time budget 측정 가능한 체크리스트 형태로 구체적 |
| `docs/perf.md` | Modified | CSS 50→100KB 상향 근거(Radix Themes 정적 CSS, 813KB unminified) 명시 — 굿 |
| `docs/quickstart.md` | Modified | `/login` 안내 + 보안 속성(HttpOnly/Secure/SameSite=Strict/CSRF) 한 줄 요약 |
| `docs/threat-model.md` | Modified | Cookie/CSRF/CSP/세션 lifetime/Logout 5포인트 정리 — 누락 없음 |
| `web/dashboard/pnpm-workspace.yaml` | Modified | **M1 — placeholder 정리 필요** |
| `.claude/plans/comax-secrets-dashboard.plan.md` | Deleted | 이동 자체는 OK, 참조 갱신 누락 (H1) |
| `.claude/plans/completed/comax-secrets-dashboard.plan.md` | Added (untracked) | 새 위치 |

---

## Next Steps

권장 순서:

1. **H1 해결** — 아래 3파일에서 plan 경로 일괄 치환:
   - `README.md` (2곳: `comax-secrets.plan.md`, `comax-secrets-dashboard.plan.md`)
   - `.claude/prds/comax-secrets.prd.md:133`
   - `.claude/reports/comax-secrets-dashboard-task-10-11.report.md:3`
   - 치환 규칙: `.claude/plans/comax-secrets.plan.md` → `.claude/plans/completed/comax-secrets.plan.md`, `.claude/plans/comax-secrets-dashboard.plan.md` → `.claude/plans/completed/comax-secrets-dashboard.plan.md` (상대 경로 prefix는 위치별로 다름).
2. **M1 해결** — `web/dashboard/pnpm-workspace.yaml`에서 legacy `allowBuilds:` 블록 2줄 삭제 + 주석 정리.
3. 위 2건만 마치면 **APPROVE** 가능. L1/L2는 별도 maintenance 백로그로 적절.

수정 후 권장 검증 명령:

```bash
# H1 검증 — 옛 경로 잔존 없는지 확인
grep -rn 'plans/comax-secrets\(-dashboard\)\?\.plan\.md' . \
  --include='*.md' \
  | grep -v completed/

# M1 검증 — pnpm install 정상 동작 + workspace yaml lint
cd web/dashboard && pnpm install --frozen-lockfile
```
