.PHONY: build run dev clean tidy test fmt lint migrate-up migrate-down migrate-create \
	federation-misskey-build federation-misskey-up federation-misskey-test \
	federation-misskey-down federation-misskey-logs

# Binary output
BINARY=misskey
BUILD_DIR=./built

# Go parameters
GOFLAGS=-trimpath
LDFLAGS=-s -w

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
