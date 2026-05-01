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
	assert.Equal(t, http.StatusOK, postExtra(h.Apps, `{}`, stubUser).Code)
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
