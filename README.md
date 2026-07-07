# Comax Secrets

[![CI](https://github.com/idenn207/comax-secrets/actions/workflows/ci.yml/badge.svg)](https://github.com/idenn207/comax-secrets/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](go.mod)

A single-binary, self-hosted secrets server with a matching CLI (`secret`).
Built to replace hand-synced `.env` files with one source of truth that ships
encrypted secrets to dev workstations, CI runners, and production containers —
without a SaaS bill or a heavy Vault-style deployment.

Why it exists (the four axes that make it different): **NAS-friendly**
(single SQLite mount, no external Postgres/Redis), **worktree-first**
(`secret run -- <cmd>` injects the right env by context), **GitHub Actions
native** (a composite action makes GitHub Secret registration disappear), and
**config templating** (`secret render` fills `redis.conf` / `nginx.conf` per
environment).

> **Status**: Milestones 1–7 shipped end-to-end — server + CLI (M1), operator
> dashboard (M2), GitHub Actions integration (M3), webhooks + secret
> referencing/overrides (M4), Node/TS SDK (M5), marketing + docs site (M6), and
> `secret render` config templating (M7). M8 packages the public MIT release
> (binaries, npm publish, Action Marketplace). Full docs live at the docs site
> (built from [`website/content/docs`](website/content/docs)).

## Install

Pick whichever fits — all ship the same `secret` CLI and `secret-server`.

**Prebuilt binary** (from the [latest release](https://github.com/idenn207/comax-secrets/releases)):

```bash
# Linux amd64 example — swap os/arch as needed (linux|darwin, amd64|arm64)
VERSION=v1.0.0
curl -fsSL -o secret \
  "https://github.com/idenn207/comax-secrets/releases/download/${VERSION}/secret-linux-amd64"
curl -fsSL -O \
  "https://github.com/idenn207/comax-secrets/releases/download/${VERSION}/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS   # verify before trusting
chmod +x secret && ./secret --version
```

Release binaries carry a GitHub build-provenance attestation — verify it with
`gh attestation verify secret --repo idenn207/comax-secrets`.

**Go install** (needs Go 1.25+):

```bash
go install github.com/idenn207/comax-secrets/cmd/cli@latest   # installs `secret`
```

**Docker** (server): see [Quickstart](#quickstart).

## Quickstart

```bash
docker compose -f deploy/compose/docker-compose.yml up -d --build
docker compose -f deploy/compose/docker-compose.yml logs secret-server \
  | grep -A 1 "bootstrap admin token"   # capture the token (one shot)

secret login --server http://localhost:8080 --token <token>
secret init  --project my-app --envs local,dev,prod --default-env local
secret push  --file .env
secret run -- npm run dev         # secrets injected as env, no disk write
```

Prefer a browser? Open <http://localhost:8080/> and paste the same bootstrap
token on `/login`. See [docs/dashboard.md](docs/dashboard.md) for the operator
dashboard.

## GitHub Actions

Inject secrets into a workflow with no GitHub Secret registration. In v1 the
action downloads and verifies the CLI itself (checksum + provenance) — just
pin `cli-version`:

```yaml
- uses: idenn207/comax-secrets@v1
  with:
    server: https://secrets.example.com
    token: ${{ secrets.COMAX_TOKEN }}
    project: my-app
    env: prod
    cli-version: v1.0.0            # downloads + verifies the CLI (or pass cli-path)
    run: npm run deploy            # secrets reach only this command's env
```

Default mode is process-env (secrets stay out of the job at large); opt into
job-wide injection with `export-to: github-env`. See
[docs/github-actions.md](docs/github-actions.md).

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
│   ├── tmpl/          # secret render config templating (M7)
│   └── version/       # shared build-time version constant
├── pkg/
│   └── client/        # HTTP client shared by CLI + SDK
├── deploy/
│   ├── docker/        # Multi-stage Dockerfile (distroless final)
│   └── compose/       # docker-compose.yml with bind-mounted data + keys
├── docs/              # dev-internal docs; user-facing docs are canonical on the site
├── sdk/               # @comax-secrets/sdk — Node/TS SDK (M5)
├── website/           # marketing + docs site — Next.js/Vercel (M6)
├── .github/workflows/ # CI: test, lint, cross-compile, action, sdk, website, release, secret-scan
├── Makefile           # build / test / lint / xbuild / xbuild-release / docker / sdk / website
└── .claude/           # PRDs, plans, working notes
```

**Docs canonical (M6)**: user-facing docs (quickstart, self-host, CLI/SDK
reference, GitHub Actions, webhooks, security) are canonical on the docs site,
built from [`website/content/docs`](website/content/docs). The user-facing
files under `docs/` are thin stubs that point there; `docs/` otherwise keeps
dev-internal material (threat-model deep-dive, perf budget, dogfood checklists).

## Development

| Action          | Command                |
| --------------- | ---------------------- |
| Build           | `make build`           |
| Test (race)     | `make test`            |
| Coverage        | `make cover`           |
| Lint            | `make lint`            |
| Cross-compile   | `make xbuild`          |
| Release build   | `make xbuild-release`  |
| Docker image    | `make docker`          |

Go **1.25+** is required; the build is pure-Go (`CGO_ENABLED=0`) so it
cross-compiles cleanly to `linux/{amd64,arm64,arm/v7}`, `darwin/{amd64,arm64}`,
and `windows/amd64`. See [CONTRIBUTING.md](CONTRIBUTING.md) to get started.

## Security

Comax Secrets assumes the self-host operator owns the DB and master key (not a
zero-knowledge model — see the threat model). Report vulnerabilities privately
per [SECURITY.md](SECURITY.md), never in a public issue.

## Conventions

- Errors wrapped with `fmt.Errorf("op: %w", err)`; sentinel errors per domain
  (e.g. `store.ErrNotFound`).
- Repository pattern over `*sql.DB`; transactions explicit at the call site.
- `log/slog` everywhere; JSON in the server, text in the CLI; secrets never
  logged (tests assert this).
- Per-package coverage floor **≥ 80%**.

## License

[MIT](LICENSE) © 2026 idenn207.
