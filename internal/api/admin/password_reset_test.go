package admin_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPasswordResetRepo struct {
	created *model.PasswordResetRequest
	err     error
}

func (s *stubPasswordResetRepo) Create(req *model.PasswordResetRequest) error {
	if s.err != nil {
		return s.err
	}
	s.created = req
	return nil
}

func (s *stubPasswordResetRepo) FindByToken(_ string) (*model.PasswordResetRequest, error) {
	return s.created, nil
}

func (s *stubPasswordResetRepo) Delete(_ string) error { return nil }

type capturedEmail struct {
	mu      sync.Mutex
	to      string
	subject string
	body    string
	called  int
}

func (c *capturedEmail) send(to, subject, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.called++
	c.to = to
	c.subject = subject
	c.body = body
}

func (c *capturedEmail) snapshot() capturedEmail {
	c.mu.Lock()
	defer c.mu.Unlock()
	return capturedEmail{to: c.to, subject: c.subject, body: c.body, called: c.called}
}

func TestResetPasswordAdmin_Empty(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, doPost(h.ResetPassword, `{}`, adminUser).Code)
}

func TestResetPasswordAdmin_Success(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "testuser"}
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1"}
	rec := doPost(h.ResetPassword, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "password")
}

func TestResetPasswordAdmin_VerifiedEmailSendsResetLink(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "testuser"}
	email := "alice@example.com"
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Email: &email, EmailVerified: true}

	resetRepo := &stubPasswordResetRepo{}
	captured := &capturedEmail{}
	h.SetPasswordResetRepo(resetRepo)
	h.SetEmailSender(captured.send)
	h.SetServerURL("https://example.com")

	rec := doPost(h.ResetPassword, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"sent":true`)
	assert.NotContains(t, rec.Body.String(), "password")

	// Email は goroutine で送られるので少し待つ
	deadline := time.Now().Add(500 * time.Millisecond)
	var snap capturedEmail
	for time.Now().Before(deadline) {
		snap = captured.snapshot()
		if snap.called > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 1, snap.called)
	assert.Equal(t, email, snap.to)
	assert.Equal(t, "Password reset", snap.subject)
	require.NotNil(t, resetRepo.created, "reset token should be persisted")
	assert.Contains(t, snap.body, resetRepo.created.Token,
		"body should contain the persisted token in the reset link")
	assert.Contains(t, snap.body, "https://example.com/reset-password/")
}

func TestResetPasswordAdmin_UnverifiedEmailFallsBack(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1"}
	email := "alice@example.com"
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Email: &email, EmailVerified: false}

	h.SetPasswordResetRepo(&stubPasswordResetRepo{})
	captured := &capturedEmail{}
	h.SetEmailSender(captured.send)

	rec := doPost(h.ResetPassword, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "password")
	assert.Equal(t, 0, captured.called, "unverified email must not trigger sender")
}

func TestResetPasswordAdmin_MissingEmailFallsBack(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1"}
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1"} // email = nil
	h.SetPasswordResetRepo(&stubPasswordResetRepo{})
	h.SetEmailSender(func(string, string, string) {})

	rec := doPost(h.ResetPassword, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "password")
}

func TestResetPasswordAdmin_NoRepoFallsBack(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1"}
	email := "a@b.c"
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Email: &email, EmailVerified: true}
	h.SetEmailSender(func(string, string, string) {})
	// Repo 未設定 → fallback

	rec := doPost(h.ResetPassword, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "password")
}

func TestResetPasswordAdmin_ResetRepoErrorFallsBack(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1"}
	email := "a@b.c"
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Email: &email, EmailVerified: true}
	h.SetPasswordResetRepo(&stubPasswordResetRepo{err: assertError{}})
	h.SetEmailSender(func(string, string, string) {})

	rec := doPost(h.ResetPassword, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "password",
		"reset repo failure should fall back to legacy temp-password path")
}
