# Plan: GitHub Actions Integration (M3)

**Source PRD**: `.claude/prds/comax-secrets.prd.md`
**Selected Milestone**: #3 GitHub Actions integration — "GitHub Secret 등록 절차 0회. action 한 줄로 step env 주입."
**Complexity**: Medium-Large (Codex R1 후 보안 hardening 흡수로 상향)

## Summary

셀프호스트 서버의 시크릿을 GitHub Actions 워크플로에 주입하는 composite action을
만든다. Codex R1 적대 검토(needs-attention)를 흡수해 보안을 코드로 닫는다: (1) 주입은
기본 **process-env**(`secret run`식, 대상 command 자식 프로세스에만 — job 비노출),
`$GITHUB_ENV` job-wide 방식은 **opt-in**; (2) CI 토큰은 **admin-only 발급 +
`revoked_at` soft revoke**(`is_admin`/`revoked_at` migration), 발급된 CI 토큰은
non-admin이라 추가 발급 불가하고 유출 시 revoke로 차단; (3) action 기본 CLI 획득은
`cli-path` required(M3), release 다운로드 fallback은 M8. project/env **read scope**는
위협모델 재정의가 필요하므로 M4로 명시 이연(비-blocking Open Question).
또한 백엔드에 더해 **대시보드 프론트엔드 화면**을 편입한다: Tokens 관리 화면
(발급/목록/회수)과 GitHub Actions 설정/레퍼런스 화면. 디자인 레퍼런스는 Claude
Design 프로토타입 `Comax Prototype.dc.html`(Integrations·WORKSPACE nav 섹션),
색/컴포넌트는 기존 `web/dashboard/src/styles/globals.css`를 재사용(새 디자인 아님).

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| Schema migration (additive) | `internal/store/migrate.go:21` | `additiveColumns` try-and-tolerate ALTER (M2 `deleted_at` 선례). `is_admin`/`revoked_at` 동일 패턴 |
| process-env 주입 | `cmd/cli/cmd_run.go:56` | `mergeEnv(os.Environ(), secrets)` + `exec.Command` + exit code 전달. Action 기본 모드가 재사용 |
| Naming (CLI cmd) | `cmd/cli/cmd_pull.go:25`, `main.go:51` | `newXxxCmd(st)`, `root.AddCommand(...)` |
| Testable helper pkg | `internal/cli/dotenv/dotenv.go:16` | 방출 로직을 `internal/cli/<pkg>`로 분리, writer 주입 |
| HTTP handler (create) | `internal/server/handlers_projects.go:53` | decode → `validateName` → `BeginTx` → repo → `appendAudit` → Commit → `writeOK(201)` |
| Repo lookup/scan | `internal/store/token_repo.go:100,129` | `ByHash`/`ByID` 스캔; nullable 처리(`sql.NullInt64`) |
| Client method | `pkg/client/client.go:210` | 추가형 typed 메서드, envelope `do()` 경유 |
| Audit actor | `internal/server/handlers_projects.go:101` | `appendAudit`가 `auth.FromContext`로 actor 스탬프 |
| authz(신규) | `internal/server/middleware.go:127` | `auth.FromContext`로 토큰 취득; 신규 `requireAdmin(w,r)` 핸들러 헬퍼가 `tok.IsAdmin` 검사 |
| CI smoke | `.github/workflows/ci.yml:257` | compose-smoke 부팅·wait·검증·teardown 골격 |
| 시크릿 미로그 | `internal/server/middleware.go:52` | body 미로그; export/run 마스킹·"로그에 평문 없음" 테스트 강제 |
| 대시보드 표 화면 | `web/dashboard/src/components/SessionRow.tsx` + `globals.css:1414` `.sessions-table` | Tokens 목록 = SessionRow/sessions-table 패턴 재사용(TokenRow) |
| 대시보드 확인 dialog | `web/dashboard/src/components/ConfirmDialog.tsx` | 토큰 회수 확인(파괴적 동작) |
| 대시보드 API/라우팅 | `web/dashboard/src/lib/api.ts`, `web/dashboard/src/router.tsx`, `AppShell.tsx` nav | createToken/listTokens/revokeToken + nav/route 추가 |
| 대시보드 코드 스니펫 | `globals.css` `.mono`/`--font-mono`, `web/dashboard/src/components`(CodeBlock류) | Actions 화면 YAML/CLI 스니펫 |
| 디자인 레퍼런스 | Claude Design `Comax Prototype.dc.html` (imported) | Integrations(GitHub Actions/Webhooks)·WORKSPACE(Sessions/Tokens) nav 구조 |

## Files to Change

| File | Action | Why |
|---|---|---|
| `internal/store/schema.sql` | UPDATE | `service_tokens`에 `is_admin INTEGER NOT NULL DEFAULT 0`, `revoked_at INTEGER` 추가 (fresh DB) |
| `internal/store/migrate.go` | UPDATE | `additiveColumns` += 두 ALTER; 신규 data-fixup(admin 부재 시 최소 id 토큰 승격) |
| `internal/store/migrate_test.go` | UPDATE | 기존 DB 업그레이드 시 컬럼 추가 + bootstrap 토큰 admin 승격 검증 |
| `internal/store/store.go` | UPDATE | `ServiceToken` += `IsAdmin bool`, `RevokedAt *time.Time` |
| `internal/store/token_repo.go` | UPDATE | `Create(...,isAdmin)`; `BootstrapIfEmpty` is_admin=1; `ByHash` += `revoked_at IS NULL` + is_admin select; `ByID` is_admin/revoked_at select; 신규 `List`, `Revoke` |
| `internal/store/token_repo_test.go` | UPDATE | Create(admin/non), ByHash revoked 거부, Revoke, List |
| `internal/server/handlers_tokens.go` | CREATE | `requireAdmin` + `handleCreateToken`(admin-only, non-admin 발급), `handleListTokens`(admin-only), `handleRevokeToken`(admin-only soft revoke) |
| `internal/server/handlers_tokens_test.go` | CREATE | 발급 1회노출·비admin 403·목록 민감필드부재·revoke 후 401·감사 actor |
| `internal/server/router.go` | UPDATE | `POST`/`GET /api/v1/tokens`, `DELETE /api/v1/tokens/{id}` |
| `internal/server/middleware.go` | UPDATE | **R2-1**: `authSession`이 `tok.RevokedAt != nil`이면 401 (세션 arm revoke 우회 차단) |
| `pkg/client/client.go` | UPDATE | `CreateToken`/`ListTokens`/`RevokeToken` + 타입 |
| `cmd/cli/loadctx.go`(또는 `cmd_pull.go`) | UPDATE | `loadContext`에 `--project` override 추가 (M1 gap: 현재 project는 `.secretrc`에서만) |
| `internal/cli/ghenv/ghenv.go` | CREATE | github-env 방출기: mask sink + env-file sink 분리, heredoc DELIM 충돌검사, 멀티라인 라인별 마스킹 (opt-in 경로 전용) |
| `internal/cli/ghenv/ghenv_test.go` | CREATE | 마스킹 완전성·DELIM 충돌·순서 결정성 |
| `cmd/cli/cmd_export.go` | CREATE | `secret export --project --env --format github-env\|dotenv\|json` (github-env는 `$GITHUB_ENV` 파일+stdout 마스크) |
| `cmd/cli/cmd_export_test.go` | CREATE | 포맷별 방출·`$GITHUB_ENV` 미설정 에러·dotenv 위임 |
| `cmd/cli/cmd_token.go` | CREATE | `secret token create --name` / `list` / `revoke --id` |
| `cmd/cli/cmd_token_test.go` | CREATE | create 1회출력·list·revoke |
| `cmd/cli/cmd_run.go` | UPDATE | `--project` 플래그 노출(loadContext 경유) |
| `cmd/cli/main.go` | UPDATE | `newExportCmd`, `newTokenCmd` 등록 |
| `action.yml` | CREATE | composite: inputs(server, token, project, env, run, export-to, cli-path[req in M3], cli-version). 기본 `secret run --project --env -- <run>`; `export-to: github-env` 시 `secret export` |
| `.github/workflows/action-smoke.yml` | CREATE | M3 수용 증명(아래 Task 11) |
| `docs/github-actions.md` | CREATE | 사용법(process-env 기본 + github-env opt-in), 토큰 발급/revoke, 마스킹/scope 한계 |
| `docs/threat-model.md` | UPDATE | CI 토큰 authz/revoke, read-scope M4 이연, job-wide opt-in 노출 명문화 |
| `README.md` | UPDATE | M3 상태 + docs 링크 |
| `web/dashboard/src/pages/Tokens.tsx` (+`.test.tsx`) | CREATE | 서비스 토큰 관리 화면: 목록 + 발급(1회노출 dialog) + 회수(confirm). admin 세션만 |
| `web/dashboard/src/pages/Actions.tsx` (+`.test.tsx`) | CREATE | GitHub Actions 설정/레퍼런스: 1-liner 사용법 + 토큰 발급 링크 + 복사가능 YAML 스니펫(process-env 기본/github-env opt-in) |
| `web/dashboard/src/components/TokenRow.tsx` | CREATE | SessionRow 미러 — 토큰 1행(name/admin/created/last-used/revoke) |
| `web/dashboard/src/lib/api.ts` (+`.test.ts`) | UPDATE | `listTokens`/`createToken`/`revokeToken` (기존 fetch 패턴) |
| `web/dashboard/src/router.tsx`, `src/components/AppShell.tsx` | UPDATE | Integrations(GitHub Actions)·WORKSPACE(Tokens) nav + route |
| `web/dashboard/tests/e2e/*` | UPDATE | Tokens/Actions 화면 axe(WCAG 2.2 AA) 커버 |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | M3 행 in-progress + Plan 셀(완료) |

## Tasks

### Task 1: 스키마 + migration (`is_admin`, `revoked_at`)
- **Action**: `schema.sql` `service_tokens`에 두 컬럼 추가. `migrate.go` `additiveColumns`에 `ALTER TABLE service_tokens ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`, `ALTER TABLE service_tokens ADD COLUMN revoked_at INTEGER`. data-fixup: admin이 하나도 없으면 최소 id 토큰을 is_admin=1로 승격(`UPDATE ... WHERE id=(SELECT MIN(id)...) AND NOT EXISTS(SELECT 1 ... WHERE is_admin=1)`) — 기존 M1/M2 DB의 bootstrap 토큰 발급권 보존.
- **Mirror**: `migrate.go:21` additive 패턴 + `isDuplicateColumn` 관용.
- **Validate**: `go test ./internal/store/ -run TestMigrate -race`

### Task 2: `TokenRepo` — admin/revoke 반영
- **Action**: `ServiceToken` += `IsAdmin`, `RevokedAt`. `Create(ctx,name,hash,isAdmin)`; `BootstrapIfEmpty` INSERT에 is_admin=1; `ByHash` SELECT is_admin/revoked_at + `WHERE token_hash=? AND revoked_at IS NULL`(bearer arm revoked 인증 거부); `ByID` is_admin/revoked_at **포함(필터 없이)** — 관리/세션 arm이 상태를 봐야 함(**R2-1**); 신규 `List(ctx) ([]ServiceToken,error)`(hash 제외), `Revoke(ctx,id)`(UPDATE revoked_at, 이미 revoked/부재 시 ErrNotFound).
- **Mirror**: `ByHash`/`ByID`(:100/:129) 스캔·nullable.
- **Validate**: `go test ./internal/store/ -run TestTokenRepo -race`

### Task 3: 토큰 핸들러 + 라우트 (admin-only)
- **Action**: `handlers_tokens.go`. `requireAdmin(w,r) bool`(`auth.FromContext`→IsAdmin, 아니면 403 `forbidden`). `handleCreateToken`(admin-only, body `{name}`→validateName→GenerateToken/HashToken→`Create(...,false)`→`appendAudit("auth.token.create")`→201 `{token,name,created_at}`), `handleListTokens`(admin-only→`tokenView`{id,name,is_admin,created_at,last_used_at,revoked_at}), `handleRevokeToken`(admin-only→`Revoke`→`appendAudit("auth.token.revoke")`→204). router 3 라우트.
- **Action(R2-1 추가)**: `authSession`(`middleware.go:152`)이 `ByID` 재로드 후 `tok.RevokedAt != nil`이면 401 — revoke된 토큰의 라이브 대시보드 세션 차단. revoked-token 세션(admin 전용 라우트 포함) 테스트 추가.
- **Mirror**: `handleCreateProject`(:53); `authSession`의 "token went away → 401" 주석 로직 확장.
- **Validate**: `go test ./internal/server/ -run 'TestToken|TestAuthSession' -race`

### Task 4: client 토큰 메서드
- **Action**: `CreateToken(name)`, `ListTokens()`, `RevokeToken(id)` + `Token`/`TokenCreated` 타입.
- **Mirror**: `CreateProject`/`ListProjects`(:202).
- **Validate**: `go build ./... && go test ./pkg/client/ -race`

### Task 5: `--project` override (M1 gap)
- **Action**: `loadContext`에 project override 파라미터/플래그. `--project` 있으면 `.secretrc` 대신 사용(CI엔 `.secretrc` 부재). `run`/`export`가 사용.
- **Mirror**: `cmd_pull.go:123` loadContext 시그니처 확장.
- **Validate**: `go test ./cmd/cli/ -run TestLoadContext -race`

### Task 6: `internal/cli/ghenv` — github-env 방출기 (opt-in 전용)
- **Action**: `Emit(maskW, envW io.Writer, entries map[string]string) error`. 각 값 `maskW`에 `::add-mask::`(멀티라인 라인별), `envW`에 `KEY<<DELIM\n<v>\nDELIM`(DELIM 미포함 보장), key 정렬.
- **Mirror**: `dotenv.Emit`(:197).
- **Validate**: `go test ./internal/cli/ghenv/ -race -cover` (≥80%)

### Task 7: `secret export` (opt-in github-env / dotenv / json)
- **Action**: `cmd_export.go`. loadContext(--project/--env)→ListSecrets→`--format`: `github-env`는 `$GITHUB_ENV`(미설정 시 명확 에러; `--github-env-file` override) 열어 `ghenv.Emit(os.Stdout, file, ...)`; `dotenv`/`json`은 대응 방출.
- **Mirror**: `cmd_pull.go` 흐름.
- **Validate**: `go test ./cmd/cli/ -run TestExport -race`

### Task 8: `secret token` 서브커맨드
- **Action**: `cmd_token.go`. `create --name`(plaintext 1회 + "GH secret 저장" 안내 stderr), `list`(표), `revoke --id`. `main.go` 등록.
- **Validate**: `go test ./cmd/cli/ -run TestToken -race`

### Task 9: `secret run --project`
- **Action**: `cmd_run.go`에 `--project` 플래그 노출(Task 5 loadContext 경유). Action 기본 모드가 사용.
- **Validate**: `go test ./cmd/cli/ -run TestRun -race`

### Task 10: composite `action.yml`
- **Action**: `using: composite`. inputs: `server`,`token`,`project`,`env`(req), `run`(기본 모드 command), `export-to`(opt-in: `github-env`), `cli-path`(**M3 required**), `cli-version`(M8 fallback용). 로직: `cli-path`로 CLI 확보 → **(R2-2) one-shot 자격증명**: `secret login --credentials "$RUNNER_TEMP/comax-creds.json"`(기본 `~/.config/comax/...` **미기록**), token은 stdin/env로 전달(로그 회피) → `export-to` 없으면 `secret run --credentials "$RUNNER_TEMP/comax-creds.json" --project --env -- <run>`(process-env), 있으면 `secret export ... --format github-env` → **최종 cleanup 스텝(`if: always()`)이 `$RUNNER_TEMP/comax-creds.json` 삭제**. `cli-path` 미제공 시 M3 실패(명확 메시지) + M8 release fallback TODO 주석.
- **Mirror**: `--credentials` persistent 플래그는 이미 존재(`main.go:48`).
- **Validate**: Task 11 smoke.

### Task 11: `.github/workflows/action-smoke.yml` — M3 수용 증명
- **Action**: `go build ./cmd/server ./cmd/cli` → 서버 기동 → bootstrap 토큰(admin) 캡처 → `secret token create --name ci`(admin이 non-admin 발급) → project/env seed + `secret set TESTKEY=<canary>` →
  - **(a) process-env 격리 + 무유출(R2-3)**: 기본 모드 `run`으로 **값을 echo하지 않는** canary 체크 실행(`sh -c '[ "$TESTKEY" = "$EXPECTED" ] && echo PASS || { echo FAIL; exit 1; }'`, EXPECTED는 마스킹된 env로 전달) → **후속 별도 step에서 TESTKEY 부재** 검증 → **스텝 로그에 canary 평문 부재** grep(있으면 fail).
  - **(b) 자격증명 미잔존(R2-2)**: 기본 cred 경로(`$HOME/.config/comax/credentials.json`) **미생성** 단언; `$RUNNER_TEMP` cred는 cleanup 후 부재.
  - **(c) github-env opt-in 마스킹**: `export-to: github-env` 주입 + 로그 평문 미출현 grep.
  - **(d) revoke**: `secret token revoke`(admin) 후 그 토큰 재호출 시 401.
- **Mirror**: `ci.yml:257` 골격.
- **Validate**: 워크플로 green.

### Task 12: 문서 + PRD 정합성
- **Action**: `docs/github-actions.md`(process-env 기본 + github-env opt-in 1-liner, 토큰 create/revoke, read-scope M4 한계), `docs/threat-model.md` CI 토큰 절, `README.md` M3, PRD 셀.
- **Validate**: 링크 grep 0 broken, `make build`.

### Task 13: 대시보드 — Tokens 관리 화면
- **Action**: `pages/Tokens.tsx` + `TokenRow.tsx`. 목록(`.sessions-table` 재사용: name·admin 배지·created·last-used·회수 버튼), 발급(name 입력 → 성공 시 plaintext를 **1회 노출 dialog**로 copy 제공, 닫으면 재조회 불가 안내), 회수(`ConfirmDialog`). `lib/api.ts`에 `listTokens/createToken/revokeToken`. **admin 세션만 진입**(비admin은 403 → 안내), 프로토타입 WORKSPACE 섹션 참조.
- **Mirror**: `Sessions.tsx`/`SessionRow.tsx` + `ConfirmDialog.tsx`.
- **Validate**: `cd web/dashboard && npm run typecheck && npm test -- Tokens`

### Task 14: 대시보드 — GitHub Actions 화면
- **Action**: `pages/Actions.tsx`. 레퍼런스/설정 화면: (1) 토큰 발급 안내(Tokens 화면 링크), (2) **복사가능 YAML 스니펫** — 기본 process-env(`with: { run: ... }`)와 opt-in github-env 두 예시, (3) 마스킹/scope 한계 노트. 순수 정적(신규 백엔드 없음). 프로토타입 Integrations 섹션 참조.
- **Mirror**: `.mono`/CodeBlock 스니펫, `PageHeader`.
- **Validate**: `cd web/dashboard && npm run typecheck && npm test -- Actions`

### Task 15: 대시보드 — nav + 라우팅
- **Action**: `AppShell.tsx`에 Integrations(GitHub Actions)·WORKSPACE(Tokens) nav 항목 + `router.tsx` route 추가. `.nav-link`/`.nav-section` 재사용. e2e axe 스펙에 두 화면 추가.
- **Mirror**: 기존 nav 구성 + `tests/e2e/a11y.spec.ts`.
- **Validate**: `cd web/dashboard && npm run lint && npm run build && npm run test:e2e`

## Validation

```bash
make build
make test          # -race, 신규 패키지 포함
make lint
go test ./internal/store/ ./internal/server/ ./internal/cli/ghenv/ ./cmd/cli/ ./pkg/client/ -race -cover
# 대시보드(Tokens/Actions 화면)
cd web/dashboard && npm run lint && npm run typecheck && npm test && npm run build && npm run test:e2e
# CI: action-smoke 워크플로 = bootstrap→token(admin→non-admin)→seed→
#     process-env 격리→github-env 마스킹→revoke 후 401 (M3 acceptance)
```

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| **migration data-fixup 오류** — 기존 DB의 bootstrap 토큰이 non-admin화되어 발급 불가 | Medium | admin 부재 시 최소 id 토큰 승격 UPDATE + `migrate_test.go`가 업그레이드 시나리오 검증 |
| **process-env 격리 미검증** — 기본 모드가 실제로 job 비노출인지 | Medium | smoke Task 11(a)가 "후속 step에 TESTKEY 부재"를 명시 검증 |
| **github-env 마스킹 누락** — opt-in 경로 로그 유출 | Medium | `ghenv` 단위테스트 + smoke가 "로그에 평문 없음" grep; 짧은/저엔트로피 한계 docs |
| **cli-path required로 크로스레포 UX 제약** — M8 전 소비자 편의 저하 | Medium(수용) | M3 dogfood는 자체 빌드/설치로 충분; release 다운로드 fallback은 M8, action.yml에 TODO 명시 |
| **non-admin 토큰도 전체 secret read 가능** — read-scope 부재 | Medium | M4 위협모델로 명시 이연(Open Q). M3은 발급 admin-only + revoke로 blast radius 축소·차단 가능 |
| **heredoc DELIM 충돌** | Low | 랜덤 DELIM + 포함검사 테스트 |
| **revoke 세션 우회(R2-1)** — bearer는 막아도 대시보드 세션 잔존 | Medium | `authSession`이 `RevokedAt` 검사 + revoked-token 세션 테스트 |
| **CI 토큰 디스크 잔존(R2-2)** | Medium | `$RUNNER_TEMP` one-shot cred + cleanup + 기본경로 미생성 단언 |
| **smoke 자체가 평문 유출(R2-3)** | Medium | 비-echo canary + 로그 평문 부재 grep |

## Acceptance

- [ ] 모든 태스크 완료
- [ ] `make build`/`make test`(-race)/`make lint` green, 신규 패키지 커버리지 ≥80%
- [ ] smoke: 기본 모드 process-env 격리(후속 step 부재) + **로그에 canary 평문 부재(R2-3)** + **기본 cred 파일 미생성(R2-2)** + github-env opt-in 마스킹 + 발급 admin-only(비admin 403) + **revoke 후 bearer·세션 모두 401(R2-1)** 을 green으로 증명
- [ ] 패턴 재사용: migration=additiveColumns, 주입=`secret run` mergeEnv, 핸들러=handleCreateProject, 방출=dotenv, client=추가형, CI=compose-smoke
- [ ] 대시보드: Tokens 화면(발급 1회노출·목록·회수, admin 세션만) + GitHub Actions 화면(YAML 스니펫) 동작, 프로토타입 참조·globals.css 재사용, axe WCAG 2.2 AA green
- [ ] 시크릿이 서버/CLI/CI/대시보드 로그·DOM 어디에도 평문으로 남지 않음(테스트로 강제; 토큰 발급 plaintext는 1회 dialog만)

## Open Questions

- **[MEDIUM, 비-blocking] read scope (project/env) M4 이연**: non-admin CI 토큰도 현재는 전 project/env secret read 가능. scope 컬럼/미들웨어 인가는 위협모델 재정의가 필요해 M4로 이연(사용자 확정). M3는 발급 admin-only + soft revoke로 위험을 닫는다.
- **[LOW] action 배포 경로 / cli-path fallback**: PRD `comax-secrets/load-action@v1`은 M8 패키징. M3은 모노레포 `uses: idenn207/comax-secrets@<ref>` + `cli-path` required. release 바이너리 다운로드 fallback은 M8.

## Design Routing Guide

계획에 대시보드 UI(Tokens/Actions 화면)가 편입되어 design_signal=true. 계획 단계는
렌더된 UI가 없으므로 impeccable을 **호출하지 않고** 아래를 구현 단계 체크리스트로
기록한다. routing mode: **auto** (구현 단계에서 발효). 구현 시 디자인 게이트가 diff
신호에 맞춰 아래 stage별 impeccable 명령을 라우팅한다.

| Stage | Command |
|---|---|
| discovery | `/impeccable shape` |
| refine | `/impeccable layout` · `/impeccable typeset` · `/impeccable animate` · `/impeccable colorize` · `/impeccable bolder` · `/impeccable quieter` · `/impeccable overdrive` · `/impeccable delight` |
| simplify | `/impeccable adapt` · `/impeccable distill` · `/impeccable clarify` |
| evaluate | `/impeccable critique` · `/impeccable audit` |
| harden | `/impeccable harden` · `/impeccable optimize` · `/impeccable onboard` |
| polish | `/impeccable polish` |
| system | `/impeccable document` · `/impeccable extract` |

> 구현 화면은 새 디자인이 아니라 기존 `globals.css`/디자인 시스템 패턴 재사용이 원칙.
> `Comax Prototype.dc.html`을 시각 레퍼런스로, evaluate 단계 `/impeccable critique`로
> 프로토타입 대비 정합성을 검증한다.

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2)
- 라운드 수: 1 (R1) → 사용자 결정 흡수 후 revised-plan R2 (아래 갱신)
- R1 합치 결론: **needs-attention** — CI 토큰의 권한·폐기·노출을 문서 경고로 넘겨 실패 모드 과다.
- R1 YAGNI Triage / 해소:
  | Finding | Sev | Verdict | 해소(사용자 결정) |
  |---|---|---|---|
  | F1 flat 토큰 = 사실상 admin | HIGH(auto-CRIT) | ACCEPT_NOW | **admin-only 발급 + is_admin migration** (Task 1-3) |
  | F2 revoke 부재 → 복구 경로 없음 | HIGH | ACCEPT_NOW | **`revoked_at` soft revoke + Verify/ByHash 거부** (Task 1-3) |
  | F3 github-env = job-wide 노출 | HIGH(auto-CRIT) | ACCEPT_NOW | **process-env 기본(`secret run`) + github-env opt-in** (Task 9-11) |
  | F4 기본 install 경로 미검증 | MEDIUM | ACCEPT_NOW | **cli-path required(M3)**, fallback M8 (Task 10) |
- Deferred to backlog: 0
- R1 Open Questions(해소됨): F1/F3 auto-CRITICAL → 사용자 결정으로 코드 해소. 남은 것은 read-scope(M4, 비-blocking).
- Codex session 참조: R1 threadId `019f1dd4-5092-7b21-ad5e-2e0defe6eaa1`

### R2 — revised-plan 재검토 (Phase 5.4, escalation 1/cap 1)

- R2 합치 결론: **needs-attention** — "revised plan still leaves a real revocation bypass and adds CI credential/log leakage paths." R1 4건은 방향 해소됐으나 새 표면에서 구현 정합성 결함 3건.
- R2 YAGNI Triage / 흡수(구현 정합성 수정 — 사용자 결정 불요, 계획에 직접 반영):
  | Finding | Sev | Verdict | 흡수 |
  |---|---|---|---|
  | R2-1 soft revoke가 라이브 대시보드 세션 미종료 (`ByID` 세션 arm 미검사) | HIGH | ACCEPT_NOW | `authSession`이 `RevokedAt != nil` 401 (Task 2/3, `middleware.go`) |
  | R2-2 action이 CI 토큰을 러너 디스크(`~/.config/comax`)에 잔존 | HIGH | ACCEPT_NOW | `--credentials "$RUNNER_TEMP/..."` one-shot + `always()` cleanup; 기본 cred 미생성 smoke 단언 (Task 10/11b) |
  | R2-3 smoke의 `printenv`가 지키려던 평문을 로그로 유출 | HIGH | ACCEPT_NOW | 비-echo canary 체크 + "로그에 평문 canary 부재" grep (Task 11a) |
- Deferred to backlog: 0. auto-CRITICAL Open Question: 없음(3건 모두 계획에 흡수 완료).
- 라운드 캡(default 1) 도달 → R3 미실행. 흡수된 수정의 **실코드 검증은 `/mccp:prp-implement`의 Implement-Codex 게이트**가 담당(revoked-token 세션 테스트, cred 파일 위치/cleanup, process-env 로그 canary 부재를 acceptance로).
- Codex session 참조: R2 threadId `019f1dee-c23b-7813-a837-c58153a8ad10`

## Codex Implementation Review

decision-set already converged in mccp-plan-codex review. No new implement-time decisions detected. Cross-gate dedupe applied.
