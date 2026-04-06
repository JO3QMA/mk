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
