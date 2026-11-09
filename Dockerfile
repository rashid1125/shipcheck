# Multi-stage build — final image has only the binary (no Go toolchain)
FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY go.mod ./
RUN go mod download

COPY . .
RUN go build \
    -ldflags="-s -w -X github.com/rashid1125/launchsafe/cmd.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o launchsafe .

# ── final image ──────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN adduser -D -u 1000 launchsafe
USER launchsafe

COPY --from=builder /build/launchsafe /usr/local/bin/launchsafe

ENTRYPOINT ["launchsafe"]
CMD ["help"]
