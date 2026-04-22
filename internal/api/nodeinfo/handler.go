// Package nodeinfo provides /nodeinfo/* endpoints.
package nodeinfo

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/repository"
)

// Handler handles nodeinfo endpoints.
type Handler struct {
	cfg      *config.Config
	metaRepo repository.MetaRepository
}

// NewHandler constructs a Handler.
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}

// SetMetaRepo injects a MetaRepository so that the nodeName / nodeDescription
// fields reflect the live admin settings instead of the config default.
// 未配線のまま呼ばれると cfg.Host fallback になる (#348)。
func (h *Handler) SetMetaRepo(r repository.MetaRepository) {
	h.metaRepo = r
}

// nodeinfoVersion builds the version string for NodeInfo.
func nodeinfoVersion(mkgoVersion, misskeyVersion string) string {
	return mkgoVersion + " (compatible: misskey " + misskeyVersion + ")"
}

// Version2_1 handles GET /nodeinfo/2.1.
func (h *Handler) Version2_1(c echo.Context) error {
	nodeName := h.cfg.Host
	var nodeDescription string
	var maintainerName, maintainerEmail string
	openRegistrations := false
	if h.metaRepo != nil {
		if m, err := h.metaRepo.Fetch(); err == nil && m != nil {
			if m.Name != nil && *m.Name != "" {
				nodeName = *m.Name
			}
			if m.Description != nil {
				nodeDescription = *m.Description
			}
			if m.MaintainerName != nil {
				maintainerName = *m.MaintainerName
			}
			if m.MaintainerEmail != nil {
				maintainerEmail = *m.MaintainerEmail
			}
			// DisableRegistration=true が登録無効、openRegistrations はその反対。
			openRegistrations = !m.DisableRegistration
		}
	}
	metadata := map[string]any{
		"nodeName":        nodeName,
		"nodeDescription": nodeDescription,
		"maintainer": map[string]any{
			"name":  maintainerName,
			"email": maintainerEmail,
		},
	}
	resp := map[string]any{
		"version": "2.1",
		"software": map[string]any{
			"name":       "misskey-go",
			"version":    nodeinfoVersion(config.MkGoVersion, h.cfg.Version),
			"repository": "https://github.com/shiroha-a/mk",
		},
		"protocols": []string{"activitypub"},
		"services": map[string]any{
			"inbound":  []string{},
			"outbound": []string{"atom1.0", "rss2.0"},
		},
		"openRegistrations": openRegistrations,
		"usage": map[string]any{
			"users": map[string]any{
				"total":          0,
				"activeMonth":    0,
				"activeHalfyear": 0,
			},
			"localPosts":    0,
			"localComments": 0,
		},
		"metadata": metadata,
	}
	return c.JSON(http.StatusOK, resp)
}
