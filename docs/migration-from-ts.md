# Misskey-TSからMisskey-Goへの移行ガイド

本ガイドでは、既存のMisskey-TSインスタンスのバックエンドをMisskey-Goに置き換え、同じデータベース・Redis・フロントエンド資産を共有させる手順を説明する。

## 前提条件

- Go 1.24+
- PostgreSQL 15+ (既存のMisskey-TSデータベース)
- Redis 7+
- Misskey-TSのソースコード (フロントエンド資産のビルドに必要)
- git

## 1. クローンとビルド

```bash
git clone https://github.com/shiroha-a/mk.git misskey-go
cd misskey-go
go build -o built/misskey ./cmd/misskey
```

## 2. フロントエンド資産のビルド

Misskey-GoはMisskey-TSと同じフロントエンドを利用する。ビルド済みのフロントエンド資産を別途用意する必要がある。

```bash
# Misskey-TSのクローン (未取得なら)
git clone https://github.com/misskey-dev/misskey.git ../misskey
cd ../misskey

# 依存インストールとビルド
pnpm install
pnpm build

# 成果物は以下に配置される:
#   built/_frontend_vite_/  (JS/CSS バンドル)
#   built/_frontend_dist_/  (locale・フォント)
#   packages/frontend/assets/  (ゲーム画像・アイコン類)
```

## 3. 設定

### 3.1 アプリケーション設定

`.config/default.yml` を作成する:

```yaml
url: https://your-instance.example.com
port: 3000

db:
  host: localhost
  port: 5432
  db: misskey        # 既存のMisskey-TSデータベース
  user: misskey
  pass: your_password

redis:
  host: localhost
  port: 6379

id: aidx             # Misskey-TS側のID生成方式と一致させること
```

> **重要:** `id` フィールドはMisskey-TSインスタンスで使われているID生成方式と必ず一致させること。Misskey-TS側の `.config/default.yml` を確認して正しい値を設定する。よく使われる値: `aidx`, `aid`, `meid`, `ulid`, `objectid`。

### 3.2 環境変数

`.env` を作成する:

```bash
# ビルド済みフロントエンドのパス (JS/CSSバンドル)
MISSKEY_FRONTEND_DIR=/path/to/misskey/built/_frontend_vite_

# フロントエンドdist資産のパス (locale・フォント)
MISSKEY_FRONTEND_DIST_DIR=/path/to/misskey/built/_frontend_dist_

# twemoji SVGファイルのパス
MISSKEY_TWEMOJI_DIR=/path/to/misskey/node_modules/@discordapp/twemoji/dist/svg

# クライアント資産 (ゲーム画像等) のパス
MISSKEY_CLIENT_ASSETS_DIR=/path/to/misskey/packages/frontend/assets

# 静的資産 (favicon、splash、アイコン等) のパス
# MISSKEY_STATIC_DIR=assets
```

## 4. データベースマイグレーション

Misskey-Goは既存のMisskey-TSテーブルには手を加えず、独自のテーブルを追加する形で共存する。既存データは保持される。

```bash
# Misskey-Goのマイグレーションを適用
go run ./cmd/migrate -direction up
```

これによりGo側で必要な追加テーブル (`app`, `auth_session`, `webhook`, `sw_subscription`, `chat_room`, `chat_message`, `bubble_game_record` 等) が作成される。

> **補足:** Misskey-Goのマイグレーションは追加のみで、既存のMisskey-TSテーブルを変更しない。両バックエンドは同じデータベース上で共存できる。

## 5. Misskey-TSの停止

```bash
# 既存のMisskey-TSサーバーを停止する
# (停止方法はデプロイ手段による: systemd、pm2、docker 等)
systemctl stop misskey
# または
pm2 stop misskey
# または
docker compose stop web
```

## 6. Misskey-Goの起動

```bash
# 環境変数を読み込む
source .env

# サーバー起動
./built/misskey -config .config/default.yml
```

または環境変数を直接指定する場合:

```bash
MISSKEY_FRONTEND_DIR=/path/to/misskey/built/_frontend_vite_ \
MISSKEY_FRONTEND_DIST_DIR=/path/to/misskey/built/_frontend_dist_ \
MISSKEY_TWEMOJI_DIR=/path/to/misskey/node_modules/@discordapp/twemoji/dist/svg \
MISSKEY_CLIENT_ASSETS_DIR=/path/to/misskey/packages/frontend/assets \
./built/misskey -config .config/default.yml
```

## 7. 動作確認

1. ブラウザで `http://localhost:3000` を開く
2. エントランスページがスタイル付きで正しく表示されることを確認する
3. 既存アカウントでログインする
4. 以下を確認する:
   - タイムラインに自分のノートが表示される
   - プロフィールページが正しく表示される
   - 通知が動作する
   - ファイルアップロードが動作する
   - リアクションが動作する

## Docker Compose で新規構築する場合

新しいインスタンスをDocker Composeで立ち上げる場合は以下で起動できる:

```bash
docker compose up -d
```

Misskey-GoがPostgreSQLおよびRedisと共に起動する。詳細は `docker-compose.yml` を参照。

## Misskey-TSへのロールバック

Misskey-TSに戻す場合の手順:

1. Misskey-Goを停止する
2. 従来通りMisskey-TSを起動する

データベースは双方向に互換性があり、Misskey-Goが追加したテーブルはMisskey-TSからは無視される。

## 既知の制限

### スタブ実装

以下の機能は正常なレスポンスを返すものの、完全な処理は未実装:

- **2FA/WebAuthn** — 204を返すのみ (未実装)
- **Export/Import** — ジョブはキューに積まれるがワーカーが未実装
- **Reversi** — ゲーム一覧は取得できるがリアルタイム対戦は未実装
- **連合 (リモート)** — ローカルのActivityPubは動作するが、リモートオブジェクトの取得は限定的

### Misskey-TSとの差異

- **タイムライン** — Redisキャッシュが空の場合 (サーバー再起動直後等) はDBクエリにフォールバックする
- **Identicon** — 生成される自動アバターの見た目が若干異なる
- **通知** — WebSocketによるリアルタイム配信は未対応。ページ再読み込みで表示される

## トラブルシューティング

### ページが「Loading...」のまま進まない

- `MISSKEY_FRONTEND_DIR` が正しいビルド済みフロントエンドディレクトリを指しているか確認する
- フロントエンドが正常にビルドされているか確認する (`ls $MISSKEY_FRONTEND_DIR/manifest.json`)

### 絵文字が表示されない

- `MISSKEY_TWEMOJI_DIR` が正しいtwemoji SVGディレクトリを指しているか確認する
- 確認コマンド: `ls $MISSKEY_TWEMOJI_DIR/1f44d.svg`

### ゲーム画像が表示されない

- `MISSKEY_CLIENT_ASSETS_DIR` が `packages/frontend/assets/` を指しているか確認する
- 確認コマンド: `ls $MISSKEY_CLIENT_ASSETS_DIR/drop-and-fusion/`

### CSS/スタイルが崩れる

- プロダクションビルドを使用していることを確認する (Viteのdevモードではない)
- `MISSKEY_FRONTEND_DIST_DIR` が設定されているか確認する (locale・フォント用)

### 再起動後にタイムラインが空になる

- これは期待通りの挙動。新規ノートは即時にタイムラインへ反映される
- 既存ノートは初回のDBフォールバッククエリ実行後に表示される

### ファイルアップロードが `CREDENTIAL_REQUIRED` で失敗する

- 認証ミドルウェアが `multipart/form-data` リクエストを正しく処理できているか確認する
- サーバーログで認証エラーを確認する
