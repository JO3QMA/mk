package repository

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/shiroha-a/mk/internal/testutil"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	tdb, err := testutil.SetupPostgres(ctx)
	if err != nil {
		log.Fatalf("failed to setup postgres: %v", err)
	}
	testDB = tdb.DB

	code := m.Run()

	tdb.Teardown(ctx)
	os.Exit(code)
}
