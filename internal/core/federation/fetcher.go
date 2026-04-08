package federation

import (
	"github.com/shiroha-a/mk/internal/activitypub"
)

// APFetcher wraps an activitypub.Client to satisfy HTTPFetcher.
type APFetcher struct {
	client *activitypub.Client
}

// NewAPFetcher constructs an APFetcher.
func NewAPFetcher(client *activitypub.Client) *APFetcher {
	return &APFetcher{client: client}
}

// FetchObject performs an unsigned GET against uri. Resolver でアクター取得や
// リモート Note 取得に共通で使う。
func (f *APFetcher) FetchObject(uri string) ([]byte, error) {
	return f.client.FetchUnsigned(uri)
}
