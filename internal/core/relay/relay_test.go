package relay_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/relay"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSysAcct records Fetch calls and returns a canned user.
type fakeSysAcct struct {
	user *model.User
	err  error
	mu   sync.Mutex
	n    int
}

func (f *fakeSysAcct) Fetch(kind string) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	return f.user, f.err
}

// fakeDeliverer captures bodies + inboxes for assertion.
type fakeDeliverer struct {
	mu    sync.Mutex
	calls []deliverCall
	err   error
}

type deliverCall struct {
	SignerID string
	Body     []byte
	Inboxes  []string
}

func (d *fakeDeliverer) DeliverActivity(signerUserID string, body []byte, inboxes []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, deliverCall{SignerID: signerUserID, Body: append([]byte(nil), body...), Inboxes: append([]string(nil), inboxes...)})
	return d.err
}

func newService(t *testing.T) (*relay.Service, *testutil.MockRelayRepository, *fakeSysAcct, *fakeDeliverer) {
	t.Helper()
	repo := testutil.NewMockRelayRepository()
	sys := &fakeSysAcct{user: &model.User{ID: "sysrelay"}}
	deliv := &fakeDeliverer{}
	r := activitypub.NewRenderer(activitypub.NewURLBuilder("https://example.com"))
	idGen, err := id.NewGenerator("aidx")
	require.NoError(t, err)
	svc := relay.NewService(repo, sys, r, deliv, idGen)
	svc.SetClock(func() time.Time { return time.Unix(1_700_000_000, 0).UTC() })
	return svc, repo, sys, deliv
}

func TestAdd_PersistsRowAndDeliversFollow(t *testing.T) {
	svc, repo, sys, deliv := newService(t)
	rel, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
	require.NotNil(t, rel)
	assert.Equal(t, "https://r.example/inbox", rel.Inbox)
	assert.Equal(t, relay.StatusRequesting, rel.Status)
	assert.Len(t, repo.Relays, 1)
	assert.Equal(t, 1, sys.n)
	require.Len(t, deliv.calls, 1)
	call := deliv.calls[0]
	assert.Equal(t, "sysrelay", call.SignerID)
	assert.Equal(t, []string{"https://r.example/inbox"}, call.Inboxes)
	// body は Follow Activity
	var follow map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &follow))
	assert.Equal(t, "Follow", follow["type"])
	assert.Equal(t, activitypub.Public, follow["object"])
}

func TestAdd_EmptyInboxRejected(t *testing.T) {
	svc, _, _, _ := newService(t)
	_, err := svc.Add(context.Background(), "")
	assert.Error(t, err)
}

func TestAdd_DelivererErrorBubbles(t *testing.T) {
	svc, repo, _, deliv := newService(t)
	deliv.err = errors.New("network")
	_, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.Error(t, err)
	// Follow 配信失敗時に挿入済み行がロールバックされる: unique inbox 制約を
	// 避けるため次回 Add を可能にする重要な挙動 (Devin review #171 対応)。
	assert.Empty(t, repo.Relays)
}

func TestRemove_SendsUndoAndDeletes(t *testing.T) {
	svc, repo, _, deliv := newService(t)
	rel, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
	deliv.mu.Lock()
	deliv.calls = nil
	deliv.mu.Unlock()

	require.NoError(t, svc.Remove(context.Background(), rel.ID))
	assert.Empty(t, repo.Relays)
	require.Len(t, deliv.calls, 1)
	var undo map[string]any
	require.NoError(t, json.Unmarshal(deliv.calls[0].Body, &undo))
	assert.Equal(t, "Undo", undo["type"])
	// 元 Follow が object にネストされている
	inner, ok := undo["object"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Follow", inner["type"])
}

func TestRemove_NotFoundIsNoop(t *testing.T) {
	svc, _, _, deliv := newService(t)
	// 存在しない id を渡しても err=nil、deliver も呼ばれない
	require.NoError(t, svc.Remove(context.Background(), "nonexistent"))
	assert.Empty(t, deliv.calls)
}

func TestList_ReturnsAll(t *testing.T) {
	svc, _, _, _ := newService(t)
	_, err := svc.Add(context.Background(), "https://a.example/inbox")
	require.NoError(t, err)
	_, err = svc.Add(context.Background(), "https://b.example/inbox")
	require.NoError(t, err)

	list, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestMarkAccepted_UpdatesStatus(t *testing.T) {
	svc, repo, _, _ := newService(t)
	rel, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
	require.NoError(t, svc.MarkAccepted(context.Background(), rel.ID))
	assert.Equal(t, relay.StatusAccepted, repo.Relays[rel.ID].Status)
}

func TestMarkRejected_UpdatesStatus(t *testing.T) {
	svc, repo, _, _ := newService(t)
	rel, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
	require.NoError(t, svc.MarkRejected(context.Background(), rel.ID))
	assert.Equal(t, relay.StatusRejected, repo.Relays[rel.ID].Status)
}

func TestMarkAccepted_InvalidatesCache(t *testing.T) {
	svc, _, _, deliv := newService(t)
	rel, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
	// まず何もない状態で DeliverToAccepted → no-op (accepted relay が無い)
	require.NoError(t, svc.DeliverToAccepted(context.Background(), "alice", map[string]any{"type": "Note"}))
	deliv.mu.Lock()
	addedCalls := len(deliv.calls)
	deliv.mu.Unlock()

	// MarkAccepted してキャッシュ無効化、再度 DeliverToAccepted で届く
	require.NoError(t, svc.MarkAccepted(context.Background(), rel.ID))
	require.NoError(t, svc.DeliverToAccepted(context.Background(), "alice", map[string]any{"type": "Note"}))

	deliv.mu.Lock()
	finalCalls := len(deliv.calls)
	deliv.mu.Unlock()
	assert.Equal(t, addedCalls+1, finalCalls)
}

func TestDeliverToAccepted_NilActivity(t *testing.T) {
	svc, _, _, deliv := newService(t)
	require.NoError(t, svc.DeliverToAccepted(context.Background(), "alice", nil))
	assert.Empty(t, deliv.calls)
}

func TestDeliverToAccepted_CacheHit(t *testing.T) {
	svc, repo, _, _ := newService(t)
	// accepted relay を直接 repo に入れる (Add→MarkAccepted の順番を短縮)
	rel := &model.Relay{ID: "r1", Inbox: "https://a.example/inbox", Status: relay.StatusAccepted}
	require.NoError(t, repo.Create(rel))

	// 1 回目: DB から取得してキャッシュ
	require.NoError(t, svc.DeliverToAccepted(context.Background(), "alice", map[string]any{"type": "Note"}))
	// 途中で DB から消しても、キャッシュ TTL 内なら 2 回目も届く
	delete(repo.Relays, "r1")
	require.NoError(t, svc.DeliverToAccepted(context.Background(), "alice", map[string]any{"type": "Note"}))
}

func TestSetClockAndCacheMaxAge_NilZeroIgnored(t *testing.T) {
	svc, _, _, _ := newService(t)
	svc.SetClock(nil)
	svc.SetCacheMaxAge(0)
	_, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
}

func TestAdd_SysAccountErrorBubbles(t *testing.T) {
	svc, _, sys, _ := newService(t)
	sys.err = errors.New("sys-down")
	_, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sys-down")
}

// Remove は Undo 配信エラーが起きても行自体は削除する (Devin review #171):
// 行が残ると admin UI から relay が消えず管理者が復旧できなくなるため、
// 配信失敗は警告ログで済ませ delete は必ず実行する。
func TestRemove_UndoDeliveryErrorStillDeletesRow(t *testing.T) {
	svc, repo, sys, _ := newService(t)
	rel, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
	sys.err = errors.New("sys-down")
	require.NoError(t, svc.Remove(context.Background(), rel.ID))
	// 行が削除されている
	assert.Empty(t, repo.Relays)
}

// failingRepo lets tests exercise repository error branches.
type failingRepo struct {
	*testutil.MockRelayRepository
	createErr error
	updateErr error
	listErr   error
	deleteErr error
}

func (f *failingRepo) Create(r *model.Relay) error {
	if f.createErr != nil {
		return f.createErr
	}
	return f.MockRelayRepository.Create(r)
}
func (f *failingRepo) UpdateStatus(id, status string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	return f.MockRelayRepository.UpdateStatus(id, status)
}
func (f *failingRepo) ListByStatus(status string) ([]*model.Relay, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.MockRelayRepository.ListByStatus(status)
}
func (f *failingRepo) Delete(id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return f.MockRelayRepository.Delete(id)
}

func TestAdd_RepoCreateErrorBubbles(t *testing.T) {
	repo := &failingRepo{MockRelayRepository: testutil.NewMockRelayRepository(), createErr: errors.New("db boom")}
	sys := &fakeSysAcct{user: &model.User{ID: "sysrelay"}}
	deliv := &fakeDeliverer{}
	r := activitypub.NewRenderer(activitypub.NewURLBuilder("https://example.com"))
	idGen, _ := id.NewGenerator("aidx")
	svc := relay.NewService(repo, sys, r, deliv, idGen)
	_, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.Error(t, err)
}

func TestMarkAccepted_RepoErrorBubbles(t *testing.T) {
	repo := &failingRepo{MockRelayRepository: testutil.NewMockRelayRepository(), updateErr: errors.New("db boom")}
	sys := &fakeSysAcct{user: &model.User{ID: "sysrelay"}}
	deliv := &fakeDeliverer{}
	r := activitypub.NewRenderer(activitypub.NewURLBuilder("https://example.com"))
	idGen, _ := id.NewGenerator("aidx")
	svc := relay.NewService(repo, sys, r, deliv, idGen)
	err := svc.MarkAccepted(context.Background(), "x")
	assert.Error(t, err)
	err = svc.MarkRejected(context.Background(), "x")
	assert.Error(t, err)
}

func TestDeliverToAccepted_RepoListError(t *testing.T) {
	repo := &failingRepo{MockRelayRepository: testutil.NewMockRelayRepository(), listErr: errors.New("db boom")}
	sys := &fakeSysAcct{user: &model.User{ID: "sysrelay"}}
	deliv := &fakeDeliverer{}
	r := activitypub.NewRenderer(activitypub.NewURLBuilder("https://example.com"))
	idGen, _ := id.NewGenerator("aidx")
	svc := relay.NewService(repo, sys, r, deliv, idGen)
	err := svc.DeliverToAccepted(context.Background(), "alice", map[string]any{"type": "Note"})
	assert.Error(t, err)
}

func TestRemove_RepoDeleteErrorBubbles(t *testing.T) {
	repo := &failingRepo{MockRelayRepository: testutil.NewMockRelayRepository(), deleteErr: errors.New("db boom")}
	sys := &fakeSysAcct{user: &model.User{ID: "sysrelay"}}
	deliv := &fakeDeliverer{}
	r := activitypub.NewRenderer(activitypub.NewURLBuilder("https://example.com"))
	idGen, _ := id.NewGenerator("aidx")
	svc := relay.NewService(repo, sys, r, deliv, idGen)
	rel, err := svc.Add(context.Background(), "https://r.example/inbox")
	require.NoError(t, err)
	err = svc.Remove(context.Background(), rel.ID)
	assert.Error(t, err)
}

// upstream Misskey #17308 (= 2026.5.0 fix / triage #1002): relay 由来の Announce
// を検出するため Service に IsRelayActor を追加。actor.Inbox / SharedInbox が
// 登録済み (status=accepted) relay の Inbox と一致したら true。
func TestIsRelayActor(t *testing.T) {
	svc, repo, _, _ := newService(t)
	relayInbox := "https://relay.example/inbox"
	// accepted な relay を登録 (= cache 経由で IsRelayActor の判定対象に入る)
	repo.Create(&model.Relay{ID: "r1", Inbox: relayInbox, Status: relay.StatusAccepted})

	t.Run("nil_actor", func(t *testing.T) {
		assert.False(t, svc.IsRelayActor(nil))
	})

	t.Run("actor_without_inbox", func(t *testing.T) {
		assert.False(t, svc.IsRelayActor(&model.User{ID: "u1"}))
	})

	t.Run("actor_inbox_does_not_match", func(t *testing.T) {
		other := "https://other.example/inbox"
		assert.False(t, svc.IsRelayActor(&model.User{ID: "u2", Inbox: &other}))
	})

	t.Run("actor_inbox_matches", func(t *testing.T) {
		inbox := relayInbox
		assert.True(t, svc.IsRelayActor(&model.User{ID: "u3", Inbox: &inbox}),
			"actor.Inbox が登録済み relay の Inbox と一致 → true")
	})

	t.Run("actor_shared_inbox_matches", func(t *testing.T) {
		other := "https://relay.example/users/relay/inbox"
		shared := relayInbox
		assert.True(t, svc.IsRelayActor(&model.User{ID: "u4", Inbox: &other, SharedInbox: &shared}),
			"actor.SharedInbox が登録済み relay の Inbox と一致 → true")
	})
}
