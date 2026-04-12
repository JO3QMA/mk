// Package url provides the /api/url endpoint for URL preview.
package url

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/urlpreview"
)

// Handler handles the URL preview endpoint.
type Handler struct {
	fetcher *urlpreview.Fetcher
}

// NewHandler creates a new URL preview handler.
func NewHandler(fetcher *urlpreview.Fetcher) *Handler {
	return &Handler{fetcher: fetcher}
}

// Preview handles GET /url (matches Misskey's endpoint path).
// フロントエンドがリンクプレビューカード表示のために呼ぶ。
func (h *Handler) Preview(c echo.Context) error {
	rawURL := c.QueryParam("url")
	if rawURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "url is required.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	result, err := h.fetcher.Fetch(c.Request().Context(), rawURL)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]any{
			"title":       nil,
			"description": nil,
			"thumbnail":   nil,
			"icon":        nil,
			"sitename":    nil,
			"url":         rawURL,
			"player":      map[string]any{"url": nil, "width": nil, "height": nil, "allow": []string{}},
			"sensitive":   false,
			"activityPub": nil,
		})
	}

	return c.JSON(http.StatusOK, result)
}
