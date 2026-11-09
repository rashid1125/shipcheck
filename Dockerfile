FROM golang:1.22-alpine AS builder

# VERSION passed via --build-arg from CI
ARG VERSION=dev

WORKDIR /build
COPY go.mod ./
RUN go mod download

COPY . .
RUN go build \
    -ldflags="-s -w -X github.com/rashid1125/launchsafe/cmd.Version=${VERSION}" \
    -o launchsafe .

# ── final image (no Go toolchain, ~2 MB) ─────────────────────────────────────
FROM alpine:3.20

LABEL org.opencontainers.image.source="https://github.com/rashid1125/launchsafe"
LABEL org.opencontainers.image.description="Deployment-readiness CLI — local, read-only project checks before you deploy"
LABEL org.opencontainers.image.licenses="MIT"

RUN adduser -D -u 1000 launchsafe
USER launchsafe

COPY --from=builder /build/launchsafe /usr/local/bin/launchsafe

ENTRYPOINT ["launchsafe"]
CMD ["help"]
