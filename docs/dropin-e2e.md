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

- [x] Phase 13-1 (#365): TS ↔ TS 基盤 + smoke test
- [x] Phase 13-2 (#367): mk-go 差し替え overlay + swap シナリオ test
- [ ] Phase 13-3: 機能マトリクス (ノート / リアクション / カスタム絵文字 / タイムライン退行検出 / 通知)
- [ ] Phase 13-4: CI nightly 統合

## Phase 13-2: mk-go 差し替え (drop-in swap)

`docker-compose.dropin.mk.yml` overlay と bash orchestrator
(`tests/dropin/run-swap-test.sh`) で「TS-A backend を mk-go に差し替えても DB /
Redis 上の state がそのまま引き継がれる」ことを e2e で検証する。

### 通常実行

```bash
# 完全自動の swap シナリオ test (推奨)
make dropin-swap-test

# orchestrator は以下を順次実行する:
#   1. TS-A + TS-B stack 起動
#   2. pytest test_swap_setup.py    (alice/bob/follow/baseline note)
#   3. docker compose stop app-a    (TS-A backend 停止、DB / Redis は維持)
#   4. overlay で app-a を mk-go ビルドに差し替えて起動
#   5. pytest test_swap_verify.py   (timeline 残存、新規 reply / reaction の連合)
#   6. teardown
```

### 手動運用 (デバッグ向け)

mk overlay を直接立ち上げて確認したい場合:

```bash
make dropin-mk-up      # base + overlay (= mk-A + TS-B)
make dropin-mk-test    # smoke test を mk-A に対して実行
make dropin-mk-down    # cleanup
make dropin-mk-logs    # ログ追跡
```

注意: `dropin-mk-up` は **clean DB** から mk-A を起動するので、TS-A→mk-A の
state 引き継ぎは検証されない。state 検証は `dropin-swap-test` 専用。

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
