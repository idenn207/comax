---
state_version: 1
task_fingerprint: comax-secrets-m8-public-release
created_at: 2026-06-13T09:30:53.231Z
updated_at: 2026-07-07T10:17:55.487Z
last_event: stop_loop_pass
last_event_at: 2026-07-07T10:17:55.487Z
unsafe_checkpoint: false
confirm_required: false
next_chunk: |
  M8 공개 릴리스 인프라 머지 완료(PR #20 → master 574adee). in-repo 인프라는
  끝났고, 남은 것은 비가역 Operator Runbook 1–8(사람 실행). plan 아카이브·PRD
  행8 complete 전환은 릴리스 완료 후로 연기.
session_end_imminent: false
chain_aborted: false
dep_check_at: 2026-06-15T16:09:58.866Z
---
## Goal
Comax Secrets M8 (Public Release, MIT). 이미 public인 레포를 신뢰 가능한 릴리스로 완성. /mccp:pr 게이트 → master 머지 완료.

## Plan
- .claude/plans/comax-secrets-m8-public-release.plan.md (**미아카이브** — Operator Runbook 비가역 단계 잔존. 릴리스 완료 후 completed/로 이동)

## Done
- M8 in-repo 인프라 구현·머지(PR #20 → master `574adee`, squash). in-repo = 되돌릴 수 있는 부분만.
- 릴리스 자동화: release.yml(bare-major 거부 CF1·attest-build-provenance F1·RC boot smoke F4) + promote-v1.yml(dispatch+environment 승인) + `make xbuild-release`(6 타깃 + SHA256SUMS + ldflags 버전).
- 공급망 검증: action.yml `cli-version` 다운로드(SHA256SUMS 체크섬 + `gh attestation verify --signer-workflow` 바인딩 CF2·안전 os/arch 매핑·per-run tmpdir·semver regex) + secret-scan.yml(gitleaks 전체이력+증분+주간) + .gitleaks.toml.
- OSS 메타: LICENSE(MIT)·SECURITY.md·CONTRIBUTING.md·CODE_OF_CONDUCT.md·ISSUE/PR 템플릿. 문서: README 공개 리라이트 + install.mdx + docs-nav.
- 게이트: PR-Codex 1라운드 수렴(actionable 0) · security-reviewer 신규 CRITICAL/HIGH 0 · impeccable silent-skip(no-signal) · receipt valid.
- **CI가 잡은 실제 회귀 fix**: action.yml 설명문의 `${{ github.event.* }}`가 매니페스트 로드를 깨뜨림(action-smoke fail) → `${{ }}` 벗겨 수정(fix 커밋, squash에 포함). master action-smoke green. → [[comax-action-yml-expression-trap]]
- report: .claude/reports/comax-secrets-m8-public-release.report.md

## In Progress
없음(in-repo). **대기: Operator Runbook 1–8**(비가역·사람 실행) — 결정 확정 → rename(comax→comax-secrets) → v1 tag ruleset → RC 리허설 → 정식 릴리스 → v1 승급 → npm publish → Marketplace 등록.

## Next Step
1. Operator Runbook 1–8 순차 실행(runbook은 report 참조). 각 단계 비가역 — 사람이 게이트.
2. 릴리스 완료 후: plan → completed/ 아카이브, PRD 행8 `complete` 전환, STATE 문서 정합화(M6 #17 패턴).
3. 또는 다음 PRD 마일스톤 선택.

## Last Decision
M8 머지(PR #20). Phase 5 CI 검증에서 action-smoke 실패 발견 → action.yml `${{ }}` 설명문 함정(게이트 3겹 통과·CI만 포착)을 한 줄 수정·재푸시, master green 확인 후 사용자 머지. plan 아카이브·PRD complete는 report 지침대로 Operator Runbook 완료까지 연기.

## Open Questions


## Last Updated
2026-07-07T10:17:55.487Z
