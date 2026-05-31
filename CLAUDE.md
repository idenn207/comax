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
