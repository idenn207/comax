# 랜딩 재디자인 핸드오프 (다음 세션용)

> 2026-07-07 작성. 비용 상한($145)으로 세션을 끊고 새 세션에서 이어감.
> 이 문서 + 아래 "다음 세션 프롬프트"만 있으면 바로 재개 가능.

## 한 줄 요약

`website/` 랜딩을 **claude_design 프로젝트의 최종 렌더 이미지**에 맞춰 전면 재디자인한다.
직전 구현(아래 "현재 상태")은 품질·AI 슬롭·깨짐·문구·구성·레이아웃 전반이 부족하다고 사용자가
평가했다. **`.dc.html`(코드 템플릿)이 아니라 디자이너의 완성 렌더 PNG를 계약으로 삼는다.**

## 기준 이미지 (claude_design)

- 프로젝트: `84e1e0ca-8cde-4380-8bfc-6f98528b9606` (DesignSync = "claude_design MCP").
- 최종 렌더 후보: `scrap/home-final.png`(전체 구성), `scrap/v2-hero.png`·`v2-hero2.png`·`v2-hero3.png`(히어로 반복), `scrap/home-check.png`·`home-check2.png`, `scrap/bottom.png`.
- 코드 시안: `Comax Home v2.dc.html`(직전에 이걸 구현함), `Comax Home.dc.html`, `Comax Docs.dc.html`.
- ⚠️ **비용 주의**: `DesignSync get_file`은 이미지를 **base64 텍스트**로 반환 → 한 장에 수십만 토큰.
  **권장: 사용자가 claude.ai/design에서 PNG를 직접 다운로드해 대화에 첨부**(이미지 블록이 훨씬 저렴).
  꼭 툴로 가져와야 하면 1~2장만.

## 현재 상태 (working tree, 미커밋)

직전 세션에서 `Comax Home v2.dc.html`을 구현했고 **teal 정체성을 monochrome graphite + blue-250
액센트로 되돌림**(대시보드와 재통합). 빌드·검증은 전부 green이었으나 사용자가 비주얼 품질 부족을 지적.

- 커밋됨(cce046c, feat/website-redesign): 폐기된 **teal** 버전(hero-graphic/feature-tabs).
- 미커밋 변경(현재): teal→mono+blue 랜딩. 재디자인의 출발점이자, 필요하면 갈아엎어도 됨.

변경/신규 파일:
- 신규: `components/hero-stage.tsx`(수렴 그래픽), `components/secret-demo.tsx`(인터랙티브 데모, client), `components/motion-ready.tsx`(reveal, client).
- 재작성: `app/page.tsx`(10섹션), `components/ui/button-link.tsx`(DS 변형), `components/site-header.tsx`·`site-footer.tsx`.
- 수정: `app/globals.css`(brand=blue 별칭, info/success-strong 추가, 랜딩 모션·hero stage CSS), `tailwind.config.ts`(info.strong/success.strong).
- 폐기: `components/hero-graphic.tsx`, `components/feature-tabs.tsx`.
- 문서: `website/DESIGN.md`·`PRODUCT.md`, 루트 `CLAUDE.md`(teal→mono+blue 반영).

## 확정된 제약·결정 (유지)

- **register = brand**. 컨텍스트: `website/PRODUCT.md` + `website/DESIGN.md`.
- **팔레트**: monochrome graphite(hue 260) + 단일 blue-250 액센트. Primary CTA는 무채색 채움("색은 의미에만"). teal 금지.
- **token-parity**: 뉴트럴·시맨틱 전부 대시보드와 parity(`npm run check:token-parity`). `info-strong`/`success-strong`는 대시보드에 없어 website에 추가해도 교집합 검사 안 걸림.
- **폰트**: 시스템 Pretendard 폴백 체인만(자체 호스팅 웹폰트·CDN 금지, CSP).
- **copy**: em dash(`—`)·`--` 금지 → 마침표/콜론/괄호. buzzword 금지. 한국어 평어.
- **범위**: 홈 랜딩(`app/page.tsx`) + 공용 크롬(header/footer/button). docs 페이지 내부는 유지.
- 링크 매핑: 무료로 시작→`/docs/quickstart`, CLI→`/docs/cli`, self-host→`/docs/self-host`, 대시보드 둘러보기→`#demo`, 홈앵커는 `/#how`·`/#demo`.

## 사용자가 지적한 부족한 점 (재디자인 시 집중)

품질 전반 / AI 슬롭 / **깨지는 디자인** / 문구 / 구성 / 레이아웃. → 렌더 이미지의 정확한 간격 리듬,
타이포 위계·밀도, 섹션 구성, 반응형 안정성을 픽셀 단위로 맞출 것. 번호 kicker(01~05)·eyebrow가
AI 슬롭으로 읽힐 수 있으니 렌더가 실제로 그렇게 생겼는지 확인하고, 아니면 impeccable 금지 규칙대로 재구성.

## 기술 게이트·함정 (다음 세션이 반드시 알 것)

- **impeccable-guard 훅**: UI 파일 편집 전 반드시 `Skill(skill="impeccable", args="craft ...")`로 스킬을 **실제 호출**해야 함(슬래시 커맨드 로드만으론 차단됨).
- **impeccable setup**: `node "C:/Users/skypark207/.claude/skills/impeccable/scripts/context.mjs"` (프로젝트 로컬 아님, 스킬 홈에서 실행). 참고 레퍼런스: `reference/craft.md`, `reference/brand.md`. 네이티브 이미지 생성 없음 → 목업 생성 단계 skip, 첨부 렌더가 계약.
- **Playwright**: 리포에 website용 미설치. `dashboard/node_modules/playwright`를 절대 file URL로 import(`import pw from 'file:///.../dashboard/node_modules/playwright/index.js'; const {chromium}=pw;` — CommonJS라 default import).
- **reveal 시스템**: `.reveal`은 `html.is-ready`에서만 `opacity:0`. Playwright `fullPage` 캡처 시 **스크롤로 IntersectionObserver를 트리거**해야 하단 섹션이 보임(스크롤 없으면 빈 페이지처럼 찍힘 — 코드 버그 아님).
- **히어로 stage 모바일 오버플로우**: 칩이 `left:75~78%`+nowrap이라 좁은 뷰포트서 넘침. 현재 `overflow-x-clip` + 모바일 `scale-[0.82]`로 처리. 렌더 기준으로 재검토.

## 검증

```
cd website
npm run verify   # typecheck + lint + build(SSG) + docs-drift + docs-coverage + token-parity + site-url
```
스크린샷: dashboard playwright로 desktop(1440)/tablet(820)/mobile(390) × light/dark, 스크롤 후 fullPage,
인터랙션(데모 prod탭→"빠짐" 배지, reveal 토글) 확인.

## 다음 세션 프롬프트 (그대로 붙여넣기)

아래를 새 세션 첫 메시지로. **렌더 PNG는 이 메시지에 이미지로 첨부**하면 가장 저렴/정확하다.

```
/impeccable craft website/ 랜딩 재디자인. website/REDESIGN-HANDOFF.md를 먼저 읽어라(전체 컨텍스트·제약·함정 정리됨).

기준: 첨부한 claude_design 최종 렌더 이미지(home-final / v2-hero / bottom)에 픽셀 단위로 맞춘다. .dc.html 코드가 아니라 이 렌더가 계약이다. 직전 구현(working tree의 teal→mono+blue 랜딩)은 품질·구성·레이아웃·문구가 부족하니, 렌더에 맞춰 필요한 만큼 갈아엎어도 된다.

유지할 제약(HANDOFF 문서 참조): register=brand, monochrome graphite+blue-250(teal 금지, primary는 무채색 채움), token-parity 유지, 시스템 Pretendard만, em dash 금지, 범위=랜딩+공용 크롬(docs 내부 유지). 스크린샷은 dashboard/node_modules의 playwright로 검증. 끝나면 npm run verify 그린 확인.

(렌더 이미지가 없으면: claude_design 프로젝트 84e1e0ca의 scrap/home-final.png 등을 DesignSync로 가져오되 base64 비용이 크니 1~2장만.)
```
