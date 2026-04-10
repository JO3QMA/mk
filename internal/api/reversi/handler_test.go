package reversi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

var errMock = assert.AnError

type mockReversiRepo struct {
	games     map[string]*model.ReversiGame
	createErr error
}

func newMock() *mockReversiRepo {
	return &mockReversiRepo{games: make(map[string]*model.ReversiGame)}
}

func (m *mockReversiRepo) Create(g *model.ReversiGame) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.games[g.ID] = g
	return nil
}

func (m *mockReversiRepo) FindByID(id string) (*model.ReversiGame, error) {
	if g, ok := m.games[id]; ok {
		return g, nil
	}
	return nil, errMock
}

func (m *mockReversiRepo) Update(g *model.ReversiGame) error {
	m.games[g.ID] = g
	return nil
}

func (m *mockReversiRepo) ListByUser(userID string, limit int) ([]*model.ReversiGame, error) {
	var result []*model.ReversiGame
	for _, g := range m.games {
		if g.User1ID == userID || g.User2ID == userID {
			result = append(result, g)
		}
	}
	return result, nil
}

func (m *mockReversiRepo) ListActive() ([]*model.ReversiGame, error) {
	var result []*model.ReversiGame
	for _, g := range m.games {
		if !g.IsEnded {
			result = append(result, g)
		}
	}
	return result, nil
}

func (m *mockReversiRepo) Delete(id string) error {
	delete(m.games, id)
	return nil
}

func newTestHandler() (*Handler, *mockReversiRepo) {
	repo := newMock()
	idGen, _ := id.NewGenerator("aidx")
	return NewHandler(repo, idGen), repo
}

func post(handler func(echo.Context) error, body string, user *model.User) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if user != nil {
		c.Set(string(middleware.UserContextKey), user)
	}
	_ = handler(c)
	return rec
}

var u1 = &model.User{ID: "u1", Username: "alice"}

func sampleGame() *model.ReversiGame {
	return &model.ReversiGame{
		ID: "g1", User1ID: "u1", User2ID: "u2",
		Map: pq.StringArray{"--------", "--------", "--------", "---wb---", "---bw---", "--------", "--------", "--------"},
		BW:  "random", TimeLimitForEachTurn: 90,
		Logs:  datatypes.JSON("[]"),
		User1: &model.User{ID: "u1", Username: "alice"},
		User2: &model.User{ID: "u2", Username: "bob"},
	}
}

// --- Games ---

func TestGames_WithUser(t *testing.T) {
	h, repo := newTestHandler()
	repo.games["g1"] = sampleGame()
	rec := post(h.Games, `{}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestGames_Anonymous(t *testing.T) {
	h, repo := newTestHandler()
	repo.games["g1"] = sampleGame()
	rec := post(h.Games, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Invitations ---

func TestInvitations_Success(t *testing.T) {
	h, repo := newTestHandler()
	g := sampleGame()
	g.IsStarted = false
	repo.games["g1"] = g
	rec := post(h.Invitations, `{}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestInvitations_Empty(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.Invitations, `{}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- ShowGame ---

func TestShowGame_Success(t *testing.T) {
	h, repo := newTestHandler()
	repo.games["g1"] = sampleGame()
	rec := post(h.ShowGame, `{"gameId":"g1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShowGame_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.ShowGame, `{"gameId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestShowGame_InvalidParam(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.ShowGame, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Match ---

func TestMatch_Success(t *testing.T) {
	h, repo := newTestHandler()
	rec := post(h.Match, `{"userId":"u2"}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, repo.games, 1)
}

func TestMatch_NoTarget(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.Match, `{}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMatch_CreateError(t *testing.T) {
	h, repo := newTestHandler()
	repo.createErr = errMock
	rec := post(h.Match, `{"userId":"u2"}`, u1)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- CancelMatch ---

func TestCancelMatch_Success(t *testing.T) {
	h, repo := newTestHandler()
	g := sampleGame()
	g.IsStarted = false
	repo.games["g1"] = g
	rec := post(h.CancelMatch, `{}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, repo.games)
}

// --- Surrender ---

func TestSurrender_Success(t *testing.T) {
	h, repo := newTestHandler()
	g := sampleGame()
	g.IsStarted = true
	repo.games["g1"] = g
	rec := post(h.Surrender, `{"gameId":"g1"}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, repo.games["g1"].IsEnded)
	assert.Equal(t, "u2", *repo.games["g1"].WinnerID)
}

func TestSurrender_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.Surrender, `{"gameId":"ghost"}`, u1)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSurrender_NotPlayer(t *testing.T) {
	h, repo := newTestHandler()
	repo.games["g1"] = sampleGame()
	rec := post(h.Surrender, `{"gameId":"g1"}`, &model.User{ID: "u3"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSurrender_InvalidParam(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.Surrender, `{}`, u1)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Verify ---

func TestVerify_Success(t *testing.T) {
	h, repo := newTestHandler()
	repo.games["g1"] = sampleGame()
	rec := post(h.Verify, `{"gameId":"g1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["desynced"])
}

func TestVerify_NotFound(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.Verify, `{"gameId":"ghost"}`, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestVerify_InvalidParam(t *testing.T) {
	h, _ := newTestHandler()
	rec := post(h.Verify, `{}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestVerify_WithLogs(t *testing.T) {
	h, repo := newTestHandler()
	g := sampleGame()
	g.Logs = datatypes.JSON(`[[26,1]]`) // pos=26 (2,3 on 8x8 board)
	repo.games["g1"] = g
	rec := post(h.Verify, `{"gameId":"g1"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Federation ---

type mockDeliverer struct {
	calls int
}

func (m *mockDeliverer) DeliverToUser(_ any, _, _ string) error {
	m.calls++
	return nil
}

func TestSetFederation(t *testing.T) {
	h, _ := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	h.SetFederation("https://example.com", d, nil, userRepo)
	assert.Equal(t, "https://example.com", h.baseURL)
	assert.NotNil(t, h.deliverer)
}

func TestMatch_WithRemoteUser(t *testing.T) {
	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/u2"
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", Host: &host, URI: &uri}
	h.SetFederation("https://example.com", d, nil, userRepo)

	rec := post(h.Match, `{"userId":"u2"}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, repo.games, 1)
	// 連合ゲーム: federationIdが設定される
	for _, g := range repo.games {
		assert.NotNil(t, g.FederationID)
	}
	assert.Equal(t, 1, d.calls) // Invite送信
}

func TestSurrender_WithRemoteUser(t *testing.T) {
	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/u2"
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", Host: &host, URI: &uri}
	h.SetFederation("https://example.com", d, nil, userRepo)

	fedID := "fed-123"
	g := sampleGame()
	g.IsStarted = true
	g.FederationID = &fedID
	repo.games["g1"] = g

	rec := post(h.Surrender, `{"gameId":"g1"}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 1, d.calls) // Leave送信
}
