# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
# Only the built Console output + the go:embed directive that references it
# — not web/src, web/node_modules, etc. The React app is built once at
# release time (see web/README.md); this Docker build never runs npm.
COPY web/embed.go ./web/embed.go
COPY web/dist ./web/dist
RUN go build -trimpath -ldflags="-s -w" -o /out/kubepreflight ./cmd/kubepreflight

# distroless/static: no shell, no package manager, CA certs included for
# TLS to the Kubernetes API server / AWS APIs. Root user for now (Week 1) so
# bind-mounted output directories don't need host-side UID matching; a
# nonroot hardening pass can follow once the write paths are finalized.
FROM gcr.io/distroless/static-debian12:latest
WORKDIR /work
COPY --from=builder /out/kubepreflight /usr/local/bin/kubepreflight

ENTRYPOINT ["/usr/local/bin/kubepreflight"]
CMD ["--help"]
