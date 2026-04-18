# syntax=docker/dockerfile:1.7

# -----------------------------------------------------------------------
# Stage 1 — Build the React frontend (static bundle embedded later by Go).
# -----------------------------------------------------------------------
FROM node:25.9-alpine3.22 AS frontend

WORKDIR /app/web

# Dependency manifests first → this layer caches as long as package*.json
# is unchanged, so application edits don't re-run `npm ci`.
COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# -----------------------------------------------------------------------
# Stage 2 — Compile the Go binary with the bundled frontend embedded.
# -----------------------------------------------------------------------
FROM golang:1.26-alpine AS backend

WORKDIR /app

# 1) Pull modules in a layer keyed only by go.mod + go.sum — reused across
#    source-code edits. BuildKit caches `go mod download`'s download dir
#    via --mount to speed up subsequent (including cold-cache CI) builds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# 2) App sources.
COPY . .

# 3) Pre-built frontend — Go's embed.FS picks it up from web/dist.
COPY --from=frontend /app/web/dist ./web/dist

# 4) Build a static, stripped binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/elara ./cmd/service

# 5) Pre-create the bbolt data directory so the scratch image can carry it
#    with the runtime UID/GID baked in. scratch has no shell, so we cannot
#    mkdir/chown there — we prepare it here and COPY --chown below.
RUN mkdir -p /out/data

# -----------------------------------------------------------------------
# Stage 3 — Minimal runtime image.
# -----------------------------------------------------------------------
FROM scratch
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /bin/elara /bin/elara
# Empty, 65532-owned directory — the service's default CONFIG_DATA_PATH.
# Without this, docker run (no mount, no env override) fails with
# "mkdir data: permission denied" because the scratch rootfs is read-only
# for the non-root UID.
COPY --from=backend --chown=65532:65532 /out/data /var/lib/elara

# Distroless-style non-root UID/GID. Matches the Helm chart defaults
# (podSecurityContext.runAsUser: 65532) and prevents the container from
# running as root when deployed outside the chart.
USER 65532:65532

# Sane runtime defaults so docker run ghcr.io/…/elara works zero-config.
ENV CONFIG_DATA_PATH=/var/lib/elara
VOLUME ["/var/lib/elara"]
EXPOSE 8080 2379

ENTRYPOINT ["/bin/elara"]
