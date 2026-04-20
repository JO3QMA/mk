package mediaproxy

import (
	"net/http"

	"github.com/shiroha-a/mk/internal/safehttp"
)

// ErrSSRFBlocked is re-exported from safehttp for backward compatibility with
// existing callers. New code should import safehttp.ErrSSRFBlocked directly.
var ErrSSRFBlocked = safehttp.ErrSSRFBlocked

// NewSSRFSafeTransport returns an *http.Transport with a custom DialContext
// that resolves DNS first and rejects connections to private/reserved IPs.
// Thin wrapper over safehttp.NewSSRFSafeTransport (#323 で共通化した)。
func NewSSRFSafeTransport(allowedCIDRs []string) *http.Transport {
	return safehttp.NewSSRFSafeTransport(allowedCIDRs)
}
