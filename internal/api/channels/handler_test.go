package channels

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	corechannel "github.com/shiroha-a/mk/internal/core/channel"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHandler(t *testing.T) (
	*Handler,
	*testutil.MockChannelRepository,
	*testutil.MockChannelFollowingRepository,
	*testutil.MockNoteRepository,
) {
	t.Helper()
	repo := testutil.NewMockChannelRepository()
	followRepo := testutil.NewMockChannelFollowingRepository()
	noteRepo := testutil.NewMockNoteRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, followRepo, noteRepo, idGen)
	return NewHandler(svc, idGen), repo, followRepo, noteRepo
}

func newReq(t *testing.T, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func setUser(c echo.Context, userID string) {
	c.Set(string(middleware.UserContextKey), &model.User{ID: userID})
}

// --- Create ----------------------------------------------------------------

func TestCreate_Success(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{"name":"alpha","color":"#abcdef"}`)
	setUser(c, "alice")
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alpha")
}

func TestCreate_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	setUser(c, "alice")
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreate_NameRequired(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{}`)
	setUser(c, "alice")
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// failingChannelRepo causes Create to fail.
type failingChannelRepo struct {
	*testutil.MockChannelRepository
}

func (r *failingChannelRepo) Create(_ *model.Channel) error { return errors.New("boom") }

func TestCreate_RepoError(t *testing.T) {
	mock := testutil.NewMockChannelRepository()
	repo := &failingChannelRepo{MockChannelRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, testutil.NewMockChannelFollowingRepository(), testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)

	c, rec := newReq(t, `{"name":"alpha"}`)
	setUser(c, "alice")
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Show ------------------------------------------------------------------

func TestShow_Success(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha"}
	c, rec := newReq(t, `{"channelId":"c1"}`)
	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShow_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_EmptyID(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{}`)
	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{"channelId":"missing"}`)
	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Update ----------------------------------------------------------------

func TestUpdate_Success(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	owner := "alice"
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha", UserID: &owner}
	c, rec := newReq(t, `{"channelId":"c1","name":"alpha-v2"}`)
	setUser(c, "alice")
	require.NoError(t, h.Update(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdate_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	setUser(c, "alice")
	require.NoError(t, h.Update(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdate_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{"channelId":"missing"}`)
	setUser(c, "alice")
	require.NoError(t, h.Update(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdate_AccessDenied(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	other := "bob"
	repo.Channels["c1"] = &model.Channel{ID: "c1", UserID: &other}
	c, rec := newReq(t, `{"channelId":"c1","name":"x"}`)
	setUser(c, "alice")
	require.NoError(t, h.Update(c))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUpdate_NameEmpty(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	owner := "alice"
	repo.Channels["c1"] = &model.Channel{ID: "c1", UserID: &owner}
	c, rec := newReq(t, `{"channelId":"c1","name":""}`)
	setUser(c, "alice")
	require.NoError(t, h.Update(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdate_DescriptionUpdated(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	owner := "alice"
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha", UserID: &owner}
	c, rec := newReq(t, `{"channelId":"c1","description":"new"}`)
	setUser(c, "alice")
	require.NoError(t, h.Update(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// failingUpdateRepo causes UpdateFields to fail to exercise the internalError
// branch.
type failingUpdateRepo struct {
	*testutil.MockChannelRepository
}

func (r *failingUpdateRepo) UpdateFields(_ string, _ map[string]any) error {
	return errors.New("boom")
}

func TestUpdate_RepoError(t *testing.T) {
	mock := testutil.NewMockChannelRepository()
	owner := "alice"
	mock.Channels["c1"] = &model.Channel{ID: "c1", UserID: &owner}
	repo := &failingUpdateRepo{MockChannelRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, testutil.NewMockChannelFollowingRepository(), testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)
	c, rec := newReq(t, `{"channelId":"c1","name":"x"}`)
	setUser(c, "alice")
	require.NoError(t, h.Update(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Follow / Unfollow -----------------------------------------------------

func TestFollow_Success(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	c, rec := newReq(t, `{"channelId":"c1"}`)
	setUser(c, "alice")
	require.NoError(t, h.Follow(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestFollow_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	setUser(c, "alice")
	require.NoError(t, h.Follow(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFollow_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{"channelId":"missing"}`)
	setUser(c, "alice")
	require.NoError(t, h.Follow(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestFollow_AlreadyFollowing(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	c, rec := newReq(t, `{"channelId":"c1"}`)
	setUser(c, "alice")
	require.NoError(t, h.Follow(c))
	c2, rec2 := newReq(t, `{"channelId":"c1"}`)
	setUser(c2, "alice")
	require.NoError(t, h.Follow(c2))
	assert.Equal(t, http.StatusBadRequest, rec2.Code)
	_ = rec
}

// failingFollowRepo causes Follow.Create to fail (other than already
// following) to exercise internalError branch.
type failingFollowRepo struct {
	*testutil.MockChannelFollowingRepository
}

func (r *failingFollowRepo) Create(_ *model.ChannelFollowing) error { return errors.New("boom") }

func TestFollow_InternalError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	followRepo := &failingFollowRepo{MockChannelFollowingRepository: testutil.NewMockChannelFollowingRepository()}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)

	c, rec := newReq(t, `{"channelId":"c1"}`)
	setUser(c, "alice")
	require.NoError(t, h.Follow(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUnfollow_Success(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	c, rec := newReq(t, `{"channelId":"c1"}`)
	setUser(c, "alice")
	require.NoError(t, h.Follow(c))
	_ = rec
	c2, rec2 := newReq(t, `{"channelId":"c1"}`)
	setUser(c2, "alice")
	require.NoError(t, h.Unfollow(c2))
	assert.Equal(t, http.StatusNoContent, rec2.Code)
}

func TestUnfollow_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	setUser(c, "alice")
	require.NoError(t, h.Unfollow(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnfollow_NotFollowing(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{"channelId":"c1"}`)
	setUser(c, "alice")
	require.NoError(t, h.Unfollow(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// failingUnfollowRepo causes Delete to fail (other than not following).
type failingUnfollowRepo struct {
	*testutil.MockChannelFollowingRepository
}

func (r *failingUnfollowRepo) Delete(_ *model.ChannelFollowing) error { return errors.New("boom") }

func TestUnfollow_InternalError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	mock := testutil.NewMockChannelFollowingRepository()
	mock.Followings["f1"] = &model.ChannelFollowing{ID: "f1", FollowerID: "alice", FolloweeID: "c1"}
	followRepo := &failingUnfollowRepo{MockChannelFollowingRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)

	c, rec := newReq(t, `{"channelId":"c1"}`)
	setUser(c, "alice")
	require.NoError(t, h.Unfollow(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Listing ---------------------------------------------------------------

func TestFollowed_Success(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{}`)
	setUser(c, "alice")
	require.NoError(t, h.Followed(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFollowed_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	setUser(c, "alice")
	require.NoError(t, h.Followed(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// listFailFollowRepo: ListFollowed returns an error.
type listFailFollowRepo struct {
	*testutil.MockChannelFollowingRepository
}

func (r *listFailFollowRepo) ListFollowed(_ string, _, _ int) ([]*model.ChannelFollowing, error) {
	return nil, errors.New("boom")
}

func TestFollowed_RepoError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	followRepo := &listFailFollowRepo{MockChannelFollowingRepository: testutil.NewMockChannelFollowingRepository()}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, followRepo, testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)
	c, rec := newReq(t, `{}`)
	setUser(c, "alice")
	require.NoError(t, h.Followed(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestOwned_Success(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{}`)
	setUser(c, "alice")
	require.NoError(t, h.Owned(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestOwned_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	setUser(c, "alice")
	require.NoError(t, h.Owned(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// listFailChannelRepo: List returns an error.
type listFailChannelRepo struct {
	*testutil.MockChannelRepository
}

func (r *listFailChannelRepo) List(_ model.ChannelListFilter) ([]*model.Channel, error) {
	return nil, errors.New("boom")
}

func TestOwned_RepoError(t *testing.T) {
	mock := testutil.NewMockChannelRepository()
	repo := &listFailChannelRepo{MockChannelRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, testutil.NewMockChannelFollowingRepository(), testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)
	c, rec := newReq(t, `{}`)
	setUser(c, "alice")
	require.NoError(t, h.Owned(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestFeatured_Success(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{}`)
	require.NoError(t, h.Featured(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFeatured_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	require.NoError(t, h.Featured(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFeatured_RepoError(t *testing.T) {
	mock := testutil.NewMockChannelRepository()
	repo := &listFailChannelRepo{MockChannelRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, testutil.NewMockChannelFollowingRepository(), testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)
	c, rec := newReq(t, `{}`)
	require.NoError(t, h.Featured(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSearch_Success(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1", Name: "alpha channel"}
	c, rec := newReq(t, `{"query":"alpha"}`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alpha channel")
}

func TestSearch_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearch_RepoError(t *testing.T) {
	mock := testutil.NewMockChannelRepository()
	repo := &listFailChannelRepo{MockChannelRepository: mock}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, testutil.NewMockChannelFollowingRepository(), testutil.NewMockNoteRepository(), idGen)
	h := NewHandler(svc, idGen)
	c, rec := newReq(t, `{"query":"x"}`)
	require.NoError(t, h.Search(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Timeline --------------------------------------------------------------

func TestTimeline_Success(t *testing.T) {
	h, repo, _, noteRepo := newHandler(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	cid := "c1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", ChannelID: &cid, UserID: "u1"}
	c, rec := newReq(t, `{"channelId":"c1"}`)
	require.NoError(t, h.Timeline(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTimeline_BadJSON(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{not`)
	require.NoError(t, h.Timeline(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTimeline_NotFound(t *testing.T) {
	h, _, _, _ := newHandler(t)
	c, rec := newReq(t, `{"channelId":"missing"}`)
	require.NoError(t, h.Timeline(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTimeline_LimitClamping(t *testing.T) {
	h, repo, _, _ := newHandler(t)
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	c, rec := newReq(t, `{"channelId":"c1","limit":9999}`)
	require.NoError(t, h.Timeline(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// failingNoteRepo: ListByChannelID returns an error so Timeline hits the
// internal-error branch.
type failingChannelNoteRepo struct {
	*testutil.MockNoteRepository
}

func (r *failingChannelNoteRepo) ListByChannelID(_ string, _, _ string, _ int) ([]*model.Note, error) {
	return nil, errors.New("boom")
}

func TestTimeline_RepoError(t *testing.T) {
	repo := testutil.NewMockChannelRepository()
	repo.Channels["c1"] = &model.Channel{ID: "c1"}
	noteRepo := &failingChannelNoteRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	idGen, _ := id.NewGenerator("aidx")
	svc := corechannel.NewService(repo, testutil.NewMockChannelFollowingRepository(), noteRepo, idGen)
	h := NewHandler(svc, idGen)
	c, rec := newReq(t, `{"channelId":"c1"}`)
	require.NoError(t, h.Timeline(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
