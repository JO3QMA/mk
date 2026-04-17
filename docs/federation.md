# ActivityPub連合

## パッケージ構成

### `internal/activitypub/` (プロトコル層)

| ファイル | 責務 |
|---|---|
| `types.go` | ActivityStreams 2.0の型定義 (Object, Person, Note, Activity等) |
| `renderer.go` | Goモデル → AP JSON-LD変換 (RenderPerson, RenderNote, RenderFollow等) |
| `signature.go` | HTTP Signatures (RSA-SHA256, Cavage draft v12) の署名・検証 |
| `client.go` | AP HTTPクライアント (署名付きPOST/GET、リダイレクト制御) |
| `jsonld.go` | `@context`構築、JSON-LD正規化 (Mastodonプレフィックス互換) |
| `keypair.go` | RSA 2048bit鍵ペアの生成・PEM解析 |
| `mfm/` | MFM(Misskey Flavored Markdown) → HTML変換 |

### `internal/core/federation/` (ビジネスロジック層)

| ファイル | 責務 |
|---|---|
| `resolver.go` | リモートアクター/ノートの取得・永続化。公開鍵の2層キャッシュ (メモリ+DB, TTL 24h) |
| `deliver_service.go` | 配信ジョブのエンキュー。フォロワーのInbox収集、ホストブロック判定 |
| `processor.go` | 受信Activityのディスパッチ (Follow, Create, Like, Announce, Delete, Update等) |
| `note_delivery_hook.go` | ノート公開時にCreate/Announceを配信 |
| `following_delivery_hook.go` | フォロー/アンフォロー/承認時にFollow/Undo/Acceptを配信 |
| `reaction_delivery_hook.go` | リアクション時にLikeを配信 |
| `note_delete_delivery_hook.go` | ノート削除時にDeleteを配信 |
| `mention_resolver.go` | メンション → AS Mentionタグ変換 |
| `fetcher.go` | 署名なしGETによるリモートオブジェクト取得 |

## HTTP Signatures

Cavage draft v12に準拠。RSA-SHA256で署名する。

**署名対象ヘッダー:**
- `(request-target)`: メソッド + パス
- `date`: RFC 2822形式
- `host`: リクエストホスト
- `digest`: リクエストボディのSHA-256 (POSTのみ)

```
Signature: keyId="https://example.com/users/abc#main-key",
           algorithm="rsa-sha256",
           headers="(request-target) date host digest",
           signature="base64..."
```

すべての送信リクエストに署名を付与する。受信時は`keyId`からアクターの公開鍵を取得して検証する。

## リモートオブジェクト解決

`resolver.go`がリモートのアクターとノートを取得し、ローカルDBに永続化する。

**公開鍵キャッシュ (2層):**
1. インメモリ (`map[userID]publicKeyEntry`, TTL 24h)
2. DB (`user_publickey`テーブル)
3. HTTPフェッチ (キャッシュミス時、署名なしGET)

リモートユーザー作成時にはインスタンス情報の登録とチャートメトリクスの記録も行う。

## 配信パイプライン

```
coreサービス (note/following/reaction)
  ↓ フック呼び出し
DeliveryHook (note_delivery_hook等)
  ↓ Activity構築 + エンキュー
DeliverService
  ↓ asynq タスク投入
Redis (ジョブキュー)
  ↓
DeliverProcessor (asynqワーカー)
  ↓ 署名付きPOST
リモートInbox
```

**レスポンス処理:**
- 2xx-3xx: 成功
- 4xx (501以外): 永続的失敗、リトライしない
- 5xx / ネットワークエラー: リトライ
- 410 Gone: 永続的失敗 (フォロワーリストから削除)

## Inbox処理

`internal/api/inbox/handler.go`がShared InboxとユーザーInboxの両方を処理する。

**処理フロー:**
1. リクエストボディ読み取り
2. Signatureヘッダー解析
3. `keyId`からアクター解決
4. 公開鍵取得 (キャッシュ優先)
5. 署名検証
6. ホストブロックチェック
7. インスタンスメタデータ更新
8. チャートメトリクス記録
9. Processorにディスパッチ

未対応のActivity種別にも202 Acceptedを返す (エラーにしない)。

## レンダリング

`renderer.go`がGoモデルをAP JSON-LDに変換する。

**Person:**
- Type: `Person` (ボットは`Service`)
- アイコン/バナー、公開鍵、プロフィールフィールド (PropertyValue)
- カスタム絵文字タグ

**Note:**
- `content`: MFM → HTML変換
- `_misskey_content`: 元のMFM
- メンション、ファイル添付 (Document)、引用URL
- `sensitive`フラグ

## 対応済みActivity

| Activity | 送信 | 受信 |
|---|---|---|
| Create (Note) | o | o |
| Delete (Note) | o | o |
| Update (Note/Person) | o | o |
| Follow | o | o |
| Accept (Follow) | o | o |
| Reject (Follow) | o | o |
| Undo (Follow/Like/Announce/Block) | o | o |
| Like (Reaction) | o | o |
| Announce (Renote) | o | o |
| Block | o | o |
| Flag (Report) | - | o |
| Move (Account Migration) | - | o |
| Add/Remove (Pin) | - | o |

## ディスカバリ

| エンドポイント | 内容 |
|---|---|
| `GET /.well-known/webfinger` | `acct:user@host`のリソース検索 |
| `GET /.well-known/host-meta` | WebFinger URLテンプレート |
| `GET /.well-known/nodeinfo` | NodeInfo URL |
| `GET /nodeinfo/2.0` | サーバーメタデータ (バージョン、プロトコル、利用統計) |
| `GET /users/:id` | AP Person |
| `GET /notes/:id` | AP Note |
| `GET /users/:id/followers` | フォロワーコレクション |
| `GET /users/:id/following` | フォローコレクション |
| `GET /users/:id/featured` | ピン留めノートコレクション |
