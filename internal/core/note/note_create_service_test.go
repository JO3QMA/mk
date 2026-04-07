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

func (f *failingPollRepo) Create(_ *model.Poll) error { return stubError }

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
	svc := note.NewCreateService(noteRepo, pollRepo, idGen)
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
	svc, _, _ := newCreateService(t)

	user := &model.User{ID: "user1"}
	renoteID := "note1"
	created, err := svc.Create(note.CreateInput{
		User:     user,
		RenoteID: &renoteID,
	})
	require.NoError(t, err)
	assert.Equal(t, &renoteID, created.RenoteID)
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

func TestCreateService_NoteRepoCreateError(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := &failingNoteRepoCreate{MockNoteRepository: testutil.NewMockNoteRepository()}
	pollRepo := testutil.NewMockPollRepository()
	svc := note.NewCreateService(noteRepo, pollRepo, idGen)

	user := &model.User{ID: "u1"}
	text := "x"
	_, err := svc.Create(note.CreateInput{User: user, Text: &text})
	require.Error(t, err)
}

func TestCreateService_PollRepoCreateError(t *testing.T) {
	idGen, _ := id.NewGenerator("aidx")
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := &failingPollRepo{}
	svc := note.NewCreateService(noteRepo, pollRepo, idGen)

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
	svc := note.NewCreateService(noteRepo, pollRepo, idGen)

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
	svc := note.NewCreateService(noteRepo, pollRepo, idGen)

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
	}
	for _, tc := range tests {
		t.Run(tc.text, func(t *testing.T) {
			assert.Equal(t, tc.expected, note.ExtractMentions(tc.text))
		})
	}
}
