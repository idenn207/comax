# Local Review: dashboard-ui · Task 12 (a11y + 시각 폴리시 + anti-template)

**Reviewed**: 2026-05-31
**Branch**: `feat/dashboard-ui` (uncommitted on top of `e128573`)
**Scope**: 13 modified + 8 untracked 파일 (`web/dashboard/**`, `.github/workflows/ci.yml`)
**Decision**: **REQUEST CHANGES** — HIGH 1건은 CI axe 게이트를 무너뜨릴 가능성이 큼.

## 요약

Task 12의 핵심(`useTheme` 상태 머신, 라이트/다크 토큰, ThemeRoot/ThemeToggle, ProjectCard 벤토, 스킵 링크/포커스 링, axe-playwright 게이트, CI `dashboard-e2e` 잡)이 깔끔하게 조립되어 있다. typecheck·lint·vitest(98 tests)·vite build 모두 그린, gzipped JS는 141.20 kB로 Task 13 예산(≤ 400 kB)에 여유.

문제는 한 곳: `ThemeToggle`의 "자동" 항목이 **WCAG 2.5.3 Label in Name (Level A)** 를 위반한다. 시각 라벨은 "자동"인데 접근가능 이름이 "OS 설정 따름"이라, axe `label-content-name-mismatch`가 fire하고 음성 제어 사용자가 "자동" 발화로 활성화하지 못한다. Task 12 자체가 axe 게이트 그린을 받는 일이므로 이 한 줄이 곧 CI 적색 신호다.

그 외에는 모두 MEDIUM 이하의 일관성·세부 a11y·테마 부트 플래시 류로, 머지 후 빠르게 따라가도 무방하다.

## Findings

### CRITICAL
없음.

### HIGH

**H1. `SegmentedControl.Item` `aria-label`이 시각 텍스트와 불일치 — WCAG 2.5.3 Level A 위반**
[web/dashboard/src/components/ThemeToggle.tsx:32-36](web/dashboard/src/components/ThemeToggle.tsx#L32-L36)

```tsx
<SegmentedControl.Item key={opt.value} value={opt.value} aria-label={opt.description}>
  {opt.label}
</SegmentedControl.Item>
```

- "라이트"/"다크" 항목의 description("라이트 테마"/"다크 테마")은 시각 라벨을 포함하므로 통과한다.
- "자동" 항목만 description="OS 설정 따름"이라 시각 라벨 "자동"을 포함하지 않는다. → 음성 제어("Click 자동")가 실패하고 axe `label-content-name-mismatch`가 violation을 낸다.
- 이 룰은 `wcag2a` 태그에 묶여 있고 [tests/e2e/helpers/axe.ts:31-37](web/dashboard/tests/e2e/helpers/axe.ts#L31-L37)의 `withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])` 가 그 태그를 명시적으로 포함하므로 `a11y.spec.ts`의 `/` `/audit` 같은 AppShell 라우트(헤더에 ThemeToggle 노출)에서 즉시 실패한다.

**Fix (택일):**
- 가장 단순: `aria-label`을 제거하고 시각 텍스트를 그대로 접근가능 이름으로 쓴다. 부가 설명은 `title` 또는 `aria-describedby`로.
  ```tsx
  <SegmentedControl.Item key={opt.value} value={opt.value} title={opt.description}>
    {opt.label}
  </SegmentedControl.Item>
  ```
- 라벨을 의미 있게 만들고 description은 보조로:
  ```tsx
  { value: 'system', label: '자동', description: '자동 (OS 설정)' },
  ```
  그리고 `aria-label`은 description으로 그대로 두면 두 토큰이 모두 포함되어 2.5.3을 통과한다.

근거: WCAG 2.5.3, axe rule `label-content-name-mismatch` (tags: `cat.semantics`, `wcag2a`, `wcag253`).

### MEDIUM

**M1. ProjectCard의 카드 제목이 매번 `<h1>`로 렌더된다 — 페이지당 h1 다중**
[web/dashboard/src/components/ProjectCard.tsx:58](web/dashboard/src/components/ProjectCard.tsx#L58)

Radix `Heading`의 `as` 기본값은 `h1`로 박혀 있다 (`@radix-ui/themes/.../heading.props.js`의 `default:"h1"`). 따라서 `<Heading size={featured ? '6' : '3'} trim="start">` 는 카드마다 `<h1>`을 그린다. 같은 화면에 페이지 h1("프로젝트")이 이미 있어 `<h1>`이 N+1개 존재한다.

axe의 `page-has-heading-one`은 best-practice 태그라 현재 게이트는 통과하지만, 문서 구조상 카드 제목은 `as="h2"` 또는 `as="h3"`이어야 한다. EnvSecrets에도 같은 패턴이 있다:
- [web/dashboard/src/pages/EnvSecrets.tsx:121](web/dashboard/src/pages/EnvSecrets.tsx#L121) — envName 헤딩 (`as` 미지정 → h1)
- [web/dashboard/src/pages/EnvSecrets.tsx:163](web/dashboard/src/pages/EnvSecrets.tsx#L163) — "시크릿이 없습니다" (h1)

**Fix:**
```tsx
<Heading size={featured ? '6' : '3'} as="h2" trim="start" ...>{project.name}</Heading>
```
EnvSecrets의 envName은 `as="h1"`, 빈 상태 헤딩은 `as="h2"`로.

**M2. `useThemeContext`가 Provider 부재 시 조용히 no-op으로 폴백**
[web/dashboard/src/components/ThemeRoot.tsx:31-47](web/dashboard/src/components/ThemeRoot.tsx#L31-L47)

테스트 보일러플레이트 절약 의도는 명확하고 주석도 있지만, 결과적으로 "ThemeRoot를 잊은 채 ThemeToggle을 다른 트리에 떨어뜨려도 토글이 작동하는 척"한다. 운영 코드에서는 발견이 늦어진다.

**Fix 후보:**
- 테스트 환경에서만 폴백을 허용(예: `import.meta.env.MODE === 'test'`).
- 또는 별도 export `useThemeContextOptional()`을 두고 production은 throw.

**M3. 첫 페인트에서 라이트 → 다크 FOUC (저장된 다크 사용자에게 깜빡임)**
[web/dashboard/index.html](web/dashboard/index.html) (this diff 외) + [web/dashboard/src/lib/theme.ts:94-98](web/dashboard/src/lib/theme.ts#L94-L98)

`useTheme`의 effect가 첫 렌더 후에 `data-theme`을 세팅하므로, localStorage에 `dark`가 저장된 사용자도 React mount 전까지 라이트 토큰으로 잠깐 그려진다.

**Fix:** `index.html` `<head>` 상단에 동기 스크립트로 사전 세팅:
```html
<script>
  try {
    var p = localStorage.getItem('comax.theme-pref');
    var m = matchMedia('(prefers-color-scheme: dark)').matches;
    var a = (p === 'light' || p === 'dark') ? p : (m ? 'dark' : 'light');
    document.documentElement.setAttribute('data-theme', a);
    document.documentElement.style.colorScheme = a;
  } catch {}
</script>
```

**M4. 글로벌 `:focus-visible` 윤곽선이 Radix 포커스 링과 충돌**
[web/dashboard/src/styles/globals.css:65-68](web/dashboard/src/styles/globals.css#L65-L68)

```css
:focus-visible {
  outline: 2px solid var(--color-focus-ring);
  outline-offset: 2px;
}
```

Radix Themes 컴포넌트는 자체 `box-shadow` 기반 포커스 링을 그리는데, 위 규칙은 그 위에 추가로 outline을 얹어 이중 링이 보일 수 있다(Buttons/SegmentedControl 등).

**Fix:** Radix가 포커스 링을 그리는 셀렉터(예: `[data-radix-themes] :focus-visible`)는 제외하거나, 글로벌 규칙을 `body > a:not([class^='rt-']):focus-visible` 류로 좁힌다. 가장 안전한 길은 `:where(a, button, input, textarea, select):not([data-radix-themes] *):focus-visible` 같은 `where`/`not` 조합.

**M5. `writeThemePreference`가 입력 검증을 하지 않는다**
[web/dashboard/src/lib/theme.ts:51-62](web/dashboard/src/lib/theme.ts#L51-L62)

읽기 쪽 `readThemePreference`는 unknown 문자열을 거르지만, 쓰기 쪽은 그대로 저장한다. 현재 호출자는 모두 타입이 있는 SegmentedControl이라 실해는 없지만 방어선이 비대칭이다. 한 줄 검사 추가:
```ts
if (pref !== 'system' && pref !== 'light' && pref !== 'dark') return;
```

### LOW

**L1. `ProjectCard`의 `<Text mt={featured ? '3' : '0'} style={{ marginTop: 'auto' }}>` 는 inline style이 항상 이긴다 — Radix `mt` prop은 사실상 죽은 코드.**
[web/dashboard/src/components/ProjectCard.tsx:71](web/dashboard/src/components/ProjectCard.tsx#L71)

**L2. `Login.tsx`의 `<main id="main" tabIndex={-1}>` 는 스킵 링크 타겟인데 Login 페이지엔 스킵 링크가 없다. id가 노출만 되고 사용되지 않는다. 일관성을 원하면 스킵 링크도 같이 두거나, id를 빼고 단순 main으로.**
[web/dashboard/src/pages/Login.tsx:54](web/dashboard/src/pages/Login.tsx#L54)

**L3. `globals.css`의 `.project-card article {...}` 는 컴포넌트가 `<article>`을 쓴다는 사실에 강하게 결합된다. ProjectCard 마크업이 `<section>` 따위로 바뀌면 모든 hover/focus 스타일이 조용히 사라진다. 클래스 한 개(예: `.project-card-surface`)를 article에 부여하면 결합이 풀린다.**
[web/dashboard/src/styles/globals.css:74-91](web/dashboard/src/styles/globals.css#L74-L91)

**L4. CI `COVER_FLOOR=70` 은 PRD 목표 80%보다 낮다는 점이 워크플로 주석에 명시되어 있다. 일정 후 task에서 80으로 올리는 작업을 별도 이슈로 큐잉할 것.**
[.github/workflows/ci.yml:16-18](.github/workflows/ci.yml#L16-L18)

**L5. `theme.ts` 주석은 "`<meta name='color-scheme'>` 도 sync한다"고 적었지만 실제로는 `documentElement.style.colorScheme`만 세팅한다 (효과는 동일하지만 주석이 사실을 살짝 앞선다).**
[web/dashboard/src/lib/theme.ts:19-20](web/dashboard/src/lib/theme.ts#L19-L20)

**L6. `tokens.css`가 `oklch()` 사용 — 모던 admin tool이라 Safari 15.4+가 기본 전제이긴 하나, 그 사실을 PRD/README 어딘가 한 줄로 못박아두면 후일 사용자 문의를 덜 수 있다.**

## Validation Results

| Check | Result | Note |
|---|---|---|
| Type check (`pnpm typecheck`) | Pass | tsc 무오류 |
| Lint (`pnpm lint`) | Pass | `--max-warnings=0` 통과 |
| Unit tests (`pnpm test`) | Pass | 13 files / 98 tests |
| Build (`pnpm build`) | Pass | gzipped JS 141.20 kB / CSS 85.08 kB |
| E2E (`pnpm test:e2e` + axe) | **Not run** | Go 바이너리 빌드 필요. **H1 미수정 시 axe `label-content-name-mismatch` 로 실패할 가능성이 매우 높음** |

E2E를 로컬에서 확인하지 않은 상태이므로, H1을 고친 직후 `go build -tags embed_dashboard ./cmd/server`와 `pnpm test:e2e`를 한 번 돌려 빨강이 사라지는지 확인하길 권장한다.

## Files Reviewed

**Modified (13)**
- `.github/workflows/ci.yml` — dashboard + dashboard-e2e 잡 추가, axe 게이트 게이트키퍼 위치
- `web/dashboard/package.json` — `@axe-core/playwright ^4.10.1` devDep 추가
- `web/dashboard/pnpm-lock.yaml` — 위 항목 잠금
- `web/dashboard/src/components/AppShell.tsx` — skip-link, breadcrumb, ThemeToggle 슬롯, main 랜드마크 정리
- `web/dashboard/src/components/DiffViewer.tsx` — semantic 토큰(`--color-danger-soft` 등) 적용, visually-hidden 라벨 추가
- `web/dashboard/src/main.tsx` — ThemeRoot 주입
- `web/dashboard/src/pages/EnvSecrets.tsx` — 숨겨진 텍스트 dup 제거(visually-hidden만 남김)
- `web/dashboard/src/pages/Login.tsx` — main 랜드마크 + `aria-errormessage` 와이어링
- `web/dashboard/src/pages/Projects.tsx` — 벤토 그리드, featured 카드 분리
- `web/dashboard/src/styles/globals.css` — skip-link, visually-hidden, project-card 호버, reduced-motion
- `web/dashboard/src/styles/tokens.css` — light/dark 토큰, semantic *-soft 페어, 포커스 링
- `web/dashboard/src/test/setup.ts` — matchMedia/ResizeObserver/PointerEvent shim
- `web/dashboard/tailwind.config.ts` — semantic 색상/타이포/duration 토큰 미러링

**Added (8)**
- `web/dashboard/src/components/ProjectCard.tsx`, `ProjectCard.test.tsx`
- `web/dashboard/src/components/ThemeRoot.tsx`, `ThemeToggle.tsx`
- `web/dashboard/src/lib/theme.ts`, `theme.test.ts`
- `web/dashboard/tests/e2e/a11y.spec.ts`, `tests/e2e/helpers/axe.ts`

## Next steps

1. **H1을 먼저 고친다** — 한 줄(또는 description 합치기) 수정이면 충분하다. 머지 차단 사유.
2. M1·M2는 같은 PR에서 처리 권장 (Heading hierarchy 정리 + ThemeContext 폴백 제약). 한 번에 묶이면 후속 코드 리뷰 비용이 낮아진다.
3. M3(FOUC 스크립트)·M4(포커스 링 충돌)·M5(검증)·L1~L6 는 후속 PR로 분리해도 무방.
4. H1 수정 후, `go build -tags embed_dashboard ./cmd/server` → `pnpm test:e2e` 로 axe 게이트가 실제로 그린인지 한 번 더 확인.
