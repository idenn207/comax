# Plan: M6 — Website + Docs (Next.js, Vercel)

**Source PRD**: `.claude/prds/comax-secrets.prd.md`
**Selected Milestone**: #6 — Website + Docs (Next.js, Vercel)
**Complexity**: Large

## Summary

SEO 최적화된 마케팅 랜딩 + 본격 docs 사이트를 **리포 내 별도 Next.js(App Router) + Vercel 앱** (`website/`)으로 신설한다. M2가 PRD Open Question #4를 이미 "Two codebases"로 확정했으므로(대시보드는 서버 임베드 Vite SPA, 마케팅/docs는 별도 Next.js+Vercel), M6는 그 결정을 **상속**한다 — 재결정하지 않는다. 신규 서버/CLI/SDK 기능은 없다. 콘텐츠는 기존 `docs/*.md`·`README.md`·`action.yml`·`cmd/cli/*.go`·`sdk/`에서 **소싱**해 MDX로 옮기고, 디자인 토큰은 `web/dashboard`의 OKLCH 모노크롬 시스템을 미러링해 브랜드 정합성을 유지한다.

성공은 이분법적이다: (1) 신규 방문자가 랜딩에서 4축 USP(NAS 친화 · Worktree 1급 · GitHub Actions 통합 · Config templating)를 5초 안에 이해하고, (2) `quickstart` 문서만 따라 ≤5분에 self-host + `secret run`까지 도달하며, (3) CLI 11개 서브커맨드 · SDK · GitHub Action · Webhooks의 레퍼런스가 사이트에서 검색·탐색 가능하고, (4) `npm run build`가 Vercel에서 그대로 배포 가능한 정적/SSG 산출물을 낸다.

## Inherited & Selected Decisions (확정 제안 — CONFIRM 시 고정)

PRD Open Question **#4 — Dashboard vs Website codebase**는 M2에서 이미 해결됨. M6는 이를 상속하고, 구현을 풀기 위한 M6-내부 결정만 신규로 고정한다.

| # | 결정 | 제안 | 근거 |
|---|---|---|---|
| **#4 (상속)** | 코드베이스 분리 | 마케팅/docs = 별도 Next.js+Vercel 앱. 대시보드(`web/dashboard`)와 미공유(디자인 토큰만 미러). | M2 load-bearing 결정. 대시보드는 auth 뒤 → SSR/SEO 무의미·서버 임베드; 마케팅은 SSR/SEO + Vercel preview 필요 → 별개 운영 모델. |
| D1 | 위치 | `website/` (repo 루트, `web/`·`sdk/`와 병렬) | `web/`는 이미 대시보드 SPA 점유. published/deployed 앱이라 앱 트리와 분리. Vercel root directory = `website/`. |
| D2 | 프레임워크 | **Next.js 15 App Router + React 19 + TypeScript 5.6 strict** | PRD 고정 스택(Next.js). App Router = SSG/메타데이터/OG/sitemap 1급 지원. Vercel 네이티브. |
| D3 | 스타일 | **Tailwind 3.4** (`web/dashboard/tailwind.config.ts` 토큰 구조 미러) + **Radix UI primitives**(unstyled) | PRD 고정(Tailwind+Radix). Tailwind 3.4로 대시보드 OKLCH 토큰을 무손실 포팅. 마케팅은 bespoke 디자인이라 Radix **Themes** 대신 **primitives**(dialog/navigation-menu/accordion 등)만 사용 — anti-template. |
| D4 | 문서 저작 | **`@next/mdx` 네이티브 MDX** + `rehype-pretty-code`(Shiki 하이라이트) + `rehype-slug`/`rehype-autolink-headings` | 문서 ~12페이지 규모엔 Nextra/Fumadocs 프레임워크 락인보다 네이티브 MDX + 손수 docs shell이 디자인 완전 제어·버전 안정성 우위. 검색은 빌드타임 인덱스 기반 client-side(cmd+k). |
| D5 | 디자인 토큰 | `web/dashboard/src/styles/tokens.css`(OKLCH 모노크롬, light/dark parity)를 `website/`로 **미러**. 지금 `@comax/ui` 워크스페이스 패키지는 만들지 않음. | pnpm 모노레포 오버헤드 회피 + Vercel 빌드 단순화. M2가 언급한 `@comax/ui` 공유 패키지는 미존재(그라운딩 확인) → 후속 추출 여지만 남김. 브랜드 정합은 토큰 값 동기화로 확보. |
| D6 | 배포 | Vercel 프로젝트(root=`website/`). preview/prod 배포는 Vercel Git 연동. 실제 `vercel link`·프로덕션 승격은 **operator 수동 액션**. | M5 D7 미러(npm publish를 operator 게이트로). 본 마일스톤 acceptance = "배포 가능한 사이트 + CI build 게이트 + Vercel 설정"까지. 토큰/도메인 등록은 사람 액션. |
| D7 | SEO | Next Metadata API(per-page title/desc/OG) + `app/sitemap.ts` + `app/robots.ts` + JSON-LD(`SoftwareApplication`) + `next/og` `ImageResponse` OG 이미지 | App Router 표준. 동적 렌더 없이 SSG로 크롤 가능. |
| D8 | 콘텐츠 정합 | user-facing 문서(quickstart·self-host·CLI·SDK·action·webhooks·security)는 **website `content/docs/`를 canonical**로. repo `docs/`의 dev-internal 문서(DESIGN·dogfood·perf)는 유지. README는 website로 link-out. | 이중 source-of-truth = 이 제품이 없애려는 바로 그 페인. 사용자 대상 문서는 사이트로 단일화. |
| D9 | 국제화/CMS | v1 제외 (한국어 UI 카피 + 영어 코드/에러, i18n·CMS 없음) | 스코프 폭주 방지. PRD Non-goals 정합. |
| D10 | 팔레트 | 대시보드 중립(neutral) OKLCH 토큰 유지 + **화면당 ≤1개 restrained accent 토큰** 신설(CTA·활성 상태·위계 강조 전용) | 순모노크롬은 SaaS 대시보드엔 옳지만 마케팅 랜딩에선 one-note로 붕괴(SKILL 체크리스트). accent 1개로 랜딩 위계를 살리되 impeccable Output Constraint #2(강조색 화면당 1개) 준수. purple gradient/blob 등 stock 패턴 금지(SKILL Anti-Patterns). |

> 실제 Vercel 프로덕션 배포(D6)는 M8 Public release와 겹치는 operator 액션이므로, 본 마일스톤은 "빌드/배포 가능한 사이트 + CI 검증 + Vercel 설정 파일"까지를 acceptance로 잡는다.

## Patterns to Mirror

| Category | Source | Pattern |
|---|---|---|
| TS 툴링 | `sdk/tsconfig.json`, `sdk/package.json` | TS 5.6 / ES2022 / strict / eslint flat config / prettier. `@comax` 스코프 정렬. |
| 프론트 파일 레이아웃 | `web/dashboard/src/` (components/pages/lib/styles) | feature 단위 구성. `app/`(라우트) + `components/`(UI) + `content/`(MDX) + `lib/`(유틸). |
| 디자인 토큰 | `web/dashboard/src/styles/tokens.css`, `web/dashboard/tailwind.config.ts` | OKLCH 모노크롬, CSS 변수 토큰, light/dark parity. Tailwind theme.extend가 var() 참조. |
| anti-template 정책 | `docs/DESIGN.md`, M2 plan "Anti-template policy" | 스톡 템플릿 금지. bento 레이아웃, editorial 타이포 대비, 실제 hover/focus/active 상태. |
| Build 배선 | `Makefile` `dashboard`/`sdk` 타깃 (`cd <dir> && npm ci && npm run build`) | `website` 타깃 동일 형태 추가, `.PHONY` 등록. |
| CI 워크플로 | `.github/workflows/ci.yml`(web/dashboard job), `sdk.yml` | `website.yml` — PR(`website/**`) 시 typecheck+lint+build. Node 22. Vercel 배포는 Git 연동(별도). |
| Quickstart 콘텐츠 | `docs/quickstart.md`(70줄), `README.md` Quickstart | docker compose up → cli login → init → push → run 5단계를 MDX로. |
| Action 레퍼런스 | `action.yml`(입력 6개, 3단계 composite) | 입력 테이블 + process-env vs github-env + cleanup. `docs/github-actions.md` 병합. |
| CLI 레퍼런스 | `cmd/cli/cmd_*.go` (login/init/pull/push/get/set/diff/run/export/token/webhook) | 서브커맨드별 페이지 or 단일 reference. 플래그·예제는 각 `cmd_*.go`에서 추출. |
| SDK 레퍼런스 | `sdk/README.md`, `sdk/src/index.ts` exports | `@comax-secrets/sdk` 설치·`createClient`·`get/getAll/reload`·webhook-verify 예제. |
| Webhooks 레퍼런스 | `docs/webhooks.md`, `internal/webhook/signer.go` | 서명 검증(`sha256=hex(HMAC)`)·이벤트 타입·재시도. |
| Security 콘텐츠 | `docs/threat-model.md` | self-host 위협 모델·마스터 키 관리·운영자 의무. |

## Files to Change

### Website scaffold (`website/`, 전부 CREATE)

| File | Action | Why |
|---|---|---|
| `website/package.json` | CREATE | Next 15 + React 19 + Tailwind 3.4 + Radix primitives + `next-mdx-remote`/rehype-pretty-code/shiki/remark-gfm 스택 + **pinned `vercel` devDep**(Codex-impl F2). scripts(dev/build/start/typecheck/lint/format). Node 22 engines. |
| `website/package-lock.json` | CREATE | `npm ci` 재현성 lockfile(Codex-impl F2). vercel smoke는 `npx --no-install vercel build`로 핀 버전만 사용. |
| `website/next.config.mjs` | CREATE | `@next/mdx` 연결, `pageExtensions`(mdx 포함), security headers(`headers()`), `output` 기본(SSG). |
| `website/tsconfig.json` | CREATE | sdk/dashboard strict 베이스라인 미러 + `@/*` path alias. |
| `website/tailwind.config.ts` | CREATE | `web/dashboard/tailwind.config.ts` 토큰 구조 미러 + typography 플러그인(docs 본문). |
| `website/postcss.config.js`, `website/eslint.config.js`, `website/.prettierrc` | CREATE | dashboard/sdk 설정 미러. |
| `website/.gitignore` | CREATE | `.next/`, `node_modules/`, `.vercel/`, `out/`. |
| `website/vercel.json` | CREATE | framework=nextjs 명시 + 캐시/보안 헤더(선택). 최소 구성. |
| `website/mdx-components.tsx` | CREATE | MDX 요소 → 디자인 시스템 컴포넌트 매핑(코드블록, 헤딩 앵커, 콜아웃, 링크). |

### App Router 라우트 & 컴포넌트 (`website/`, CREATE)

| File | Action | Why |
|---|---|---|
| `website/app/layout.tsx` | CREATE | 루트 레이아웃: 폰트, ThemeProvider(light/dark), 헤더/푸터, 글로벌 metadata. |
| `website/app/globals.css` | CREATE | 포팅된 OKLCH 토큰(`tokens.css` 미러) + Tailwind base/components/utilities. |
| `website/app/page.tsx` | CREATE | 랜딩: hero + 4축 USP + features(bento) + quickstart teaser + CTA. per-page metadata. |
| `website/app/sitemap.ts` | CREATE | 전 라우트 sitemap.xml. |
| `website/app/robots.ts` | CREATE | robots.txt(allow all + sitemap 링크). |
| `website/app/opengraph-image.tsx` | CREATE | `next/og` ImageResponse 브랜드 OG 이미지(랜딩). |
| `website/app/docs/layout.tsx` | CREATE | docs shell: 사이드바 nav + 우측 TOC + prev/next + 모바일 드로어. |
| `website/app/docs/[...slug]/page.tsx` | CREATE | MDX 동적 라우트: `content/docs/**`를 렌더, `generateStaticParams`로 SSG, per-page metadata. |
| `website/components/` | CREATE | `SiteHeader`, `SiteFooter`, `Hero`, `FeatureBento`, `CodeBlock`(Shiki), `Callout`, `DocsSidebar`, `TableOfContents`, `ThemeToggle`, `CommandSearch`(cmd+k) 등 bespoke UI. |
| `website/lib/docs.ts` | CREATE | `content/docs` 파일시스템 워크: nav 트리, frontmatter, 이전/다음, 검색 인덱스 생성. |
| `website/lib/metadata.ts` | CREATE | 공통 metadata 헬퍼(title 템플릿, canonical, OG 기본값). |

### Docs 콘텐츠 (`website/content/docs/`, CREATE — 기존 소스에서 adapt)

| File | Action | Source |
|---|---|---|
| `content/docs/index.mdx` | CREATE | docs 랜딩/개요. `docs/PRODUCT.md` 톤. |
| `content/docs/quickstart.mdx` | CREATE | `docs/quickstart.md` + README Quickstart. ≤5분 walkthrough. |
| `content/docs/self-host.mdx` | CREATE | `deploy/docker/*`·`deploy/compose/*` + README. docker compose up, 마스터 키, SQLite 마운트. |
| `content/docs/cli.mdx` | CREATE | `cmd/cli/cmd_*.go` 11개 서브커맨드 레퍼런스(플래그·예제). |
| `content/docs/sdk.mdx` | CREATE | `sdk/README.md`·`sdk/src/index.ts`. 설치·`createClient`·reload·webhook-verify. |
| `content/docs/github-actions.mdx` | CREATE | `action.yml` + `docs/github-actions.md`. 입력 테이블·process-env vs github-env. |
| `content/docs/webhooks.mdx` | CREATE | `docs/webhooks.md` + `internal/webhook/signer.go`. 서명 검증·이벤트. |
| `content/docs/security.mdx` | CREATE | `docs/threat-model.md`. self-host 위협 모델·운영자 의무. |

### 리포 통합 (UPDATE)

| File | Action | Why |
|---|---|---|
| `Makefile` | UPDATE | `website` 타깃(npm ci + build) + `.PHONY` 등록. |
| `.github/workflows/website.yml` | CREATE | PR(`website/**`): typecheck+lint+build. Node 22. |
| `README.md` | UPDATE | Layout에 `website/` 추가, user-facing 문서를 website로 link-out(D8). |
| `docs/quickstart.md`·`docs/github-actions.md`·`docs/webhooks.md` | UPDATE | user-facing repo 문서를 **stub/redirect**로 축소(canonical=website 명시, Codex F2). dev-internal(DESIGN/threat-model/perf/dogfood)은 유지. |
| `website/scripts/check-docs-drift.mjs` | CREATE | **소스 계약** 대조(Codex-impl F3): `cmd/cli/*.go` 명령명 · `action.yml` 입력키 · `sdk/src/index.ts` export가 대응 MDX(cli/github-actions/sdk)에 존재하는지 + 내부 링크/앵커 무결성. repo docs stub과 무관하게 소스가 기준. |
| `website/scripts/check-site-url.mjs` | CREATE | 빌드 산출(sitemap/robots/canonical/OG)에 placeholder·localhost·빈 `SITE_URL`이 남으면 실패(Codex-impl F4 fail-closed). |
| `website/scripts/check-docs-coverage.mjs` | CREATE | nav 트리·prev/next·검색 인덱스가 `content/docs` 8종 전부 포함하는지(frontmatter 필수 필드 포함) 검증(Codex F3). |
| `website/scripts/check-token-parity.mjs` | CREATE | `web/dashboard` neutral 토큰 이름/값 parity 비교, website-only accent(D10)만 예외 허용(Codex F4). |
| `.claude/prds/comax-secrets.prd.md` | UPDATE | Milestone #6 행 `pending`→`in-progress`, Plan 셀에 본 plan 경로. |

## Tasks

### Task 1: Website scaffold + 툴체인
- **Action**: `website/` Next.js 15 App Router 프로젝트 부트스트랩(package.json/next.config/tsconfig/tailwind/postcss/eslint/prettier/.gitignore). `@next/mdx`+rehype 스택 배선.
- **Mirror**: `sdk/tsconfig.json`(strict), `web/dashboard/tailwind.config.ts`(토큰), `sdk/package.json`(scripts).
- **Validate**: `cd website && npm install && npm run typecheck && npm run build` (빈 라우트라도 그린).

### Task 2: 디자인 시스템 포팅 + 앱 셸
- **Action**: `web/dashboard/src/styles/tokens.css` neutral OKLCH 토큰을 `globals.css`로 미러 + **restrained accent 토큰 1개 신설**(D10, CTA/활성/위계 전용). `layout.tsx`(폰트·테마·헤더·푸터), `SiteHeader`/`SiteFooter`/`ThemeToggle`. light/dark parity.
- **Mirror**: `docs/DESIGN.md` 모노크롬 전략(중립 유지), M2 anti-template 정책.
- **Validate**: `npm run build` + 로컬 렌더에서 light/dark 토글 확인. accent는 화면당 ≤1(Output Constraint #2).

### Task 3: 랜딩 페이지
- **Action**: hero + 4축 USP + FeatureBento + quickstart teaser + CTA. per-page metadata + JSON-LD. accent 토큰은 primary CTA 1곳에만(D10). 정보 위계 3단계(heading depth ≤3).
- **Mirror**: bento 레이아웃, editorial 타이포 대비(anti-template). purple gradient/blob 금지(SKILL Anti-Patterns).
- **Validate**: `npm run build`; metadata/OG 태그 출력 확인. viewport당 accent ≤1 · heading depth ≤3.

### Task 4: Docs 인프라 (MDX 파이프라인 + 셸)
- **Action**: `next-mdx-remote/rsc` `compileMDX`로 `content/docs/*.mdx`를 RSC 빌드타임 컴파일(Codex-impl F1) + `rehype-pretty-code`(Shiki) + `rehype-slug`/`rehype-autolink-headings` + `remark-gfm`. `app/docs/layout.tsx`(사이드바+TOC+prev/next), `app/docs/[...slug]/page.tsx`(`generateStaticParams` + **`export const dynamicParams = false`** + 누락 slug `notFound()`), `lib/docs.ts`(nav 트리·검색 인덱스), `mdx-components.tsx`.
- **Mirror**: `web/dashboard` lib 유틸 스타일.
- **Validate**: 더미 MDX 1개로 라우팅·하이라이트·TOC·prev/next 렌더 확인 + 존재하지 않는 slug가 404(`notFound`)인지 확인 + `npm run build` SSG(전 slug 사전생성).

### Task 5: Docs 콘텐츠 8종 이관
- **Action**: `docs/*.md`·`README`·`action.yml`·`cmd/cli/*.go`·`sdk/README.md`에서 8개 MDX 작성(index/quickstart/self-host/cli/sdk/github-actions/webhooks/security).
- **Mirror**: 원문 정확성 유지, 코드/명령/경로 원문. UI 카피 한국어.
- **Validate**: `npm run build`; 내부 링크·앵커 깨짐 없음(링크 체크).

### Task 6: SEO 마감
- **Action**: `sitemap.ts`·`robots.ts`·per-page metadata·JSON-LD·`opengraph-image.tsx`. `metadataBase`/canonical/sitemap host를 `SITE_URL` env로 파라미터화(Codex F1).
- **Validate**: `npm run build` + `vercel build`(preview 산출) smoke. `SITE_URL` 주입 시 sitemap/robots/canonical/OG URL이 실 host 기준으로 렌더되는지 검사(Codex F1).

### Task 7: 리포 통합 + CI + 검증 스크립트 + PRD
- **Action**: `Makefile` `website` 타깃, `.github/workflows/website.yml`(typecheck+lint+build+`vercel build` smoke+drift/coverage/parity 스크립트), `README` link-out, user-facing repo docs stub/redirect 축소(F2), PRD 행 갱신. 3개 검증 스크립트(`check-docs-drift`·`check-docs-coverage`·`check-token-parity`) 작성.
- **Mirror**: `.github/workflows/ci.yml`·`sdk.yml` 구조.
- **Validate**: `make website` 로컬 통과; `node scripts/check-docs-drift.mjs && node scripts/check-docs-coverage.mjs && node scripts/check-token-parity.mjs` 전부 0; 워크플로 YAML lint.

## Validation

```bash
# 1. Website 로컬 검증 (Task 1~6)
cd website && npm ci && npm run typecheck && npm run lint && npm run build

# 2. Vercel 빌드 smoke + SEO URL 검증 (Codex F1 / impl-F2·F4)
SITE_URL="https://comax-secrets.example" npx --no-install vercel build   # pinned CLI, preview 산출
node scripts/check-site-url.mjs        # placeholder/localhost/빈 SITE_URL fail-closed

# 3. 검증 스크립트 (docs/토큰 계약)
node scripts/check-docs-drift.mjs      # 소스 계약(CLI/action/SDK)↔MDX + 링크/앵커 (impl-F3)
node scripts/check-docs-coverage.mjs   # nav/prev-next/search가 8종 docs 전부 포함
node scripts/check-token-parity.mjs    # dashboard↔website neutral 토큰 parity

# 4. Makefile 통합 (Task 7)
make website

# 5. (operator) Vercel 프로덕션 승격 — 수동 게이트 (D6)
#    vercel link && vercel --prod  (M8/operator 액션)
```

> Go 서버 스택은 M6에서 불변 — `make build`/`make test`는 회귀 없음만 확인(신규 Go 코드 없음).

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| **스코프 폭주** — 마케팅+docs 풀사이트 | High | 페이지 인벤토리 고정(랜딩 1 + docs 8). CMS·i18n·블로그 v1 제외(D9). |
| **문서 이중 source-of-truth drift** | Medium | website `content/docs`를 user-facing canonical로 단일화(D8). README/repo docs는 link-out + 표기. |
| **Next.js/Tailwind 버전 처닝** — Vercel 빌드 실패 | Medium | 버전 핀 고정, CI build 게이트(`website.yml`)로 잠금. Tailwind 3.4로 대시보드 토큰 무손실 포팅. |
| **디자인 품질(anti-template)** — 스톡 랜딩처럼 보임 | Medium | 대시보드 OKLCH 토큰 시스템 포팅 + impeccable design routing guide 준수 + bento/editorial 명시. |
| **콘텐츠 부정확** — CLI/action 플래그 오기 | Medium | `cmd/cli/*.go`·`action.yml` 원문에서 직접 추출, 발명 금지(CLAUDE.md). |
| **Vercel 미설정으로 CI 불완전** | Low | 배포는 operator 게이트(D6). CI는 build까지만 강제 — 배포 실패가 머지 차단 아님. |

## Acceptance

- [ ] `website/` Next.js 앱: `typecheck` 0 · `lint` 0 · `build` 성공(SSG)
- [ ] `npx --no-install vercel build`(pinned) preview smoke 성공 + `check-site-url` fail-closed 통과(Codex F1/impl-F2·F4)
- [ ] docs `[...slug]` `dynamicParams=false` + 누락 slug `notFound()` 404(impl-F1)
- [ ] 랜딩이 4축 USP를 전달 + SEO(metadata/sitemap/robots/OG/JSON-LD) 완비 + accent ≤1/viewport · heading depth ≤3(D10)
- [ ] docs 8종(index/quickstart/self-host/cli/sdk/github-actions/webhooks/security) 렌더 + 사이드바/TOC/prev-next/검색
- [ ] `check-docs-coverage` 통과 — nav/prev-next/search가 8종 docs 전부 포함(Codex F3)
- [ ] `check-docs-drift` 통과 — user-facing repo docs↔MDX 최신성 + 링크/앵커 무결(Codex F2)
- [ ] `check-token-parity` 통과 — dashboard↔website neutral 토큰 parity(Codex F4)
- [ ] CLI 11개 서브커맨드 · action 입력 6개 · SDK API가 원문과 일치
- [ ] `make website` 통합 + `website.yml` CI 게이트(build+smoke+검증 3종)
- [ ] 문서 canonical 단일화(D8) — README link-out, user-facing repo docs stub/redirect 축소
- [ ] PRD Milestone #6 행 갱신, 패턴 미러(발명 아님)
- [ ] Go 스택 회귀 없음(`make build`/`make test` 그린)

## Design Critique

- 게이트: impeccable design-critique 루프 (mccp v1.3.0-m2). detect: `skill_available=1 design_signal=1 reason=ok` (signal files: `web/dashboard/src/styles/tokens.css`, `website/app/{layout,page,globals,opengraph-image}` , `website/app/docs/*`, `website/mdx-components.tsx`).
- SKILL first-step Read: `frontend-design-direction/SKILL.md` — 4 Output Constraints(정보위계 3단계 · 강조색 화면당 1개 · raw markdown 금지 · 한 화면 항목 상한) 컨텍스트 로드 완료.
- 라운드 수: 2 (R0 발견 → 흡수 → R1 수렴)
- Verdict: **CONVERGED**
- Findings & 흡수:
  | # | Severity | Finding | 해소 |
  |---|---|---|---|
  | F1 | MEDIUM | 대시보드 순모노크롬 토큰 미러가 마케팅 랜딩에서 one-note 팔레트로 붕괴 위험(SKILL 체크리스트 "color … not collapse into a one-note palette") | **D10 신설** — neutral 유지 + 화면당 ≤1 restrained accent 토큰(Output Constraint #2 준수). Task 2/3 갱신. |
  | F2 | LOW | 랜딩 hero가 stock 패턴(purple gradient/blob/oversized hero)으로 흐를 위험 | Task 3에 SKILL Anti-Patterns 금지 명시 + editorial 타이포·bento. |
  | F3 | LOW | docs/랜딩 heading depth가 3 초과 가능 | Task 3/plan에 heading depth ≤3 명시(Output Constraint #1). implement Phase 3.7 H15 lint가 렌더 diff에서 정적 재확인. |
- 잔여 HIGH/CRITICAL: 없음.

## Design Routing Guide

routing mode: auto (implement 단계에서 유효). plan 단계는 렌더 UI가 없어 impeccable을 호출하지 않고 recommend-only. implement 시 design 게이트가 아래 stage별 impeccable 커맨드를 라우팅한다 — 여기선 체크리스트.

| Stage | Command |
|---|---|
| discovery | `/impeccable shape` |
| refine | `/impeccable layout` · `/impeccable typeset` · `/impeccable animate` · `/impeccable colorize` · `/impeccable bolder` · `/impeccable quieter` · `/impeccable overdrive` · `/impeccable delight` |
| simplify | `/impeccable adapt` · `/impeccable distill` · `/impeccable clarify` |
| evaluate | `/impeccable critique` · `/impeccable audit` |
| harden | `/impeccable harden` · `/impeccable optimize` · `/impeccable onboard` |
| polish | `/impeccable polish` |
| system | `/impeccable document` · `/impeccable extract` |

## Codex Adversarial Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2)
- 라운드 수: 1 (needs-attention, blocking=false, class=ok, HIGH/CRITICAL 0 → 재실행 불필요)
- 합치 결론: 정적 사이트라 보안 노출은 없음. 단 "빌드 성공 ≠ 정확성" — SEO URL·docs drift·nav/검색 커버리지·토큰 parity가 build만으로 검증 안 됨. acceptance/CI에 검증 게이트 추가로 흡수.
- YAGNI Triage:
  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | F1 SEO/Vercel이 실배포 없이 통과 | MEDIUM | ACCEPT_NOW | `vercel build` smoke + `SITE_URL` 기준 sitemap/robots/canonical/OG 검증을 Task 6·Acceptance·CI에 추가. |
  | F2 canonical 문서 drift 미방지 | MEDIUM | ACCEPT_NOW (부분) | `check-docs-drift.mjs` + user-facing repo docs stub/redirect 축소 흡수. "Go 소스에서 reference 자동생성(golden extraction)"은 별개 비용 → DEFER_TO_BACKLOG. |
  | F3 손수 docs shell 검색/탐색 누락 | MEDIUM | ACCEPT_NOW | `check-docs-coverage.mjs`(nav/prev-next/search가 8종 전부 포함) 추가. D4(Nextra/Fumadocs 미채택)는 유지 — 커버리지 테스트로 리스크 상쇄. |
  | F4 복사 미러 토큰 drift 계약 | LOW | ACCEPT_NOW (경량) | `check-token-parity.mjs`(neutral 토큰 parity, accent만 예외) 추가. 풀 `@comax/ui` 패키지는 과함 → CSS 스크립트로 대체. |
- Deferred to backlog: 1 → `.claude/plans/codex-findings-backlog.md` (F2 source-generated CLI/action/SDK reference)
- Open Questions: 없음 (auto-CRITICAL 0)
- Codex session 참조: threadId `019f2c14-af50-7b30-97d5-0f4dbf4d0d57`

## Codex Implementation Review

- 호출: `node C:/Users/skypark207/.claude/plugins/cache/mccp/mccp/1.20.0/scripts/lib/codex-invoke.js adversarial-review` (fail-closed Bash wrapper, v0.2.2) — implement-time 결정 focus.
- 라운드 수: 1 (needs-attention, blocking=false, class=ok, HIGH/CRITICAL 0 → 재실행 불필요)
- 합치 결론: 정적 SSG 사이트라 보안 표면 없음. MDX 라우팅 404·빌드 재현성·소스-대조 drift·SITE_URL fail-closed를 명시화하라는 4건 — 전부 흡수.
- YAGNI Triage:
  | Finding | Severity | Verdict | Why |
  |---|---|---|---|
  | impl-F1 MDX 404 미보장 | MEDIUM | ACCEPT_NOW | `next-mdx-remote/rsc compileMDX` + `generateStaticParams` + `dynamicParams=false` + 누락 slug `notFound()` 명시(Task 4). |
  | impl-F2 unpinned npx vercel | MEDIUM | ACCEPT_NOW | `package-lock.json` 필수 산출 + `vercel` devDep 핀 + `npx --no-install vercel build`. |
  | impl-F3 drift가 소스 계약 미검증 | MEDIUM | ACCEPT_NOW | `check-docs-drift.mjs`를 repo docs stub이 아닌 **소스**(`cmd/cli/*.go`·`action.yml`·`sdk/src/index.ts`) 대조로 재정의. |
  | impl-F4 SITE_URL placeholder 미차단 | MEDIUM | ACCEPT_NOW | `check-site-url.mjs` — 빌드 산출에 placeholder/localhost/빈 host 발견 시 실패. |
- Deferred to backlog: 0 신규 (impl-F3의 풀 source-generated reference는 plan-codex F2에서 이미 defer 기록; 본 사이클은 경량 소스-대조로 커버).
- Open Questions: 없음 (auto-CRITICAL 0)

### Security Reviewer

- N/A — M6는 정적 marketing/docs SSG 사이트. auth/crypto/secrets/user-input/SQL/SSRF/path-traversal 표면 없음(no server actions, no runtime user input, MDX는 저자-신뢰 빌드타임 콘텐츠). 따라서 security-reviewer 미적용(skip이 아니라 해당 없음) — 영수증에 `security_skipped` 미forward가 정확.

### Design Review

- impeccable design 게이트: SKILL_AVAIL=1. plan 단계에서 design-critique converged(D10 accent 흡수) + routing guide 기록됨. implement 단계는 pre-EXECUTE(코드 미생성)라 grounding 방향을 캡처하고, 산출 diff는 Phase 3.7 H15(heading depth ≤3) 기계 린트로 검증.
- Codex session 참조: 위 implement focus 호출(동일 wrapper).
