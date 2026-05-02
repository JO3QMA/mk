# API互換性状況

対象バージョン: **Misskey 2026.3.2**
最終更新: 2026-04-17

本ドキュメントは互換性調査 (#107, #124) の結果に基づく。

## エンドポイントカバー率

| カテゴリ | TS | Go | カバー率 |
|---|---|---|---|
| admin/* | ~100 | ~100 | **100%** |
| notes/* | ~30 | ~30 | **100%** |
| users/* | ~28 | ~28 | **100%** |
| i/* | ~45 | ~45 | **100%** |
| drive/* | ~18 | ~18 | **100%** |
| federation/* | 7 | 7 | **100%** |
| following/* | 9 | 9 | **100%** |
| channels/* | 16 | 16 | **100%** |
| clips/* | 10 | 10 | **100%** |
| blocking/* | 3 | 3 | **100%** |
| mute/* | 3 | 3 | **100%** |
| auth/* | 4 | 4 | **100%** |
| app/* | 3 | 3 | **100%** |
| roles/* | 4 | 4 | **100%** |
| chat/* | 13 | 18 | **設計相違** |
| その他 | ~20 | ~20 | **~100%** |

chat/*はTS版とGo版でAPI設計が根本的に異なる。TS版のクライアントがそのまま使う場合、パス名レベルで非互換。

未実装エンドポイントへのリクエストはキャッチオールハンドラが`200 {}`で応答するため、クライアントがクラッシュすることはない。

## 対応済みの互換性修正

### 第1次調査 (#107) サブissue

| # | 内容 | 状態 |
|---|---|---|
| #109 | AP: MFM→HTML変換、attachment、context拡充 | 完了 |
| #110 | NoteEntity/UserDetailed レスポンス完全化 | 完了 |
| #111 | パスワードリセット、MiAuth gen-token、App API | 完了 |
| #112 | Timelineフィルタリング、signin-flow captcha | 完了 |
| #113 | 欠損テーブル7種 + 補助エンドポイント12+ | 完了 |

### 第2次調査 (#124) サブissue

| # | 内容 | 状態 |
|---|---|---|
| #125 | WebSocket 9チャンネル追加 + タイムラインフィルタパラメータ | 完了 |
| #126 | AP Inbox Block/Flag/Move/Add/Remove | 完了 |
| #127 | AP Accept完全実装 + Question(投票)受信 + EmojiReactバリアント | 完了 |
| #128 | users/lists update + update-membership | 完了 |
| #129 | trustProxyサポート | 完了 |
| #130 | エラーレスポンス標準化 (UUID統一) | 完了 |
| #131 | DBスキーマ欠損カラム追加 | 完了 |
| #132 | WebSocketプロトコル改善 (OAuth2スコープ, pong応答) | 完了 |
| #133 | dbSlaves (リードレプリカ)サポート | 完了 |
| #134 | chart tables, social auth, AP拡張, Sentry | 完了 |

## DB構造

### テーブル

Go側のマイグレーション (000001〜000036) はTS版テーブルに対して追加のみで破壊的変更を行わない。TS版のマイグレーションで作成される全テーブルは維持される。

Go版で追加したテーブル:
- `password_reset_request` — パスワードリセット要求
- `signin` — ログイン履歴
- `channel_favorite`, `channel_muting` — チャンネルお気に入り/ミュート
- `clip_favorite`, `user_list_favorite` — クリップ/リストお気に入り
- `retention_aggregation` — リテンション統計
- `system_account` — システムアカウント
- `used_username` — ユーザー名再利用防止
- `note_thread_muting` — スレッドミュート

### 既知のカラム差分

| テーブル | 状況 |
|---|---|
| `user_profile` | followedMessage, lang, publicReactionsは#131で追加済み |
| `note` | appId (App連携識別)、score (ノートスコア) はGo版では未使用 |
| `abuse_user_report` | resolvedAsのサイズ差 (Go: varchar(16), TS: varchar(128)) |

## ActivityPub互換性

### 対応済みActivity

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

### AP Person

MFM→HTML変換、`featured`、`attachment`(プロフィールフィールド)、`tag`(絵文字タグ)、`image`(バナー)は#109で対応済み。

### AP Note

`content`のHTML化、`attachment`(ファイル)、拡張`@context`(Misskey独自vocabulary)は#109で対応済み。Question(投票)オブジェクトの受信は#127で対応済み。

## WebSocket/Streaming

### チャンネルカバー率

全19チャンネル中19チャンネル実装済み (**100%**)。#125で欠損9チャンネルを追加。

プロトコル改善 (#132):
- OAuth2スコープに基づくチャンネルアクセス制御
- `pong`応答の実装
- パラメータバリデーション強化

## エラーレスポンス

#130でエラーレスポンスを標準化:
- 同一エラーコードに対するUUIDを統一
- ヘルパー関数を統合

## 設定ファイル互換性

TS版の`.config/default.yml`をそのまま使用可能。以下の設定もGo版で対応済み:
- 基本接続 (DB, Redis, URL, port)
- trustProxy (#129)
- dbSlaves (#133)
- 各種Redis分離設定

## 既知の制限

- **Identiconの外見** — TS版と生成アルゴリズムが異なるため、アイコン未設定ユーザーの表示が異なる
- **chat/*のAPI設計** — TS版とパス名・パラメータが異なる
- **Reversi 連合の自動 smoke test** — ローカルリアルタイム対局・cross-instance 対戦 (#417 で実装済) は手動検証で動作確認済だが、CI で round-trip を assert する自動 smoke test は #435 で別途追加予定 (実装は完了済)

詳細は[TS版からの移行ガイド](migration-from-ts.md)の「既知の制限」セクションも参照。

## 関連issue

- #107 — 第1次互換性調査 (API/DB/挙動の3軸)
- #124 — 第2次互換性調査 (6軸: +WebSocket, エラー, 設定)
