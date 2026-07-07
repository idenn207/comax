# Contributing to Comax Secrets

Thanks for your interest. This is a small, self-host-first project; contributions
that keep it lean and NAS-friendly are especially welcome.

## Ground rules

- **Pure Go.** Go **1.25+**, `CGO_ENABLED=0`. No cgo, so the build cross-compiles
  cleanly to `linux/{amd64,arm64,arm/v7}`.
- **Secrets never hit logs.** This is enforced by tests — do not weaken it.
- **Per-package test coverage ≥ 80%.**
- **User-facing error messages are English.** (Dashboard UI labels stay Korean.)

## Development

```bash
make build      # server (embedded dashboard) + CLI → ./bin
make test       # race-enabled unit tests with coverage
make lint       # golangci-lint (v2.x)
make xbuild     # cross-compile the CLI for NAS targets
```

Contributors without a Node toolchain can run `go test ./...` and
`go build ./...` directly — the dashboard embed sits behind the
`embed_dashboard` build tag (off by default). See the
[`Makefile`](Makefile) header for the full target list.

Conventions the codebase follows (mirror them, don't reinvent):

- Errors wrapped with `fmt.Errorf("op: %w", err)`; per-domain sentinel errors
  (e.g. `store.ErrNotFound`).
- Repository pattern over `*sql.DB`; transactions explicit at the call site.
- `log/slog` everywhere — JSON in the server, text in the CLI.

## Pull requests

1. Fork and branch from `master` (`feat/…`, `fix/…`, `docs/…`).
2. Keep the change focused; one logical change per PR.
3. `make build && make test && make lint` must pass locally before you push.
   CI runs the same gates plus cross-compile, dashboard, SDK, website, and a
   `docker compose up` smoke.
4. Add or update tests for behavior you change.
5. Write the PR description in plain language: what changed and why.

## Reporting bugs / requesting features

Use the issue templates. For **security** problems, follow
[`SECURITY.md`](SECURITY.md) instead of opening a public issue.
