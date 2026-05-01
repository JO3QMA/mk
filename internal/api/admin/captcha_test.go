package admin_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- thin smoke tests ---

func TestCaptchaCurrent(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.CaptchaCurrent, `{}`, adminUser).Code)
}

func TestCaptchaSave(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.CaptchaSave, `{}`, adminUser).Code)
}

// --- detailed tests (旧 queue_stats_test.go から #578 で移動 / #583 半分回収) ---

func TestCaptchaCurrent_WithHcaptchaEnabled(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	siteKey := "sk-hcap"
	metaRepo.Meta.EnableHcaptcha = true
	metaRepo.Meta.HcaptchaSiteKey = &siteKey
	rec := doPost(h.CaptchaCurrent, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "hcaptcha", got["provider"])
	assert.Equal(t, "sk-hcap", got["siteKey"])
}

func TestCaptchaCurrent_NoneEnabled(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.CaptchaCurrent, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Nil(t, got["provider"])
}

func TestCaptchaSave_AppliesFlags(t *testing.T) {
	h, _, metaRepo, _ := newTestHandler(t)
	rec := doPost(h.CaptchaSave,
		`{"provider":"turnstile","turnstileSiteKey":"sk","turnstileSecretKey":"ss"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.True(t, metaRepo.Meta.EnableTurnstile)
	assert.False(t, metaRepo.Meta.EnableHcaptcha)
}

func TestCaptchaSave_UnknownProvider(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.CaptchaSave, `{"provider":"garbage"}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
