package federation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	coreblocking "github.com/shiroha-a/mk/internal/core/blocking"
	"github.com/shiroha-a/mk/internal/core/federation"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const aliceActor = `{
	"id": "https://remote.example/users/alice",
	"type": "Person",
	"preferredUsername": "alice",
	"name": "Alice",
	"inbox": "https://remote.example/users/alice/inbox",
	"publicKey": {"publicKeyPem": "PEM"}
}`

func newProcessor(t *testing.T, fetcherBody string) (*federation.Processor, *testutil.MockUserRepository, *testutil.MockFollowingRepository, *testutil.MockNoteRepository) {
	t.Helper()
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(fetcherBody)}, idGen)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	processor := federation.NewProcessor(resolver, followingSvc, nil, nil, repo, noteRepo)
	return processor, repo, followingRepo, noteRepo
}

func TestProcess_FollowHappyPath(t *testing.T) {
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	// 受信側の自分 (local) を登録
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, followingRepo.Followings, 1)
}

// stubInboundFollowAcceptor records SendAcceptForInboundFollow calls for
// assertion.
type stubInboundFollowAcceptor struct {
	calls []struct {
		followerID, followeeID string
		followRaw              string
	}
}

func (s *stubInboundFollowAcceptor) SendAcceptForInboundFollow(follower, followee *model.User, raw json.RawMessage) error {
	s.calls = append(s.calls, struct {
		followerID, followeeID string
		followRaw              string
	}{follower.ID, followee.ID, string(raw)})
	return nil
}

func TestProcess_FollowInvokesAcceptorWithOriginalRaw(t *testing.T) {
	// inbound Follow が成立 (Following 作成) したら、original Follow の raw を
	// そのまま InboundFollowAcceptor に渡す。相手は自分の送った Follow.id と
	// 照合できるようになる。
	p, repo, _, _ := newProcessor(t, aliceActor)
	p.SetLocalBaseURL("https://example.com")
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob"}
	acceptor := &stubInboundFollowAcceptor{}
	p.SetInboundFollowAcceptor(acceptor)

	body := []byte(`{
		"type": "Follow",
		"id": "https://remote.example/follows/abc123",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))
	require.Len(t, acceptor.calls, 1)
	assert.Equal(t, "bob", acceptor.calls[0].followeeID)
	// original の id を含んだ raw JSON が渡っていること。
	assert.Contains(t, acceptor.calls[0].followRaw, "https://remote.example/follows/abc123")
}

func TestProcess_FollowLocalUserByIDResolution(t *testing.T) {
	// 実本番のローカルユーザー row は user.uri が NULL のまま保存されるため、
	// FindByURI では解決できない。localBaseURL が設定されていれば
	// "{baseURL}/users/{id}" 形式の URI から ID を抜き出して FindByID で
	// lookup する経路が通るはず。
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	p.SetLocalBaseURL("https://example.com")
	// URI を持たないローカルユーザー
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob"}

	body := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))
	require.Len(t, followingRepo.Followings, 1)
	for _, f := range followingRepo.Followings {
		assert.Equal(t, "bob", f.FolloweeID)
	}
}

func TestProcess_FollowAlreadyFollowing(t *testing.T) {
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}
	// alice → bob を resolver で予め解決させる
	body := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))

	// 2回目: ErrAlreadyFollowing は飲み込まれる
	require.NoError(t, p.Process(body))
	assert.Len(t, followingRepo.Followings, 1)
}

func TestProcess_FollowUnknownFollowee(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/ghost"
	}`)
	err := p.Process(body)
	assert.Error(t, err)
}

func TestProcess_FollowResolveError(t *testing.T) {
	p, _, _, _ := newProcessor(t, "{not json")
	body := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	err := p.Process(body)
	assert.Error(t, err)
}

func TestProcess_FollowMissingObject(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{"type":"Follow","actor":"https://remote.example/users/alice"}`)
	err := p.Process(body)
	assert.Error(t, err)
}

func TestProcess_UndoFollow(t *testing.T) {
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	// First Follow
	follow := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(follow))
	require.Len(t, followingRepo.Followings, 1)

	// Then Undo
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://remote.example/users/alice",
			"object": "https://example.com/users/bob"
		}
	}`)
	require.NoError(t, p.Process(undo))
	assert.Empty(t, followingRepo.Followings)
}

func TestProcess_UndoUnknownType(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Like"}
	}`)
	err := p.Process(undo)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

func TestProcess_UndoBadInner(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": "not-a-json-object"
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoNotFollowing(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://remote.example/users/alice",
			"object": "https://example.com/users/bob"
		}
	}`)
	// 既にフォロー解除済みでもエラーにならない
	require.NoError(t, p.Process(undo))
}

func TestProcess_UndoResolveError(t *testing.T) {
	p, _, _, _ := newProcessor(t, "{not json")
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Follow","actor":"x","object":"https://example.com/users/bob"}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoUnknownFollowee(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Follow","actor":"x","object":"https://example.com/users/ghost"}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoMissingObject(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {"type":"Follow","actor":"x"}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

func TestProcess_Accept(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{"type":"Accept","actor":"https://remote.example/users/alice","object":{}}`)
	require.NoError(t, p.Process(body))
}

func TestProcess_UnsupportedType(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{"type":"Like","actor":"https://remote.example/users/alice"}`)
	assert.ErrorIs(t, p.Process(body), federation.ErrUnsupportedActivity)
}

func TestProcess_UnknownType(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{"type":"TentativeReject","actor":"https://remote.example/users/alice"}`)
	assert.ErrorIs(t, p.Process(body), federation.ErrUnsupportedActivity)
}

func TestProcess_BadJSON(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	err := p.Process([]byte(`{not json`))
	assert.Error(t, err)
}

func TestProcess_MissingActor(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	err := p.Process([]byte(`{"type":"Follow"}`))
	assert.Error(t, err)
}

// failingFollowingService isn't easy to construct since it's a concrete type;
// we exercise the non-AlreadyFollowing follow error path by using a follower
// who doesn't exist yet (resolver create works but follow service errors out
// because follower==followee). Use SelfFollow.
func TestProcess_UndoFollowWithNestedObject(t *testing.T) {
	p, repo, followingRepo, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	// First follow
	follow := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(follow))
	require.Len(t, followingRepo.Followings, 1)

	// Undo with nested object form (object: {id: ...})
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"object": {"id": "https://example.com/users/bob"}
		}
	}`)
	require.NoError(t, p.Process(undo))
	assert.Empty(t, followingRepo.Followings)
}

func TestProcess_UndoNestedObjectInvalid(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	// nested object missing id field
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"object": {"foo": "bar"}
		}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

func TestProcess_UndoNestedObjectBadJSON(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	// nested object is a JSON number — not string and not object
	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"object": 42
		}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

// failingFollowingRepo causes Delete to fail (non-NotFollowing path).
type failingFollowingRepo struct {
	*testutil.MockFollowingRepository
}

func (f *failingFollowingRepo) Delete(_ *model.Following) error {
	return errors.New("delete failed")
}

func TestProcess_UndoFollowDeleteError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	mockFR := testutil.NewMockFollowingRepository()
	// pre-existing follow record so Unfollow finds it
	mockFR.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "alice", FolloweeID: "bob"}
	followingRepo := &failingFollowingRepo{MockFollowingRepository: mockFR}

	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	p := federation.NewProcessor(resolver, followingSvc, nil, nil, repo, noteRepo)

	// Setup followee bob
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}
	// Pre-resolve alice to ID "alice" by first calling resolver
	aliceURI := "https://remote.example/users/alice"
	repo.Users["alice"] = &model.User{ID: "alice", Username: "alice", URI: &aliceURI}

	undo := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"object": "https://example.com/users/bob"
		}
	}`)
	err := p.Process(undo)
	assert.Error(t, err)
}

func TestProcess_FollowSelfFollow(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	// alice/bob 同IDのケース: alice as both actor and target via URI alias
	uri := "https://remote.example/users/alice"
	// alice already in repo (resolver populates) ... we register the same user
	// as a local "followee" by setting URI.
	repo.Users["alice-local"] = &model.User{ID: "alice-local", Username: "alice", URI: &uri}
	// 直接呼ぶことで follower==followee を発生させる
	body := []byte(`{
		"type": "Follow",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/users/alice"
	}`)
	err := p.Process(body)
	// SelfFollow がそのまま伝搬する
	assert.Error(t, err)
}

// --- Block ---

func newProcessorWithBlocking(t *testing.T) (*federation.Processor, *testutil.MockUserRepository, *testutil.MockBlockingRepository) {
	t.Helper()
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	blockingRepo := testutil.NewMockBlockingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	blockingSvc := coreblocking.NewService(repo, blockingRepo, followingRepo, idGen)
	processor := federation.NewProcessor(resolver, followingSvc, nil, nil, repo, noteRepo)
	processor.SetBlockingService(blockingSvc)
	return processor, repo, blockingRepo
}

func TestProcess_BlockHappyPath(t *testing.T) {
	p, repo, blockingRepo := newProcessorWithBlocking(t)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": "Block",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))
	// resolverがaliceを解決した際に生成されたIDを取得
	var aliceID string
	for _, u := range repo.Users {
		if u.Username == "alice" {
			aliceID = u.ID
			break
		}
	}
	require.NotEmpty(t, aliceID)
	exists, _ := blockingRepo.Exists(aliceID, "bob")
	assert.True(t, exists)
}

func TestProcess_BlockAlreadyBlocking(t *testing.T) {
	p, repo, _ := newProcessorWithBlocking(t)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": "Block",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	// 2回ブロックしてもエラーにならない（冪等）
	require.NoError(t, p.Process(body))
	require.NoError(t, p.Process(body))
}

func TestProcess_BlockRemoteUser_Ignored(t *testing.T) {
	p, repo, _ := newProcessorWithBlocking(t)
	host := "remote2.example"
	charlieURI := "https://remote2.example/users/charlie"
	repo.Users["charlie"] = &model.User{ID: "charlie", Username: "charlie", URI: &charlieURI, Host: &host}

	body := []byte(`{
		"type": "Block",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote2.example/users/charlie"
	}`)
	// リモートユーザーへのブロックは無視
	require.NoError(t, p.Process(body))
}

func TestProcess_UndoBlock(t *testing.T) {
	p, repo, blockingRepo := newProcessorWithBlocking(t)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	// まずブロック
	body := []byte(`{
		"type": "Block",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	require.NoError(t, p.Process(body))

	// resolverが生成したaliceのIDを取得
	var aliceID string
	for _, u := range repo.Users {
		if u.Username == "alice" {
			aliceID = u.ID
			break
		}
	}

	// Undo Block
	undoBody := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Block",
			"actor": "https://remote.example/users/alice",
			"object": "https://example.com/users/bob"
		}
	}`)
	require.NoError(t, p.Process(undoBody))
	exists, _ := blockingRepo.Exists(aliceID, "bob")
	assert.False(t, exists)
}

func TestProcess_UndoBlock_NotBlocking(t *testing.T) {
	p, repo, _ := newProcessorWithBlocking(t)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	// ブロックしていない状態でUndo Blockしてもエラーにならない
	undoBody := []byte(`{
		"type": "Undo",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Block",
			"actor": "https://remote.example/users/alice",
			"object": "https://example.com/users/bob"
		}
	}`)
	require.NoError(t, p.Process(undoBody))
}

// --- Flag ---

func TestProcess_FlagHappyPath(t *testing.T) {
	p, repo, _ := newProcessorWithBlocking(t)
	abuseRepo := testutil.NewMockAbuseReportRepository()
	idGenFlag, _ := id.NewGenerator("aidx")
	p.SetAbuseReportRepo(abuseRepo, idGenFlag)

	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": "Flag",
		"actor": "https://remote.example/users/alice",
		"object": ["https://example.com/users/bob"],
		"content": "spam"
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, abuseRepo.Reports, 1)
	for _, r := range abuseRepo.Reports {
		assert.Equal(t, "spam", r.Comment)
	}
}

func TestProcess_FlagSingleURI(t *testing.T) {
	p, repo, _ := newProcessorWithBlocking(t)
	abuseRepo := testutil.NewMockAbuseReportRepository()
	idGenFlag, _ := id.NewGenerator("aidx")
	p.SetAbuseReportRepo(abuseRepo, idGenFlag)

	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": "Flag",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob",
		"content": "abuse"
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, abuseRepo.Reports, 1)
}

func TestProcess_FlagFromNote(t *testing.T) {
	p, repo, _ := newProcessorWithBlocking(t)
	abuseRepo := testutil.NewMockAbuseReportRepository()
	idGenFlag, _ := id.NewGenerator("aidx")
	p.SetAbuseReportRepo(abuseRepo, idGenFlag)
	noteRepo := testutil.NewMockNoteRepository()

	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}
	noteURI := "https://example.com/notes/n1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "bob", URI: &noteURI}

	// noteRepoはProcessor内のものとは別なので、直接ユーザーURIで解決する
	body := []byte(`{
		"type": "Flag",
		"actor": "https://remote.example/users/alice",
		"object": ["https://example.com/users/bob"],
		"content": "reported note"
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, abuseRepo.Reports, 1)
}

// --- Move ---

func TestProcess_Move(t *testing.T) {
	p, _, _ := newProcessorWithBlocking(t)
	// Moveはactorの再解決のみ
	body := []byte(`{
		"type": "Move",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/users/alice"
	}`)
	require.NoError(t, p.Process(body))
}

// --- EmojiReaction ---

func TestProcess_EmojiReaction(t *testing.T) {
	p, repo, _ := newProcessorWithBlocking(t)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	// EmojiReaction typeはLikeと同じハンドラにルーティングされる
	// reactionServiceがnilなので ErrUnsupportedActivity が返る
	body := []byte(`{
		"type": "EmojiReaction",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "👍"
	}`)
	err := p.Process(body)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

func TestProcess_EmojiReact(t *testing.T) {
	p, _, _ := newProcessorWithBlocking(t)
	body := []byte(`{
		"type": "EmojiReact",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/notes/n1",
		"content": "❤️"
	}`)
	err := p.Process(body)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

// --- Block nil service ---

func TestProcess_BlockWithoutService(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Block",
		"actor": "https://remote.example/users/alice",
		"object": "https://example.com/users/bob"
	}`)
	assert.ErrorIs(t, p.Process(body), federation.ErrUnsupportedActivity)
}

// --- Add/Remove ---

func newProcessorWithPinning(t *testing.T) (*federation.Processor, *testutil.MockUserRepository, *testutil.MockNoteRepository, *testutil.MockUserNotePiningRepository) {
	t.Helper()
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	piningRepo := testutil.NewMockUserNotePiningRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen)
	processor := federation.NewProcessor(resolver, followingSvc, nil, nil, repo, noteRepo)
	processor.SetPinningRepo(piningRepo, idGen)
	return processor, repo, noteRepo, piningRepo
}

// resolveAliceAndSetFeatured はalice actorを事前解決してFeaturedフィールドを設定する。
func resolveAliceAndSetFeatured(t *testing.T, p *federation.Processor, repo *testutil.MockUserRepository) string {
	t.Helper()
	// Followでaliceを解決させる（ダミーのfollowee不要、ResolveActorが呼ばれれば良い）
	dummyURI := "https://example.com/users/dummy"
	repo.Users["dummy"] = &model.User{ID: "dummy", Username: "dummy", URI: &dummyURI}
	_ = p.Process([]byte(`{"type":"Follow","actor":"https://remote.example/users/alice","object":"https://example.com/users/dummy"}`))

	var aliceID string
	featured := "https://remote.example/users/alice/collections/featured"
	for _, u := range repo.Users {
		if u.Username == "alice" {
			aliceID = u.ID
			u.Featured = &featured
			break
		}
	}
	require.NotEmpty(t, aliceID)
	return aliceID
}

func TestProcess_Add(t *testing.T) {
	p, repo, noteRepo, piningRepo := newProcessorWithPinning(t)
	_ = resolveAliceAndSetFeatured(t, p, repo)

	noteURI := "https://remote.example/notes/n1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "alice", URI: &noteURI}

	body := []byte(`{
		"type": "Add",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1",
		"target": "https://remote.example/users/alice/collections/featured"
	}`)
	require.NoError(t, p.Process(body))
	assert.True(t, len(piningRepo.Pinings) > 0)
}

func TestProcess_Add_WrongTarget(t *testing.T) {
	p, _, _, piningRepo := newProcessorWithPinning(t)
	// targetがfeaturedでない場合は無視
	body := []byte(`{
		"type": "Add",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1",
		"target": "https://remote.example/users/alice/collections/other"
	}`)
	require.NoError(t, p.Process(body))
	assert.Empty(t, piningRepo.Pinings)
}

func TestProcess_Remove(t *testing.T) {
	p, repo, noteRepo, piningRepo := newProcessorWithPinning(t)
	_ = resolveAliceAndSetFeatured(t, p, repo)

	noteURI := "https://remote.example/notes/n1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "alice", URI: &noteURI}

	// まずAddでピン留め
	addBody := []byte(`{
		"type": "Add",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1",
		"target": "https://remote.example/users/alice/collections/featured"
	}`)
	require.NoError(t, p.Process(addBody))
	require.True(t, len(piningRepo.Pinings) > 0)

	// Remove
	removeBody := []byte(`{
		"type": "Remove",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1",
		"target": "https://remote.example/users/alice/collections/featured"
	}`)
	require.NoError(t, p.Process(removeBody))
}

func TestProcess_Remove_NotPinned(t *testing.T) {
	p, _, noteRepo, _ := newProcessorWithPinning(t)
	noteURI := "https://remote.example/notes/n1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", UserID: "alice", URI: &noteURI}

	// ピン留めしていないノートのRemoveはエラーにならない
	body := []byte(`{
		"type": "Remove",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1",
		"target": "https://remote.example/users/alice/collections/featured"
	}`)
	require.NoError(t, p.Process(body))
}

func TestProcess_AddWithoutRepo(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Add",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1",
		"target": "https://remote.example/users/alice/collections/featured"
	}`)
	assert.ErrorIs(t, p.Process(body), federation.ErrUnsupportedActivity)
}

func TestProcess_RemoveWithoutRepo(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Remove",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/notes/n1",
		"target": "https://remote.example/users/alice/collections/featured"
	}`)
	assert.ErrorIs(t, p.Process(body), federation.ErrUnsupportedActivity)
}

// --- Accept ---

func TestProcess_AcceptFollow(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	// ローカルユーザーbob（follower）を登録
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob"}

	// aliceからのAcceptを処理（bobからaliceへのFollow承認）
	// ExtractLocalUserIDで /users/bob → ID "bob" と解決される
	acceptBody := []byte(`{
		"type": "Accept",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://example.com/users/bob",
			"object": "https://remote.example/users/alice"
		}
	}`)
	// AcceptRequest自体はフォローリクエストが存在しなくてもエラーにならない（nil返却）
	require.NoError(t, p.Process(acceptBody))
}

func TestProcess_AcceptNonFollow(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	// inner objectがFollowでない場合は無視
	body := []byte(`{
		"type": "Accept",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Like",
			"actor": "https://example.com/users/bob"
		}
	}`)
	require.NoError(t, p.Process(body))
}

func TestProcess_AcceptBadObject(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	// objectが文字列（Follow IDのURI）の場合は無視
	body := []byte(`{
		"type": "Accept",
		"actor": "https://remote.example/users/alice",
		"object": "https://remote.example/follows/123"
	}`)
	require.NoError(t, p.Process(body))
}

func TestProcess_AcceptUnknownFollower(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	// followerが見つからない場合は無視
	body := []byte(`{
		"type": "Accept",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Follow",
			"actor": "https://example.com/users/ghost",
			"object": "https://remote.example/users/alice"
		}
	}`)
	require.NoError(t, p.Process(body))
}

// --- Question/Poll ---

func TestProcess_CreateQuestion(t *testing.T) {
	p, repo, _, noteRepo := newProcessor(t, aliceActor)
	// resolverにpollRepoを注入するためresolverへのアクセスが必要
	// newProcessorで作成されたresolverにSetPollRepoできないため、
	// processor経由でresolverを取得する方法がない。代わりにfull setup。
	pollRepo := testutil.NewMockPollRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen2, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen2)
	resolver.SetPollRepo(pollRepo)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen2)
	p = federation.NewProcessor(resolver, followingSvc, nil, nil, repo, noteRepo)

	body := []byte(`{
		"type": "Create",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Question",
			"id": "https://remote.example/notes/q1",
			"attributedTo": "https://remote.example/users/alice",
			"content": "Best language?",
			"oneOf": [
				{"type": "Note", "name": "Go", "replies": {"type": "Collection", "totalItems": 5}},
				{"type": "Note", "name": "Rust", "replies": {"type": "Collection", "totalItems": 3}}
			],
			"endTime": "2026-12-31T23:59:59Z",
			"to": ["https://www.w3.org/ns/activitystreams#Public"]
		}
	}`)
	require.NoError(t, p.Process(body))
	// ノートが作成されていること
	assert.True(t, len(noteRepo.Notes) > 0)
	// Pollが作成されていること
	assert.Len(t, pollRepo.Polls, 1)
	for _, poll := range pollRepo.Polls {
		assert.False(t, poll.Multiple) // oneOf → single choice
		assert.Len(t, poll.Choices, 2)
		assert.Equal(t, "Go", poll.Choices[0])
		assert.Equal(t, "Rust", poll.Choices[1])
	}
}

func TestProcess_CreateQuestionAnyOf(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	pollRepo := testutil.NewMockPollRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen2, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen2)
	resolver.SetPollRepo(pollRepo)
	followingSvc := corefollowing.NewService(repo, followingRepo, testutil.NewMockFollowRequestRepository(), idGen2)
	p := federation.NewProcessor(resolver, followingSvc, nil, nil, repo, noteRepo)

	body := []byte(`{
		"type": "Create",
		"actor": "https://remote.example/users/alice",
		"object": {
			"type": "Question",
			"id": "https://remote.example/notes/q2",
			"attributedTo": "https://remote.example/users/alice",
			"content": "Pick all that apply",
			"anyOf": [
				{"type": "Note", "name": "A"},
				{"type": "Note", "name": "B"},
				{"type": "Note", "name": "C"}
			],
			"closed": "2026-12-31T23:59:59Z",
			"to": ["https://www.w3.org/ns/activitystreams#Public"]
		}
	}`)
	require.NoError(t, p.Process(body))
	assert.Len(t, pollRepo.Polls, 1)
	for _, poll := range pollRepo.Polls {
		assert.True(t, poll.Multiple) // anyOf → multiple choice
		assert.Len(t, poll.Choices, 3)
		assert.NotNil(t, poll.ExpiresAt) // closedから設定
	}
}

// --- ExtractLocalUserID ---

func TestExtractLocalUserID(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	resolver := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(aliceActor)}, idGen)

	assert.Equal(t, "abc123", resolver.ExtractLocalUserID("https://example.com/users/abc123"))
	assert.Equal(t, "", resolver.ExtractLocalUserID("https://remote.example/users/abc123"))
	assert.Equal(t, "", resolver.ExtractLocalUserID(""))
}

func TestProcess_FlagWithoutRepo(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Flag",
		"actor": "https://remote.example/users/alice",
		"object": ["https://example.com/users/bob"]
	}`)
	assert.ErrorIs(t, p.Process(body), federation.ErrUnsupportedActivity)
}

// fakeRelayMarker records MarkAccepted/MarkRejected calls for assertion.
type fakeRelayMarker struct {
	accepted []string
	rejected []string
	err      error
}

func (f *fakeRelayMarker) MarkAccepted(_ context.Context, id string) error {
	f.accepted = append(f.accepted, id)
	return f.err
}

func (f *fakeRelayMarker) MarkRejected(_ context.Context, id string) error {
	f.rejected = append(f.rejected, id)
	return f.err
}

func TestProcess_AcceptFollowRelay_MarksAccepted(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	marker := &fakeRelayMarker{}
	p.SetRelayMarker(marker)

	body := []byte(`{
		"type": "Accept",
		"actor": "https://remote.example/users/alice",
		"object": {
			"id": "https://example.com/activities/follow-relay/rel123",
			"type": "Follow",
			"actor": "https://example.com/users/relay.actor",
			"object": "https://www.w3.org/ns/activitystreams#Public"
		}
	}`)
	require.NoError(t, p.Process(body))
	assert.Equal(t, []string{"rel123"}, marker.accepted)
	assert.Empty(t, marker.rejected)
}

func TestProcess_RejectFollowRelay_MarksRejected(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	marker := &fakeRelayMarker{}
	p.SetRelayMarker(marker)

	body := []byte(`{
		"type": "Reject",
		"actor": "https://remote.example/users/alice",
		"object": {
			"id": "https://example.com/activities/follow-relay/rel456",
			"type": "Follow"
		}
	}`)
	require.NoError(t, p.Process(body))
	assert.Equal(t, []string{"rel456"}, marker.rejected)
}

func TestProcess_AcceptNonRelay_IgnoresMarker(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	marker := &fakeRelayMarker{}
	p.SetRelayMarker(marker)
	bobURI := "https://example.com/users/bob"
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob", URI: &bobURI}

	body := []byte(`{
		"type": "Accept",
		"actor": "https://remote.example/users/alice",
		"object": {
			"id": "https://remote.example/activities/follow/xyz",
			"type": "Follow",
			"actor": "https://example.com/users/bob",
			"object": "https://remote.example/users/alice"
		}
	}`)
	// フォローリクエストが無いので内部的には ErrRequestNotFound を飲み込む
	require.NoError(t, p.Process(body))
	// relay marker は呼ばれない
	assert.Empty(t, marker.accepted)
}

// --- Chat federation (Misskey:ChatMessage) ---

type stubChatReceiver struct {
	called    int
	lastURI   string
	lastFrom  *model.User
	lastTo    string
	lastText  string
	returnErr error
}

func (s *stubChatReceiver) CreateMessageViaAP(_ context.Context, uri string, fromUser *model.User, toUserID, text string) (*model.ChatMessage, error) {
	s.called++
	s.lastURI = uri
	s.lastFrom = fromUser
	s.lastTo = toUserID
	s.lastText = text
	if s.returnErr != nil {
		return nil, s.returnErr
	}
	return &model.ChatMessage{ID: "cm_ap_1"}, nil
}

func TestProcess_ChatMessage_HappyPath(t *testing.T) {
	p, repo, _, _ := newProcessor(t, aliceActor)
	// ローカルユーザーはURI==nil (本番と同じ)。ExtractLocalUserID→FindByIDで解決。
	repo.Users["bob"] = &model.User{ID: "bob", Username: "bob"}
	chatSvc := &stubChatReceiver{}
	p.SetChatService(chatSvc)

	body := []byte(`{
		"id": "https://remote.example/chat-messages/1",
		"type": "Misskey:ChatMessage",
		"actor": "https://remote.example/users/alice",
		"attributedTo": "https://remote.example/users/alice",
		"to": "https://example.com/users/bob",
		"content": "hello via AP"
	}`)
	require.NoError(t, p.Process(body))
	assert.Equal(t, 1, chatSvc.called)
	assert.Equal(t, "https://remote.example/chat-messages/1", chatSvc.lastURI)
	assert.Equal(t, "bob", chatSvc.lastTo)
	assert.Equal(t, "hello via AP", chatSvc.lastText)
}

func TestProcess_ChatMessage_NoChatService(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"id": "https://remote.example/chat-messages/2",
		"type": "Misskey:ChatMessage",
		"actor": "https://remote.example/users/alice",
		"attributedTo": "https://remote.example/users/alice",
		"to": "https://example.com/users/bob",
		"content": "hi"
	}`)
	err := p.Process(body)
	assert.ErrorIs(t, err, federation.ErrUnsupportedActivity)
}

func TestProcess_ChatMessage_ActorMismatch(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	chatSvc := &stubChatReceiver{}
	p.SetChatService(chatSvc)
	body := []byte(`{
		"id": "https://remote.example/chat-messages/3",
		"type": "Misskey:ChatMessage",
		"actor": "https://remote.example/users/alice",
		"attributedTo": "https://remote.example/users/mallory",
		"to": "https://example.com/users/bob",
		"content": "spoof"
	}`)
	err := p.Process(body)
	assert.Error(t, err)
	assert.Equal(t, 0, chatSvc.called)
}

func TestProcess_ChatMessage_MissingTo(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	chatSvc := &stubChatReceiver{}
	p.SetChatService(chatSvc)
	body := []byte(`{
		"id": "https://remote.example/chat-messages/4",
		"type": "Misskey:ChatMessage",
		"actor": "https://remote.example/users/alice",
		"attributedTo": "https://remote.example/users/alice",
		"content": "no to"
	}`)
	err := p.Process(body)
	assert.Error(t, err)
}
