// Package charts provides the /api/charts/* and /api/charts/user/*
// endpoints. Each endpoint reads from a single chart engine instance
// and returns the nested-object form of the requested time series.
//
// The handlers are intentionally thin: validation, span/limit/offset
// parsing and JSON shaping live here, while the actual aggregation
// logic stays in internal/core/chart/charts.
package charts

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/chart"
)

// Charts is the construction-time bundle of every chart instance the
// handler serves. router.go builds it once when wiring the chart
// management service and passes it into NewHandler.
type Charts struct {
	Notes            *chart.Chart
	Users            *chart.Chart
	Drive            *chart.Chart
	Federation       *chart.Chart
	Instance         *chart.Chart
	ApRequest        *chart.Chart
	ActiveUsers      *chart.Chart
	PerUserNotes     *chart.Chart
	PerUserDrive     *chart.Chart
	PerUserFollowing *chart.Chart
	PerUserPv        *chart.Chart
	PerUserReaction  *chart.Chart
}

// Handler serves every chart endpoint. Each chart pointer is allowed
// to be nil — endpoints whose chart is not configured return a 503-
// equivalent JSON error so wiring bugs are visible to clients without
// crashing the server.
type Handler struct {
	c     Charts
	clock chart.Clock
}

// NewHandler constructs a handler from a Charts bundle. The clock is
// only used to compute the request `offset`; pass nil to use
// chart.SystemClock.
func NewHandler(c Charts, clock chart.Clock) *Handler {
	if clock == nil {
		clock = chart.SystemClock{}
	}
	return &Handler{c: c, clock: clock}
}

// Request is the shared request body for every chart endpoint.
// `Limit` defaults to 30 and is clamped to [1, 500]; `Offset` shifts
// the window back in time by that many spans (so offset=24 with
// span=hour returns the chart ending 24 hours ago).
type Request struct {
	Span   string `json:"span"`
	Limit  *int   `json:"limit"`
	Offset *int   `json:"offset"`
	UserID string `json:"userId"`
	Host   string `json:"host"`
}

// --- instance-wide endpoints -------------------------------------------------

// Notes handles POST /api/charts/notes.
func (h *Handler) Notes(c echo.Context) error {
	return h.serveInstance(c, h.c.Notes, false)
}

// Users handles POST /api/charts/users.
func (h *Handler) Users(c echo.Context) error {
	return h.serveInstance(c, h.c.Users, false)
}

// Drive handles POST /api/charts/drive.
func (h *Handler) Drive(c echo.Context) error {
	return h.serveInstance(c, h.c.Drive, false)
}

// Federation handles POST /api/charts/federation.
func (h *Handler) Federation(c echo.Context) error {
	return h.serveInstance(c, h.c.Federation, false)
}

// Instance handles POST /api/charts/instance. The chart is grouped by
// host so the request must supply a non-empty host.
func (h *Handler) Instance(c echo.Context) error {
	return h.serveInstance(c, h.c.Instance, true)
}

// ApRequest handles POST /api/charts/ap-request.
func (h *Handler) ApRequest(c echo.Context) error {
	return h.serveInstance(c, h.c.ApRequest, false)
}

// ActiveUsers handles POST /api/charts/active-users.
func (h *Handler) ActiveUsers(c echo.Context) error {
	return h.serveInstance(c, h.c.ActiveUsers, false)
}

// --- per-user endpoints ------------------------------------------------------

// UserNotes handles POST /api/charts/user/notes.
func (h *Handler) UserNotes(c echo.Context) error {
	return h.servePerUser(c, h.c.PerUserNotes)
}

// UserDrive handles POST /api/charts/user/drive.
func (h *Handler) UserDrive(c echo.Context) error {
	return h.servePerUser(c, h.c.PerUserDrive)
}

// UserFollowing handles POST /api/charts/user/following.
func (h *Handler) UserFollowing(c echo.Context) error {
	return h.servePerUser(c, h.c.PerUserFollowing)
}

// UserPv handles POST /api/charts/user/pv.
func (h *Handler) UserPv(c echo.Context) error {
	return h.servePerUser(c, h.c.PerUserPv)
}

// UserReactions handles POST /api/charts/user/reactions.
func (h *Handler) UserReactions(c echo.Context) error {
	return h.servePerUser(c, h.c.PerUserReaction)
}

// --- shared serving logic ----------------------------------------------------

// serveInstance executes an instance-wide chart query. requireHost is
// true for the host-grouped "instance" chart and false otherwise.
func (h *Handler) serveInstance(c echo.Context, ch *chart.Chart, requireHost bool) error {
	if ch == nil {
		return chartUnavailable(c)
	}
	var req Request
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	span, amount, cursor, ok := h.parseRequest(&req)
	if !ok {
		return invalidParam(c)
	}
	group := ""
	if requireHost {
		if req.Host == "" {
			return invalidParam(c)
		}
		group = req.Host
	}
	return h.respond(c, ch, span, amount, cursor, group)
}

// servePerUser executes a per-user chart query. The chart is always
// grouped so the request must supply a non-empty userId.
func (h *Handler) servePerUser(c echo.Context, ch *chart.Chart) error {
	if ch == nil {
		return chartUnavailable(c)
	}
	var req Request
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	span, amount, cursor, ok := h.parseRequest(&req)
	if !ok {
		return invalidParam(c)
	}
	if req.UserID == "" {
		return invalidParam(c)
	}
	return h.respond(c, ch, span, amount, cursor, req.UserID)
}

// respond runs the actual GetChart call and writes the JSON response.
func (h *Handler) respond(c echo.Context, ch *chart.Chart, span chart.Span, amount int, cursor *time.Time, group string) error {
	result, err := ch.GetChart(c.Request().Context(), span, amount, cursor, group)
	if err != nil {
		return internalError(c)
	}
	return c.JSON(http.StatusOK, chart.Unflatten(result))
}

// parseRequest validates the shared chart fields and computes a
// cursor for the requested offset. The cursor is nil when offset
// is zero so the engine simply uses its own clock.
//
// Validation rules (matching upstream):
//   - span must be "hour" or "day"
//   - limit defaults to 30 and is clamped to [1, 500]
//   - offset defaults to 0 and must be >= 0
func (h *Handler) parseRequest(req *Request) (chart.Span, int, *time.Time, bool) {
	span := chart.Span(req.Span)
	if span != chart.SpanHour && span != chart.SpanDay {
		return "", 0, nil, false
	}
	amount := 30
	if req.Limit != nil {
		if *req.Limit < 1 || *req.Limit > 500 {
			return "", 0, nil, false
		}
		amount = *req.Limit
	}
	offset := 0
	if req.Offset != nil {
		if *req.Offset < 0 {
			return "", 0, nil, false
		}
		offset = *req.Offset
	}
	var cursor *time.Time
	if offset > 0 {
		// cursor は希望ウィンドウの末尾を指す。offset 分だけ過去へずらす。
		now := h.clock.Now()
		var shifted time.Time
		if span == chart.SpanDay {
			shifted = now.AddDate(0, 0, -offset)
		} else {
			shifted = now.Add(-time.Duration(offset) * time.Hour)
		}
		cursor = &shifted
	}
	return span, amount, cursor, true
}

// --- error helpers -----------------------------------------------------------

func invalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.InvalidParam())
}

func internalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, apierr.InternalError())
}

func chartUnavailable(c echo.Context) error {
	return c.JSON(http.StatusServiceUnavailable, apierr.Error("CHART_UNAVAILABLE", "Chart not available.", "5e3a8d12-6c4f-4d70-b3bc-1f3dd0c7a37a"))
}
