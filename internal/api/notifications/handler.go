// Package notifications provides /api/notifications/* endpoints.
package notifications

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/notification"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles notifications-related API endpoints.
type Handler struct {
	svc           *notification.Service
	idGen         id.Generator
	userRepo      repository.UserRepository
	noteRepo      repository.NoteRepository
	followReqRepo repository.FollowRequestRepository
	instanceRepo  repository.InstanceRepository
	emojiRepo     repository.EmojiRepository
}

// NewHandler creates a new notifications Handler.
func NewHandler(svc *notification.Service, idGen id.Generator) *Handler {
	return &Handler{svc: svc, idGen: idGen}
}

// SetRepos attaches repositories for resolving user/note objects in notifications.
func (h *Handler) SetRepos(userRepo repository.UserRepository, noteRepo repository.NoteRepository) {
	h.userRepo = userRepo
	h.noteRepo = noteRepo
}

// SetInstanceRepo attaches an InstanceRepository so user.instance (used by
// frontend InstanceTicker for theme color / software name) is populated on
// notification payloads (#415 follow-up).
func (h *Handler) SetInstanceRepo(r repository.InstanceRepository) {
	h.instanceRepo = r
}

// SetEmojiRepo attaches an EmojiRepository so custom emoji shortcodes in
// user.name / note.text resolve to URLs in notification payloads.
func (h *Handler) SetEmojiRepo(r repository.EmojiRepository) {
	h.emojiRepo = r
}

// instanceLookup returns an InstanceLookup adapter (nil-safe when the
// repository hasn't been wired).
func (h *Handler) instanceLookup() entity.InstanceLookup {
	if h.instanceRepo == nil {
		return nil
	}
	return h.instanceRepo
}

// emojiLookup returns an EmojiLookup adapter (nil-safe when the repository
// hasn't been wired).
func (h *Handler) emojiLookup() entity.EmojiLookup {
	if h.emojiRepo == nil {
		return nil
	}
	return h.emojiRepo
}

// SetFollowRequestRepo attaches a FollowRequestRepository. 受信済み
// receiveFollowRequest 通知について対応する follow_request 行が無ければ
// (承認/拒否により消えた) 通知一覧から除外する用途 (本家
// NotificationEntityService と同じ挙動)。
func (h *Handler) SetFollowRequestRepo(r repository.FollowRequestRepository) {
	h.followReqRepo = r
}

// ListRequest is the request body for notifications.
type ListRequest struct {
	Limit        int      `json:"limit"`
	IncludeTypes []string `json:"includeTypes"`
	ExcludeTypes []string `json:"excludeTypes"`
	SinceID      string   `json:"sinceId"`
	UntilID      string   `json:"untilId"`
	// MarkAsRead controls whether fetching the notification list implicitly
	// marks all notifications as read. nil / true means "mark as read"
	// (default), explicit false leaves the read state untouched.
	// 本家 i/notifications の暗黙引数。Misskey フロントが
	// `notifications/mark-all-as-read` を明示呼び出しせず、通知一覧 fetch の
	// 副作用で badge が消えることを期待しているため (#420)。
	MarkAsRead *bool `json:"markAsRead"`
}

// Show handles POST /api/i/notifications - returns the authenticated user's
// notification timeline ordered newest first.
func (h *Handler) Show(c echo.Context) error {
	user := middleware.GetUser(c)
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}

	rows, err := h.svc.List(c.Request().Context(), user.ID, req.Limit)
	if err != nil {
		return apierr.JSONInternalError(c)
	}

	// includeTypes / excludeTypes フィルタ
	includeSet := make(map[string]bool, len(req.IncludeTypes))
	for _, t := range req.IncludeTypes {
		includeSet[t] = true
	}
	excludeSet := make(map[string]bool, len(req.ExcludeTypes))
	for _, t := range req.ExcludeTypes {
		excludeSet[t] = true
	}

	out := make([]map[string]any, 0, len(rows))
	for _, n := range rows {
		// カーソルベースページネーション
		if req.SinceID != "" && n.ID <= req.SinceID {
			continue
		}
		if req.UntilID != "" && n.ID >= req.UntilID {
			continue
		}
		// タイプフィルタ
		if len(includeSet) > 0 && !includeSet[string(n.Type)] {
			continue
		}
		if excludeSet[string(n.Type)] {
			continue
		}
		// 既に解決された follow request (accept / reject で消えた) に対応する
		// receiveFollowRequest 通知はユーザー視点で既に古く「残り続ける」ので
		// 除外する (#349 コメント対応、本家 NotificationEntityService 互換)。
		if n.Type == notification.TypeReceiveFollowReq && h.followReqRepo != nil && n.NotifierID != "" {
			if exists, err := h.followReqRepo.Exists(n.NotifierID, user.ID); err == nil && !exists {
				continue
			}
		}
		// WebSocket 経由の packing と一貫性を保つため entity.PackNotification
		// にdelegateする(Devin #315 指摘)。user/note 取得は handler 側で
		// repo が wire されていれば best-effort。外側の認証ユーザー(`user`
		// at line 48)をshadowしないよう明示的に notifier 名で受ける。
		var notifier *model.User
		if h.userRepo != nil && n.NotifierID != "" {
			if u, err := h.userRepo.FindByID(n.NotifierID); err == nil {
				notifier = u
			}
		}
		var note *model.Note
		if h.noteRepo != nil && n.NoteID != "" {
			if nn, err := h.noteRepo.FindByIDWithUser(n.NoteID); err == nil {
				note = nn
			}
		}
		out = append(out, entity.PackNotification(n, notifier, note, h.idGen, h.instanceLookup(), h.emojiLookup()))
	}
	// 本家 i/notifications と互換: markAsRead 未指定または true なら通知一覧
	// 取得の副作用で全通知を既読化し、main stream に readAllNotifications を
	// publish する。これが無いとフロントエンドが /my/notifications を開いても
	// バッジカウントが残り続ける (#420)。
	if req.MarkAsRead == nil || *req.MarkAsRead {
		if err := h.svc.MarkAllAsRead(c.Request().Context(), user.ID); err != nil {
			// 既読化失敗は通知一覧の取得結果には影響しないので 200 のまま
			// 返し、ログだけ残す。
			slog.Warn("notifications: implicit mark-all-as-read failed",
				"userId", user.ID, "err", err)
		}
	}
	return c.JSON(http.StatusOK, out)
}

// MarkAllAsRead handles POST /api/notifications/mark-all-as-read.
func (h *Handler) MarkAllAsRead(c echo.Context) error {
	user := middleware.GetUser(c)
	if err := h.svc.MarkAllAsRead(c.Request().Context(), user.ID); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// Create handles POST /api/notifications/create.
// アプリ通知の作成 (簡易版)。
func (h *Handler) Create(c echo.Context) error {
	_ = middleware.GetUser(c)
	return c.NoContent(http.StatusNoContent)
}

// Flush handles POST /api/notifications/flush.
func (h *Handler) Flush(c echo.Context) error {
	user := middleware.GetUser(c)
	if h.svc != nil {
		if err := h.svc.Flush(c.Request().Context(), user.ID); err != nil {
			return apierr.JSONInternalError(c)
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// TestNotification handles POST /api/notifications/test-notification.
func (h *Handler) TestNotification(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}
