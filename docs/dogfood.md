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

---

## Milestone 2 — dashboard dogfood

M2 ships the operator dashboard. The PRD's M2 success criterion is
binary: the operator can do **everything `secret` does** through the
browser, plus the three things the CLI cannot do well — version-by-
version diff, single-secret rollback, and "key X exists in `local` but
not `prod`". The dogfood here pins time + click budgets against those
three CLI-painful flows, completed using *only the dashboard*.

This document is a **checklist the operator fills in by hand**, on the
operator's own seeded instance. The dashboard is browser-driven and
wall-clock-on-the-operator's-machine; nothing here can be auto-tested.

### M2 setup (one-time, before the timed flows)

- [ ] `docker compose -f deploy/compose/docker-compose.yml up -d`.
- [ ] Capture the bootstrap admin token from
      `docker compose logs secret-server`.
- [ ] Open `http://localhost:8080/` in a browser, paste the bootstrap
      token on `/login`, confirm you land on `/`.
- [ ] Verify a seeded fixture exists: at least one project with
      `local` + `prod` environments, ≥ 5 secrets in `local`, and one
      key that exists in `local` but not in `prod`.
- [ ] If not seeded, create the fixture via CLI once (out of band, not
      counted against the dogfood timings).

The dogfood is invalid if any of the three flows below required the
CLI. Note the device + browser used; touch vs trackpad changes the
click count materially.

### Flow A — Add one new env var to all envs of a project

The M1 painful flow ("12 `.env` files, 5 minutes baseline") repeated
through the dashboard. The point is that the dashboard makes this
roughly the same speed as the CLI, *plus* an audit trail.

| Step | Click budget | Time budget (sec) |
|---|---|---|
| Project home → pick project | 1 | 2 |
| Open `local` env secrets | 1 | 2 |
| "Add secret" → type key + value → save | 4 (open, key, value, save) | 10 |
| Open `prod` env secrets | 1 (breadcrumb or env switcher) | 2 |
| "Add secret" → type key + value → save | 4 | 10 |
| Verify both audit rows on `/audit` | 1 | 4 |
| **Total** | **≤ 12** | **≤ 30** |

Record:

- Date / device / browser: …
- Clicks: …
- Wall-clock: … s
- Audit rows show `secret.create` for both envs: yes / no
- Failures or surprises: …

### Flow B — Roll one secret back to a prior version

The CLI cannot show a version-by-version diff and cannot roll back a
single secret. This is the second flow.

| Step | Click budget | Time budget (sec) |
|---|---|---|
| Secret table → click the affected key row | 1 | 2 |
| Open the version timeline panel | 1 | 2 |
| Pick the prior version → see side-by-side diff | 1 | 4 |
| Confirm "this is the rollback target" | 0 (already focused) | 4 |
| "Roll back" → confirm dialog → submit | 3 (open, confirm checkbox, submit) | 10 |
| Verify the new version appears at the top of the timeline | 1 | 4 |
| Verify audit row `secret.rollback` on `/audit` | 1 | 4 |
| **Total** | **≤ 8** | **≤ 30** |

Record:

- New version number: …
- Old ciphertext re-appears as the latest value: yes / no
- Audit row attributes to the operator's underlying token: yes / no
- Wall-clock: … s

### Flow C — Find the key that exists in `local` but not in `prod`

The CLI requires the operator to manually diff two `secret list`
outputs. The dashboard's env-vs-env diff makes the answer literal.

| Step | Click budget | Time budget (sec) |
|---|---|---|
| Project home → pick project | 1 | 2 |
| Open `local` env → "Compare with…" → pick `prod` | 3 | 8 |
| Read the "Added in `local`" column (= keys not in `prod`) | 0 | 6 |
| Click through to a row → land on that key in the secrets table | 1 | 4 |
| Decide: add to `prod` (use Flow A from there) or leave intentional | 0 | — |
| **Total** | **≤ 5** | **≤ 20** |

Record:

- Keys-only-in-local count returned by the diff: …
- One concrete key the operator did not previously realise was
  local-only: …
- Wall-clock: … s

### Acceptance — M2 dogfood

- [ ] Flow A ≤ 30 s, ≤ 12 clicks.
- [ ] Flow B ≤ 30 s, ≤ 8 clicks.
- [ ] Flow C ≤ 20 s, ≤ 5 clicks.
- [ ] All three flows completed **without opening the CLI** for the
      duration of the measurement.
- [ ] Audit log on `/audit` shows the matching `secret.create`,
      `secret.rollback`, and (if Flow C led to Flow A) `secret.create`
      rows newest-first, attributed to the operator's bearer-derived
      session.

If any budget is missed, do not mark Milestone 2 complete. The most
likely culprits are listed against the matching flow in
[perf.md](perf.md) and the audit / version timeline routes in
[dashboard.md](dashboard.md).

### Recording the M2 result

Paste the four "Record" blocks above into this file with the actual
numbers, then commit. That commit is the artefact that closes
Milestone 2.
