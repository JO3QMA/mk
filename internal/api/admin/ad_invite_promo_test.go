package admin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Ad ---------------------------------------------------------------------

func TestAdCreate_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAdRepository()
	h.SetAdRepo(repo)
	rec := doPost(h.AdCreate,
		`{"url":"https://x","imageUrl":"https://y","place":"square","memo":"m","priority":"high","ratio":2,"dayOfWeek":3,"expiresAt":1760000000000,"startsAt":1759000000000,"isSensitive":true}`,
		adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got model.Ad
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "https://x", got.URL)
	assert.Equal(t, "square", got.Place)
	assert.Equal(t, 2, got.Ratio)
	assert.True(t, got.IsSensitive)
	assert.Contains(t, repo.Ads, got.ID)
}

func TestAdCreate_MissingRequired(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAdRepo(testutil.NewMockAdRepository())
	assert.Equal(t, http.StatusBadRequest,
		doPost(h.AdCreate, `{"url":""}`, adminUser).Code)
}

func TestAdCreate_Defaults(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAdRepository()
	h.SetAdRepo(repo)
	rec := doPost(h.AdCreate, `{"url":"https://x","imageUrl":"https://y"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got model.Ad
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "square", got.Place)
	assert.Equal(t, "middle", got.Priority)
	assert.Equal(t, 1, got.Ratio)
}

func TestAdCreate_RepoError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAdRepository()
	repo.CreateErr = assertError{}
	h.SetAdRepo(repo)
	rec := doPost(h.AdCreate, `{"url":"https://x","imageUrl":"https://y"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdList_Paginated(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAdRepository()
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("a%02d", i)
		require.NoError(t, repo.Create(&model.Ad{
			ID: id, URL: "u", ImageURL: "i", Place: "square", Priority: "middle", Ratio: 1,
			StartsAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(time.Hour),
		}))
	}
	h.SetAdRepo(repo)

	rec := doPost(h.AdList, `{"limit":3,"offset":0}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.Ad
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 3)
	// id DESC
	assert.Equal(t, "a04", rows[0].ID)
}

func TestAdUpdate_PartialFields(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAdRepository()
	require.NoError(t, repo.Create(&model.Ad{
		ID: "a1", URL: "old", Place: "square", Priority: "middle", Ratio: 1, ImageURL: "i",
	}))
	h.SetAdRepo(repo)

	rec := doPost(h.AdUpdate,
		`{"id":"a1","url":"new","priority":"high"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "new", repo.Ads["a1"].URL)
	assert.Equal(t, "high", repo.Ads["a1"].Priority)
}

func TestAdUpdate_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAdRepo(testutil.NewMockAdRepository())
	assert.Equal(t, http.StatusNotFound,
		doPost(h.AdUpdate, `{"id":"missing","url":"x"}`, adminUser).Code)
}

func TestAdUpdate_MissingID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAdRepo(testutil.NewMockAdRepository())
	assert.Equal(t, http.StatusBadRequest,
		doPost(h.AdUpdate, `{}`, adminUser).Code)
}

// Explicit expiresAt:0 in Update はクライアントが明示的に 1970-01-01 を指定した
// ケースとして扱う (nil なら変更なし)。handler 側の millisOrNow 適用で now に
// 読み替えてしまう regression を防ぐ。
func TestAdUpdate_ExplicitZeroExpiresAtStays1970(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAdRepository()
	require.NoError(t, repo.Create(&model.Ad{
		ID: "a1", URL: "u", Place: "square", Priority: "middle", Ratio: 1, ImageURL: "i",
		StartsAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}))
	h.SetAdRepo(repo)

	rec := doPost(h.AdUpdate, `{"id":"a1","expiresAt":0}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	// time.UnixMilli(0) は UTC 1970-01-01。time.Now() と大きく異なること。
	got := repo.Ads["a1"].ExpiresAt
	assert.Equal(t, int64(0), got.UnixMilli(),
		"explicit expiresAt:0 should map to UNIX epoch, not time.Now()")
}

func TestAdDelete_WithRepo(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAdRepository()
	require.NoError(t, repo.Create(&model.Ad{ID: "a1"}))
	h.SetAdRepo(repo)

	rec := doPost(h.AdDelete, `{"id":"a1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotContains(t, repo.Ads, "a1")
}

// --- AvatarDecorations ------------------------------------------------------

func TestAvatarDecorationsCreate_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAvatarDecorationRepository()
	h.SetAvatarDecorationRepo(repo)
	rec := doPost(h.AvatarDecorationsCreate,
		`{"name":"deco","description":"d","url":"https://i","roleIdsThatCanBeUsedThisDecoration":["r1","r2"]}`,
		adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var got model.AvatarDecoration
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "deco", got.Name)
	assert.Equal(t, []string(got.RoleIDs), []string{"r1", "r2"})
}

func TestAvatarDecorationsCreate_MissingName(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAvatarDecorationRepo(testutil.NewMockAvatarDecorationRepository())
	assert.Equal(t, http.StatusBadRequest,
		doPost(h.AvatarDecorationsCreate, `{"url":"https://i"}`, adminUser).Code)
}

func TestAvatarDecorationsList_WithRepo(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAvatarDecorationRepository()
	require.NoError(t, repo.Create(&model.AvatarDecoration{ID: "d1", Name: "a"}))
	require.NoError(t, repo.Create(&model.AvatarDecoration{ID: "d2", Name: "b"}))
	h.SetAvatarDecorationRepo(repo)
	rec := doPost(h.AvatarDecorationsList, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.AvatarDecoration
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 2)
}

func TestAvatarDecorationsUpdate_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAvatarDecorationRepository()
	require.NoError(t, repo.Create(&model.AvatarDecoration{ID: "d1", Name: "old", URL: "u"}))
	h.SetAvatarDecorationRepo(repo)
	rec := doPost(h.AvatarDecorationsUpdate,
		`{"id":"d1","name":"new","roleIdsThatCanBeUsedThisDecoration":["r"]}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "new", repo.Decorations["d1"].Name)
	assert.Equal(t, []string(repo.Decorations["d1"].RoleIDs), []string{"r"})
}

func TestAvatarDecorationsUpdate_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAvatarDecorationRepo(testutil.NewMockAvatarDecorationRepository())
	assert.Equal(t, http.StatusNotFound,
		doPost(h.AvatarDecorationsUpdate, `{"id":"missing","name":"x"}`, adminUser).Code)
}

func TestAvatarDecorationsUpdate_MissingID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAvatarDecorationRepo(testutil.NewMockAvatarDecorationRepository())
	assert.Equal(t, http.StatusBadRequest,
		doPost(h.AvatarDecorationsUpdate, `{}`, adminUser).Code)
}

func TestAvatarDecorationsDelete_WithRepo(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockAvatarDecorationRepository()
	require.NoError(t, repo.Create(&model.AvatarDecoration{ID: "d1"}))
	h.SetAvatarDecorationRepo(repo)
	rec := doPost(h.AvatarDecorationsDelete, `{"id":"d1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotContains(t, repo.Decorations, "d1")
}

// --- Invite -----------------------------------------------------------------

func TestInviteCreate_Default(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockRegistrationTicketRepository()
	h.SetInviteRepo(repo)
	rec := doPost(h.InviteCreate, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.RegistrationTicket
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 1)
	assert.NotEmpty(t, rows[0].Code)
}

func TestInviteCreate_MultipleCount(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockRegistrationTicketRepository()
	h.SetInviteRepo(repo)
	rec := doPost(h.InviteCreate, `{"count":5}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.RegistrationTicket
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 5)
}

func TestInviteCreate_CountClampedToMax(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockRegistrationTicketRepository()
	h.SetInviteRepo(repo)
	rec := doPost(h.InviteCreate, `{"count":250}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.RegistrationTicket
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 100)
}

func TestInviteCreate_InvalidExpiresAt(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetInviteRepo(testutil.NewMockRegistrationTicketRepository())
	rec := doPost(h.InviteCreate, `{"expiresAt":"not-a-date"}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInviteList_FilterUnused(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	repo := testutil.NewMockRegistrationTicketRepository()
	usedBy := "u1"
	require.NoError(t, repo.Create(&model.RegistrationTicket{ID: "t1", Code: "c1"}))
	require.NoError(t, repo.Create(&model.RegistrationTicket{ID: "t2", Code: "c2", UsedByID: &usedBy}))
	h.SetInviteRepo(repo)

	rec := doPost(h.InviteList, `{"type":"unused"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	var rows []model.RegistrationTicket
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 1)
	assert.Equal(t, "t1", rows[0].ID)
}

// --- Promo ------------------------------------------------------------------

// stubNoteFinder satisfies admin.NoteFinder with just FindByID.
type stubNoteFinder struct {
	note *model.Note
	err  error
}

func (s *stubNoteFinder) FindByID(_ string) (*model.Note, error) { return s.note, s.err }

type stubPromoRepo struct {
	created  *model.PromoNote
	exists   bool
	existErr error
}

func (s *stubPromoRepo) Create(p *model.PromoNote) error {
	s.created = p
	return nil
}
func (s *stubPromoRepo) ListActive(_ time.Time) ([]*model.PromoNote, error) { return nil, nil }
func (s *stubPromoRepo) Exists(_ string) (bool, error)                      { return s.exists, s.existErr }

func TestPromoCreate_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	promo := &stubPromoRepo{}
	h.SetPromoNoteRepo(promo)
	note := &model.Note{ID: "n1", UserID: "authorU"}
	h.SetNoteFinder(&stubNoteFinder{note: note})

	expires := time.Now().Add(24 * time.Hour).UnixMilli()
	body := fmt.Sprintf(`{"noteId":"n1","expiresAt":%d}`, expires)
	rec := doPost(h.PromoCreate, body, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.NotNil(t, promo.created)
	assert.Equal(t, "n1", promo.created.NoteID)
	assert.Equal(t, "authorU", promo.created.UserID)
}

func TestPromoCreate_MissingNoteID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetPromoNoteRepo(&stubPromoRepo{})
	h.SetNoteFinder(&stubNoteFinder{})
	assert.Equal(t, http.StatusBadRequest,
		doPost(h.PromoCreate, `{}`, adminUser).Code)
}

func TestPromoCreate_NoSuchNote(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetPromoNoteRepo(&stubPromoRepo{})
	h.SetNoteFinder(&stubNoteFinder{err: assertError{}})
	rec := doPost(h.PromoCreate, `{"noteId":"missing","expiresAt":1}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPromoCreate_AlreadyPromoted(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetPromoNoteRepo(&stubPromoRepo{exists: true})
	h.SetNoteFinder(&stubNoteFinder{note: &model.Note{ID: "n1", UserID: "u"}})
	rec := doPost(h.PromoCreate, `{"noteId":"n1","expiresAt":1}`, adminUser)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	errField, ok := body["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ALREADY_PROMOTED", errField["code"])
}
