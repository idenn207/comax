# CLAUDE.md

## 페르소나

- **항상 한국어로 답변한다.** 코드, 식별자, 외부 라이브러리 이름은 원문 유지.
- **문서도 한국어로 작성한다.** PRD, plan, docs, report, issue, PR 설명, 커밋 메시지 본문 모두 해당. 코드 블록·명령어·경로는 원문.
- 군더더기 없이 핵심만. 결정과 결과를 먼저, 배경은 짧게.

## 프로젝트

Comax Secrets의 `dashboard-ui` worktree. 서버/CLI 위에 올라갈 운영자용 대시보드를 만든다.

- 백엔드 컨벤션: [README.md](README.md) "Conventions" 섹션
- 진행 중 작업: [.claude/plans/comax-secrets.plan.md](.claude/plans/comax-secrets.plan.md)
- 빌드/테스트/린트: `make build` / `make test` / `make lint`

## 작업 규칙

- Go 1.25+, `CGO_ENABLED=0`. 순수 Go 유지.
- 시크릿은 절대 로그에 남기지 않는다 (테스트로 강제).
- 패키지별 커버리지 80% 이상.
- 에러 메시지는 영어로 작성 (사용자 노출용 포함). UI 라벨/설명문은 한국어 유지.

## 디자인 컨텍스트

표면이 둘이다. UI/디자인 작업 전 해당 표면의 컨텍스트를 먼저 읽는다 (impeccable).

- **마케팅·문서 사이트** (`website/`, register **brand**): [website/PRODUCT.md](website/PRODUCT.md). 진행 중인 `feat/website-redesign`의 방향 — 대시보드 monochrome parity에서 이탈해 독립 정체성(Bun류 대담한 인디 캐릭터, docs 중심), 독자는 개인 개발자, voice는 정직한 장인. 다음 단계: `/impeccable shape landing`으로 새 팔레트·display 폰트·DESIGN.md 확정. ⚠️ 랜딩 정체성 분리 시 `website/scripts/check-token-parity.mjs`를 랜딩 한정으로 완화해야 함(CI 게이트).
- **운영자 대시보드** (`dashboard/`, register **product**): [docs/PRODUCT.md](docs/PRODUCT.md) + [docs/DESIGN.md](docs/DESIGN.md). "색은 의미에만"의 monochrome, GitHub 레퍼런스. website의 뉴트럴·시맨틱 토큰이 계승하는 원본.
