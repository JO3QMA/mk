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

// FetchActor performs an unsigned GET against uri.
func (f *APFetcher) FetchActor(uri string) ([]byte, error) {
	return f.client.FetchUnsigned(uri)
}
