# Comax Secrets — Lightweight Self-Hosted Secret Management

## Problem

개인 개발자와 소규모 팀이 multi-service × multi-environment 구성(예: api/web/mq/infra × local/dev/prod = 12개 `.env` 파일)을 운영할 때, 환경별 시크릿 동기화를 수작업·복사·재배포에 의존하고 있다. 그 결과 환경 간 누락·불일치, 감사/롤백 불가, Local `.env` 와 GitHub Secret 의 이중 관리 포인트, worktree·container 환경마다 동일 시크릿을 다시 주입해야 하는 비용이 매일 발생한다. 기존 도구(Infisical, Doppler, Vault)는 SaaS 종속·고비용이거나 self-host 설정·운영 부담이 크고, NAS·홈랩 같은 제약된 환경에서는 그대로 도입하기 어렵다.

## Evidence

- **본인의 직접 운영 경험 (검증된 페인)**:
  - 현재 `api.env`, `web.env`, `mq.env`, `infra.env` × `local/dev/prod` = **12개 `.env` 파일을 수작업으로 동기화**.
  - Local 환경에 신규 envvar 추가 시 dev/prod 파일에 누락되는 사고가 반복 발생.
  - Local 인프라(DB 등)를 Docker로 운영해서 GitHub Secret 으로는 local 을 커버할 수 없음 → `.env` + GitHub Secret 의 **이중 source of truth**.
  - Docker Swarm + GitHub Runner 자동 배포 환경에서 `docker secret` 자동 회전이 불가, 추가 스크립트 필요한데 **NAS 환경 제약**으로 사용 가능한 도구가 한정적.
  - Worktree 다수 운영 시 worktree 마다 `.env` 를 새로 복사해야 함.
- **확장 페인 (검증된 추가 관찰)**:
  - 인프라 자체의 환경별 설정 파일(`redis.[local|dev|prod].conf`, `nginx.[dev|prod].conf`)도 동일한 수작업 분기/복사로 관리됨 — 시크릿과 같은 패턴, 같은 페인.
- **시장 증거 (참고)**: Infisical/Doppler GitHub 트래커 상의 "lightweight self-host", "single-binary", "config templating" 요청 다수 — 정량 검증은 _Assumption — needs validation via maintainer issue triage_.

## Users

- **Primary**: **개인 개발자 / 1–3인 사이드프로젝트 운영자**
  - 컨텍스트: Self-host 인프라(NAS, 홈서버, 소규모 VPS)에서 Docker(또는 Docker Swarm)로 multi-service 를 운영.
  - 트리거: 신규 환경변수 추가 / 환경 분리 / worktree 분기 / 자동 배포 파이프라인 변경 시 매번 발생하는 동기화 비용.
  - 의사결정 기준: **(1) 무료 / 오픈소스, (2) 가벼운 self-host, (3) Local–CI–Prod 단일 흐름, (4) DX 만족도**.
- **Secondary (간접 수혜)**: 동일한 페인을 가진 소규모 스타트업 백엔드/플랫폼 개발자.
- **Not for (v1 명시 제외)**:
  - 대규모 enterprise 멀티 테넌트 SaaS 운영자 (조직 관리, billing, audit certification 등 미포함).
  - 동적 시크릿(Vault 스타일 DB credential 자동 발급/로테이션) 사용자.
  - PKI / 인증서 발급·회수가 핵심인 사용자.
  - Node 외 다양한 언어 SDK(Python/Go/Rust 등)를 v1 에 요구하는 사용자.

## Hypothesis

**"Worktree 와 multi-service 환경을 1급(first-class) 으로 다루는 가벼운 self-host 시크릿 도구"가, 개인 개발자의 `.env` × `GitHub Secret` 이중화와 환경별 수작업 동기화를 제거해서, 신규 환경변수 1건 추가에 드는 시간을 ≥ 80% 줄이고 "Local → dev → prod 누락" 사고를 0 으로 만든다.**

검증 신호:

- 본인의 12개 `.env` 파일 운영을 단일 도구로 대체할 수 있는가.
- `secret run -- <cmd>` 한 줄로 worktree 별 환경이 자동 주입되는가.
- GitHub Actions 에서 동일 source 로부터 시크릿이 주입되어 GitHub Secret 등록 단계가 사라지는가.

## Success Metrics

| Metric                                             | Target                                     | How measured                                         |
| -------------------------------------------------- | ------------------------------------------ | ---------------------------------------------------- |
| **신규 envvar 1건 추가 → 전 환경 반영 시간**       | 기존 수작업 대비 ≥ 80% 단축 (5분 → 1분 내) | 본인 운영 기록 (before/after 로그)                   |
| **"Local 에는 있는데 dev/prod 에 누락" 사고 건수** | 도입 후 **0건/월**                         | 본인 운영 사고 로그 + CI fail 추적                   |
| **Self-host 최초 부팅까지 걸리는 시간**            | `docker compose up` 후 **≤ 2분**           | 클린 VM 에서 실측                                    |
| **CLI cold start latency (`secret run`)**          | ≤ 300ms p95                                | 로컬 벤치                                            |
| **GitHub Star (커뮤니티 시그널, 6개월)**           | ≥ 50                                       | GitHub                                               |
| **MAU 자기 검증**                                  | 본인 + 외부 사용자 ≥ 5명                   | self-host telemetry opt-in 또는 Discord/Issue 카운트 |

> ⚠ 마지막 두 항목은 PRD 단계의 _assumption-grade_ 지표. 정식 측정 방법은 `/plan` 단계에서 확정.

## Scope

### MVP (가설 검증에 필요한 최소 범위)

> **Goal**: 본인의 12개 `.env` × Docker Swarm × GitHub Actions 흐름을 **한 도구로 완전히 대체**.

1. **Self-host 서버 (single-binary or single-compose)**
   - SQLite 마운트 1개로 동작 (외부 Postgres/Redis 의존성 없음).
   - `docker compose up` 한 줄 부팅. NAS 환경 호환.
   - 평문 시크릿은 서버측 대칭 암호화(master key) 후 SQLite 에 저장. (E2EE 미포함 — self-host = 본인 소유라는 위협 모델 가정.)
2. **Dashboard UI (web)**
   - **Doppler 스타일 UI 레퍼런스**.
   - Project / Environment / Secret CRUD.
   - Secret versioning + diff + rollback.
   - 환경 간 비교 view ("local 에는 있는데 prod 에 없는 키" 하이라이트).
   - Audit log (누가 / 언제 / 어떤 키 변경).
3. **CLI (`secret`)**
   - `login`, `init`, `pull`, `push`, `run -- <cmd>`, `set`, `get`, `diff`.
   - `secret run -- npm dev` 형태로 child process 에 환경변수 주입.
   - **Worktree-aware 동작**: `.secretrc` (gitignore 필수) → git branch 매핑 → 명시적 `--env` 플래그 순으로 컨텍스트 결정. (구체 선택은 `/plan` 에서 확정.)
   - 가능한 한 부팅 빠른 단일 바이너리 (Go/Rust 중 `/plan` 단계 결정).
4. **GitHub Actions Integration**
   - 공식 action: `comax-secrets/load-action@v1`.
   - PAT/서비스 토큰으로 인증 → 지정 environment 의 시크릿을 step env 로 주입.
   - 이로써 **GitHub Secret 등록 절차 자체가 사라짐**.
5. **Secret Referencing & Overrides**
   - Infisical 스타일 `${{ shared.DB_HOST }}` 같은 inline 참조.
   - Environment 간 inherit + override 모델.
6. **Website + Docs (Next.js + Tailwind + Radix UI + Vercel)**
   - SEO 최적화된 랜딩 + 본격 docs.
   - Quickstart (≤ 5분), self-host 가이드, CLI/SDK reference, GitHub Action 예제.
7. **Node.js / TypeScript SDK (npm publish)**
   - 단일 패키지로 client + cache + reload.
   - SSR/Edge 호환 (Next.js 사용자 고려).
8. **Webhooks**
   - Secret 변경 / version 생성 / rollback 이벤트.
   - 본인 운영 환경에서 Docker Swarm 서비스 자동 재시작 트리거 용도.

### Expanded MVP candidate — 별도 결정 필요

- **Infra config 파일 관리 (redis.conf / nginx.conf 등)**:
  - "Secret 으로 채워진 템플릿을 환경별로 렌더링" 모델.
  - 본인 운영 페인에 직접 해당. v1 포함 시 가치 큼.
  - 다만 envvar 모델과 별개의 시스템(templating, file render, mount sync) 필요 → 범위 위험.
  - **결정 보류 — `/plan` 단계에서 작은 PoC 후 In/Out 결정.** (Open Question #5 로 기록.)

### Out of Scope (v1 에서 명시적으로 제외)

| 항목                                                      | 이유                                                                           |
| --------------------------------------------------------- | ------------------------------------------------------------------------------ |
| **Multi-tenant SaaS 강주 운영** (billing, org, plan 관리) | v1 은 self-host 우선. SaaS 는 검증 후.                                         |
| **다국어 SDK** (Python / Go / Rust / Java …)              | v1 은 Node/TS SDK 만. 검증 이후 확장.                                          |
| **PKI / 인증서 발급·회수**                                | 별개 제품 영역.                                                                |
| **동적 시크릿** (Vault 스타일 DB credential 동적 생성)    | 운영 복잡도 폭증. v1 위협 모델 밖.                                             |
| **E2EE (zero-knowledge client-side encryption)**          | Self-host = 본인 DB 소유 → 위협 모델상 ROI 낮음. 서버측 대칭 암호화로 대체.    |
| **Docker Swarm secret 자체와의 결합**                     | 사용자가 `docker secret` 미사용 결정. 대신 webhook + service restart 패턴.     |
| **Secret sharing (외부인에게 일회성 URL 공유)**           | 원 요청에는 있었으나 multi-service 운영 페인과 직접성이 낮아 v1 제외. v2 후보. |

> **참고**: 원본 요청의 "Secret sharing" 항목은 페인 검증 우선순위에서 밀려나 v2 후보로 이동. 이의 있으면 다시 끌어올 수 있음.

## Constraints (사전에 고정된 결정)

- **License**: **MIT**. (Enterprise 분리 시 dual-license 가능.)
- **Stack 고정**:
  - Website / Docs: **Next.js + Tailwind + Radix UI + Vercel**.
  - Dashboard: 별도 SPA 또는 Next.js 통합 (`/plan` 에서 결정).
  - SDK 배포: **npm**.
- **Self-host first**: SaaS 운영은 v1 책임 범위 밖.
- **위협 모델**: Self-host 운영자가 DB 와 마스터 키를 소유. 외부 노출 ≠ 운영 가정.
- **NAS / 저사양 환경 호환**: 외부 Postgres/Redis 미요구.

## Delivery Milestones

<!-- 비즈니스 outcome 중심. /plan 이 각 항목을 implementation plan 으로 분해. -->

| #   | Milestone                                   | Outcome                                                                             | Status      | Plan                                                                                            |
| --- | ------------------------------------------- | ----------------------------------------------------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------- |
| 1   | **Self-host server + CLI MVP**              | 본인의 12개 `.env` 를 단일 서버 + `secret pull/run` 으로 대체. SQLite 단일 마운트.  | complete    | [plan](../plans/completed/comax-secrets.plan.md) · [report](../reports/comax-secrets-report.md) |
| 2   | **Dashboard UI (Doppler 스타일)**           | 웹 UI 에서 project/env/secret CRUD + 버전 diff + rollback + 환경 간 누락 detection. | complete    | [plan](../plans/completed/comax-secrets-dashboard.plan.md) · [cleanup](../plans/completed/comax-secrets-dashboard-m2-cleanup.plan.md) · [closure](../plans/completed/comax-secrets-dashboard-m2-close.plan.md) · [report](../reports/comax-secrets-dashboard-m2.report.md) <br/>(gate: impeccable + codex `019ec6da`; receipt: `.claude/receipts/mccp-{plan,implement}-codex/comax-secrets-dashboard-m2-close.json`, git-ignored) |
| 3   | **GitHub Actions integration**              | GitHub Secret 등록 절차 0 회. action 한 줄로 step env 주입. + 서비스 토큰 admin 발급/soft-revoke + 대시보드 Tokens/Actions 화면. | complete    | [plan](../plans/completed/comax-secrets-m3-github-actions.plan.md) · [report](../reports/comax-secrets-m3-github-actions.report.md) <br/>(gate: impeccable critique + codex plan R1/R2 `019f1dd4`/`019f1dee`; PR #12; receipt: `.claude/receipts/mccp-{plan,implement,pr}-codex/comax-secrets-m3-github-actions.json`, git-ignored) |
| 4   | **Webhooks + Secret referencing/overrides** | Secret 변경 → Docker 서비스 재시작 webhook. inline 참조와 env override 모델.        | complete    | [plan](../plans/completed/comax-secrets-m4-webhooks.plan.md) · [report](../reports/comax-secrets-m4-webhooks.report.md) <br/>(참조/오버라이드는 M1/M2 resolver로 선-배송; 본 마일스톤 실질 범위=webhooks. gate: impeccable layout + cross-gate dedupe(codex plan R1) + code-review APPROVE(CRITICAL/HIGH 0); PR #13 (merge `27ef96b`, 2026-07-03); receipt: `.claude/receipts/mccp-{plan,implement,pr}-codex,code-reviewer/comax-secrets-m4-webhooks.json`, git-ignored) |
| 5   | **Node/TS SDK + npm publish**               | Next.js 앱에서 runtime 시크릿 fetch + cache + reload.                               | complete    | [plan](../plans/comax-secrets-m5-node-ts-sdk.plan.md) · [report](../reports/comax-secrets-m5-node-ts-sdk.report.md) <br/>(`@comax-secrets/sdk` zero-dep, dual ESM/CJS. gate: plan-codex needs-attention 4건 흡수(D5/D8/D9) + cross-gate dedupe(implement); 40 tests·cov 95.7%·live smoke PASS; receipt: `.claude/receipts/mccp-{plan,implement}-codex/comax-secrets-m5-node-ts-sdk.json`, git-ignored; PR: auto-chain) |
| 6   | **Website + Docs (Next.js, Vercel)**        | 랜딩 + quickstart + self-host + CLI/SDK reference + action 예제 + SEO.              | pending     | —                                                                                               |
| 7   | **(Decision)** Infra config templating PoC  | redis.conf / nginx.conf 환경별 렌더링 v1 포함 여부 결정.                            | pending     | —                                                                                               |
| 8   | **Public release (MIT)**                    | GitHub repo + npm 패키지 + docs site + GH Action marketplace 등록.                  | pending     | —                                                                                               |

## Open Questions

- [ ] **#1 Worktree-aware CLI 의 컨텍스트 결정 방식**
  - 후보: ① `.secretrc` per-worktree, ② Git branch → env 매핑, ③ 하이브리드 (.secretrc 우선, branch fallback).
  - 권장(잠정): **③ 하이브리드** — 명시성과 자동화 균형. `/plan` 에서 확정.
- [ ] **#2 Docker Swarm 자동 배포에서 시크릿 변경 후 서비스 재시작 방식**
  - 후보: ① Webhook → 사용자 정의 스크립트, ② 공식 Swarm adapter (sidecar/init), ③ `secret run` 으로 swarm service entrypoint 래핑.
  - 사용자는 `docker secret` 미사용 결정 → ① 또는 ③ 우선.
- [ ] **#3 Server master key 관리 모델**
  - 후보: ① 파일 + filesystem 권한, ② OS keyring, ③ 외부 KMS 옵션, ④ 위 모두 지원.
  - NAS 환경 호환성 우선 → ① 기본 + ④ 확장 가능 설계.
- [ ] **#4 Dashboard 와 Website 의 코드베이스 분리/통합**
  - 후보: ① 모노레포 + Next.js 한 앱 (route 분기), ② Next.js (마케팅) + 별도 SPA (대시보드).
  - SEO 는 마케팅 페이지만 중요 → 통합 시 SSR/CSR 경계 정의 필요. `/plan` 에서 확정.
- [ ] **#5 Infra config templating (redis.conf, nginx.conf) v1 포함 여부**
  - 사용자의 실제 페인에 직접 부합하나 envvar 모델과 별도 시스템 필요.
  - **결정 보류** — PoC 후 In/Out 판단.
- [ ] **#6 CLI 구현 언어 (Go vs Rust)**
  - Cold start, 단일 바이너리 배포, cross-compile 측면에서 둘 다 후보. `/plan` 에서 trade-off 분석.
- [ ] **#7 본인 운영 환경 telemetry/측정 방법**
  - 성공 지표("80% 단축", "누락 0") 측정의 baseline 기록 방식 필요.

## Risks

| Risk                                                                           | Likelihood | Impact | Mitigation                                                                                              |
| ------------------------------------------------------------------------------ | ---------- | ------ | ------------------------------------------------------------------------------------------------------- |
| **스코프 폭주** — Dashboard + CLI + SDK + Action + Webhook + Website 동시 추진 | High       | High   | Milestone 1 (서버 + CLI) 까지는 본인 운영을 실제로 대체할 수 있어야 다음 단계 진행. Dashboard 는 그 뒤. |
| **본인 1인 운영의 동기 소진**                                                  | High       | High   | "본인의 12개 `.env` 를 실제로 없앤다"는 단일 즉각 보상을 Milestone 1 에 잠가둠.                         |
| **Self-host 보안 모델 misconfig** (마스터 키 노출 등)                          | Medium     | High   | 기본값 안전 (filesystem 권한 강제), 잘못된 권한 시 부팅 거부. docs 에 위협 모델 명문화.                 |
| **시장 차별화 약함** — Infisical/Doppler 대비 USP 흐림                         | Medium     | Medium | "NAS 친화 + Worktree 1급 + GitHub Actions 통합 + Config templating" 이라는 4축으로 명시 포지셔닝.       |
| **Infra config templating 포함 시 v1 범위 폭증**                               | Medium     | Medium | PoC → 별도 결정. v1 미포함 시 v2 우선순위 1순위 예약.                                                   |
| **MIT 라이센스 하 SaaS 복제 위험**                                             | Low        | Medium | v1 운영 검증 후 Enterprise 모듈을 별도 라이선스로 분리하는 dual-license 옵션 유지.                      |
| **SQLite 동시성 / 성능 한계**                                                  | Low        | Medium | v1 사용처(개인/소규모)는 충분. 향후 backend 추상화로 Postgres 옵션 추가 여지 둠.                        |

---

_Status: DRAFT — requirements only. Implementation planning pending via `/plan`._
_Generated by `/ecc:plan-prd` on 2026-05-30._
