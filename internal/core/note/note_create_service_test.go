package note_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/note"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubError is a sentinel error used in tests.
var stubError = errors.New("stub error")

// failingNoteRepoCreate fails on Create.
type failingNoteRepoCreate struct{ *testutil.MockNoteRepository }

func (f *failingNoteRepoCreate) Create(_ *model.Note) error { return stubError }

// failingNoteRepoUpdate fails on Update only.
type failingNoteRepoUpdate struct{ *testutil.MockNoteRepository }

func (f *failingNoteRepoUpdate) Update(_ *model.Note, _ string, _ any) error {
	return stubError
}

// failingPollRepo fails on Create.
type failingPollRepo struct{}

func (f *failingPollRepo) Create(_ *model.Poll) error                 { return stubError }
func (f *failingPollRepo) FindByNoteID(_ string) (*model.Poll, error) { return nil, stubError }
func (f *failingPollRepo) IncrementVote(_ string, _ int, _ int) error { return nil }

// findFailNoteRepo creates successfully but FindByIDWithUser always fails.
type findFailNoteRepo struct{ *testutil.MockNoteRepository }

func (f *findFailNoteRepo) FindByIDWithUser(_ string) (*model.Note, error) {
	return nil, stubError
}

func newCreateService(t *testing.T) (*note.CreateService, *testutil.MockNoteRepository, *testutil.MockPollRepository) {
	t.Helper()
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	idGen, _ := id.NewGenerator("aidx")
	svc := note.NewCreateService(noteRepo, pollRepo, idGen, nil)
	return svc, noteRepo, pollRepo
}

func TestCreateService_Success(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)

	user := &model.User{ID: "user1", Username: "alice"}
	text := "Hello"
	created, err := svc.Create(note.CreateInput{
		User:       user,
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "Hello", *created.Text)
	assert.Equal(t, model.NoteVisibilityPublic, created.Visibility)
	assert.Equal(t, user.ID, created.UserID)
	assert.Len(t, noteRepo.Notes, 1)
}

func TestCreateService_DefaultVisibility(t *testing.T) {
	svc, _, _ := newCreateService(t)

	user := &model.User{ID: "user1"}
	text := "x"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})

	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilityPublic, created.Visibility)
}

func TestCreateService_RequiresContent(t *testing.T) {
	svc, _, _ := newCreateService(t)

	_, err := svc.Create(note.CreateInput{User: &model.User{ID: "user1"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, note.ErrNoteContentRequired))
}

func TestCreateService_NilUser(t *testing.T) {
	svc, _, _ := newCreateService(t)

	text := "x"
	_, err := svc.Create(note.CreateInput{Text: &text})
	require.Error(t, err)
}

func TestCreateService_FileIDsOnly(t *testing.T) {
	svc, _, _ := newCreateService(t)

	user := &model.User{ID: "user1"}
	created, err := svc.Create(note.CreateInput{
		User:    user,
		FileIDs: []string{"file1"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"file1"}, []string(created.FileIDs))
}

func TestCreateService_RenoteOnly(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)

	// renote先のノートを事前に作成しておく
	noteRepo.Notes["note1"] = &model.Note{
		ID:         "note1",
		UserID:     "other",
		Visibility: model.NoteVisibilityPublic,
	}

	user := &model.User{ID: "user1"}
	renoteID := "note1"
	created, err := svc.Create(note.CreateInput{
		User:     user,
		RenoteID: &renoteID,
	})
	require.NoError(t, err)
	assert.Equal(t, &renoteID, created.RenoteID)
	// pure renoteなのでrenoteCountが+1される
	assert.Equal(t, int16(1), noteRepo.Notes["note1"].RenoteCount)
}

func TestCreateService_VisibleUserIDs(t *testing.T) {
	svc, _, _ := newCreateService(t)

	user := &model.User{ID: "user1"}
	text := "secret"
	created, err := svc.Create(note.CreateInput{
		User:           user,
		Text:           &text,
		Visibility:     model.NoteVisibilitySpecified,
		VisibleUserIDs: []string{"u2", "u3"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"u2", "u3"}, []string(created.VisibleUserIDs))
}

func TestCreateService_WithPoll(t *testing.T) {
	svc, _, pollRepo := newCreateService(t)

	user := &model.User{ID: "user1"}
	text := "vote!"
	expires := time.Now().Add(24 * time.Hour)
	created, err := svc.Create(note.CreateInput{
		User: user,
		Text: &text,
		Poll: &note.PollInput{
			Choices:   []string{"A", "B"},
			Multiple:  true,
			ExpiresAt: &expires,
		},
	})
	require.NoError(t, err)
	assert.True(t, created.HasPoll)
	assert.Len(t, pollRepo.Polls, 1)
	assert.True(t, pollRepo.Polls[created.ID].Multiple)
	assert.NotNil(t, pollRepo.Polls[created.ID].ExpiresAt)
}

func TestCreateService_MentionExtraction(t *testing.T) {
	svc, _, _ := newCreateService(t)

	user := &model.User{ID: "user1"}
	text := "hi @alice and @bob"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	assert.Equal(t, []string{"alice", "bob"}, []string(created.Mentions))
}

func TestCreateService_MentionResolution_WithUserRepo(t *testing.T) {
	svc, _, _ := newCreateService(t)
	userRepo := testutil.NewMockUserRepository()
	remoteHost := "remote.example"
	userRepo.Users["uA"] = &model.User{
		ID:            "uA",
		Username:      "alice",
		UsernameLower: "alice",
	}
	userRepo.Users["uB"] = &model.User{
		ID:            "uB",
		Username:      "bob",
		UsernameLower: "bob",
		Host:          &remoteHost,
	}
	svc.SetUserRepo(userRepo)

	author := &model.User{ID: "author1"}
	text := "hi @alice and @bob@remote.example and @ghost"
	created, err := svc.Create(note.CreateInput{User: author, Text: &text})
	require.NoError(t, err)
	// Resolved IDs only; unknown @ghost is skipped.
	assert.ElementsMatch(t, []string{"uA", "uB"}, []string(created.Mentions))
}

func TestCreateService_NoteRepoCreateError(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := &failingNoteRepoCreate{MockNoteRepository: testutil.NewMockNoteRepository()}
	pollRepo := testutil.NewMockPollRepository()
	svc := note.NewCreateService(noteRepo, pollRepo, idGen, nil)

	user := &model.User{ID: "u1"}
	text := "x"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.Error(t, err)
}

func TestCreateService_PollRepoCreateError(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := &failingPollRepo{}
	svc := note.NewCreateService(noteRepo, pollRepo, idGen, nil)

	user := &model.User{ID: "u1"}
	text := "x"
	_, err := svc.Create(note.CreateInput{
		User: user,
		Text: &text,
		Poll: &note.PollInput{Choices: []string{"A", "B"}},
	})
	require.Error(t, err)
}

func TestCreateService_NoteRepoUpdateError(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := &failingNoteRepoUpdate{MockNoteRepository: testutil.NewMockNoteRepository()}
	pollRepo := testutil.NewMockPollRepository()
	svc := note.NewCreateService(noteRepo, pollRepo, idGen, nil)

	user := &model.User{ID: "u1"}
	text := "x"
	_, err := svc.Create(note.CreateInput{
		User: user,
		Text: &text,
		Poll: &note.PollInput{Choices: []string{"A", "B"}},
	})
	require.Error(t, err)
}

func TestCreateService_FindByIDWithUserFails(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := &findFailNoteRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	pollRepo := testutil.NewMockPollRepository()
	svc := note.NewCreateService(noteRepo, pollRepo, idGen, nil)

	user := &model.User{ID: "u1"}
	text := "x"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	// FindByIDWithUserが失敗してもin.Userが埋められて返る
	assert.Equal(t, user, created.User)
}

func TestExtractMentions(t *testing.T) {
	tests := []struct {
		text     string
		expected []string
	}{
		{"Hello @alice @bob", []string{"alice", "bob"}},
		{"No mentions", nil},
		{"@solo", []string{"solo"}},
		{"@", nil},
		{"", nil},
		// 重複は除去される
		{"@alice and @alice again", []string{"alice"}},
		// リモートメンション
		{"hi @alice@example.com", []string{"alice@example.com"}},
		// テキスト内の埋め込み
		{"foo@bar baz@qux", []string{"bar", "qux"}},
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			assert.Equal(t, tc.expected, note.ExtractMentions(tc.text))
		})
	}
}

func TestExtractMentionStructs(t *testing.T) {
	out := note.ExtractMentionStructs("hi @alice and @bob@example.com and @alice")
	require.Len(t, out, 2)
	assert.Equal(t, "alice", out[0].Username)
	assert.Equal(t, "", out[0].Host)
	assert.Equal(t, "bob", out[1].Username)
	assert.Equal(t, "example.com", out[1].Host)

	assert.Empty(t, note.ExtractMentionStructs("nothing here"))
}

func TestCreateService_ReplyTargetNotFound(t *testing.T) {
	svc, _, _ := newCreateService(t)

	user := &model.User{ID: "u1"}
	text := "x"
	replyID := "ghost"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text, ReplyID: &replyID})
	require.ErrorIs(t, err, note.ErrReplyTargetNotFound)
}

func TestCreateService_RenoteTargetNotFound(t *testing.T) {
	svc, _, _ := newCreateService(t)

	user := &model.User{ID: "u1"}
	renoteID := "ghost"
	_, err := svc.Create(note.CreateInput{User: user, RenoteID: &renoteID})
	require.ErrorIs(t, err, note.ErrRenoteTargetNotFound)
}

func TestCreateService_CannotReplyToInvisibleNote(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteRepo.Notes["secret"] = &model.Note{
		ID: "secret", UserID: "author", Visibility: model.NoteVisibilityFollowers,
	}

	user := &model.User{ID: "viewer"}
	text := "x"
	replyID := "secret"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text, ReplyID: &replyID})
	require.ErrorIs(t, err, note.ErrCannotReplyToInvisibleNote)
}

func TestCreateService_CannotRenoteInvisibleNote(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteRepo.Notes["secret"] = &model.Note{
		ID: "secret", UserID: "author", Visibility: model.NoteVisibilityFollowers,
	}

	user := &model.User{ID: "viewer"}
	renoteID := "secret"
	_, err := svc.Create(note.CreateInput{User: user, RenoteID: &renoteID})
	require.ErrorIs(t, err, note.ErrCannotRenoteInvisibleNote)
}

func TestCreateService_ReplyHappyPath(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteRepo.Notes["target"] = &model.Note{
		ID: "target", UserID: "author", Visibility: model.NoteVisibilityPublic,
	}

	user := &model.User{ID: "u1"}
	text := "ack"
	replyID := "target"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text, ReplyID: &replyID})
	require.NoError(t, err)
	assert.Equal(t, &replyID, created.ReplyID)
	// repliesCountが更新される
	assert.Equal(t, int16(1), noteRepo.Notes["target"].RepliesCount)
}

func TestCreateService_QuoteRenoteDoesNotIncrementRenoteCount(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteRepo.Notes["target"] = &model.Note{
		ID: "target", UserID: "author", Visibility: model.NoteVisibilityPublic,
	}

	user := &model.User{ID: "u1"}
	text := "quoted!"
	renoteID := "target"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text, RenoteID: &renoteID})
	require.NoError(t, err)
	// quote renoteなのでrenoteCountは増えない
	assert.Equal(t, int16(0), noteRepo.Notes["target"].RenoteCount)
}

func TestIsPureRenote(t *testing.T) {
	target := "x"
	choices := []string{"a", "b"}

	cases := []struct {
		name string
		in   note.CreateInput
		want bool
	}{
		{"no renote", note.CreateInput{}, false},
		{"pure", note.CreateInput{RenoteID: &target}, true},
		{"with text", note.CreateInput{RenoteID: &target, Text: ptrString("hi")}, false},
		{"with cw", note.CreateInput{RenoteID: &target, CW: ptrString("warn")}, false},
		{"with file", note.CreateInput{RenoteID: &target, FileIDs: []string{"f1"}}, false},
		{"with poll", note.CreateInput{RenoteID: &target, Poll: &note.PollInput{Choices: choices}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, note.IsPureRenoteForTest(tc.in))
		})
	}
}

func ptrString(s string) *string { return &s }

// recordingHook captures fanout invocations for tests.
type recordingHook struct {
	called bool
	note   *model.Note
	user   *model.User
}

func (h *recordingHook) OnNoteCreated(n *model.Note, u *model.User) {
	h.called = true
	h.note = n
	h.user = u
}

func TestCreateService_FanoutHookInvoked(t *testing.T) {
	svc, _, _ := newCreateService(t)
	hook := &recordingHook{}
	svc.SetFanoutHook(hook)

	user := &model.User{ID: "u1"}
	text := "hello"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	assert.True(t, hook.called)
	assert.Equal(t, created.ID, hook.note.ID)
	assert.Equal(t, user, hook.user)
}

// recordingNotificationHook captures notification calls for tests.
type recordingNotificationHook struct {
	called       bool
	gotNote      *model.Note
	gotAuthor    *model.User
	gotReplyTgt  *model.Note
	gotRenoteTgt *model.Note
}

func (h *recordingNotificationHook) OnNoteCreated(n *model.Note, u *model.User, reply, renote *model.Note) {
	h.called = true
	h.gotNote = n
	h.gotAuthor = u
	h.gotReplyTgt = reply
	h.gotRenoteTgt = renote
}

func TestCreateService_NotificationHookInvoked(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteRepo.Notes["target"] = &model.Note{
		ID: "target", UserID: "author", Visibility: model.NoteVisibilityPublic,
	}
	hook := &recordingNotificationHook{}
	svc.SetNotificationHook(hook)

	user := &model.User{ID: "u1"}
	text := "hi"
	replyID := "target"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text, ReplyID: &replyID})
	require.NoError(t, err)
	assert.True(t, hook.called)
	assert.Equal(t, "target", hook.gotReplyTgt.ID)
}

func TestCreateService_FanoutHookCalledOnFallbackPath(t *testing.T) {
	// FindByIDWithUser失敗時もfanoutは発火する
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := &findFailNoteRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	pollRepo := testutil.NewMockPollRepository()
	svc := note.NewCreateService(noteRepo, pollRepo, idGen, nil)
	hook := &recordingHook{}
	svc.SetFanoutHook(hook)

	user := &model.User{ID: "u1"}
	text := "hi"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	assert.True(t, hook.called)
	// fallbackパスではin.Userが埋め込まれる
	assert.Equal(t, user, hook.note.User)
}

// recordingFederationHook captures federation hook calls for tests.
type recordingFederationHook struct {
	called bool
	note   *model.Note
	user   *model.User
}

func (h *recordingFederationHook) OnNoteCreated(n *model.Note, u *model.User) {
	h.called = true
	h.note = n
	h.user = u
}

func TestCreateService_FederationHookInvoked(t *testing.T) {
	svc, _, _ := newCreateService(t)
	hook := &recordingFederationHook{}
	svc.SetFederationHook(hook)

	user := &model.User{ID: "u1"}
	text := "hello"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	assert.True(t, hook.called)
	assert.Equal(t, created.ID, hook.note.ID)
}

// recordingChannelHook captures channel hook calls for tests.
type recordingChannelHook struct {
	ensureCalled bool
	ensureID     string
	ensureErr    error
	postedCalled bool
	postedID     string
}

func (h *recordingChannelHook) EnsureChannelExists(channelID string) error {
	h.ensureCalled = true
	h.ensureID = channelID
	return h.ensureErr
}

func (h *recordingChannelHook) OnNotePosted(channelID string) {
	h.postedCalled = true
	h.postedID = channelID
}

func TestCreateService_ChannelHookEnsureAndOnPosted(t *testing.T) {
	svc, _, _ := newCreateService(t)
	hook := &recordingChannelHook{}
	svc.SetChannelHook(hook)

	user := &model.User{ID: "u1"}
	text := "hello"
	channelID := "ch1"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text, ChannelID: &channelID})
	require.NoError(t, err)
	assert.True(t, hook.ensureCalled)
	assert.Equal(t, "ch1", hook.ensureID)
	assert.True(t, hook.postedCalled)
	assert.Equal(t, "ch1", hook.postedID)
}

func TestCreateService_ChannelNotFound(t *testing.T) {
	svc, _, _ := newCreateService(t)
	hook := &recordingChannelHook{ensureErr: errors.New("missing")}
	svc.SetChannelHook(hook)

	user := &model.User{ID: "u1"}
	text := "hello"
	channelID := "ch1"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text, ChannelID: &channelID})
	assert.ErrorIs(t, err, note.ErrChannelNotFound)
}

func TestCreateService_NoChannelHookSkipsCheck(t *testing.T) {
	svc, _, _ := newCreateService(t)
	user := &model.User{ID: "u1"}
	text := "hello"
	channelID := "ch1"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text, ChannelID: &channelID})
	require.NoError(t, err)
}

// recordingAntennaHook captures antenna hook calls for tests.
type recordingAntennaHook struct {
	called bool
	note   *model.Note
}

func (h *recordingAntennaHook) OnNoteCreated(n *model.Note, _ *model.User) {
	h.called = true
	h.note = n
}

func TestCreateService_AntennaHookInvoked(t *testing.T) {
	svc, _, _ := newCreateService(t)
	hook := &recordingAntennaHook{}
	svc.SetAntennaHook(hook)

	user := &model.User{ID: "u1"}
	text := "hello"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	assert.True(t, hook.called)
	assert.Equal(t, created.ID, hook.note.ID)
}

// recordingIndexHook records calls into note.IndexHook so the create / delete
// services の連携経路を検証できる。
type recordingIndexHook struct {
	created *model.Note
	deleted *model.Note
}

func (h *recordingIndexHook) OnNoteCreated(n *model.Note) { h.created = n }
func (h *recordingIndexHook) OnNoteDeleted(n *model.Note) { h.deleted = n }

func TestCreateService_IndexHookInvoked(t *testing.T) {
	svc, _, _ := newCreateService(t)
	hook := &recordingIndexHook{}
	svc.SetIndexHook(hook)

	user := &model.User{ID: "u1"}
	text := "hello"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	require.NotNil(t, hook.created)
	assert.Equal(t, created.ID, hook.created.ID)
}

// recordingChartHook captures the chart hook firing path so we can
// verify NoteCreateService.Create / NoteDeleteService.Delete fan it
// out alongside the other registered hooks.
type recordingChartHook struct {
	created *model.Note
}

func (h *recordingChartHook) OnNoteCreated(n *model.Note) { h.created = n }

func TestCreateService_ChartHookInvoked(t *testing.T) {
	svc, _, _ := newCreateService(t)
	hook := &recordingChartHook{}
	svc.SetChartHook(hook)

	user := &model.User{ID: "u1"}
	text := "hello"
	created, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.NoError(t, err)
	require.NotNil(t, hook.created)
	assert.Equal(t, created.ID, hook.created.ID)
}

func TestIsPureRenote_ModelNote(t *testing.T) {
	renoteID := "r1"
	text := "x"
	cw := "x"
	cases := []struct {
		name string
		n    *model.Note
		want bool
	}{
		{"nil", nil, false},
		{"no renote", &model.Note{}, false},
		{"pure", &model.Note{RenoteID: &renoteID}, true},
		{"with text", &model.Note{RenoteID: &renoteID, Text: &text}, false},
		{"with cw", &model.Note{RenoteID: &renoteID, CW: &cw}, false},
		{"with files", &model.Note{RenoteID: &renoteID, FileIDs: []string{"f"}}, false},
		{"with poll", &model.Note{RenoteID: &renoteID, HasPoll: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, note.IsPureRenote(tc.n))
		})
	}
}

// --- Phase 7-1 follow-up (#254): 追加 sentinel error 検出テスト ---

func strPtr254(s string) *string { return &s }

func TestCreateService_ExpiredPoll(t *testing.T) {
	svc, _, _ := newCreateService(t)
	past := time.Now().Add(-1 * time.Hour)
	text := "vote!"
	_, err := svc.Create(note.CreateInput{
		User: &model.User{ID: "u1"},
		Text: &text,
		Poll: &note.PollInput{Choices: []string{"a", "b"}, ExpiresAt: &past},
	})
	assert.ErrorIs(t, err, note.ErrCannotCreateAlreadyExpiredPoll)
}

func TestCreateService_CannotRenoteToAPureRenote(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteID := "pure_renote_target"
	renoteTargetInner := "other_note"
	noteRepo.Notes[noteID] = &model.Note{
		ID: noteID, UserID: "other", Visibility: model.NoteVisibilityPublic,
		RenoteID: &renoteTargetInner, // pure renote: text/cw/files/poll なし
	}
	_, err := svc.Create(note.CreateInput{
		User:     &model.User{ID: "u1"},
		RenoteID: strPtr254(noteID),
	})
	assert.ErrorIs(t, err, note.ErrCannotRenoteToAPureRenote)
}

func TestCreateService_CannotReplyToAPureRenote(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteID := "pure_renote_target_2"
	inner := "other_note"
	noteRepo.Notes[noteID] = &model.Note{
		ID: noteID, UserID: "other", Visibility: model.NoteVisibilityPublic,
		RenoteID: &inner,
	}
	text := "hi"
	_, err := svc.Create(note.CreateInput{
		User:    &model.User{ID: "u1"},
		Text:    &text,
		ReplyID: strPtr254(noteID),
	})
	assert.ErrorIs(t, err, note.ErrCannotReplyToAPureRenote)
}

func TestCreateService_CannotReplyToSpecifiedVisibility(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	noteID := "specified_target"
	noteRepo.Notes[noteID] = &model.Note{
		ID: noteID, UserID: "u1", Visibility: model.NoteVisibilitySpecified,
		VisibleUserIDs: []string{"u1"},
	}
	text := "hi"
	_, err := svc.Create(note.CreateInput{
		User:       &model.User{ID: "u1"},
		Text:       &text,
		ReplyID:    strPtr254(noteID),
		Visibility: model.NoteVisibilityPublic, // widerで不許可
	})
	assert.ErrorIs(t, err, note.ErrCannotReplyToSpecifiedVisibility)
}

func TestCreateService_YouHaveBeenBlocked_Renote(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	blockRepo := testutil.NewMockBlockingRepository()
	svc.SetBlockingRepo(blockRepo)

	// block: target user が actor を block している
	require.NoError(t, blockRepo.Create(&model.Blocking{ID: "b1", BlockerID: "target_user", BlockeeID: "u1"}))

	noteID := "renote_target_blocked"
	noteRepo.Notes[noteID] = &model.Note{
		ID: noteID, UserID: "target_user", Visibility: model.NoteVisibilityPublic,
	}
	_, err := svc.Create(note.CreateInput{
		User:     &model.User{ID: "u1"},
		RenoteID: strPtr254(noteID),
	})
	assert.ErrorIs(t, err, note.ErrYouHaveBeenBlocked)
}

func TestCreateService_YouHaveBeenBlocked_Reply(t *testing.T) {
	svc, noteRepo, _ := newCreateService(t)
	blockRepo := testutil.NewMockBlockingRepository()
	svc.SetBlockingRepo(blockRepo)

	require.NoError(t, blockRepo.Create(&model.Blocking{ID: "b1", BlockerID: "target_user", BlockeeID: "u1"}))

	noteID := "reply_target_blocked"
	noteRepo.Notes[noteID] = &model.Note{
		ID: noteID, UserID: "target_user", Visibility: model.NoteVisibilityPublic,
	}
	text := "hi"
	_, err := svc.Create(note.CreateInput{
		User:    &model.User{ID: "u1"},
		Text:    &text,
		ReplyID: strPtr254(noteID),
	})
	assert.ErrorIs(t, err, note.ErrYouHaveBeenBlocked)
}

func TestCreateService_NoSuchFile(t *testing.T) {
	svc, _, _ := newCreateService(t)
	fileRepo := testutil.NewMockDriveFileRepository()
	svc.SetDriveFileRepo(fileRepo)

	text := "hello"
	_, err := svc.Create(note.CreateInput{
		User:    &model.User{ID: "u1"},
		Text:    &text,
		FileIDs: []string{"nonexistent_file"},
	})
	assert.ErrorIs(t, err, note.ErrNoSuchFile)
}

func TestCreateService_ContainsProhibitedWords(t *testing.T) {
	svc, _, _ := newCreateService(t)
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "m1", ProhibitedWords: []string{"badword"}}
	svc.SetMetaRepo(metaRepo)

	text := "contains badword here"
	_, err := svc.Create(note.CreateInput{
		User: &model.User{ID: "u1"},
		Text: &text,
	})
	assert.ErrorIs(t, err, note.ErrContainsProhibitedWords)
}

func TestCreateService_ContainsTooManyMentions(t *testing.T) {
	svc, _, _ := newCreateService(t)
	// 21 メンションで default limit (20) 超過
	mentions := ""
	for i := 0; i < 21; i++ {
		mentions += "@user" + strPtr254Str(i) + " "
	}
	_, err := svc.Create(note.CreateInput{
		User: &model.User{ID: "u1"},
		Text: &mentions,
	})
	assert.ErrorIs(t, err, note.ErrContainsTooManyMentions)
}

func strPtr254Str(i int) string {
	// base36: 0-9,a-z
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	if i < 36 {
		return string(chars[i])
	}
	return string(chars[i/36]) + string(chars[i%36])
}
