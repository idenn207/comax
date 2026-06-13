# Quickstart

From a clean clone to your first `secret run` in under five minutes. The
goal of this walkthrough is to replace one `.env` file with a server-
managed secret store and run a service against it.

## Prerequisites

- Docker 24+ with Compose v2 (`docker compose`, not the old
  `docker-compose` binary).
- Go 1.25+ (only if you want to build the CLI from source — pre-built
  binaries are available under `bin/` after `make xbuild`).

## 1. Start the server

```bash
git clone https://github.com/idenn207/comax-secrets.git
cd comax-secrets

docker compose -f deploy/compose/docker-compose.yml up -d --build
```

First boot does three things automatically:

1. Creates `./data/secrets.db` (SQLite database).
2. Generates `./keys/master.key` (32-byte AES-256 master key, mode
   `0600`).
3. Mints the **bootstrap admin token** and prints it to the container
   log exactly once.

Capture the token now — it is shown only on the first boot:

```bash
docker compose -f deploy/compose/docker-compose.yml logs secret-server \
  | grep -A 1 "bootstrap admin token"
```

The output looks like:

```
Comax Secrets: bootstrap admin token (shown ONCE):
    <long-opaque-token-here>
```

If you miss it, delete `./data/secrets.db` and `./keys/master.key` and
restart — the server will bootstrap again on the next boot. Do **not**
rotate the key file in production; that would invalidate all stored
secrets.

## 2. Install the CLI

```bash
make build       # produces ./bin/secret and ./bin/secret-server
export PATH="$PWD/bin:$PATH"
```

Cross-compiled binaries for NAS targets are available via
`make xbuild`.

## 3. Log in

Paste the token from step 1:

```bash
secret login --server http://localhost:8080 --token <token-from-step-1>
```

This writes `~/.config/comax/credentials.json` with mode `0600`. The
plaintext token is **only** persisted here; the server stores its
SHA-256 hash.

> Or open `http://localhost:8080/` in a browser and paste the same
> token on `/login`. The dashboard sets an `HttpOnly`+`Secure`+
> `SameSite=Strict` session cookie scoped to `/`, and CSRF-protects
> every mutating call. See [dashboard.md](dashboard.md) for the full
> operator walkthrough.

## 4. Initialise the project

```bash
cd my-app                                # any directory you want to manage
secret init --project my-app \
            --envs local,dev,prod \
            --default-env local
```

This creates the project + three envs on the server and writes
`.secretrc` (gitignored) into the current directory. `.secretrc` pins
the project and the default env for this checkout; later milestones
will use it for worktree-aware context resolution.

## 5. Import your existing `.env`

Suppose you have a working `.env` you want to migrate:

```bash
cat .env
# DB_URL=postgres://localhost/dev
# API_KEY=sk-abc123
# REDIS_URL=redis://localhost:6379
```

Push it to the server in one shot:

```bash
secret push --file .env
```

Each key is uploaded as a new version under the `local` env.
`secret get DB_URL` should now return the value, and you can delete
your local `.env` file:

```bash
rm .env
```

## 6. Run your service

The headline command. `secret run -- <cmd>` pulls the env in memory,
merges with the parent shell's env, and forks `<cmd>` with secrets
injected as environment variables. **Nothing is written to disk during
this step.**

```bash
secret run -- npm run dev
# or
secret run --env prod -- ./my-binary
```

The child sees `DB_URL`, `API_KEY`, etc. directly — same as if you
had `source .env` first, except the values never live on disk.

## 7. Update a secret across envs

```bash
secret set NEW_FLAG=enabled --env local
secret set NEW_FLAG=enabled --env dev
secret set NEW_FLAG=enabled --env prod
```

`secret diff --against prod` shows what differs between the current
env and `prod`.

## What's next

- [Threat model](threat-model.md) — what the server protects against
  and what it does not.
- Milestones 2–6 in the [PRD](../.claude/prds/comax-secrets.prd.md) add
  a dashboard, GitHub Action, language SDK, and webhook events.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `docker compose up` exits with "permission denied" on `keys/master.key` | The host file is mode > 0600. Run `chmod 0600 ./keys/master.key` or delete the file and let the server regenerate it. |
| `secret login` returns 401 | Token typed wrong or the wrong server URL. The server logs the failed-auth path; check `docker compose logs secret-server`. |
| `secret pull` returns an empty `.env` | Project + env exist but no secrets have been pushed. Run `secret push --file .env` first. |
| `secret run` exits with the child's non-zero code | That's intentional — `run` is transparent to the child's exit code so CI pipelines and supervisors behave the same with or without it. |
