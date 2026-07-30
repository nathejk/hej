# syntax=docker/dockerfile:1
#
# Multistage build for the Hej Nathejk app.
#
#   golang:1.25 ──► api-dev ──► base ──► build ─┐
#                                               │
#   node:20-alpine ──► ui-dev ──► ui-builder ───┤
#                                               ▼
#                                     alpine ──► prod
#
# Dev `docker compose up` only ever targets `base` (api) and `ui-dev` (ui).
# `prod` is the only stage that ships.

##########################################################################
# API (Go BFF)
##########################################################################

# api-dev: Go toolchain + inotify for the hot-reload dev loop. The go tool
# linters (staticcheck/gosec/govulncheck) are pinned in go/go.mod and run via
# `go tool`, so nothing extra is installed here.
FROM golang:1.25 AS api-dev
RUN apt-get update \
    && apt-get install -y --no-install-recommends inotify-tools \
    && rm -rf /var/lib/apt/lists/*
ENV GOCACHE=/go/.cache/go-build
WORKDIR /app
COPY docker/init/api-dev /usr/local/bin/api-dev
RUN chmod +x /usr/local/bin/api-dev
ENTRYPOINT ["api-dev"]

# base: the image the `api` compose service runs. Modules are pre-downloaded so
# cold container starts are fast; the actual source is bind-mounted in dev.
FROM api-dev AS base
COPY go/go.mod go/go.sum ./
RUN go mod download
COPY go/ ./

# build: prod-only. Runs the test + lint gates, then compiles a static,
# CGO-free binary that runs on the bare alpine prod image.
FROM base AS build
ARG BUILD_VERSION=dev
RUN go test -timeout 60s ./... \
    && go tool staticcheck ./...
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w" \
    -o /hej-api \
    nathejk.dk/cmd/api

##########################################################################
# UI (Vue 3 SPA)
##########################################################################

# ui-dev: the image the `ui` compose service runs. Source + node_modules are
# provided via volumes; the entrypoint runs `npm ci && npm run dev` (Vite HMR).
# Vite's dev server listens on :80, so EXPOSE it — Traefik's docker provider
# uses the exposed port to discover the service (no loadbalancer port label
# needed since it's the default 80).
FROM node:20-alpine AS ui-dev
WORKDIR /app
COPY docker/init/ui-dev /usr/local/bin/ui-dev
RUN chmod +x /usr/local/bin/ui-dev
EXPOSE 80
ENTRYPOINT ["ui-dev"]

# ui-builder: prod-only. Installs deps and builds the SPA to /app/dist.
FROM node:20-alpine AS ui-builder
WORKDIR /app
COPY vue/package.json vue/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY vue/ ./
RUN npm run build

##########################################################################
# Production image
##########################################################################

# prod: bare alpine + the Go binary, which serves the built SPA from /www as a
# fallback after /api/... routes. BUILD_VERSION is exposed as SENTRY_RELEASE to
# match the org CI convention.
FROM alpine:3.20 AS prod
ARG BUILD_VERSION=dev
ENV SENTRY_RELEASE=${BUILD_VERSION}
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /hej-api /hej-api
COPY --from=ui-builder /app/dist /www
ENV ENV=production
ENV WEB_ROOT=/www
ENV PORT=4000
EXPOSE 4000
ENTRYPOINT ["/hej-api"]
