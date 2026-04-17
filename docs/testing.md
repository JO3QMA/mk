# テスト

## テストの種類

| 種類 | 対象 | DB/Redis | 実行方法 |
|---|---|---|---|
| ユニットテスト | APIハンドラ、サービスロジック | モック | `go test ./internal/api/...` |
| 統合テスト | リポジトリ、Redis連携 | 実DB (testcontainers) | `go test ./internal/core/...` |
| E2Eテスト | フロントエンド操作 | 実DB + フロントエンド | `make e2e-run` (詳細は[E2Eテスト](e2e.md)) |
| 連合テスト | mk-go ↔ Misskey AP通信 | Docker Compose多段 | `make federation-misskey-test` |

## 実行方法

```bash
# 全テスト (testcontainersでPostgreSQL/Redisが自動起動、Docker必須)
make test

# 特定パッケージ
go test ./internal/api/notes/...

# レース検出 + カバレッジ (CIと同条件)
go test -race -count=1 -timeout 10m \
  -coverprofile=coverage.out -covermode=atomic ./...

# カバレッジHTMLレポート
go tool cover -html=coverage.out
```

## カバレッジ目標

| レベル | 閾値 | 説明 |
|---|---|---|
| CIゲート (最低ライン) | 90% | これを下回るとCIが失敗しマージ不可 |
| 推奨ライン | 95% | 通常のPRではここを目指す |
| 目標ライン | 100% | 新規パッケージや小規模パッケージで積極的に狙う |
| `internal/api/admin` | 60% | 管理APIのみCIゲートが低め (可能な限り引き上げる) |

CIではパッケージごとにカバレッジを計測し、閾値未達のパッケージがあればジョブが失敗する。

## testcontainers

`internal/testutil/containers.go`がtestcontainers-goでPostgreSQL 16とRedis 7のコンテナを自動起動する。ローカルにDocker環境があれば特別な準備なしでテストを実行できる。

```go
// PostgreSQLコンテナ起動 + マイグレーション自動適用
testDB, err := testutil.SetupPostgres(ctx)
defer testDB.Teardown(ctx)

// Redisコンテナ起動
testRedis, err := testutil.SetupRedis(ctx)
defer testRedis.Teardown(ctx)

// テスト間のデータクリーンアップ
testDB.TruncateAll()
testRedis.FlushAll(ctx)
```

Docker環境がない場合は`testutil.SkipIfNoDocker(t)`でテストをスキップする。

### CI環境

CIではtestcontainersの代わりにGitHub Actionsの`services`でPostgreSQL/Redisを起動し、環境変数で接続先を指定する:

| 環境変数 | 値 |
|---|---|
| `TEST_DB_HOST` | `localhost` |
| `TEST_DB_PORT` | `5432` |
| `TEST_DB_NAME` | `misskey_test` |
| `TEST_DB_USER` | `mk` |
| `TEST_DB_PASS` | `mk` |
| `TEST_DB_SSLMODE` | `disable` |
| `TEST_REDIS_HOST` | `localhost` |
| `TEST_REDIS_PORT` | `6379` |

## テストパターン

### APIハンドラテスト (モック)

```go
func newTestHandler(t *testing.T) (*Handler, *testutil.MockUserRepository) {
    userRepo := testutil.NewMockUserRepository()
    metaRepo := testutil.NewMockMetaRepository()
    metaRepo.Meta = &model.Meta{ID: "x"}
    // モックをサービスに注入
    svc := NewService(userRepo, metaRepo)
    h := NewHandler(svc)
    return h, userRepo
}

func doPost(h func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
    e := echo.New()
    req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    if user != nil {
        c.Set(string(middleware.UserContextKey), user)
    }
    _ = h(c)
    return rec
}
```

### サービステスト (実Redis)

```go
var testRedis *testutil.TestRedis

func TestMain(m *testing.M) {
    ctx := context.Background()
    tr, err := testutil.SetupRedis(ctx)
    if err != nil {
        log.Fatalf("redis setup failed: %v", err)
    }
    testRedis = tr
    code := m.Run()
    testRedis.Teardown(ctx)
    os.Exit(code)
}

func newSvc(t *testing.T) *Service {
    t.Helper()
    testRedis.FlushAll(context.Background())
    return NewService(testutil.NewMockRepository(), testRedis.Client)
}
```

## モック一覧

`internal/testutil/`に以下のモック実装がある:

| ファイル | 内容 |
|---|---|
| `mock_repository.go` | User, Note, Following, Reaction, Meta, Role, Channel, Chat等 (~20種) |
| `mock_drive.go` | DriveFileRepository, DriveFolderRepository |
| `mock_block_mute.go` | BlockingRepository, MutingRepository, RenoteMutingRepository |
| `mock_allowlist.go` | AllowlistChecker (mediaproxy用) |
| `errors.go` | テスト用エラー定数 |

各モックはインメモリの`map[string]*Model`でデータを保持し、CRUD操作をシミュレートする。

## 連合テスト

`docker-compose.federation.misskey.yml`でmk-goとMisskey TSの2インスタンスを起動し、AP通信をテストする。

```bash
# ビルド + 起動
make federation-misskey-up

# テスト実行
make federation-misskey-test

# ログ確認
make federation-misskey-logs

# 停止
make federation-misskey-down
```

テストはPython (pytest)で記述され、`tests/federation/`に配置。両インスタンスに共通のAPI互換クライアント(`MisskeyLikeClient`)を使ってフォロー、ノート作成、リアクション等の連合動作を検証する。
