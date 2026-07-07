# Plan: Comax Secrets M8 — Public Release (MIT)

**Source PRD**: `.claude/prds/comax-secrets.prd.md`
**Selected Milestone**: #8 Public release (MIT) — GitHub repo + npm 패키지 + docs site + GH Action marketplace 등록
**Complexity**: Medium–Large (릴리스 엔지니어링 중심 + `action.yml` 실코드 1건 + 신규 CI 워크플로 1건)

## Summary

M1–M7는 기능적으로 완성됐고 repo는 이미 `PUBLIC`이다. M8은 "공개하기"가 아니라 **이미 공개된 저장소를 신뢰할 수 있는 릴리스로 완성**하는 작업이다 — 이름 일관성 정정, MIT LICENSE + OSS 메타파일, 첫 `v*` 릴리스(크로스컴파일 바이너리 + 체크섬), 첫 npm publish, `action.yml`의 M8 예약 기능(`cli-version` 릴리스 다운로드 폴백 + Marketplace branding) 구현, install/download 문서, GH Action Marketplace 등록.

**릴리스 블로커 1건(선결)**: 실제 repo는 `github.com/idenn207/comax`인데 `go.mod` module·npm scope·코드/문서 링크는 전부 `comax-secrets`를 가리킨다. `go install`이 실패하고 GitHub이 라이선스를 인식하지 못한다(`licenseInfo:null`). Task 0에서 정정한다.

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| Workflow 네이밍 | `.github/workflows/{ci,sdk-publish,website}.yml` | kebab-case `name:`, `permissions: contents: read` 최소권한, push branches `[main, master, feat/**]` — release는 `on: push: tags: ['v*']` |
| 크로스컴파일 빌드 | `.github/workflows/ci.yml:77-127` (build matrix) | `GOOS/GOARCH/GOARM` env + `CGO_ENABLED=0` + `-trimpath -ldflags "-s -w"`; `internal/version.Version`는 `-X` ldflag 주입(`Makefile:41-42`) |
| 크로스컴파일 타깃 | `Makefile:127-131` (`xbuild`) | `linux/{amd64,arm64,arm-v7}`; 릴리스에서 darwin/windows 확장은 Task 4 결정 |
| 배포 안전장치 | `.github/workflows/sdk-publish.yml:49-59` | live publish ref-guard(`master` 또는 exact `vN.N.N`), `dry_run` 기본값, npm `--provenance`, `id-token: write` |
| Action 규약 | `action.yml` | `runs: using: composite`, 에러는 영어 `::error::`, 토큰은 env로만 전달(argv 금지) |
| 문서 드리프트 게이트 | `website/scripts/check-{docs-drift,docs-coverage,token-parity,site-url}.mjs` (`Makefile:110-118`) | 신규 docs는 이 4개 게이트를 통과해야 함 |
| 에러/라벨 언어 규약 | `CLAUDE.md` "작업 규칙" | 사용자 노출 에러 영어, UI 라벨/설명문 한국어 |

## Files to Change

| File | Action | Why |
|---|---|---|
| *(GitHub repo 설정)* `comax` → `comax-secrets` | RENAME | module path/npm scope/action ref와 일치. GitHub 자동 리다이렉트 |
| `go.mod` 외 module 경로 참조 | VERIFY | 이름 정정 후 `github.com/idenn207/comax-secrets`가 실제 repo와 일치하는지 확인(모듈 경로 자체는 이미 comax-secrets라 rename 채택 시 무변경) |
| `cmd/cli/main.go:43` / `internal/version/version.go:5` | UPDATE | 리다이렉트되는 repo URL 문자열 확인·정정(문맥상 comax-secrets 유지) |
| `LICENSE` | CREATE | MIT 전문(Constraints 고정). GitHub 라이선스 인식 + npm/action provenance 근거 |
| `README.md` | UPDATE | stale("M1–4 shipped") → M1–7 shipped + 설치 매트릭스(download/`go install`/docker) + docs site 링크 + 배지 + license/contributing/security 링크 |
| `SECURITY.md` | CREATE | 책임 있는 공개(연락처) + `docs/threat-model.md` 링크. 시크릿 도구라 필수 |
| `CONTRIBUTING.md` | CREATE | Go 1.25, `make build/test/lint`, 커버리지 플로어, PR 흐름 |
| `CODE_OF_CONDUCT.md` | CREATE | 표준 OSS 위생(Contributor Covenant) |
| `.github/ISSUE_TEMPLATE/{bug_report,feature_request}.yml` | CREATE | 이슈 트리아지 |
| `.github/PULL_REQUEST_TEMPLATE.md` | CREATE | PR 체크리스트 |
| `action.yml` | UPDATE | `cli-path` 필수→선택, `cli-version` 릴리스 다운로드 폴백 구현, `branding:` 추가(Marketplace 필수) |
| `.github/workflows/release.yml` | CREATE | `v*` 태그 → xbuild + 체크섬 + GitHub Release + 바이너리 업로드 + `v1` major-tag alias |
| `.github/workflows/action-smoke.yml` | UPDATE | `cli-version` 다운로드 경로 스모크 잡 추가(fixture/사후 실릴리스) |
| `.github/workflows/secret-scan.yml` | CREATE | gitleaks 이력/증분 스캔(이미 public → 유출 여부 검증 게이트) |
| `sdk/package.json` | UPDATE | version 정렬(0.1.0 → 릴리스 버전), 첫 live publish |
| `website/content/docs/install.mdx` | CREATE | 바이너리 다운로드 + `go install` + docker + action `cli-version` 자동 다운로드 |
| `website/content/docs/*` (nav/search) | UPDATE | install 페이지 등록, drift/coverage 게이트 통과 |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | 행 8 `pending` → `in-progress`(본 플랜 링크). 종료 시 complete |

## Tasks

### Task 0: 이름 일관성 정정 (DECISION — 선결 블로커) — **rename 마이그레이션 게이트 (Codex F3 흡수)**
- **결정**: repo `idenn207/comax` → `idenn207/comax-secrets` **rename**(권장). 근거: `go.mod`(`github.com/idenn207/comax-secrets`)·npm scope(`@comax-secrets/sdk`)·코드 링크가 이미 전부 comax-secrets를 가정 → rename이 최소 변경이며 제품명과 일치. 대안(module path를 `comax`로 변경)은 npm scope를 못 바꿔 반쪽 정합이라 기각.
- **선결 게이트 (rename 전 필수, F3)**: rename은 되돌리기 번거로운 identity 변경이므로 **rename 실행 전** 마이그레이션 인벤토리를 완성하고 통과시킨다:
  1. 외부 참조 인벤토리 — 코드/문서 내 `idenn207/comax`·`comax-secrets` 문자열 전수(`grep -rn "idenn207/comax"`), Vercel 프로젝트 git 링크, docs canonical/site-url, README 배지, action 사용 예시, `sdk/package.json` repository url, npm 메타.
  2. 설정 인벤토리 — branch protection, CI secrets(NPM_TOKEN 등), Actions 권한, Pages/도메인.
  3. old→new 해석 리허설 — rename 직후 `git clone` 리다이렉트, `go install github.com/idenn207/comax-secrets/cmd/cli@<sha>`, `uses: idenn207/comax-secrets@<sha>` 3종이 해석되는지 확인.
  4. 롤백 절차 — old namespace(`comax`)를 **재사용/점유하지 않고 예약**(리다이렉트 유지). 문제 시 즉시 원복 경로 문서화.
- **Action**: 위 게이트 통과 후 (1) GitHub UI rename, (2) `git remote set-url origin https://github.com/idenn207/comax-secrets.git`, (3) Vercel git 링크 갱신 + `website` site-url/canonical 확인, (4) action 공개 ref는 root action이므로 `idenn207/comax-secrets@v1`로 문서화(PRD의 `comax-secrets/load-action` 표기는 별도 org 미도입 → repo-root ref로 확정), (5) 코드 링크 정정(`cmd/cli/main.go:43`, `internal/version/version.go:5`).
- **Mirror**: 없음(설정 작업).
- **Validate**: 게이트 4항목 체크 완료; `git remote -v` = comax-secrets; `gh repo view --json name` = comax-secrets; rename 후 clone/`go install`/`uses:` 3종 해석 성공.

### Task 1: MIT LICENSE
- **Action**: 루트 `LICENSE`에 MIT 전문(저작권자 `idenn207`, 연도 2026) 작성.
- **Mirror**: `sdk/package.json:"license":"MIT"`와 일치.
- **Validate**: `gh repo view --json licenseInfo` 이 non-null(MIT); GitHub 리포 페이지 라이선스 배지 표시.

### Task 2: OSS 메타파일 + README 공개 리라이트
- **Action**: `README.md` 공개용 재작성(상태 최신화 M1–7, 설치 매트릭스, docs site 링크, 배지); `SECURITY.md`(연락처 + threat-model 링크); `CONTRIBUTING.md`; `CODE_OF_CONDUCT.md`; `.github/ISSUE_TEMPLATE/*`; `.github/PULL_REQUEST_TEMPLATE.md`.
- **Mirror**: 문서 한국어(CLAUDE.md), 코드블록/명령/경로 원문. README 기존 Layout/Conventions 섹션 톤 유지.
- **Validate**: 마크다운 링크 체커 통과; `README.md` 내 마일스톤 상태가 PRD 표와 일치; SECURITY.md가 `docs/threat-model.md`로 해석.

### Task 3: `action.yml` M8 기능 완성 (실코드) — **provenance 검증 (Codex F1 흡수)**
- **Action**: `cli-path`를 `required: false`로; 미제공 시 `cli-version`으로 GitHub Release에서 `secret-<os>-<arch>` 다운로드 → **검증 2중화**: (a) `SHA256SUMS` 대조 + (b) **`gh attestation verify <binary> --repo idenn207/comax-secrets`로 build provenance attestation 검증**(Task 4가 `actions/attest-build-provenance`로 서명). provenance 검증 실패 시 **hard-fail**(다운로드 코드가 시크릿 CLI라 신뢰경계에 놓임 — 체크섬만으로는 릴리스 워크플로/토큰 침해 시 바이너리+체크섬 동시 위조를 못 막음). `cli-path`/`cli-version` 둘 다 없으면 영어 `::error::`; `branding:`(icon+color) 추가.
- **고보안 사용자 경로**: docs에 `cli-version` 대신 **immutable digest 핀**(릴리스 자산 SHA로 다운로드) 옵션 명시.
- **Mirror**: 기존 composite step 구조·영어 에러·토큰 env-only 규약(`action.yml:57-77`). 다운로드는 `curl -fsSL` + 체크섬 + attestation 검증(공급망 위생). SDK가 이미 쓰는 npm `--provenance`(`sdk-publish.yml:29,75-79`)와 동일 신뢰 모델을 CLI 바이너리로 확장.
- **Validate**: `cli-path` 명시 경로(기존 action-smoke) 여전히 PASS; 다운로드 리졸버(os/arch→자산명) 단위 스모크 PASS; 위조 체크섬/미서명 자산 주입 시 hard-fail 확인.

### Task 4: 릴리스 워크플로 + 버전 전략 (DECISION: 버전/타깃) — **provenance + `v1` 승급 보호 + RC boot (Codex F1/F2/F4 흡수)**
- **결정**: 첫 공개 버전 `v1.0.0`(CLI/server 태그·SDK npm·action major `v1` 통일). 대안 v0.x 소프트런칭은 명시 시 채택. 크로스컴파일 타깃 = 기존 `xbuild`(linux amd64/arm64/armv7) + **darwin/{amd64,arm64}·windows/amd64 추가**(개발자 로컬 CLI 설치용) 여부는 Task 내 결정(권장: 추가).
- **Action**: `.github/workflows/release.yml` — `on: push: tags: ['v*']`; CI build matrix 미러로 CLI(+server) 크로스컴파일(`-ldflags "-X .../internal/version.Version=<tag> -s -w"`), `SHA256SUMS` 생성, **`actions/attest-build-provenance`로 각 자산에 build provenance attestation 발행**(F1 — Task 3의 `gh attestation verify`가 소비), `gh release create`로 릴리스 노트 + 자산 업로드. 릴리스 잡은 **최소권한**(`contents: write`, `id-token: write`, `attestations: write`만).
- **`v1` 승급 보호 (F2)**: floating `v1` major-tag 이동은 **자동 금지**. (a) `v1.x.x` 정식 태그 push → 릴리스 자산/attestation 생성까지 완료, (b) 별도 **수동 승인 워크플로**(environment protection)로만 `v1` → 최신 `v1.x.x` 재지정, (c) docs에 `@v1`(편의) vs `@v1.0.0`/`@<sha>`(immutable, 고보안) 트레이드오프 + **yank/rollback 런북**(잘못된 `v1` 이동 시 즉시 되돌림) 명시. → downstream `uses: ...@v1` 자동 RCE 표면 축소.
- **최소 RC 서버-boot 게이트 (F4)**: `v1.0.0` 승급 **전** RC 태그(`v1.0.0-rc.N`)에서 릴리스된 CLI/action 경로가 **패키징된 server를 실제 기동·통신**함을 증명하는 잡 1개 추가(action-smoke를 릴리스 자산으로 재실행 또는 compose-smoke 확장). startup/config/version-skew 실패 시 승급 차단. (전체 CI 서버-기동 매트릭스는 backlog 유지 — 여기선 "최소 RC boot"만 M8로 승격.)
- **Mirror**: `ci.yml:77-127` 빌드 매트릭스, `Makefile:41-42` ldflags, `sdk-publish.yml:49-59` ref-guard·provenance 사고방식, `action-smoke.yml`(server boot + E2E 자산 검증) 재사용.
- **Validate**: RC 태그로 워크플로 실행 → 자산 + `SHA256SUMS` + attestation 존재; `gh attestation verify` 통과; 다운로드 바이너리 `secret --version` = 태그; RC boot 잡 green(server 기동 + CLI/action 통신); `go install ...@v1.0.0-rc.1` 성공; `v1` 이동이 수동 승인 없이는 실패함을 확인.

### Task 5: 첫 npm publish + 버전 정렬
- **Action**: `sdk/package.json` version을 릴리스 버전으로 정렬; `npm publish --dry-run` → live(`sdk-publish.yml` dispatch, `dry_run=false`). 사전: npm `@comax-secrets` scope/org 소유 확인.
- **Mirror**: `sdk-publish.yml`(provenance, access public, ref-guard).
- **Validate**: `npm view @comax-secrets/sdk version` = 릴리스 버전; provenance attestation 존재; dual ESM/CJS import 스모크(`ci.yml:243-248` 재사용).

### Task 6: install/download 문서
- **Action**: `website/content/docs/install.mdx` 신설(다운로드/`go install`/docker/action 자동 다운로드), nav·검색 인덱스 등록; `self-host.mdx`/`cli.mdx`/`github-actions.mdx`에 릴리스 다운로드·`cli-version` 사용 반영.
- **Mirror**: 기존 mdx 구조 + Shiki 코드블록; docs 게이트 4종.
- **Validate**: `make website`(typecheck/lint/build + check-token-parity/coverage/drift/site-url) green.

### Task 7: 시크릿 이력 위생 검증
- **Action**: gitleaks/trufflehog로 전체 히스토리 1회 스캔(이미 public → 유출 여부 검증), `.github/workflows/secret-scan.yml` 증분 게이트 상시화. 현재 tracked sensitive = 0(확인됨)이라 히스토리 확인이 핵심.
- **Mirror**: `permissions: contents: read` 최소권한(다른 워크플로).
- **Validate**: 히스토리 스캔 0 findings(또는 발견 시 remediation 결정); CI secret-scan 잡 green.

### Task 8: GH Action Marketplace 등록 (operator runbook)
- **Action**: 사전조건(public repo ✓, root `action.yml` ✓, `branding:` Task 3, 릴리스 태그 Task 4) 충족 후 GitHub "Publish to Marketplace" UI 단계 runbook 문서화(2FA 요구·카테고리·`idenn207/comax-secrets@v1` ref). 실제 게시는 operator 조작.
- **Mirror**: 없음(runbook). action.yml name/description/author 재사용.
- **Validate**: Marketplace 리스팅 노출 + `uses: idenn207/comax-secrets@v1`가 외부 워크플로에서 해석(operator acceptance).

## Validation

```bash
# Go 코어 회귀
make build && make test && make lint && make xbuild

# 이름 정정 후 모듈 해석 (rename + 태그 이후)
go install github.com/idenn207/comax-secrets/cmd/cli@v1.0.0-rc.1
secret --version   # == 태그

# 라이선스 인식
gh repo view --json licenseInfo,name,visibility

# SDK 배포 (dry-run 먼저)
cd sdk && npm ci && npm run build && npm publish --dry-run
# live 후: npm view @comax-secrets/sdk version

# 웹사이트 문서 게이트
make website

# action 다운로드 경로 스모크 (CI: action-smoke.yml 신규 잡)
# 릴리스 워크플로 (사전릴리스 태그로 리허설)
git tag v1.0.0-rc.1 && git push origin v1.0.0-rc.1  # release.yml 트리거

# provenance 검증 (F1) — 다운로드 바이너리가 서명됐는지
gh attestation verify bin/secret-linux-amd64 --repo idenn207/comax-secrets

# 최소 RC 서버-boot 게이트 (F4) — 릴리스 자산이 실제 기동·통신하는지 (CI 잡)
# v1 승급 보호 (F2) — 수동 승인 없이는 v1 이동이 실패해야 함

# 시크릿 이력 스캔
gitleaks detect --source . --redact
```

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| **Repo rename 파급** (Vercel git 링크, 외부 링크, dashboard-ui worktree) | Medium | GitHub 자동 리다이렉트 + Vercel 재링크 + site-url 갱신; rename은 Task 0에서 원자적 선처리 |
| **npm `@comax-secrets` scope 미소유** → live publish 실패 | Medium | Task 5 사전 org/scope 소유 확인, dry-run 우선; 미소유 시 scope 생성 또는 unscoped 대안 결정 |
| **Marketplace 게시 요건**(계정 2FA, unverified creator 정책) | Low | Task 8 runbook에 사전조건 명시, operator 조작 단계로 격리 |
| **첫 배포 비가역성**(npm 72h unpublish 창, 태그·Release) | Medium | 모든 경로 dry-run/rc 태그로 리허설 후 live; 버전 v1.0.0 확정 전 rc 검증 |
| **action 다운로드 폴백 공급망**(체크섬+바이너리 동일 워크플로 생성 → 침해 시 동시 위조) `[Codex F1]` | Medium | Task 3에서 `SHA256SUMS` + **build provenance attestation**(`gh attestation verify`) 2중 검증, 미서명 시 hard-fail; immutable digest 핀 옵션 문서화 |
| **`v1` floating tag 비가역 폭발반경**(retarget=downstream 자동 RCE) `[Codex F2]` | Medium | `v1` 이동을 수동 승인 워크플로로만 허용, `@v1.0.0`/`@<sha>` immutable 핀 권장, yank/rollback 런북 |
| **첫 릴리스 E2E 미검증**(broken 바이너리/version-skew가 v1.0.0에 영구 각인) `[Codex F4]` | Medium | Task 4 최소 RC 서버-boot 게이트로 승급 전 기동·통신 증명 |
| **release.yml가 CI 빌드와 드리프트**(ldflags/타깃 불일치) | Low | `ci.yml` 매트릭스 미러, 동일 `-trimpath`/`CGO_ENABLED=0`; rc 태그로 산출물 검증 |

## Acceptance

- [ ] 모든 태스크 완료
- [ ] Validation 명령 전부 green
- [ ] 이름 일관성: repo=module=npm scope=action ref 모두 comax-secrets, `go install` 성공
- [ ] LICENSE(MIT) GitHub 인식, OSS 메타파일 5종 존재
- [ ] `v1.0.0`(또는 결정 버전) GitHub Release + 크로스컴파일 바이너리 + 체크섬
- [ ] `@comax-secrets/sdk` npm live 배포 + provenance
- [ ] `action.yml` `cli-version` 다운로드 폴백 동작 + **provenance 검증(F1)** + `branding` + Marketplace 등록
- [ ] 릴리스 자산 build provenance attestation 발행 + `gh attestation verify` 통과 (F1)
- [ ] `v1` 승급이 수동 승인 워크플로로만 가능 + yank/rollback 런북 (F2)
- [ ] 최소 RC 서버-boot 게이트 green(승급 전 기동·통신 증명) (F4)
- [ ] rename 마이그레이션 게이트 4항목 통과 후 rename 실행 (F3)
- [ ] install/download 문서 배송 + docs 게이트 green
- [ ] 시크릿 이력 스캔 0 findings (또는 remediation 완료)
- [ ] 패턴 미러 준수(재발명 아님): release.yml는 ci.yml 매트릭스, publish는 sdk-publish.yml 규약
- [ ] backlog defer 유지 결정 명시(source-generated reference / CI 서버-기동 **전체** 매트릭스 = 릴리스 범위 밖; F4의 최소 RC boot만 M8 승격)

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2) · classification=ok · blocking=false
- 라운드 수: 1 (R1에서 4건 전부 ACCEPT_NOW 흡수 → HIGH 잔여 0 → 5.4 재실행 불필요)
- 합치 결론: **needs-attention 4건**(HIGH×2, MEDIUM×2) — 전부 플랜에 흡수. 시크릿 도구 첫 공개 릴리스에 **provenance/서명 검증**·**`v1` 승급 보호**·**rename 마이그레이션 게이트**·**최소 RC 서버-boot**을 차단조건으로 추가.
- YAGNI Triage:
  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | F1 릴리스 아티팩트 provenance 부재(체크섬만, 동일 워크플로가 바이너리+체크섬 생성) | HIGH | ACCEPT_NOW | 시크릿 CLI가 다운로드 코드를 신뢰경계에 올림. GitHub build provenance attestation(네이티브·저비용)로 흡수 → Task 3/4 |
  | F2 `v1` floating tag 비가역 공급망 폭발반경(retarget=downstream RCE) | HIGH | ACCEPT_NOW | `@v1` 이동을 수동 승인+immutable 핀 문서+yank 런북으로 흡수 → Task 4/8 |
  | F3 rename이 redirect 의존, 마이그레이션 게이트 부재 | MEDIUM | ACCEPT_NOW | rename 전 인벤토리 체크리스트+old namespace 예약+롤백을 Task 0 선결 게이트로 추가 |
  | F4 서버-boot 통합 defer → 첫 릴리스 E2E 게이트 없음 | MEDIUM | ACCEPT_NOW | 최소 RC 서버-boot 잡을 M8로 승격(전체 매트릭스는 backlog 유지) → Task 4 |
- Deferred to backlog: 0 (신규 defer 없음. 기존 backlog의 "CI 서버-기동 **전체** 매트릭스"는 유지, F4의 "최소 RC boot"만 M8로 승격)
- R1 흡수 self-attest: 4건 모두 플랜 본문 편집(Task 0/3/4 + Risks/Validation/Acceptance)으로 완전 해소. HIGH 2건이 ACCEPT_NOW이나 R1에서 fully resolve되어 5.4 escalation 조건(b) 미충족 → R2 불필요.
- Open Questions (사용자 확정 필요, 전부 non-CRITICAL):
  - rename vs module-path 변경 — **HIGH** (권장=rename+선결 게이트)
  - 첫 공개 버전 `v1.0.0` vs v0.x 소프트런칭 — **MEDIUM**
  - npm `@comax-secrets` scope/org 소유 확인 — **MEDIUM** (live publish 전)
  - 릴리스 타깃에 darwin/windows 추가 — **LOW** (권장=추가)
- Codex session 참조: threadId `019f3aae-e413-7130-9942-ebc113e7d124`

## Codex Implementation Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.5/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2) · classification=ok · blocking=false
- 라운드 수: 1 (구현-time 결정 3건 + security-reviewer 11건 전부 R1에서 흡수 → HIGH 잔여 0 → 재실행 불필요)
- 합치 결론: **needs-attention** — 공급망 검증이 아직 우회 가능. attestation을 요청 버전/워크플로/커밋에 바인딩하고, `v1` 재지정을 tag ruleset로 원천 차단하고, 첫 npm publish를 exact tag에 강제 연결해야 함.
- YAGNI Triage:
  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | CF1 `v1` 수동승인이 직접 tag push/`v*` 트리거를 못 막음 | HIGH | ACCEPT_NOW | release.yml 트리거에서 bare `v[0-9]+` 제외 + **GitHub tag ruleset**(리포 레벨)로 `v1` create/modify 차단 + acceptance에 `git push -f origin v1` 실패 검증 → Task 4 |
  | CF2 attestation이 요청 버전/워크플로/커밋에 미바인딩 | HIGH | ACCEPT_NOW | floating `cli-version` → immutable full semver resolve 후 `gh attestation verify --format json --signer-workflow <release.yml> --source-ref refs/tags/<resolved>` 바인딩 → Task 3 |
  | CF3 첫 npm live publish가 immutable tag에 미강제(master 허용) | MEDIUM | ACCEPT_NOW | M8 첫 publish는 exact release tag에서만 + environment approval + post-publish `gitHead`/provenance subject == release tag commit 검증 → Task 5 |
- Deferred to backlog: 0
- REJECT_YAGNI: 별도 `cli-version-sha256` action 입력 신설 — resolve-to-immutable + attestation 바인딩(CF2)이 재현성/불변성을 이미 커버. digest 수동 핀은 docs-only로 충분(입력 표면 증설은 과설계).
- R1 흡수 self-attest: HIGH 2건(CF1/CF2)은 구현 요구사항 명세로 완전 해소(release.yml tag-filter/ruleset·action verifier 바인딩은 `gh` 지원 플래그로 구현 가능) → escalation 조건(b) 미충족 → R2 불필요.
- Open Questions: 없음(신규 auto-CRITICAL 없음). 기존 플랜 Open Questions(rename/version/scope/targets) 유지.
- Codex session 참조: implement threadId (`.git/mccp/tmp/codex-impl-stdout.json`)

### Security Reviewer

- 에이전트: `mccp:security-reviewer` (pre-implementation 설계 리뷰) · CRITICAL×1, HIGH×4, MEDIUM×5, LOW×1 + 안전 확인 5영역.
- 안전 확인(재발명 아님, 유지): 토큰 env-only(argv 금지)·일회성 cred + `always()` cleanup(R2-2)·attestation 신뢰체인·ref-guard·xbuild 타깃 일관성.
- 흡수:
  | # | Severity | 항목 | 처리 |
  |---|---|---|---|
  | S1 | CRITICAL | `run` 입력 `bash -c` 셸 실행 | **기존 M3 동작·호출 워크플로 작성자 책임**(신규 M8 취약점 아님). docs 보안 노트(신뢰 코드만 `run`에; `${{ github.event.* }}` 직접 삽입 금지) + action-smoke에 악성 입력 회귀 → Task 3 docs |
  | S3 | HIGH | `gh attestation verify` 러너 이식성(hosted-only) | action에서 `command -v gh` 선재 체크, 없으면 영어 `::error::` + docs에 hosted-only 명시 → Task 3 |
  | S4 | HIGH | os/arch 매핑 위조 | `runner.os`→linux/darwin/windows, `runner.arch`→amd64/arm64/arm safe-map, resolve된 자산명 로깅(non-secret), 미존재 시 명시 `::error::` → Task 3 |
  | S5 | HIGH | `v1` 승급 자동화 | 별도 `.github/workflows/promote-v1.yml`(`workflow_dispatch` + environment protection + `--force-with-lease`) — CF1 tag ruleset와 결합 → Task 4 |
  | S6 | MED | cli-version 포맷 검증 | regex `^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$`, 불일치 시 `::error::` → Task 3 |
  | S7 | MED | 검증→실행 TOCTOU | `$RUNNER_TEMP` 하위 랜덤 서브디렉터리 다운로드 + chmod/exec 동일 step → Task 3 |
  | S8 | MED | ci↔release ldflags 드리프트 | 공용 `make xbuild-release VERSION=` 타깃 추출, ci/release 동일 호출 → Task 4 |
  | S9 | MED | gitleaks baseline | `gitleaks detect` 선행 → `.gitleaksignore`/config TOML로 FP 제외 → CI 동일 config 참조 → Task 7 |
  | S10 | MED | immutable digest 재현성 | CF2 resolve-to-immutable로 충족 + docs 고보안 핀 경로(수동 SHA) → Task 3 docs |
  | S11 | LOW | branding | `branding: { icon: lock, color: blue }` → Task 3 |
