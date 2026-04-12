package i

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TOTP フルフロー: register → done → unregister。それぞれの happy path と
// 各種エラー (NoProfile / WrongPassword / NoTempSecret / InvalidToken) を
// カバーする。

func TestTwoFARegister_Success(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	rec := postExtra(h.TwoFARegister, `{"password":"pass"}`, user)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["secret"])
	assert.Equal(t, "Misskey", resp["issuer"])

	// tempSecret がプロファイルに書き込まれている
	profile := repo.Profiles["u1"]
	require.NotNil(t, profile.TwoFactorTempSecret)
}

func TestTwoFARegister_WrongPassword(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "correct")
	rec := postExtra(h.TwoFARegister, `{"password":"wrong"}`, user)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTwoFARegister_NoProfile(t *testing.T) {
	h, repo := newExtraHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	rec := postExtra(h.TwoFARegister, `{"password":"pass"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTwoFADone_Success(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")

	// register で tempSecret を生成
	rec := postExtra(h.TwoFARegister, `{"password":"pass"}`, user)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	secret := resp["secret"].(string)

	// 現在時刻で有効な TOTP を生成
	token, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	// done で有効化。新しい挙動: 200 + body にバックアップコードを含む。
	rec2 := postExtra(h.TwoFADone, `{"token":"`+token+`"}`, user)
	require.Equal(t, http.StatusOK, rec2.Code)
	var doneResp map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &doneResp))
	codes, _ := doneResp["backupCodes"].([]any)
	require.Len(t, codes, 5, "TwoFADone should return 5 backup codes")

	profile := repo.Profiles["u1"]
	assert.True(t, profile.TwoFactorEnabled)
	require.NotNil(t, profile.TwoFactorSecret)
	assert.Equal(t, secret, *profile.TwoFactorSecret)
	assert.Nil(t, profile.TwoFactorTempSecret)
	assert.Len(t, profile.TwoFactorBackupSecret, 5)
}

func TestTwoFADone_NoTempSecret(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	// register を飛ばして done を呼ぶ
	rec := postExtra(h.TwoFADone, `{"token":"123456"}`, user)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	_ = repo
}

func TestTwoFADone_InvalidToken(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	// tempSecret を手動でセット
	tempSecret := "JBSWY3DPEHPK3PXP"
	repo.Profiles["u1"].TwoFactorTempSecret = &tempSecret

	rec := postExtra(h.TwoFADone, `{"token":"000000"}`, user)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTwoFAUnregister_Success(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "pass")
	// 事前に 2FA を有効化状態にする
	secret := "JBSWY3DPEHPK3PXP"
	repo.Profiles["u1"].TwoFactorSecret = &secret
	repo.Profiles["u1"].TwoFactorEnabled = true

	rec := postExtra(h.TwoFAUnregister, `{"password":"pass"}`, user)
	require.Equal(t, http.StatusNoContent, rec.Code)

	profile := repo.Profiles["u1"]
	assert.False(t, profile.TwoFactorEnabled)
	assert.Nil(t, profile.TwoFactorSecret)
}

func TestTwoFAUnregister_WrongPassword(t *testing.T) {
	h, repo := newExtraHandler(t)
	user := setupUserWithPassword(repo, "u1", "correct")
	rec := postExtra(h.TwoFAUnregister, `{"password":"wrong"}`, user)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTwoFAUnregister_NoProfile(t *testing.T) {
	h, repo := newExtraHandler(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	rec := postExtra(h.TwoFAUnregister, `{"password":"pass"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
