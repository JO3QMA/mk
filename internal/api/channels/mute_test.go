package channels

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- MuteCreate ---

func TestMuteCreate_Success(t *testing.T) {
	h, chRepo, _, mutRepo := newStubHandler(t)
	chRepo.Channels["ch1"] = &model.Channel{ID: "ch1", Name: "test"}
	rec := postStubWithBody(t, h.MuteCreate, `{"channelId":"ch1"}`, "u1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	exists, _ := mutRepo.Exists("u1", "ch1")
	assert.True(t, exists)
}

func TestMuteCreate_MissingChannelID(t *testing.T) {
	h, _, _, _ := newStubHandler(t)
	rec := postStubWithBody(t, h.MuteCreate, `{}`, "u1")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMuteCreate_ChannelNotFound(t *testing.T) {
	h, _, _, _ := newStubHandler(t)
	rec := postStubWithBody(t, h.MuteCreate, `{"channelId":"nonexist"}`, "u1")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMuteCreate_AlreadyMuted(t *testing.T) {
	h, chRepo, _, mutRepo := newStubHandler(t)
	chRepo.Channels["ch1"] = &model.Channel{ID: "ch1", Name: "test"}
	mutRepo.Mutings["u1:ch1"] = &model.ChannelMuting{ID: "m1", UserID: "u1", ChannelID: "ch1"}
	rec := postStubWithBody(t, h.MuteCreate, `{"channelId":"ch1"}`, "u1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// --- MuteDelete ---

func TestMuteDelete_Success(t *testing.T) {
	h, _, _, mutRepo := newStubHandler(t)
	mutRepo.Mutings["u1:ch1"] = &model.ChannelMuting{ID: "m1", UserID: "u1", ChannelID: "ch1"}
	rec := postStubWithBody(t, h.MuteDelete, `{"channelId":"ch1"}`, "u1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	exists, _ := mutRepo.Exists("u1", "ch1")
	assert.False(t, exists)
}

func TestMuteDelete_MissingChannelID(t *testing.T) {
	h, _, _, _ := newStubHandler(t)
	rec := postStubWithBody(t, h.MuteDelete, `{}`, "u1")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- MuteList ---

func TestMuteList_Empty(t *testing.T) {
	h, _, _, _ := newStubHandler(t)
	rec := postStubWithBody(t, h.MuteList, `{}`, "u1")
	assert.Equal(t, http.StatusOK, rec.Code)
	var arr []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &arr))
	assert.Empty(t, arr)
}

func TestMuteList_WithData(t *testing.T) {
	h, chRepo, _, mutRepo := newStubHandler(t)
	chRepo.Channels["ch1"] = &model.Channel{ID: "ch1", Name: "test"}
	mutRepo.Mutings["u1:ch1"] = &model.ChannelMuting{ID: "m1", UserID: "u1", ChannelID: "ch1"}
	rec := postStubWithBody(t, h.MuteList, `{}`, "u1")
	assert.Equal(t, http.StatusOK, rec.Code)
	var arr []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &arr))
	assert.Len(t, arr, 1)
}
