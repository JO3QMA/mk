# 設定リファレンス

## 設定ファイル

設定ファイルはMisskey互換のYAML形式。CLIフラグで指定する:

```bash
./built/misskey -config .config/default.yml
```

### 初回セットアップ

`.config/` 配下には Misskey 本家流儀で `.example` テンプレートを配布している。初回は以下のいずれかを手元の `.config/default.yml` (および `docker.yml`) として複製してから編集する:

```bash
# ローカル開発 / バイナリ直起動
cp .config/default.yml.example .config/default.yml

# Docker / docker-compose
cp .config/docker.yml.example .config/docker.yml
```

`.config/default.yml` と `.config/docker.yml` は `.gitignore` 対象 (operator-local) なので、コピーした手元のファイルが履歴に混入することはない。Docker image は build 時に `.config/docker.yml.example` をコンテナ内の `/app/.config/default.yml` として焼き込むので、運用環境では docker-compose 等で実 config を volume mount で上書きする想定。

## 全設定項目

### 基本設定

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `url` | string | (必須) | サーバーの公開URL (例: `https://misskey.example.com`) |
| `port` | int | `3000` | HTTPポート |
| `socket` | string | - | UNIXドメインソケットパス (設定するとTCPの代わりにUDSでリッスン) |
| `chmodSocket` | string | - | UDSファイルのパーミッション (例: `"770"`) |
| `id` | string | `"aidx"` | IDジェネレータ (`aidx`, `aid`, `meid`, `ulid`, `objectid`) |
| `setupPassword` | string | - | 初期セットアップ時のパスワード |
| `disableHsts` | bool | `false` | HSTSヘッダーを無効化 |
| `enableIpRateLimit` | bool | `true` | IPベースのレート制限を有効化 |
| `pidFile` | string | - | PIDファイルパス |
| `testMode` | bool | `false` | テスト用エンドポイント(/api/reset-db)を有効化。**本番で絶対に使わない** |
| `jobQueueDriver` | string | `"asynq"` | ジョブキュー実装の選択。`asynq` (デフォルト) または `mkq` (BullMQ互換)。`mkq`にすると admin queue 画面が BullMQ 前提の Misskey TS frontend と wire-compatible になる。`MK_JOBQUEUEDRIVER`で上書き可。 |

### データベース (`db.*`)

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `db.host` | string | `"localhost"` | ホスト (`/`始まりならUNIXソケット) |
| `db.port` | int | `5432` | ポート |
| `db.db` | string | `"misskey"` | データベース名 |
| `db.user` | string | `"misskey"` | ユーザー名 |
| `db.pass` | string | - | パスワード |
| `db.disableCache` | bool | `false` | GORMのキャッシュ無効化 |
| `db.extra.ssl` | bool | `false` | SSL接続 |

`dbReplications: true`とすると`dbSlaves`設定でリードレプリカを使用可能。

### Redis (`redis.*`)

基本のRedis設定。全フィールドは各用途別Redis設定にも適用可能。

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `redis.host` | string | `"localhost"` | ホスト |
| `redis.port` | int | `6379` | ポート |
| `redis.pass` | string | - | パスワード |
| `redis.db` | int | `0` | データベース番号 |
| `redis.username` | string | - | ユーザー名 (Redis 6+ ACL) |
| `redis.prefix` | string | - | キープレフィックス |

### 用途別Redis

設定しない場合は`redis`にフォールバック。

| キー | 用途 |
|---|---|
| `redisForPubsub.*` | PubSub (イベント配信) |
| `redisForJobQueue.*` | ジョブキュー (asynq) |
| `redisForTimelines.*` | タイムラインキャッシュ |
| `redisForReactions.*` | リアクションバッファ |

### ジョブキュー制御

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `deliverJobConcurrency` | int | `16` | AP配信worker数 (mkq driver では deliver queue 専用、asynq driver では総 worker pool 上限) |
| `inboxJobConcurrency` | int | - | Inbox処理 worker 数 (mk-go は Inbox を同期処理する設計のため**現状 no-op**。TS互換維持のため受付のみ) |
| `relationshipJobConcurrency` | int | - | フォロー処理 worker 数 (mk-go は relationship queue を持たないため**現状 no-op**) |
| `deliverJobPerSec` | int | - | AP配信レート上限 (tasks/sec)。設定すると asynq middleware / mkq.WithRateLimit で worker dispatch が back-pressure される |
| `inboxJobPerSec` | int | - | mk-go では**現状 no-op** (上記同様) |
| `relationshipJobPerSec` | int | - | mk-go では**現状 no-op** (上記同様) |
| `deliverJobMaxAttempts` | int | driver 既定 | AP配信の最大リトライ回数 default。EnqueueDeliver で `WithMaxRetry` 未指定時にだけ適用される (#495) |
| `inboxJobMaxAttempts` | int | - | mk-go では**現状 no-op** (Inbox は同期処理) |

> **driver 間の差分**:
> - `asynq` driver は worker pool が共有なので `deliverJobConcurrency` は **総 concurrency** として扱われる (queue priority weight で deliver が優先される)。
> - `mkq` driver は queue ごとに worker を分けているので `deliverJobConcurrency` は **deliver queue 専用** の worker 数として扱われる。それ以外の queue は `Concurrency / len(queues)` の既定値を使う。

### メディア

ローカルストレージ (S3未設定時) のファイル保存先は`./drive-files`固定。Docker環境ではコンテナ内の`/app/drive-files`にボリュームマウントが必要。

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `maxFileSize` | int | `262144000` (250MB) | アップロードファイルの最大サイズ (バイト) |
| `mediaProxy` | string | - | メディアプロキシURL |
| `mediaProxySecret` | string | - | メディアプロキシの署名シークレット |
| `videoThumbnailGenerator` | string | - | 動画サムネイル生成サービスURL |

### ネットワーク

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `proxy` | string | - | HTTPプロキシURL |
| `proxySmtp` | string | - | SMTPプロキシURL |
| `proxyBypassHosts` | []string | - | プロキシを迂回するホスト |
| `allowedPrivateNetworks` | []string | - | AP fetchで許可するプライベートネットワーク |
| `outgoingAddress` | string | - | 送信元IPアドレス |
| `outgoingAddressFamily` | string | - | アドレスファミリー (`"ipv4"`, `"ipv6"`, `"dual"`) |

### 検索

| キー | 型 | 説明 |
|---|---|---|
| `fulltextSearch.provider` | string | 検索プロバイダ名 |
| `meilisearch.host` | string | Meilisearchホスト |
| `meilisearch.port` | int | Meilisearchポート |
| `meilisearch.apiKey` | string | APIキー |
| `meilisearch.ssl` | bool | SSL接続 |
| `meilisearch.index` | string | インデックス名 |
| `meilisearch.scope` | string | 検索スコープ |

Meilisearch未設定時はSQL ILIKE検索にフォールバック。

### パフォーマンス

| キー | 型 | デフォルト | 説明 |
|---|---|---|---|
| `perChannelMaxNoteCacheCount` | int | `1000` | チャンネルあたりのノートキャッシュ上限 |
| `perUserNotificationsMaxCount` | int | `500` | ユーザーあたりの通知キャッシュ上限 |
| `deactivateAntennaThreshold` | int | - | アンテナ非活性化の閾値 |

### ロギング

| キー | 型 | 説明 |
|---|---|---|
| `logging.sql.disableQueryTruncation` | bool | SQLログのクエリ切り詰めを無効化 |
| `logging.sql.enableQueryParamLogging` | bool | SQLログにパラメータ値を含める |

## 環境変数オーバーライド

`MK_`プレフィックス付きの環境変数で設定値を上書きできる。ネストキーは`_`区切り。

| 環境変数 | 対応YAMLキー |
|---|---|
| `MK_URL` | `url` |
| `MK_PORT` | `port` |
| `MK_SOCKET` | `socket` |
| `MK_DB_HOST` | `db.host` |
| `MK_DB_PORT` | `db.port` |
| `MK_DB_DB` | `db.db` |
| `MK_DB_USER` | `db.user` |
| `MK_DB_PASS` | `db.pass` |
| `MK_REDIS_HOST` | `redis.host` |
| `MK_REDIS_PORT` | `redis.port` |
| `MK_REDIS_PASS` | `redis.pass` |
| `MK_REDIS_DB` | `redis.db` |
| `MK_REDIS_USERNAME` | `redis.username` |
| `MK_ID` | `id` |
| `MK_MAXFILESIZE` | `maxFileSize` |
| `MK_MEDIAPROXYSECRET` | `mediaProxySecret` |
| `MK_TESTMODE` | `testMode` |

用途別Redisも同様 (例: `MK_REDISFORPUBSUB_HOST`)。

新しいオーバーライド対象を追加する場合は`internal/config/config.go`の`bindEnvKeys()`にキーを追加する。

## フロントエンド関連環境変数

| 環境変数 | 用途 |
|---|---|
| `MISSKEY_FRONTEND_DIR` | フロントエンドのルートディレクトリ |
| `MISSKEY_FRONTEND_DIST_DIR` | ビルド済みフロントエンドディレクトリ |
| `MISSKEY_TWEMOJI_DIR` | Twemojiアセットディレクトリ |
| `MISSKEY_CLIENT_ASSETS_DIR` | クライアントアセットディレクトリ |
| `MISSKEY_STATIC_DIR` | 静的ファイルディレクトリ (backend/assets: favicon等) |
| `MISSKEY_REPO_ASSETS_DIR` | リポジトリ直下アセット (ai.png, banner等) |

## テスト用環境変数

CIでのテスト実行時に使用。ローカルではtestcontainersが自動起動するため通常不要。

| 環境変数 | 用途 |
|---|---|
| `TEST_DB_HOST` | テスト用PostgreSQLホスト |
| `TEST_DB_PORT` | テスト用PostgreSQLポート |
| `TEST_DB_NAME` | テスト用データベース名 |
| `TEST_DB_USER` | テスト用ユーザー名 |
| `TEST_DB_PASS` | テスト用パスワード |
| `TEST_DB_SSLMODE` | テスト用SSLモード |
| `TEST_REDIS_HOST` | テスト用Redisホスト |
| `TEST_REDIS_PORT` | テスト用Redisポート |

## マイグレーション用環境変数

| 環境変数 | 用途 |
|---|---|
| `DATABASE_URL` | `make migrate-up/down`で使用するPostgreSQL接続文字列 |

## 設定例

### 最小構成

```yaml
url: http://localhost:3000
port: 3000
db:
  host: localhost
  port: 5432
  db: misskey
  user: misskey
  pass: misskey
redis:
  host: localhost
  port: 6379
id: aidx
```

### 本番構成例

```yaml
url: https://misskey.example.com
port: 3000
db:
  host: db.internal
  port: 5432
  db: misskey
  user: misskey
  pass: strong-password
  extra:
    ssl: true
redis:
  host: redis.internal
  port: 6379
  pass: redis-password
redisForJobQueue:
  host: redis-jobs.internal
  port: 6379
  pass: redis-password
  db: 1
meilisearch:
  host: search.internal
  port: 7700
  apiKey: your-api-key
  index: misskey
id: aidx
maxFileSize: 524288000
```
