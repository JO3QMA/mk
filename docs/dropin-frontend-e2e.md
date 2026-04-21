# Drop-in frontend e2e テスト (#380, Phase 14-)

Misskey TS の実フロントエンドが期待する挙動を cypress で固定し、TS-A backend
を mk-go に差し替えた時も同じ挙動が得られるかを検証する e2e 基盤。

## 目的

- `pytest` ベースの Phase 13 e2e (`tests/dropin/`) は API レベルでの state
  preservation を確認するが、フロントエンド描画層のバグ (画像表示 / 削除反映 /
  emoji 描画 等) までは捕捉しにくい。
- cypress で実ブラウザ (electron) から TS フロントエンドを動かし、観測可能な
  挙動を regression test として固定する。
- 親 issue: #380、サブ: #381 (Phase 14-1、本ドキュメントの対象), #382〜 (Phase 14-2 以降)。

## 構成

```
  TS-C ─┐
        ├─ mutual follow + activities ─┐
  TS-B ─┤                              │
        └────────────────────┬─────────┤
                             ▼         ▼
                            TS-A → [swap, Phase 14-3] → mk-A
```

3 インスタンス (A / B / C) + 各々独立の Postgres / Redis / nginx + cypress runner。
Phase 14-1 では 3 台とも TS のまま (baseline)。mk 差し替え overlay は Phase 14-3。

```
tests/dropin_frontend/
  gen-certs.sh             # a / b / c + bundle.pem 用自己署名証明書
  instance_a.yml           # TS 用 default.yml (A)
  instance_b.yml
  instance_c.yml
  nginx_a.conf             # SSL 前段 (alias: a)
  nginx_b.conf
  nginx_c.conf
  cypress/
    cypress.config.ts      # electron self-signed cert 許容
    tsconfig.json
    package.json
    support/
      e2e.ts               # uncaught 抑制
      api.ts               # cy.request wrapper (createRootOrSignin, retryUntil 等)
    e2e/
      smoke.cy.ts          # baseline spec (Phase 14-1)
  run-frontend-baseline.sh # bash orchestrator

docker-compose.dropin-frontend.yml
```

## 実行

```bash
# baseline (3 TS のみ) 1 回走らせて cypress spec が全 pass することを確認
make dropin-frontend-baseline

# 手動で stack だけ上げて中に入りたいとき
make dropin-frontend-up
docker compose -f docker-compose.dropin-frontend.yml --profile test run --rm cypress-runner
make dropin-frontend-down

# ログ追跡
make dropin-frontend-logs
```

## Phase 進捗

- [x] Phase 14-1 (#381): 3 TS 基盤 + cypress smoke spec
- [x] Phase 14-2 (#387): spec マトリクス拡充 (visibility / userList / cross-instance / delete)
- [ ] Phase 14-3: mk-go 差し替え overlay + baseline / swap 両モード orchestrator + CI nightly

## Phase 14-2 カバー spec

| spec | 内容 | 状態 |
|------|------|------|
| `smoke.cy.ts` | ping / webfinger / A follows B / C note → A | pass |
| `visibility.cy.ts` | public / home / followers / specified (DM) 4 種 | pass |
| `user_list.cy.ts` | alice が list 作成 + bob 追加 + list timeline 取得 | pass |
| `cross_instance_view.cy.ts` | charlie note を A/B 両方から observe して一致確認 | pass |
| `delete_note.cy.ts` | charlie 削除 → A/B timeline から消える | pass |
| `reply_chain.cy.ts` | charlie → bob reply の 2 hop | skip (#389 で調整後 activate) |

attachment / emoji / reaction は Phase 14-2.5 以降 (admin emoji 投入フロー / 
remote image ingest (#378) / reaction deliver (#369) の条件整備が必要)。

## トラブルシューティング

### cypress が self-signed cert を拒否する

`cypress.config.ts` で `setupNodeEvents` 内から `--ignore-certificate-errors`
を electron に渡している。spec 側は何もしなくて良い。

### `misskey/misskey:2025.2.1` の pull 失敗

`docker login` 不要。network / rate limit の可能性。`make dropin-frontend-up`
前に `docker pull misskey/misskey:2025.2.1` で先読みする手もある。

### cypress runner のログが流れない

`--rm` で使い捨て起動なので終了時にログが消える。persistent に確認したい時は
`--profile test run cypress-runner` (without `--rm`) で起動したあと
`docker logs <name>` で追う。

### 3 インスタンスが互いに到達できない

docker compose の `networks: [dropin_frontend]` で共有しているため通常は
自動解決する。`a` / `b` / `c` という alias で nginx が待ち受けるので、
federation URL は `https://a/`, `https://b/`, `https://c/` を使う。
