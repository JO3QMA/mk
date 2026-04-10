package timeline

import (
	"context"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServiceWithRepo wires a Service against the shared Redis container
// and an in-memory MockNoteRepository so that timeline reads return real notes.
func newTestServiceWithRepo(t *testing.T) (*Service, *FanoutTimelineService, *testutil.MockNoteRepository) {
	t.Helper()
	testRedis.FlushAll(context.Background())
	noteRepo := testutil.NewMockNoteRepository()
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	svc := NewService(fanout, noteRepo, testutil.NewMockFollowingRepository())
	return svc, fanout, noteRepo
}

func TestService_HomeTimelineRequiresUser(t *testing.T) {
	svc, _, _ := newTestServiceWithRepo(t)
	_, err := svc.HomeTimeline(context.Background(), nil, "", "", 10)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestService_HomeTimeline(t *testing.T) {
	svc, fanout, repo := newTestServiceWithRepo(t)
	ctx := context.Background()

	user := &model.User{ID: "viewer"}
	noteID := idGen.Generate(time.Now())
	repo.Notes[noteID] = &model.Note{ID: noteID, UserID: "viewer", Visibility: model.NoteVisibilityPublic}
	require.NoError(t, fanout.Push(ctx, HomeTimelineName(user.ID), noteID, 100))

	out, err := svc.HomeTimeline(ctx, user, "", "", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, noteID, out[0].ID)
}

func TestService_HomeTimelineFanoutError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	fanout := NewFanoutTimelineService(closedClient(t), idGen)
	svc := NewService(fanout, noteRepo, nil)
	_, err := svc.HomeTimeline(context.Background(), &model.User{ID: "u"}, "", "", 10)
	assert.Error(t, err)
}

func TestService_LocalTimeline(t *testing.T) {
	svc, fanout, repo := newTestServiceWithRepo(t)
	ctx := context.Background()

	noteID := idGen.Generate(time.Now())
	repo.Notes[noteID] = &model.Note{ID: noteID, UserID: "a", Visibility: model.NoteVisibilityPublic}
	require.NoError(t, fanout.Push(ctx, LocalTimeline, noteID, 100))

	out, err := svc.LocalTimeline(ctx, nil, "", "", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestService_LocalTimelineFanoutError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	svc := NewService(NewFanoutTimelineService(closedClient(t), idGen), noteRepo, nil)
	_, err := svc.LocalTimeline(context.Background(), nil, "", "", 10)
	assert.Error(t, err)
}

func TestService_GlobalTimeline(t *testing.T) {
	svc, fanout, repo := newTestServiceWithRepo(t)
	ctx := context.Background()

	noteID := idGen.Generate(time.Now())
	repo.Notes[noteID] = &model.Note{ID: noteID, UserID: "a", Visibility: model.NoteVisibilityPublic}
	require.NoError(t, fanout.Push(ctx, GlobalTimeline, noteID, 100))

	out, err := svc.GlobalTimeline(ctx, nil, "", "", 10)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestService_GlobalTimelineFanoutError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	svc := NewService(NewFanoutTimelineService(closedClient(t), idGen), noteRepo, nil)
	_, err := svc.GlobalTimeline(context.Background(), nil, "", "", 10)
	assert.Error(t, err)
}

func TestService_HybridTimelineRequiresUser(t *testing.T) {
	svc, _, _ := newTestServiceWithRepo(t)
	_, err := svc.HybridTimeline(context.Background(), nil, "", "", 10)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestService_HybridTimeline(t *testing.T) {
	svc, fanout, repo := newTestServiceWithRepo(t)
	ctx := context.Background()

	user := &model.User{ID: "viewer"}
	homeID := idGen.Generate(time.Now())
	localID := idGen.Generate(time.Now().Add(time.Millisecond))
	repo.Notes[homeID] = &model.Note{ID: homeID, UserID: "a", Visibility: model.NoteVisibilityPublic}
	repo.Notes[localID] = &model.Note{ID: localID, UserID: "b", Visibility: model.NoteVisibilityPublic}

	require.NoError(t, fanout.Push(ctx, HomeTimelineName(user.ID), homeID, 100))
	require.NoError(t, fanout.Push(ctx, LocalTimeline, localID, 100))

	out, err := svc.HybridTimeline(ctx, user, "", "", 10)
	require.NoError(t, err)
	require.Len(t, out, 2)
}

func TestService_HybridTimelineFanoutError(t *testing.T) {
	noteRepo := testutil.NewMockNoteRepository()
	svc := NewService(NewFanoutTimelineService(closedClient(t), idGen), noteRepo, nil)
	_, err := svc.HybridTimeline(context.Background(), &model.User{ID: "u"}, "", "", 10)
	assert.Error(t, err)
}

func TestService_HybridTimelineDeduplicates(t *testing.T) {
	svc, fanout, repo := newTestServiceWithRepo(t)
	ctx := context.Background()

	user := &model.User{ID: "viewer"}
	noteID := idGen.Generate(time.Now())
	repo.Notes[noteID] = &model.Note{ID: noteID, UserID: "a", Visibility: model.NoteVisibilityPublic}
	// 同じIDをhome/localの両方にpushする
	require.NoError(t, fanout.Push(ctx, HomeTimelineName(user.ID), noteID, 100))
	require.NoError(t, fanout.Push(ctx, LocalTimeline, noteID, 100))

	out, err := svc.HybridTimeline(ctx, user, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestService_ResolveEmpty(t *testing.T) {
	svc, _, _ := newTestServiceWithRepo(t)
	out, err := svc.LocalTimeline(context.Background(), nil, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, out)
}

// --- DB fallback tests (Redis empty) ---

func TestService_HomeTimeline_DBFallback(t *testing.T) {
	svc, _, repo := newTestServiceWithRepo(t)
	// Redisにpushしない → DBフォールバック
	repo.Notes["n1"] = &model.Note{ID: "n1", UserID: "viewer", Visibility: model.NoteVisibilityPublic}
	out, err := svc.HomeTimeline(context.Background(), &model.User{ID: "viewer"}, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestService_LocalTimeline_DBFallback(t *testing.T) {
	svc, _, repo := newTestServiceWithRepo(t)
	repo.Notes["n1"] = &model.Note{ID: "n1", UserID: "a", Visibility: model.NoteVisibilityPublic}
	out, err := svc.LocalTimeline(context.Background(), nil, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestService_GlobalTimeline_DBFallback(t *testing.T) {
	svc, _, repo := newTestServiceWithRepo(t)
	repo.Notes["n1"] = &model.Note{ID: "n1", UserID: "a", Visibility: model.NoteVisibilityPublic}
	out, err := svc.GlobalTimeline(context.Background(), nil, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestService_HybridTimeline_DBFallback(t *testing.T) {
	svc, _, repo := newTestServiceWithRepo(t)
	repo.Notes["n1"] = &model.Note{ID: "n1", UserID: "viewer", Visibility: model.NoteVisibilityPublic}
	out, err := svc.HybridTimeline(context.Background(), &model.User{ID: "viewer"}, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestService_HomeTimeline_DefaultLimit(t *testing.T) {
	svc, _, _ := newTestServiceWithRepo(t)
	out, err := svc.HomeTimeline(context.Background(), &model.User{ID: "u"}, "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestService_LocalTimeline_DefaultLimit(t *testing.T) {
	svc, _, _ := newTestServiceWithRepo(t)
	out, err := svc.LocalTimeline(context.Background(), nil, "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestService_GlobalTimeline_DefaultLimit(t *testing.T) {
	svc, _, _ := newTestServiceWithRepo(t)
	out, err := svc.GlobalTimeline(context.Background(), nil, "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestService_HybridTimeline_DefaultLimit(t *testing.T) {
	svc, _, _ := newTestServiceWithRepo(t)
	out, err := svc.HybridTimeline(context.Background(), &model.User{ID: "u"}, "", "", 0)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestService_ResolveError(t *testing.T) {
	// RedisにIDはあるがrepoのFindManyByIDsWithUserがエラー
	testRedis.FlushAll(context.Background())
	failRepo := &failingFindManyRepo{testutil.NewMockNoteRepository()}
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	svc := NewService(fanout, failRepo, testutil.NewMockFollowingRepository())
	ctx := context.Background()

	noteID := idGen.Generate(time.Now())
	require.NoError(t, fanout.Push(ctx, LocalTimeline, noteID, 100))
	_, err := svc.LocalTimeline(ctx, nil, "", "", 10)
	assert.Error(t, err)
}

type failingFindManyRepo struct{ *testutil.MockNoteRepository }

func (f *failingFindManyRepo) FindManyByIDsWithUser(_ []string) ([]*model.Note, error) {
	return nil, assert.AnError
}

func TestMergeIDs_LimitClamping(t *testing.T) {
	out := mergeIDs([][]string{{"a", "b"}, {"c", "d"}}, 2)
	assert.Len(t, out, 2)
	// 文字列降順
	assert.Equal(t, "d", out[0])
	assert.Equal(t, "c", out[1])
}
