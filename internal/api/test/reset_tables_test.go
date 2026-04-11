package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// listUserTables が常に失敗する状態で resetTables を呼ぶと、リトライ上限 (3 回)
// まで lastErr を更新し続け、最終的に非 nil のエラーを返す。この経路 (line 91-92 の
// `lastErr = err; continue`) は既存テストで未カバーだった。
func TestResetTables_ListTablesErrorPath(t *testing.T) {
	requireContainers(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // listUserTables がすぐに ctx.Err を返す

	err := resetTables(ctx, testPg.DB)
	assert.Error(t, err)
}
