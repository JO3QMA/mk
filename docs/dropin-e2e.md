# Drop-in e2e テスト (#364, Phase 13-1〜)

Misskey TS から mk-go への **drop-in 切替互換** を自動検証する e2e テスト基盤。
#362 で露呈した Redis キー名前空間違い / HTML→MFM 変換の取りこぼしのような
互換ギャップを、手動テストに頼らず CI / on-demand で再発検知することを目的とする。

## 構成

```
tests/dropin/
  gen-certs.sh        # 自己署名証明書 (a, b 用)
  instance_a.yml      # Misskey TS 設定 (instance A)
  instance_b.yml      # Misskey TS 設定 (instance B)
  nginx_a.conf        # SSL 前段 (a domain)
  nginx_b.conf        # SSL 前段 (b domain)
  conftest.py         # pytest fixtures (tests/federation/common/ の Client を再利用)
  test_smoke.py       # Phase 13-1 の smoke test

docker-compose.dropin.yml  # TS-A / TS-B stack
```

### なぜ共通 harness が `tests/federation/common/` にあるのか

mk-go は既に `tests/federation/` で mk-go ↔ Misskey TS 連合テストを持っており、
そこに `MisskeyLikeClient` (httpx ベース) が整備済である。drop-in テストは
さらに TS ↔ TS (後続フェーズで TS ↔ mk) に広げるだけなので client は共有する。

## 実行方法

すべて docker compose 経由。ホストに pnpm / Python をインストールしない。

```bash
# 1. インスタンス起動 (初回は misskey/misskey image の pull で数分)
make dropin-up

# 2. smoke test 実行
make dropin-test

# 3. 片付け (volume まで削除)
make dropin-down

# (任意) ログ追跡
make dropin-logs
```

## Phase 進捗

- [x] Phase 13-1 (#365): TS ↔ TS 基盤 + smoke test (本ドキュメント)
- [ ] Phase 13-2: mk-go 差し替え overlay
- [ ] Phase 13-3: 機能マトリクス (ノート / リアクション / カスタム絵文字 / タイムライン退行検出 / 通知)
- [ ] Phase 13-4: CI nightly 統合

## トラブルシューティング

### `misskey/misskey:2025.2.1` が pull できない

`docker login` 等の認証不要。pull が失敗する場合は network / rate limit。
compose 内で pull を再試行するか `docker pull misskey/misskey:2025.2.1` で
先読みしておく。

### 連合 follow が timeout する

両 instance が互いに到達できる必要がある。`make dropin-logs` で nginx / app の
エラーを確認する。自己署名証明書のため app-a / app-b は
`NODE_TLS_REJECT_UNAUTHORIZED=0` で起動している。

### 残ったリソースが後続テストを汚染する

`make dropin-down` が volume まで削除する (`down -v`)。named volume `certs` も
同時に削除されるため、次回 `dropin-up` で再生成される。
