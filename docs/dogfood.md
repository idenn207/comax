# Operator dogfood — Task 14

The PRD's headline success metric for Milestone 1 is binary:

> Adding one new envvar to all 12 production `.env` files takes
> ≤ 1 minute (vs ≥ 5 minutes baseline), end-to-end, with the operator
> running the actual services.

This document is a **checklist the operator fills in by hand**. The
software cannot self-validate this metric — the inputs (12 real `.env`
files, the operator's actual project) are private and the measurement
is wall-clock-on-the-operator's-machine.

## Setup

- [ ] Deploy the server via `deploy/compose/docker-compose.yml` on the
      operator's NAS / VPS (`docker compose up -d`).
- [ ] Capture the bootstrap admin token from
      `docker compose logs secret-server`.
- [ ] Run `secret login --server <url> --token <token>` from the
      operator's workstation.
- [ ] For each of the 12 services × envs (`api`/`web`/`mq`/`infra` ×
      `local`/`dev`/`prod`), run:
      ```bash
      cd <service-dir>
      secret init --project <service> --envs local,dev,prod \
                  --default-env local
      secret push --file .env       # for the local env
      secret push --file .env.dev --env dev
      secret push --file .env.prod --env prod
      ```
- [ ] Verify each service starts via
      `secret run -- <existing-start-cmd>` instead of the previous
      direct `npm run dev` / `go run` / etc.
- [ ] Delete the now-redundant `.env*` files from each repo (or move
      them to `.env.example` placeholders).

## Baseline measurement (BEFORE)

Time it takes to add **one new envvar** (e.g. `FEATURE_FLAG_X=true`)
across the 12 existing files **without** Comax Secrets:

| Step | Time (ss) |
|---|---|
| Open 12 `.env` files in editor | |
| Add the line to each | |
| Distribute the changes (Slack? rsync? individual SCPs?) | |
| Restart affected services | |
| **Total** | |

Record once, before the migration.

## After measurement (AFTER)

Same change with Comax:

```bash
secret set FEATURE_FLAG_X=true --env local --project api
secret set FEATURE_FLAG_X=true --env dev --project api
secret set FEATURE_FLAG_X=true --env prod --project api
# ...repeat for web, mq, infra (or script a loop)
```

| Step | Time (ss) |
|---|---|
| Run `secret set` 12 times (or scripted) | |
| Restart affected services (services pick up the new env on next `secret run`) | |
| **Total** | |

## Acceptance

- [ ] AFTER ≤ 60 seconds.
- [ ] AFTER ≤ BEFORE × 0.2 (5× speedup minimum).

If either fails, do not mark the milestone complete. Common causes:

- CLI cold start above 300 ms (see [perf.md](perf.md)) — a per-call
  cost that compounds when you script the loop.
- Server cold start above its budget — the long-running container
  should be warm; if the container restarts during the test, throw out
  that sample.

## Recording the result

Once both gates above pass, paste the table values into this file and
commit. That commit is the artefact that closes Milestone 1.
