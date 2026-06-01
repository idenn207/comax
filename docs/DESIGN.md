# Design

운영자가 시그널만 보는 self-hosted 시크릿 매니저 UI. 색은 의미에만, 폰트는 한 가족, 액션은 그 액션이 일어나는 섹션에 둔다.

## Register

product (impeccable register: SERVES the task).

## Color strategy — monochrome 100

| 토큰 | Light | Dark | 용도 |
|---|---|---|---|
| `--color-surface` | `oklch(98.5% 0.002 260)` | `oklch(15% 0.006 260)` | 페이지 배경 |
| `--color-surface-elevated` | `oklch(100% 0 0)` | `oklch(19% 0.008 260)` | 카드/테이블 표면 |
| `--color-panel` | `oklch(97% 0.003 260)` | `oklch(12% 0.005 260)` | 사이드바 |
| `--color-text` | `oklch(20% 0.012 260)` | `oklch(96% 0.004 260)` | 본문 (≥5.5:1) |
| `--color-text-subtle` | `oklch(40% 0.010 260)` | `oklch(78% 0.006 260)` | 보조 텍스트 |
| `--color-border` | `oklch(90% 0.004 260)` | `oklch(27% 0.010 260)` | 기본 외곽 |
| `--color-border-strong` | `oklch(78% 0.006 260)` | `oklch(38% 0.012 260)` | 호버/선택 외곽 |
| `--color-accent` | `oklch(22% 0.010 260)` | `oklch(96% 0.004 260)` | **Primary 채움 (모노)** |
| `--color-accent-text` | `oklch(98% 0.002 260)` | `oklch(15% 0.006 260)` | Primary 텍스트 |
| `--color-focus-ring` | `oklch(58% 0.18 250)` | `oklch(72% 0.16 250)` | 키보드 포커스 (cool blue) |

**원칙**: 현재/선택/Primary는 표면 elevation + border-strong + 굵기 대비로 표현. 채도 액센트는 금지. 포커스 링만 별도 hue(blue 250)로 분리해 "선택"과 "포커스"를 시각적으로 구분.

**Semantic (상태 전용)**: success=green(150), danger=red(25), warning=amber(70), info=blue(250). 장식 금지.

### 이 색을 고른 이유

- amber accent는 2026년 AI YouTube PPT의 saturated reflex. self-hosted 시크릿 매니저의 톤(조용함, 정직함)과 충돌.
- GitHub Primer + Linear 두 도구가 운영 도구 reference. 둘 다 monochrome filled primary + 색은 상태에만.
- Drei "amber 20% on neutral 80%"는 신호로 작동했지만, 결국 색 자체가 신호여야 한다는 가정이 잘못. 신호는 **굵기·표면·간격**의 조합으로 충분.

## Typography

- Family: **Pretendard Variable** → Apple SD Gothic Neo → Malgun Gothic → system-ui. CSP가 jsDelivr를 차단해도 OS 한글 폰트로 fall through.
- Scale: 0.75 → 2.75rem 고정 rem. clamp 없음 (데스크탑 앱 포팅 대비).
- Hierarchy: 4 단계만. xs / sm / base-md / lg-xl / 2xl-3xl.
- Weight: 본문 400, 라벨 500, 강조/Primary 600. 4K + 한글에서 500이 400처럼 보이는 회귀를 막기 위해 Radix solid 버튼은 600으로 오버라이드 (`globals.css` `.rt-Button.rt-variant-solid`).
- Line-height: 1.55 본문, 1.2 헤딩.

## Layout

### Shell

```
┌────────────┬────────────────────────────────────────────────┐
│            │ crumb LEFT │ search CENTER (lock) │ theme RT │
│  sidebar   ├────────────────────────────────────────────────┤
│            │            (page owns its header)              │
└────────────┴────────────────────────────────────────────────┘
```

- 헤더: `grid-template-columns: 1fr | var(--shell-header-search-width) | 1fr` — breadcrumb 길이와 무관하게 검색바 위치 고정.
- 1080px 이하: breadcrumb 숨김, 검색바 풀 폭.
- 880px 이하: 사이드바를 상단 스트립으로 축소.

### PageHeader

페이지마다 `<PageHeader title actions />` 사용. AppShell의 `actions` prop은 제거. **이유**: GitHub/Linear는 모두 액션을 섹션 우측에 둠. top bar는 글로벌(search/theme)만.

### Bento grid (Projects)

- 최신 프로젝트가 2×2 featured, 나머지는 1×1.
- featured 시각 신호: padding 키움 + border-strong. side stripe 금지.
- mobile: 단일 컬럼으로 붕괴.

## Components

| 컴포넌트 | 패턴 |
|---|---|
| `Button` | Radix Button + `accentColor="gray"` + `highContrast` 자동. solid → 모노 채움. |
| `.btn-primary` | 커스텀 대체. 600 굵기, monochrome 필. 페이지 액션 보조. |
| `Card` | 라운드 + soft shadow. 중첩 금지. |
| `Chip` | mono/accent/danger/success/warning/info. accent는 "current/selected" 의미로 재정의. |
| Theme toggle | 헤더 우측 아이콘 버튼 + `DropdownMenu`. SegmentedControl는 폐기. |
| `ProjectCard` | 헤딩 + #id chip + 생성일. 설명문 없음. featured는 padding + border-strong만. |
| Sidebar link | hover → surface-hover, current → surface-elevated + 좌측 2px ink rail. amber rail 제거. |

## Motion

- 120ms (fast) / 200ms (normal) / 360ms (slow).
- Easing: `ease-out-expo`, `ease-out-quart`만. bounce/elastic 금지.
- Transform / opacity / shadow / border-color만 트랜지션. layout 속성 금지.
- `@media (prefers-reduced-motion: reduce)` 전역 0.01ms.

## A11y

- WCAG 2.2 AA 기본, 시크릿 값 ≥ 5.5:1.
- 모든 인터랙티브: default / hover / focus-visible / active / disabled / loading / error.
- focus-visible은 Radix가 자체 ring을 그리는 요소(`[class*='rt-']`) 제외하고 전역 적용.
- skip link, landmark (`<nav aria-label>`, `<main id>`, `<header role="banner">`), aria-current="page", aria-keyshortcuts.

## What was removed (and why)

| 제거된 것 | 이유 |
|---|---|
| amber accent 전체 | AI-YouTube reflex. 모노에 위배. |
| 사이드바 ThemeToggle (SegmentedControl) | 트렌드는 아이콘 드롭다운. 사이드바 공간 낭비. |
| AppShell `actions` prop | 액션은 페이지가 소유. top bar는 글로벌만. |
| Projects/Project/EnvDiff/EnvSecrets 설명문 | 개발자 사용자에게 UI-as-help는 노이즈. 행동 중심으로 전환. |
| ProjectCard featured 설명 단락 | 카드 자체가 신호. 추가 문장은 중복. |
| Featured card amber 좌측 rail | impeccable Absolute ban: side-stripe border > 1px. |
| Inline `style={{...}}` (페이지/카드 다수) | Tailwind 토큰 클래스(`text-text-faint`, `min-w-[200px]`)로 이식. |

## Files

- `src/styles/tokens.css` — single source of OKLCH tokens.
- `src/styles/globals.css` — Pretendard `@font-face`, shell grid, page-head, btn-primary/secondary, dropdown, sec-row, audit-row, diff-col.
- `src/components/AppShell.tsx` — shell + 3-col header. `actions` prop 없음.
- `src/components/PageHeader.tsx` — title + eyebrow + actions.
- `src/components/ThemeToggle.tsx` — icon button + DropdownMenu.
- `tailwind.config.ts` — semantic 토큰 매핑.
