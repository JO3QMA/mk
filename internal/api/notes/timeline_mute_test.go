package notes

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMutedChannelIDs(t *testing.T) {
	h, _ := newTestHandler(t)
	repo := testutil.NewMockChannelMutingRepository()
	require.NoError(t, repo.Create(&model.ChannelMuting{ID: "m1", UserID: "viewer", ChannelID: "ch-muted-1"}))
	require.NoError(t, repo.Create(&model.ChannelMuting{ID: "m2", UserID: "viewer", ChannelID: "ch-muted-2"}))
	require.NoError(t, repo.Create(&model.ChannelMuting{ID: "m3", UserID: "other", ChannelID: "ch-other"}))
	h.SetChannelMutingRepo(repo)

	viewer := &model.User{ID: "viewer"}
	ids := h.loadMutedChannelIDs(viewer)
	assert.ElementsMatch(t, []string{"ch-muted-1", "ch-muted-2"}, ids)
}

func TestLoadMutedChannelIDs_AnonymousReturnsNil(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetChannelMutingRepo(testutil.NewMockChannelMutingRepository())
	assert.Nil(t, h.loadMutedChannelIDs(nil))
}

func TestLoadMutedChannelIDs_RepoUnsetReturnsNil(t *testing.T) {
	h, _ := newTestHandler(t)
	assert.Nil(t, h.loadMutedChannelIDs(&model.User{ID: "viewer"}))
}

func TestLoadMutedChannelIDs_EmptyResultReturnsNil(t *testing.T) {
	h, _ := newTestHandler(t)
	repo := testutil.NewMockChannelMutingRepository()
	h.SetChannelMutingRepo(repo)
	assert.Nil(t, h.loadMutedChannelIDs(&model.User{ID: "viewer"}))
}
