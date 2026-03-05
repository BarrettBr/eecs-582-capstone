# --- build ---
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Only install what you actually need to build (git only if you pull modules via git)
RUN apk add --no-cache ca-certificates git

# Cache deps
COPY ./ingest/go.mod ./ingest/go.sum ./
RUN go mod download

# Copy source
COPY ./ingest .

# Build a small static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/main .

# --- runtime ---
FROM alpine:3.23.3
WORKDIR /app

# Non-root user + runtime deps (certs are usually needed for HTTPS)
RUN addgroup -S app && adduser -S -G app app && \
    apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/main ./main

USER app
CMD ["./main"]
