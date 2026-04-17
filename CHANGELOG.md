# Changelog

## 互換バージョン: Misskey 2026.3.2

### Phase P3 — 補助エンドポイント + 欠損テーブル

- 欠損テーブル7種追加 (channel_favorite, clip_favorite, retention_aggregation等)
- 補助エンドポイント12+追加 (roles/notes, hashtags, gallery等)

### Phase P2 — 互換性修正 (#107サブissue)

- P2.1: パスワードリセット、MiAuth gen-token、App API、サインイン履歴
- P2.2: タイムラインフィルタリング、signin-flow captcha
- P2.3: NoteEntity/UserDetailedレスポンス完全化
- P2.4: AP MFM→HTML変換、attachment、context拡充

### Phase P1 — 第2次互換性修正 (#124サブissue)

- WebSocket 9チャンネル追加 (100%カバー)
- AP Inbox Block/Flag/Move/Add/Remove
- AP Accept完全実装 + Question(投票)受信
- users/lists update + update-membership
- trustProxyサポート
- エラーレスポンスUUID統一
- DBスキーマ欠損カラム追加
- WebSocketプロトコル改善 (OAuth2スコープ, pong応答)
- dbSlaves (リードレプリカ)サポート
- chart tables, social auth, AP拡張, Sentry

### Phase 11 — E2E + テストモード

- Cypress E2Eテスト基盤
- `/api/reset-db`テスト用エンドポイント

### Phase 10 — 管理機能

- admin/* 全エンドポイント実装

### Phase 9 — 認証 + ActivityPub

- 9.1: TOTP 2要素認証
- 9.2: リモートActivityPubオブジェクト解決

### Phase 1-8 — 基盤構築

- HTTPサーバー、DB/Redis接続、設定ローダー
- ユーザー、ノート、タイムライン、ドライブ
- フォロー、リアクション、通知
- WebSocketストリーミング
- ActivityPub送信/受信
- ジョブキュー (asynq)
- 検索 (Meilisearch/SQLフォールバック)
