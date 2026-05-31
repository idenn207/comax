# Local Review: feat/dashboard-ui — M2 Task 4·5·6

**리뷰 시각**: 2026-05-31
**리뷰 범위**: `master..feat/dashboard-ui` 커밋 + 워크트리 미커밋 변경
**커밋 헤드**: `eb43675`
**Decision**: REQUEST CHANGES (HIGH 2건은 미커밋 e2e에 있음)

---

## 요약

대시보드 SPA shell + 임베드 파이프라인 + 로그인/세션 라이프사이클이 깔끔하게 들어왔고, Go/SPA 양쪽 핵심 보안 동선(CSP nonce, 세션 쿠키 HttpOnly, CSRF double-submit)이 일관되게 설계되었다. 백엔드 변경분에는 머지 차단할 이슈가 없고 테스트도 모두 통과한다. 다만 **미커밋된 e2e 변경 두 건**(error-text 단언 약화, global-setup 타이머 누수)은 커밋 전에 반드시 손봐야 한다.

---

## Findings

### CRITICAL
없음.

### HIGH

#### H1. e2e 에러 단언 약화 — `web/dashboard/tests/e2e/auth.spec.ts:43` (미커밋)
워킹트리 변경으로 `toContainText('토큰이 올바르지 않습니다')` → `toBeVisible()`로 약해졌다.
```
-    await expect(page.getByRole('alert')).toContainText('토큰이 올바르지 않습니다');
+    await expect(page.getByRole('alert')).toBeVisible();
```
이제 `Login.tsx`의 `formatLoginError`에서 `unknown_token` → 한국어 메시지 매핑이 깨져도 테스트가 잡아내지 못한다(alert 컴포넌트만 떠 있으면 통과). e2e가 가지고 있어야 할 가장 핵심적인 "회귀 잡힘" 신호를 잃은 것이라 HIGH로 분류한다.

**Fix**: 원래의 텍스트 단언으로 복구하거나, 적어도 `unknown_token` 매핑이 끼지 않으면 fail이 나도록 정규식 단언으로 바꾼다.
```ts
await expect(page.getByRole('alert')).toContainText('토큰이 올바르지 않습니다');
```

#### H2. `global-setup` 타이머/exit 핸들러 누수 — `web/dashboard/tests/e2e/global-setup.ts:35-54` (미커밋)
`tokenPromise` 안에서 30초 `setTimeout`을 걸지만 해피패스에서 `clearTimeout`이 없다. `child.on('exit', onExit)`도 토큰 매치 시점에 한 번 떼지만, 매치 후 서버가 e2e 도중 죽는 경우 `onExit`이 이미 resolved된 promise에 reject를 부르고 조용히 사라진다(no-op). 결과:

1. 해피패스에서도 30초 동안 setTimeout 콜백이 event loop에 남아 있다.
2. 서버가 e2e 도중 비정상 종료해도 테스트는 다른 곳에서 hang/timeout으로 죽고, 진짜 원인인 "서버가 죽었다"는 시그널이 사라진다.

```ts
let onExit: (code: number | null) => void;
let timer: NodeJS.Timeout;
const tokenPromise = new Promise<string>((resolve, reject) => {
  onExit = (code) => reject(new Error(`secret-server exited early with code ${code}\nstdout:\n${stdoutBuf}`));
  timer = setTimeout(() => reject(new Error(`timed out waiting for bootstrap token\nstdout:\n${stdoutBuf}`)), 30_000);
  child.on('exit', onExit);
  child.stdout!.on('data', (chunk: Buffer) => {
    stdoutBuf += chunk.toString();
    const m = stdoutBuf.match(BOOT_LINE);
    if (m) resolve(m[1]);
  });
});
try {
  const token = await tokenPromise;
  // ...
} finally {
  clearTimeout(timer!);
  child.off('exit', onExit!);
}
```
e2e가 CI에서 결국 도는 코드라 신뢰성 영향이 크므로 HIGH.

### MEDIUM

#### M1. CSP `style-src 'unsafe-inline'`는 의도된 절충 — `internal/server/middleware.go:283-293`
주석에 합리화되어 있고 Radix Themes의 inline style 주입 때문이라 현실적인 선택이긴 하나, ECC 글로벌 룰(`web/security.md`)은 nonce 기반을 권장한다. 현재 nonce-able로 Radix를 끌고 갈 수 있는 옵션은 사실상 없으니, **PRD / 위협 모델에 "v1 의도된 예외" 한 줄을 명문화**하고, 후속 작업으로 Radix 측이 nonce 흘려보내기 가능해지는 시점에 회수하는 트래킹 코멘트를 남기길 권한다.

#### M2. SPA traversal guard 잘못된 prefix 매칭 — `internal/server/handlers_spa.go:74-79`
```go
if strings.HasPrefix(cleaned, "..") {
    writeError(w, http.StatusNotFound, "not_found", "no such route", s.logger)
    return
}
```
`path.Clean`은 정상 traversal을 이미 정규화하므로 진짜 위협은 없지만, 이 prefix 매칭은 `..foo` 같은 정상 파일명도 거부한다. Vite 빌드 산출물이 그런 이름을 갖지 않는다는 사실 덕분에 운영에선 무해하나, 정확하게는 다음이 맞다.
```go
if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
    ...
}
```
Defense-in-depth 의도는 살리되 false-positive를 줄인다.

#### M3. Vite `emptyOutDir: true`와 `.gitkeep` 경합 — `web/dashboard/vite.config.ts:22`, `package.json:14`
Vite가 출력 디렉터리를 비우면서 `.gitkeep`까지 지우고, `postbuild` 스크립트가 다시 만들어 넣는다. 빌드가 vite build 단계에서 실패하면 `.gitkeep`이 사라진 상태로 남아 다음 `go build`가 `//go:embed all:dist`에서 "no matching files" 에러로 깨질 수 있다.

**Fix 후보**: `postbuild`를 `prebuild`로 옮기지 말고(어차피 emptyOutDir이 또 지움), Vite가 끝난 뒤 *그리고* 실패 시에도 `.gitkeep`을 보장하도록 trap을 두거나, `Makefile dashboard` 타겟에서 vite build 직후 항상 `touch` 보장하도록 한다. 최소한 부정 경로 케이스의 회복 절차를 README에 한 줄.

#### M4. `global-setup` stderr 출력은 forwarded되지만 단언 없음 — `web/dashboard/tests/e2e/global-setup.ts:47-49`
서버가 stderr에 fatal을 찍어도(예: master.key permission), 테스트는 30s 후 generic timeout으로 죽는다. 디버그 가능성이 크게 떨어지므로, 알려진 fatal 패턴(`level=ERROR`, `failed to`)을 정규식으로 잡아 `reject(stderr-cause)` 처리하는 것을 권장. HIGH는 아니나 CI debuggability 측면에서 효과가 큰 개선이다.

### LOW

#### L1. `auth.spec.ts:19` — 불필요한 non-null assertion
```ts
if (!token) throw new Error('DASHBOARD_TOKEN missing — global-setup did not run');
await page.getByLabel('서비스 토큰').fill(token!);  // ← '!' 제거 가능
```
바로 위 `if (!token) throw`로 TypeScript narrowing 적용된 상태라 `token!`는 잡음.

#### L2. `HomePage.tsx:33-37` — error 노출 직후 즉시 redirect
`finally`에서 `router.navigate({to: '/login'})`이 항상 실행되므로, 사용자에게 잠깐 보이던 에러가 곧바로 사라진다. 의도된 UX일 수 있지만, "삭제 실패는 알려줘야 한다"가 목표라면 redirect 전에 짧은 toast로 전달하거나 `/login`에 `?logout_error=` 같은 query를 실어 한 번 보여주는 옵션 검토.

#### L3. `forceLogout`는 쿠키를 직접 못 지움 — `web/dashboard/src/lib/auth.ts:112-118`
의도된 설계(서버가 401을 줬으면 쿠키는 이미 무효)지만, `forceLogout` 후에 GET 요청이 살아 있는 쿠키로 fly할 수 있다(서버가 쿠키만 보고 401이 아니라 200을 돌려줄 수 있는 경우). 위협은 미미(GET 읽기 한정)이나 PRD에 "forceLogout 직후 GET 한 호흡은 stale read 가능"이라 명기.

#### L4. 403 코드 별 처리 미세 차이 — `web/dashboard/src/lib/api.ts:154-162`
지금은 `csrf_mismatch`만 강제 로그아웃을 트리거하고 다른 403 코드는 통과. 향후 RBAC가 들어와 `forbidden_role` 같은 코드를 돌려주면 사용자는 "왜 안 되는지" 모른 채 같은 화면에 머문다. 코멘트에 "신규 403 코드 추가 시 정책 재검토"를 남기면 좋다.

#### L5. `global-teardown.ts` Windows `process.kill` 동작
Linux/macOS는 SIGTERM 기본, Windows는 `TerminateProcess` 동기 종료. 워크플로상 e2e는 주로 CI/리눅스라 무방하나, 로컬 윈도우에서 `pnpm test:e2e` 돌릴 사람이 있다면 짧은 코멘트 한 줄.

---

## 잘 된 점 (적시할 가치 있음)

- **CSP middleware가 SPA 라우터에만 narrow하게 wrap**되어 `/api` JSON 응답에 불필요한 헤더가 안 붙음 (`router.go:50`).
- `nonceCtxKey struct{}` 패턴으로 context key collision 회피 (`middleware.go:231`).
- 임베드 모드/dev 모드 양쪽 contract를 `embed_test.go` + `embed_dev_test.go` 빌드 태그 split 테스트가 동시에 박아둠 → 회귀 잡힘 강도 높음.
- SPA traversal/직접 접근/HEAD/404 envelope 등 `handlers_spa_test.go`가 10개 가까운 케이스 다 잡고 있음 (`handlers_spa_test.go:79-308`).
- 시크릿이 로그에 새지 않는 invariant를 `logMiddleware`가 그대로 유지: body 없이 method/path/status/dur만 (`middleware.go:55-67`).
- CSRF + 세션 쿠키 double-submit이 mutating method일 때만 enforce, GET은 통과 → CLI 베어러 흐름과 깔끔히 공존 (`middleware.go:165-170, 190-196`).
- 프런트의 `apiFetch`는 `credentials: 'include'` + 세션 401/403 csrf_mismatch에 한해서만 `forceLogout` 콜백 호출 → POST `/dashboard/session` 자체의 401(잘못된 토큰)은 폼에 머무름. 로그인 화면 무한 루프 회피 (`api.ts:154-162`).

---

## Validation Results

| 검사 | 결과 |
|---|---|
| `go vet ./internal/... ./cmd/...` | ✅ Pass |
| `go build ./internal/... ./cmd/...` (default tag) | ✅ Pass |
| `go build -tags embed_dashboard ./internal/server/dashboard/...` | ✅ Pass |
| `go test ./...` (default tag) | ✅ Pass (13/13 패키지) |
| `go test -tags embed_dashboard ./internal/server/dashboard/...` | ✅ Pass |
| `go test -cover ./internal/server/...` | ✅ server 80.2%, dashboard 100.0% (≥80% 충족) |
| Frontend tests (Vitest) | ⏭️ 실행 보류 (Node toolchain 본 워크트리에 미설치) |
| Playwright e2e | ⏭️ 실행 보류 (위 H1/H2 픽스 후 권장) |

---

## Files Reviewed

### 커밋된 변경 (41 files / +8111 / -78)

**Go 백엔드**
- `cmd/server/main.go` — Modified (대시보드 FS 해석, `--dashboard-enabled` 플래그)
- `internal/server/dashboard/embed.go` — Added (`-tags embed_dashboard`)
- `internal/server/dashboard/embed_dev.go` — Added (no-tag 변형)
- `internal/server/dashboard/embed_test.go` — Added
- `internal/server/dashboard/embed_dev_test.go` — Added
- `internal/server/dashboard/dist/.gitkeep` — Added
- `internal/server/handlers_spa.go` — Added
- `internal/server/handlers_spa_test.go` — Added
- `internal/server/middleware.go` — Modified (CSP middleware + nonce 추가)
- `internal/server/router.go` — Modified (SPA fallthrough wiring)
- `internal/server/server.go` — Modified (SPAFS 옵션)

**SPA (React/TypeScript)**
- `web/dashboard/{index.html, package.json, eslint.config.js, tsconfig*.json, postcss.config.js, tailwind.config.ts, .prettierrc, vite.config.ts, vitest.config.ts, playwright.config.ts, pnpm-workspace.yaml, pnpm-lock.yaml}` — Added
- `web/dashboard/src/{main.tsx, router.tsx}` — Added
- `web/dashboard/src/lib/{api.ts, auth.ts}` (+ 각 `.test.ts`) — Added
- `web/dashboard/src/pages/{Home.tsx, Login.tsx}` (+ 각 `.test.tsx`) — Added
- `web/dashboard/src/styles/{globals.css, tokens.css}` — Added
- `web/dashboard/src/test/{setup.ts, renderWithProviders.tsx}` — Added
- `web/dashboard/tests/e2e/auth.spec.ts` — Added (이번 커밋에서 추가)

**Tooling / 문서**
- `Makefile` — Modified (dashboard 타겟, build 의존성)
- `.gitignore` — Modified (frontend 산출물 패턴)
- `.claude/prds/comax-secrets.prd.md` — Modified

### 워크트리 미커밋 변경 (4 items)

- `web/dashboard/playwright.config.ts` — Modified (globalSetup 연결, baseURL 9090)
- `web/dashboard/tests/e2e/auth.spec.ts` — Modified (skip 제거 + **에러 단언 약화 H1**)
- `web/dashboard/tests/e2e/global-setup.ts` — Untracked (**타이머 누수 H2**)
- `web/dashboard/tests/e2e/global-teardown.ts` — Untracked

---

## 권고 머지 절차

1. H1 단언 복구, H2 타이머/exit handler 정리 후 한 커밋으로 추가 (`fix(e2e): global-setup 자원 정리 + 에러 단언 복구`).
2. M2 (traversal guard 정확화)는 다음 커밋에서 같이 처리.
3. M1/M3는 별도 follow-up 이슈로 추적 (PRD/Makefile 보완).
4. Frontend 단위 + Playwright 로컬 1회 그린 확인 후 PR 생성.
