package move_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/move"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResolver returns preset results and records which URIs were asked.
type fakeResolver struct {
	byURI map[string]*model.User
	err   error
	calls []string
}

func (f *fakeResolver) ResolveActor(uri string) (*model.User, error) {
	f.calls = append(f.calls, uri)
	if f.err != nil {
		return nil, f.err
	}
	return f.byURI[uri], nil
}

// fakeDeliverer captures the activity body for assertions.
type fakeDeliverer struct {
	called    int
	signer    string
	body      []byte
	returnErr error
}

func (f *fakeDeliverer) DeliverToFollowers(signerUserID string, body []byte) error {
	f.called++
	f.signer = signerUserID
	f.body = body
	return f.returnErr
}

func strPtr(s string) *string { return &s }

// newService is a helper that wires all the mocks together. baseURL is fixed
// so tests can predict generated srcURI (urls.UserURI).
func newService(resolver move.Resolver, deliverer move.Deliverer) (*move.Service, *testutil.MockUserRepository, *testutil.MockFollowingRepository) {
	userRepo := testutil.NewMockUserRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://local.example")
	renderer := activitypub.NewRenderer(urls)
	svc := move.NewService(userRepo, followingRepo, urls, renderer, resolver, deliverer)
	return svc, userRepo, followingRepo
}

func TestMove_Success(t *testing.T) {
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"

	dst := &model.User{
		ID:          "dst",
		URI:         strPtr(dstURI),
		AlsoKnownAs: strPtr(srcURI),
	}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	deliverer := &fakeDeliverer{}

	svc, userRepo, _ := newService(resolver, deliverer)
	me := &model.User{ID: "me", URI: strPtr(srcURI)}
	userRepo.Users[me.ID] = me

	require.NoError(t, svc.Move(me, dstURI))

	// In-place struct was updated so the caller sees the new state.
	assert.NotNil(t, me.MovedToURI)
	assert.Equal(t, dstURI, *me.MovedToURI)
	assert.NotNil(t, me.MovedAt)
	require.NotNil(t, me.AlsoKnownAs)
	assert.Equal(t, dstURI, *me.AlsoKnownAs)

	// UpdateUser was persisted to the repo row (not just the in-place struct).
	// これを検証しないと UpdateUser 呼び出しが消えたリグレッションを検出できない。
	persisted := userRepo.Users["me"]
	require.NotNil(t, persisted.MovedToURI)
	assert.Equal(t, dstURI, *persisted.MovedToURI)
	assert.NotNil(t, persisted.MovedAt)
	require.NotNil(t, persisted.AlsoKnownAs)
	assert.Equal(t, dstURI, *persisted.AlsoKnownAs)

	// Delivery was enqueued with a Move-shaped body.
	require.Equal(t, 1, deliverer.called)
	assert.Equal(t, "me", deliverer.signer)
	var body map[string]any
	require.NoError(t, json.Unmarshal(deliverer.body, &body))
	assert.Equal(t, "Move", body["type"])
	assert.Equal(t, srcURI, body["actor"])
	assert.Equal(t, srcURI, body["object"])
	assert.Equal(t, dstURI, body["target"])
}

func TestMove_AlreadyMoved(t *testing.T) {
	svc, _, _ := newService(&fakeResolver{}, &fakeDeliverer{})
	existing := "https://other.example/users/x"
	me := &model.User{ID: "me", URI: strPtr("https://local.example/users/me"), MovedToURI: &existing}
	assert.ErrorIs(t, svc.Move(me, "https://other.example/users/new"), move.ErrAlreadyMoved)
}

func TestMove_RemoteSourceForbidden(t *testing.T) {
	svc, _, _ := newService(&fakeResolver{}, &fakeDeliverer{})
	host := "remote.example"
	me := &model.User{ID: "me", Host: &host, URI: strPtr("https://remote.example/users/me")}
	assert.ErrorIs(t, svc.Move(me, "https://other.example/users/new"), move.ErrRemoteSourceForbidden)
}

func TestMove_EmptyDestinationURI(t *testing.T) {
	svc, _, _ := newService(&fakeResolver{}, &fakeDeliverer{})
	me := &model.User{ID: "me"}
	assert.ErrorIs(t, svc.Move(me, ""), move.ErrNoSuchUser)
	assert.ErrorIs(t, svc.Move(me, "   "), move.ErrNoSuchUser)
}

func TestMove_ResolverError(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("boom")}
	svc, _, _ := newService(resolver, &fakeDeliverer{})
	me := &model.User{ID: "me", URI: strPtr("https://local.example/users/me")}
	assert.ErrorIs(t, svc.Move(me, "https://other.example/users/new"), move.ErrNoSuchUser)
}

func TestMove_ResolverReturnsNil(t *testing.T) {
	resolver := &fakeResolver{byURI: map[string]*model.User{}}
	svc, _, _ := newService(resolver, &fakeDeliverer{})
	me := &model.User{ID: "me", URI: strPtr("https://local.example/users/me")}
	assert.ErrorIs(t, svc.Move(me, "https://other.example/users/new"), move.ErrNoSuchUser)
}

func TestMove_ResolverMissing(t *testing.T) {
	svc, _, _ := newService(nil, nil)
	me := &model.User{ID: "me", URI: strPtr("https://local.example/users/me")}
	assert.ErrorIs(t, svc.Move(me, "https://other.example/users/new"), move.ErrNoSuchUser)
}

func TestMove_AlsoKnownAsMissing(t *testing.T) {
	dstURI := "https://other.example/users/new"
	dst := &model.User{ID: "dst", URI: strPtr(dstURI), AlsoKnownAs: nil}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	svc, _, _ := newService(resolver, &fakeDeliverer{})
	me := &model.User{ID: "me", URI: strPtr("https://local.example/users/me")}
	assert.ErrorIs(t, svc.Move(me, dstURI), move.ErrDestinationForbids)
}

func TestMove_DestinationAlreadyMoved(t *testing.T) {
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"
	other := "https://yet.another/users/z"
	dst := &model.User{
		ID:          "dst",
		URI:         strPtr(dstURI),
		AlsoKnownAs: strPtr(srcURI),
		MovedToURI:  &other,
	}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	svc, _, _ := newService(resolver, &fakeDeliverer{})
	me := &model.User{ID: "me", URI: strPtr(srcURI)}
	assert.ErrorIs(t, svc.Move(me, dstURI), move.ErrDestinationForbids)
}

func TestMove_AlsoKnownAsIncludesMultipleValues(t *testing.T) {
	// 自分の URI が csv の途中にあっても検出されることを確認する。
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"
	csv := "https://foo/users/a, " + srcURI + " , https://bar/users/b"
	dst := &model.User{ID: "dst", URI: strPtr(dstURI), AlsoKnownAs: &csv}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	deliverer := &fakeDeliverer{}
	svc, userRepo, _ := newService(resolver, deliverer)
	me := &model.User{ID: "me", URI: strPtr(srcURI)}
	userRepo.Users[me.ID] = me

	require.NoError(t, svc.Move(me, dstURI))
	assert.Equal(t, 1, deliverer.called)
}

func TestMove_DelivererMissing_StillUpdatesDB(t *testing.T) {
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"
	dst := &model.User{ID: "dst", URI: strPtr(dstURI), AlsoKnownAs: strPtr(srcURI)}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	svc, userRepo, _ := newService(resolver, nil)
	me := &model.User{ID: "me", URI: strPtr(srcURI)}
	userRepo.Users[me.ID] = me

	require.NoError(t, svc.Move(me, dstURI))
	require.NotNil(t, me.MovedToURI)
	assert.Equal(t, dstURI, *me.MovedToURI)
}

func TestMove_AppendAlsoKnownAsDedup(t *testing.T) {
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"
	dst := &model.User{ID: "dst", URI: strPtr(dstURI), AlsoKnownAs: strPtr(srcURI)}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	svc, userRepo, _ := newService(resolver, &fakeDeliverer{})
	// me は既に dst を alsoKnownAs に入れている (2 回 Move を叩いたケース相当)
	existing := dstURI
	me := &model.User{ID: "me", URI: strPtr(srcURI), AlsoKnownAs: &existing}
	userRepo.Users[me.ID] = me

	require.NoError(t, svc.Move(me, dstURI))
	require.NotNil(t, me.AlsoKnownAs)
	// 重複は追加されない。
	assert.Equal(t, dstURI, *me.AlsoKnownAs)
}

func TestMove_DelivererError(t *testing.T) {
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"
	dst := &model.User{ID: "dst", URI: strPtr(dstURI), AlsoKnownAs: strPtr(srcURI)}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	boom := errors.New("enqueue boom")
	deliverer := &fakeDeliverer{returnErr: boom}
	svc, userRepo, _ := newService(resolver, deliverer)
	me := &model.User{ID: "me", URI: strPtr(srcURI)}
	userRepo.Users[me.ID] = me

	err := svc.Move(me, dstURI)
	assert.ErrorIs(t, err, boom)
}

func TestMove_NilSource(t *testing.T) {
	svc, _, _ := newService(&fakeResolver{}, &fakeDeliverer{})
	assert.ErrorIs(t, svc.Move(nil, "https://other.example/users/new"), move.ErrNoSuchUser)
}

// UpdateUser がエラーを返した場合、エラーが伝搬して delivery は発生しない。
func TestMove_UserRepoUpdateError(t *testing.T) {
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"
	dst := &model.User{ID: "dst", URI: strPtr(dstURI), AlsoKnownAs: strPtr(srcURI)}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	deliverer := &fakeDeliverer{}

	userRepo := testutil.NewMockUserRepository()
	followingRepo := testutil.NewMockFollowingRepository()
	urls := activitypub.NewURLBuilder("https://local.example")
	renderer := activitypub.NewRenderer(urls)
	// UpdateUser がエラーを返すラッパー。
	failRepo := &failingUserRepo{MockUserRepository: userRepo}
	svc := move.NewService(failRepo, followingRepo, urls, renderer, resolver, deliverer)
	me := &model.User{ID: "me", URI: strPtr(srcURI)}

	err := svc.Move(me, dstURI)
	require.Error(t, err)
	assert.Equal(t, 0, deliverer.called, "update 失敗時は deliver されない")
}

type failingUserRepo struct {
	*testutil.MockUserRepository
}

func (f *failingUserRepo) UpdateUser(string, map[string]any) error {
	return errors.New("update boom")
}

// 解決後の dst.URI が空なら dstURI 引数をそのまま canonical として使うこと。
func TestMove_FallbackToInputURIWhenResolverURIEmpty(t *testing.T) {
	srcURI := "https://local.example/users/me"
	dstURI := "https://other.example/users/new"
	dst := &model.User{ID: "dst", URI: nil, AlsoKnownAs: strPtr(srcURI)}
	resolver := &fakeResolver{byURI: map[string]*model.User{dstURI: dst}}
	svc, userRepo, _ := newService(resolver, &fakeDeliverer{})
	me := &model.User{ID: "me", URI: strPtr(srcURI)}
	userRepo.Users[me.ID] = me

	require.NoError(t, svc.Move(me, dstURI))
	require.NotNil(t, me.MovedToURI)
	assert.Equal(t, dstURI, *me.MovedToURI)
}
