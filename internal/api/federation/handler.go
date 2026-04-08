// Package federation provides /api/federation/* endpoints.
package federation

import (
	"net/http"

	"github.com/labstack/echo/v4"
	coreinstance "github.com/shiroha-a/mk/internal/core/instance"
	"github.com/shiroha-a/mk/internal/model"
)

// Handler handles federation-related API endpoints.
type Handler struct {
	svc *coreinstance.Service
}

// NewHandler creates a new federation Handler.
func NewHandler(svc *coreinstance.Service) *Handler {
	return &Handler{svc: svc}
}

// InstancesRequest is the request body for federation/instances.
type InstancesRequest struct {
	Host          string `json:"host"`
	Suspended     *bool  `json:"suspended"`
	NotResponding *bool  `json:"notResponding"`
	Federating    *bool  `json:"federating"`
	Subscribing   *bool  `json:"subscribing"`
	Publishing    *bool  `json:"publishing"`
	Sort          string `json:"sort"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

// Instances handles POST /api/federation/instances.
func (h *Handler) Instances(c echo.Context) error {
	var req InstancesRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	filter := model.InstanceListFilter{
		Host:          req.Host,
		Suspended:     req.Suspended,
		NotResponding: req.NotResponding,
		Federating:    req.Federating,
		Subscribing:   req.Subscribing,
		Publishing:    req.Publishing,
		SortBy:        req.Sort,
		Limit:         req.Limit,
		Offset:        req.Offset,
	}
	rows, err := h.svc.List(filter)
	if err != nil {
		return internalError(c)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, inst := range rows {
		out = append(out, instanceToMap(inst))
	}
	return c.JSON(http.StatusOK, out)
}

// ShowInstanceRequest is the request body for federation/show-instance.
type ShowInstanceRequest struct {
	Host string `json:"host"`
}

// ShowInstance handles POST /api/federation/show-instance.
func (h *Handler) ShowInstance(c echo.Context) error {
	var req ShowInstanceRequest
	if err := c.Bind(&req); err != nil || req.Host == "" {
		return invalidParam(c)
	}
	inst, err := h.svc.FindByHost(req.Host)
	if err != nil {
		// Service.FindByHost は常に ErrInstanceNotFound にラップされるため
		// 404 のみを返す。
		return notFound(c)
	}
	return c.JSON(http.StatusOK, instanceToMap(inst))
}

// instanceToMap shapes an Instance row into the JSON response object expected
// by misskey-js. 不明値や nil ポインタはそのまま JSON null になるよう map に
// 載せる。
func instanceToMap(inst *model.Instance) map[string]any {
	return map[string]any{
		"id":                      inst.ID,
		"firstRetrievedAt":        inst.FirstRetrievedAt,
		"host":                    inst.Host,
		"usersCount":              inst.UsersCount,
		"notesCount":              inst.NotesCount,
		"followingCount":          inst.FollowingCount,
		"followersCount":          inst.FollowersCount,
		"latestRequestReceivedAt": inst.LatestRequestReceivedAt,
		"isNotResponding":         inst.IsNotResponding,
		"notRespondingSince":      inst.NotRespondingSince,
		"isSuspended":             inst.SuspensionState != model.SuspensionStateNone,
		"suspensionState":         inst.SuspensionState,
		"softwareName":            inst.SoftwareName,
		"softwareVersion":         inst.SoftwareVersion,
		"openRegistrations":       inst.OpenRegistrations,
		"name":                    inst.Name,
		"description":             inst.Description,
		"maintainerName":          inst.MaintainerName,
		"maintainerEmail":         inst.MaintainerEmail,
		"iconUrl":                 inst.IconURL,
		"faviconUrl":              inst.FaviconURL,
		"themeColor":              inst.ThemeColor,
		"infoUpdatedAt":           inst.InfoUpdatedAt,
		"moderationNote":          inst.ModerationNote,
	}
}

func invalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"message": "Invalid param.",
			"code":    "INVALID_PARAM",
			"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
		},
	})
}

func internalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, map[string]any{
		"error": map[string]any{
			"message": "Internal error.",
			"code":    "INTERNAL_ERROR",
			"id":      "5d37dbcb-891e-41ca-a3d6-e690c97775ac",
		},
	})
}

func notFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "No such instance.",
			"code":    "NO_SUCH_INSTANCE",
			"id":      "8be60c3a-6f3a-4d3e-8a13-ba81b9b2c1c8",
		},
	})
}
