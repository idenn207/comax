# Comax Secrets — top-level Makefile.
#
# Targets:
#   build       Compile server + CLI binaries to ./bin
#   test        Race-enabled unit tests with coverage profile
#   cover       Print per-function coverage summary
#   lint        golangci-lint
#   bench       Run benchmarks (no tests)
#   docker      Build the server container image
#   xbuild      Cross-compile CLI for NAS targets (amd64, arm64, arm/v7)
#   clean       Remove build artefacts
#
# This Makefile assumes a POSIX-ish shell. Windows users should run it under
# WSL, Git Bash, or MSYS2 (CI uses ubuntu-latest).

GO        ?= go
PKG       := github.com/idenn207/comax-secrets
BIN_DIR   := bin
COVER_OUT := coverage.out

# Pull version from git when available; fall back to "dev" for clean trees.
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X '$(PKG)/internal/version.Version=$(VERSION)'

.PHONY: build test cover lint bench docker xbuild clean

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret-server ./cmd/server
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/secret        ./cmd/cli

test:
	$(GO) test ./internal/... -race -coverprofile=$(COVER_OUT)

cover: test
	$(GO) tool cover -func=$(COVER_OUT)

lint:
	golangci-lint run

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
