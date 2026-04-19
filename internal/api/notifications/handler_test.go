package notifications

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testRedis *testutil.TestRedis
	idGen     id.Generator
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		log.Fatalf("redis setup failed: %v", err)
	}
	testRedis = tr
	idGen, _ = id.NewGenerator("aidx")
	code := m.Run()
	tr.Teardown(ctx)
	os.Exit(code)
}

func newTestHandler(t *testing.T) (*Handler, *notification.Service) {
	t.Helper()
	testRedis.FlushAll(context.Background())
	svc := notification.NewService(testRedis.Client, idGen)
	return NewHandler(svc, idGen), svc
}

func newJSONRequest(t *testing.T, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func setAuth(c echo.Context, user *model.User) {
	c.Set(string(middleware.UserContextKey), user)
}

func TestShow_Empty(t *testing.T) {
	h, _ := newTestHandler(t)
	c, rec := newJSONRequest(t, "/api/i/notifications", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp)
}

func TestShow_WithEntries(t *testing.T) {
	h, svc := newTestHandler(t)
	ctx := context.Background()
	_, err := svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeFollow,
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeReaction,
		NoteID: "n1", Reaction: "👍",
	})
	require.NoError(t, err)

	c, rec := newJSONRequest(t, "/api/i/notifications", `{"limit":50}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, "reaction", resp[0]["type"])
	assert.Equal(t, "n1", resp[0]["noteId"])
	assert.Equal(t, "👍", resp[0]["reaction"])
	assert.Equal(t, "bob", resp[0]["userId"])
}

func TestShow_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	c, rec := newJSONRequest(t, "/api/i/notifications", `{invalid`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShow_RedisError(t *testing.T) {
	closed := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	_ = closed.Close()
	svc := notification.NewService(closed, idGen)
	h := NewHandler(svc, idGen)

	c, rec := newJSONRequest(t, "/api/i/notifications", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestMarkAllAsRead_OK(t *testing.T) {
	h, svc := newTestHandler(t)
	_, err := svc.Create(context.Background(), notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeFollow,
	})
	require.NoError(t, err)

	c, rec := newJSONRequest(t, "/api/notifications/mark-all-as-read", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.MarkAllAsRead(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestMarkAllAsRead_RedisError(t *testing.T) {
	closed := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	_ = closed.Close()
	svc := notification.NewService(closed, idGen)
	h := NewHandler(svc, idGen)

	c, rec := newJSONRequest(t, "/api/notifications/mark-all-as-read", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.MarkAllAsRead(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestCreate_Success(t *testing.T) {
	h, _ := newTestHandler(t)
	c, rec := newJSONRequest(t, "/api/notifications/create", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Create(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestFlush_Success(t *testing.T) {
	h, svc := newTestHandler(t)
	ctx := context.Background()
	// 1件作成してから flush するとストリームが消えることを確認。
	_, err := svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeFollow,
	})
	require.NoError(t, err)

	c, rec := newJSONRequest(t, "/api/notifications/flush", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Flush(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// handler が Flush() を呼んでいれば stream 長 0。
	// 以前は MarkAllAsRead() を呼んでいたため元通知が残ってしまっていた。
	out, err := svc.List(ctx, "alice", 10)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestFlush_NilService(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	h := NewHandler(nil, idGen)
	c, rec := newJSONRequest(t, "/api/notifications/flush", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Flush(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTestNotification_Success(t *testing.T) {
	h, _ := newTestHandler(t)
	c, rec := newJSONRequest(t, "/api/notifications/test-notification", `{}`)
	require.NoError(t, h.TestNotification(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestShow_WithUserAndNoteResolution(t *testing.T) {
	h, svc := newTestHandler(t)
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bobuser"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: "public", User: &model.User{ID: "bob", Username: "bobuser"}}
	h.SetRepos(userRepo, noteRepo)

	ctx := context.Background()
	_, err := svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeMention, NoteID: "n1",
	})
	require.NoError(t, err)

	c, rec := newJSONRequest(t, "/api/i/notifications", `{}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "mention", resp[0]["type"])
	assert.NotNil(t, resp[0]["user"])
	assert.NotNil(t, resp[0]["note"])
}

func TestShow_IncludeTypesFilter(t *testing.T) {
	h, svc := newTestHandler(t)
	ctx := context.Background()
	_, _ = svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeFollow,
	})
	_, _ = svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeMention, NoteID: "n1",
	})

	c, rec := newJSONRequest(t, "/api/i/notifications", `{"includeTypes":["mention"]}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "mention", resp[0]["type"])
}

func TestShow_ExcludeTypesFilter(t *testing.T) {
	h, svc := newTestHandler(t)
	ctx := context.Background()
	_, _ = svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeFollow,
	})
	_, _ = svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeMention, NoteID: "n1",
	})

	c, rec := newJSONRequest(t, "/api/i/notifications", `{"excludeTypes":["follow"]}`)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "mention", resp[0]["type"])
}

func TestShow_CursorPagination(t *testing.T) {
	h, svc := newTestHandler(t)
	ctx := context.Background()
	_, _ = svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeFollow,
	})
	n2, _ := svc.Create(ctx, notification.CreateInput{
		NotifieeID: "alice", NotifierID: "bob", Type: notification.TypeMention, NoteID: "n1",
	})

	// sinceId: n2より新しいものだけ (= 該当なし)
	body := `{"sinceId":"` + n2.ID + `"}`
	c, rec := newJSONRequest(t, "/api/i/notifications", body)
	setAuth(c, &model.User{ID: "alice"})
	require.NoError(t, h.Show(c))

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp)
}
