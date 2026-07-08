# Design

> Comax Secrets **마케팅·문서 사이트**(`website/`)의 시각 시스템. 전략 컨텍스트는
> [website/PRODUCT.md](PRODUCT.md). 운영자 대시보드([docs/DESIGN.md](../docs/DESIGN.md),
> monochrome)와 **뉴트럴·시맨틱 토큰 전부 parity 유지** — 랜딩도 같은 시스템 위에 산다.

`feat/website-redesign`에서 확정. claude_design 프로젝트의 `Comax Home v2` 시안을
가져와(claude_design MCP) 구현. 짧게 시도했던 teal 독립 정체성은 폐기하고, 사이트를
대시보드와 한 몸으로 재통합했다.

## Register

brand (impeccable register: 디자인이 곧 설득). 단, 팔레트는 대시보드를 그대로 계승한다 —
설득은 색이 아니라 **구조·타이포·리듬·실증(인터랙티브 데모)**으로 한다.

## Color strategy — monochrome graphite + 단일 blue 액센트

대시보드와 동일한 **monochrome graphite(hue 260, 저채도)**. Primary 액션·현재·선택은
색이 아니라 **표면 elevation + border-strong + 굵기 대비**로 표현한다. 유일한 유채색 노트는
**blue 250**(focus-ring·`--color-info`와 같은 계열)로, 링크·히어로 강조어·스텝 번호 같은
**액센트에만** 쓴다. 시맨틱(success/danger/warning/info)은 상태에만.

### 왜 이 색인가

- teal 독립 정체성 실험은 폐기. 이유: (1) 시크릿 도구에서 브랜드색이 시맨틱과 경쟁하면
  "색=의미" 신호가 흐려진다. (2) 대시보드와 "다른 회사"로 읽혀 한 제품감이 깨졌다.
- Primary CTA는 **monochrome graphite 채움**(대시보드 미러). "색은 의미에만"을 랜딩도 지킨다.
- blue 250은 이미 focus-ring·info로 존재하던 값 → 새 액센트를 발명하지 않고 계승. AI-SaaS
  인디고 그라디언트(PRODUCT.md anti-reference)를 피한다.

### Brand 토큰

`--color-brand*`는 이제 **공유 blue 250의 별칭**(= `--color-info` 계열). 링크·인라인
액센트·로고 점·`::selection`이 이 값을 참조한다. 랜딩은 대체로 시맨틱 `--color-info`를 직접
쓰고, docs prose 링크·TOC는 `brand`를 쓴다(같은 색).

| 토큰 | Light | Dark | 용도 |
|---|---|---|---|
| `--color-accent` | `oklch(22% 0.01 260)` | `oklch(96% 0.004 260)` | **Primary 채움(monochrome)** |
| `--color-brand`(=info) | `oklch(50% 0.14 250)` | `oklch(72% 0.14 250)` | 링크·히어로 강조어·스텝 번호 |
| `--color-info-strong` | `oklch(38% 0.14 250)` | `oklch(84% 0.14 250)` | 링크 hover, info-soft 위 텍스트 |
| `--color-success-strong` | `oklch(36% 0.12 150)` | `oklch(82% 0.12 150)` | success 카드 헤더·배지 텍스트 |
| `--color-danger-strong` | `oklch(38% 0.2 25)` | `oklch(86% 0.2 25)` | 누락 배너·"빠짐" 배지 |

### Parity

surface / text / border / **모든 시맨틱**(danger 25 · warning 70 · success 150 · info 250) /
focus-ring(blue 250) / code 전부 대시보드와 동일. `check-token-parity.mjs`는 교집합 전체를
강제하고 예외는 없다(`--color-brand*`는 blue 별칭이라 실질적으로 parity). `info-strong`·
`success-strong`은 대시보드에 없어 교집합에서 빠지므로 추가해도 parity를 깨지 않는다.

### A11y 색 규칙

- 본문 ≥4.5:1. blue-as-link 검증 통과. 시크릿류 텍스트는 계승 규칙대로 한 단계 상향.
- **색만으로 상태 전달 금지**: 누락·성공·위험은 항상 아이콘+라벨 동반(데모의 "빠짐" 배지,
  before/after 카드의 ✕/✓). blue↔green↔red 색약 혼동을 역할·라벨로 차단.

## Typography

- Family: **Pretendard Variable**(한글+Latin) → Apple SD Gothic Neo → Malgun Gothic → system.
  woff2를 `public/fonts/`에 **self-host**(same-origin이라 CSP `font-src 'self'` 충족, CDN 아님).
  claude_design 시안이 Pretendard로 조판됐고 OS 폴백(맑은 고딕)은 더 넓어 히어로·CTA 헤딩이 한 줄
  더 쪼개지므로, 시안과 1:1로 맞추려 번들한다. 디스플레이 폰트는 따로 없음. mono는 시스템 mono 스택.
- **Refined semibold**: 히어로 h1은 `clamp(2.5rem, 6vw, 4.1rem)` / weight 600 /
  `letter-spacing: -0.035em`. 섹션 h2는 `--text-3xl` / 600. 굵기 계층 400 본문 / 500 라벨 /
  600 강조·헤딩. 극적 대비가 아니라 **절제된 대비 + 정밀한 간격**으로 계층을 세운다.
- **mono 텍스처**: 수치·명령명·시크릿 키·eyebrow 라벨을 `--font-mono`로. 실제 데이터·명령에
  쓰고, 검은 배경 costume은 쓰지 않는다(터미널 카드는 인용).
- 한글 줄바꿈: h1~h3 `text-wrap: balance` + `word-break: keep-all`, 본문 `text-wrap: pretty`.

## Layout

섹션 척추(10섹션): **헤더 → 히어로 → 시크릿 설명 밴드 → why(before/after) → how(3스텝) →
benefits(4) → 인터랙티브 데모 → 개발자/CLI → 최종 CTA → 4열 푸터**.

- **히어로**: 비대칭 2열(카피 리드 + 수렴 그래픽). 강조어 1개만 blue. primary CTA는 monochrome
  채움. 배지·메타는 mono. 모바일에서 그래픽은 카피 아래로 stack + `scale`로 프레임 유지.
- **수렴 그래픽(`HeroStage`)**: 흩어진 시크릿 칩 → 중앙 vault로 수렴. 순수 CSS 키프레임,
  장식이라 `aria-hidden`. 히어로 카피가 의미를 진다.
- **번호 kicker(01~05)**: 순차 내러티브(왜→동작→베네핏→미리보기→개발자)를 진짜로 나타내므로
  사용. 장식 eyebrow가 아니라 읽는 순서 정보.
- **인터랙티브 데모(`SecretDemo`)**: 스톡 이미지 대신 제품 축소판. env 전환·값 reveal·누락
  표시로 "누락은 1급 신호"(PRODUCT 원칙 3)를 show-don't-tell.
- 공용 컨테이너 `max-w-content`(72rem) + `px-4 sm:px-6` — 헤더/푸터와 좌우 정렬선 공유.
- 반응형: 히어로·데모·개발자 grid → 모바일 1열. 헤드라인 오버플로우 각 breakpoint 검증.

## Motion

- 계승: 120/200/360ms, `ease-out-expo`/`ease-out-quart`, transform·opacity·shadow·border만.
- **reveal 시스템**: `.reveal`은 `html.is-ready`(클라이언트 `MotionReady` 마운트 시 부착)에서만
  `opacity:0`이 걸린다 → **JS·헤드리스 렌더에서 콘텐츠가 완전히 보임**. IntersectionObserver로
  스크롤 진입 시 켠다. 히어로 칩 수렴/vault pop/glow는 CSS 키프레임(is-ready 게이트).
- `prefers-reduced-motion`: 전역 0.01ms + reveal/히어로 애니메이션 즉시 표시 override.

## Components

| 컴포넌트 | 패턴 |
|---|---|
| `ButtonLink` | primary(monochrome accent 채움) / outline / soft / ghost, size sm·md·lg |
| 링크(prose·인라인) | `--color-brand`(=blue), underline offset |
| `HeroStage` | 수렴 그래픽(정적 마크업 + CSS 키프레임), aria-hidden |
| `SecretDemo` | env segmented control + 마스킹/reveal + 누락 배너·배지 (client) |
| `MotionReady` | `is-ready` 부착 + `.reveal` IntersectionObserver (client, 렌더 없음) |
| before/after 카드 | danger/success 계열 border-tint + ✕/✓ 리스트 |
| `::selection` | brand-soft(=info-soft) 배경 |

## A11y

- WCAG 2.2 AA. 인터랙티브 전 상태(default/hover/focus-visible/active/disabled). focus-visible ring 전역.
- 키보드 완주(랜딩→docs→검색→복사), skip-link 계승. 데모의 env 버튼·reveal은 `aria-pressed`.
- 색 비의존 상태 신호(위 색 규칙).

## Files

- `app/globals.css` — brand 토큰을 blue로 환원(별칭), `info/success-strong` 추가, 랜딩 모션·
  히어로 stage CSS.
- `tailwind.config.ts` — `info.strong`·`success.strong` 매핑 추가.
- `app/page.tsx` — 10섹션 랜딩.
- `components/` — `hero-stage`·`secret-demo`·`motion-ready`·`ui/button-link`,
  `site-header`·`site-footer`(리스킨).
- `scripts/check-token-parity.mjs` — 예외 없이 교집합 전체 parity 강제.

## What changed from the teal experiment (and why)

| 바뀐 것 | 이유 |
|---|---|
| teal 202 brand → **blue 250 환원**(= info 별칭) | 브랜드색이 시맨틱과 경쟁 → "색=의미" 흐림. 대시보드와 한 몸. |
| Committed(teal 밴드) → **monochrome + blue 액센트** | Primary는 색이 아니라 표면·굵기로. AI-SaaS 유채 밴드 회피. |
| brand parity 예외 → **예외 없음** | 랜딩이 대시보드 시스템으로 재통합. 완전 parity. |
| 4-bento / teal-flow | Comax Home v2: 수렴 그래픽 + 인터랙티브 데모 + before/after 내러티브. |
