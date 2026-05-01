package clips

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	coreclip "github.com/shiroha-a/mk/internal/core/clip"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStubHandler(t *testing.T) (*Handler, *testutil.MockClipRepository, *testutil.MockClipFavoriteRepository) {
	t.Helper()
	repo := testutil.NewMockClipRepository()
	noteRepo := testutil.NewMockClipNoteRepository()
	notes := testutil.NewMockNoteRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := coreclip.NewService(repo, noteRepo, notes, idGen)
	h := NewHandler(svc, idGen)
	favRepo := testutil.NewMockClipFavoriteRepository()
	h.SetFavoriteRepo(favRepo)
	return h, repo, favRepo
}

func postStubWithBody(t *testing.T, handler func(echo.Context) error, body string, userID string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if userID != "" {
		c.Set(string(middleware.UserContextKey), &model.User{ID: userID})
	}
	_ = handler(c)
	return rec
}

func TestClipFavorite_MissingClipID(t *testing.T) {
	h, _, _ := newStubHandler(t)
	rec := postStubWithBody(t, h.Favorite, `{}`, "u1")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestClipFavorite_Success(t *testing.T) {
	h, clipRepo, favRepo := newStubHandler(t)
	clipRepo.Clips["cl1"] = &model.Clip{ID: "cl1", UserID: "u1", Name: "test", IsPublic: true}
	rec := postStubWithBody(t, h.Favorite, `{"clipId":"cl1"}`, "u1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	exists, _ := favRepo.Exists("u1", "cl1")
	assert.True(t, exists)
}

func TestClipFavorite_AlreadyFavorited(t *testing.T) {
	h, clipRepo, favRepo := newStubHandler(t)
	clipRepo.Clips["cl1"] = &model.Clip{ID: "cl1", UserID: "u1", Name: "test", IsPublic: true}
	favRepo.Favorites["u1:cl1"] = &model.ClipFavorite{ID: "f1", UserID: "u1", ClipID: "cl1"}
	rec := postStubWithBody(t, h.Favorite, `{"clipId":"cl1"}`, "u1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestClipFavorite_NilRepo(t *testing.T) {
	h, _, _, _ := newHandler(t)
	// favoriteRepo is nil → graceful NoContent
	rec := postStubWithBody(t, h.Favorite, `{"clipId":"cl1"}`, "u1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestClipUnfavorite_Success(t *testing.T) {
	h, _, favRepo := newStubHandler(t)
	favRepo.Favorites["u1:cl1"] = &model.ClipFavorite{ID: "f1", UserID: "u1", ClipID: "cl1"}
	rec := postStubWithBody(t, h.Unfavorite, `{"clipId":"cl1"}`, "u1")
	assert.Equal(t, http.StatusNoContent, rec.Code)
	exists, _ := favRepo.Exists("u1", "cl1")
	assert.False(t, exists)
}

func TestClipUnfavorite_MissingClipID(t *testing.T) {
	h, _, _ := newStubHandler(t)
	rec := postStubWithBody(t, h.Unfavorite, `{}`, "u1")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestClipMyFavorites_Empty(t *testing.T) {
	h, _, _ := newStubHandler(t)
	rec := postStubWithBody(t, h.MyFavorites, `{}`, "u1")
	assert.Equal(t, http.StatusOK, rec.Code)
	var arr []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &arr))
	assert.Empty(t, arr)
}

func TestClipMyFavorites_WithData(t *testing.T) {
	h, clipRepo, favRepo := newStubHandler(t)
	clipRepo.Clips["cl1"] = &model.Clip{ID: "cl1", UserID: "u1", Name: "test", IsPublic: true}
	favRepo.Favorites["u1:cl1"] = &model.ClipFavorite{ID: "f1", UserID: "u1", ClipID: "cl1"}
	rec := postStubWithBody(t, h.MyFavorites, `{}`, "u1")
	assert.Equal(t, http.StatusOK, rec.Code)
	var arr []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &arr))
	assert.Len(t, arr, 1)
}
