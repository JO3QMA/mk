package i

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApps(t *testing.T) {
	h, _ := newExtraHandler(t)
	// accessTokenRepo 未配線時は空配列で graceful fallback
	rec := postExtra(h.Apps, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]\n", rec.Body.String())
}

// /i/apps は本家 Misskey 互換で access_token + app の JOIN を返す。
// owner-app 経由 (token.appId 指定あり) と raw token (app 無し) の混在を
// テスト。本実装化は #587 で行った (旧実装は常に空配列を返す stub)。
func TestApps_ReturnsTokensWithApp(t *testing.T) {
	h, _ := newExtraHandler(t)
	tokens := testutil.NewMockAccessTokenRepository()
	idGen, _ := id.NewGenerator("aidx")

	// owner-app 経由の token: app メタを app テーブルから引く
	appID := "app1"
	tokens.SetApp(&model.App{
		ID:          appID,
		Name:        "Misskey IDE",
		Description: "OAuth2 demo app",
		Permission:  []string{"read:account"},
	})
	tApp := idGen.Generate(time.Now())
	tokens.Tokens["h1"] = &model.AccessToken{
		ID: tApp, Hash: "h1", UserID: stubUser.ID, AppID: &appID,
	}

	// raw token (app なし): token 自身の name / permission を使う
	tRaw := idGen.Generate(time.Now())
	rawName := "personal token"
	tokens.Tokens["h2"] = &model.AccessToken{
		ID: tRaw, Hash: "h2", UserID: stubUser.ID,
		Name:       &rawName,
		Permission: []string{"write:notes"},
	}

	// 別ユーザーの token は除外
	tokens.Tokens["h3"] = &model.AccessToken{ID: "other", Hash: "h3", UserID: "other"}

	h.SetAccessTokenRepo(tokens)

	rec := postExtra(h.Apps, `{}`, stubUser)
	require.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 2)

	// default sort = id ASC のため、ID 順に並ぶ。
	byID := map[string]map[string]any{}
	for _, e := range got {
		byID[e["id"].(string)] = e
	}
	// owner-app 経由: name は app.name、permission は app.permission
	app := byID[tApp]
	assert.Equal(t, "Misskey IDE", app["name"])
	perm, _ := app["permission"].([]any)
	require.Len(t, perm, 1)
	assert.Equal(t, "read:account", perm[0])
	assert.Equal(t, "OAuth2 demo app", app["description"])

	// raw token: name は token.name、permission は token.permission
	raw := byID[tRaw]
	assert.Equal(t, "personal token", raw["name"])
	perm2, _ := raw["permission"].([]any)
	require.Len(t, perm2, 1)
	assert.Equal(t, "write:notes", perm2[0])
}

// sort=+createdAt は新しい順 (ID DESC)。
func TestApps_SortByCreatedAtDesc(t *testing.T) {
	h, _ := newExtraHandler(t)
	tokens := testutil.NewMockAccessTokenRepository()
	idGen, _ := id.NewGenerator("aidx")
	t1 := idGen.Generate(time.Now())
	t2 := idGen.Generate(time.Now().Add(time.Second))
	tokens.Tokens["h1"] = &model.AccessToken{ID: t1, Hash: "h1", UserID: stubUser.ID}
	tokens.Tokens["h2"] = &model.AccessToken{ID: t2, Hash: "h2", UserID: stubUser.ID}
	h.SetAccessTokenRepo(tokens)

	rec := postExtra(h.Apps, `{"sort":"+createdAt"}`, stubUser)
	require.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 2)
	// 新しい順 = t2 が先
	assert.Equal(t, t2, got[0]["id"])
	assert.Equal(t, t1, got[1]["id"])
}

func TestAuthorizedApps(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.AuthorizedApps, `{}`, stubUser).Code)
}

func TestRevokeToken(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusNoContent, postExtra(h.RevokeToken, `{}`, stubUser).Code)
}

// --- P4-6 (#166): i/authorized-apps ---

func TestAuthorizedApps_ReturnsOwnedTokens(t *testing.T) {
	h, _ := newExtraHandler(t)
	tokens := testutil.NewMockAccessTokenRepository()
	idGen, _ := id.NewGenerator("aidx")
	t1ID := idGen.Generate(time.Now())
	name1 := "app one"
	tokens.Tokens["h1"] = &model.AccessToken{ID: t1ID, Hash: "h1", UserID: stubUser.ID, Name: &name1}
	tokens.Tokens["h2"] = &model.AccessToken{ID: "other-user-token", Hash: "h2", UserID: "other"}
	h.SetAccessTokenRepo(tokens)

	rec := postExtra(h.AuthorizedApps, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	// 自分のトークンだけ返る
	require.Len(t, got, 1)
	assert.Equal(t, t1ID, got[0]["id"])
}

// --- P4-6 (#166): i/revoke-token ---

func TestRevokeToken_Owned(t *testing.T) {
	h, _ := newExtraHandler(t)
	tokens := testutil.NewMockAccessTokenRepository()
	tokens.Tokens["h1"] = &model.AccessToken{ID: "t1", Hash: "h1", UserID: stubUser.ID}
	h.SetAccessTokenRepo(tokens)

	rec := postExtra(h.RevokeToken, `{"tokenId":"t1"}`, stubUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, err := tokens.FindByID("t1")
	assert.Error(t, err)
}

func TestRevokeToken_ForeignToken_AccessDenied(t *testing.T) {
	h, _ := newExtraHandler(t)
	tokens := testutil.NewMockAccessTokenRepository()
	tokens.Tokens["h2"] = &model.AccessToken{ID: "t2", Hash: "h2", UserID: "other"}
	h.SetAccessTokenRepo(tokens)

	rec := postExtra(h.RevokeToken, `{"tokenId":"t2"}`, stubUser)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRevokeToken_UnknownTokenIsIdempotent(t *testing.T) {
	h, _ := newExtraHandler(t)
	h.SetAccessTokenRepo(testutil.NewMockAccessTokenRepository())
	rec := postExtra(h.RevokeToken, `{"tokenId":"ghost"}`, stubUser)
	// 存在しないなら 204 (idempotent)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRevokeToken_ByTokenHash(t *testing.T) {
	h, _ := newExtraHandler(t)
	repo := testutil.NewMockAccessTokenRepository()
	// SHA-256("mytoken") をハッシュとして登録 (map key = hash)
	hash := "1a17ea3569204d6c4114794ca73fa257457fc0612928c7bf024801659b77dba8"
	repo.Tokens[hash] = &model.AccessToken{ID: "at1", Hash: hash, UserID: stubUser.ID}
	h.SetAccessTokenRepo(repo)
	// 生tokenで失効できる
	rec := postExtra(h.RevokeToken, `{"token":"mytoken"}`, stubUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRevokeToken_NoParams(t *testing.T) {
	h, _ := newExtraHandler(t)
	h.SetAccessTokenRepo(testutil.NewMockAccessTokenRepository())
	// tokenId も token も空 → 400
	assert.Equal(t, http.StatusBadRequest, postExtra(h.RevokeToken, `{}`, stubUser).Code)
}
