package activitypub

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/shiroha-a/mk/internal/safehttp"
)

// MaxBodyBytes caps the response body size for FetchJSON / FetchUnsigned to
// prevent memory exhaustion via attacker-controlled remote AP servers (#323).
const MaxBodyBytes = safehttp.DefaultAPBodyLimit

// Client is a thin wrapper around http.Client that signs outgoing AP requests.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient constructs a Client. Pass nil for httpClient to use http.DefaultClient.
func NewClient(httpClient *http.Client, userAgent string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, userAgent: userAgent}
}

// DisableRedirect configures the HTTP client to reject all redirects.
// meta.allowExternalApRedirect が false のとき呼ぶ。外部 AP サーバーが
// 30x レスポンスを返した場合にオープンリダイレクト攻撃を防ぐ。
func (c *Client) DisableRedirect() {
	c.httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
}

// PostSigned sends a signed POST containing body to url, signed with key.
// 戻り値は呼び出し側で Body.Close() すること。
func (c *Client) PostSigned(url string, body []byte, key *PrivateKey) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", MimeType)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	digest := SHA256Digest(body)
	if err := SignRequest(req, key, digest, []string{"(request-target)", "date", "host", "digest"}); err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

// GetSigned sends a signed GET to url. acceptOverride may be empty to use the
// default activity+json accept header.
func (c *Client) GetSigned(url string, key *PrivateKey, acceptOverride string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if acceptOverride == "" {
		req.Header.Set("Accept", MimeType+`, `+LDMimeType)
	} else {
		req.Header.Set("Accept", acceptOverride)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if err := SignRequest(req, key, "", []string{"(request-target)", "date", "host"}); err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

// FetchJSON performs a signed GET and returns the response body. Non-2xx
// responses produce an error.
func (c *Client) FetchJSON(url string, key *PrivateKey) ([]byte, error) {
	resp, err := c.GetSigned(url, key, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	return safehttp.ReadAllLimit(resp.Body, MaxBodyBytes)
}

// FetchUnsigned performs a plain GET without HTTP signing. 多くのAPサーバーは
// アクター取得を未署名で許可するため、resolver の初回 fetch 用に使う。
func (c *Client) FetchUnsigned(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", MimeType+`, `+LDMimeType)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	return safehttp.ReadAllLimit(resp.Body, MaxBodyBytes)
}

// FetchHTML performs a plain GET with Accept: text/html. リモートインスタンスの
// トップページを取得して <link rel="icon"> 等を抽出するための用途を想定。
// 同じhttpClient (SSRF guard / timeout / redirect policy) を使うので nodeinfo
// 取得と同水準の安全策が効く。
func (c *Client) FetchHTML(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.5")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	return safehttp.ReadAllLimit(resp.Body, MaxBodyBytes)
}
