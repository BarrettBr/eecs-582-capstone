# syntax=docker/dockerfile:1

# --- build ---
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src

RUN apk add --no-cache ca-certificates git

# Cache deps
COPY ./ingest/go.mod ./ingest/go.sum ./
RUN go mod download

# Copy source
COPY ./ingest .

# Build for target platform (defaults keep local builds simple).
RUN set -eux; \
    export CGO_ENABLED=0; \
    export GOOS="${TARGETOS:-linux}"; \
    export GOARCH="${TARGETARCH:-amd64}"; \
    if [ "${TARGETARCH}" = "arm" ] && [ -n "${TARGETVARIANT}" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    go build -trimpath -ldflags="-s -w" -o /out/main ./main.go

# --- runtime ---
FROM alpine:3.23.3 AS runtime
WORKDIR /app

RUN addgroup -S app && adduser -S -G app app && \
    apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/data && \
    chown -R app:app /app

COPY --from=builder --chown=app:app /src /app
COPY --from=builder --chown=app:app /out/main ./main

USER app
CMD ["./main"]
