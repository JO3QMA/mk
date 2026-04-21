package testutil

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applyUserFieldsは mock 内部関数だが、production の resolver / move 双方が
// 渡す 型 (string / *string, time.Time / *time.Time) を両方正しく反映する
// ことを issue #357 の回帰防止として検証する。
func TestApplyUserFields_MovedAndAlsoKnownAs_BothForms(t *testing.T) {
	repo := NewMockUserRepository()
	const uid = "u_applytest"
	require.NoError(t, repo.Create(&model.User{ID: uid, Username: "alice"}))

	// resolver.refreshActor が渡す *string / *time.Time 形式
	movedToPtr := "https://remote.example/users/new"
	nowPtr := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	akaPtr := "https://remote.example/users/aka"
	require.NoError(t, repo.UpdateUser(uid, map[string]any{
		"movedToUri":  &movedToPtr,
		"movedAt":     &nowPtr,
		"alsoKnownAs": &akaPtr,
	}))
	got, err := repo.FindByID(uid)
	require.NoError(t, err)
	require.NotNil(t, got.MovedToURI)
	assert.Equal(t, movedToPtr, *got.MovedToURI)
	require.NotNil(t, got.MovedAt)
	assert.Equal(t, nowPtr, *got.MovedAt)
	require.NotNil(t, got.AlsoKnownAs)
	assert.Equal(t, akaPtr, *got.AlsoKnownAs)

	// core/move が渡す string / time.Time (非ポインタ) 形式
	movedTo2 := "https://remote.example/users/newer"
	now2 := time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)
	aka2 := "https://remote.example/users/aka2"
	require.NoError(t, repo.UpdateUser(uid, map[string]any{
		"movedToUri":  movedTo2,
		"movedAt":     now2,
		"alsoKnownAs": aka2,
	}))
	got, err = repo.FindByID(uid)
	require.NoError(t, err)
	require.NotNil(t, got.MovedToURI)
	assert.Equal(t, movedTo2, *got.MovedToURI)
	require.NotNil(t, got.MovedAt)
	assert.Equal(t, now2, *got.MovedAt)
	require.NotNil(t, got.AlsoKnownAs)
	assert.Equal(t, aka2, *got.AlsoKnownAs)
}
