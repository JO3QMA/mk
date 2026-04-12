package testutil

import "context"

// MockAllowlistChecker is a test double for mediaproxy.AllowlistChecker.
type MockAllowlistChecker struct {
	AllowedURLs map[string]bool
}

// IsAllowedURL returns true if the URL is in the AllowedURLs map.
func (m *MockAllowlistChecker) IsAllowedURL(_ context.Context, url string) (bool, error) {
	return m.AllowedURLs[url], nil
}
