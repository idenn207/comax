# Comax Secrets — top-level Makefile.
#
# Targets:
#   build               Compile server (with embedded dashboard) + CLI to ./bin
#   build-server-nodash Server-only build, no dashboard, no Node toolchain required
#   dashboard           npm ci + vite build, staged into the Go embed dir
#   dashboard-clean     Remove the staged SPA bundle (keeps .gitkeep)
#   dev                 Run secret-server + Vite dev server side-by-side
#   dev-api             secret-server only (no SPA, /api routes still respond)
#   dev-web             Vite dev server only (proxies /api → :8080)
#   test                Race-enabled unit tests with coverage profile
#   cover               Print per-function coverage summary
#   lint                golangci-lint
#   sdk                 Install + typecheck + lint + test + build the TS SDK
#   website             Install + typecheck + lint + build + verify the docs site
#   bench               Run benchmarks (no tests)
#   docker              Build the server container image
#   xbuild              Cross-compile CLI for NAS targets (amd64, arm64, arm/v7)
#   clean               Remove build artefacts
#
# Notes:
#   - This Makefile assumes a POSIX-ish shell. Windows users should run it
#     under WSL, Git Bash, or MSYS2 (CI uses ubuntu-latest).
#   - `build` requires Node + npm because it embeds the dashboard. Use
#     `build-server-nodash` if you only need the API surface.
#   - Contributors without Node can run `go test ./...` and `go build ./...`
#     directly; the //go:embed directive sits behind a build tag
#     (`embed_dashboard`) that is off by default.

GO            ?= go
NPM           ?= npm
PKG           := github.com/idenn207/comax-secrets
BIN_DIR       := bin
COVER_OUT     := coverage.out
DASHBOARD_DIR := dashboard
DASHBOARD_OUT := internal/server/dashboard/dist
SDK_DIR       := sdk
WEBSITE_DIR   := website

# Pull version from git when available; fall back to "dev" for clean trees.
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X '$(PKG)/internal/version.Version=$(VERSION)'

.PHONY: build build-server-nodash test cover lint bench docker xbuild clean \
        dashboard dashboard-clean dev dev-api dev-web sdk website

# Production build: dashboard first (so dist/ is populated), then the
# server binary with -tags embed_dashboard so //go:embed picks it up.
build: dashboard
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -tags embed_dashboard -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret-server ./cmd/server
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret        ./cmd/cli

# Server-only build that skips the SPA. The binary still serves /api and
# /healthz; the SPA endpoint returns the envelope-shape 404.
build-server-nodash:
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret-server ./cmd/server
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret        ./cmd/cli

# Build the SPA. Vite is configured to write directly into the Go embed
# dir (see dashboard/vite.config.ts) so no extra copy step is needed.
dashboard:
	cd $(DASHBOARD_DIR) && $(NPM) ci --silent
	cd $(DASHBOARD_DIR) && $(NPM) run build

# Wipe the staged SPA bundle but preserve the .gitkeep sentinel so the
# //go:embed all:dist pattern still resolves in a clean tree.
dashboard-clean:
	find $(DASHBOARD_OUT) -mindepth 1 ! -name .gitkeep -delete

# Side-by-side dev loop. Vite proxies /api → :8080. Ctrl-C tears down
# both processes via the trap (works in bash, zsh, Git Bash).
dev:
	@trap 'kill 0' SIGINT SIGTERM; \
	$(GO) run ./cmd/server & \
	cd $(DASHBOARD_DIR) && $(NPM) run dev

dev-api:
	$(GO) run ./cmd/server

dev-web:
	cd $(DASHBOARD_DIR) && $(NPM) run dev

test:
	$(GO) test ./internal/... -race -coverprofile=$(COVER_OUT)

cover: test
	$(GO) tool cover -func=$(COVER_OUT)

lint:
	golangci-lint run

# Build + verify the TypeScript SDK (@comax-secrets/sdk). Deliberately NOT a
# dependency of `test`/`lint`: those stay Go-only so contributors without a
# Node toolchain can still run them (see header note). CI runs `sdk` as a
# required check via the `sdk` job in .github/workflows/ci.yml (npm publish is
# a separate manual workflow, .github/workflows/sdk-publish.yml).
sdk:
	cd $(SDK_DIR) && $(NPM) ci
	cd $(SDK_DIR) && $(NPM) run typecheck
	cd $(SDK_DIR) && $(NPM) run lint
	cd $(SDK_DIR) && $(NPM) run test
	cd $(SDK_DIR) && $(NPM) run build

# Build + verify the marketing/docs website (@comax/website, Next.js). Like
# `sdk`, deliberately NOT a dependency of `test`/`lint` (Go-only). CI runs
# `website` via .github/workflows/website.yml; Vercel deploys via Git
# integration (operator-linked project, not this Makefile).
website:
	cd $(WEBSITE_DIR) && $(NPM) ci
	cd $(WEBSITE_DIR) && $(NPM) run typecheck
	cd $(WEBSITE_DIR) && $(NPM) run lint
	cd $(WEBSITE_DIR) && $(NPM) run build
	cd $(WEBSITE_DIR) && node scripts/check-token-parity.mjs
	cd $(WEBSITE_DIR) && node scripts/check-docs-coverage.mjs
	cd $(WEBSITE_DIR) && node scripts/check-docs-drift.mjs
	cd $(WEBSITE_DIR) && node scripts/check-site-url.mjs

bench:
	$(GO) test -bench=. -run=^$$ -benchmem ./...

docker:
	docker build -t comax-secrets-server -f deploy/docker/Dockerfile .

# Cross-compile the CLI for typical self-host NAS targets.
xbuild:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64           CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret-linux-amd64  ./cmd/cli
	GOOS=linux GOARCH=arm64           CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret-linux-arm64  ./cmd/cli
	GOOS=linux GOARCH=arm   GOARM=7   CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret-linux-armv7  ./cmd/cli

clean:
	rm -rf $(BIN_DIR) $(COVER_OUT)
