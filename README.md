# Comax Secrets

A single-binary, self-hosted secrets server with a matching CLI (`secret`).
Built to replace hand-synced `.env` files with one source of truth that ships
encrypted secrets to dev workstations, CI runners, and production
containers.

> **Status**: Milestone 1 in progress — see
> [`.claude/plans/comax-secrets.plan.md`](.claude/plans/comax-secrets.plan.md).
> Today the binaries only print their version; subcommands land in
> Tasks 7–10.

## Layout

```
.
├── cmd/
│   ├── server/        # secret-server binary entrypoint
│   └── cli/           # secret CLI binary entrypoint
├── internal/
│   └── version/       # shared build-time version constant
├── .github/workflows/ # CI: test, lint, cross-compile matrix
├── Makefile           # build / test / lint / xbuild / docker
└── .claude/           # PRDs, plans, working notes
```

`internal/store`, `internal/crypto`, `internal/server`, `internal/auth`,
`internal/config`, `internal/secret`, and `pkg/client` are added in
Tasks 2–9.

## Quickstart (M1 placeholder)

```bash
make build
./bin/secret-server   # prints: secret-server <version>
./bin/secret          # prints: secret <version>
```

Real quickstart (`docker compose up` → `secret login` → `secret push`) ships
with Task 13.

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
