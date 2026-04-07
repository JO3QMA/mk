package notification

import (
	"context"
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHook(t *testing.T) (*Hook, *Service, *testutil.MockUserRepository) {
	t.Helper()
	testRedis.FlushAll(context.Background())
	svc := NewService(testRedis.Client, idGen)
	userRepo := testutil.NewMockUserRepository()
	return NewHook(svc, userRepo), svc, userRepo
}

// addLocalUser registers a user with no host so notifyLocalUser delivers.
func addLocalUser(repo *testutil.MockUserRepository, id, username string) {
	repo.Users[id] = &model.User{ID: id, Username: username, UsernameLower: username}
}

func addRemoteUser(repo *testutil.MockUserRepository, id, username, host string) {
	repo.Users[id] = &model.User{ID: id, Username: username, UsernameLower: username, Host: &host}
}

func TestHook_OnNoteCreated_Nil(t *testing.T) {
	h, _, _ := newTestHook(t)
	h.OnNoteCreated(nil, &model.User{ID: "u"}, nil, nil)
	h.OnNoteCreated(&model.Note{}, nil, nil, nil)
}

func TestHook_OnNoteCreated_ReplyNotification(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	addLocalUser(repo, "bob", "bob")

	parent := &model.Note{ID: "n_parent", UserID: "alice"}
	note := &model.Note{ID: "n_reply", UserID: "bob"}
	h.OnNoteCreated(note, &model.User{ID: "bob"}, parent, nil)

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeReply, out[0].Type)
}

func TestHook_OnNoteCreated_ReplyToSelfSkipped(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")

	parent := &model.Note{ID: "n_parent", UserID: "alice"}
	note := &model.Note{ID: "n_reply", UserID: "alice"}
	h.OnNoteCreated(note, &model.User{ID: "alice"}, parent, nil)

	out, _ := svc.List(context.Background(), "alice", 10)
	assert.Empty(t, out)
}

func TestHook_OnNoteCreated_PureRenote(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	addLocalUser(repo, "bob", "bob")

	target := "n_target"
	original := &model.Note{ID: target, UserID: "alice"}
	note := &model.Note{ID: "n_renote", UserID: "bob", RenoteID: &target}
	h.OnNoteCreated(note, &model.User{ID: "bob"}, nil, original)

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeRenote, out[0].Type)
}

func TestHook_OnNoteCreated_QuoteRenote(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	addLocalUser(repo, "bob", "bob")

	target := "n_target"
	original := &model.Note{ID: target, UserID: "alice"}
	text := "quoted"
	note := &model.Note{ID: "n_quote", UserID: "bob", RenoteID: &target, Text: &text}
	h.OnNoteCreated(note, &model.User{ID: "bob"}, nil, original)

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeQuote, out[0].Type)
}

func TestHook_OnNoteCreated_Mention(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	addLocalUser(repo, "bob", "bob")

	note := &model.Note{
		ID: "n_mention", UserID: "bob",
		Mentions: pq.StringArray{"alice"},
	}
	h.OnNoteCreated(note, &model.User{ID: "bob"}, nil, nil)

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeMention, out[0].Type)
}

func TestHook_OnNoteCreated_MentionSkipsSelfAndUnknown(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "bob", "bob")

	note := &model.Note{
		ID: "n_mention", UserID: "bob",
		Mentions: pq.StringArray{"bob", "ghost", ""},
	}
	h.OnNoteCreated(note, &model.User{ID: "bob"}, nil, nil)

	// 自分自身&存在しない/空文字はスキップされる
	out, _ := svc.List(context.Background(), "bob", 10)
	assert.Empty(t, out)
}

func TestHook_OnNoteCreated_MentionDedupedWithReply(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	addLocalUser(repo, "bob", "bob")

	parent := &model.Note{ID: "n_p", UserID: "alice"}
	note := &model.Note{
		ID: "n_r", UserID: "bob",
		Mentions: pq.StringArray{"alice"},
	}
	h.OnNoteCreated(note, &model.User{ID: "bob"}, parent, nil)

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	// reply通知のみで、mentionは抑制される
	require.Len(t, out, 1)
	assert.Equal(t, TypeReply, out[0].Type)
}

func TestHook_OnFollowed(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	h.OnFollowed("bob", "alice")

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeFollow, out[0].Type)
}

func TestHook_OnFollowRequested(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	h.OnFollowRequested("bob", "alice")

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeReceiveFollowReq, out[0].Type)
}

func TestHook_OnFollowAccepted(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "bob", "bob")
	h.OnFollowAccepted("bob", "alice")

	out, err := svc.List(context.Background(), "bob", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeFollowRequestAccept, out[0].Type)
}

func TestHook_OnReactionCreated(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addLocalUser(repo, "alice", "alice")
	h.OnReactionCreated("alice", "bob", "n1", "👍")

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, TypeReaction, out[0].Type)
	assert.Equal(t, "👍", out[0].Reaction)
}

func TestHook_NotifyRemoteSkipped(t *testing.T) {
	h, svc, repo := newTestHook(t)
	addRemoteUser(repo, "alice", "alice", "remote.example")
	h.OnFollowed("bob", "alice")
	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestHook_NotifyMissingUserSkipped(t *testing.T) {
	h, svc, _ := newTestHook(t)
	h.OnFollowed("bob", "ghost")
	out, _ := svc.List(context.Background(), "ghost", 10)
	assert.Empty(t, out)
}

func TestHook_NotifyWithoutUserRepo(t *testing.T) {
	// userRepo == nil の場合はホストチェックをスキップする
	testRedis.FlushAll(context.Background())
	svc := NewService(testRedis.Client, idGen)
	h := NewHook(svc, nil)
	h.OnFollowed("bob", "alice")

	out, err := svc.List(context.Background(), "alice", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestHook_NotifyServiceErrorLogged(t *testing.T) {
	// closed clientではCreate失敗 → ログ出力されるだけで例外なし
	svc := NewService(closedClient(t), idGen)
	repo := testutil.NewMockUserRepository()
	addLocalUser(repo, "alice", "alice")
	h := NewHook(svc, repo)
	h.OnFollowed("bob", "alice")
}

func TestIsQuote_Variants(t *testing.T) {
	target := "x"
	cases := []struct {
		name string
		n    *model.Note
		want bool
	}{
		{"no renote", &model.Note{}, false},
		{"pure renote", &model.Note{RenoteID: &target}, false},
		{"with text", &model.Note{RenoteID: &target, Text: ptrString("hi")}, true},
		{"with cw", &model.Note{RenoteID: &target, CW: ptrString("warn")}, true},
		{"with file", &model.Note{RenoteID: &target, FileIDs: pq.StringArray{"f1"}}, true},
		{"with poll", &model.Note{RenoteID: &target, HasPoll: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isQuote(tc.n))
		})
	}
}

func ptrString(s string) *string { return &s }
