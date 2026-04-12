package signup_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	apisignup "github.com/shiroha-a/mk/internal/api/signup"
	"github.com/shiroha-a/mk/internal/core/captcha"
	coresignup "github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTicketStore is a test double for signup.TicketStore.
type mockTicketStore struct {
	tickets  map[string]*model.RegistrationTicket // keyed by code
	markUsed map[string]string                    // ticketID → userID
	markErr  error
}

func newMockTicketStore() *mockTicketStore {
	return &mockTicketStore{
		tickets:  make(map[string]*model.RegistrationTicket),
		markUsed: make(map[string]string),
	}
}

func (m *mockTicketStore) FindByCode(code string) (*model.RegistrationTicket, error) {
	t, ok := m.tickets[code]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (m *mockTicketStore) MarkUsed(ticketID, userID string) error {
	if m.markErr != nil {
		return m.markErr
	}
	m.markUsed[ticketID] = userID
	return nil
}

func newTestHandler(t *testing.T) (*apisignup.Handler, *testutil.MockUserRepository, *testutil.MockMetaRepository) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	signupSvc := coresignup.NewService(userRepo, metaRepo, idGen)
	h := apisignup.NewHandler(signupSvc, metaRepo, idGen)
	return h, userRepo, metaRepo
}

func doPost(h func(echo.Context) error, body string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = h(c)
	return rec
}

func parseResp(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

// --- Success ---

func TestSignup_Success(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.Signup, `{"username":"alice","password":"pass1234"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := parseResp(t, rec)
	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "alice", resp["username"])
	assert.NotEmpty(t, resp["token"])
	// MeDetailedフィールドが含まれていること
	assert.Equal(t, false, resp["isAdmin"])
	assert.Equal(t, false, resp["isModerator"])
	assert.NotNil(t, resp["policies"])
	assert.Equal(t, false, resp["twoFactorEnabled"])
	assert.Equal(t, true, resp["preventAiLearning"])
}

// --- Validation ---

func TestSignup_EmptyUsername(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.Signup, `{"username":"","password":"pass"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSignup_EmptyPassword(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.Signup, `{"username":"alice","password":""}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSignup_InvalidJSON(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.Signup, `not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSignup_WhitespaceOnlyUsername(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.Signup, `{"username":"   ","password":"pass"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Username conflicts ---

func TestSignup_DuplicateUsername(t *testing.T) {
	h, userRepo, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "taken", UsernameLower: "taken"}
	rec := doPost(h.Signup, `{"username":"taken","password":"pass"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)

	resp := parseResp(t, rec)
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "USERNAME_ALREADY_EXISTS", errObj["code"])
}

func TestSignup_ReservedUsername(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.PreservedUsernames = []string{"admin", "root"}
	rec := doPost(h.Signup, `{"username":"admin","password":"pass"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	resp := parseResp(t, rec)
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "USED_USERNAME", errObj["code"])
}

// --- Meta errors ---

func TestSignup_MetaFetchError(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta = nil
	rec := doPost(h.Signup, `{"username":"alice","password":"pass"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSignup_EmailRequired(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.EmailRequiredForSignup = true
	rec := doPost(h.Signup, `{"username":"alice","password":"pass"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Registration disabled (invitation code) ---

func TestSignup_RegistrationDisabled_NoCode(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.DisableRegistration = true
	rec := doPost(h.Signup, `{"username":"alice","password":"pass"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	resp := parseResp(t, rec)
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "INVITATION_CODE_INVALID", errObj["code"])
}

func TestSignup_RegistrationDisabled_NoTicketStore(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.DisableRegistration = true
	// ticketStore未設定、codeあり → 検証不可
	rec := doPost(h.Signup, `{"username":"alice","password":"pass","invitationCode":"abc"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSignup_RegistrationDisabled_ValidCode(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.DisableRegistration = true
	store := newMockTicketStore()
	store.tickets["valid-code"] = &model.RegistrationTicket{ID: "t1", Code: "valid-code"}
	h.SetTicketStore(store)

	rec := doPost(h.Signup, `{"username":"alice","password":"pass","invitationCode":"valid-code"}`)
	assert.Equal(t, http.StatusOK, rec.Code)

	// ticketが使用済みにされたことを確認
	assert.Equal(t, store.markUsed["t1"], parseResp(t, rec)["id"])
}

func TestSignup_RegistrationDisabled_CodeNotFound(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.DisableRegistration = true
	store := newMockTicketStore()
	h.SetTicketStore(store)

	rec := doPost(h.Signup, `{"username":"alice","password":"pass","invitationCode":"bad-code"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSignup_RegistrationDisabled_UsedCode(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.DisableRegistration = true
	store := newMockTicketStore()
	usedBy := "someone"
	store.tickets["used-code"] = &model.RegistrationTicket{ID: "t2", Code: "used-code", UsedByID: &usedBy}
	h.SetTicketStore(store)

	rec := doPost(h.Signup, `{"username":"alice","password":"pass","invitationCode":"used-code"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSignup_RegistrationDisabled_ExpiredCode(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.DisableRegistration = true
	store := newMockTicketStore()
	expired := time.Now().Add(-24 * time.Hour)
	store.tickets["expired-code"] = &model.RegistrationTicket{ID: "t3", Code: "expired-code", ExpiresAt: &expired}
	h.SetTicketStore(store)

	rec := doPost(h.Signup, `{"username":"alice","password":"pass","invitationCode":"expired-code"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- CAPTCHA ---

// failingCaptchaVerifier is defined but unused because we use the real testcaptcha.
var _ captcha.Verifier = (*failingCaptchaVerifier)(nil)

type failingCaptchaVerifier struct{}

func (f *failingCaptchaVerifier) Verify(_ context.Context, _ string) error {
	return captcha.ErrVerificationFail
}

func TestSignup_CaptchaFailed(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.EnableTestcaptcha = true
	captchaSvc := captcha.NewService(metaRepo.Meta)
	h.SetCaptcha(captchaSvc)

	// testcaptcha-response が空 → 検証失敗
	rec := doPost(h.Signup, `{"username":"alice","password":"pass"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	resp := parseResp(t, rec)
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "CAPTCHA_FAILED", errObj["code"])
}

func TestSignup_CaptchaSuccess(t *testing.T) {
	h, _, metaRepo := newTestHandler(t)
	metaRepo.Meta.EnableTestcaptcha = true
	captchaSvc := captcha.NewService(metaRepo.Meta)
	h.SetCaptcha(captchaSvc)

	rec := doPost(h.Signup, `{"username":"alice","password":"pass","testcaptcha-response":"testcaptcha-passed"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Internal error from signup service ---

type failingCreateUserRepo struct {
	*testutil.MockUserRepository
}

func (r *failingCreateUserRepo) Create(_ *model.User) error {
	return assert.AnError
}

func TestSignup_InternalError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}
	idGen, _ := id.NewGenerator("aidx")
	failRepo := &failingCreateUserRepo{userRepo}
	signupSvc := coresignup.NewService(failRepo, metaRepo, idGen)
	h := apisignup.NewHandler(signupSvc, metaRepo, idGen)

	rec := doPost(h.Signup, `{"username":"alice","password":"pass"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Response shape ---

func TestSignup_ResponseShape(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doPost(h.Signup, `{"username":"carol","password":"pass1234"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := parseResp(t, rec)

	requiredFields := []string{
		"id", "username", "token", "host", "avatarUrl",
		"isBot", "isCat", "isLocked", "isSuspended", "isSilenced",
		"isAdmin", "isModerator", "followersCount", "followingCount", "notesCount",
		"createdAt", "policies", "twoFactorEnabled", "preventAiLearning",
		"publicReactions", "autoAcceptFollowed",
	}
	for _, f := range requiredFields {
		_, exists := resp[f]
		assert.True(t, exists, "missing field: %s", f)
	}
}
