package reversi

import (
	"context"
	"log"
	"net/http"
	"os"
	"testing"

	corereversi "github.com/shiroha-a/mk/internal/core/reversi"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
// 実 Redis に federationId を Set してから Surrender を叩く。
func TestSurrender_FederatedGame_SendsLeave(t *testing.T) {
	ctx := context.Background()
	apiReversiRedis.FlushAll(ctx)

	h, repo := newTestHandler()
	d := &mockDeliverer{}
	userRepo := testutil.NewMockUserRepository()
	host := "remote.example"
	uri := "https://remote.example/users/u2"
	userRepo.Users["u2"] = &model.User{ID: "u2", Username: "bob", Host: &host, URI: &uri}

	g := sampleGame()
	g.IsStarted = true
	repo.games["g1"] = g

	// FederationIDCache を Redis ベースで構築し、session ↔ game を事前登録する。
	fedCache := corereversi.NewFederationIDCache(apiReversiRedis.Client)
	fedCache.Set(ctx, "sess-1", "g1")

	h.SetFederation("https://example.com", d, fedCache, userRepo)

	rec := post(h.Surrender, `{"gameId":"g1"}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Leave アクティビティが送信されたことを確認
	assert.Equal(t, 1, d.calls)

	// mapping が削除されたこと (Delete 経路) を確認
	_, ok := fedCache.GetSessionByGame(ctx, "g1")
	assert.False(t, ok)
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

	g := sampleGame()
	g.IsStarted = true
	repo.games["g1"] = g

	fedCache := corereversi.NewFederationIDCache(apiReversiRedis.Client)
	fedCache.Set(ctx, "sess-2", "g1")
	h.SetFederation("https://example.com", d, fedCache, userRepo)

	rec := post(h.Surrender, `{"gameId":"g1"}`, u1)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, 0, d.calls)

	// mapping は Delete 経路で消える
	_, ok := fedCache.GetSessionByGame(ctx, "g1")
	require.False(t, ok)
}
