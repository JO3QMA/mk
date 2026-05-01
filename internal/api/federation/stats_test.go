package federation

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStats_EmptyService(t *testing.T) {
	h, _ := newHandler(t)
	assert.Equal(t, http.StatusOK, postStub(h.Stats).Code)
}

func TestStats_WithTopInstances(t *testing.T) {
	h, instRepo := newHandler(t)
	// 3 instance を登録。2 は top に入り、1 は "other" に流れる想定で
	// limit=2 で呼び出す。
	_ = instRepo.Create(&model.Instance{ID: "inst-1", Host: "alpha.example", FollowersCount: 10, FollowingCount: 5})
	_ = instRepo.Create(&model.Instance{ID: "inst-2", Host: "beta.example", FollowersCount: 8, FollowingCount: 20})
	_ = instRepo.Create(&model.Instance{ID: "inst-3", Host: "gamma.example", FollowersCount: 2, FollowingCount: 3})

	rec := postBody(h.Stats, `{"limit":2}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	topSub, ok := body["topSubInstances"].([]any)
	require.True(t, ok)
	assert.Len(t, topSub, 2)
	topPub, ok := body["topPubInstances"].([]any)
	require.True(t, ok)
	assert.Len(t, topPub, 2)
	// topSubInstances は followers 上位 2 件 = 10 + 8 = 18
	// 合計 followers = 20、other = 2
	assert.EqualValues(t, 2, body["otherFollowersCount"])
	// topPubInstances は following 上位 2 件 = 20 + 5 = 25
	// 合計 following = 28、other = 3
	assert.EqualValues(t, 3, body["otherFollowingCount"])
}
