package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenTestDB connects to the test PostgreSQL database using settings from
// .env.test. テスト環境の接続情報を環境変数またはファイルから読み取る。
// 環境変数 TEST_DB_HOST, TEST_DB_PORT, TEST_DB_NAME, TEST_DB_USER,
// TEST_DB_PASS, TEST_DB_SSLMODE が設定されていればそちらを優先する。
func OpenTestDB() (*gorm.DB, error) {
	loadEnvFile()

	host := envOrDefault("TEST_DB_HOST", "localhost")
	port := envOrDefault("TEST_DB_PORT", "5432")
	name := envOrDefault("TEST_DB_NAME", "misskey_test")
	user := envOrDefault("TEST_DB_USER", "mk")
	pass := envOrDefault("TEST_DB_PASS", "mk")
	sslmode := envOrDefault("TEST_DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, name, sslmode)

	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

// MustOpenTestDB は OpenTestDB のパニック版。init() で使う。
func MustOpenTestDB() *gorm.DB {
	db, err := OpenTestDB()
	if err != nil {
		panic("failed to open test DB: " + err.Error())
	}
	return db
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadEnvFile loads .env.test from the project root (best-effort).
func loadEnvFile() {
	// プロジェクトルートの .env.test を探す
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	// testutil/ → internal/ → project root
	root := filepath.Join(filepath.Dir(file), "..", "..")
	envPath := filepath.Join(root, ".env.test")

	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// 既に環境変数が設定されていればスキップ
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
