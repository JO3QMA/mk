package repository

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetaRepository_Fetch(t *testing.T) {
	repo := NewMetaRepository(testDB)

	name := "Test Instance"
	meta := &model.Meta{
		ID:   "m_f_1",
		Name: &name,
	}
	require.NoError(t, testDB.Create(meta).Error)
	defer testDB.Exec(`DELETE FROM "meta" WHERE id = ?`, meta.ID)

	found, err := repo.Fetch()
	require.NoError(t, err)
	assert.Equal(t, &name, found.Name)
}

func TestMetaRepository_Fetch_NotFound(t *testing.T) {
	repo := NewMetaRepository(testDB)

	// テーブルを空にする
	testDB.Exec(`DELETE FROM "meta"`)

	_, err := repo.Fetch()
	assert.Error(t, err)
}

func TestMetaRepository_Update(t *testing.T) {
	repo := NewMetaRepository(testDB)

	meta := &model.Meta{ID: "m_u_1"}
	require.NoError(t, testDB.Create(meta).Error)
	defer testDB.Exec(`DELETE FROM "meta" WHERE id = ?`, meta.ID)

	newName := "Updated"
	require.NoError(t, repo.Update(map[string]any{"name": newName}))

	found, err := repo.Fetch()
	require.NoError(t, err)
	assert.Equal(t, &newName, found.Name)
}

func TestMetaRepository_EnsureInitial_Creates(t *testing.T) {
	repo := NewMetaRepository(testDB)

	// 空の状態から初期行を作成する。
	testDB.Exec(`DELETE FROM "meta"`)

	require.NoError(t, repo.EnsureInitial("m_ei_1"))
	defer testDB.Exec(`DELETE FROM "meta" WHERE id = ?`, "m_ei_1")

	got, err := repo.Fetch()
	require.NoError(t, err)
	assert.Equal(t, "m_ei_1", got.ID)
}

func TestMetaRepository_EnsureInitial_NoopWhenExists(t *testing.T) {
	repo := NewMetaRepository(testDB)

	testDB.Exec(`DELETE FROM "meta"`)
	existing := &model.Meta{ID: "m_ei_2"}
	require.NoError(t, testDB.Create(existing).Error)
	defer testDB.Exec(`DELETE FROM "meta" WHERE id = ?`, existing.ID)

	// 既存行がある場合は何もせず、別 ID を渡しても既存のまま。
	require.NoError(t, repo.EnsureInitial("m_ei_other"))

	got, err := repo.Fetch()
	require.NoError(t, err)
	assert.Equal(t, "m_ei_2", got.ID)
}
