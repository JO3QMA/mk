package federation_test

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
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
	body := []byte(`{"type":"Move","actor":"https://remote.example/users/alice"}`)
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
