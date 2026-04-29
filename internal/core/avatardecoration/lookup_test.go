package avatardecoration

import (
	"errors"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestResolver_LookupURL_Found(t *testing.T) {
	repo := testutil.NewMockAvatarDecorationRepository()
	repo.Decorations["d1"] = &model.AvatarDecoration{ID: "d1", URL: "https://e/x.png"}
	r := NewResolver(repo)

	url, ok := r.LookupURL("d1")
	assert.True(t, ok)
	assert.Equal(t, "https://e/x.png", url)
}

func TestResolver_LookupURL_NotFound(t *testing.T) {
	r := NewResolver(testutil.NewMockAvatarDecorationRepository())
	_, ok := r.LookupURL("missing")
	assert.False(t, ok)
}

func TestResolver_LookupURL_NilSafe(t *testing.T) {
	var r *Resolver
	_, ok := r.LookupURL("anything")
	assert.False(t, ok)
}

func TestResolver_LookupURL_NilRepo(t *testing.T) {
	r := NewResolver(nil)
	_, ok := r.LookupURL("anything")
	assert.False(t, ok)
}

func TestResolver_LookupURL_EmptyID(t *testing.T) {
	r := NewResolver(testutil.NewMockAvatarDecorationRepository())
	_, ok := r.LookupURL("")
	assert.False(t, ok)
}

// failingDecoRepo causes List to error so refresh keeps the previous map.
type failingDecoRepo struct {
	*testutil.MockAvatarDecorationRepository
	listErr error
}

func (f *failingDecoRepo) List() ([]*model.AvatarDecoration, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.MockAvatarDecorationRepository.List()
}

// countingDecoRepo counts List invocations so backoff tests can verify
// repo.List is suppressed during the failure cooldown.
type countingDecoRepo struct {
	*testutil.MockAvatarDecorationRepository
	listCalls int
	listErr   error
}

func (c *countingDecoRepo) List() ([]*model.AvatarDecoration, error) {
	c.listCalls++
	if c.listErr != nil {
		return nil, c.listErr
	}
	return c.MockAvatarDecorationRepository.List()
}

func TestResolver_LookupURL_RefreshErrorKeepsCache(t *testing.T) {
	mock := testutil.NewMockAvatarDecorationRepository()
	mock.Decorations["d1"] = &model.AvatarDecoration{ID: "d1", URL: "u1"}
	repo := &failingDecoRepo{MockAvatarDecorationRepository: mock}
	r := NewResolver(repo)

	// 最初のロードで cache 構築。
	url, ok := r.LookupURL("d1")
	assert.True(t, ok)
	assert.Equal(t, "u1", url)

	// TTL を強制的に切らして次回 refresh をトリガする。
	r.mu.Lock()
	r.loaded = time.Now().Add(-2 * cacheTTL)
	r.mu.Unlock()
	repo.listErr = errors.New("db down")

	// refresh が失敗しても直前の cache が残る。
	url, ok = r.LookupURL("d1")
	assert.True(t, ok)
	assert.Equal(t, "u1", url)
}

// 失敗時に loaded を failureBackoff 分進めて連続 refresh を抑制する
// (retry storm 防止)。直後の LookupURL は repo.List を再呼び出ししない。
func TestResolver_LookupURL_RefreshErrorBackoffSkipsRepo(t *testing.T) {
	mock := testutil.NewMockAvatarDecorationRepository()
	mock.Decorations["d1"] = &model.AvatarDecoration{ID: "d1", URL: "u1"}
	repo := &countingDecoRepo{MockAvatarDecorationRepository: mock}
	r := NewResolver(repo)

	// 1 回目: 正常に cache 構築 (List 1 回)。
	_, _ = r.LookupURL("d1")
	assert.Equal(t, 1, repo.listCalls)

	// TTL 切れさせて次回 refresh を強制 + 以降は List を必ず失敗させる。
	r.mu.Lock()
	r.loaded = time.Now().Add(-2 * cacheTTL)
	r.mu.Unlock()
	repo.listErr = errors.New("db down")

	// 失敗 refresh で List +1 (合計 2)。loaded は failureBackoff 分進む。
	_, _ = r.LookupURL("d1")
	assert.Equal(t, 2, repo.listCalls)

	// 直後の連続 lookup は backoff 内なので List を呼ばない (合計 2 のまま)。
	for range 5 {
		_, _ = r.LookupURL("d1")
	}
	assert.Equal(t, 2, repo.listCalls, "failure backoff should suppress repo.List during cooldown")
}

func TestResolver_LookupURL_CacheServesWithinTTL(t *testing.T) {
	mock := testutil.NewMockAvatarDecorationRepository()
	mock.Decorations["d1"] = &model.AvatarDecoration{ID: "d1", URL: "u1"}
	r := NewResolver(mock)

	// 1 回目 → DB 経由で cache 構築。
	_, _ = r.LookupURL("d1")
	// catalog を変更しても TTL 内の lookup は古い値を返す。
	mock.Decorations["d1"].URL = "u2"
	url, ok := r.LookupURL("d1")
	assert.True(t, ok)
	assert.Equal(t, "u1", url)
}
