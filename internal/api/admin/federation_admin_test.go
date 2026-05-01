package admin_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFederationDeleteAllFiles(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationDeleteAllFiles, `{}`, adminUser).Code)
}

func TestFederationRefreshRemoteInstanceMetadata(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	// fetcher 未設定で host 未指定 (stub 相当の呼出) → 204 で no-op
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationRefreshRemoteInstanceMetadata, `{}`, adminUser).Code)
}

// stubInstanceMetadataFetcher records Fetch calls for assertion.
type stubInstanceMetadataFetcher struct {
	calls []string
	err   error
}

func (s *stubInstanceMetadataFetcher) Fetch(host string) error {
	s.calls = append(s.calls, host)
	return s.err
}

func TestFederationRefreshRemoteInstanceMetadata_CallsFetcher(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	fetcher := &stubInstanceMetadataFetcher{}
	h.SetInstanceMetadataFetcher(fetcher)
	assert.Equal(t, http.StatusNoContent,
		doPost(h.FederationRefreshRemoteInstanceMetadata, `{"host":"remote.example"}`, adminUser).Code)
	assert.Equal(t, []string{"remote.example"}, fetcher.calls)
}

func TestFederationRefreshRemoteInstanceMetadata_EmptyHostNoCall(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	fetcher := &stubInstanceMetadataFetcher{}
	h.SetInstanceMetadataFetcher(fetcher)
	assert.Equal(t, http.StatusNoContent,
		doPost(h.FederationRefreshRemoteInstanceMetadata, `{}`, adminUser).Code)
	// host 未指定で fetcher は叩かれない
	assert.Empty(t, fetcher.calls)
}

func TestFederationRefreshRemoteInstanceMetadata_FetchError_Still204(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	fetcher := &stubInstanceMetadataFetcher{err: errors.New("net down")}
	h.SetInstanceMetadataFetcher(fetcher)
	// fetch 失敗してもクライアントには 204 を返す (ログのみ)
	assert.Equal(t, http.StatusNoContent,
		doPost(h.FederationRefreshRemoteInstanceMetadata, `{"host":"remote.example"}`, adminUser).Code)
	assert.Equal(t, []string{"remote.example"}, fetcher.calls)
}

func TestFederationRemoveAllFollowing(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationRemoveAllFollowing, `{}`, adminUser).Code)
}

func TestFederationUpdateInstance(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationUpdateInstance, `{}`, adminUser).Code)
}
