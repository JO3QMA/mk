# Cypress e2e wrapper

本ディレクトリは Misskey 本家の Cypress e2e スイートを mk-go のバックエンドに
差し向けるためのラッパー。**本家コードは 1 行もコピーしていない**。
spec / support / fixtures はすべて `third_party/misskey/cypress` を参照する
設定になっている (`cypress.config.ts` を参照)。

ライセンス境界: Misskey 本家は AGPL-3.0 で、本家の cypress 資産を mk-go の
リポジトリにコピーするのは再配布扱いになるため、submodule で参照する方式を
取っている。LICENSE の整備自体はフォーク元オーナーの作業範囲。

## 前提

- Docker が動くこと (すべての node/cypress コマンドは Docker 経由で実行する。
  mk-go のグローバル規約で「パッケージはホストに直接入れない」と決まっているため)
- mk-go を `MK_TESTMODE=1` で起動できること
- submodule が初期化済みであること (`make e2e-submodule-init`)

## セットアップ手順

```bash
# 1. submodule を初期化する
make e2e-submodule-init

# 2. 本家フロントエンドをビルドする (数分〜10 分かかる)
make e2e-frontend-build

# 3. cypress の依存を入れる (Docker 内)
make e2e-deps

# 4. 別端末で mk-go を TEST MODE で起動する
MK_TESTMODE=1 \
MISSKEY_FRONTEND_DIR=$(pwd)/third_party/misskey/built/_frontend_vite_ \
MISSKEY_FRONTEND_DIST_DIR=$(pwd)/third_party/misskey/built/_frontend_dist_ \
MISSKEY_CLIENT_ASSETS_DIR=$(pwd)/third_party/misskey/packages/frontend/assets \
./built/misskey -config .config/default.yml

# 5. Cypress を実行する
make e2e-run
```

`MK_TESTMODE=1` を忘れると `/api/reset-db` が存在せず Cypress の `resetState`
カスタムコマンドが失敗する。

## cypress のカスタムコマンドが依存するエンドポイント

Misskey 本家の `cypress/support/commands.ts` は以下を前提にしている:

| コマンド | エンドポイント | mk-go 側の対応 |
|---------|---------------|---------------|
| `visitHome` | `GET /` | `internal/server/frontend.go` の SPA ハンドラ |
| `resetState` | `POST /api/reset-db` | `internal/api/test/handler.go` (TestMode=true 時のみ登録) |
| `registerUser(admin=true)` | `POST /api/admin/accounts/create` | `internal/api/admin/handler.go` |
| `registerUser(admin=false)` | `POST /api/signup` | 既存の signup フロー |
| `login` | `POST /api/signin-flow` | 既存の signin フロー |

## 既知の落ち

初回実行で落ちるテストはここに追記する。Phase 11-1 の完了条件は「Cypress が
起動してカスタムコマンドが通ること」までで、個々の spec の緑化は次 phase に
分けて対応する。

- (初回実行後に埋める)

## 環境変数

| 変数 | 既定値 | 説明 |
|---|---|---|
| `E2E_BASE_URL` | `http://localhost:3000` | mk-go のホスト URL。CI などで上書きする |

## 補足: なぜ docker 経由なのか

CLAUDE.md ([project](../../CLAUDE.md) / ユーザーグローバル) で
「パッケージやツールはホストに直接インストールせず Docker コンテナで動かす」
と決まっている。cypress は electron / node バイナリを同梱するので素朴に
`pnpm install` するとホストを汚すため、Makefile では `docker run --rm
-v $(PWD):/work -w /work ...` 形式でラップしている。
