package reaction_test

import (
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService(t *testing.T) (
	*reaction.Service,
	*testutil.MockNoteRepository,
	*testutil.MockNoteReactionRepository,
	*testutil.MockEmojiRepository,
	*testutil.MockFollowingRepository,
) {
	t.Helper()
	noteRepo := testutil.NewMockNoteRepository()
	reactRepo := testutil.NewMockNoteReactionRepository()
	emojiRepo := testutil.NewMockEmojiRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := reaction.NewService(noteRepo, reactRepo, emojiRepo, followingRepo, idGen)
	return svc, noteRepo, reactRepo, emojiRepo, followingRepo
}

func seedNote(repo *testutil.MockNoteRepository, id, userID string, vis model.NoteVisibility) *model.Note {
	n := &model.Note{ID: id, UserID: userID, Visibility: vis}
	repo.Notes[id] = n
	return n
}

func TestService_Create_NilUser(t *testing.T) {
	svc, _, _, _, _ := newService(t)
	_, err := svc.Create(nil, "n", "")
	require.Error(t, err)
}

func TestService_Create_NoteNotFound(t *testing.T) {
	svc, _, _, _, _ := newService(t)
	_, err := svc.Create(&model.User{ID: "u"}, "ghost", "")
	require.ErrorIs(t, err, reaction.ErrNoteNotFound)
}

func TestService_Create_NotVisible(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityFollowers)
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "")
	require.ErrorIs(t, err, reaction.ErrNoteNotVisible)
}

func TestService_Create_PureRenote(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	target := "x"
	repo.Notes["n1"] = &model.Note{
		ID: "n1", UserID: "author", Visibility: model.NoteVisibilityPublic, RenoteID: &target,
	}
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "")
	require.ErrorIs(t, err, reaction.ErrCannotReactToPureRenote)
}

func TestService_Create_HappyPathFallback(t *testing.T) {
	svc, repo, reactRepo, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	r, err := svc.Create(&model.User{ID: "viewer"}, "n1", "")
	require.NoError(t, err)
	assert.Equal(t, reaction.FallbackReaction, r)
	assert.Len(t, reactRepo.Reactions, 1)
}

func TestService_Create_LegacyTranslation(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	r, err := svc.Create(&model.User{ID: "viewer"}, "n1", "like")
	require.NoError(t, err)
	assert.Equal(t, "👍", r)
}

func TestService_Create_AlreadyReactedSame(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.NoError(t, err)
	_, err = svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.ErrorIs(t, err, reaction.ErrAlreadyReacted)
}

func TestService_Create_ReplaceExisting(t *testing.T) {
	svc, repo, reactRepo, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.NoError(t, err)
	_, err = svc.Create(&model.User{ID: "viewer"}, "n1", "❤")
	require.NoError(t, err)
	// 古いレコードは削除されているので1件
	assert.Len(t, reactRepo.Reactions, 1)
}

func TestService_Create_CustomEmojiLocal(t *testing.T) {
	svc, repo, _, emojiRepo, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	emojiRepo.Emojis["smile@"] = &model.Emoji{Name: "smile"}
	r, err := svc.Create(&model.User{ID: "viewer"}, "n1", ":smile:")
	require.NoError(t, err)
	assert.Equal(t, ":smile@.:", r)
}

func TestService_Create_CustomEmojiRemote(t *testing.T) {
	svc, repo, _, emojiRepo, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	host := "remote.example"
	emojiRepo.Emojis["smile@remote.example"] = &model.Emoji{Name: "smile", Host: &host}
	r, err := svc.Create(&model.User{ID: "viewer"}, "n1", ":smile@remote.example:")
	require.NoError(t, err)
	assert.Equal(t, ":smile@remote.example:", r)
}

func TestService_Create_CustomEmojiNotFoundFallback(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	r, err := svc.Create(&model.User{ID: "viewer"}, "n1", ":nonexistent:")
	require.NoError(t, err)
	assert.Equal(t, reaction.FallbackReaction, r)
}

// failingReactionRepo simulates a Create error.
type failingReactionRepo struct {
	*testutil.MockNoteReactionRepository
}

func (f *failingReactionRepo) Create(_ *model.NoteReaction) error {
	return errors.New("boom")
}

func TestService_Create_RepoError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	seedNote(noteRepo, "n1", "author", model.NoteVisibilityPublic)
	idGen, _ := id.NewGenerator("aidx")
	svc := reaction.NewService(
		noteRepo,
		&failingReactionRepo{MockNoteReactionRepository: testutil.NewMockNoteReactionRepository()},
		testutil.NewMockEmojiRepository(),
		testutil.NewMockFollowingRepository(),
		idGen,
	)
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	assert.Error(t, err)
}

// failingReactionRepoOnDelete fails on Delete (replace path).
type failingReactionRepoOnDelete struct {
	*testutil.MockNoteReactionRepository
}

func (f *failingReactionRepoOnDelete) Delete(_ *model.NoteReaction) error {
	return errors.New("delete boom")
}

func TestService_Create_ReplaceDeleteError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	seedNote(noteRepo, "n1", "author", model.NoteVisibilityPublic)
	mock := testutil.NewMockNoteReactionRepository()
	// 既存リアクションを差し込む
	mock.Reactions["existing"] = &model.NoteReaction{
		ID: "existing", UserID: "viewer", NoteID: "n1", Reaction: "👍",
	}
	idGen, _ := id.NewGenerator("aidx")
	svc := reaction.NewService(
		noteRepo,
		&failingReactionRepoOnDelete{MockNoteReactionRepository: mock},
		testutil.NewMockEmojiRepository(),
		testutil.NewMockFollowingRepository(),
		idGen,
	)
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "❤")
	assert.Error(t, err)
}

func TestService_Delete_NilUser(t *testing.T) {
	svc, _, _, _, _ := newService(t)
	err := svc.Delete(nil, "n")
	require.Error(t, err)
}

func TestService_Delete_NoteNotFound(t *testing.T) {
	svc, _, _, _, _ := newService(t)
	err := svc.Delete(&model.User{ID: "u"}, "ghost")
	require.ErrorIs(t, err, reaction.ErrNoteNotFound)
}

func TestService_Delete_ReactionNotFound(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	err := svc.Delete(&model.User{ID: "viewer"}, "n1")
	require.ErrorIs(t, err, reaction.ErrReactionNotFound)
}

func TestService_Delete_HappyPath(t *testing.T) {
	svc, repo, reactRepo, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.NoError(t, err)
	require.NoError(t, svc.Delete(&model.User{ID: "viewer"}, "n1"))
	assert.Empty(t, reactRepo.Reactions)
}

// failingDeleteRepo causes Delete to fail (used for delete path coverage).
type failingDeleteRepo struct {
	*testutil.MockNoteReactionRepository
}

func (f *failingDeleteRepo) Delete(_ *model.NoteReaction) error {
	return errors.New("delete fail")
}

func TestService_Delete_RepoError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	seedNote(noteRepo, "n1", "author", model.NoteVisibilityPublic)
	mock := testutil.NewMockNoteReactionRepository()
	mock.Reactions["existing"] = &model.NoteReaction{
		ID: "existing", UserID: "viewer", NoteID: "n1", Reaction: "👍",
	}
	idGen, _ := id.NewGenerator("aidx")
	svc := reaction.NewService(
		noteRepo,
		&failingDeleteRepo{MockNoteReactionRepository: mock},
		testutil.NewMockEmojiRepository(),
		testutil.NewMockFollowingRepository(),
		idGen,
	)
	err := svc.Delete(&model.User{ID: "viewer"}, "n1")
	assert.Error(t, err)
}

func TestService_List_NoteNotFound(t *testing.T) {
	svc, _, _, _, _ := newService(t)
	_, err := svc.List(nil, "ghost", "", "", 10, "")
	require.ErrorIs(t, err, reaction.ErrNoteNotFound)
}

func TestService_List_NotVisible(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityFollowers)
	_, err := svc.List(&model.User{ID: "viewer"}, "n1", "", "", 10, "")
	require.ErrorIs(t, err, reaction.ErrNoteNotVisible)
}

func TestService_List_Filtered(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	_, err := svc.Create(&model.User{ID: "u1"}, "n1", "👍")
	require.NoError(t, err)
	_, err = svc.Create(&model.User{ID: "u2"}, "n1", "❤")
	require.NoError(t, err)

	out, err := svc.List(nil, "n1", "", "", 0, "👍")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "👍", out[0].Reaction)
}

func TestIsPureRenote_Variants(t *testing.T) {
	target := "x"
	cases := []struct {
		name string
		n    *model.Note
		want bool
	}{
		{"no renote", &model.Note{}, false},
		{"pure", &model.Note{RenoteID: &target}, true},
		{"with text", &model.Note{RenoteID: &target, Text: ptrString("hi")}, false},
		{"with cw", &model.Note{RenoteID: &target, CW: ptrString("warn")}, false},
		{"with file", &model.Note{RenoteID: &target, FileIDs: pq.StringArray{"f1"}}, false},
		{"with poll", &model.Note{RenoteID: &target, HasPoll: true}, false},
	}
	// IsPureRenote はパッケージ内なのでテストヘルパ経由ではなく
	// 同じ判定をServiceの動作で間接的に検証する
	noteRepo := testutil.NewMockNoteRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := reaction.NewService(
		noteRepo,
		testutil.NewMockNoteReactionRepository(),
		testutil.NewMockEmojiRepository(),
		testutil.NewMockFollowingRepository(),
		idGen,
	)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.n.ID = "n_" + tc.name
			tc.n.UserID = "author"
			tc.n.Visibility = model.NoteVisibilityPublic
			noteRepo.Notes[tc.n.ID] = tc.n
			_, err := svc.Create(&model.User{ID: "viewer"}, tc.n.ID, "👍")
			if tc.want {
				assert.ErrorIs(t, err, reaction.ErrCannotReactToPureRenote)
			} else {
				assert.NoError(t, err)
			}
			delete(noteRepo.Notes, tc.n.ID)
		})
	}
}

func ptrString(s string) *string { return &s }

// recordingNotificationHook captures reaction notification calls.
type recordingNotificationHook struct {
	called   bool
	notifiee string
	notifier string
	noteID   string
	reaction string
}

func (h *recordingNotificationHook) OnReactionCreated(notifieeID, notifierID, noteID, rx string) {
	h.called = true
	h.notifiee = notifieeID
	h.notifier = notifierID
	h.noteID = noteID
	h.reaction = rx
}

func TestService_NotificationHook_OnReaction(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	hook := &recordingNotificationHook{}
	svc.SetNotificationHook(hook)

	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.NoError(t, err)
	assert.True(t, hook.called)
	assert.Equal(t, "author", hook.notifiee)
	assert.Equal(t, "viewer", hook.notifier)
	assert.Equal(t, "n1", hook.noteID)
	assert.Equal(t, "👍", hook.reaction)
}

var stubReactionError = errors.New("reaction stub error")

// stubBlockingChecker for tests.
type stubBlockingChecker struct {
	blocked bool
	err     error
}

func (s *stubBlockingChecker) IsBlocked(_, _ string) (bool, error) {
	return s.blocked, s.err
}

func TestService_Create_Blocked(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	svc.SetBlockingChecker(&stubBlockingChecker{blocked: true})
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.ErrorIs(t, err, reaction.ErrBlocked)
}

func TestService_Create_BlockingCheckerError(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "author", model.NoteVisibilityPublic)
	svc.SetBlockingChecker(&stubBlockingChecker{err: stubReactionError})
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	assert.ErrorIs(t, err, stubReactionError)
}

func TestService_Create_BlockingCheckerSelfSkipped(t *testing.T) {
	// 自分自身のノートにはblockチェックが走らない
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "viewer", model.NoteVisibilityPublic)
	svc.SetBlockingChecker(&stubBlockingChecker{blocked: true})
	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.NoError(t, err)
}

func TestService_NotificationHook_SelfReactionSkipped(t *testing.T) {
	svc, repo, _, _, _ := newService(t)
	seedNote(repo, "n1", "viewer", model.NoteVisibilityPublic)
	hook := &recordingNotificationHook{}
	svc.SetNotificationHook(hook)

	_, err := svc.Create(&model.User{ID: "viewer"}, "n1", "👍")
	require.NoError(t, err)
	assert.False(t, hook.called)
}
