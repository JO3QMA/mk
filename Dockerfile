# Stage 1: Build Go binary
FROM golang:1.25-alpine AS builder

# build-base は chai2010/webp などの cgo 依存をリンクするのに必要。
# git は go mod download 時の private module fetch に使う。
RUN apk add --no-cache git build-base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# third_party/misskey (submodule) がruntime stageのCOPY対象になるので、
# ビルド前に初期化されているか確認する。CIは docker.yml 側で submodules:
# recursive を指定して取得する。ローカル build 時はユーザに指示を出す。
RUN test -d third_party/misskey/packages/backend/assets || \
    (echo "ERROR: third_party/misskey submodule not initialized." && \
     echo "Run: git submodule update --init --recursive" && exit 1)

RUN go build -trimpath -ldflags="-s -w" -o /app/built/misskey ./cmd/misskey && \
    go build -trimpath -ldflags="-s -w" -o /app/built/migrate ./cmd/migrate

# Stage 2: Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/built/misskey /app/misskey
COPY --from=builder /app/built/migrate /app/migrate
COPY --from=builder /app/migration /app/migration

# 本家のpackages/backend/assets (favicon / icons等) をimageに焼き込む。
# bind-mountなしでも /favicon.ico / /static-assets/* 等が serve できる
# (issue #346)。third_party/misskey はsubmoduleなのでビルド前に
# `git submodule update --init --recursive` が必要。
COPY --from=builder /app/third_party/misskey/packages/backend/assets /app/static-assets
ENV MISSKEY_STATIC_DIR=/app/static-assets

# デフォルト設定ファイルをコピー (docker-compose でマウント上書き可能)
COPY .config/docker.yml /app/.config/default.yml

EXPOSE 3000

ENTRYPOINT ["/app/misskey"]
CMD ["-config", ".config/default.yml"]
