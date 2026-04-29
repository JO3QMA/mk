# dbgtimeline

タイムライン経路の JSON encoder panic を切り分けるためのデバッグ用ツール。
本番には焼かない (`go build ./cmd/dbgtimeline` 経由で開発者がローカルで使う想定)。

## 何が出来るか

1. mk-go と同じ DSN で PostgreSQL に接続して **read-only** で最近の note を取得
2. `entity.PackNotes` + `NoteFieldResolver.Apply` を実 timeline 経路と同じ手順で適用
3. 各 packed `NoteEntity` を goccy/go-json `Encoder.Encode` に流して panic / fatal を recover
4. `-bisect` で panic 範囲を二分探索、`-dump` で原因 entity を stdlib で pretty-print

DB は `SELECT` のみ。書き込み一切なし。production-like UDS DB に向けても安全。

## 由来

[#542](https://github.com/shiroha-a/mk/issues/542) (goccy 0.10.6 の `ptrToString` panic) を
追跡するために作った。トリガー note を特定したあと、汎用デバッグ ツールとして残してある。
将来「特定の timeline で encoder panic が出る」報告が来たら、同じ手順で切り分けられる。

## 使い方

### ローカルビルド

```bash
go build -o /tmp/dbgtimeline ./cmd/dbgtimeline
/tmp/dbgtimeline -config .config/default.yml -limit 200 -viewer your-username
```

ローカルで mk-go を立てている場合は `-config` を実 config に向ければそのまま使える。

### Docker 内で実行 (UDS 環境向け)

UDS 環境はホストから DB 直接アクセスできないので、`mkgo` コンテナ内に
バイナリを `docker cp` してから実行する:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -o /tmp/dbgtimeline ./cmd/dbgtimeline
docker cp /tmp/dbgtimeline mk-mkgo-1:/tmp/dbgtimeline
docker compose -f compose.uds.yaml exec mkgo \
  /tmp/dbgtimeline -limit 500 -viewer your-username -bisect -dump
```

### フラグ

| フラグ | デフォルト | 説明 |
|---|---|---|
| `-config` | `/app/.config/default.yml` | mk-go config の path (DB credentials を取る) |
| `-limit` | `200` | 走査する note の最大件数 (`ORDER BY id DESC LIMIT N`) |
| `-viewer` | `""` | viewer-dependent field (MyReaction 等) を埋めるための `username_lower`。空 = nil viewer (production の未ログイン timeline と同等) |
| `-bisect` | `false` | panic を起こす note の slice 範囲を二分探索する |
| `-dump` | `false` | 最初に panic を起こした note の packed entity を stdlib JSON で deep-dump する |

## 出力例

```
Loaded 200 notes
Using viewer: alice (id=al9hjjx4b0ey0002)
PANIC at index=1 note.id=alng2ov5ffpp0007 userHost=remote.example visibility=public renoteId=0xc0003c58e0
PANIC at index=85 note.id=alm00w90sjj8000w userHost=remote.example visibility=public renoteId=0xc000463e90
...
Total panicking notes: 5

--- Slice bisect (find smallest panicking range) ---
[bisect lo=0 mid=100 hi=200] left err=panic / right err=<nil>
[bisect lo=0 mid=50 hi=100] left err=panic / right err=<nil>
...
[single index=1] err=panic id=alng2ov5ffpp0007
```

## 既知の制約

- **fatal error は recover できない**: goccy が出す `runtime.fatal` (sigsegv) は `defer recover()`
  では掴めず、プロセスが死ぬ。複数 note 連続で encode を試みると goccy 内部 cache が
  汚染されて後続呼び出しで fatal に昇格することがある。`-bisect` 中にプロセスが落ちたら、
  `-limit` を絞って再実行する。
- **synthetic な reproducer は再現性が低い**: 手で同じ shape の `entity.NoteEntity` を組み立てても
  panic しないケースがある (#542 調査結果)。実 DB の packed entity を経由する必要がある。

## 削除条件

`internal/server/json_serializer.go` が goccy 以外 (例: jsoniter / encoding/json 永続) に
切り替わって、本ツールが追う対象 (= goccy 静的コンパイルバグ) が消えたら削除して良い。
それまでは似た encoder バグの triage に再利用できる。
