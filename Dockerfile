# ── Stage 1: Statically Compile the Go Binary ─────────────────────────────────
FROM golang:alpine AS builder

# Install SSL certificates and timezone data for HTTPS scraping calls
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Cache Go module dependencies
COPY src/go.mod src/go.sum ./
RUN go mod download

# Compile static binary with zero external libc/CGO dependencies
COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o /searqon .

# ── Stage 2: Scratch (Lightest possible container — 0 MB OS footprint) ────────
FROM scratch

# Copy system SSL CA certificates and timezone database
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# Copy statically linked binary
COPY --from=builder /searqon /app/searqon

# Copy default YAML configuration
COPY config/ /app/config/

EXPOSE 7493

ENTRYPOINT ["/app/searqon"]
