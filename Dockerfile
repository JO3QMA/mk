# syntax=docker/dockerfile:1.7
#
# `# syntax=` directive は BuildKit の RUN --mount=type=cache を有効に
# するために必須 (#432)。ローカル `docker build` でも CI (GitHub Actions
# runner) でも default frontend が 1.5+ になる現代では `1.7` で問題なし。

# Stage 1: Build Go binary
FROM golang:1.25-alpine AS builder

# build-base は chai2010/webp などの cgo 依存をリンクするのに必要。
# git は go mod download 時の private module fetch に使う。
RUN apk add --no-cache git build-base

WORKDIR /app

COPY go.mod go.sum ./
# go module cache を BuildKit cache mount に乗せると、再ビルド時の
# `go mod download` が依存に変更が無ければ no-op で済む (#432)。
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# third_party/misskey (submodule) がruntime stageのCOPY対象になるので、
# ビルド前に初期化されているか確認する。CIは docker.yml 側で submodules:
# recursive を指定して取得する。ローカル build 時はユーザに指示を出す。
RUN test -f third_party/misskey/packages/backend/assets/favicon.ico || \
    (echo "ERROR: third_party/misskey submodule not initialized (or partial clone)." && \
     echo "Run: git submodule update --init --recursive" && exit 1)

# twemojiは本家frontendがUnicode絵文字描画に使うSVG set。pnpm installで
# node_modulesに hoistされる前提 (make e2e-frontend-build等で install済み)。
RUN test -f third_party/misskey/packages/backend/node_modules/@discordapp/twemoji/dist/svg/1f004.svg || \
    (echo "ERROR: twemoji assets not found (pnpm install not run?)." && \
     echo "Run: make e2e-frontend-build (installs third_party/misskey node_modules)" && exit 1)

# Go の build cache (`$GOCACHE` = /root/.cache/go-build) と module cache を
# BuildKit cache mount として永続化する。再ビルド時に変更の無いパッケージは
# 再コンパイルされずに layer 完成までが秒単位になる (#432)。
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /app/built/misskey ./cmd/misskey && \
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

# repo-level assets (ai.png等)。frontendが /assets/ai.png で参照する
# (mascotImageUrl のデフォルト)。submodule直下 (issue #360)。
COPY --from=builder /app/third_party/misskey/assets /app/repo-assets
ENV MISSKEY_REPO_ASSETS_DIR=/app/repo-assets

# twemoji SVG set (Unicode絵文字描画)。frontendが /twemoji/<codepoint>.svg
# で参照する。約18MB (issue #359)。
COPY --from=builder /app/third_party/misskey/packages/backend/node_modules/@discordapp/twemoji/dist/svg /app/twemoji
ENV MISSKEY_TWEMOJI_DIR=/app/twemoji

# デフォルト設定ファイルをコピー (docker-compose でマウント上書き可能)
COPY .config/docker.yml /app/.config/default.yml

EXPOSE 3000

ENTRYPOINT ["/app/misskey"]
CMD ["-config", ".config/default.yml"]
