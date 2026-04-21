// Package channels provides /api/channels/* endpoints.
package channels

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	corechannel "github.com/shiroha-a/mk/internal/core/channel"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles channel-related API endpoints.
type Handler struct {
	svc          *corechannel.Service
	idGen        id.Generator
	favoriteRepo ChannelFavoriteRepository
	mutingRepo   ChannelMutingRepository
	instanceRepo repository.InstanceRepository
	emojiRepo    repository.EmojiRepository
}

// SetInstanceRepo attaches an InstanceRepository so channel timeline responses
// populate UserLite.Instance for remote users (#277).
func (h *Handler) SetInstanceRepo(r repository.InstanceRepository) {
	h.instanceRepo = r
}

func (h *Handler) instanceLookup() entity.InstanceLookup {
	if h.instanceRepo == nil {
		return nil
	}
	return h.instanceRepo
}

// SetEmojiRepo attaches an EmojiRepository so custom emoji shortcodes in
// note text and user displayNames get resolved to URLs.
func (h *Handler) SetEmojiRepo(r repository.EmojiRepository) {
	h.emojiRepo = r
}

func (h *Handler) emojiLookup() entity.EmojiLookup {
	if h.emojiRepo == nil {
		return nil
	}
	return h.emojiRepo
}

// ChannelFavoriteRepository is the interface for channel favorite operations.
type ChannelFavoriteRepository interface {
	Create(fav *model.ChannelFavorite) error
	Delete(userID, channelID string) error
	ListByUser(userID string) ([]*model.ChannelFavorite, error)
	Exists(userID, channelID string) (bool, error)
}

// ChannelMutingRepository is the interface for channel muting operations.
type ChannelMutingRepository interface {
	Create(mut *model.ChannelMuting) error
	Delete(userID, channelID string) error
	ListByUser(userID string) ([]*model.ChannelMuting, error)
	Exists(userID, channelID string) (bool, error)
}

// NewHandler creates a new channels Handler.
func NewHandler(svc *corechannel.Service, idGen id.Generator) *Handler {
	return &Handler{svc: svc, idGen: idGen}
}

// CreateRequest is the request body for channels/create.
type CreateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Color       string  `json:"color"`
	IsSensitive bool    `json:"isSensitive"`
}

// Create handles POST /api/channels/create.
func (h *Handler) Create(c echo.Context) error {
	user := middleware.GetUser(c)
	var req CreateRequest
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return apierr.JSONInvalidParam(c)
	}
	ch, err := h.svc.Create(corechannel.CreateInput{
		OwnerID:     user.ID,
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		IsSensitive: req.IsSensitive,
	})
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, channelToMap(ch))
}

// ShowRequest is the request body for channels/show.
type ShowRequest struct {
	ChannelID string `json:"channelId"`
}

// Show handles POST /api/channels/show.
func (h *Handler) Show(c echo.Context) error {
	var req ShowRequest
	if err := c.Bind(&req); err != nil || req.ChannelID == "" {
		return apierr.JSONInvalidParam(c)
	}
	ch, err := h.svc.Show(req.ChannelID)
	if err != nil {
		return notFound(c)
	}
	return c.JSON(http.StatusOK, channelToMap(ch))
}

// UpdateRequest is the request body for channels/update.
type UpdateRequest struct {
	ChannelID   string  `json:"channelId"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	IsArchived  *bool   `json:"isArchived"`
	IsSensitive *bool   `json:"isSensitive"`
}

// Update handles POST /api/channels/update.
func (h *Handler) Update(c echo.Context) error {
	user := middleware.GetUser(c)
	var req UpdateRequest
	if err := c.Bind(&req); err != nil || req.ChannelID == "" {
		return apierr.JSONInvalidParam(c)
	}
	in := corechannel.UpdateInput{
		Name:        req.Name,
		Color:       req.Color,
		IsArchived:  req.IsArchived,
		IsSensitive: req.IsSensitive,
	}
	if req.Description != nil {
		desc := req.Description
		in.Description = &desc
	}
	ch, err := h.svc.Update(user.ID, req.ChannelID, in)
	if err != nil {
		switch {
		case errors.Is(err, corechannel.ErrChannelNotFound):
			return notFound(c)
		case errors.Is(err, corechannel.ErrAccessDenied):
			return apierr.JSONAccessDenied(c)
		case errors.Is(err, corechannel.ErrChannelNameRequired):
			return apierr.JSONInvalidParam(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, channelToMap(ch))
}

// FollowRequest / UnfollowRequest reuse the channelId field.
type FollowRequest struct {
	ChannelID string `json:"channelId"`
}

// Follow handles POST /api/channels/follow.
func (h *Handler) Follow(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FollowRequest
	if err := c.Bind(&req); err != nil || req.ChannelID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Follow(user.ID, req.ChannelID); err != nil {
		switch {
		case errors.Is(err, corechannel.ErrChannelNotFound):
			return notFound(c)
		case errors.Is(err, corechannel.ErrAlreadyFollowing):
			return alreadyFollowing(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// Unfollow handles POST /api/channels/unfollow.
func (h *Handler) Unfollow(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FollowRequest
	if err := c.Bind(&req); err != nil || req.ChannelID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if err := h.svc.Unfollow(user.ID, req.ChannelID); err != nil {
		if errors.Is(err, corechannel.ErrNotFollowing) {
			return notFollowing(c)
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// PaginatedListRequest is shared by followed / owned / featured / search /
// timeline endpoints.
type PaginatedListRequest struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Followed handles POST /api/channels/followed.
func (h *Handler) Followed(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PaginatedListRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.ListFollowed(user.ID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, channelsToList(rows))
}

// Owned handles POST /api/channels/owned.
func (h *Handler) Owned(c echo.Context) error {
	user := middleware.GetUser(c)
	var req PaginatedListRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.ListOwned(user.ID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, channelsToList(rows))
}

// Featured handles POST /api/channels/featured.
func (h *Handler) Featured(c echo.Context) error {
	var req PaginatedListRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.ListFeatured(req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, channelsToList(rows))
}

// SearchRequest carries the query string for channels/search.
type SearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Search handles POST /api/channels/search.
func (h *Handler) Search(c echo.Context) error {
	var req SearchRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	rows, err := h.svc.Search(req.Query, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, channelsToList(rows))
}

// TimelineRequest is the request body for channels/timeline.
type TimelineRequest struct {
	ChannelID string `json:"channelId"`
	UntilID   string `json:"untilId"`
	SinceID   string `json:"sinceId"`
	Limit     int    `json:"limit"`
}

// Timeline handles POST /api/channels/timeline.
func (h *Handler) Timeline(c echo.Context) error {
	var req TimelineRequest
	if err := c.Bind(&req); err != nil || req.ChannelID == "" {
		return apierr.JSONInvalidParam(c)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	notes, err := h.svc.Timeline(req.ChannelID, req.UntilID, req.SinceID, limit)
	if err != nil {
		if errors.Is(err, corechannel.ErrChannelNotFound) {
			return notFound(c)
		}
		return apierr.JSONInternalError(c)
	}
	entities := entity.PackNotes(notes, h.idGen, h.instanceLookup(), h.emojiLookup())
	out := make([]any, 0, len(entities))
	for _, pn := range entities {
		out = append(out, pn)
	}
	return c.JSON(http.StatusOK, out)
}

func channelToMap(ch *model.Channel) map[string]any {
	return map[string]any{
		"id":                    ch.ID,
		"name":                  ch.Name,
		"description":           ch.Description,
		"userId":                ch.UserID,
		"bannerId":              ch.BannerID,
		"pinnedNoteIds":         ch.PinnedNoteIDs,
		"color":                 ch.Color,
		"isArchived":            ch.IsArchived,
		"notesCount":            ch.NotesCount,
		"usersCount":            ch.UsersCount,
		"isSensitive":           ch.IsSensitive,
		"allowRenoteToExternal": ch.AllowRenoteToExternal,
		"lastNotedAt":           ch.LastNotedAt,
	}
}

func channelsToList(rows []*model.Channel) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, ch := range rows {
		out = append(out, channelToMap(ch))
	}
	return out
}

func notFound(c echo.Context) error {
	return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_CHANNEL", "No such channel.", "8ee5d9d4-9cb0-4f40-bba4-aaa31a3b48b9"))
}

func alreadyFollowing(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_FOLLOWING", "You are already following that channel.", "35dbf050-f1cc-4da8-9322-87bb0acce8c7"))
}

func notFollowing(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("NOT_FOLLOWING", "You are not following that channel.", "1f87a25b-7c72-4ed1-b3c3-bcaf57636a64"))
}
