# Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /app/built/misskey ./cmd/misskey

# Stage 2: Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/built/misskey /app/misskey
COPY --from=builder /app/migration /app/migration

# デフォルト設定ファイルをコピー (docker-compose でマウント上書き可能)
COPY .config/docker.yml /app/.config/default.yml

EXPOSE 3000

ENTRYPOINT ["/app/misskey"]
CMD ["-config", ".config/default.yml"]
