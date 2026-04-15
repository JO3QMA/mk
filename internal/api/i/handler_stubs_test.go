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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
)

var stubUser = &model.User{ID: "u1"}

func hashPassword(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	return string(h)
}

func TestApps(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.Apps, `{}`, stubUser).Code)
}
func TestAuthorizedApps(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.AuthorizedApps, `{}`, stubUser).Code)
}
func TestSigninHistory(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.SigninHistory, `{}`, stubUser).Code)
}
func TestRevokeToken(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusNoContent, postExtra(h.RevokeToken, `{}`, stubUser).Code)
}
func TestUpdateEmail_WrongPassword(t *testing.T) {
	h, repo := newExtraHandler(t)
	pwd := hashPassword("secret")
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Password: &pwd}
	// パスワードが間違っているので 403
	assert.Equal(t, http.StatusForbidden, postExtra(h.UpdateEmail, `{"password":"wrong"}`, stubUser).Code)
}

func TestUpdateEmail_ClearEmail(t *testing.T) {
	h, repo := newExtraHandler(t)
	pwd := hashPassword("secret")
	email := "old@example.com"
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Password: &pwd, Email: &email}
	// email を null にセット��てクリア
	rec := postExtra(h.UpdateEmail, `{"password":"secret","email":null}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdateEmail_SetNewEmail(t *testing.T) {
	h, repo := newExtraHandler(t)
	pwd := hashPassword("secret")
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Password: &pwd}
	rec := postExtra(h.UpdateEmail, `{"password":"secret","email":"new@example.com"}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	// emailVerifyCode が生成されている
	p := repo.Profiles["u1"]
	assert.NotNil(t, p.EmailVerifyCode)
}

func TestUpdateEmail_NoProfile(t *testing.T) {
	h, _ := newExtraHandler(t)
	// profile がない → 500
	assert.Equal(t, http.StatusInternalServerError, postExtra(h.UpdateEmail, `{"password":"x"}`, stubUser).Code)
}
func TestMoveAccount(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusNoContent, postExtra(h.Move, `{}`, stubUser).Code)
}
func TestTwoFARegister_NoPassword(t *testing.T) {
	h, _ := newExtraHandler(t)
	// パスワード未指定 → BadRequest
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFARegister, `{}`, stubUser).Code)
}
func TestTwoFADone_NoToken(t *testing.T) {
	h, _ := newExtraHandler(t)
	// トークン未指定 → BadRequest
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFADone, `{}`, stubUser).Code)
}
func TestTwoFAUnregister_NoPassword(t *testing.T) {
	h, _ := newExtraHandler(t)
	// パスワード未指定 → BadRequest
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFAUnregister, `{}`, stubUser).Code)
}

// 5 つの WebAuthn handler は実装後はパラメータ必須なので、空 body で 400 を返す。
// password 等を渡したケースでの正常系は handler_2fa_flow_test.go に別途追加する。
func TestTwoFARegisterKey(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFARegisterKey, `{}`, stubUser).Code)
}
func TestTwoFAKeyDone(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFAKeyDone, `{}`, stubUser).Code)
}
func TestTwoFARemoveKey(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFARemoveKey, `{}`, stubUser).Code)
}
func TestTwoFAUpdateKey(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFAUpdateKey, `{}`, stubUser).Code)
}
func TestTwoFAPasswordLess(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusBadRequest, postExtra(h.TwoFAPasswordLess, `{}`, stubUser).Code)
}
func TestGalleryLikes(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.GalleryLikes, `{}`, stubUser).Code)
}
func TestGalleryPostsI(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.GalleryPosts, `{}`, stubUser).Code)
}
func TestPageLikes(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.PageLikes, `{}`, stubUser).Code)
}
func TestRegistryGetDetail(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.RegistryGetDetail, `{}`, stubUser).Code)
}
func TestRegistryKeys(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.RegistryKeys, `{}`, stubUser).Code)
}
func TestRegistryScopesWithDomain(t *testing.T) {
	h, _ := newExtraHandler(t)
	assert.Equal(t, http.StatusOK, postExtra(h.RegistryScopesWithDomain, `{}`, stubUser).Code)
}

func TestSigninHistory_WithData(t *testing.T) {
	h, _ := newExtraHandler(t)
	signinRepo := testutil.NewMockSigninRepository()
	h.SetSigninRepo(signinRepo)

	idGen, _ := id.NewGenerator("aidx")
	now := time.Now()
	signinRepo.Signins = append(signinRepo.Signins, &model.Signin{
		ID:      idGen.Generate(now),
		UserID:  "u1",
		IP:      "192.168.1.1",
		Headers: datatypes.JSON(`{"User-Agent":["test"]}`),
		Success: true,
	})

	rec := postExtra(h.SigninHistory, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, "192.168.1.1", resp[0]["ip"])
	assert.Equal(t, true, resp[0]["success"])
	assert.NotEmpty(t, resp[0]["createdAt"])
}

func TestSigninHistory_Empty(t *testing.T) {
	h, _ := newExtraHandler(t)
	signinRepo := testutil.NewMockSigninRepository()
	h.SetSigninRepo(signinRepo)

	rec := postExtra(h.SigninHistory, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 0)
}

func TestSigninHistory_WithLimit(t *testing.T) {
	h, _ := newExtraHandler(t)
	signinRepo := testutil.NewMockSigninRepository()
	h.SetSigninRepo(signinRepo)

	idGen, _ := id.NewGenerator("aidx")
	for i := 0; i < 5; i++ {
		signinRepo.Signins = append(signinRepo.Signins, &model.Signin{
			ID:      idGen.Generate(time.Now().Add(time.Duration(i) * time.Millisecond)),
			UserID:  "u1",
			IP:      "1.2.3.4",
			Headers: datatypes.JSON(`{}`),
			Success: true,
		})
	}

	rec := postExtra(h.SigninHistory, `{"limit":2}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)
}

func TestSigninHistory_NoRepo(t *testing.T) {
	h, _ := newExtraHandler(t)
	// signinRepoが未設定の場合は空配列を返す
	rec := postExtra(h.SigninHistory, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
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

// stubGalleryRepo implements i.GalleryRepository.
type stubGalleryRepo struct {
	posts []*model.GalleryPost
	likes []*model.GalleryLike
	err   error
}

func (s *stubGalleryRepo) ListByUser(_ string, _, _ int) ([]*model.GalleryPost, error) {
	return s.posts, s.err
}
func (s *stubGalleryRepo) ListLikesByUser(_ string, _, _ int) ([]*model.GalleryLike, error) {
	return s.likes, s.err
}

// --- P4-6 (#166): i/gallery/* ---

func TestGalleryPosts_WithRepo(t *testing.T) {
	h, _ := newExtraHandler(t)
	idGen, _ := id.NewGenerator("aidx")
	postID := idGen.Generate(time.Now())
	h.SetGalleryRepo(&stubGalleryRepo{posts: []*model.GalleryPost{
		{ID: postID, Title: "t", UserID: stubUser.ID},
	}})
	rec := postExtra(h.GalleryPosts, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, postID, got[0]["id"])
}

func TestGalleryLikes_WithRepo(t *testing.T) {
	h, _ := newExtraHandler(t)
	h.SetGalleryRepo(&stubGalleryRepo{likes: []*model.GalleryLike{
		{ID: "l1", PostID: "p1", UserID: stubUser.ID},
	}})
	rec := postExtra(h.GalleryLikes, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, "p1", got[0]["postId"])
}

// --- P4-6 (#166): i/page-likes ---

func TestPageLikes_WithRepo(t *testing.T) {
	h, _ := newExtraHandler(t)
	pageLike := testutil.NewMockPageLikeRepository()
	require.NoError(t, pageLike.Create(&model.PageLike{ID: "pl1", UserID: stubUser.ID, PageID: "pg1"}))
	h.SetPageLikeRepo(pageLike)
	rec := postExtra(h.PageLikes, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, "pg1", got[0]["pageId"])
}

// --- P4-6 (#166): i/registry/* ---

func TestRegistryGetDetail_Found(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	require.NoError(t, reg.Set(&model.RegistryItem{
		ID: "r1", UserID: stubUser.ID, Key: "theme", Value: datatypes.JSON(`"dark"`),
	}))
	h.SetRegistryRepo(reg)
	rec := postExtra(h.RegistryGetDetail, `{"key":"theme"}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRegistryGetDetail_NotFound(t *testing.T) {
	h, _ := newExtraHandler(t)
	h.SetRegistryRepo(testutil.NewMockRegistryRepository())
	rec := postExtra(h.RegistryGetDetail, `{"key":"ghost"}`, stubUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRegistryKeys_ReturnsKeysArray(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r1", UserID: stubUser.ID, Key: "a", Value: datatypes.JSON(`"x"`)}))
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r2", UserID: stubUser.ID, Key: "b", Value: datatypes.JSON(`"y"`)}))
	h.SetRegistryRepo(reg)
	rec := postExtra(h.RegistryKeys, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestVerifyEmail_InvalidCode(t *testing.T) {
	h, _ := newExtraHandler(t)
	rec := postExtra(h.VerifyEmail, `{"code":"nonexistent"}`, stubUser)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestVerifyEmail_EmptyCodeRejected(t *testing.T) {
	h, _ := newExtraHandler(t)
	rec := postExtra(h.VerifyEmail, `{}`, stubUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestVerifyEmail_Success(t *testing.T) {
	h, repo := newExtraHandler(t)
	code := "abc"
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", EmailVerifyCode: &code}
	rec := postExtra(h.VerifyEmail, `{"code":"abc"}`, stubUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, repo.Profiles["u1"].EmailVerified)
}

func TestRegistryScopesWithDomain_ReturnsDistinctPairs(t *testing.T) {
	h, _ := newExtraHandler(t)
	reg := testutil.NewMockRegistryRepository()
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r1", UserID: stubUser.ID, Key: "a", Value: datatypes.JSON(`"x"`), Scope: []string{"client"}}))
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r2", UserID: stubUser.ID, Key: "b", Value: datatypes.JSON(`"y"`), Scope: []string{"client"}}))
	require.NoError(t, reg.Set(&model.RegistryItem{ID: "r3", UserID: stubUser.ID, Key: "c", Value: datatypes.JSON(`"z"`), Scope: []string{"default"}}))
	h.SetRegistryRepo(reg)
	rec := postExtra(h.RegistryScopesWithDomain, `{}`, stubUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	// 2 distinct scopes, domain は nil のみ
	assert.Len(t, got, 2)
}
