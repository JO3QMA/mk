package federation_test

import (
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

func TestProcess_FlagWithoutRepo(t *testing.T) {
	p, _, _, _ := newProcessor(t, aliceActor)
	body := []byte(`{
		"type": "Flag",
		"actor": "https://remote.example/users/alice",
		"object": ["https://example.com/users/bob"]
	}`)
	assert.ErrorIs(t, p.Process(body), federation.ErrUnsupportedActivity)
}
