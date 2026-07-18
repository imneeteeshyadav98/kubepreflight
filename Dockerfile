# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0

# Release identity, threaded through to internal/buildinfo via -X. Default
# to the same dev/unknown placeholders buildinfo.go itself falls back to,
# so an ordinary `docker build .` with no --build-arg still works.
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
# Only the built Console output + the go:embed directive that references it
# — not web/src, web/node_modules, etc. The React app is built once at
# release time (see web/README.md); this Docker build never runs npm.
COPY web/embed.go ./web/embed.go
COPY web/dist ./web/dist
RUN go build -trimpath -ldflags="-s -w \
      -X kubepreflight/internal/buildinfo.Version=${VERSION} \
      -X kubepreflight/internal/buildinfo.Commit=${COMMIT} \
      -X kubepreflight/internal/buildinfo.BuildDate=${BUILD_DATE}" \
      -o /out/kubepreflight ./cmd/kubepreflight

# distroless/static: no shell, no package manager, CA certs included for
# TLS to the Kubernetes API server / AWS APIs. The runtime is non-root;
# docker-compose maps it to a configurable host UID/GID for bind mounts.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /work
COPY --from=builder /out/kubepreflight /usr/local/bin/kubepreflight

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/kubepreflight"]
CMD ["--help"]
