package emojis_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/emojis"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup() (*emojis.Handler, *testutil.MockEmojiRepository) {
	repo := testutil.NewMockEmojiRepository()
	h := emojis.NewHandler(repo)
	return h, repo
}

func doPost(h *emojis.Handler) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/emojis", strings.NewReader("{}"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = h.Emojis(c)
	return rec
}

func TestEmojis_Empty(t *testing.T) {
	h, _ := setup()
	rec := doPost(h)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	arr, ok := body["emojis"].([]any)
	require.True(t, ok)
	assert.Empty(t, arr)
}

func TestEmojis_WithLocalEmojis(t *testing.T) {
	h, repo := setup()
	cat := "faces"
	repo.Emojis["smile@"] = &model.Emoji{
		Name:      "smile",
		Category:  &cat,
		PublicURL: "https://example.com/emoji/smile.png",
		Aliases:   []string{"happy"},
	}

	rec := doPost(h)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	arr, ok := body["emojis"].([]any)
	require.True(t, ok)
	assert.Len(t, arr, 1)

	emoji := arr[0].(map[string]any)
	assert.Equal(t, "smile", emoji["name"])
	assert.Equal(t, "faces", emoji["category"])
	assert.Equal(t, "https://example.com/emoji/smile.png", emoji["url"])
}

// failingEmojiRepo always fails on ListLocal.
type failingEmojiRepo struct {
	*testutil.MockEmojiRepository
}

func (f *failingEmojiRepo) ListLocal() ([]*model.Emoji, error) {
	return nil, assert.AnError
}

func TestEmojis_RepoError_ReturnsEmptyArray(t *testing.T) {
	repo := &failingEmojiRepo{MockEmojiRepository: testutil.NewMockEmojiRepository()}
	h := emojis.NewHandler(repo)
	rec := doPost(h)
	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	arr := body["emojis"].([]any)
	assert.Empty(t, arr)
}

func TestEmojis_ExcludesRemote(t *testing.T) {
	h, repo := setup()
	host := "remote.example.com"
	repo.Emojis["alien@remote.example.com"] = &model.Emoji{
		Name: "alien",
		Host: &host,
	}

	rec := doPost(h)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	arr := body["emojis"].([]any)
	assert.Empty(t, arr) // リモート絵文字は除外
}
