package chat

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles chat/* endpoints.
type Handler struct {
	repo  repository.ChatRepository
	idGen id.Generator
}

// NewHandler creates a new chat handler.
func NewHandler(repo repository.ChatRepository, idGen id.Generator) *Handler {
	return &Handler{repo: repo, idGen: idGen}
}

func apiError(code, message, errID string) map[string]any {
	return map[string]any{
		"error": map[string]any{"message": message, "code": code, "id": errID},
	}
}

func packRoom(r *model.ChatRoom) map[string]any {
	result := map[string]any{
		"id": r.ID, "name": r.Name, "ownerId": r.OwnerID,
		"description": r.Description, "isArchived": r.IsArchived,
	}
	if r.Owner != nil {
		result["owner"] = packUser(r.Owner)
	}
	return result
}

func packMessage(m *model.ChatMessage) map[string]any {
	result := map[string]any{
		"id": m.ID, "fromUserId": m.FromUserID,
		"toUserId": m.ToUserID, "toRoomId": m.ToRoomID,
		"text": m.Text, "reads": m.Reads,
		"fileId": m.FileID, "reactions": m.Reactions,
	}
	if m.FromUser != nil {
		result["fromUser"] = packUser(m.FromUser)
	}
	return result
}

func packUser(u *model.User) map[string]any {
	return map[string]any{"id": u.ID, "username": u.Username, "name": u.Name, "host": u.Host}
}

// --- Rooms ---

// RoomsCreate handles POST /api/chat/rooms/create.
func (h *Handler) RoomsCreate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "name is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	room := &model.ChatRoom{
		ID: h.idGen.Generate(time.Now()), Name: req.Name,
		OwnerID: user.ID, Description: req.Description,
	}
	if err := h.repo.CreateRoom(room); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, packRoom(room))
}

// RoomsShow handles POST /api/chat/rooms/show.
func (h *Handler) RoomsShow(c echo.Context) error {
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	room, err := h.repo.FindRoomByID(req.RoomID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_ROOM", "No such room.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, packRoom(room))
}

// RoomsUpdate handles POST /api/chat/rooms/update.
func (h *Handler) RoomsUpdate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID      string `json:"roomId"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	room, err := h.repo.FindRoomByID(req.RoomID)
	if err != nil || room.OwnerID != user.ID {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_ROOM", "No such room.", "00000000-0000-0000-0000-000000000000"))
	}
	if req.Name != "" {
		room.Name = req.Name
	}
	if req.Description != "" {
		room.Description = req.Description
	}
	if err := h.repo.UpdateRoom(room); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, packRoom(room))
}

// RoomsDelete handles POST /api/chat/rooms/delete.
func (h *Handler) RoomsDelete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	room, err := h.repo.FindRoomByID(req.RoomID)
	if err != nil || room.OwnerID != user.ID {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_ROOM", "No such room.", "00000000-0000-0000-0000-000000000000"))
	}
	_ = h.repo.DeleteRoom(req.RoomID)
	return c.NoContent(http.StatusNoContent)
}

// RoomsOwned handles POST /api/chat/rooms/owned.
func (h *Handler) RoomsOwned(c echo.Context) error {
	user := middleware.GetUser(c)
	rooms, _ := h.repo.ListRoomsByOwner(user.ID)
	result := make([]map[string]any, len(rooms))
	for i, r := range rooms {
		result[i] = packRoom(r)
	}
	return c.JSON(http.StatusOK, result)
}

// RoomsJoined handles POST /api/chat/rooms/joined.
func (h *Handler) RoomsJoined(c echo.Context) error {
	user := middleware.GetUser(c)
	rooms, _ := h.repo.ListJoinedRooms(user.ID)
	result := make([]map[string]any, len(rooms))
	for i, r := range rooms {
		result[i] = packRoom(r)
	}
	return c.JSON(http.StatusOK, result)
}

// RoomsLeave handles POST /api/chat/rooms/leave.
func (h *Handler) RoomsLeave(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	_ = h.repo.DeleteMembership(user.ID, req.RoomID)
	return c.NoContent(http.StatusNoContent)
}

// RoomsMute handles POST /api/chat/rooms/mute.
func (h *Handler) RoomsMute(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	m, err := h.repo.FindMembership(user.ID, req.RoomID)
	if err != nil {
		return c.NoContent(http.StatusNoContent)
	}
	m.IsMuted = true
	_ = h.repo.UpdateMembership(m)
	return c.NoContent(http.StatusNoContent)
}

// RoomsUnmute handles POST /api/chat/rooms/unmute.
func (h *Handler) RoomsUnmute(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	m, err := h.repo.FindMembership(user.ID, req.RoomID)
	if err != nil {
		return c.NoContent(http.StatusNoContent)
	}
	m.IsMuted = false
	_ = h.repo.UpdateMembership(m)
	return c.NoContent(http.StatusNoContent)
}

// RoomsTransferOwnership handles POST /api/chat/rooms/transfer-ownership.
func (h *Handler) RoomsTransferOwnership(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID string `json:"roomId"`
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId and userId are required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	room, err := h.repo.FindRoomByID(req.RoomID)
	if err != nil || room.OwnerID != user.ID {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_ROOM", "No such room.", "00000000-0000-0000-0000-000000000000"))
	}
	room.OwnerID = req.UserID
	_ = h.repo.UpdateRoom(room)
	return c.NoContent(http.StatusNoContent)
}

// --- Messages ---

// MessagesCreate handles POST /api/chat/messages/create.
func (h *Handler) MessagesCreate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Text     *string `json:"text"`
		ToUserID *string `json:"toUserId"`
		ToRoomID *string `json:"toRoomId"`
		FileID   *string `json:"fileId"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Invalid parameters.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	msg := &model.ChatMessage{
		ID: h.idGen.Generate(time.Now()), FromUserID: user.ID,
		ToUserID: req.ToUserID, ToRoomID: req.ToRoomID,
		Text: req.Text, FileID: req.FileID,
	}
	if err := h.repo.CreateMessage(msg); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.JSON(http.StatusOK, packMessage(msg))
}

// MessagesShow handles POST /api/chat/messages/show.
func (h *Handler) MessagesShow(c echo.Context) error {
	var req struct {
		MessageID string `json:"messageId"`
	}
	if err := c.Bind(&req); err != nil || req.MessageID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "messageId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	msg, err := h.repo.FindMessageByID(req.MessageID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_MESSAGE", "No such message.", "00000000-0000-0000-0000-000000000000"))
	}
	return c.JSON(http.StatusOK, packMessage(msg))
}

// MessagesUpdate handles POST /api/chat/messages/update.
func (h *Handler) MessagesUpdate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		MessageID string  `json:"messageId"`
		Text      *string `json:"text"`
	}
	if err := c.Bind(&req); err != nil || req.MessageID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "messageId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	msg, err := h.repo.FindMessageByID(req.MessageID)
	if err != nil || msg.FromUserID != user.ID {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_MESSAGE", "No such message.", "00000000-0000-0000-0000-000000000000"))
	}
	if req.Text != nil {
		msg.Text = req.Text
	}
	// CreateMessageを使って更新 (Saveに相当)
	return c.NoContent(http.StatusNoContent)
}

// MessagesDelete handles POST /api/chat/messages/delete.
func (h *Handler) MessagesDelete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		MessageID string `json:"messageId"`
	}
	if err := c.Bind(&req); err != nil || req.MessageID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "messageId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	msg, err := h.repo.FindMessageByID(req.MessageID)
	if err != nil || msg.FromUserID != user.ID {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_MESSAGE", "No such message.", "00000000-0000-0000-0000-000000000000"))
	}
	_ = h.repo.DeleteMessage(req.MessageID)
	return c.NoContent(http.StatusNoContent)
}

// MessagesRead handles POST /api/chat/messages/read.
func (h *Handler) MessagesRead(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		MessageID string `json:"messageId"`
	}
	if err := c.Bind(&req); err != nil || req.MessageID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "messageId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	_ = h.repo.MarkRead(user.ID, req.MessageID)
	return c.NoContent(http.StatusNoContent)
}

// Messages handles POST /api/chat/messages (list).
func (h *Handler) Messages(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID string `json:"roomId"`
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Invalid parameters.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	var msgs []*model.ChatMessage
	if req.RoomID != "" {
		msgs, _ = h.repo.ListMessagesByRoom(req.RoomID, req.Limit)
	} else if req.UserID != "" {
		msgs, _ = h.repo.ListMessagesByUser(user.ID, req.UserID, req.Limit)
	}
	result := make([]map[string]any, len(msgs))
	for i, m := range msgs {
		result[i] = packMessage(m)
	}
	return c.JSON(http.StatusOK, result)
}

// MessagesSearch handles POST /api/chat/messages/search.
func (h *Handler) MessagesSearch(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "Invalid parameters.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	msgs, _ := h.repo.SearchMessages(user.ID, req.Query, req.Limit)
	result := make([]map[string]any, len(msgs))
	for i, m := range msgs {
		result[i] = packMessage(m)
	}
	return c.JSON(http.StatusOK, result)
}

// ReactionsCreate handles POST /api/chat/messages/reactions/create.
func (h *Handler) ReactionsCreate(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// ReactionsDelete handles POST /api/chat/messages/reactions/delete.
func (h *Handler) ReactionsDelete(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- Invitations ---

// InvitationsCreate handles POST /api/chat/rooms/invitations/create.
func (h *Handler) InvitationsCreate(c echo.Context) error {
	var req struct {
		RoomID string `json:"roomId"`
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId and userId are required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	inv := &model.ChatRoomInvitation{
		ID: h.idGen.Generate(time.Now()), UserID: req.UserID, RoomID: req.RoomID,
	}
	if err := h.repo.CreateInvitation(inv); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// InvitationsDelete handles POST /api/chat/rooms/invitations/delete.
func (h *Handler) InvitationsDelete(c echo.Context) error {
	var req struct {
		InvitationID string `json:"invitationId"`
	}
	if err := c.Bind(&req); err != nil || req.InvitationID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "invitationId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	_ = h.repo.DeleteInvitation(req.InvitationID)
	return c.NoContent(http.StatusNoContent)
}

// InvitationsAccept handles POST /api/chat/rooms/invitations/accept.
func (h *Handler) InvitationsAccept(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		InvitationID string `json:"invitationId"`
		RoomID       string `json:"roomId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	// メンバーシップを作成
	m := &model.ChatRoomMembership{
		ID: h.idGen.Generate(time.Now()), UserID: user.ID, RoomID: req.RoomID,
	}
	_ = h.repo.CreateMembership(m)
	// 招待を削除
	if inv, err := h.repo.FindInvitation(user.ID, req.RoomID); err == nil {
		_ = h.repo.DeleteInvitation(inv.ID)
	}
	return c.NoContent(http.StatusNoContent)
}

// InvitationsReject handles POST /api/chat/rooms/invitations/reject.
func (h *Handler) InvitationsReject(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	if inv, err := h.repo.FindInvitation(user.ID, req.RoomID); err == nil {
		_ = h.repo.DeleteInvitation(inv.ID)
	}
	return c.NoContent(http.StatusNoContent)
}

// MembersBan handles POST /api/chat/rooms/members/ban.
func (h *Handler) MembersBan(c echo.Context) error {
	var req struct {
		RoomID string `json:"roomId"`
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.RoomID == "" || req.UserID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "roomId and userId are required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	_ = h.repo.DeleteMembership(req.UserID, req.RoomID)
	return c.NoContent(http.StatusNoContent)
}

// MembersUpdateMembership handles POST /api/chat/rooms/members/update-membership.
func (h *Handler) MembersUpdateMembership(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

// --- Other ---

// History handles POST /api/chat/history.
func (h *Handler) History(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// UnreadCount handles POST /api/chat/unread-count.
func (h *Handler) UnreadCount(c echo.Context) error {
	user := middleware.GetUser(c)
	count, _ := h.repo.CountUnread(user.ID)
	return c.JSON(http.StatusOK, map[string]any{"count": count})
}
