package notes

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/poll"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// PollVoteRequest is the request body for notes/polls/vote.
type PollVoteRequest struct {
	NoteID string `json:"noteId"`
	Choice *int   `json:"choice"`
}

// PollsVote handles POST /api/notes/polls/vote.
func (h *Handler) PollsVote(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PollVoteRequest
	if err := c.Bind(&req); err != nil || req.NoteID == "" || req.Choice == nil {
		return invalidParam(c)
	}

	if err := h.pollService.Vote(user, req.NoteID, *req.Choice); err != nil {
		switch {
		case errors.Is(err, poll.ErrNoteNotFound), errors.Is(err, poll.ErrNoPoll):
			return noSuchNote(c)
		case errors.Is(err, poll.ErrNoteNotVisible):
			return c.JSON(http.StatusForbidden, map[string]any{
				"error": map[string]any{
					"message": "You can not see this note.",
					"code":    "ACCESS_DENIED",
					"id":      "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9",
				},
			})
		case errors.Is(err, poll.ErrInvalidChoice):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "Invalid choice.",
					"code":    "INVALID_CHOICE",
					"id":      "e0cc9a04-f2e8-41e4-a5f1-4127293260cc",
				},
			})
		case errors.Is(err, poll.ErrPollExpired):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "The poll is already expired.",
					"code":    "ALREADY_EXPIRED",
					"id":      "1022a357-b085-4054-9083-8f8de358337e",
				},
			})
		case errors.Is(err, poll.ErrAlreadyVoted):
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "You have already voted.",
					"code":    "ALREADY_VOTED",
					"id":      "0963fc77-efac-419b-9424-b391608dc6d8",
				},
			})
		}
		return internalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}
