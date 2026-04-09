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
