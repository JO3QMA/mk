// Package translate provides note translation via external APIs.
// Currently supports DeepL (Free and Pro).
package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var (
	ErrNotConfigured = errors.New("translate: translator not configured")
	ErrRequestFailed = errors.New("translate: request failed")
	ErrNoResult      = errors.New("translate: no translation result")
)

const (
	deeplFreeURL = "https://api-free.deepl.com/v2/translate"
	deeplProURL  = "https://api.deepl.com/v2/translate"
)

// DeepLClient translates text via the DeepL API.
type DeepLClient struct {
	authKey string
	apiURL  string
	client  *http.Client
}

// NewDeepL creates a DeepL translator. isPro selects the Pro endpoint.
func NewDeepL(authKey string, isPro bool) *DeepLClient {
	apiURL := deeplFreeURL
	if isPro {
		apiURL = deeplProURL
	}
	return &DeepLClient{authKey: authKey, apiURL: apiURL, client: http.DefaultClient}
}

// NewDeepLWithClient allows injecting a custom http.Client (for tests).
func NewDeepLWithClient(authKey, apiURL string, client *http.Client) *DeepLClient {
	return &DeepLClient{authKey: authKey, apiURL: apiURL, client: client}
}

type deeplResponse struct {
	Translations []struct {
		DetectedSourceLanguage string `json:"detected_source_language"`
		Text                   string `json:"text"`
	} `json:"translations"`
}

// TranslateResult holds a single translation result.
type TranslateResult struct {
	SourceLang string `json:"sourceLang"`
	Text       string `json:"text"`
}

// Translate sends text to DeepL and returns the translated text.
func (c *DeepLClient) Translate(ctx context.Context, text, targetLang string) (*TranslateResult, error) {
	body := url.Values{
		"text":        {text},
		"target_lang": {strings.ToUpper(targetLang)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "DeepL-Auth-Key "+c.authKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrRequestFailed, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ErrRequestFailed, resp.StatusCode, string(data))
	}

	var result deeplResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("%w: parse: %v", ErrRequestFailed, err)
	}
	if len(result.Translations) == 0 {
		return nil, ErrNoResult
	}
	return &TranslateResult{
		SourceLang: strings.ToLower(result.Translations[0].DetectedSourceLanguage),
		Text:       result.Translations[0].Text,
	}, nil
}
