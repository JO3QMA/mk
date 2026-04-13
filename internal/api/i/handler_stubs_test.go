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
