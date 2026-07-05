# Plan: M7 — Infra Config Templating PoC (Decision)

**Source PRD**: `.claude/prds/comax-secrets.prd.md`
**Selected Milestone**: #7 — (Decision) Infra config templating PoC
**Complexity**: Small (PoC 스코프; decision 산출물 우선)

## Summary

redis.conf / nginx.conf 같은 인프라 설정 파일을 "시크릿으로 채운 템플릿을 환경별로 렌더링"하는 모델의 **최소 PoC**를 만들어, v1 포함 여부(Open Question #5)를 **증거 기반으로 결정**한다. 산출물은 완성된 기능이 아니라 In/Out 판단 근거다. 기존 `${{ env.KEY }}` 참조 문법(`internal/secret/reference.go`)과 클라이언트측 렌더(`pull`/`export` 형제)를 재사용해 서버 API·마이그레이션 변경 0으로 되돌리기 쉬운 형태를 유지한다.

## Decision Framing (이 마일스톤의 본질)

M7은 **decision 마일스톤**이다. Acceptance는 "코드 완성"이 아니라 "In/Out을 근거와 함께 확정하고 PRD에 반영"이다. PoC는 결정을 내리기에 충분한 만큼만 만들고, 결정이 **Out**이면 코드는 폐기 가능해야 한다(그래서 서버 변경 0, 되돌리기 쉬운 클라이언트측 설계).

### Decision Criteria (PoC가 답해야 할 질문)

1. **실 페인 커버**: 운영자의 실제 redis.conf / nginx.conf를 이 모델로 렌더할 수 있는가? 손유지 버전과 diff가 사라지는가? **핵심: 단일 템플릿 1개가 local/dev/prod 3환경으로 렌더되는가**(환경별 템플릿 복제 없이). 이게 안 되면 "drift 제거" 페인 자체를 검증 못 하므로 In이 아니라 Partial/Out.
2. **문법 의미론**: 렌더 대상 env를 가리키는 current-env 참조(`${{ self.KEY }}`)와 cross-env 참조(`${{ shared.KEY }}`)가 둘 다 동작하는가? 기존 resolver가 이미 cross-env를 지원하므로 이를 숨기지 않아야 한다.
3. **스코프 비용**: envvar/resolver 모델 안에 머무는가(클라이언트측 render), 아니면 별도 시스템(서버측 templating, file mount sync, reload 오케스트레이션)을 끌어들이는가? (PRD 최상위 리스크 "스코프 폭주"의 직접 판정)
4. **apply 경계**: 파일 렌더 ≠ 반영. mount sync + service reload는 어디까지 M4 webhooks로 커버되고, 어디부터 신규 범위인가? (활성 config **덮어쓰기**는 PoC 밖 — 아래 F3 참조)
5. **권고**: In(v1, 클라이언트측 render로 스코프 한정) / Out(v2로 연기, v2 1순위 예약) / Partial.

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| 참조 문법 | `internal/secret/reference.go:33` | `${{ env.KEY }}` 정규식(`referencePattern`) + `findReferences` — nginx `$var`와 충돌 없는 delimiter. 문법은 이걸 단일 진실원으로 두고 parity 테스트로 drift 방지 |
| 클라이언트측 materialize | `cmd/cli/cmd_pull.go:82` (`writeAtomic`) | tempfile→rename 원자적 쓰기. 렌더도 동일 경로 재사용 |
| 명령 골격 | `cmd/cli/cmd_export.go:28` | `loadContext` → `client.ListSecrets(project, env)` → 포맷 방출. render는 여기에 "템플릿 채우기"만 추가 |
| 명령 등록 | `cmd/cli/main.go:51` | `newRootCmd`의 `root.AddCommand(...)`에 `newRenderCmd(st)` 추가 |
| Fail-closed 안전 | `cmd/cli/cmd_export.go:104` (`emitGithubEnv`) | 대상 미지정/전제 미충족 시 조용한 no-op 대신 hard error |
| 에러 래핑 | `internal/secret/resolver.go:127` | `fmt.Errorf("op: %w", err)` + 도메인 sentinel |
| 테스트 스타일 | `cmd/cli/cmd_export_test.go:15` | `newRootCmd()` + `SetArgs` + stdout/stderr 분리 캡처, `loggedInWorktree` 픽스처 |
| 테이블 테스트 | `internal/secret/reference_test.go` | 순수 함수는 table-driven; 시크릿 값은 로그/에러에 노출 금지(테스트로 강제) |

## Files to Change

| File | Action | Why |
|---|---|---|
| `internal/tmpl/render.go` | CREATE | 의존성 없는(regexp+strings) 순수 render 함수. CLI cold-start 300ms 예산 때문에 `internal/secret`(sql/crypto/store)를 CLI에 끌어오지 않으려는 leaf 패키지 |
| `internal/tmpl/render_test.go` | CREATE | table-driven: 치환/누락키 fail-closed/cross-env-ref 에러/리터럴 `$` 보존/multiline 값 |
| `internal/tmpl/parity_test.go` | CREATE | `${{ }}` 문법이 `internal/secret`의 `referencePattern`과 동일함을 강제(drift 방지). test-only import이므로 CLI 바이너리에는 무영향 |
| `internal/secret/reference.go` | UPDATE | parity 테스트가 읽을 수 있게 패턴을 export(`ReferencePattern`) — 서버 동작 무변경, 이름만 공개 |
| `cmd/cli/cmd_render.go` | CREATE | `secret render` 명령: `loadContext`→`ListSecrets`→`tmpl.Render`→`writeAtomic`(0600). `--template`, `--out`(`{env}` 치환 지원), `--env`, `--quiet` |
| `cmd/cli/cmd_render_test.go` | CREATE | CLI 통합 테스트, `cmd_export_test.go` 미러: 렌더 성공/누락키 에러/템플릿 부재 에러/`{env}` out 치환 |
| `cmd/cli/main.go` | UPDATE | `newRootCmd`에 `newRenderCmd(st)` 등록 |
| `docs/poc/redis.conf.tmpl` | CREATE | 실 PoC 픽스처(운영 redis.conf 기반, `${{ self.REDIS_MAXMEM }}` — env 하드코딩 금지, 단일 템플릿이 3환경으로 렌더) |
| `docs/poc/nginx.conf.tmpl` | CREATE | 실 PoC 픽스처(`$host` 리터럴 + `${{ self.TLS_CERT_PATH }}` + cross-env `${{ shared.UPSTREAM }}` 혼재 — 충돌 없음 + cross-env 실증) |
| `.gitignore` | UPDATE | `tmp-render/`(render staging 출력) 무시 등록 — 시크릿 담은 산출물이 커밋되지 않도록 |
| `.claude/reports/comax-secrets-m7-infra-config-templating.report.md` | CREATE | Decision Criteria 4항목 증거 + In/Out 권고(마일스톤 산출물) |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | M7 행 status(pending→in-progress, 완료 시 결정 반영) + Plan 셀 링크; Open Question #5 결론 stamp |

## Tasks

### Task 1: `internal/tmpl` 순수 render 코어 (Codex F1 흡수)
- **Action**: 예약 namespace **`self`**를 도입해 "단일 템플릿 → N환경 렌더"를 지원한다. 템플릿은 env를 하드코딩하지 않고 `${{ self.KEY }}`로 렌더 대상 env의 키를 참조 → `--env prod` 렌더 시 prod 값, `--env local` 렌더 시 local 값. cross-env 참조 `${{ shared.KEY }}`도 숨기지 않는다(기존 resolver가 지원). 시그니처(의미 수준): `References(tmpl) []Ref`(CLI가 사전 스캔해 필요한 env를 파악) + `Render(tmpl, currentEnv string, snapshots map[string]map[string]string) (rendered string, missing []Ref, err error)` — `self`는 `currentEnv`로 해석, 타 env는 CLI가 사전 fetch해 `snapshots`에 담음. 누락 키/env는 `missing`에 수집 후 **fail-closed**. `${{ ... }}` 아닌 `$`(nginx `$host` 등)는 리터럴 보존.
  - **`self` 예약어 결정론(impl-F2)**: `${{ self.KEY }}`는 문맥과 무관하게 **항상 current render env**로 치환한다(alias vs cross-env 문맥 의존 제거 → "조용히 다른 secret 렌더" 위험 차단). 가드: `currentEnv == "self"`(운영자가 실제로 env를 `self`로 만든 모호 상황)이면 명시적 에러("'self' is reserved as the current-env alias"). 서버 env-name 생성 단계에서 `self`를 예약어로 거부하는 것은 v1-full 몫으로 **DEFER**(backlog; PoC는 서버 변경 0 유지).
  - **에러에 시크릿 절대 금지(sec#2/#10 — CLAUDE.md 하드 규칙 "시크릿은 절대 로그에 남기지 않는다")**: 렌더/치환 실패 시 에러 메시지에는 **키 이름과 위치만** 담고 resolved 값은 절대 포함하지 않는다. `%v/%q/%s`로 secret 값을 포매팅하는 경로 없음. 전용 테스트 `TestRender_NoSecretsInErrorMessages`(시드된 카나리 값이 어떤 실패 경로의 stderr/error에도 안 나옴)로 강제. (기존 `internal/secret/resolver.go`가 이미 값 아닌 키명으로 에러 — 동일 규약 준수.)
- **Mirror**: `internal/secret/reference.go`의 `referencePattern`/`findReferences` 문법(첫 세그먼트에 `self` 예약어만 추가); 역순 splice로 바이트 오프셋 유지(`resolver.go:200`).
- **Validate**: `go test ./internal/tmpl/...` — `self` 치환, 단일 템플릿이 3환경으로 서로 다르게 렌더, cross-env 참조, 누락 키 fail-closed, 리터럴 `$` 보존, multiline 값 케이스 그린, 커버리지 ≥80%.

### Task 2: `secret render` CLI 명령 (Codex F2·F3 흡수)
- **Action**: `newRenderCmd(st)` 추가. `loadContext`로 (creds, project, env) 해결 → `tmpl.References`로 참조 env 집합 파악 → 각 env를 `client.ListSecrets` 한 번씩 fetch(캐시) → `tmpl.Render`. `missing`가 있으면 키/env 목록과 함께 에러(fail-closed).
- **출력 정책(F2/impl-F1 — 시크릿 노출 fail-closed)**: 렌더 산출물은 원문 시크릿(`requirepass` 등)을 담으므로:
  - `--out` 대상은 **worktree 안의 gitignored staging 경로만** 허용한다. (impl-F1: "worktree 밖 허용"은 `/etc/redis/redis.conf` 같은 활성 config를 통과시켜 staging-only 원칙과 충돌하므로 **제거**.)
  - 경로 판정은 **`{env}` 치환 + symlink 정규화(canonical) 후의 최종 경로**를 기준으로 한다. 판정 순서 역전으로 인한 우회 금지.
  - gitignored 여부는 최종 경로의 canonical parent가 속한 worktree에서 `git check-ignore`로 확인. **git 부재 / rev-parse·check-ignore 오류 / bare repo / 다른 worktree 소속 / 미등록 경로 → 모두 hard error**(경고 아님).
  - 퍼미션 **0600** 강제. temp 파일도 쓰기 전에 명시적 `os.Chmod(tmp, 0600)`(sec#6 — `CreateTemp` 기본 의존 금지). `writeAtomic` 대신 `writeAtomicSecret` 변형 사용.
  - stdout 출력(`--out -`) **미지원**(sec#8 — config 파일 생성이 목적이라 불필요. 가드보다 제거가 더 fail-closed). `--allow-stdout` 플래그도 없앤다.
  - **`--out` 리터럴 경로만**: `${`/`{{` 등 템플릿 마커 포함 시 거부(sec#3). 허용 치환은 `{env}` 토큰 뿐이며, env 이름 charset은 `[A-Za-z0-9_.+-]`(validateName)라 `/`·`..` 주입 불가.
  - **symlink 하드닝(sec#1/#9/#11 — 방어적, 위협모델상 blocking 아님)**: 판정 직후가 아니라 **쓰기 직전 재정규화** + 최종 open은 `O_NOFOLLOW`(Unix; Windows는 symlink stat 감지 후 에러). staging 부모 디렉토리가 world-writable(`mode & 0o022 != 0`)이면 거부.
  - `--quiet`는 상태 배너만 억제하고 **보안 거부/경고는 억제하지 못한다**(테스트로 강제). `--out` 파일 덮어쓰기 시 non-quiet면 stderr 고지(sec#12).
- **파일 교체 정책(F3 — 활성 config 안전)**: PoC는 **staging 경로에만 렌더**한다. 활성/라이브 config를 in-place로 덮어쓰지 않는다. `writeAtomic`의 delete-then-rename 공백(Windows Rename 제약)이 활성 config에선 서비스 기동 실패/롤백 불능을 유발할 수 있으므로, 안전한 atomic replace(백업·rollback·fsync·OS별 전략 + 실패주입 테스트)는 **v1-full로 DEFER**(backlog 기록). `--out`이 `{env}` 토큰을 해석된 env로 치환(`redis.{env}.conf`→`redis.prod.conf`)해 staging 파일명을 만든다.
- **Mirror**: `cmd_pull.go`(writeAtomic/loadContext), `cmd_export.go`(플래그·분기·fail-closed 에러).
- **Validate**: `./bin/secret render --template docs/poc/redis.conf.tmpl --out ./tmp-render/redis.{env}.conf --env local`(gitignored `tmp-render/`) 스모크 — 렌더 파일 생성·값 주입·0600 확인; 미등록 경로 지정 시 에러; `--quiet`가 보안 거부를 숨기지 않음.

### Task 3: PoC 증거 수집 (staging-only)
- **Action**: 운영자의 실 redis.conf / nginx.conf를 **단일 템플릿**으로 템플릿화(`docs/poc/*.tmpl`, `${{ self.KEY }}` 사용). local/dev/prod 3환경에 대해 render 실행(gitignored `tmp-render/`로만 출력), 손유지 버전과 diff 캡처. **apply 경계** 관찰 기록: 렌더 후 파일 배치(mount)·서비스 reload·**활성 config in-place 교체**는 별도 시스템인가, M4 webhook(secret 변경→service restart)으로 어디까지 닿는가.
- **Mirror**: 없음(신규 관찰). 측정 결과는 report에 표로.
- **Validate**: 3환경 × 2파일 = 6개 staging 렌더 산출물이 손유지 버전과 의미상 일치(diff 검토). **단일 템플릿이 env별로 다른 값으로 렌더됨**을 확인(F1 핵심). multiline 값(예: TLS 인증서 블록) 무손실 확인.

### Task 4: Decision 문서화 + PRD 반영
- **Action**: report에 Decision Criteria 4항목별 증거와 **In/Out 권고** 작성. PRD M7 행 status·Plan 갱신, Open Question #5에 결론 stamp. Out/Partial이면 잔여를 `.claude/plans/codex-findings-backlog.md`에 v2 1순위로 기록.
- **Mirror**: 기존 마일스톤 report 형식(`.claude/reports/comax-secrets-m*.report.md`).
- **Validate**: PRD M7 행이 in-progress→(결정 반영), OQ#5가 열림→결론. report에 4항목 모두 증거 인용.

## Validation

```bash
make build            # CLI/서버 컴파일 (CGO_ENABLED=0, 순수 Go)
make test             # 전체 테스트 (internal/tmpl 신규 포함)
make lint             # golangci-lint
make cover            # internal/tmpl ≥80% 확인

# render 스모크 (서버 기동 + 시드 후)
./bin/secret set REDIS_MAXMEM=256mb --quiet
./bin/secret render --template docs/poc/redis.conf.tmpl --out /tmp/redis.{env}.conf --env local
stat -c '%a' /tmp/redis.local.conf   # 0600 확인 (win: icacls)
```

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| **스코프 폭주** (PRD 최상위 리스크) — templating이 file-render/mount-sync/reload로 번짐 | High | PoC = 클라이언트측 render 단일 명령. 서버 변경 0. apply/reload는 관찰만 하고 M4 webhook로 위임. In이어도 v1 범위를 "render"로 명시 한정 |
| **nginx `$` 충돌** — 설정 파일이 `$host` 등으로 가득 | Medium | 기존 `${{ env.KEY }}` delimiter 재사용(nginx 미사용). parity 테스트로 문법 고정. nginx.conf 픽스처로 충돌 없음 실증 |
| **시크릿 디스크 노출** — 렌더된 redis.conf에 `requirepass <secret>` (Codex F2) | Medium | `pull`이 `.env`에 시크릿을 쓰는 것과 동일 위협군이나 통제는 **fail-closed**로: 미등록 경로=에러(경고 아님), 0600 강제, stdout(`--out -`) 기본 금지·`--allow-stdout` 게이트, `--quiet`가 보안 거부를 못 숨김. threat-model 동일 가정(self-host=본인 소유) |
| **활성 config 교체 사고** — writeAtomic delete-then-rename 공백이 라이브 config 소멸 창 (Codex F3) | Medium | PoC는 **staging-only**(활성 config in-place 미교체). 안전 atomic replace(백업·rollback·fsync·실패주입)는 v1-full로 DEFER, backlog 기록 |
| **문법 drift** — leaf 패키지 정규식이 서버와 어긋남 | Low | `parity_test.go`가 `internal/secret.ReferencePattern`과 문자열 동일성 강제(CI 게이트) |
| **CLI cold-start 예산(300ms) 침해** | Low | render 로직을 의존성 없는 `internal/tmpl`에 격리; CLI가 sql/crypto/store를 물지 않음. `cmd/cli/bench_test.go`로 회귀 감시 |
| **결정 미루기** — PoC만 만들고 In/Out 안 냄 | Medium | Acceptance에 "PRD M7/OQ#5 stamp"를 명시. report 없이는 마일스톤 미완 |

## Acceptance

- [ ] Task 1~4 완료
- [ ] Validation 그린 (`make build`/`test`/`lint`, `internal/tmpl` cover ≥80%)
- [ ] **단일 템플릿 1개가 local/dev/prod로 서로 다르게 렌더**됨 실증(`${{ self.KEY }}`) + cross-env 참조 동작(F1)
- [ ] redis.conf + nginx.conf 실 렌더 증거 확보(3환경, diff 검토, multiline 무손실)
- [ ] 시크릿 노출 표면 **fail-closed** 통제 확인(0600, worktree 내 gitignored staging 경로만, `{env}`치환·symlink 정규화 후 판정, git부재/오류/bare/타worktree 거부, stdout 게이트, quiet가 못 숨김)(F2/impl-F1)
- [ ] `self` 예약어 결정론적 처리(항상 current env) + `currentEnv=="self"` 가드 에러(impl-F2)
- [ ] **에러에 시크릿 값 없음** — `TestRender_NoSecretsInErrorMessages` green(sec#2/#10, CLAUDE.md 하드 규칙)
- [ ] `--out` 리터럴만(마커 거부)·stdout 미지원·temp 0600 명시·symlink 방어(O_NOFOLLOW+부모 world-writable 거부)(sec#3/#8/#6/#1/#9/#11)
- [ ] 활성 config in-place 미교체(staging-only) — atomic-replace는 backlog로 이관 확인(F3)
- [ ] **In/Out 결정 확정** + report에 Decision Criteria 5항목 증거
- [ ] PRD M7 행 갱신 + Open Question #5 결론 stamp
- [ ] 패턴 재사용(기존 `${{ }}` 문법·writeAtomic·loadContext), 재발명 아님

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2; `--impeccable-available`)
- 라운드 수: 1 (classification=ok, blocking=false)
- 합치 결론: verdict=`needs-attention` 3건(HIGH 2 + MEDIUM 1). 전부 R1 계획 편집으로 흡수 → 0 blocking. 계획 단계 finding이라 plan 본문 수정으로 완전 해소(코드 미작성).
- YAGNI Triage:
  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | F1 current-env 전용 문법이 단일 템플릿 PoC를 무력화 | HIGH | ACCEPT_NOW | `${{ self.KEY }}` 예약 namespace 도입 — 단일 템플릿→N환경 렌더가 이 마일스톤의 핵심 페인. 없으면 drift 제거 검증 불가 |
  | F2 렌더된 시크릿 파일 노출이 warning 수준 | HIGH | ACCEPT_NOW | 미등록 경로=hard error·stdout 기본 금지(`--allow-stdout` 게이트)·quiet가 보안 거부 못 숨김 → fail-closed. 마일스톤 acceptance("노출 표면 통제")와 직결 |
  | F3 writeAtomic 재사용이 활성 config 교체에 위험 | MEDIUM | ACCEPT_NOW(scope) + DEFER(framework) | PoC를 staging-only로 한정(활성 config in-place 미교체)은 ACCEPT. backup/rollback/fsync/OS별 atomic-replace + 실패주입은 v1-full로 DEFER |
- Deferred to backlog: 1 → `.claude/plans/codex-findings-backlog.md` (F3 atomic-replace framework)
- Open Questions: 없음 (auto-CRITICAL 0 — 시크릿 노출은 fail-closed로 해소, self-host threat-model 내이며 render는 `pull`의 `.env` 쓰기와 동일 위협군)
- Codex session 참조: threadId `019f2d58-df47-7a13-8441-d9edf4b89d3e`

## Codex Implementation Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2; `--impeccable-available`)
- 라운드 수: 1 (classification=ok, blocking=false)
- 합치 결론: verdict=`needs-attention` 2건(HIGH 2). 새 implement-time 결정(안전 경로 판정 메커니즘·`self` 예약어)에 focus. 둘 다 R1 계획 편집으로 완전 흡수 → 0 blocking. dedupe 미적용(신규 결정 존재).
- YAGNI Triage:
  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | impl-F1 `--out` 안전 정책이 Git 누출·라이브 덮어쓰기를 fail-closed로 못 막음 | HIGH | ACCEPT_NOW | "worktree 밖 허용" 제거 → **worktree 내 gitignored staging 경로만**. `{env}`치환·symlink 정규화 후 판정, git부재/오류/bare/타worktree=hard error |
  | impl-F2 `self` 예약어가 실제 env와 충돌해 silently wrong secret | HIGH | ACCEPT_NOW(render) + DEFER(server) | render에서 `self`=결정론적 current-env alias 고정 + `currentEnv=="self"` 가드 에러. 서버 env-name 예약 거부는 v1-full DEFER |
- Deferred to backlog: 1 → `.claude/plans/codex-findings-backlog.md` (impl-F2 서버측 `self` env-name 예약 거부)
- Open Questions: 없음 (auto-CRITICAL 0 — 시크릿 노출은 staging-dir-only fail-closed로 해소, `self` 충돌은 결정론 alias로 해소)
- Codex session 참조: threadId `019f3005-f1c7-7c81-a773-f9555c753921`

### Security Reviewer

- 에이전트: `mccp:security-reviewer` (pre-code 설계 리뷰, secrets+경로검증 영역)
- 원 보고: CRITICAL 4 · HIGH 5 · MEDIUM 3. self-host 위협 모델(운영자가 머신·DB·마스터키 소유, "외부 노출 ≠ 운영 가정")과 PoC 스코프 기준으로 triage:

  | # | Finding | 원 Sev | Triage | 처리 |
  |---|---|---|---|---|
  | 2/10 | 렌더/치환 에러에 시크릿 값 노출 | CRITICAL | **ACCEPT_NOW** | 에러=키명+위치만. `TestRender_NoSecretsInErrorMessages` 강제. CLAUDE.md 하드 규칙 |
  | 3 | `--out` 템플릿 마커로 staging 이탈 | CRITICAL | **ACCEPT_NOW** | `--out`=리터럴만, `${`/`{{` 거부. `{env}`는 charset 안전 |
  | 8 | `--allow-stdout` 가드 부족 | HIGH | **ACCEPT(제거)** | stdout 경로 자체 삭제 → 위험 표면 제거 |
  | 6 | temp 퍼미션 미강제 | HIGH | **ACCEPT_NOW** | 쓰기 전 명시적 `Chmod(tmp,0600)` |
  | 7 | `self` 검증 타이밍 | HIGH | **ACCEPT_NOW** | `loadContext` 직후 `--env=="self"` 거부 + 테스트(impl-F2와 합류) |
  | 1/9/11 | symlink TOCTOU·world-writable 부모·rename race | CRITICAL/HIGH/MED | **ACCEPT(방어적, non-blocking)** | 쓰기 직전 재정규화 + `O_NOFOLLOW` + 부모 world-writable 거부. **위협모델상 로컬 공격자는 범위 밖**이라 blocking 아님 |
  | 5 | git check-ignore 상태 유효성 | HIGH | **ACCEPT(부분)** | `--is-inside-work-tree` 강제(이미 계획). `.gitignore` dirty 경고는 선택 |
  | 12 | 덮어쓰기 경고 부재 | MEDIUM | **ACCEPT_NOW** | non-quiet 시 덮어쓰기 stderr 고지 |
  | 4 | `--template` 임의 경로 | CRITICAL | **REJECT(security-blocking으로는)** | 템플릿은 운영자가 읽는 자기 파일(`cat`류). `/etc/redis/redis.conf.tmpl`처럼 worktree 밖 정당 사용 → worktree 제한은 실사용 파괴. 위협모델상 경계 위반 아님 |
- 종합: 실 must-fix는 에러-시크릿-노출(#2/#10)과 `--out` 마커 거부(#3). 나머지 CRITICAL(#1/#4)은 self-host 위협 모델상 blocking 아님(방어적 채택 또는 reject). 전부 구현 요구사항으로 흡수 — 미해결 auto-CRITICAL open question **없음**.
