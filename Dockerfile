FROM golang:1.22-alpine AS builder

ARG VERSION=dev

WORKDIR /build
COPY go.mod ./
RUN go mod download

COPY . .
RUN go build \
    -ldflags="-s -w -X github.com/rashid1125/shipcheck/cmd.Version=${VERSION}" \
    -o shipcheck .

# ── final image (no Go toolchain, ~2 MB) ─────────────────────────────────────
FROM alpine:3.20

LABEL org.opencontainers.image.source="https://github.com/rashid1125/shipcheck"
LABEL org.opencontainers.image.description="Offline deployment-readiness preflight CLI"
LABEL org.opencontainers.image.licenses="MIT"

RUN adduser -D -u 1000 shipcheck
USER shipcheck

COPY --from=builder /build/shipcheck /usr/local/bin/shipcheck

ENTRYPOINT ["shipcheck"]
CMD ["help"]
