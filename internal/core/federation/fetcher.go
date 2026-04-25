package federation

import (
	"errors"
	"log/slog"

	"github.com/shiroha-a/mk/internal/activitypub"
)

// SignerProvider supplies the credentials of a local signer (typically the
// instance.actor system account) used to sign outgoing AP fetches. Returning
// an error keeps APFetcher in unsigned-only mode for that call so callers
// don't fail just because the system account isn't available yet.
type SignerProvider interface {
	SignerCredentials() (keyID, keyPEM string, err error)
}

// APFetcher wraps an activitypub.Client to satisfy HTTPFetcher。
//
// Default behaviour after #419: try signed GET first using the configured
// SignerProvider (instance.actor), fall back to unsigned GET on failure.
// If no SignerProvider is wired, behaviour reverts to plain unsigned GET so
// existing tests continue to work.
type APFetcher struct {
	client *activitypub.Client
	signer SignerProvider
}

// NewAPFetcher constructs an APFetcher.
func NewAPFetcher(client *activitypub.Client) *APFetcher {
	return &APFetcher{client: client}
}

// SetSigner attaches a SignerProvider used for default signed AP fetches.
// 未配線なら従来通り未署名 GET のみ。
func (f *APFetcher) SetSigner(s SignerProvider) {
	f.signer = s
}

// FetchObject performs a default-signed GET against uri, falling back to
// unsigned GET on signed-side error. Resolver でアクター取得やリモート Note
// 取得に共通で使う。
//
// IceShrimp.NET のように authorized-fetch を強制する peer ではこちらが署名
// しないと 401 が返る (#419)。逆に署名検証が緩い peer では unsigned GET の
// 方が成功するケースもあるため、signed → unsigned の二段構えにしておく。
func (f *APFetcher) FetchObject(uri string) ([]byte, error) {
	// signer が wire されていないか、credentials 取得失敗時はそのまま
	// unsigned で行く (test / 起動直後の race)。
	if f.signer != nil {
		if keyID, keyPEM, err := f.signer.SignerCredentials(); err == nil {
			if key, kerr := activitypub.NewPrivateKey(keyID, keyPEM); kerr == nil {
				body, ferr := f.client.FetchJSON(uri, key)
				if ferr == nil {
					return body, nil
				}
				// 署名 fetch が失敗したら unsigned にフォールバック。
				// 失敗種別 (network / 4xx / 5xx) を問わず試行する: peer の
				// 設定次第で signed を弾くケースも有り得るため。
				slog.Debug("ap fetcher: signed fetch failed, falling back to unsigned",
					"uri", uri, "err", ferr)
			} else {
				slog.Debug("ap fetcher: signer key parse failed, falling back to unsigned",
					"err", kerr)
			}
		}
	}
	body, err := f.client.FetchUnsigned(uri)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// FetchHTML performs an unsigned GET with Accept: text/html. Instance metadata
// fetcher がリモートトップページから <link rel="icon"> を抜き出す用途で使う。
func (f *APFetcher) FetchHTML(uri string) ([]byte, error) {
	return f.client.FetchHTML(uri)
}

// ErrNoSigner is returned by SignerProvider implementations when the local
// instance actor / keypair is not yet provisioned. Callers treat this as
// "skip signing this call" rather than a hard failure.
var ErrNoSigner = errors.New("federation: instance signer not available")
