package l10n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignupConfirm_Japanese(t *testing.T) {
	subject, lead, link := SignupConfirm("ja", "てすと鯖")
	assert.Equal(t, "アカウントの確認", subject)
	assert.Contains(t, lead, "てすと鯖")
	assert.Equal(t, "登録を完了", link)
}

func TestPasswordReset_English(t *testing.T) {
	subject, _, link := PasswordReset("en")
	assert.Equal(t, "Password reset", subject)
	assert.Equal(t, "Reset password", link)
}

func TestNewLogin_Japanese(t *testing.T) {
	subject, body := NewLogin("ja")
	assert.Equal(t, "ログインがありました", subject)
	assert.Contains(t, body, "新しいログインがありました")
	assert.NotContains(t, body, "There is a new login")
}

func TestModeratorInactivityWarning_LocalizedTime(t *testing.T) {
	_, bodyJa := ModeratorInactivityWarning("ja", 0, 6)
	assert.Contains(t, bodyJa, "6時間")

	_, bodyEn := ModeratorInactivityWarning("en", 2, 48)
	assert.Contains(t, bodyEn, "2 days")
}
