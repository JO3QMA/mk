package reversi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/lib/pq"
	corereversi "github.com/shiroha-a/mk/internal/core/reversi"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

var apiReversiRedis *testutil.TestRedis

func TestMain(m *testing.M) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		log.Fatalf("api/reversi test: redis setup failed: %v", err)
	}
	apiReversiRedis = tr
	code := m.Run()
	apiReversiRedis.Teardown(ctx)
	os.Exit(code)
}

// federation 経路 (GetSessionByGame が true を返す) をカバーするため、
// 実 Redis に federationId を Set してから Surrender を叩く。delivery は
// Service 側 (core/reversi) に移動したため (#417 P1)、Service も wire する。
func TestSurrender_FederatedGame_SendsLeave(t *testing.T) {
	ctx := context.Background()
	apiReversiRedis.FlushAll(ctx)

	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/u2"
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", Host: &host, URI: &uri}
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}

	g := sampleGame()
	g.IsStarted = true
	repo.games["g1"] = g

	fedCache := corereversi.NewFederationIDCache(apiReversiRedis.Client)
	fedCache.Set(ctx, "sess-1", "g1")
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	// Service 経由で delivery させるため Service を wire する。
	svc := corereversi.NewService(repo, nil, apiReversiRedis.Client)
	svc.SetFederationCache(fedCache)
	svc.SetFederationDeliverer(d)
	svc.SetUserRepo(userRepo)
	svc.SetBaseURL("https://example.com")
	h.SetService(svc)

	rec := post(h.Surrender, `{"gameId":"g1"}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Leave アクティビティが送信されたことを確認
	assert.Equal(t, 1, d.calls)

	// mapping が削除されたこと (Delete 経路) を確認
	_, ok := fedCache.GetSessionByGame(ctx, "g1")
	assert.False(t, ok)
}

// /api/reversi/match で同じ相手から既に招待を受けていれば、既存のゲーム行を
// 再利用して Join を送り返すだけにする (#417 P1)。これをやらないと毎回
// 新しいゲーム + 新 session_id を作って二重招待になり state 伝播が破綻する。
func TestMatch_AcceptsPendingRemoteInvitation_ReusesGameAndSendsJoin(t *testing.T) {
	ctx := context.Background()
	apiReversiRedis.FlushAll(ctx)

	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/remoteAlice"
	userRepo.Users["remoteAlice"] = &model.User{
		ID: "remoteAlice", Username: "alice", Host: &host, URI: &uri,
	}

	// inbound Invite で作成された pending game を事前に用意する。
	pending := &model.ReversiGame{
		ID:                   "gPending",
		User1ID:              "remoteAlice", // inviter (remote)
		User2ID:              "u1",          // invitee (local viewer)
		Map:                  pq.StringArray{"--------", "--------", "--------", "---wb---", "---bw---", "--------", "--------", "--------"},
		BW:                   "random",
		TimeLimitForEachTurn: 90,
		Logs:                 datatypes.JSON("[]"),
	}
	repo.games["gPending"] = pending

	// handleReversiInvite で紐付けられていたと仮定する session 情報を Redis に入れる。
	fedCache := corereversi.NewFederationIDCache(apiReversiRedis.Client)
	fedCache.Set(ctx, "sess-inbound", "gPending")
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	// match 呼び出し: 相手 ID を指定 (フロントエンドが invitation クリックで送る形)
	rec := post(h.Match, `{"userId":"remoteAlice"}`, u1)
	assert.Equal(t, http.StatusOK, rec.Code)

	// ゲームが 1 件のまま (新しい row が作られていないこと)
	assert.Len(t, repo.games, 1)

	// deliverer に Join が 1 回送られたこと
	assert.Equal(t, 1, d.calls)

	// 返却ゲーム ID が既存の pending と同じであること
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "gPending", resp["id"])
}

// userRepo.FindByID が err を返すケース (リモート相手がまだ取り込まれていない)
// では Leave 送信はスキップされる。
func TestSurrender_FederatedGame_UserNotFound(t *testing.T) {
	ctx := context.Background()
	apiReversiRedis.FlushAll(ctx)

	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	// u2 を登録しない → FindByID が失敗する
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}

	g := sampleGame()
	g.IsStarted = true
	repo.games["g1"] = g

	fedCache := corereversi.NewFederationIDCache(apiReversiRedis.Client)
	fedCache.Set(ctx, "sess-2", "g1")
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	svc := corereversi.NewService(repo, nil, apiReversiRedis.Client)
	svc.SetFederationCache(fedCache)
	svc.SetFederationDeliverer(d)
	svc.SetUserRepo(userRepo)
	svc.SetBaseURL("https://example.com")
	h.SetService(svc)

	rec := post(h.Surrender, `{"gameId":"g1"}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 0, d.calls)

	// mapping は Delete 経路で消える
	_, ok := fedCache.GetSessionByGame(ctx, "g1")
	require.False(t, ok)
}
