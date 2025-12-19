# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

# ---- final image ----
FROM alpine:3.20

WORKDIR /app

RUN adduser -D -g '' appuser
USER appuser

COPY --from=builder /app/server /app/server

ENV MP_BASE_DIR=/app/data
ENV MP_DB_PATH=/app/data/meta.db

EXPOSE 8080

ENTRYPOINT ["/app/server"]
