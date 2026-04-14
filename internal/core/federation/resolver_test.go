package federation_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFetcher returns canned bytes/error for FetchObject.
type stubFetcher struct {
	body []byte
	err  error
}

func (s *stubFetcher) FetchObject(_ string) ([]byte, error) {
	return s.body, s.err
}

const sampleActor = `{
	"id": "https://remote.example/users/alice",
	"type": "Person",
	"preferredUsername": "alice",
	"name": "Alice",
	"inbox": "https://remote.example/users/alice/inbox",
	"endpoints": {"sharedInbox": "https://remote.example/inbox"},
	"publicKey": {
		"id": "https://remote.example/users/alice#main-key",
		"owner": "https://remote.example/users/alice",
		"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nFAKE\n-----END PUBLIC KEY-----"
	}
}`

func newResolver(t *testing.T, body string, err error) (*federation.Resolver, *testutil.MockUserRepository) {
	t.Helper()
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	return federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(body), err: err}, idGen), repo
}

func TestResolveActor_NewUser(t *testing.T) {
	r, repo := newResolver(t, sampleActor, nil)
	user, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.Len(t, repo.Users, 1)

	pem, err := r.PublicKeyForActor(user.ID)
	require.NoError(t, err)
	assert.Contains(t, pem, "FAKE")
}

func TestResolveActor_ExistingUser(t *testing.T) {
	r, repo := newResolver(t, sampleActor, nil)
	uri := "https://remote.example/users/alice"
	repo.Users["existing"] = &model.User{
		ID:       "existing",
		Username: "alice",
		URI:      &uri,
	}
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	assert.Equal(t, "existing", user.ID)
	// 公開鍵キャッシュは refresh により埋まる
	pem, err := r.PublicKeyForActor("existing")
	require.NoError(t, err)
	assert.Contains(t, pem, "FAKE")
}

func TestResolveActor_FetchError(t *testing.T) {
	r, _ := newResolver(t, "", errors.New("network down"))
	_, err := r.ResolveActor("https://remote.example/users/x")
	assert.Error(t, err)
}

func TestResolveActor_BadJSON(t *testing.T) {
	r, _ := newResolver(t, "{not json", nil)
	_, err := r.ResolveActor("https://remote.example/users/x")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

func TestResolveActor_MissingFields(t *testing.T) {
	r, _ := newResolver(t, `{"id":"x"}`, nil)
	_, err := r.ResolveActor("https://remote.example/users/x")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

func TestResolveActor_BadHost(t *testing.T) {
	// invalid URL with control char
	body := `{
		"id": "://invalid",
		"preferredUsername": "alice",
		"inbox": "x"
	}`
	r, _ := newResolver(t, body, nil)
	_, err := r.ResolveActor("https://x.example/")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

func TestResolveActor_EmptyHost(t *testing.T) {
	// mailto: parses but has no host
	body := `{
		"id": "mailto:alice@example.com",
		"preferredUsername": "alice",
		"inbox": "x"
	}`
	r, _ := newResolver(t, body, nil)
	_, err := r.ResolveActor("https://x.example/")
	require.ErrorIs(t, err, federation.ErrInvalidActor)
}

// failingUserRepo returns Create errors for the resolver.
type failingUserRepo struct {
	*testutil.MockUserRepository
}

func (f *failingUserRepo) Create(_ *model.User) error {
	return errors.New("create failed")
}

func TestResolveActor_RepoCreateError(t *testing.T) {
	mock := testutil.NewMockUserRepository()
	repo := &failingUserRepo{MockUserRepository: mock}
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	_, err := r.ResolveActor("https://remote.example/users/alice")
	assert.Error(t, err)
}

func TestResolveActorByKeyID(t *testing.T) {
	r, _ := newResolver(t, sampleActor, nil)
	user, err := r.ResolveActorByKeyID("https://remote.example/users/alice#main-key")
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
}

func TestPublicKeyForActor_Missing(t *testing.T) {
	r, _ := newResolver(t, sampleActor, nil)
	_, err := r.PublicKeyForActor("ghost")
	assert.Error(t, err)
}

// --- ResolveNote / IngestNote --------------------------------------------------

const sampleRemoteNote = `{
	"id": "https://remote.example/notes/n1",
	"type": "Note",
	"attributedTo": "https://remote.example/users/alice",
	"content": "hello",
	"to": ["https://www.w3.org/ns/activitystreams#Public"],
	"cc": ["https://remote.example/users/alice/followers"],
	"summary": "cw",
	"sensitive": true
}`

// scriptedFetcher returns different bodies based on a counter so a single
// resolver call can resolve both an actor and a note.
type scriptedFetcher struct {
	bodies [][]byte
	idx    int
}

func (s *scriptedFetcher) FetchObject(_ string) ([]byte, error) {
	if s.idx >= len(s.bodies) {
		return nil, errors.New("no more bodies")
	}
	b := s.bodies[s.idx]
	s.idx++
	return b, nil
}

func TestResolveNote_LocalAlreadyStored(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	noteRepo.Notes["nlocal"] = &model.Note{ID: "nlocal", UserID: "u1"}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)

	got, err := r.ResolveNote("https://example.com/notes/nlocal")
	require.NoError(t, err)
	assert.Equal(t, "nlocal", got.ID)
}

func TestResolveNote_LocalUnknown(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)

	_, err := r.ResolveNote("https://example.com/notes/missing")
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestResolveNote_RemoteCached(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	noteRepo.Notes["n1"] = &model.Note{ID: "n1", URI: &uri}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)

	got, err := r.ResolveNote(uri)
	require.NoError(t, err)
	assert.Equal(t, "n1", got.ID)
}

func TestResolveNote_RemoteFetchAndIngest(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	// 最初の呼び出しで Note JSON、続いて attributedTo を解決するための actor JSON を返す
	fetcher := &scriptedFetcher{bodies: [][]byte{[]byte(sampleRemoteNote), []byte(sampleActor)}}
	r := federation.NewResolver(repo, noteRepo, urls, fetcher, idGen)

	got, err := r.ResolveNote("https://remote.example/notes/n1")
	require.NoError(t, err)
	assert.NotNil(t, got)
	require.NotNil(t, got.URI)
	assert.Equal(t, "https://remote.example/notes/n1", *got.URI)
	require.NotNil(t, got.Text)
	assert.Equal(t, "hello", *got.Text)
	require.NotNil(t, got.CW)
	assert.Equal(t, "cw", *got.CW)
	assert.Equal(t, model.NoteVisibilityPublic, got.Visibility)
}

func TestResolveNote_FetchError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{err: errors.New("net down")}, idGen)
	_, err := r.ResolveNote("https://remote.example/notes/x")
	assert.Error(t, err)
}

func TestResolveNote_NoNoteRepo(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, nil, urls, &stubFetcher{}, idGen)
	_, err := r.ResolveNote("https://remote.example/notes/x")
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestIngestNote_BadJSON(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)
	_, err := r.IngestNote([]byte(`{not json`))
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestIngestNote_MissingFields(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)
	_, err := r.IngestNote([]byte(`{"id":"x"}`))
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestIngestNote_NoNoteRepo(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, nil, urls, &stubFetcher{}, idGen)
	_, err := r.IngestNote([]byte(sampleRemoteNote))
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestIngestNote_DedupOnExisting(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	noteRepo.Notes["existing"] = &model.Note{ID: "existing", URI: &uri}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	got, err := r.IngestNote([]byte(sampleRemoteNote))
	require.NoError(t, err)
	assert.Equal(t, "existing", got.ID)
}

func TestIngestNote_ResolveActorError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{err: errors.New("actor down")}, idGen)
	_, err := r.IngestNote([]byte(sampleRemoteNote))
	assert.Error(t, err)
}

// failingNoteCreateRepo causes Create on noteRepo to fail.
type failingNoteCreateRepo struct {
	*testutil.MockNoteRepository
}

func (f *failingNoteCreateRepo) Create(_ *model.Note) error {
	return errors.New("create failed")
}

func TestIngestNote_NoteCreateError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := &failingNoteCreateRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	_, err := r.IngestNote([]byte(sampleRemoteNote))
	assert.Error(t, err)
}

func TestIngestNote_ReplyToLocal(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	noteRepo.Notes["parent"] = &model.Note{ID: "parent", UserID: "uparent"}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	body := []byte(`{
		"id": "https://remote.example/notes/n2",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "reply",
		"inReplyTo": "https://example.com/notes/parent",
		"to": ["https://www.w3.org/ns/activitystreams#Public"]
	}`)
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	require.NotNil(t, got.ReplyID)
	assert.Equal(t, "parent", *got.ReplyID)
}

func TestIngestNote_ReplyToRemote(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	parentURI := "https://remote.example/notes/parent"
	noteRepo.Notes["parent"] = &model.Note{ID: "parent", UserID: "uparent", URI: &parentURI}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	body := []byte(`{
		"id": "https://remote.example/notes/n3",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "reply",
		"inReplyTo": "https://remote.example/notes/parent",
		"to": ["https://www.w3.org/ns/activitystreams#Public"]
	}`)
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	require.NotNil(t, got.ReplyID)
	assert.Equal(t, "parent", *got.ReplyID)
}

func TestIngestNote_SensitiveWithoutSummary(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	body := []byte(`{
		"id": "https://remote.example/notes/n4",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "nsfw",
		"sensitive": true,
		"to": ["https://www.w3.org/ns/activitystreams#Public"]
	}`)
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	require.NotNil(t, got.CW)
	assert.Equal(t, "", *got.CW)
}

// --- IngestNote visibility (ported from nekonoverse fetch_remote_note) --------

// makeNoteJSON builds a minimal AP Note with the given to/cc audience.
func makeNoteJSON(noteID string, to, cc []string) []byte {
	toJSON, _ := json.Marshal(to)
	ccJSON, _ := json.Marshal(cc)
	return []byte(fmt.Sprintf(`{
		"id": "https://remote.example/notes/%s",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "visibility test",
		"to": %s,
		"cc": %s
	}`, noteID, toJSON, ccJSON))
}

func TestIngestNote_PublicVisibility(t *testing.T) {
	// to=[Public], cc=[followers] → public
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)

	body := makeNoteJSON("vis-public", []string{"https://www.w3.org/ns/activitystreams#Public"}, []string{"https://remote.example/users/alice/followers"})
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilityPublic, got.Visibility)
}

func TestIngestNote_HomeVisibility(t *testing.T) {
	// to=[followers], cc=[Public] → home
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)

	body := makeNoteJSON("vis-home", []string{"https://remote.example/users/alice/followers"}, []string{"https://www.w3.org/ns/activitystreams#Public"})
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilityHome, got.Visibility)
}

func TestIngestNote_FollowersVisibility(t *testing.T) {
	// to=[followers], cc=[] → followers
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)

	body := makeNoteJSON("vis-followers", []string{"https://remote.example/users/alice/followers"}, []string{})
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilityFollowers, got.Visibility)
}

func TestIngestNote_DirectVisibility(t *testing.T) {
	// to=[specific user], cc=[] → specified
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)

	body := makeNoteJSON("vis-direct", []string{"https://remote.example/users/bob"}, []string{})
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilitySpecified, got.Visibility)
}

func TestIngestNote_EmptyAudienceVisibility(t *testing.T) {
	// to=[], cc=[] → specified (fallback)
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)

	body := makeNoteJSON("vis-empty", []string{}, []string{})
	got, err := r.IngestNote(body)
	require.NoError(t, err)
	assert.Equal(t, model.NoteVisibilitySpecified, got.Visibility)
}

// --- UpdateRemoteNote (Step J) -----------------------------------------------

const remoteNoteUpdateBody = `{
	"id": "https://remote.example/notes/n1",
	"type": "Note",
	"attributedTo": "https://remote.example/users/alice",
	"content": "edited content",
	"summary": "edited cw"
}`

func TestUpdateRemoteNote_HappyPath(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	host := "remote.example"
	original := "original"
	noteRepo.Notes["n1"] = &model.Note{
		ID: "n1", URI: &uri, UserID: "alice-id", UserHost: &host, Text: &original,
	}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)

	got, err := r.UpdateRemoteNote([]byte(remoteNoteUpdateBody))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Text)
	assert.Equal(t, "edited content", *got.Text)
	require.NotNil(t, got.CW)
	assert.Equal(t, "edited cw", *got.CW)
}

func TestUpdateRemoteNote_NoNoteRepo(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, nil, urls, &stubFetcher{}, idGen)
	_, err := r.UpdateRemoteNote([]byte(remoteNoteUpdateBody))
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestUpdateRemoteNote_BadJSON(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)
	_, err := r.UpdateRemoteNote([]byte(`{not json`))
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestUpdateRemoteNote_MissingID(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)
	_, err := r.UpdateRemoteNote([]byte(`{"type":"Note"}`))
	assert.ErrorIs(t, err, federation.ErrInvalidNote)
}

func TestUpdateRemoteNote_NotFound(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)
	got, err := r.UpdateRemoteNote([]byte(remoteNoteUpdateBody))
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUpdateRemoteNote_LocalNoteSkipped(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	original := "original"
	// UserHost == nil → ローカルノート扱い
	noteRepo.Notes["n1"] = &model.Note{
		ID: "n1", URI: &uri, UserID: "alice-id", Text: &original,
	}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)

	got, err := r.UpdateRemoteNote([]byte(remoteNoteUpdateBody))
	require.NoError(t, err)
	require.NotNil(t, got)
	// Text は変わらない
	require.NotNil(t, got.Text)
	assert.Equal(t, "original", *got.Text)
}

func TestUpdateRemoteNote_EmptyContentNoOp(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	host := "remote.example"
	original := "original"
	noteRepo.Notes["n1"] = &model.Note{
		ID: "n1", URI: &uri, UserID: "alice-id", UserHost: &host, Text: &original,
	}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)

	body := []byte(`{
		"id": "https://remote.example/notes/n1",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice"
	}`)
	got, err := r.UpdateRemoteNote(body)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Text)
	assert.Equal(t, "original", *got.Text)
}

func TestUpdateRemoteNote_SensitiveWithoutSummary(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	host := "remote.example"
	noteRepo.Notes["n1"] = &model.Note{
		ID: "n1", URI: &uri, UserID: "alice-id", UserHost: &host,
	}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)

	body := []byte(`{
		"id": "https://remote.example/notes/n1",
		"type": "Note",
		"attributedTo": "https://remote.example/users/alice",
		"content": "nsfw",
		"sensitive": true
	}`)
	got, err := r.UpdateRemoteNote(body)
	require.NoError(t, err)
	require.NotNil(t, got.CW)
	assert.Equal(t, "", *got.CW)
}

// failingNoteUpdateRepo causes UpdateFields to fail.
type failingNoteUpdateRepo struct {
	*testutil.MockNoteRepository
}

func (f *failingNoteUpdateRepo) UpdateFields(_ string, _ map[string]any) error {
	return errors.New("update failed")
}

func TestUpdateRemoteNote_UpdateFieldsError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	mock := testutil.NewMockNoteRepository()
	uri := "https://remote.example/notes/n1"
	host := "remote.example"
	mock.Notes["n1"] = &model.Note{
		ID: "n1", URI: &uri, UserID: "alice-id", UserHost: &host,
	}
	noteRepo := &failingNoteUpdateRepo{MockNoteRepository: mock}
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{}, idGen)
	_, err := r.UpdateRemoteNote([]byte(remoteNoteUpdateBody))
	assert.Error(t, err)
}

// --- TTL cache (Step G) -------------------------------------------------------

func TestResolveActor_TTLRefresh(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	uri := "https://remote.example/users/alice"
	// LastFetchedAt が古いため refresh が走るはず
	old := time.Now().Add(-48 * time.Hour)
	repo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "alice",
		URI:           &uri,
		LastFetchedAt: &old,
	}
	// 更新後の actor を返す
	updated := `{
		"id": "https://remote.example/users/alice",
		"type": "Person",
		"preferredUsername": "alice",
		"name": "Alice Refreshed",
		"inbox": "https://remote.example/users/alice/inbox-v2",
		"endpoints": {"sharedInbox": "https://remote.example/inbox-v2"},
		"publicKey": {"publicKeyPem": "REFRESHED"}
	}`
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(updated)}, idGen)
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	require.NotNil(t, user.Name)
	assert.Equal(t, "Alice Refreshed", *user.Name)
	require.NotNil(t, user.Inbox)
	assert.Equal(t, "https://remote.example/users/alice/inbox-v2", *user.Inbox)
	require.NotNil(t, user.SharedInbox)
	assert.Equal(t, "https://remote.example/inbox-v2", *user.SharedInbox)

	pem, err := r.PublicKeyForActor("existing")
	require.NoError(t, err)
	assert.Contains(t, pem, "REFRESHED")
}

func TestResolveActor_NoRefreshWhenFresh(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	uri := "https://remote.example/users/alice"
	// LastFetchedAt が十分新しいので refresh は走らない (refreshActor は呼ばれない)
	now := time.Now()
	originalName := "Original"
	repo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "alice",
		URI:           &uri,
		Name:          &originalName,
		LastFetchedAt: &now,
	}
	// fetcher が何を返しても name は更新されないはず
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	// 公開鍵キャッシュは空 → refreshPublicKey 経路を通る
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	require.NotNil(t, user.Name)
	assert.Equal(t, "Original", *user.Name)

	// publicKey は in-memory にキャッシュされたはず
	pem, err := r.PublicKeyForActor("existing")
	require.NoError(t, err)
	assert.Contains(t, pem, "FAKE")
}

func TestResolveActor_TTLRefreshFetchError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	uri := "https://remote.example/users/alice"
	old := time.Now().Add(-48 * time.Hour)
	originalName := "Original"
	repo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "alice",
		URI:           &uri,
		Name:          &originalName,
		LastFetchedAt: &old,
	}
	// fetcher エラーでも refresh はベストエフォートなので既存 user を返す
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{err: errors.New("net down")}, idGen)
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	assert.Equal(t, "Original", *user.Name)
}

func TestPublicKeyForActor_DBFallback(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	pkRepo := testutil.NewMockUserPublickeyRepository()
	r.SetPublickeyRepo(pkRepo)

	// ResolveActorで公開鍵をin-memory + DBにキャッシュ
	user, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)

	// DBに永続化されていることを確認
	require.Len(t, pkRepo.Keys, 1)
	pk := pkRepo.Keys[user.ID]
	require.NotNil(t, pk)
	assert.Equal(t, "https://remote.example/users/alice#main-key", pk.KeyID)
	assert.Contains(t, pk.KeyPEM, "FAKE")

	// in-memoryキャッシュを期限切れにする
	r.SetClock(func() time.Time { return time.Now().Add(48 * time.Hour) })

	// DBフォールバックで復元できることを確認
	pem, err := r.PublicKeyForActor(user.ID)
	require.NoError(t, err)
	assert.Contains(t, pem, "FAKE")
}

func TestPublicKeyForActor_DBFallback_NilRepo(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	// publickeyRepo未設定

	user, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)

	// in-memoryキャッシュを期限切れにする
	r.SetClock(func() time.Time { return time.Now().Add(48 * time.Hour) })

	// DB無しなのでmiss
	_, err = r.PublicKeyForActor(user.ID)
	assert.Error(t, err)
}

func TestPublicKeyForActor_TTLExpiry(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)

	// 解決して publicKey をキャッシュする (現在時刻で fetched 扱い)
	user, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)
	pem, err := r.PublicKeyForActor(user.ID)
	require.NoError(t, err)
	assert.Contains(t, pem, "FAKE")

	// 時計を進める → expired として miss するはず
	r.SetClock(func() time.Time { return time.Now().Add(48 * time.Hour) })
	_, err = r.PublicKeyForActor(user.ID)
	assert.Error(t, err)
}

func TestResolver_SetClockNil(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	// nil を渡しても panic / 変更なし
	r.SetClock(nil)
	// 直後に呼んでもデフォルト clock のまま動く
	_, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)
}

// stubInstanceTracker collects host registrations for assertions.
type stubInstanceTracker struct {
	hosts []string
}

func (s *stubInstanceTracker) RegisterFromHost(host string) (*model.Instance, error) {
	s.hosts = append(s.hosts, host)
	return nil, nil
}

func TestResolveActor_NotifiesInstanceTrackerOnCreate(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	tracker := &stubInstanceTracker{}
	r.SetInstanceTracker(tracker)

	_, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)
	require.Len(t, tracker.hosts, 1)
	assert.Equal(t, "remote.example", tracker.hosts[0])
}

func TestResolveActor_NotifiesInstanceTrackerOnRefresh(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	uri := "https://remote.example/users/alice"
	host := "remote.example"
	old := time.Now().Add(-48 * time.Hour)
	repo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "alice",
		URI:           &uri,
		Host:          &host,
		LastFetchedAt: &old,
	}
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	tracker := &stubInstanceTracker{}
	r.SetInstanceTracker(tracker)

	_, err := r.ResolveActor(uri)
	require.NoError(t, err)
	require.Len(t, tracker.hosts, 1)
	assert.Equal(t, "remote.example", tracker.hosts[0])
}

func TestResolveActor_NoTrackerNoOp(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	r.SetInstanceTracker(nil)
	_, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)
}

// stubChartHook captures chart hook fires from the federation resolver.
type stubChartHook struct {
	users []string // user IDs
}

func (s *stubChartHook) OnRemoteUserCreated(u *model.User) {
	s.users = append(s.users, u.ID)
}

func TestResolveActor_ChartHookFiresOnNewUser(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	hook := &stubChartHook{}
	r.SetChartHook(hook)

	user, err := r.ResolveActor("https://remote.example/users/alice")
	require.NoError(t, err)
	require.Len(t, hook.users, 1)
	assert.Equal(t, user.ID, hook.users[0])
}

func TestResolveActor_FreshButCacheMissFetchError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	uri := "https://remote.example/users/alice"
	now := time.Now()
	repo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "alice",
		URI:           &uri,
		LastFetchedAt: &now,
	}
	// publicKey は cache 空 → refreshPublicKey 経路に入るが fetcher はエラー
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{err: errors.New("net down")}, idGen)
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	assert.Equal(t, "existing", user.ID)
	// fetch 失敗時もエラーは伝搬しない
	_, perr := r.PublicKeyForActor("existing")
	assert.Error(t, perr)
}

func TestResolver_SetActorTTL(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(sampleActor)}, idGen)
	// 0 / 負値は無視されデフォルト維持
	r.SetActorTTL(0)
	r.SetActorTTL(-1)
	r.SetActorTTL(time.Minute)

	// 1 分 TTL に設定し、LastFetchedAt が 5 分前のユーザーを refresh するか確認
	uri := "https://remote.example/users/alice"
	old := time.Now().Add(-5 * time.Minute)
	originalName := "Original"
	repo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "alice",
		URI:           &uri,
		Name:          &originalName,
		LastFetchedAt: &old,
	}
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	require.NotNil(t, user.Name)
	assert.Equal(t, "Alice", *user.Name) // sampleActor の name で上書き
}

func TestRefreshPublicKey_OnExistingUser_FetchError(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	uri := "https://remote.example/users/alice"
	repo.Users["existing"] = &model.User{ID: "existing", Username: "alice", URI: &uri}
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{err: errors.New("oops")}, idGen)
	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	assert.Equal(t, "existing", user.ID)
	// fetch失敗してもエラーは返さない
	_, err = r.PublicKeyForActor("existing")
	assert.Error(t, err)
}

// --- multi actor type support (#153) ---

// actorJSON renders a minimal actor JSON document with the given type.
func actorJSON(actorType string) string {
	return fmt.Sprintf(`{
		"id": "https://remote.example/users/x",
		"type": %q,
		"preferredUsername": "x",
		"name": "X",
		"inbox": "https://remote.example/users/x/inbox",
		"endpoints": {"sharedInbox": "https://remote.example/inbox"},
		"publicKey": {
			"id": "https://remote.example/users/x#main-key",
			"owner": "https://remote.example/users/x",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nFAKE\n-----END PUBLIC KEY-----"
		}
	}`, actorType)
}

func TestResolveActor_AllValidActorTypesAccepted(t *testing.T) {
	for _, typ := range activitypub.ValidActorTypes {
		t.Run(typ, func(t *testing.T) {
			r, repo := newResolver(t, actorJSON(typ), nil)
			user, err := r.ResolveActor("https://remote.example/users/x")
			require.NoError(t, err)
			assert.Equal(t, "x", user.Username)
			assert.Len(t, repo.Users, 1)
		})
	}
}

func TestResolveActor_InvalidActorTypeRejected(t *testing.T) {
	for _, typ := range []string{"Note", "Tombstone", "Article", ""} {
		t.Run(typ, func(t *testing.T) {
			r, _ := newResolver(t, actorJSON(typ), nil)
			_, err := r.ResolveActor("https://remote.example/users/x")
			require.ErrorIs(t, err, federation.ErrInvalidActor)
		})
	}
}

func TestResolveActor_ServiceTypeSetsIsBot(t *testing.T) {
	r, repo := newResolver(t, actorJSON("Service"), nil)
	user, err := r.ResolveActor("https://remote.example/users/x")
	require.NoError(t, err)
	assert.True(t, user.IsBot)
	// 永続化された user も isBot=true
	require.Len(t, repo.Users, 1)
	for _, u := range repo.Users {
		assert.True(t, u.IsBot)
	}
}

func TestResolveActor_ApplicationTypeSetsIsBot(t *testing.T) {
	r, _ := newResolver(t, actorJSON("Application"), nil)
	user, err := r.ResolveActor("https://remote.example/users/x")
	require.NoError(t, err)
	assert.True(t, user.IsBot)
}

func TestResolveActor_GroupTypeDoesNotSetIsBot(t *testing.T) {
	r, _ := newResolver(t, actorJSON("Group"), nil)
	user, err := r.ResolveActor("https://remote.example/users/x")
	require.NoError(t, err)
	assert.False(t, user.IsBot)
}

func TestResolveActor_OrganizationTypeDoesNotSetIsBot(t *testing.T) {
	r, _ := newResolver(t, actorJSON("Organization"), nil)
	user, err := r.ResolveActor("https://remote.example/users/x")
	require.NoError(t, err)
	assert.False(t, user.IsBot)
}

func TestRefreshActor_UpdatesIsBotOnTypeChange(t *testing.T) {
	// 既存 Person ユーザーが Service に切り替わった場合、refresh で IsBot=true
	// に追従する。
	repo := testutil.NewMockUserRepository()
	uri := "https://remote.example/users/x"
	stale := time.Now().Add(-48 * time.Hour) // TTL超過
	repo.Users["existing"] = &model.User{
		ID:            "existing",
		Username:      "x",
		URI:           &uri,
		IsBot:         false,
		LastFetchedAt: &stale,
	}
	noteRepo := testutil.NewMockNoteRepository()
	urls := activitypub.NewURLBuilder("https://example.com")
	idGen, _ := id.NewGenerator("aidx")
	r := federation.NewResolver(repo, noteRepo, urls, &stubFetcher{body: []byte(actorJSON("Service"))}, idGen)

	user, err := r.ResolveActor(uri)
	require.NoError(t, err)
	assert.True(t, user.IsBot)
}
