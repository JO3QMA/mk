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
