package i

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestRegistryGetDetail(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.RegistryGetDetail, `{}`, stubUser).Code)
}

func TestRegistryKeys(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.RegistryKeys, `{}`, stubUser).Code)
}

func TestRegistryScopesWithDomain(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.RegistryScopesWithDomain, `{}`, stubUser).Code)
}

// --- P4-6 (#166): i/registry/* ---

func TestRegistryGetDetail_Found(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	require.NoError(t, reg.Set(&model.RegistryItem{
		ID: "r1", UserID: stubUser.ID, Key: "theme", Value: datatypes.JSON(`"dark"`),
	}))
	h.SetRegistryRepo(reg)
	rec := postExtra(h.RegistryGetDetail, `{"key":"theme"}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegistryGetDetail_NotFound(t *testing.T) {
	h, _ := newExtraHandler(t)
	h.SetRegistryRepo(testutil.NewMockRegistryRepository())
	rec := postExtra(h.RegistryGetDetail, `{"key":"ghost"}`, stubUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// 本家互換: JSON で scope を省略した場合でも、default の empty-scope アイテムに
// マッチすること (nil スライスが SQL NULL に化けると常に miss する bug の
// regression test — Devin review #175)。
func TestRegistryGetDetail_OmittedScopeFindsDefaultItem(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	// Scope を明示せず作成 (Set は item.Scope をそのまま使う)。デフォルト
	// の空スライス状態で保存することを mock で再現する。
	require.NoError(t, reg.Set(&model.RegistryItem{
		ID: "r1", UserID: stubUser.ID, Key: "theme", Value: datatypes.JSON(`"dark"`),
		Scope: []string{},
	}))
	h.SetRegistryRepo(reg)
	// scope 無しリクエスト
	rec := postExtra(h.RegistryGetDetail, `{"key":"theme"}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegistryKeys_OmittedScopeFindsDefaultItem(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	require.NoError(t, reg.Set(&model.RegistryItem{
		ID: "r1", UserID: stubUser.ID, Key: "alpha", Value: datatypes.JSON(`"x"`),
		Scope: []string{},
	}))
	h.SetRegistryRepo(reg)
	rec := postExtra(h.RegistryKeys, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Contains(t, got, "alpha")
}

func TestRegistryKeys_ReturnsKeysArray(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r1", UserID: stubUser.ID, Key: "a", Value: datatypes.JSON(`"x"`)}))
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r2", UserID: stubUser.ID, Key: "b", Value: datatypes.JSON(`"y"`)}))
	h.SetRegistryRepo(reg)
	rec := postExtra(h.RegistryKeys, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestRegistryScopesWithDomain_ReturnsDistinctPairs(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r1", UserID: stubUser.ID, Key: "a", Value: datatypes.JSON(`"x"`), Scope: []string{"client"}}))
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r2", UserID: stubUser.ID, Key: "b", Value: datatypes.JSON(`"y"`), Scope: []string{"client"}}))
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r3", UserID: stubUser.ID, Key: "c", Value: datatypes.JSON(`"z"`), Scope: []string{"default"}}))
	h.SetRegistryRepo(reg)
	rec := postExtra(h.RegistryScopesWithDomain, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	// 2 distinct scopes, domain は nil のみ
	assert.Len(t, got, 2)
}
