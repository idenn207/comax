# syntax=docker/dockerfile:1.7

# ----- dashboard builder -----------------------------------------------
# Build the React/Vite SPA first so the Go builder can embed it via
# //go:embed. Node 22 LTS matches CI (.github/workflows/ci.yml) and the
# engines.node field in web/dashboard/package.json. Node 22 LTS ships
# npm 10, which is what we use as the package manager.
#
# vite.config.ts writes its output to ../../internal/server/dashboard/dist
# (an absolute path resolved at build time), so the staged layout under
# /src/web/dashboard reproduces that relationship and the Go builder
# below picks up /src/internal/server/dashboard/dist unchanged.
FROM node:22-alpine AS dashboard-builder
# CI=true keeps npm from emitting progress/spinner output in the
# no-TTY container build context.
ENV CI=true
WORKDIR /src/web/dashboard
COPY web/dashboard/package.json web/dashboard/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci
COPY web/dashboard ./
RUN mkdir -p /src/internal/server/dashboard/dist && npm run build

# ----- go builder -------------------------------------------------------
# Pinned Go 1.25 because modernc.org/sqlite v1.51 needs it. CGO is off so
# the final binary is fully static and can run on distroless/static.
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Cache go.mod/go.sum separately so layer reuse skips the slow download
# when only source code (not deps) changes.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
# Pull the freshly built SPA bundle into the embed dir so //go:embed
# all:dist picks it up under -tags embed_dashboard.
COPY --from=dashboard-builder /src/internal/server/dashboard/dist /src/internal/server/dashboard/dist

ARG VERSION=docker
ENV CGO_ENABLED=0 GOOS=linux

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -tags embed_dashboard \
      -ldflags "-s -w -X github.com/idenn207/comax-secrets/internal/version.Version=${VERSION}" \
      -o /out/secret-server ./cmd/server

# ----- runtime ----------------------------------------------------------
# distroless/static is ~2 MB, no shell, no package manager — minimal
# attack surface. The :nonroot variant runs as uid 65532 by default.
FROM gcr.io/distroless/static:nonroot

# Volume mount points (writable by uid 65532 when bind-mounted by compose).
WORKDIR /app
COPY --from=builder /out/secret-server /app/secret-server

# Defaults that match the bind-mount layout in deploy/compose.
ENV COMAX_DB_PATH=/app/data/secrets.db \
    COMAX_MASTER_KEY_FILE=/app/keys/master.key \
    COMAX_LISTEN=:8080 \
    COMAX_AUTO_GENERATE_KEY=1

EXPOSE 8080

# distroless doesn't ship a shell, so HEALTHCHECK in the Dockerfile must
# call the binary directly or use docker-compose's healthcheck (which we
# do — see deploy/compose/docker-compose.yml). Setting it here would
# require shipping wget/curl.

ENTRYPOINT ["/app/secret-server"]
