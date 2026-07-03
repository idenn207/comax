# Codex Findings Backlog

> mccp gate에서 `DEFER_TO_BACKLOG` 처리된 finding과,
> closure plan이 의도적으로 미룬 reversibility trigger를 기록한다.
> 한 줄 한 항목, 형식: `YYYY-MM-DD | <severity> | <source plan path> | <one-line finding>`.

- 2026-06-15 | INFO | .claude/plans/completed/comax-secrets-dashboard-m2-close.plan.md | D1(최소 live smoke) 결정 — dashboard 관련 PRD 갱신 시 docs/dashboard-dogfood.md Flow A/B/C 측정 trigger 여부 revisit
- 2026-06-15 | HIGH | .claude/plans/completed/comax-secrets-dashboard.plan.md#L259 | M2 본체 Task 15 acceptance (three flows ≤ 30s logged in docs/dogfood.md) — closure D1에 의해 deferred. 운영 trigger 발생 시 acceptance 충족 또는 명시적 폐기 결정 필요. Codex stop-time review가 충돌 flag (2026-06-15).
- 2026-07-02 | MEDIUM | .claude/plans/comax-secrets-m4-webhooks.plan.md | (R1 F3) 웹훅 서명 시크릿 in-place 회전(`RotateWebhookSecret` + old/new overlap + 배달 이력 보존) 부재 — v1은 delete+recreate 워크어라운드. 유출 대응 UX 개선/이력 보존 필요 시 M4 후속 또는 v2에서 구현.
- 2026-07-03 | MEDIUM | .claude/reviews/archive/pr-13-review.md | (code-review PR#13 M1) `webhook_deliveries`에 prune/retention 부재 — 세션은 hourly 스위퍼가 있으나 배달 이력(delivered/dead)은 무한 누적. `DeliveryRepo.PruneTerminal(before)` + 세션 대칭 스위퍼 추가 또는 시간 기반 삭제. 비차단(자가호스팅·저빈도 slow-burn).
- 2026-07-03 | LOW | .claude/reviews/archive/pr-13-review.md | (code-review PR#13 L1) `webhook_deliveries.webhook_id` 인덱스 부재 — `ListByWebhook`·`ON DELETE CASCADE`가 풀스캔. `CREATE INDEX idx_deliveries_webhook ON webhook_deliveries(webhook_id)`. 위 M1(prune)과 함께 처리 권장.
