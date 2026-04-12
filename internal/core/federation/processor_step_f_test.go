package federation_test

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	corereaction "github.com/shiroha-a/mk/internal/core/reaction"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fullProcessor builds a processor with reactionService / noteDeleteService
// wired in (Step F handlers depend on these). It returns the constructed
// processor and the underlying repos so tests can inspect/mutate state.
type fullProcessorEnv struct {
	processor    *federation.Processor
	userRepo     *testutil.MockUserRepository
	noteRepo     *testutil.MockNoteRepository
	reactionRepo *testutil.MockNoteReactionRepository
}

func newFullProcessor(t *testing.T, fetcherBody string) *fullProcessorEnv {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	reactionRepo := testutil.NewMockNoteReactionRepository()
	emojiRepo := testutil.NewMockEmojiRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(fetcherBody)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	reactionSvc := corereaction.NewService(noteRepo, reactionRepo, emojiRepo, followingRepo, idGen)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	p := federation.NewProcessor(resolver, followingSvc, reactionSvc, deleteSvc, userRepo, noteRepo)
	return &fullProcessorEnv{processor: p, userRepo: userRepo, noteRepo: noteRepo, reactionRepo: reactionRepo}
}

const noteCreateBody = `{
	"type": "Create",
	"actor": "https://remote.example/users/alice",
	"object": {
		"id": "https://remote.example/notes/n1",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "hi",
		"to": ["https://www.w3.org/ns/activitystreams#Public"]
	}
}`

func TestProcess_CreateNote(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	require.NoError(t, env.processor.Process([]byte(noteCreateBody)))
	// 取り込まれた note は uri で検索できるはず
	found, err := env.noteRepo.FindByURI("https://remote.example/notes/n1")
	require.NoError(t, err)
	require.NotNil(t, found.Text)
	assert.Equal(t, "hi", *found.Text)
}

func TestProcess_CreateActorError(t *testing.T) {
	env := newFullProcessor(t, "{not json")
	err := env.processor.Process([]byte(noteCreateBody))
	assert.Error(t, err)
}

func TestProcess_CreateNoteIngestError(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{"type":"Create","actor":"https://remote.example/users/alice","object":{"id":"x"}}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

// --- Like ---------------------------------------------------------------------

func TestProcess_LikeHappyPath(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	// ローカルノート bob/n1 を登録
	env.noteRepo.Notes["n1"] = &model.Note{
		ID:         "n1",
		UserID:     "bob",
		Visibility: model.NoteVisibilityPublic,
	}
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "👍"
	}`)
	require.NoError(t, env.processor.Process(body))
	assert.Len(t, env.reactionRepo.Reactions, 1)
}

func TestProcess_LikeFromObjectURI(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	require.NoError(t, env.processor.Process(body))
	assert.Len(t, env.reactionRepo.Reactions, 1)
}

func TestProcess_LikeAlreadyReacted(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "👍"
	}`)
	require.NoError(t, env.processor.Process(body))
	// 2 回目: ErrAlreadyReacted は飲み込まれる
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_LikeUnknownTarget(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/missing"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_LikeNoReactionService(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	err := p.Process(body)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

func TestProcess_LikeBadObject(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": 42
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_LikeActorError(t *testing.T) {
	env := newFullProcessor(t, "{not json")
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

// --- Like content / _misskey_reaction (ported from nekonoverse handler_like) --

func TestProcess_LikeWithContent(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "⭐"
	}`)
	require.NoError(t, env.processor.Process(body))
	require.Len(t, env.reactionRepo.Reactions, 1)
	for _, r := range env.reactionRepo.Reactions {
		assert.Equal(t, "⭐", r.Reaction)
	}
}

func TestProcess_LikeWithMisskeyReaction(t *testing.T) {
	// _misskey_reaction は content より優先される
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "Like",
		"_misskey_reaction": "😀"
	}`)
	require.NoError(t, env.processor.Process(body))
	require.Len(t, env.reactionRepo.Reactions, 1)
	for _, r := range env.reactionRepo.Reactions {
		assert.Equal(t, "😀", r.Reaction)
	}
}

func TestProcess_LikeEmptyContent(t *testing.T) {
	// content が無い場合は reaction service が FallbackReaction (❤) に正規化する
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	require.NoError(t, env.processor.Process(body))
	require.Len(t, env.reactionRepo.Reactions, 1)
	for _, r := range env.reactionRepo.Reactions {
		assert.Equal(t, "\u2764", r.Reaction)
	}
}

// --- Announce ----------------------------------------------------------------

func TestProcess_AnnounceHappyPath(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Announce",
		"id": "https://remote.example/announces/a1",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	require.NoError(t, env.processor.Process(body))
	// 取り込まれた renote が 1 件存在するはず
	var renotes int
	for _, n := range env.noteRepo.Notes {
		if n.RenoteID != nil && *n.RenoteID == "n1" {
			renotes++
		}
	}
	assert.Equal(t, 1, renotes)
}

func TestProcess_AnnounceDuplicate(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Announce",
		"id": "https://remote.example/announces/a1",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	require.NoError(t, env.processor.Process(body))
	// 2 回目は重複検出で no-op
	require.NoError(t, env.processor.Process(body))
	var renotes int
	for _, n := range env.noteRepo.Notes {
		if n.RenoteID != nil && *n.RenoteID == "n1" {
			renotes++
		}
	}
	assert.Equal(t, 1, renotes)
}

func TestProcess_AnnounceUnknownTarget(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Announce",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/missing"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_AnnounceActorError(t *testing.T) {
	env := newFullProcessor(t, "{not json")
	body := []byte(`{
		"type": "Announce",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_AnnounceBadObject(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Announce",
		"actor": "https://remote.example/users/alice",
		"object": 42
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_AnnounceIncrementsRenoteCount(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	body := []byte(`{
		"type": "Announce",
		"id": "https://remote.example/announces/a2",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	require.NoError(t, env.processor.Process(body))
	assert.Equal(t, int16(1), env.noteRepo.Notes["n1"].RenoteCount)
}

// --- Delete ------------------------------------------------------------------

func TestProcess_DeleteRemoteNote(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	// alice 自身が著者であるリモートnoteを事前に登録
	uri := "https://remote.example/notes/n1"
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "alice-id", URI: &uri}
	// resolver が alice を作る前に DB に揃える: 上記 ResolveActor で生成される ID と
	// 一致させるため、alice を予め repo に置いておく
	aliceURI := "https://remote.example/users/alice"
	env.userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}

	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": {"id": "https://remote.example/notes/n1", "type": "Tombstone"}
	}`)
	require.NoError(t, env.processor.Process(body))
	_, err := env.noteRepo.FindByID("n1")
	assert.Error(t, err)
}

// Delete アクティビティで返信が消されたとき、親ノートの repliesCount が
// デクリメントされることを end-to-end で確認する (issue #11, item 1)。
func TestProcess_DeleteRemoteReply_DecrementsParent(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	parentID := "parent1"
	env.noteRepo.Notes[parentID] = &model.Note{ID: parentID, UserID: "local-user", RepliesCount: 2}

	replyURI := "https://remote.example/notes/reply1"
	parent := parentID
	env.noteRepo.Notes["reply1"] = &model.Note{
		ID: "reply1", UserID: "alice-id", URI: &replyURI, ReplyID: &parent,
	}
	aliceURI := "https://remote.example/users/alice"
	env.userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}

	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": {"id": "https://remote.example/notes/reply1", "type": "Tombstone"}
	}`)
	require.NoError(t, env.processor.Process(body))

	assert.Equal(t, int16(1), env.noteRepo.Notes[parentID].RepliesCount)
}

func TestProcess_DeleteUnknownNote(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/missing"
	}`)
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_DeleteFromNonAuthor(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	uri := "https://remote.example/notes/n1"
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "other", URI: &uri}
	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_DeleteActorSelfDelete(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/users/alice"
	}`)
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_DeleteActorError(t *testing.T) {
	env := newFullProcessor(t, "{not json")
	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_DeleteBadObject(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": 42
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

// noteDeleteSvc が nil の Processor では noteRepo.Delete が直接呼ばれる
func TestProcess_DeleteWithoutDeleteSvc(t *testing.T) {
	p, repo, _, noteRepo := newProcessor(t, aliceActor)
	uri := "https://remote.example/notes/n1"
	aliceURI := "https://remote.example/users/alice"
	repo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "alice-id", URI: &uri}
	body := []byte(`{
		"type": "Delete",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1"
	}`)
	require.NoError(t, p.Process(body))
	_, err := noteRepo.FindByID("n1")
	assert.Error(t, err)
}

// --- Undo Like ---------------------------------------------------------------

func TestProcess_UndoLikeHappyPath(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}

	// まず Like を作る
	likeBody := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "👍"
	}`)
	require.NoError(t, env.processor.Process(likeBody))
	require.Len(t, env.reactionRepo.Reactions, 1)

	// Undo Like
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Like",
			"object": "https://example.com/notes/n1"
		}
	}`)
	require.NoError(t, env.processor.Process(undo))
	assert.Empty(t, env.reactionRepo.Reactions)
}

func TestProcess_UndoLikeNotFound(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Like",
			"object": "https://example.com/notes/n1"
		}
	}`)
	require.NoError(t, env.processor.Process(undo))
}

func TestProcess_UndoLikeNoReactionService(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Like","object":"https://example.com/notes/n1"}
	}`)
	err := p.Process(undo)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

func TestProcess_UndoLikeUnknownTarget(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Like","object":"https://example.com/notes/missing"}
	}`)
	err := env.processor.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoLikeActorError(t *testing.T) {
	env := newFullProcessor(t, "{not json")
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Like","object":"https://example.com/notes/n1"}
	}`)
	err := env.processor.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoLikeBadObject(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Like","object":42}
	}`)
	err := env.processor.Process(undo)
	assert.Error(t, err)
}

// --- Undo Announce -----------------------------------------------------------

func TestProcess_UndoAnnounceHappyPath(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}

	announce := []byte(`{
		"type": "Announce",
		"id": "https://remote.example/announces/a1",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	require.NoError(t, env.processor.Process(announce))

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Announce",
			"object": "https://example.com/notes/n1"
		}
	}`)
	require.NoError(t, env.processor.Process(undo))

	var renotes int
	for _, n := range env.noteRepo.Notes {
		if n.RenoteID != nil && *n.RenoteID == "n1" {
			renotes++
		}
	}
	assert.Equal(t, 0, renotes)
}

func TestProcess_UndoAnnounceNoMatch(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Announce",
			"object": "https://example.com/notes/n1"
		}
	}`)
	require.NoError(t, env.processor.Process(undo))
}

func TestProcess_UndoAnnounceActorError(t *testing.T) {
	env := newFullProcessor(t, "{not json")
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Announce","object":"https://example.com/notes/n1"}
	}`)
	err := env.processor.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoAnnounceUnknownTarget(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Announce","object":"https://example.com/notes/missing"}
	}`)
	err := env.processor.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoAnnounceBadObject(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Announce","object":42}
	}`)
	err := env.processor.Process(undo)
	assert.Error(t, err)
}

// --- Update ------------------------------------------------------------------

// --- Note update (Step J) -----------------------------------------------------

func TestProcess_UpdateNoteHappyPath(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	uri := "https://remote.example/notes/n1"
	host := "remote.example"
	original := "original"
	env.noteRepo.Notes["n1"] = &model.Note{
		ID: "n1", URI: &uri, UserID: "alice-id", UserHost: &host, Text: &original,
	}
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {
			"id": "https://remote.example/notes/n1",
			"type": "Note",
			"attributedTo": "https://remote.example/users/alice",
			"content": "edited"
		}
	}`)
	require.NoError(t, env.processor.Process(body))
	got := env.noteRepo.Notes["n1"]
	require.NotNil(t, got.Text)
	assert.Equal(t, "edited", *got.Text)
}

func TestProcess_UpdateNote_NotFound(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {
			"id": "https://remote.example/notes/missing",
			"type": "Note",
			"attributedTo": "https://remote.example/users/alice",
			"content": "edited"
		}
	}`)
	// 未取得ノートは silently ignore
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_UpdateNote_InvalidNoteIsAccepted(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	// type は Note だが id が無いので ErrInvalidNote → 200 扱い
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Note"
		}
	}`)
	require.NoError(t, env.processor.Process(body))
}

// updateFailNoteRepo causes UpdateFields to fail, exercising the
// non-ErrInvalidNote error path of handleUpdate.
type updateFailNoteRepo struct {
	*testutil.MockNoteRepository
}

func (r *updateFailNoteRepo) UpdateFields(_ string, _ map[string]any) error {
	return errors.New("update boom")
}

func TestProcess_UpdateNote_RepoErrorPropagates(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	mock := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	host := "remote.example"
	mock.Notes["n1"] = &model.Note{
		ID: "n1", URI: &uri, UserID: "alice-id", UserHost: &host,
	}
	noteRepo := &updateFailNoteRepo{MockNoteRepository: mock}
	emojiRepo := testutil.NewMockEmojiRepository()
	reactionRepo := testutil.NewMockNoteReactionRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	reactionSvc := corereaction.NewService(noteRepo, reactionRepo, emojiRepo, followingRepo, idGen)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	p := federation.NewProcessor(resolver, followingSvc, reactionSvc, deleteSvc, userRepo, noteRepo)

	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {
			"id": "https://remote.example/notes/n1",
			"type": "Note",
			"attributedTo": "https://remote.example/users/alice",
			"content": "edited"
		}
	}`)
	err := p.Process(body)
	assert.Error(t, err)
}

func TestProcess_UpdatePerson(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	uri := "https://remote.example/users/alice"
	env.userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &uri}
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {
			"id": "https://remote.example/users/alice",
			"type": "Person",
			"name": "Alice Updated"
		}
	}`)
	require.NoError(t, env.processor.Process(body))
	require.NotNil(t, env.userRepo.Users["alice-id"].Name)
	assert.Equal(t, "Alice Updated", *env.userRepo.Users["alice-id"].Name)
}

func TestProcess_UpdateUnknownActor(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {"id":"https://remote.example/users/ghost","type":"Person","name":"X"}
	}`)
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_UpdateNonPerson(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	uri := "https://remote.example/users/alice"
	env.userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &uri}
	// Note ではない非対応 type (Article 等) は no-op
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {"id":"https://remote.example/users/alice","type":"Article","name":"X"}
	}`)
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_UpdateMissingObject(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice"
	}`)
	// object フィールド無し → peekObjectType は "" を返し Person 経路に入る
	// → readObjectString が "missing object" を返してエラー
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_UpdateNoChanges(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	uri := "https://remote.example/users/alice"
	env.userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &uri}
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": {"id":"https://remote.example/users/alice","type":"Person"}
	}`)
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_UpdateObjectAsURI(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/users/alice"
	}`)
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_UpdateBadObject(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Update",
		"actor": "https://remote.example/users/alice",
		"object": 42
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

// --- Reject ------------------------------------------------------------------

func TestProcess_RejectFollow(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	// local follower bob, remote followee alice (resolver で取り込まれる)
	bobURI := "https://example.com/users/bob"
	env.userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}
	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://example.com/users/bob",
			"object": "https://remote.example/users/alice"
		}
	}`)
	require.NoError(t, env.processor.Process(body))
}

func TestProcess_RejectInvalidJSON(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": "not-json"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_RejectInnerNotFollow(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Like"}
	}`)
	err := env.processor.Process(body)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

func TestProcess_RejectActorError(t *testing.T) {
	env := newFullProcessor(t, "{not json")
	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Follow","actor":"https://example.com/users/bob"}
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_RejectInnerMissingActor(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Follow"}
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

func TestProcess_RejectUnknownFollower(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://example.com/users/ghost"
		}
	}`)
	require.NoError(t, env.processor.Process(body))
}

// --- Visibility derivation ----------------------------------------------------

func TestIngestNote_VisibilityHome(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	body := []byte(`{
		"id": "https://remote.example/notes/home",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "home",
		"to": ["https://remote.example/users/alice/followers"],
		"cc": ["https://www.w3.org/ns/activitystreams#Public"]
	}`)
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilityHome, got.Visibility)
}

func TestIngestNote_VisibilityFollowers(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	body := []byte(`{
		"id": "https://remote.example/notes/fol",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "fol",
		"to": ["https://remote.example/users/alice/followers"]
	}`)
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilityFollowers, got.Visibility)
}

func TestIngestNote_VisibilitySpecified(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	body := []byte(`{
		"id": "https://remote.example/notes/spec",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "spec",
		"to": ["https://example.com/users/bob"]
	}`)
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilitySpecified, got.Visibility)
}

// --- extractLocalNoteID via ResolveNote with extra path -----------------------

func TestResolveNote_LocalURIWithExtraSegment(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "u1"}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)
	got, err := r.ResolveNote("https://example.com/notes/n1/activity")
	require.NoError(t, err)
	assert.Equal(t, "n1", got.ID)
}

// noteCreateFailRepo causes Create on noteRepo to fail (for handleAnnounce).
type noteCreateFailRepo struct {
	*testutil.MockNoteRepository
}

func (r *noteCreateFailRepo) Create(_ *model.Note) error { return errors.New("create failed") }

// listRenotesFailRepo causes ListRenotesOf to fail (for handleUndoAnnounce).
type listRenotesFailRepo struct {
	*testutil.MockNoteRepository
}

func (r *listRenotesFailRepo) ListRenotesOf(_, _, _ string, _ int) ([]*model.Note, error) {
	return nil, errors.New("list failed")
}

// deleteFailNoteRepo causes Delete on noteRepo to fail (for handleUndoAnnounce).
type deleteFailNoteRepo struct {
	*testutil.MockNoteRepository
}

func (r *deleteFailNoteRepo) Delete(_ *model.Note) error { return errors.New("delete failed") }

// deleteFailReactionRepo causes Delete on reactionRepo to fail (for handleUndoLike).
type deleteFailReactionRepo struct {
	*testutil.MockNoteReactionRepository
}

func (r *deleteFailReactionRepo) Delete(_ *model.NoteReaction) error {
	return errors.New("react delete failed")
}

// deleteFailFollowingRepo causes Delete on followingRepo to fail (for handleReject Unfollow).
type deleteFailFollowingRepo struct {
	*testutil.MockFollowingRepository
}

func (r *deleteFailFollowingRepo) Delete(_ *model.Following) error {
	return errors.New("following delete failed")
}

// deleteFailFollowRequestRepo causes Delete on followRequestRepo to fail (for handleReject CancelRequest).
type deleteFailFollowRequestRepo struct {
	*testutil.MockFollowRequestRepository
}

func (r *deleteFailFollowRequestRepo) Delete(_ *model.FollowRequest) error {
	return errors.New("request delete failed")
}

func TestProcess_UndoUnknownInnerType(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type": "Move"}
	}`)
	err := p.Process(undo)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

// extractLocalNoteID with urls==nil branch.
func TestResolveNote_NilURLs(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", URI: &uri}
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, nil, &stubFetcher{}, idGen)
	got, err := r.ResolveNote(uri)
	require.NoError(t, err)
	assert.Equal(t, "n1", got.ID)
}

// handleUndoLike: reactionService.Delete returns a non-NotFound error.
func TestProcess_UndoLikeDeleteError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	mockReactionRepo := testutil.NewMockNoteReactionRepository()
	// 既存リアクションを 1 つ仕込んでおく (FindByPair が成功するように)
	mockReactionRepo.Reactions["r1"] = &model.NoteReaction{ID: "r1", UserID: "alice-id", NoteID: "n1", Reaction: "👍"}
	reactionRepo := &deleteFailReactionRepo{MockNoteReactionRepository: mockReactionRepo}
	emojiRepo := testutil.NewMockEmojiRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	// alice を予め repo に置いておく (resolver が新規作成しないように)
	aliceURI := "https://remote.example/users/alice"
	userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}
	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	reactionSvc := corereaction.NewService(noteRepo, reactionRepo, emojiRepo, followingRepo, idGen)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	p := federation.NewProcessor(resolver, followingSvc, reactionSvc, deleteSvc, userRepo, noteRepo)

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Like","object":"https://example.com/notes/n1"}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

// handleUndoAnnounce: ListRenotesOf returns error.
func TestProcess_UndoAnnounceListError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	mockNoteRepo := testutil.NewMockNoteRepository()
	mockNoteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	noteRepo := &listRenotesFailRepo{MockNoteRepository: mockNoteRepo}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, testutil.NewMockFollowingRepository(), testutil.NewMockFollowRequestRepository(), idGen)
	p := federation.NewProcessor(resolver, followingSvc, nil, nil, userRepo, noteRepo)

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Announce","object":"https://example.com/notes/n1"}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

// handleUndoAnnounce: skip renote owned by another user / skip non-pure renote / Delete error.
func TestProcess_UndoAnnounceSkipsAndDeleteError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	mockNoteRepo := testutil.NewMockNoteRepository()
	target := &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	mockNoteRepo.Notes["n1"] = target
	// alice (announcer) を予め登録 → resolver は新規作成しない
	aliceURI := "https://remote.example/users/alice"
	userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}
	// renote 1: 別ユーザー (skip 対象)
	mockNoteRepo.Notes["r-other"] = &model.Note{ID: "r-other", UserID: "other", RenoteID: &target.ID}
	// renote 2: alice の quote renote (text あり: pure renote ではない)
	text := "quote!"
	mockNoteRepo.Notes["r-quote"] = &model.Note{ID: "r-quote", UserID: "alice-id", RenoteID: &target.ID, Text: &text}
	// renote 3: alice の pure renote → これが削除対象だが Delete エラーで失敗
	mockNoteRepo.Notes["r-pure"] = &model.Note{ID: "r-pure", UserID: "alice-id", RenoteID: &target.ID}

	noteRepo := &deleteFailNoteRepo{MockNoteRepository: mockNoteRepo}

	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, testutil.NewMockFollowingRepository(), testutil.NewMockFollowRequestRepository(), idGen)
	p := federation.NewProcessor(resolver, followingSvc, nil, nil, userRepo, noteRepo)

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Announce","object":"https://example.com/notes/n1"}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

// handleUndoAnnounce: only non-matching renotes exist → loop completes without
// deleting anything but exercises both continue branches.
func TestProcess_UndoAnnounceNoMatchSkips(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	target := &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	env.noteRepo.Notes["n1"] = target
	// alice (announcer) を予め登録
	aliceURI := "https://remote.example/users/alice"
	env.userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}
	// renote by other user (skip 対象)
	env.noteRepo.Notes["r-other"] = &model.Note{ID: "r-other", UserID: "other", RenoteID: &target.ID}
	// alice の quote renote (pure ではない → skip)
	text := "quote!"
	env.noteRepo.Notes["r-quote"] = &model.Note{ID: "r-quote", UserID: "alice-id", RenoteID: &target.ID, Text: &text}

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Announce","object":"https://example.com/notes/n1"}
	}`)
	require.NoError(t, env.processor.Process(undo))
}

// handleLike: reactionService.Create returns a non-AlreadyReacted error
// (use a followers-only target so it returns ErrNoteNotVisible).
func TestProcess_LikeNotVisible(t *testing.T) {
	env := newFullProcessor(t, aliceActor)
	env.noteRepo.Notes["n1"] = &model.Note{
		ID:         "n1",
		UserID:     "bob",
		Visibility: model.NoteVisibilityFollowers,
	}
	body := []byte(`{
		"type": "Like",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "👍"
	}`)
	err := env.processor.Process(body)
	assert.Error(t, err)
}

// handleReject: Unfollow returns a non-NotFollowing error.
func TestProcess_RejectUnfollowError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	mockFR := testutil.NewMockFollowingRepository()
	// pre-existing follow record so Unfollow finds it
	mockFR.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "bob", FolloweeID: "alice-id"}
	followingRepo := &deleteFailFollowingRepo{MockFollowingRepository: mockFR}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	bobURI := "https://example.com/users/bob"
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}
	aliceURI := "https://remote.example/users/alice"
	userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}

	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	p := federation.NewProcessor(resolver, followingSvc, nil, nil, userRepo, noteRepo)

	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://example.com/users/bob",
			"object": "https://remote.example/users/alice"
		}
	}`)
	err := p.Process(body)
	assert.Error(t, err)
}

// handleReject: CancelRequest returns a non-NotRequest error.
func TestProcess_RejectCancelError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	mockFR := testutil.NewMockFollowingRepository()
	mockFRR := testutil.NewMockFollowRequestRepository()
	// pending request の存在: FindByPair が成功するように仕込む
	mockFRR.Requests["fr1"] = &model.FollowRequest{ID: "fr1", FollowerID: "bob", FolloweeID: "alice-id"}
	frRepo := &deleteFailFollowRequestRepo{MockFollowRequestRepository: mockFRR}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	bobURI := "https://example.com/users/bob"
	userRepo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}
	aliceURI := "https://remote.example/users/alice"
	userRepo.Users["alice-id"] = &model.User{ID: "alice-id", Username: "alice", URI: &aliceURI}

	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, mockFR, frRepo, idGen)
	p := federation.NewProcessor(resolver, followingSvc, nil, nil, userRepo, noteRepo)

	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://example.com/users/bob",
			"object": "https://remote.example/users/alice"
		}
	}`)
	err := p.Process(body)
	assert.Error(t, err)
}

// --- JSON-LD normalization (Step I) ------------------------------------------

// Process accepts activities whose keys use the "as:" prefix or full IRI form.
// activitypub.Normalize はそれらを canonical な短形式に変換する。
func TestProcess_NormalizesPrefixedFollow(t *testing.T) {
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"as:type": "Follow",
		"as:actor": "https://remote.example/users/alice",
		"as:object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, followingRepo.Followings, 1)
}

func TestProcess_NormalizesIRIFollow(t *testing.T) {
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"https://www.w3.org/ns/activitystreams#type": "Follow",
		"https://www.w3.org/ns/activitystreams#actor": "https://remote.example/users/alice",
		"https://www.w3.org/ns/activitystreams#object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, followingRepo.Followings, 1)
}

func TestProcess_NormalizesTypeArray(t *testing.T) {
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": ["Follow", "https://example.com/SomethingElse"],
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, followingRepo.Followings, 1)
}

func TestProcess_NormalizesObjectIDShortcut(t *testing.T) {
	// {"object": {"@id": "..."}} 形式は string に縮約される
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": {"@id": "https://example.com/users/bob"}
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, followingRepo.Followings, 1)
}

func TestProcess_AnnounceCreateError(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	noteRepo := &noteCreateFailRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", Visibility: model.NoteVisibilityPublic}
	emojiRepo := testutil.NewMockEmojiRepository()
	reactionRepo := testutil.NewMockNoteReactionRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(userRepo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(userRepo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	reactionSvc := corereaction.NewService(noteRepo, reactionRepo, emojiRepo, followingRepo, idGen)
	deleteSvc := corenote.NewDeleteService(noteRepo)
	p := federation.NewProcessor(resolver, followingSvc, reactionSvc, deleteSvc, userRepo, noteRepo)

	body := []byte(`{
		"type": "Announce",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1"
	}`)
	err := p.Process(body)
	assert.Error(t, err)
}
