package entity

import (
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPackAnnouncement_Basic(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	aid := idGen.Generate(time.Now())
	updated := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	userID := "u1"
	a := &model.Announcement{
		ID:                     aid,
		UpdatedAt:              &updated,
		Title:                  "Hello",
		Text:                   "World",
		Icon:                   "info",
		Display:                "normal",
		NeedConfirmationToRead: true,
		Silence:                false,
		UserID:                 &userID,
	}
	out := PackAnnouncement(a, idGen, true)
	assert.Equal(t, aid, out["id"])
	assert.Equal(t, "Hello", out["title"])
	assert.Equal(t, "World", out["text"])
	assert.Equal(t, "info", out["icon"])
	assert.Equal(t, "normal", out["display"])
	assert.Equal(t, true, out["needConfirmationToRead"])
	assert.Equal(t, false, out["silence"])
	assert.Equal(t, true, out["forYou"])
	assert.Equal(t, true, out["isRead"])
	// updatedAt はフォーマット済み文字列 (ポインタ)。
	updatedStr, ok := out["updatedAt"].(*string)
	assert.True(t, ok)
	assert.Equal(t, "2026-04-19T12:00:00.000Z", *updatedStr)
	// createdAt は aidx 由来で非空。
	assert.NotEmpty(t, out["createdAt"])
}

func TestPackAnnouncement_NilInput(t *testing.T) {
	assert.Nil(t, PackAnnouncement(nil, nil, false))
}

func TestPackAnnouncement_NoUserID_ForYouFalse(t *testing.T) {
	a := &model.Announcement{ID: "x", Title: "G", Text: "global"}
	out := PackAnnouncement(a, nil, false)
	assert.Equal(t, false, out["forYou"])
	assert.Nil(t, out["updatedAt"])
	assert.Equal(t, "", out["createdAt"])
}
