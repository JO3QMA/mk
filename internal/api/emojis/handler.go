package emojis

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/repository"
)

// Handler handles emoji-related API endpoints.
type Handler struct {
	emojiRepo repository.EmojiRepository
}

// NewHandler creates a new emojis Handler.
func NewHandler(emojiRepo repository.EmojiRepository) *Handler {
	return &Handler{emojiRepo: emojiRepo}
}

// Emojis returns local custom emojis.
// POST /api/emojis
func (h *Handler) Emojis(c echo.Context) error {
	emojis, err := h.emojiRepo.ListLocal()
	if err != nil {
		// エラー時は空配列で返却 (best-effort)
		return c.JSON(http.StatusOK, map[string]any{"emojis": []any{}})
	}

	result := make([]map[string]any, 0, len(emojis))
	for _, e := range emojis {
		item := map[string]any{
			"name":        e.Name,
			"category":    e.Category,
			"aliases":     e.Aliases,
			"url":         e.PublicURL,
			"localOnly":   e.LocalOnly,
			"isSensitive": e.IsSensitive,
		}
		result = append(result, item)
	}

	return c.JSON(http.StatusOK, map[string]any{"emojis": result})
}

// Emoji returns a single custom emoji by name.
// POST /api/emoji
func (h *Handler) Emoji(c echo.Context) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "Invalid param.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}
	e, err := h.emojiRepo.FindByNameAndHost(req.Name, nil)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"message": "No such emoji.",
				"code":    "NO_SUCH_EMOJI",
				"id":      "14141e4b-dea8-41f0-9ba1-1721a6b5b92c",
			},
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id":          e.ID,
		"name":        e.Name,
		"category":    e.Category,
		"aliases":     e.Aliases,
		"url":         e.PublicURL,
		"localOnly":   e.LocalOnly,
		"isSensitive": e.IsSensitive,
		"host":        e.Host,
	})
}
