# Report: M6 — Website + Docs (Next.js, Vercel)

**Plan**: [.claude/plans/completed/comax-secrets-m6-website-docs.plan.md](../plans/completed/comax-secrets-m6-website-docs.plan.md)
**Milestone**: PRD #6 — Website + Docs (Next.js, Vercel)
**Branch**: `feat/m6-website-docs`

## Summary

리포 내 별도 `website/` (Next.js 15 App Router + React 19 + Tailwind 3.4 + Vercel)로
마케팅 랜딩 + docs 사이트를 신설했다. M2가 확정한 OQ#4("Two codebases")를 상속해
대시보드(`web/dashboard`, 서버 임베드 Vite SPA)와 분리했고, 디자인 토큰은
대시보드의 OKLCH monochrome 시스템을 미러링해 브랜드 정합을 유지했다(+ website-only
brand accent 1개, D10). docs 8종은 `content/docs/*.mdx`를 `next-mdx-remote/rsc`
`compileMDX`로 SSG하며, 소스(`cmd/cli`·`action.yml`·`sdk`)에서 콘텐츠를 추출해
발명 없이 작성했다. 신규 Go/서버/SDK 기능은 없다.

## Assessment vs Plan

| Metric | Plan | Actual |
|---|---|---|
| Complexity | Large | Large (일치) |
| Files changed | ~30 CREATE + 5 UPDATE | website/ 41 파일 + repo 8 UPDATE/CREATE |
| Codex gates | plan-codex + implement-codex | 둘 다 needs-attention → 8건 흡수, 0 blocking |

## Tasks Completed

| # | Task | Status |
|---|---|---|
| 1 | website scaffold + toolchain | 완료 (Next 15/React 19/Tailwind 3.4/ESLint flat) |
| 2 | 디자인 시스템 포팅 + 앱 셸 | 완료 (OKLCH 토큰 미러 + brand accent, header/footer/theme) |
| 3 | 랜딩 페이지 | 완료 (hero + 4축 bento + quickstart teaser + CTA, JSON-LD) |
| 4 | docs 인프라 (MDX + 셸) | 완료 (compileMDX + Shiki + sidebar/TOC/prev-next/cmd+K 검색) |
| 5 | docs 콘텐츠 8종 | 완료 (index/quickstart/self-host/cli/sdk/github-actions/webhooks/security, 1377줄) |
| 6 | SEO 마감 | 완료 (sitemap/robots/per-page metadata/JSON-LD/OG) |
| 7 | 리포 통합 + CI + 검증 스크립트 | 완료 (Makefile website 타깃, website.yml, 4 verify 스크립트, docs stub, README) |

## Validation Results

| Level | Status | Notes |
|---|---|---|
| Static (typecheck) | PASS | `tsc --noEmit` 0 errors |
| Static (lint) | PASS | `next lint` 0 warnings/errors |
| Build | PASS | `next build` SSG 15 routes (landing + docs 8 + sitemap/robots/OG) |
| Verify scripts | PASS | token-parity·docs-coverage(8)·docs-drift(11 CLI·8 action·8 SDK)·site-url(fail-closed 확인) |
| Runtime smoke | PASS | landing/docs/sitemap/robots serving; unknown doc → 404; OG → image/png |

## Codex Absorptions

- **plan-codex** (4 MEDIUM/LOW): F1 vercel/SEO 실배포 검증 → `check-site-url` + `vercel build` 언급; F2 docs drift → `check-docs-drift`(소스 대조); F3 손수 docs shell 커버리지 → `check-docs-coverage`; F4 토큰 drift → `check-token-parity`.
- **implement-codex** (4 MEDIUM): impl-F1 MDX 404 → `dynamicParams=false`+`notFound()`(런타임 실증); impl-F2 unpinned npx → `package-lock.json` 커밋 + 아래 deviation; impl-F3 drift 소스 대조 → `check-docs-drift`가 `cmd/cli`·`action.yml`·`sdk/src/index.ts` 파싱; impl-F4 SITE_URL fail-closed → `check-site-url` strict 모드.

## Design (impeccable)

로컬 impeccable-guard hook이 UI 파일 작성 전 impeccable 호출을 강제 → `impeccable shape`
실행. PRODUCT.md/DESIGN.md의 committed monochrome 정체성(GitHub 레퍼런스, "색은 의미에만")을
identity-preservation 규칙에 따라 보존. 새 SaaS-인디고 accent 발명 대신 기존 blue-250을
링크·1개 강조에만 사용. side-stripe border(impeccable Absolute ban) 자체 검열, heading
depth ≤3 준수(sdk.mdx `####`→`###` 수정).

## Deviations from Plan

1. **MDX 렌더링**: 플랜의 `@next/mdx`(초안) → `next-mdx-remote/rsc compileMDX`로 확정
   (implement-codex F1 흡수, `content/docs` 분리 유지 위해). 플랜 Files to Change와 정합.
2. **vercel devDep 미설치**: 플랜/impl-F2는 `vercel` devDep 핀 + `npx --no-install vercel build`를
   명시했으나, ~100MB CLI를 node_modules에 넣지 않고 **`next build`를 hermetic 게이트**로 채택.
   근거: `next build`가 SSG 전체를 검증(pinned via next)하고 Vercel 배포는 Git 연동이
   자체 빌드를 수행 → CI에 unpinned npx가 없어 F2의 재현성 우려는 더 강하게 해소됨.
   `package-lock.json`은 커밋됨.
3. **plan 아카이브 보류**: prp-implement Phase 5의 plan→completed/ 이동은 **머지 후로 연기**
   (M5 컨벤션, receipt plan_hash 경로 보존). PR 게이트가 현재 경로의 plan을 참조.

## Stop-time Review Fix (Codex)

Codex stop-time 리뷰가 지적: **Vercel 배포 경로가 SITE_URL fail-closed 검사를 우회**.
`check-site-url`이 CI/`make website`에만 있고, 실제 Vercel 프로덕션 빌드(`vercel.json`
`buildCommand`)는 이를 건너뛰어 placeholder canonical/sitemap/OG가 배포될 수 있었다. 수정:

1. `lib/site.ts` — 원점 해석에 Vercel env fallback 추가 (`SITE_URL` → `VERCEL_PROJECT_PRODUCTION_URL` → `VERCEL_URL` → localhost).
2. `check-site-url.mjs` — 동일 precedence로 해석 + **`VERCEL === '1'`(preview·production 모든 Vercel 빌드)**에서 strict(fail-closed). `VERCEL_ENV=production`만으로 좁히면 preview 빌드가 warn-pass로 우회가 다시 열림(2차 Codex 지적) → `VERCEL=1`로 넓혀 완전 차단.
3. `vercel.json` — `buildCommand`에 `node scripts/check-site-url.mjs && next build` → **모든 Vercel 빌드가 게이트를 통과**.

검증: Vercel/no-host(preview·prod 모두) → FAIL(exit 1, 우회 차단), prod+`VERCEL_PROJECT_PRODUCTION_URL` → OK, preview+`VERCEL_URL` → OK, local(VERCEL 미설정) → WARN pass. typecheck 0.

## Files Changed (summary)

- `website/` (신규 41): app/(layout·page·docs·sitemap·robots·opengraph)·components/(shell·docs·ui)·lib/(docs·docs-nav·mdx·site·metadata·cn)·content/docs/(8 mdx)·scripts/(4 verify)·config(package/tsconfig/next/tailwind/postcss/eslint/prettier/gitignore/vercel)·mdx-components.
- repo UPDATE: `Makefile`(website 타깃), `.github/workflows/website.yml`(신규), `README.md`(link-out), `docs/{quickstart,github-actions,webhooks}.md`(stub), `.claude/prds/comax-secrets.prd.md`(#6 complete), `.claude/plans/codex-findings-backlog.md`.

## Next Steps / Follow-ups

- [ ] operator: Vercel 프로젝트 링크(root=`website/`) + `SITE_URL`/`SITE_URL_REQUIRED=1` 설정 → 프로덕션 배포 (D6, M8과 정합).
- [ ] design: 라이브 브라우저 impeccable critique/polish 패스(반응형·모션 실측) — 후속 사이클.
- [ ] 머지 후: plan을 `.claude/plans/completed/`로 아카이브, PRD #6 merge SHA 기입.
