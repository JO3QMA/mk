// Package nodeinfo provides /nodeinfo/* endpoints.
package nodeinfo

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/repository"
)

// Handler handles nodeinfo endpoints.
type Handler struct {
	cfg      *config.Config
	metaRepo repository.MetaRepository
	userRepo repository.UserRepository
	noteRepo repository.NoteRepository
	clock    func() time.Time
}

// NewHandler constructs a Handler.
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg, clock: time.Now}
}

// SetMetaRepo injects a MetaRepository so that the nodeName / nodeDescription
// fields reflect the live admin settings instead of the config default.
// 未配線のまま呼ばれると cfg.Host fallback になる (#348)。
func (h *Handler) SetMetaRepo(r repository.MetaRepository) {
	h.metaRepo = r
}

// SetUsageRepos injects repositories used to populate the usage statistics
// (users.total / activeMonth / activeHalfyear / localPosts / localComments).
// 未配線のまま呼ばれると対応 field は 0 のままになる (#403)。
func (h *Handler) SetUsageRepos(userRepo repository.UserRepository, noteRepo repository.NoteRepository) {
	h.userRepo = userRepo
	h.noteRepo = noteRepo
}

// SetClock overrides the clock source. Intended for tests.
func (h *Handler) SetClock(now func() time.Time) {
	if now != nil {
		h.clock = now
	}
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
	// 統計値は repo 経由で集計。未配線なら 0 (#403)。
	var (
		usersTotal, usersMonth, usersHalf, localPosts, localComments int64
	)
	if h.userRepo != nil {
		now := h.clock()
		if v, err := h.userRepo.CountLocalUsers(); err == nil {
			usersTotal = v
		}
		if v, err := h.userRepo.CountLocalUsersActiveSince(now.AddDate(0, -1, 0)); err == nil {
			usersMonth = v
		}
		if v, err := h.userRepo.CountLocalUsersActiveSince(now.AddDate(0, -6, 0)); err == nil {
			usersHalf = v
		}
	}
	if h.noteRepo != nil {
		if v, err := h.noteRepo.CountLocalNotes(); err == nil {
			localPosts = v
		}
		if v, err := h.noteRepo.CountLocalComments(); err == nil {
			localComments = v
		}
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
				"total":          usersTotal,
				"activeMonth":    usersMonth,
				"activeHalfyear": usersHalf,
			},
			"localPosts":    localPosts,
			"localComments": localComments,
		},
		"metadata": metadata,
	}
	return c.JSON(http.StatusOK, resp)
}
