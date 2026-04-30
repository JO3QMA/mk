# Queue bench (#563)

ジョブキュー配送スループットを **3 driver** (Misskey TS BullMQ / mk-go asynq / mk-go mkq) で公正比較するベンチマーク基盤。HTTP latency 用 `tests/bench/` とは別運用。

## 計測対象

### Outbound (deliver job throughput)

各 stack の local user → `blackhole` 受信機への AP deliver job スループット。

- 各 stack に **dummy follower 100 名** (host=`blackhole`, ユニーク inbox URL) を pre-seed
- driver が `notes/create` (visibility=public) を 100 回叩く → fan-out で **10,000 deliver job** が enqueue
- `blackhole` は POST を全て 204 で即返すので receiver overhead 無し
- 計測: post 開始から blackhole hit が expected 数に達するまでの drain 時間 / RPS

### Inbound (inbox job throughput)

`faker` (Go HTTPS, AP HTTP signature 直接) → 3 receiver inbox に signed Create を blast。

- 各 receiver に faker actor を pubkey 込みで pre-seed (外部 fetch 排除)
- `faker` は **pre-sign 並列化** で sender 側を律速から外し、receiver の verify+enqueue が bottleneck になるよう設計
- 計測: faker → receiver の RPS (= 受信→200 までの rate) と post-send drain time

## 実行

```bash
# 1) 3 stack + blackhole + faker を up (5-10 min、初回 build を伴う)
make queue-bench-up

# 2) seed (user / follower / faker actor を DB 直挿入)
#    meta.federation='all' を強制設定 → app コンテナを restart
make queue-bench-seed

# 3) outbound 計測
make queue-bench-outbound

# 4) inbound 計測
make queue-bench-inbound

# 5) report 生成 (tests/queue-bench/results/queue-report.md)
make queue-bench-report

# まとめて: queue-bench-all (seed → outbound → inbound → report)
make queue-bench-all

# cleanup
make queue-bench-down
```

## 結果ファイル

`tests/queue-bench/results/`:

- `outbound.json` — 生データ (per-stack drain time / hits / depth time series)
- `inbound.json` — faker.send 統計 + per-receiver drain
- `queue-report.md` — markdown 比較表

## 設計メモ

### なぜ TS sender でなく faker?

inbound bench で TS instance を sender に使うと、TS 側の deliver throughput が上限になり、receiver inbox 性能を測れない。faker は Misskey 実装非依存の Go HTTPS server で:

- 固定 RSA-2048 keypair + 固定 actor URI (`https://faker/users/bench-sender`)
- HTTP signature 計算は **bench 開始前に並列で pre-sign**
- send phase は単純な HTTP POST blast で sender 側がボトルネックにならない

### 計測限界

- inbound で receiver workers が即処理する場合、queue depth は polling 粒度 (50ms) では peak ≈ 0 に見える。実効 throughput は faker.send rps が真の receiver ingest+process rate
- outbound は blackhole hits (delivered count) が primary signal なので polling race の影響を受けない

### Federation flag 注意

mk-go は新規 DB 初期化時 `meta.federation='none'` (= 連合無効) で立ち上がる。seed が DB 直接 UPDATE で `federation='all'` にしたあと、app の meta cache (5min TTL) を再読み込みさせるため `make queue-bench-seed` の最後で `app-asynq` / `app-mkq` / `app-ts` を restart する。

### Network allowlist

mk-go の SSRF 防止 (`allowedPrivateNetworks`) は production default で private IP を block する。bench 内の `blackhole` / faker / 他 stack は Docker network の private IP なので、bench config (`tests/queue-bench/common/mk-{asynq,mkq}.yml`) で `127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` を allowlist 化している。

## 関連

- #560 (HTTP bench 結果トラッキング)
- #561 (HTTP bench nginx fix)
- #562 (HTTP bench rate limit fix)
- #413 (パフォーマンス改善ロードマップ)
