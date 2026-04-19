package channel_test

import (
	"testing"

	"github.com/shiroha-a/mk/internal/core/channel"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteCreateHook_EnsureChannelExists(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	hook := channel.NewNoteCreateHook(svc)
	require.NoError(t, hook.EnsureChannelExists("c1"))
}

func TestNoteCreateHook_EnsureChannelMissing(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	hook := channel.NewNoteCreateHook(svc)
	err := hook.EnsureChannelExists("missing")
	assert.ErrorIs(t, err, channel.ErrChannelNotFound)
}

func TestNoteCreateHook_OnNotePosted(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	hook := channel.NewNoteCreateHook(svc)
	hook.OnNotePosted("c1", "", "")
	assert.Equal(t, 1, repo.Channels["c1"].NotesCount)
}
