FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go_scraper/go.mod go_scraper/go.sum ./
RUN go mod download

COPY go_scraper/ ./
RUN go build -ldflags="-s -w" -o /searqon .

# ── Final minimal image ──────────────────────────────────────────────────────
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /searqon .

EXPOSE 4001

ENTRYPOINT ["/app/searqon"]
