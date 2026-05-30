# Comax Secrets

A single-binary, self-hosted secrets server with a matching CLI (`secret`).
Built to replace hand-synced `.env` files with one source of truth that ships
encrypted secrets to dev workstations, CI runners, and production
containers.

> **Status**: Milestone 1 server + CLI shipped end-to-end. Operator
> dogfood (Task 14) is the remaining acceptance gate. See
> [`.claude/plans/comax-secrets.plan.md`](.claude/plans/comax-secrets.plan.md)
> for the full task list and [`docs/quickstart.md`](docs/quickstart.md)
> for the 5-minute walkthrough.

## Layout

```
.
├── cmd/
│   ├── server/        # secret-server binary entrypoint (HTTP server)
│   └── cli/           # secret CLI binary entrypoint
├── internal/
│   ├── auth/          # bearer tokens + bootstrap flow
│   ├── cli/           # CLI helpers (credentials, dotenv, envctx, secretrc)
│   ├── crypto/        # AES-256-GCM seal/open + KeyProvider interface
│   ├── secret/        # ${{ env.KEY }} resolver + inheritance
│   ├── server/        # HTTP handlers, router, middleware
│   ├── store/         # SQLite store + per-entity repositories
│   └── version/       # shared build-time version constant
├── pkg/
│   └── client/        # HTTP client shared by CLI + future SDK
├── deploy/
│   ├── docker/        # Multi-stage Dockerfile (distroless final)
│   └── compose/       # docker-compose.yml with bind-mounted data + keys
├── docs/              # quickstart, threat-model, perf, dogfood
├── .github/workflows/ # CI: test, lint, cross-compile matrix
├── Makefile           # build / test / lint / xbuild / docker
└── .claude/           # PRDs, plans, working notes
```

## Quickstart

```bash
docker compose -f deploy/compose/docker-compose.yml up -d --build
docker compose -f deploy/compose/docker-compose.yml logs secret-server \
  | grep -A 1 "bootstrap admin token"   # capture the token (one shot)

make build
./bin/secret login --server http://localhost:8080 --token <token>
./bin/secret init  --project my-app --envs local,dev,prod --default-env local
./bin/secret push  --file .env
./bin/secret run -- npm run dev         # secrets injected as env, no disk write
```

See [docs/quickstart.md](docs/quickstart.md) for the full 5-minute
walkthrough, [docs/threat-model.md](docs/threat-model.md) for the
operator security obligations, and [docs/perf.md](docs/perf.md) for
the 300 ms cold-start budget.

## Development

| Action          | Command                |
| --------------- | ---------------------- |
| Build           | `make build`           |
| Test (race)     | `make test`            |
| Coverage        | `make cover`           |
| Lint            | `make lint`            |
| Cross-compile   | `make xbuild`          |
| Docker image    | `make docker`          |

Go **1.25+** is required (raised from the plan's 1.22 floor because
`modernc.org/sqlite` v1.51 requires it); the build is pure-Go
(`CGO_ENABLED=0`) so it
cross-compiles cleanly to `linux/{amd64,arm64,arm/v7}` for typical NAS
targets.

## Conventions

This milestone **establishes** the conventions later milestones must
follow. See the plan's "Patterns to Mirror" table; the short version:

- Errors wrapped with `fmt.Errorf("op: %w", err)`; sentinel errors per
  domain (e.g. `store.ErrNotFound`).
- Repository pattern over `*sql.DB`; transactions explicit at the call
  site.
- `log/slog` everywhere; JSON in the server, text in the CLI; secrets
  never logged (tests assert this).
- Per-package coverage floor **>=80%** by Task 5; CI starts at 70%.
