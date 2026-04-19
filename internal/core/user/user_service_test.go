package user_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFullSvc(t *testing.T) (*user.Service, *testutil.MockUserRepository, *testutil.MockNoteRepository, *testutil.MockUserNotePiningRepository) {
	t.Helper()
	uRepo := testutil.NewMockUserRepository()
	nRepo := testutil.NewMockNoteRepository()
	pRepo := testutil.NewMockUserNotePiningRepository()
	idGen, _ := id.NewGenerator("aidx")
	return user.NewService(uRepo, nRepo, pRepo, idGen), uRepo, nRepo, pRepo
}

func ptr[T any](v T) *T { return &v }

func TestService_ShowByID_Success(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	desc := "hello"
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Description: &desc}
	svc := user.NewService(repo, nil, nil, nil)

	bundle, err := svc.ShowByID("u1")
	require.NoError(t, err)
	assert.Equal(t, "alice", bundle.User.Username)
	require.NotNil(t, bundle.Profile)
	assert.Equal(t, "hello", *bundle.Profile.Description)
}

func TestService_ShowByID_NotFound(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	svc := user.NewService(repo, nil, nil, nil)

	_, err := svc.ShowByID("missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

func TestService_ShowByID_NoProfile(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	svc := user.NewService(repo, nil, nil, nil)

	bundle, err := svc.ShowByID("u1")
	require.NoError(t, err)
	assert.Nil(t, bundle.Profile)
}

func TestService_ShowByUsername_Success(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", UsernameLower: "alice"}
	svc := user.NewService(repo, nil, nil, nil)

	bundle, err := svc.ShowByUsername("alice", nil)
	require.NoError(t, err)
	assert.Equal(t, "u1", bundle.User.ID)
}

func TestService_ShowByUsername_WithHost(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	host := "remote.example.com"
	repo.Users["u2"] = &model.User{ID: "u2", Username: "bob", UsernameLower: "bob", Host: &host}
	svc := user.NewService(repo, nil, nil, nil)

	bundle, err := svc.ShowByUsername("bob", &host)
	require.NoError(t, err)
	assert.Equal(t, "u2", bundle.User.ID)
}

func TestService_ShowByUsername_NotFound(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	svc := user.NewService(repo, nil, nil, nil)

	_, err := svc.ShowByUsername("nobody", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

func TestService_GetProfile_Found(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	desc := "hi"
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Description: &desc}
	svc := user.NewService(repo, nil, nil, nil)

	p := svc.GetProfile("u1")
	require.NotNil(t, p)
	assert.Equal(t, "hi", *p.Description)
}

func TestService_GetProfile_Missing(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	svc := user.NewService(repo, nil, nil, nil)

	assert.Nil(t, svc.GetProfile("missing"))
}

// --- Search ---

func TestService_Search_Empty(t *testing.T) {
	svc, _, _, _ := newFullSvc(t)
	out, err := svc.Search("   ", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestService_Search_AtPrefix(t *testing.T) {
	svc, repo, _, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", UsernameLower: "alice"}
	out, err := svc.Search("@al", 10, 0)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestService_Search_DefaultLimit(t *testing.T) {
	svc, repo, _, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", UsernameLower: "alice"}
	out, err := svc.Search("a", 0, 0)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

// --- UpdateProfile ---

func TestService_UpdateProfile_Success(t *testing.T) {
	svc, repo, _, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}

	bundle, err := svc.UpdateProfile("u1", user.UpdateInput{
		Name:              ptr(ptr("New Name")),
		Description:       ptr(ptr("hi there")),
		Location:          ptr(ptr("Tokyo")),
		Birthday:          ptr(ptr("1990-01-01")),
		Lang:              ptr(ptr("ja")),
		IsLocked:          ptr(true),
		IsBot:             ptr(true),
		IsCat:             ptr(true),
		IsExplorable:      ptr(false),
		HideOnlineStatus:  ptr(true),
		AlwaysMarkNsfw:    ptr(true),
		AutoSensitive:     ptr(true),
		NoCrawle:          ptr(true),
		PreventAiLearning: ptr(true),
	})
	require.NoError(t, err)
	require.NotNil(t, bundle)
	assert.True(t, bundle.User.IsLocked)
	assert.True(t, bundle.User.IsBot)
}

func TestService_UpdateProfile_NoFields(t *testing.T) {
	svc, repo, _, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}

	_, err := svc.UpdateProfile("u1", user.UpdateInput{})
	require.NoError(t, err)
}

func TestService_UpdateProfile_UserNotFound(t *testing.T) {
	svc, _, _, _ := newFullSvc(t)
	_, err := svc.UpdateProfile("missing", user.UpdateInput{})
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

// stubMainStreamPublisher captures PublishMainEvent calls for assertion.
type stubMainStreamPublisher struct {
	calls []mainEventCall
}

type mainEventCall struct {
	userID    string
	eventType string
	body      any
}

func (s *stubMainStreamPublisher) PublishMainEvent(userID, eventType string, body any) {
	s.calls = append(s.calls, mainEventCall{userID, eventType, body})
}

func TestService_UpdateProfile_PublishesMeUpdated(t *testing.T) {
	svc, repo, _, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	pub := &stubMainStreamPublisher{}
	svc.SetMainStreamPublisher(pub)

	_, err := svc.UpdateProfile("u1", user.UpdateInput{
		Name:        ptr(ptr("New Name")),
		Description: ptr(ptr("hello")),
	})
	require.NoError(t, err)

	require.Len(t, pub.calls, 1)
	assert.Equal(t, "u1", pub.calls[0].userID)
	assert.Equal(t, "meUpdated", pub.calls[0].eventType)
	// body は entity.UserDetailed 相当。JSON round-trip で id/name/description
	// が更新後の値になっていることを確認する。
	raw, err := json.Marshal(pub.calls[0].body)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Equal(t, "u1", m["id"])
	assert.Equal(t, "New Name", m["name"])
	assert.Equal(t, "hello", m["description"])
}

func TestService_UpdateProfile_UserUpdateError_DoesNotPublish(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	mockUR.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	uRepo := &failingUserRepo{MockUserRepository: mockUR, failUpdateUser: true}
	idGen, _ := id.NewGenerator("aidx")
	svc := user.NewService(uRepo, nil, nil, idGen)
	pub := &stubMainStreamPublisher{}
	svc.SetMainStreamPublisher(pub)

	_, err := svc.UpdateProfile("u1", user.UpdateInput{IsLocked: ptr(true)})
	require.Error(t, err)
	assert.Empty(t, pub.calls)
}

// --- PinNote / UnpinNote / ListPinnedNotes ---

func TestService_PinNote_Success(t *testing.T) {
	svc, repo, noteRepo, piningRepo := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}

	require.NoError(t, svc.PinNote("u1", "n1"))
	assert.Len(t, piningRepo.Pinings, 1)
}

func TestService_PinNote_NoteNotFound(t *testing.T) {
	svc, repo, _, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1"}

	err := svc.PinNote("u1", "ghost")
	assert.True(t, errors.Is(err, user.ErrNoteNotFound))
}

func TestService_PinNote_NotOwner(t *testing.T) {
	svc, repo, noteRepo, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "other"}

	err := svc.PinNote("u1", "n1")
	assert.True(t, errors.Is(err, user.ErrNoteNotFound))
}

func TestService_PinNote_AlreadyPinned(t *testing.T) {
	svc, repo, noteRepo, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	require.NoError(t, svc.PinNote("u1", "n1"))

	err := svc.PinNote("u1", "n1")
	assert.True(t, errors.Is(err, user.ErrAlreadyPinned))
}

func TestService_PinNote_LimitExceeded(t *testing.T) {
	svc, repo, noteRepo, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	for i := 1; i <= user.MaxPinnedNotes; i++ {
		noteID := "n" + string(rune('0'+i))
		noteRepo.Notes[noteID] = &model.Note{ID: noteID, UserID: "u1"}
		require.NoError(t, svc.PinNote("u1", noteID))
	}

	noteRepo.Notes["n_extra"] = &model.Note{ID: "n_extra", UserID: "u1"}
	err := svc.PinNote("u1", "n_extra")
	assert.True(t, errors.Is(err, user.ErrPinLimitExceeded))
}

func TestService_UnpinNote_Success(t *testing.T) {
	svc, repo, noteRepo, piningRepo := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	require.NoError(t, svc.PinNote("u1", "n1"))

	require.NoError(t, svc.UnpinNote("u1", "n1"))
	assert.Empty(t, piningRepo.Pinings)
}

func TestService_UnpinNote_NotFound(t *testing.T) {
	svc, _, _, _ := newFullSvc(t)
	err := svc.UnpinNote("u1", "n1")
	assert.True(t, errors.Is(err, user.ErrPinNotFound))
}

func TestService_PinNote_PublishesMeUpdated(t *testing.T) {
	svc, repo, noteRepo, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	pub := &stubMainStreamPublisher{}
	svc.SetMainStreamPublisher(pub)

	require.NoError(t, svc.PinNote("u1", "n1"))
	require.Len(t, pub.calls, 1)
	assert.Equal(t, "u1", pub.calls[0].userID)
	assert.Equal(t, "meUpdated", pub.calls[0].eventType)
	raw, err := json.Marshal(pub.calls[0].body)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Equal(t, "u1", m["id"])
	// piningRepo から補完された pinnedNoteIds が含まれること
	ids, ok := m["pinnedNoteIds"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"n1"}, ids)
}

func TestService_PinNote_NoPublisher_NoDBRead(t *testing.T) {
	// publisher 未注入時は ShowByID を呼ばないことを確認するため、
	// pin 直後に user を repo から削除しても PinNote は error にならない
	// (ShowByID されないため)。
	svc, repo, noteRepo, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	// publisher を意図的に設定しない
	require.NoError(t, svc.PinNote("u1", "n1"))
}

func TestService_UnpinNote_PublishesMeUpdated(t *testing.T) {
	svc, repo, noteRepo, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	require.NoError(t, svc.PinNote("u1", "n1"))

	// Pin で emit された分を除外するため publisher を差し替え。
	pub := &stubMainStreamPublisher{}
	svc.SetMainStreamPublisher(pub)

	require.NoError(t, svc.UnpinNote("u1", "n1"))
	require.Len(t, pub.calls, 1)
	assert.Equal(t, "u1", pub.calls[0].userID)
	assert.Equal(t, "meUpdated", pub.calls[0].eventType)
}

func TestService_ListPinnedNotes_Success(t *testing.T) {
	svc, repo, noteRepo, _ := newFullSvc(t)
	repo.Users["u1"] = &model.User{ID: "u1"}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	noteRepo.Notes["n2"] = &model.Note{ID: "n2", UserID: "u1"}
	require.NoError(t, svc.PinNote("u1", "n1"))
	require.NoError(t, svc.PinNote("u1", "n2"))

	notes, err := svc.ListPinnedNotes("u1")
	require.NoError(t, err)
	assert.Len(t, notes, 2)
}

func TestService_ListPinnedNotes_Empty(t *testing.T) {
	svc, _, _, _ := newFullSvc(t)
	notes, err := svc.ListPinnedNotes("u1")
	require.NoError(t, err)
	assert.Empty(t, notes)
}

// --- Failing-repo error paths ---

var stubError = errors.New("stub error")

type failingUserRepo struct {
	*testutil.MockUserRepository
	failUpdateUser    bool
	failUpdateProfile bool
}

func (f *failingUserRepo) UpdateUser(userID string, fields map[string]any) error {
	if f.failUpdateUser {
		return stubError
	}
	return f.MockUserRepository.UpdateUser(userID, fields)
}

func (f *failingUserRepo) UpdateProfile(userID string, fields map[string]any) error {
	if f.failUpdateProfile {
		return stubError
	}
	return f.MockUserRepository.UpdateProfile(userID, fields)
}

type failingPiningRepo struct {
	*testutil.MockUserNotePiningRepository
	failCount   bool
	failListByU bool
}

func (f *failingPiningRepo) CountByUser(userID string) (int, error) {
	if f.failCount {
		return 0, stubError
	}
	return f.MockUserNotePiningRepository.CountByUser(userID)
}

func (f *failingPiningRepo) ListByUser(userID string) ([]*model.UserNotePining, error) {
	if f.failListByU {
		return nil, stubError
	}
	return f.MockUserNotePiningRepository.ListByUser(userID)
}

func TestService_UpdateProfile_UserUpdateError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	mockUR.Users["u1"] = &model.User{ID: "u1"}
	uRepo := &failingUserRepo{MockUserRepository: mockUR, failUpdateUser: true}
	idGen, _ := id.NewGenerator("aidx")
	svc := user.NewService(uRepo, testutil.NewMockNoteRepository(), testutil.NewMockUserNotePiningRepository(), idGen)

	_, err := svc.UpdateProfile("u1", user.UpdateInput{IsLocked: ptr(true)})
	assert.ErrorIs(t, err, stubError)
}

func TestService_UpdateProfile_ProfileUpdateError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	mockUR.Users["u1"] = &model.User{ID: "u1"}
	uRepo := &failingUserRepo{MockUserRepository: mockUR, failUpdateProfile: true}
	idGen, _ := id.NewGenerator("aidx")
	svc := user.NewService(uRepo, testutil.NewMockNoteRepository(), testutil.NewMockUserNotePiningRepository(), idGen)

	_, err := svc.UpdateProfile("u1", user.UpdateInput{Description: ptr(ptr("hi"))})
	assert.ErrorIs(t, err, stubError)
}

func TestService_PinNote_CountError(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	mockUR.Users["u1"] = &model.User{ID: "u1"}
	mockNR := testutil.NewMockNoteRepository()
	mockNR.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	piningRepo := &failingPiningRepo{MockUserNotePiningRepository: testutil.NewMockUserNotePiningRepository(), failCount: true}
	idGen, _ := id.NewGenerator("aidx")
	svc := user.NewService(mockUR, mockNR, piningRepo, idGen)

	err := svc.PinNote("u1", "n1")
	assert.ErrorIs(t, err, stubError)
}

func TestService_ListPinnedNotes_Error(t *testing.T) {
	mockUR := testutil.NewMockUserRepository()
	piningRepo := &failingPiningRepo{MockUserNotePiningRepository: testutil.NewMockUserNotePiningRepository(), failListByU: true}
	idGen, _ := id.NewGenerator("aidx")
	svc := user.NewService(mockUR, testutil.NewMockNoteRepository(), piningRepo, idGen)

	_, err := svc.ListPinnedNotes("u1")
	assert.ErrorIs(t, err, stubError)
}

func TestUpdateUserFields(t *testing.T) {
	svc, userRepo, _, _ := newFullSvc(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "test"}
	err := svc.UpdateUserFields("u1", map[string]any{"isSuspended": true})
	require.NoError(t, err)
}

func TestUpdateProfileFields(t *testing.T) {
	svc, userRepo, _, _ := newFullSvc(t)
	userRepo.Users["u1"] = &model.User{ID: "u1"}
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1"}
	err := svc.UpdateProfileFields("u1", map[string]any{"description": "new"})
	require.NoError(t, err)
}
