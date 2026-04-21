.PHONY: build run dev clean tidy test fmt lint migrate-up migrate-down migrate-create \
	federation-misskey-build federation-misskey-up federation-misskey-test \
	federation-misskey-down federation-misskey-logs \
	dropin-up dropin-down dropin-test dropin-logs \
	dropin-mk-up dropin-mk-test dropin-mk-down dropin-mk-logs dropin-swap-test \
	e2e-submodule-init e2e-frontend-build e2e-deps e2e-run e2e-open \
	uds-init uds-frontend-build uds-build uds-up uds-down uds-down-v uds-logs uds-ps \
	bench-up bench-run bench-down bench-logs

# Binary output
BINARY=misskey
BUILD_DIR=./built

# Go parameters
GOFLAGS=-trimpath

# バージョン情報: submodule の package.json があれば Misskey バージョンを自動取得
MKGO_VERSION ?= 0.0.1
MISSKEY_PKG_JSON = third_party/misskey/package.json
MISSKEY_VERSION ?= $(shell [ -f $(MISSKEY_PKG_JSON) ] && grep -o '"version": *"[^"]*"' $(MISSKEY_PKG_JSON) | head -1 | grep -o '"[^"]*"$$' | tr -d '"' || echo "2026.3.2")
LDFLAGS=-s -w \
	-X github.com/shiroha-a/mk/internal/config.MkGoVersion=$(MKGO_VERSION) \
	-X github.com/shiroha-a/mk/internal/config.MisskeyVersion=$(MISSKEY_VERSION)

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/misskey

run: build
	$(BUILD_DIR)/$(BINARY) -config .config/default.yml

dev:
	go run ./cmd/misskey -config .config/default.yml

clean:
	rm -rf $(BUILD_DIR)

tidy:
	go mod tidy

test:
	go test ./... -v

fmt:
	gofmt -s -w .

lint:
	go vet ./...

# Migration (requires DATABASE_URL env var)
migrate-up:
	go run ./cmd/migrate -direction up

migrate-down:
	go run ./cmd/migrate -direction down

migrate-create:
	@read -p "Migration name: " name; \
	touch migration/$$(printf "%06d" $$(($$(ls migration/*.up.sql 2>/dev/null | wc -l) + 1)))_$${name}.up.sql; \
	touch migration/$$(printf "%06d" $$(($$(ls migration/*.down.sql 2>/dev/null | wc -l) + 1)))_$${name}.down.sql

# Docker
docker-build:
	docker build -t misskey-go .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Federation tests ― 本家 Misskey と実際に立ち上げて連合動作を検証する。
# 各ターゲット (misskey / mastodon / pleroma / ...) ごとに docker-compose.federation.<target>.yml を用意する。
FEDERATION_MISSKEY_COMPOSE=docker-compose.federation.misskey.yml

federation-misskey-build:
	docker compose -f $(FEDERATION_MISSKEY_COMPOSE) build

federation-misskey-up:
	docker compose -f $(FEDERATION_MISSKEY_COMPOSE) up -d --build

federation-misskey-test:
	docker compose -f $(FEDERATION_MISSKEY_COMPOSE) --profile test run --rm test-runner

federation-misskey-down:
	docker compose -f $(FEDERATION_MISSKEY_COMPOSE) down -v

federation-misskey-logs:
	docker compose -f $(FEDERATION_MISSKEY_COMPOSE) logs -f

# Drop-in e2e (#365) ― Misskey TS 2 インスタンス (A, B) を立ち上げて
# 連合基盤を検証する。Phase 13-1 では TS ↔ TS の smoke test のみ。
# Phase 13-2 以降で mk 差し替え overlay を追加する予定。
DROPIN_COMPOSE=docker-compose.dropin.yml

dropin-up:
	docker compose -f $(DROPIN_COMPOSE) up -d --build

dropin-test:
	docker compose -f $(DROPIN_COMPOSE) --profile test run --rm test-runner

dropin-down:
	docker compose -f $(DROPIN_COMPOSE) down -v

dropin-logs:
	docker compose -f $(DROPIN_COMPOSE) logs -f

# Drop-in mk overlay (#367) — instance A の backend を mk-go に差し替えた
# 状態で TS-A 用 stack を起動する。連合先 (instance B) は TS のままなので
# mk ↔ TS federation も同時に検証できる。
DROPIN_MK_OVERLAY=docker-compose.dropin.mk.yml

dropin-mk-up:
	docker compose -f $(DROPIN_COMPOSE) -f $(DROPIN_MK_OVERLAY) up -d --build

dropin-mk-test:
	docker compose -f $(DROPIN_COMPOSE) -f $(DROPIN_MK_OVERLAY) --profile test run --rm test-runner

dropin-mk-down:
	docker compose -f $(DROPIN_COMPOSE) -f $(DROPIN_MK_OVERLAY) down -v

dropin-mk-logs:
	docker compose -f $(DROPIN_COMPOSE) -f $(DROPIN_MK_OVERLAY) logs -f

# Drop-in swap シナリオ (#367): TS-A → mk-A 切替で state が引き継げることを
# 検証する end-to-end テスト。bash orchestrator が以下を順次実行する:
#   1. TS-A + TS-B 起動
#   2. test_swap_setup.py で alice/bob/follow/note を作る
#   3. TS-A backend を停止
#   4. overlay で mk-A 起動 (DB-A / Redis-A はそのまま)
#   5. test_swap_verify.py で state preserved + 新規 federation を確認
dropin-swap-test:
	./tests/dropin/run-swap-test.sh

# Cypress e2e ― Misskey 本家の cypress spec を mk-go に向けて実行する。
#
# ライセンス境界のため、本家コードはすべて third_party/misskey/ の git submodule
# 参照で扱う。mk-go のリポジトリには 1 行もコピーしない。
#
# CLAUDE.md の規約で「パッケージはホストに直接入れずコンテナ経由で動かす」と
# 決まっているため、pnpm / cypress はすべて docker run で実行する。
E2E_NODE_IMAGE=node:22-bookworm
E2E_CYPRESS_IMAGE=cypress/included:15.11.0
E2E_WORKDIR=/work

# submodule を初期化し、Misskey 本家の cypress 資産とフロントエンドソースを取得する。
e2e-submodule-init:
	git submodule update --init --recursive third_party/misskey

# 本家フロントエンドを Docker 内でビルドする。数分〜10 分程度かかる。
# 成果物は third_party/misskey/packages/frontend/... 配下に出力される。
# パッチは submodule (shiroha-a/misskey-ts 2026.3.2-fix) に直接コミット済み。
e2e-frontend-build:
	docker run --rm -v $(PWD):$(E2E_WORKDIR) -w $(E2E_WORKDIR)/third_party/misskey \
		$(E2E_NODE_IMAGE) \
		bash -lc "corepack enable && corepack prepare pnpm@latest --activate && pnpm install --frozen-lockfile && pnpm build"

# Cypress ラッパーの依存を Docker 内で解決する。
e2e-deps:
	docker run --rm -v $(PWD):$(E2E_WORKDIR) -w $(E2E_WORKDIR)/e2e/cypress \
		$(E2E_NODE_IMAGE) \
		bash -lc "corepack enable && corepack prepare pnpm@latest --activate && pnpm install"

# ヘッドレスで cypress run を実行する。
# host network で動かして mk-go (localhost:3000) に直接届かせる。
e2e-run:
	docker run --rm --network=host -v $(PWD):$(E2E_WORKDIR) -w $(E2E_WORKDIR)/e2e/cypress \
		-e E2E_BASE_URL=$${E2E_BASE_URL:-http://localhost:3000} \
		$(E2E_CYPRESS_IMAGE) \
		cypress run --e2e --browser electron --config-file cypress.config.ts

# 開発者向けに cypress open を起動する (X11 forward が必要なので CI では使わない)。
e2e-open:
	docker run --rm --network=host -v $(PWD):$(E2E_WORKDIR) -w $(E2E_WORKDIR)/e2e/cypress \
		-e DISPLAY=$$DISPLAY -v /tmp/.X11-unix:/tmp/.X11-unix \
		-e E2E_BASE_URL=$${E2E_BASE_URL:-http://localhost:3000} \
		$(E2E_CYPRESS_IMAGE) \
		cypress open --e2e --browser electron --config-file cypress.config.ts

# UDS-only compose stack (Phase 12-2)。Phase 12-1 で入った UNIX domain socket
# 対応を使って nginx → mk-go → postgres / valkey をすべて UDS で繋ぐ。
# 詳細は docs/docker-uds.md を参照。
UDS_COMPOSE=compose.uds.yaml
UDS_CONFIG=deploy/uds/config/default.yml

# compose / config の実ファイルはデプロイ先ごとに書き換えるため gitignore 済み。
# 初回起動時のみ .example からコピーする (order-only prerequisite なので、
# 一度作成したあとは .example を更新してもユーザのローカル編集を上書きしない)。
$(UDS_COMPOSE):
	cp $(UDS_COMPOSE).example $(UDS_COMPOSE)

$(UDS_CONFIG):
	cp $(UDS_CONFIG).example $(UDS_CONFIG)

uds-init: | $(UDS_COMPOSE) $(UDS_CONFIG)

# 本家 vite フロントエンドを docker 内でビルドする。初回は 3〜10 分程度かかる。
# 既存 e2e-frontend-build のエイリアス (成果物先が同じなので共有して OK)。
uds-frontend-build: e2e-frontend-build

uds-build: | $(UDS_COMPOSE) $(UDS_CONFIG)
	docker compose -f $(UDS_COMPOSE) build

uds-up: | $(UDS_COMPOSE) $(UDS_CONFIG)
	docker compose -f $(UDS_COMPOSE) up -d --build

uds-down: | $(UDS_COMPOSE)
	docker compose -f $(UDS_COMPOSE) down

# named volume も含めて完全削除する (DB データも全部消える)。
uds-down-v: | $(UDS_COMPOSE)
	docker compose -f $(UDS_COMPOSE) down -v

uds-logs: | $(UDS_COMPOSE)
	docker compose -f $(UDS_COMPOSE) logs -f

uds-ps: | $(UDS_COMPOSE)
	docker compose -f $(UDS_COMPOSE) ps

# Benchmark ― mk-go vs 本家 Misskey のストレステスト比較。
# k6 (Docker) で同一エンドポイントに負荷をかけ、レイテンシ・スループットを比較する。
# 結果は tests/bench/results/report.md に出力される。
BENCH_COMPOSE=tests/bench/docker-compose.bench.yml

bench-up:
	docker compose -f $(BENCH_COMPOSE) up -d --build

bench-run:
	docker compose -f $(BENCH_COMPOSE) --profile bench up --abort-on-container-exit compare

bench-down:
	docker compose -f $(BENCH_COMPOSE) down -v

bench-logs:
	docker compose -f $(BENCH_COMPOSE) logs -f
