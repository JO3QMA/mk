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

func newTestHook(t *testing.T) (*FanoutHook, *FanoutTimelineService, *testutil.MockFollowingRepository) {
	t.Helper()
	testRedis.FlushAll(context.Background())
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	following := testutil.NewMockFollowingRepository()
	return NewFanoutHook(fanout, following), fanout, following
}

func TestFanoutHook_Nil(t *testing.T) {
	h, _, _ := newTestHook(t)
	// nilノートはno-op
	h.OnNoteCreated(nil, &model.User{ID: "u"})
	h.OnNoteCreated(&model.Note{ID: "n"}, nil)
}

func TestFanoutHook_PublicLocal(t *testing.T) {
	h, fanout, _ := newTestHook(t)
	ctx := context.Background()

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	author := &model.User{ID: "author"} // Hostがnil = local
	h.OnNoteCreated(n, author)

	// home/user/local/global の4タイムラインに入る
	for _, name := range []Name{
		HomeTimelineName("author"),
		UserTimelineName("author"),
		LocalTimeline,
		GlobalTimeline,
	} {
		out, err := fanout.Get(ctx, name, "", "", 10)
		require.NoError(t, err)
		assert.Equal(t, []string{noteID}, out, "timeline %q should contain note", name)
	}
}

func TestFanoutHook_RemoteAuthorSkipsLocal(t *testing.T) {
	h, fanout, _ := newTestHook(t)
	ctx := context.Background()

	noteID := idGen.Generate(time.Now())
	host := "remote.example"
	n := &model.Note{ID: noteID, UserID: "ra", UserHost: &host, Visibility: model.NoteVisibilityPublic}
	author := &model.User{ID: "ra", Host: &host}
	h.OnNoteCreated(n, author)

	out, err := fanout.Get(ctx, LocalTimeline, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, out)

	// グローバルには入る
	out, err = fanout.Get(ctx, GlobalTimeline, "", "", 10)
	require.NoError(t, err)
	assert.Equal(t, []string{noteID}, out)
}

func TestFanoutHook_FollowersVisibilityNoGlobal(t *testing.T) {
	h, fanout, _ := newTestHook(t)
	ctx := context.Background()

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityFollowers}
	author := &model.User{ID: "author"}
	h.OnNoteCreated(n, author)

	out, err := fanout.Get(ctx, GlobalTimeline, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, out)

	out, err = fanout.Get(ctx, LocalTimeline, "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestFanoutHook_SpecifiedDoesNotFanoutToFollowers(t *testing.T) {
	h, fanout, following := newTestHook(t)
	ctx := context.Background()
	following.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "follower", FolloweeID: "author"}

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilitySpecified}
	author := &model.User{ID: "author"}
	h.OnNoteCreated(n, author)

	out, err := fanout.Get(ctx, HomeTimelineName("follower"), "", "", 10)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestFanoutHook_FanoutsToFollowers(t *testing.T) {
	h, fanout, following := newTestHook(t)
	ctx := context.Background()
	following.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "follower1", FolloweeID: "author"}
	following.Followings["f2"] = &model.Following{ID: "f2", FollowerID: "follower2", FolloweeID: "author"}

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	h.OnNoteCreated(n, &model.User{ID: "author"})

	for _, fid := range []string{"follower1", "follower2"} {
		out, err := fanout.Get(ctx, HomeTimelineName(fid), "", "", 10)
		require.NoError(t, err)
		assert.Equal(t, []string{noteID}, out, "follower %s home should receive", fid)
	}
}

// failingFollowingRepo errors out on ListFollowers to exercise the warning path.
type failingFollowingRepo struct {
	*testutil.MockFollowingRepository
}

func (f *failingFollowingRepo) ListFollowers(_ string, _, _ int) ([]*model.Following, error) {
	return nil, assertError{}
}

type assertError struct{}

func (assertError) Error() string { return "boom" }

func TestFanoutHook_ListFollowersError(t *testing.T) {
	testRedis.FlushAll(context.Background())
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	following := &failingFollowingRepo{MockFollowingRepository: testutil.NewMockFollowingRepository()}
	h := NewFanoutHook(fanout, following)

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	// エラーがあっても上には伝搬しない
	h.OnNoteCreated(n, &model.User{ID: "author"})
}

func TestFanoutHook_FanoutsAcrossPages(t *testing.T) {
	testRedis.FlushAll(context.Background())
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	following := testutil.NewMockFollowingRepository()
	// 201人のフォロワーを用意してページ境界を踏ませる
	for i := range 201 {
		fid := "f-" + idGen.Generate(time.Now().Add(time.Duration(i)*time.Microsecond))
		following.Followings[fid] = &model.Following{
			ID:         fid,
			FollowerID: "follower-" + fid,
			FolloweeID: "author",
		}
	}
	h := NewFanoutHook(fanout, following)

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	h.OnNoteCreated(n, &model.User{ID: "author"})
	// 配信成功 (詳細な検証はsmoke扱い)
}

func TestFanoutHook_PushErrorIsLogged(t *testing.T) {
	// closed clientへのpushでエラーを発生させる. ログ出力されるだけで例外なし.
	following := testutil.NewMockFollowingRepository()
	fanout := NewFanoutTimelineService(closedClient(t), idGen)
	h := NewFanoutHook(fanout, following)
	noteID := idGen.Generate(time.Now())
	h.OnNoteCreated(&model.Note{ID: noteID, UserID: "u", Visibility: model.NoteVisibilityPublic}, &model.User{ID: "u"})
}

func TestFanoutHook_NilFollowingRepo(t *testing.T) {
	testRedis.FlushAll(context.Background())
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	h := NewFanoutHook(fanout, nil)
	noteID := idGen.Generate(time.Now())
	h.OnNoteCreated(&model.Note{ID: noteID, UserID: "u", Visibility: model.NoteVisibilityPublic}, &model.User{ID: "u"})
}

// stubStreamingPublisher records every PublishNote call.
type stubStreamingPublisher struct {
	topics []string
}

func (s *stubStreamingPublisher) PublishNote(topic string, _ *model.Note, _ *model.User) {
	s.topics = append(s.topics, topic)
}

func TestFanoutHook_PublishesStreamingTopics(t *testing.T) {
	h, _, following := newTestHook(t)
	following.Followings["f1"] = &model.Following{ID: "f1", FollowerID: "follower1", FolloweeID: "author"}
	pub := &stubStreamingPublisher{}
	h.SetStreamingPublisher(pub)

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	h.OnNoteCreated(n, &model.User{ID: "author"})

	assert.Contains(t, pub.topics, "homeTimeline:author")
	assert.Contains(t, pub.topics, "homeTimeline:follower1")
	assert.Contains(t, pub.topics, "localTimeline")
	assert.Contains(t, pub.topics, "globalTimeline")
}

func TestFanoutHook_StreamingHomeOnlyForFollowersVisibility(t *testing.T) {
	h, _, _ := newTestHook(t)
	pub := &stubStreamingPublisher{}
	h.SetStreamingPublisher(pub)

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityFollowers}
	h.OnNoteCreated(n, &model.User{ID: "author"})

	// followers visibility は localTimeline / globalTimeline 配信なし
	assert.NotContains(t, pub.topics, "localTimeline")
	assert.NotContains(t, pub.topics, "globalTimeline")
	assert.Contains(t, pub.topics, "homeTimeline:author")
}

func TestFanoutHook_StreamingPublisherUnsetIsNoOp(t *testing.T) {
	h, _, _ := newTestHook(t)
	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	h.OnNoteCreated(n, &model.User{ID: "author"})
}

func TestFanoutHook_StreamingFanoutErrorPath(t *testing.T) {
	// failingFollowingRepo は ListFollowers でエラーを返す → fanoutStreamingToFollowers
	// は早期 return する
	testRedis.FlushAll(context.Background())
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	following := &failingFollowingRepo{MockFollowingRepository: testutil.NewMockFollowingRepository()}
	h := NewFanoutHook(fanout, following)
	pub := &stubStreamingPublisher{}
	h.SetStreamingPublisher(pub)

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	h.OnNoteCreated(n, &model.User{ID: "author"})
	// follower 配信は届かないが、自分宛と local/global は publish される
	assert.Contains(t, pub.topics, "homeTimeline:author")
}

func TestFanoutHook_StreamingFanoutAcrossPages(t *testing.T) {
	testRedis.FlushAll(context.Background())
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	following := testutil.NewMockFollowingRepository()
	for i := range 201 {
		fid := "f-" + idGen.Generate(time.Now().Add(time.Duration(i)*time.Microsecond))
		following.Followings[fid] = &model.Following{
			ID:         fid,
			FollowerID: "follower-" + fid,
			FolloweeID: "author",
		}
	}
	h := NewFanoutHook(fanout, following)
	pub := &stubStreamingPublisher{}
	h.SetStreamingPublisher(pub)

	noteID := idGen.Generate(time.Now())
	n := &model.Note{ID: noteID, UserID: "author", Visibility: model.NoteVisibilityPublic}
	h.OnNoteCreated(n, &model.User{ID: "author"})
	// 201 人 + 自分 + local + global = 204 トピック以上
	assert.GreaterOrEqual(t, len(pub.topics), 201+1+1+1)
}

func TestFanoutHook_StreamingPublisherNilFollowingRepo(t *testing.T) {
	testRedis.FlushAll(context.Background())
	fanout := NewFanoutTimelineService(testRedis.Client, idGen)
	fanout.randFn = func() float64 { return 1.0 }
	h := NewFanoutHook(fanout, nil)
	pub := &stubStreamingPublisher{}
	h.SetStreamingPublisher(pub)
	// followingRepo nil でも publish 自体は走る (followers パートだけスキップ)
	noteID := idGen.Generate(time.Now())
	h.OnNoteCreated(&model.Note{ID: noteID, UserID: "u", Visibility: model.NoteVisibilityPublic}, &model.User{ID: "u"})
}
