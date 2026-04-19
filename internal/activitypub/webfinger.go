package activitypub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// WebFingerLink models a single link entry in an RFC 7033 response.
type WebFingerLink struct {
	Rel      string `json:"rel"`
	Type     string `json:"type,omitempty"`
	Href     string `json:"href,omitempty"`
	Template string `json:"template,omitempty"`
}

// WebFingerResponse models the minimal subset of an RFC 7033 WebFinger
// response needed to discover remote actor URIs.
type WebFingerResponse struct {
	Subject string          `json:"subject"`
	Links   []WebFingerLink `json:"links"`
}

// WebFingerClient performs outbound WebFinger queries used to discover remote
// ActivityPub actor URIs from `(username, host)` pairs.
type WebFingerClient struct {
	httpClient *http.Client
	userAgent  string
	// endpointOverride is used by tests to redirect the default
	// https://<host>/.well-known/webfinger URL to an httptest server.
	endpointOverride func(host string) string
}

// NewWebFingerClient constructs a WebFingerClient. Pass nil for httpClient to
// use http.DefaultClient.
func NewWebFingerClient(httpClient *http.Client, userAgent string) *WebFingerClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &WebFingerClient{httpClient: httpClient, userAgent: userAgent}
}

// SetEndpointOverride replaces the default `https://<host>/.well-known/webfinger`
// URL with the function's return value. Intended for tests only.
func (c *WebFingerClient) SetEndpointOverride(fn func(host string) string) {
	c.endpointOverride = fn
}

// LookupActorURI performs a WebFinger lookup for `acct:<username>@<host>` and
// returns the href of the first `rel="self"` link whose type matches an
// ActivityPub-compatible content type (application/activity+json or
// application/ld+json).
//
// 呼び出し側は返却 URI を activitypub.Resolver.ResolveActor に渡すことで
// リモート actor を取り込める。
func (c *WebFingerClient) LookupActorURI(username, host string) (string, error) {
	if username == "" || host == "" {
		return "", errors.New("webfinger: username and host are required")
	}
	resource := "acct:" + username + "@" + host
	endpoint := c.endpoint(host) + "?resource=" + url.QueryEscape(resource)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("webfinger: build request: %w", err)
	}
	// RFC 7033 §4.2 では application/jrd+json が優先、application/json は互換
	// として受け入れるサーバーが多い。両方を提示しておく。
	req.Header.Set("Accept", "application/jrd+json, application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("webfinger: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("webfinger: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("webfinger: read body: %w", err)
	}
	var wf WebFingerResponse
	if err := json.Unmarshal(body, &wf); err != nil {
		return "", fmt.Errorf("webfinger: parse response: %w", err)
	}
	for _, link := range wf.Links {
		if link.Rel != "self" {
			continue
		}
		if link.Href == "" {
			continue
		}
		if isActivityPubLinkType(link.Type) {
			return link.Href, nil
		}
	}
	return "", errors.New("webfinger: no self link with ActivityPub type")
}

// endpoint returns the WebFinger endpoint URL for a host. The default is
// `https://<host>/.well-known/webfinger`. テストから httptest server に向けた
// い場合のみ SetEndpointOverride で差し替える。
func (c *WebFingerClient) endpoint(host string) string {
	if c.endpointOverride != nil {
		if o := c.endpointOverride(host); o != "" {
			return o
		}
	}
	return "https://" + host + "/.well-known/webfinger"
}

// isActivityPubLinkType reports whether t is an ActivityPub-compatible link
// type. Matches `application/activity+json` と `application/ld+json` のベース
// 部分 (profile パラメータ付き含む)。
func isActivityPubLinkType(t string) bool {
	base := strings.TrimSpace(strings.SplitN(t, ";", 2)[0])
	return base == MimeType || base == "application/ld+json"
}
