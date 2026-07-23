# syntax=docker/dockerfile:1

# Multi-stage build for RetroBytes.
#
# Runtime configuration is supplied as environment variables by the host, not
# baked into the image and not passed as build args:
#   PORT               listen port (defaults to 8081 when unset)
#   SEED_DEMO          set to "true" to seed demo user accounts
#   ADMIN_EMAIL        admin account email (with ADMIN_PASSWORD, seeds an admin)
#   ADMIN_PASSWORD     admin account password (no hardcoded default)
#   COOKIE_SECURE      set to "true" behind HTTPS to mark cookies Secure
#   SESSION_TTL_HOURS  authenticated session lifetime in hours (default 24)
# No secrets are stored in the image.

# ---- Builder ----
# Pinned to match the toolchain in go.mod (go1.25.12).
FROM golang:1.25.12 AS builder

WORKDIR /src

# Cache module downloads first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

# Build the static binary. The SQLite driver (modernc.org/sqlite) is pure Go,
# so CGO can be disabled to produce a self-contained static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/retrobytes ./cmd/retrobytes

# ---- Runtime ----
# Distroless static: no shell, no package manager, runs as nonroot by default.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# The compiled binary and the web assets it serves (templates, static, media).
COPY --from=builder /out/retrobytes /app/retrobytes
COPY --from=builder /src/web /app/web

# Default listen port. The app reads PORT at runtime and falls back to 8081.
ENV PORT=8081
EXPOSE 8081

# nonroot user is already set by the base image.
ENTRYPOINT ["/app/retrobytes"]
