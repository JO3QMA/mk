package i

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageLikes(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.PageLikes, `{}`, stubUser).Code)
}

// --- P4-6 (#166): i/page-likes ---

func TestPageLikes_WithRepo(t *testing.T) {
	h, _ := newExtraHandler(t)
	pageLike := testutil.NewMockPageLikeRepository()
	require.NoError(t, pageLike.Create(&model.PageLike{ID: "pl1", UserID: stubUser.ID, PageID: "pg1"}))
	h.SetPageLikeRepo(pageLike)
	rec := postExtra(h.PageLikes, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, "pg1", got[0]["pageId"])
}
