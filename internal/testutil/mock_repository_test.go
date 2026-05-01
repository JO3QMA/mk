package testutil

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applyUserFieldsはmock内部関数だが、productionのresolver/move双方が
// 渡す型(string/*string, time.Time/*time.Time)を両方正しく反映する
// ことをissue #357の回帰防止として検証する。
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

// MockMetaRepository.Update は real repo の挙動に近づけるため、varchar[]
// 列に nil / 非 string element 含む []any / 不明型を渡すと error を返す
// (#590 review #4)。これにより handler 側の coerce 漏れが mock テストでも
// 検出できるようになり、real DB を使わない handler テストでも regression
// detection が効く。
func TestMockMeta_Update_RejectsInvalidArrayValue(t *testing.T) {
	t.Run("nil for varchar[] column errors", func(t *testing.T) {
		repo := NewMockMetaRepository()
		err := repo.Update(map[string]any{"blockedHosts": nil})
		require.Error(t, err)
	})
	t.Run("non-string element in []any errors", func(t *testing.T) {
		repo := NewMockMetaRepository()
		err := repo.Update(map[string]any{"blockedHosts": []any{"ok.example", 42}})
		require.Error(t, err)
	})
	t.Run("unsupported type errors", func(t *testing.T) {
		repo := NewMockMetaRepository()
		err := repo.Update(map[string]any{"blockedHosts": "not-an-array"})
		require.Error(t, err)
	})
	t.Run("pq.StringArray succeeds", func(t *testing.T) {
		repo := NewMockMetaRepository()
		err := repo.Update(map[string]any{"blockedHosts": []string{"ok.example"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"ok.example"}, []string(repo.Meta.BlockedHosts))
	})
	t.Run("[]any of strings succeeds", func(t *testing.T) {
		repo := NewMockMetaRepository()
		err := repo.Update(map[string]any{"blockedHosts": []any{"ok.example"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"ok.example"}, []string(repo.Meta.BlockedHosts))
	})
}
