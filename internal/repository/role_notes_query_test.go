package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RoleNotesQueryはrole_assignment経由でnoteを拾うJOINクエリ。本テストでは
// ざっくりとJOINが成立し、sinceId/untilId分岐が実行されることだけ確かめる。
// データ整合性の厳密なテストは上位のcore/role側に任せる。
func TestRoleNotesQuery_ListByRole(t *testing.T) {
	q := NewRoleNotesQuery(testDB)

	// 空結果でもクエリがエラーにならないこと
	notes, err := q.ListByRole("nonexistent_role", 10, "", "")
	require.NoError(t, err)
	assert.Empty(t, notes)

	// sinceId / untilId 分岐を通す
	notes, err = q.ListByRole("nonexistent_role", 5, "since_dummy", "until_dummy")
	require.NoError(t, err)
	assert.Empty(t, notes)
}
