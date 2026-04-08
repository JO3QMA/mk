package channel_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/channel"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSvc(t *testing.T) (
	*channel.Service,
	*testutil.MockChannelRepository,
	*testutil.MockChannelFollowingRepository,
	*testutil.MockNoteRepository,
) {
	t.Helper()
	repo := testutil.NewMockChannelRepository()
	followRepo := testutil.NewMockChannelFollowingRepository()
	noteRepo := testutil.NewMockNoteRepository()
	idGen, _ := id.NewGenerator("aidx")
	return channel.NewService(repo, followRepo, noteRepo, idGen), repo, followRepo, noteRepo
}

// --- Create ----------------------------------------------------------------

func TestCreate_HappyPath(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	c, err := svc.Create(channel.CreateInput{OwnerID: "u1", Name: "alpha"})
	require.NoError(t, err)
	assert.Equal(t, "alpha", c.Name)
	assert.Equal(t, "#86b300", c.Color)
	assert.Len(t, repo.Channels, 1)
}

func TestCreate_CustomColor(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	c, err := svc.Create(channel.CreateInput{OwnerID: "u1", Name: "alpha", Color: "#ff0000"})
	require.NoError(t, err)
	assert.Equal(t, "#ff0000", c.Color)
}

func TestCreate_NameRequired(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Create(channel.CreateInput{OwnerID: "u1"})
	assert.ErrorIs(t, err, channel.ErrChannelNameRequired)
}

func TestCreate_OwnerRequired(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Create(channel.CreateInput{Name: "alpha"})
	assert.Error(t, err)
}

func TestCreate_RepoError(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.CreateErr = errors.New("boom")
	_, err := svc.Create(channel.CreateInput{OwnerID: "u1", Name: "alpha"})
	assert.Error(t, err)
}

// --- Show ------------------------------------------------------------------

func TestShow_HappyPath(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha"}
	got, err := svc.Show("c1")
	require.NoError(t, err)
	assert.Equal(t, "alpha", got.Name)
}

func TestShow_NotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Show("missing")
	assert.ErrorIs(t, err, channel.ErrChannelNotFound)
}

// --- Update ----------------------------------------------------------------

func TestUpdate_HappyPath(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	owner := "u1"
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha", UserID: &owner, Color: "#86b300"}
	newName := "alpha-v2"
	desc := "new"
	descPtr := &desc
	color := "#000"
	archived := true
	sensitive := true
	got, err := svc.Update("u1", "c1", channel.UpdateInput{
		Name:        &newName,
		Description: &descPtr,
		Color:       &color,
		IsArchived:  &archived,
		IsSensitive: &sensitive,
	})
	require.NoError(t, err)
	assert.Equal(t, "alpha-v2", got.Name)
}

func TestUpdate_NotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Update("u1", "missing", channel.UpdateInput{})
	assert.ErrorIs(t, err, channel.ErrChannelNotFound)
}

func TestUpdate_AccessDenied(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	owner := "u1"
	repo.Channels["c1"] = &model.Channel{ID: "c1", UserID: &owner}
	_, err := svc.Update("u2", "c1", channel.UpdateInput{})
	assert.ErrorIs(t, err, channel.ErrAccessDenied)
}

func TestUpdate_NameEmpty(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	owner := "u1"
	repo.Channels["c1"] = &model.Channel{ID: "c1", UserID: &owner}
	empty := ""
	_, err := svc.Update("u1", "c1", channel.UpdateInput{Name: &empty})
	assert.ErrorIs(t, err, channel.ErrChannelNameRequired)
}

func TestUpdate_NoOwner(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"} // UserID nil
	_, err := svc.Update("u1", "c1", channel.UpdateInput{})
	assert.ErrorIs(t, err, channel.ErrAccessDenied)
}

func TestUpdate_RepoError(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	owner := "u1"
	repo.Channels["c1"] = &model.Channel{ID: "c1", UserID: &owner}
	repo.UpdateErr = errors.New("boom")
	name := "x"
	_, err := svc.Update("u1", "c1", channel.UpdateInput{Name: &name})
	assert.Error(t, err)
}

// --- Follow / Unfollow -----------------------------------------------------

func TestFollow_HappyPath(t *testing.T) {
	svc, repo, followRepo, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	require.NoError(t, svc.Follow("u1", "c1"))
	assert.Len(t, followRepo.Followings, 1)
	assert.Equal(t, 1, repo.Channels["c1"].UsersCount)
}

func TestFollow_ChannelNotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.Follow("u1", "missing")
	assert.ErrorIs(t, err, channel.ErrChannelNotFound)
}

func TestFollow_AlreadyFollowing(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	require.NoError(t, svc.Follow("u1", "c1"))
	err := svc.Follow("u1", "c1")
	assert.ErrorIs(t, err, channel.ErrAlreadyFollowing)
}

func TestUnfollow_HappyPath(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	require.NoError(t, svc.Follow("u1", "c1"))
	require.NoError(t, svc.Unfollow("u1", "c1"))
	assert.Equal(t, 0, repo.Channels["c1"].UsersCount)
}

func TestUnfollow_NotFollowing(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	err := svc.Unfollow("u1", "c1")
	assert.ErrorIs(t, err, channel.ErrNotFollowing)
}

// failingFollowRepo causes Exists / Create / Delete to fail.
type failingFollowRepo struct {
	*testutil.MockChannelFollowingRepository
	existsErr error
	createErr error
	deleteErr error
}

func (f *failingFollowRepo) Exists(followerID, channelID string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.MockChannelFollowingRepository.Exists(followerID, channelID)
}
func (f *failingFollowRepo) Create(fw *model.ChannelFollowing) error {
	if f.createErr != nil {
		return f.createErr
	}
	return f.MockChannelFollowingRepository.Create(fw)
}
func (f *failingFollowRepo) Delete(fw *model.ChannelFollowing) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return f.MockChannelFollowingRepository.Delete(fw)
}

func TestFollow_ExistsError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	followRepo := &failingFollowRepo{
		MockChannelFollowingRepository: testutil.NewMockChannelFollowingRepository(),
		existsErr:                      errors.New("exists boom"),
	}
	idGen, _ := id.NewGenerator("aidx")
	svc := channel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)
	err := svc.Follow("u1", "c1")
	assert.Error(t, err)
}

func TestFollow_CreateError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	followRepo := &failingFollowRepo{
		MockChannelFollowingRepository: testutil.NewMockChannelFollowingRepository(),
		createErr:                      errors.New("create boom"),
	}
	idGen, _ := id.NewGenerator("aidx")
	svc := channel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)
	err := svc.Follow("u1", "c1")
	assert.Error(t, err)
}

func TestUnfollow_DeleteError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	mock := testutil.NewMockChannelFollowingRepository()
	mock.Followings["f1"] = &model.ChannelFollowing{ID: "f1", FollowerID: "u1", FolloweeID: "c1"}
	followRepo := &failingFollowRepo{
		MockChannelFollowingRepository: mock,
		deleteErr:                      errors.New("delete boom"),
	}
	idGen, _ := id.NewGenerator("aidx")
	svc := channel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)
	err := svc.Unfollow("u1", "c1")
	assert.Error(t, err)
}

// --- Listing ---------------------------------------------------------------

func TestListFollowed(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha"}
	repo.Channels["c2"] = &model.Channel{ID: "c2", Name: "beta"}
	require.NoError(t, svc.Follow("u1", "c1"))
	require.NoError(t, svc.Follow("u1", "c2"))

	rows, err := svc.ListFollowed("u1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

// brokenChannelRepo makes FindByID fail so ListFollowed skips entries.
type brokenChannelRepo struct {
	*testutil.MockChannelRepository
}

func (r *brokenChannelRepo) FindByID(_ string) (*model.Channel, error) {
	return nil, errors.New("boom")
}

func TestListFollowed_FindByIDFailure(t *testing.T) {
	mock := testutil.NewMockChannelRepository()
	repo := &brokenChannelRepo{MockChannelRepository: mock}
	followRepo := testutil.NewMockChannelFollowingRepository()
	followRepo.Followings["f1"] = &model.ChannelFollowing{ID: "f1", FollowerID: "u1", FolloweeID: "c1"}
	idGen, _ := id.NewGenerator("aidx")
	svc := channel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)

	rows, err := svc.ListFollowed("u1", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// listFailFollowRepo causes ListFollowed to fail.
type listFailFollowRepo struct {
	*testutil.MockChannelFollowingRepository
}

func (r *listFailFollowRepo) ListFollowed(_ string, _, _ int) ([]*model.ChannelFollowing, error) {
	return nil, errors.New("list boom")
}

func TestListFollowed_ListError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	followRepo := &listFailFollowRepo{MockChannelFollowingRepository: testutil.NewMockChannelFollowingRepository()}
	idGen, _ := id.NewGenerator("aidx")
	svc := channel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)
	_, err := svc.ListFollowed("u1", 10, 0)
	assert.Error(t, err)
}

func TestListOwned(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	owner := "u1"
	repo.Channels["c1"] = &model.Channel{ID: "c1", UserID: &owner}
	repo.Channels["c2"] = &model.Channel{ID: "c2", UserID: &owner}
	rows, err := svc.ListOwned("u1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestListFeatured(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	rows, err := svc.ListFeatured(10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func TestSearch(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha"}
	repo.Channels["c2"] = &model.Channel{ID: "c2", Name: "beta"}
	rows, err := svc.Search("alp", 10, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

// --- Timeline --------------------------------------------------------------

func TestTimeline_HappyPath(t *testing.T) {
	svc, repo, _, noteRepo := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	cid := "c1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", ChannelID: &cid}
	rows, err := svc.Timeline("c1", "", "", 10)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func TestTimeline_ChannelNotFound(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	_, err := svc.Timeline("missing", "", "", 10)
	assert.ErrorIs(t, err, channel.ErrChannelNotFound)
}

// --- OnNotePosted ----------------------------------------------------------

func TestOnNotePosted_UpdatesCounters(t *testing.T) {
	svc, repo, _, _ := newSvc(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	svc.OnNotePosted("c1")
	assert.Equal(t, 1, repo.Channels["c1"].NotesCount)
	require.NotNil(t, repo.Channels["c1"].LastNotedAt)
}

// --- SetClock --------------------------------------------------------------

func TestSetClock(t *testing.T) {
	svc, _, _, _ := newSvc(t)
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return fixed })
	svc.SetClock(nil) // nil 渡し無視
	c, err := svc.Create(channel.CreateInput{OwnerID: "u1", Name: "alpha"})
	require.NoError(t, err)
	// id は idGen で fixed タイムから派生する
	assert.NotEmpty(t, c.ID)
}
