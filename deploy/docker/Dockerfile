# syntax=docker/dockerfile:1.7

# ----- builder ----------------------------------------------------------
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

ARG VERSION=docker
ENV CGO_ENABLED=0 GOOS=linux

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
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
