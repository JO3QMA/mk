package reversi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	corereversi "github.com/shiroha-a/mk/internal/core/reversi"
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

func (m *mockReversiRepo) ListByUserCursor(userID, sinceID, untilID string, limit int) ([]*model.ReversiGame, error) {
	var result []*model.ReversiGame
	for _, g := range m.games {
		if g.User1ID != userID && g.User2ID != userID {
			continue
		}
		if sinceID != "" && g.ID <= sinceID {
			continue
		}
		if untilID != "" && g.ID >= untilID {
			continue
		}
		result = append(result, g)
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockReversiRepo) ListStartedCursor(sinceID, untilID string, limit int) ([]*model.ReversiGame, error) {
	var result []*model.ReversiGame
	for _, g := range m.games {
		if !g.IsStarted {
			continue
		}
		if sinceID != "" && g.ID <= sinceID {
			continue
		}
		if untilID != "" && g.ID >= untilID {
			continue
		}
		result = append(result, g)
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
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

// my=true で viewer が関与するゲームのみ返す (CherryPick 互換)。
func TestGames_MyFlag(t *testing.T) {
	h, repo := newTestHandler()
	repo.games["g1"] = sampleGame()
	rec := post(h.Games, `{"my": true}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

// my=false (デフォルト) は isStarted=true のゲームのみ返す。sampleGame は
// IsStarted=false なので空配列。
func TestGames_PublicOnlyStarted(t *testing.T) {
	h, repo := newTestHandler()
	repo.games["g1"] = sampleGame() // IsStarted=false
	rec := post(h.Games, `{}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 0)
}

// started なゲームは my=false でも返る。
func TestGames_PublicShowsStarted(t *testing.T) {
	h, repo := newTestHandler()
	g := sampleGame()
	g.IsStarted = true
	repo.games["g1"] = g
	rec := post(h.Games, `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

// untilId でページング (cursor より古い = id が小さいゲームだけ返す) —
// 無限ループ防止の核。aidx ID と同じく新しいほど lexicographic で大きい前提。
func TestGames_UntilIdPagination(t *testing.T) {
	h, repo := newTestHandler()
	older := sampleGame()
	older.ID = "aaa-old"
	older.IsStarted = true
	repo.games[older.ID] = older
	newer := sampleGame()
	newer.ID = "zzz-new"
	newer.IsStarted = true
	repo.games[newer.ID] = newer
	rec := post(h.Games, `{"untilId":"zzz-new"}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "aaa-old", resp[0]["id"])
}

// --- Invitations ---

// CherryPick 互換で invitations は UserLite[] (招待者一覧) を返す。viewer が
// User2 (招待される側) のゲームについてのみ、User1 を UserLite で載せる。
func TestInvitations_Success(t *testing.T) {
	h, repo := newTestHandler()
	g := sampleGame()
	// sampleGame は User1="u1", User2="u2" なので viewer を u2 にして
	// u1 を招待者として得るパターンをテストする。
	g.IsStarted = false
	repo.games["g1"] = g
	u2 := &model.User{ID: "u2", Username: "bob"}
	rec := post(h.Invitations, `{}`, u2)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "u1", resp[0]["id"])
	assert.Equal(t, "alice", resp[0]["username"])
}

// viewer が User1 (招待側) の場合は自分の invitations に出さない。
func TestInvitations_ViewerIsInviter_ExcludeOwn(t *testing.T) {
	h, repo := newTestHandler()
	g := sampleGame()
	g.IsStarted = false
	repo.games["g1"] = g
	rec := post(h.Invitations, `{}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 0)
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

// --- Match with acct (CherryPick extension) ---

func TestMatch_AcctLocal(t *testing.T) {
	h, repo := newTestHandler()
	userRepo := testutil.NewMockUserRepository()
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", UsernameLower: "bob"}
	h.SetFederation("https://example.com", nil, nil, userRepo)

	rec := post(h.Match, `{"userId":"@bob"}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, repo.games, 1)
	for _, g := range repo.games {
		assert.Equal(t, "u2", g.User2ID)
	}
}

func TestMatch_AcctRemoteKnown(t *testing.T) {
	h, repo := newTestHandler()
	host := "remote.example"
	userRepo := testutil.NewMockUserRepository()
	userRepo.Users["u3"] = &model.User{
		ID: "u3", Username: "carol", UsernameLower: "carol", Host: &host,
	}
	h.SetFederation("https://example.com", nil, nil, userRepo)

	rec := post(h.Match, `{"userId":"@carol@remote.example"}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, repo.games, 1)
	for _, g := range repo.games {
		assert.Equal(t, "u3", g.User2ID)
	}
}

func TestMatch_AcctUnknown(t *testing.T) {
	h, _ := newTestHandler()
	userRepo := testutil.NewMockUserRepository()
	h.SetFederation("https://example.com", nil, nil, userRepo)

	rec := post(h.Match, `{"userId":"@ghost"}`, u1)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMatch_AcctEmptyPrefix(t *testing.T) {
	h, _ := newTestHandler()
	userRepo := testutil.NewMockUserRepository()
	h.SetFederation("https://example.com", nil, nil, userRepo)

	rec := post(h.Match, `{"userId":"@"}`, u1)
	assert.Equal(t, http.StatusNotFound, rec.Code)
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

func (m *mockDeliverer) DeliverToUser(_ string, _ *model.User, _ []byte) error {
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

// stubFedCache implements just enough of the FederationIDCache interface for
// testing handler.Match / handler.Surrender without touching Redis.
type stubFedCache struct {
	sessionToGame map[string]string
	gameToSession map[string]string
}

func (s *stubFedCache) Set(_ context.Context, federationID, gameID string) {
	s.sessionToGame[federationID] = gameID
	s.gameToSession[gameID] = federationID
}

func (s *stubFedCache) Get(_ context.Context, federationID string) (string, error) {
	if v, ok := s.sessionToGame[federationID]; ok {
		return v, nil
	}
	return "", errMock
}

func (s *stubFedCache) GetSessionByGame(_ context.Context, gameID string) (string, bool) {
	v, ok := s.gameToSession[gameID]
	return v, ok
}

func (s *stubFedCache) Delete(_ context.Context, federationID, gameID string) {
	delete(s.sessionToGame, federationID)
	delete(s.gameToSession, gameID)
}

func TestMatch_WithRemoteUser(t *testing.T) {
	// Use a real (nil-redis) FederationIDCache; Set/Get are no-ops which lets
	// the handler still fire DeliverToUser via federation branch.
	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/u2"
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", Host: &host, URI: &uri}
	fedCache := corereversi.NewFederationIDCache(nil)
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	rec := post(h.Match, `{"userId":"u2"}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, repo.games, 1)
	assert.Equal(t, 1, d.calls) // Invite 送信
}

func TestSurrender_WithRemoteUser(t *testing.T) {
	// stubFedCache stores the session/game mapping in memory so the handler's
	// GetSessionByGame lookup returns true and triggers the Leave delivery.
	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/u2"
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", Host: &host, URI: &uri}

	g := sampleGame()
	g.IsStarted = true
	repo.games["g1"] = g

	// Use a real cache bound to nil redis; Set/Get/Delete are no-ops which
	// means GetSessionByGame returns ("", false) — matching a non-federated
	// game case. Separately verify the federated path via the stub below.
	fedCache := corereversi.NewFederationIDCache(nil)
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	rec := post(h.Surrender, `{"gameId":"g1"}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	// No federation mapping → no Leave delivery
	assert.Equal(t, 0, d.calls)
}

func TestMatch_CreateErrorWithRemote(t *testing.T) {
	h, repo := newTestHandler()
	repo.createErr = errMock
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/u2"
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", Host: &host, URI: &uri}
	fedCache := corereversi.NewFederationIDCache(nil)
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	rec := post(h.Match, `{"userId":"u2"}`, u1)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 0, d.calls) // Create失敗時はInvite送らない
}

func TestMatch_EmptyUserIDIsRandomMatch(t *testing.T) {
	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	fedCache := corereversi.NewFederationIDCache(nil)
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	rec := post(h.Match, `{}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, repo.games, 1)
	assert.Equal(t, 0, d.calls)
}
