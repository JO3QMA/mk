package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// seedUser はFK制約を満たすためにuserテーブルへ最小限のレコードを投入する。
// 本ファイルのtest helperはtest DBを共有するリポジトリテスト専用で、
// 必ずt.Cleanupで該当行を削除する。
func seedUser(t *testing.T, userID string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO "user" (id, "updatedAt", username, "usernameLower", token) VALUES (?, NOW(), ?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		userID, "u_"+userID, "u_"+userID, "tok_"+userID,
	).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM "user" WHERE id = ?`, userID)
	})
}

// seedChannel はchannelテーブルへ最小限のレコードを投入する。userが先に存在する必要があるため
// seedUserを事前に呼ぶこと。t.Cleanupで後片付け。
func seedChannel(t *testing.T, channelID, ownerUserID string) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`INSERT INTO "channel" (id, "userId", name) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		channelID, ownerUserID, "ch_"+channelID,
	).Error)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM "channel" WHERE id = ?`, channelID)
	})
}
