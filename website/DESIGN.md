# Design

> Comax Secrets **마케팅·문서 사이트**(`website/`)의 시각 시스템. 전략 컨텍스트는
> [website/PRODUCT.md](PRODUCT.md). 운영자 대시보드([docs/DESIGN.md](../docs/DESIGN.md),
> monochrome)와 **뉴트럴·시맨틱 토큰은 parity 유지**, **teal brand 계열만 이 사이트의 새 레이어**다.

`feat/website-redesign`에서 확정. shape 브리프 → 팔레트 계산 검증 → 이 문서.

## Register

brand (impeccable register: 디자인이 곧 설득). docs 하위 표면은 brand 토큰을 상속하되 가독성 우선.

## Color strategy — Committed teal (hue 202)

대시보드의 monochrome-미러에서 이탈해, **틸 하나가 랜딩 정체성을 짊어진다**(Committed). 크림·그라디언트 없이 흰 배경 위 틸 + Pretendard 굵기로 "대담하되 근거 있게". hue는 청록끼 teal(202)로 확정(초록↔청록 후보 비교 후 선택).

### 왜 teal 202인가

- crimson 시드(`palette.mjs` seed-024, hue 27)는 `--color-danger`(hue 25)와 충돌 → 시크릿 도구에서 브랜드=위험색은 금지. 시드 폐기.
- 예약 시맨틱 hue(danger 25 · warning 70 · success 150 · info/focus 250)와 **모두 충돌 없는** 열린 hue가 teal 202(청록끼 teal). info-blue(focus-ring)와 48°, success-green과 52° 떨어져 시각적으로 뚜렷이 구분.
- teal = 엔지니어링·self-host 인프라 톤. Bun의 "대담한 인디 캐릭터"를 playful-loud가 아니라 **crafted-confident**로 실행(Pretendard-only 결정과 맞물림).
- 옛 brand accent(blue 250)는 info/focus와 겹쳐 "선택/포커스"와 혼동됐다. teal은 그 혼동을 없앤다.

### Brand 토큰 (계산 검증됨 — OKLCH→sRGB→WCAG)

| 토큰 | Light | Dark | 용도 · 대비 |
|---|---|---|---|
| `--color-brand` | `oklch(0.52 0.088 202)` | `oklch(0.78 0.10 202)` | 링크·primary 필·히어로 강조어. 링크 on bg **5.07:1**(L)/**10.21:1**(D), 흰텍스트 on 필 **5.15:1**(L) |
| `--color-brand-hover` | `oklch(0.46 0.078 202)` | `oklch(0.84 0.09 202)` | hover |
| `--color-brand-active` | `oklch(0.40 0.06 202)` | `oklch(0.72 0.10 202)` | active/press |
| `--color-brand-soft` | `oklch(0.95 0.03 202)` | `oklch(0.28 0.04 202)` | chip·selection·soft 배경 |
| `--color-brand-strong` | `oklch(0.42 0.07 202)` | `oklch(0.86 0.06 202)` | brand-soft 위 텍스트. on-soft **7.15:1**(L)/**9.54:1**(D) |
| `--color-brand-text` | `oklch(0.99 0.005 202)` | `oklch(0.15 0.006 260)` | brand 필 위 텍스트 |

**Committed 실행 규칙**: teal은 accent가 아니라 **surface를 짊어질 수 있다** — 히어로/시그니처 밴드는 `--color-brand` 배경 + `--color-brand-text`(흰/근검정)로 채워 커밋 모멘트를 만든다(대비 5:1+로 대형 텍스트 안전). 나머지 면은 뉴트럴 유지 → 틸이 "30~60%가 아니라 결정적 순간"을 지도록 절제. docs 표면은 Restrained(틸은 링크·인라인 액센트만).

### Parity-locked (대시보드와 동일, 건드리지 않음)

surface / surface-elevated / surface-hover / panel / text / text-subtle / muted / border / border-strong / **모든 시맨틱**(danger 25 · warning 70 · success 150 · info 250) / focus-ring(blue 250) / code. `check-token-parity.mjs`는 이 목록만 계속 강제. brand 계열은 parity 예외로 허용(스크립트에 allowlist 추가).

### A11y 색 규칙

- 본문 ≥4.5:1, brand-as-link 검증 통과. 시크릿류 텍스트는 계승 규칙대로 한 단계 상향(docs 코드블록).
- **색만으로 상태 전달 금지**: teal(brand)은 내비게이션·CTA용, 시맨틱 상태(success/info/danger)는 항상 아이콘+라벨 동반. teal↔green↔blue 색약 혼동을 역할·라벨로 차단.
- focus-ring은 blue 250 유지 → "포커스"와 "브랜드(teal)"가 hue로 분리.

## Typography

정체성 요약: **대담함은 시끄러움이 아니라 조용한 확신**. 대담함은 커밋된 teal(색) + mono 시그니처(텍스처) + de-card된 구조가 지고, **타이포는 refined하게 물러선다**. "정직한 장인"은 소리치지 않고 디테일로 말한다.

- Family: **Pretendard Variable**(한글+Latin) → Apple SD Gothic Neo → Malgun Gothic → system. **디스플레이 폰트 없음**(CDN 종속·costume 회피).
- **Refined semibold**: 히어로는 Pretendard **600~700**, 스케일 ~3.2rem(계승 `--text-display` clamp(2.5→3.5rem)급, `--text-hero`의 4.5rem은 쓰지 않음) + `letter-spacing: -0.02em`. 굵기 계층은 400 본문 / 500 라벨 / 600 강조·헤딩 / 700 히어로. 극적 대비가 아니라 **절제된 대비 + 정밀한 간격**으로 계층을 세운다.
- **mono 시그니처 텍스처**: 디스플레이 폰트 부재를 mono가 메운다. 수치·명령명·버전·메타(예: `14 secrets`, `secret run`, `MIT`, `v0.3`)를 `--font-mono`로. **실제 데이터·명령에만** 쓰고, 장식적 번호 eyebrow(`01 · …`)로는 쓰지 않는다(절대금지). costume(검은 배경 도배·`$` 프롬프트)이 아니라 인용.
- Scale·line-height·`text-wrap: balance`(h1~h3)/`pretty`(prose)는 기존 globals.css 계승.

## Layout

섹션 척추 유지(**히어로 → 4축 차별점 → quickstart 티저 → 최종 CTA**). IA 재발명 아님 — 정체성·계층·크래프트만 손봄.

- **히어로**: 비대칭 2열(카피 리드 + `CommandBlock` 증명 공동주연). semibold heading, 강조어 1개만 teal. primary CTA teal 필. 배지·메타는 mono. `max-w-content`(72rem) 계승.
- **4축 차별점 = 교차 풀-width 행**: 동일 카드 그리드 폐기(impeccable "identical card grid" 회피). 각 축 = **텍스트 + 미니 터미널/diff 실증 비주얼**의 좌우 교차 행(1행 텍스트-좌/비주얼-우, 2행 반전…). terminal/diff가 곧 imagery(원칙 충족), show-don't-tell 강화. NAS·self-host 축이 첫 행.
- **quickstart 티저 + CTA**: 유지, 재보이스. **Committed teal 밴드 1개**(밴드 채움 + brand-text)로 커밋 모멘트 — 히어로 다음 또는 최종 CTA 중 한 곳만.
- 수직 리듬 변주(관대한 분리 ↔ 촘촘한 그룹). 섹션 경계는 기존 border 패턴.
- 반응형: 히어로·교차행 → 모바일 stack(비주얼이 텍스트 아래로). **헤드라인 오버플로우 각 breakpoint 테스트**(절대금지).

## Motion

- 계승: 120/200/360ms, `ease-out-expo`/`ease-out-quart`, transform·opacity·shadow·border만.
- 히어로 진입 모션 1개(이미 보이는 기본을 강화, class-gate로 콘텐츠 숨기지 않음). `prefers-reduced-motion` 전역 0.01ms 계승.

## Components

| 컴포넌트 | 패턴 |
|---|---|
| `ButtonLink` primary | teal 필(`--color-brand` + `--color-brand-text`), hover→`--color-brand-hover`, 600 |
| `ButtonLink` secondary | surface-elevated + border-strong, 텍스트 teal 아님 |
| 링크(prose·인라인) | `--color-brand`, underline offset 2px |
| `CommandBlock` | mono, 출력 강조행에 teal. 클릭-복사 어피던스 |
| Chip | brand-soft 배경 + brand-strong 텍스트 (selected/current 의미) |
| 시그니처 밴드 | `--color-brand` 채움 + brand-text (Committed 모멘트, 1~2개) |
| `::selection` | brand-soft 배경(계승) |

## A11y

- WCAG 2.2 AA. 인터랙티브 전 상태(default/hover/focus-visible/active/disabled). focus-visible ring 전역.
- 키보드 완주(랜딩→docs→검색→복사), skip-link 계승.
- 색 비의존 상태 신호(위 색 규칙).

## Files (구현 시 대상)

- `app/globals.css` — brand 토큰 6종 ×(light/dark) 추가, `::selection`·prose 링크는 이미 `--color-brand` 참조 → 값만 teal로.
- `tailwind.config.ts` — brand 계열 이미 매핑됨(값만 갱신). brand-active/strong 추가.
- `scripts/check-token-parity.mjs` — brand 계열 allowlist 추가(랜딩 한정 완화).
- `app/page.tsx` — 히어로·4축 de-card·밴드 재구성.
- `components/site-header.tsx`·`site-footer.tsx` — teal 링크·CTA 리스킨.

## What changed from the mirror (and why)

| 바뀐 것 | 이유 |
|---|---|
| blue-250 brand accent → **teal 202** | info/focus와 겹쳐 혼동. teal은 예약 hue 전부와 분리, 독립 정체성. |
| Restrained(링크만) → **Committed**(밴드가 틸 짊어짐) | brand register + "대담한 인디". 미러의 무색 절제 탈피. |
| 토큰 100% parity → **brand 계열 parity 예외** | 랜딩 독립 정체성. 뉴트럴·시맨틱은 여전히 parity(한 제품). |
| 4-동일카드 벤토 | "identical card grid" 금지. de-card 예정. |
