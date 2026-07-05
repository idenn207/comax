# Product

> Comax Secrets **마케팅·문서 사이트**(`website/`)의 전략 컨텍스트.
> 운영자 대시보드(`dashboard/`, product register)의 [`docs/PRODUCT.md`](../docs/PRODUCT.md)와는
> 별개 표면이다. 대시보드는 제품을 **서빙**하고, 이 사이트는 디자인이 곧 **설득**이다.

## Register

brand

랜딩이 1급 표면(디자인이 곧 제품). docs는 큰 비중의 하위 표면으로, 브랜드 정체성의 토큰·타이포를 물려받되 페이지 자체는 가독성·정보구조가 우선하는 유틸리티로 동작한다.

## Users

**1순위: 개인 개발자(indie).** 혼자 여러 프로젝트·worktree·서비스를 굴리며 multi-service × multi-environment의 `.env` 사본을 손으로 맞추는 데 지친 솔로 개발자. self-host를 스스로 굴릴 능력과 의지가 있고, NAS·홈랩을 돌리기도 한다. CLI에 익숙하지만 러닝커브 없는 진입을 선호한다.

- **방문 맥락**: 대개 GitHub·검색에서 흘러 들어온다. 랜딩에서 30초 안에 "이게 내 페인(hand-synced `.env`)을 푸는가, 가볍고 self-host인가"를 판단하고, 맞다 싶으면 곧장 docs/quickstart로 넘어간다. 방문 목적의 절반 이상이 **문서를 읽는 것**(설치·CLI·SDK·self-host·Action 레퍼런스)이다.
- **해야 할 일(job-to-be-done)**:
  1. "가볍고 self-host 가능한가"를 빠르게 확신한다.
  2. 5분 안에 실제로 깔아 첫 시크릿을 주입한다(`docker compose up` → `secret push` → `secret run`).
  3. CLI/SDK/Action 레퍼런스를 필요할 때 정확히 찾는다.

부차 독자(홈랩·self-host 애호가, 소규모 팀)는 같은 화면을 공유하되, voice와 레퍼런스의 기준점은 항상 1순위 indie 개발자다.

## Product Purpose

Comax Secrets는 **가벼운 self-host 시크릿 매니저**다. SQLite 하나로 부팅하고, worktree·multi-service 환경을 1급으로 다루며, CLI 한 줄과 GitHub Action 한 줄로 시크릿을 주입한다. 외부 Postgres·Redis를 요구하지 않는다.

이 사이트의 목적:

- indie 개발자가 이 도구가 자기 페인을 푼다는 걸 랜딩에서 **확신**하게 한다.
- 마찰 없이 docs로 넘어가 **실제로 설치·완주**하게 한다.
- "정성껏 만든 인디 도구"라는 첫인상 = **크래프트가 곧 신뢰**임을 시각적으로 전달한다(시크릿을 다루는 도구라 특히).

**성공 신호**:

- 랜딩 → docs/quickstart 전환, GitHub 유입.
- 문서만으로 self-host를 처음부터 끝까지 완주할 수 있다(설치가 막히지 않는다).
- 사이트를 본 사람이 "AI가 찍어낸 SaaS 랜딩"이 아니라 "누가 손으로 잘 만든 도구"라고 느낀다.

## Brand Personality

세 단어: **대담함, 정직함, 손맛(craft).**

- **대담함**: Bun류 인디 캐릭터. 색·타이포·놀이성을 겁내지 않는다. 뚜렷한 POV를 가지고 "이건 가볍고, self-host고, 네 문제를 푼다"를 자신 있게 말한다. 안전한 무색 절제(대시보드 monochrome 미러)에서 **의도적으로 이탈**한다.
- **정직함**: 모든 대담함은 구체적 주장으로 뒷받침한다. buzzword(streamline / empower / seamless…)·hero-metric 템플릿·과장 금지. "12개의 `.env`", "SQLite 한 개", 실제 `secret` 명령처럼 진짜 숫자와 진짜 커맨드로 말한다.
- **손맛(craft)**: 신뢰는 광택이 아니라 디테일에서 온다. 타이포 리듬·코드블록·상태·간격을 손으로 맞춘 티가 곧 "이 도구를 믿어도 된다"는 신호다.

**Voice: 정직한 장인.** 대시보드의 정직함을 잇되 indie 제작자의 개인성·확신을 더한다. UI 라벨·본문은 한국어 평어, 시스템·에러·코드는 영어(개발자가 grep 가능해야 함). em dash 금지, 군더더기 없는 명사+동사.

## Anti-references

명시적으로 피한다:

- **AI SaaS 랜딩** — 인디고/바이올렛 그라디언트 + 라운드 카드 그리드 반복 + hero-metric 템플릿. "AI가 만든 티"의 정의.
- **Doppler·Vault 다크 SaaS** — 다크 표면 + 채도 액센트. 같은 카테고리 모든 도구가 머무는 첫 reflex 레인.
- **에디토리얼 매거진** — display serif italic + 드롭캡 + 룰 구분선 + broadsheet 그리드. brand register의 포화된 2차 reflex(Klim/Fraunces 모방). 우리는 세리프-매거진 룩으로 도망치지 않는다.
- **Vercel/shadcn 템플릿 룩**(계승) — 흰 배경 + 미묘한 그림자 + 라운드 카드 반복. "본 적 있다" 틱.

**허용하되 경계 있는 것 — 터미널 인용 ≠ 터미널 costume**: 이 도구는 CLI-first이고 Bun도 mono 놀이성을 쓴다. 실제 명령을 보여주는 `CommandBlock` 같은 **의도된 터미널/mono 인용은 브랜드 표면 위에서 허용**한다. 다만 검은 배경 도배·`$` 프롬프트 흉내·CRT 효과 같은 **wholesale costume은 금지**한다(진짜 터미널이 필요한 사람은 진짜 터미널을 켠다).

## Design Principles

신규 화면·컴포넌트·copy 결정 시 비추어 보는 5개 가드레일(픽셀·OKLCH 규칙이 아님):

1. **docs가 본체, 랜딩은 현관** — 사이트의 무게중심은 문서다. 랜딩은 "이게 내 문제인가"를 30초에 판단시키고 docs로 넘긴다. 브랜드 표현이 문서 가독성·정보구조를 이기지 않는다.
2. **대담하되 근거 있게** — Bun류 인디 캐릭터(색·타이포·놀이성)를 쓰되, 모든 대담함은 구체적 주장으로 갚는다. buzzword·hero-metric·과장은 금지. 진짜 숫자와 진짜 명령으로 말한다.
3. **크래프트가 신뢰다** — 시크릿 도구의 신뢰는 광택이 아니라 손으로 맞춘 디테일에서 온다. 타이포 리듬·코드블록·상태·간격의 완성도가 곧 브랜드다.
4. **자체 정체성, 그러나 한 제품** — 랜딩은 대시보드 monochrome parity에서 이탈해 자기 컬러·타이포를 갖는다. 단, 공유 문서 셸과 시맨틱 상태 색은 제품과 이어져 "다른 회사"로 읽히지 않게 한다(token-parity 계약은 랜딩 한정으로 완화, docs 셸은 뉴트럴 유지).
5. **CLI를 흉내내지 말고 인용하라** — CLI-first 도구지만 사이트는 터미널 costume이 아니다. 터미널/mono는 실제 명령을 보여줄 때, 브랜드 표면 위의 의도된 인용으로만 쓴다.

## Accessibility & Inclusion

프로젝트에 이미 확립된 기준을 계승한다(이탈하지 않는다):

- **WCAG 2.2 AA**를 최저선으로. 본문 ≥4.5:1, 코드·명령 텍스트는 한 단계 상향.
- 모든 인터랙티브 요소는 default / hover / focus-visible / active / disabled 상태를 갖는다. focus-visible ring은 전역 토큰(`--color-focus-ring`).
- `prefers-reduced-motion`을 전역 토큰으로 존중한다. 대담한 모션을 쓰더라도 reduced-motion 대체(크로스페이드/즉시 전환)를 항상 함께 제공한다.
- 색상만으로 상태를 전달하지 않는다(변경·성공·실패는 아이콘·라벨·텍스트로 함께).
- 키보드만으로 랜딩→docs→검색→코드 복사까지 완주할 수 있어야 한다.
