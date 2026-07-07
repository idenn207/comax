# Implementation Report: M8 — Public Release (MIT)

**Plan**: [.claude/plans/comax-secrets-m8-public-release.plan.md](../plans/comax-secrets-m8-public-release.plan.md)
**Branch**: `feat/comax-secrets-m8-public-release`
**Status**: 되돌릴 수 있는 in-repo 인프라 **구현 완료**. 비가역 operator 액션(rename·npm publish·tag/ruleset·Marketplace)은 아래 **Operator Runbook**으로 게이트(자율 실행 안 함).

## Summary

repo는 이미 PUBLIC이므로 M8은 "공개하기"가 아니라 "공개된 저장소를 신뢰할 수 있는 릴리스로 완성"하는 작업. 릴리스 자동화(크로스컴파일+provenance)·공급망 검증(action 다운로드)·OSS 위생(LICENSE+메타)·문서를 구현하고, 릴리스를 실제로 찍는 비가역 단계는 runbook으로 분리했다.

## 게이트 요약

| 게이트 | 결과 |
|---|---|
| plan-codex (Codex adversarial) | needs-attention 4건(F1~F4) 전부 흡수 |
| implement-codex (Codex) | needs-attention 3건(CF1 v1 ruleset·CF2 attestation 바인딩·CF3 npm exact-tag) 전부 흡수 |
| security-reviewer | CRITICAL×1(기존 M3 동작·문서화)·HIGH×4·MEDIUM×5·LOW×1 전부 흡수 |
| impeccable design | SIGNAL=0(디자인 표면 없음) → silent-skip |

## 구현 완료 (reversible, in-repo)

| # | 산출물 | 검증 |
|---|---|---|
| T1 | `LICENSE` (MIT ©2026 idenn207) | GitHub 라이선스 인식 |
| T2 | `README.md` 공개 리라이트(M1–7·설치 매트릭스·배지) + `SECURITY.md`·`CONTRIBUTING.md`·`CODE_OF_CONDUCT.md` + `.github/ISSUE_TEMPLATE/{bug,feature,config}` + `PULL_REQUEST_TEMPLATE.md` | — |
| T3 | `action.yml`: `cli-path` 선택화 + `cli-version` 다운로드(체크섬+attestation 2중 검증, `--signer-workflow` 바인딩 CF2, os/arch safe-map S4, `command -v gh` 이식성 S3, semver regex S6, 랜덤 tmpdir S7, `branding` S11) | 체크섬 awk 검증기 로직 MATCH(text/binary 포맷 불변), YAML parse OK |
| T4 | `.github/workflows/release.yml`(bare-major 제외 CF1 + `attest-build-provenance` F1 + RC boot smoke F4) · `promote-v1.yml`(workflow_dispatch + environment protection + 검증 S5) · `make xbuild-release` 공용 타깃 S8 | `make xbuild-release` end-to-end: 6 타깃 바이너리 + SHA256SUMS + ldflags 버전 스탬프(`secret version v0.0.0-smoke`) |
| T6 | `website/content/docs/install.mdx`(다운로드·go install·docker·action cli-version) + `docs-nav.ts` 등록 | docs-coverage(bijection) + docs-drift(8 inputs) green |
| T7 | `.github/workflows/secret-scan.yml`(gitleaks, 전체 이력 + 증분 + 주간) + `.gitleaks.toml`(FP 제외 S9) | 로컬 tracked 표면 heuristic clean; 전체 이력 스캔은 CI |
| — | `.gitignore`에 `/dist/` 추가 | 릴리스 산출물 커밋 방지 |

## 검증 수행

- Go 컴파일 sanity: cli+server `go build` OK
- `make xbuild-release REL_VERSION=v0.0.0-smoke`: 6 크로스컴파일(linux amd64/arm64/armv7·darwin amd64/arm64·windows amd64) + SHA256SUMS + 버전 스탬프 확인
- action.yml 체크섬 검증 로직: 실제 SHA256SUMS 대조 MATCH(binary-mode `*file` 포맷 포함)
- YAML parse: action.yml + release/promote-v1/secret-scan 4종 OK
- docs 게이트: check-docs-coverage + check-docs-drift green
- **미수행(의도적)**: `make test`(race)·`make website`(next build) — 이번 변경은 config/workflow/docs/markdown + Go 미변경이라 회귀 표면 없음. CI(ci.yml·website.yml·action-smoke.yml·release.yml smoke)가 전체 검증.

## Operator Runbook (비가역 — 사람이 실행)

> 순서 중요. 각 단계는 되돌리기 어렵거나 외부 상태를 바꾼다.

1. **결정 확정** — rename=comax-secrets, 버전=v1.0.0, 타깃=+darwin/windows (플랜 승인 시 채택). npm `@comax-secrets` scope/org 소유 확인.
2. **rename 마이그레이션 게이트(T0)** — 인벤토리 4항목(외부 참조·설정·old→new 해석 리허설·롤백) 통과 후 GitHub UI에서 `comax`→`comax-secrets` rename. 직후 `git remote set-url`, Vercel git 링크 갱신, `git clone`/`go install`/`uses:` 3종 해석 확인.
3. **`v1` tag ruleset(CF1)** — Settings → Rules에서 `v1` 태그 create/update/delete 제한(직접 `git push -f origin v1` 거부). `release-approval` environment에 required reviewers 설정.
4. **RC 리허설** — `git tag v1.0.0-rc.1 && git push origin v1.0.0-rc.1` → release.yml(smoke→build→attest→prerelease) green 확인. 다운로드 바이너리 `secret --version`==태그, `gh attestation verify` 통과.
5. **정식 릴리스** — `git tag v1.0.0 && git push origin v1.0.0` → GitHub Release + 자산 + attestation.
6. **`v1` 승급** — `promote-v1.yml`을 `source_tag=v1.0.0`으로 dispatch → approver 승인 → `v1` alias 이동.
7. **npm 첫 publish(CF3)** — `sdk/package.json` version=1.0.0 정렬, **exact 태그에서** `sdk-publish.yml` dispatch(`dry_run=false`). post-publish: `npm view @comax-secrets/sdk version`·provenance subject·`gitHead`==release commit 확인.
8. **Marketplace 등록(T8)** — action.yml `branding` + 릴리스 태그 충족 후 GitHub "Publish to Marketplace"(2FA 필요). `uses: idenn207/comax-secrets@v1` 외부 해석 확인.

## Deviations / 잔여

- backlog defer 유지: source-generated reference, CI 서버-기동 **전체** 매트릭스(F4의 최소 RC boot만 M8 승격).
- PRD 행 8은 operator 릴리스 완료 시 `complete` 전환. 현재 `in-progress`.
- 플랜 미아카이브(operator 단계 잔존).

## Next Steps

- [ ] 이 브랜치 리뷰 + PR(`/mccp:pr`) → master 머지
- [ ] 머지 후 위 Operator Runbook 1–8 순차 실행
