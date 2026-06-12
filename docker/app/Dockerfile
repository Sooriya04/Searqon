FROM golang:1.24-alpine AS builder

WORKDIR /app

# Note: The context of this build is assumed to be the root of the project
COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./
RUN go build -ldflags="-s -w" -o /searqon .

# ── Final minimal image ──────────────────────────────────────────────────────
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /searqon .

EXPOSE 4001

ENTRYPOINT ["/app/searqon"]
