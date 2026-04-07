// Package nodeinfo provides /nodeinfo/* endpoints.
package nodeinfo

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
)

// Handler handles nodeinfo endpoints.
type Handler struct {
	cfg *config.Config
}

// NewHandler constructs a Handler.
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}

// Version2_1 handles GET /nodeinfo/2.1.
func (h *Handler) Version2_1(c echo.Context) error {
	resp := map[string]any{
		"version": "2.1",
		"software": map[string]any{
			"name":       "misskey-go",
			"version":    h.cfg.Version,
			"repository": "https://github.com/shiroha-a/mk",
		},
		"protocols": []string{"activitypub"},
		"services": map[string]any{
			"inbound":  []string{},
			"outbound": []string{"atom1.0", "rss2.0"},
		},
		"openRegistrations": false,
		"usage": map[string]any{
			"users": map[string]any{
				"total":          0,
				"activeMonth":    0,
				"activeHalfyear": 0,
			},
			"localPosts":    0,
			"localComments": 0,
		},
		"metadata": map[string]any{
			"nodeName": h.cfg.Host,
		},
	}
	return c.JSON(http.StatusOK, resp)
}
