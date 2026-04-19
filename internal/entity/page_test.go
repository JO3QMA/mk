package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPackPage_Basic(t *testing.T) {
	idGen := newTestIDGen(t)
	pageID := idGen.Generate(time.Now())
	summary := "a summary"
	imgID := "img_1"
	updated := time.Date(2026, 4, 10, 1, 2, 3, 0, time.UTC)

	p := &model.Page{
		ID:                  pageID,
		UpdatedAt:           updated,
		Title:               "Hello",
		Name:                "hello",
		Summary:             &summary,
		AlignCenter:         true,
		HideTitleWhenPinned: false,
		Font:                "sans-serif",
		UserID:              "u1",
		EyeCatchingImageID:  &imgID,
		Content:             []byte(`[{"x":1}]`),
		Variables:           []byte(`[]`),
		Script:              "",
		Visibility:          model.PageVisibilityPublic,
		LikedCount:          3,
	}

	out := PackPage(p, idGen)
	assert.Equal(t, pageID, out["id"])
	assert.Equal(t, "Hello", out["title"])
	assert.Equal(t, "hello", out["name"])
	assert.Equal(t, &summary, out["summary"])
	assert.Equal(t, true, out["alignCenter"])
	assert.Equal(t, false, out["hideTitleWhenPinned"])
	assert.Equal(t, "sans-serif", out["font"])
	assert.Equal(t, "u1", out["userId"])
	assert.Equal(t, &imgID, out["eyeCatchingImageId"])
	assert.Equal(t, json.RawMessage(`[{"x":1}]`), out["content"])
	assert.Equal(t, json.RawMessage(`[]`), out["variables"])
	assert.Equal(t, "public", out["visibility"])
	assert.Equal(t, 3, out["likedCount"])
	assert.Equal(t, "2026-04-10T01:02:03.000Z", out["updatedAt"])
	// createdAtはaidx-IDから復元されるためkey自体が存在することだけ確認。
	_, ok := out["createdAt"]
	assert.True(t, ok)
}

func TestPackPage_NilInput(t *testing.T) {
	assert.Nil(t, PackPage(nil))
}

func TestPackPage_WithoutIDGen_OmitsCreatedAt(t *testing.T) {
	p := &model.Page{ID: "bogus-id", Visibility: model.PageVisibilityPublic}
	out := PackPage(p)
	_, ok := out["createdAt"]
	assert.False(t, ok)
}

func TestPackPage_InvalidIDOmitsCreatedAt(t *testing.T) {
	idGen := newTestIDGen(t)
	p := &model.Page{ID: "not-a-valid-aidx", Visibility: model.PageVisibilityPublic}
	out := PackPage(p, idGen)
	_, ok := out["createdAt"]
	assert.False(t, ok)
}

func TestRawJSONBytes(t *testing.T) {
	assert.Nil(t, rawJSONBytes(nil))
	assert.Nil(t, rawJSONBytes([]byte{}))
	assert.Equal(t, json.RawMessage(`[1]`), rawJSONBytes([]byte(`[1]`)))
}
