package users

import (
	"net/http"

	"github.com/labstack/echo/v4"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	"github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// ChartHook is invoked after a user/show request resolves so the
// chart subsystem can record the profile pageview and the
// activeUsers Read event. パッケージ間の循環依存を避けるため
// interface で受け取る (実装は core/chart/charthook)。
type ChartHook interface {
	OnUserShow(ownerID, viewerID, visitorKey string)
}

// Handler handles user-related API endpoints.
type Handler struct {
	userService          *user.Service
	followingService     *corefollowing.Service
	noteRepo             repository.NoteRepository
	idGen                id.Generator
	chartHook            ChartHook
	abuseRepo            repository.AbuseReportRepository
	followingRepo        repository.FollowingRepository
	memoRepo             repository.UserMemoRepository
	blockingRepo         repository.BlockingRepository
	mutingRepo           repository.MutingRepository
	renoteMutingRepo     repository.RenoteMutingRepository
	followRequestRepo    repository.FollowRequestRepository
	instanceRepo         repository.InstanceRepository
	userListFavoriteRepo UserListFavoriteRepository
}

// SetMemoRepo attaches a UserMemoRepository for users/update-memo.
func (h *Handler) SetMemoRepo(r repository.UserMemoRepository) {
	h.memoRepo = r
}

// SetFollowingRepo attaches a FollowingRepository for follow relation queries.
func (h *Handler) SetFollowingRepo(r repository.FollowingRepository) {
	h.followingRepo = r
}

// SetBlockingRepo attaches a BlockingRepository for block status queries.
func (h *Handler) SetBlockingRepo(r repository.BlockingRepository) {
	h.blockingRepo = r
}

// SetMutingRepo attaches a MutingRepository for mute status queries.
func (h *Handler) SetMutingRepo(r repository.MutingRepository) {
	h.mutingRepo = r
}

// SetRenoteMutingRepo attaches a RenoteMutingRepository for renote mute status queries.
func (h *Handler) SetRenoteMutingRepo(r repository.RenoteMutingRepository) {
	h.renoteMutingRepo = r
}

// SetFollowRequestRepo attaches a FollowRequestRepository for pending request queries.
func (h *Handler) SetFollowRequestRepo(r repository.FollowRequestRepository) {
	h.followRequestRepo = r
}

// SetInstanceRepo attaches an InstanceRepository for remote user instance info.
func (h *Handler) SetInstanceRepo(r repository.InstanceRepository) {
	h.instanceRepo = r
}

// NewHandler creates a new users Handler.
// followingService, noteRepo, idGen are optional for the bare /show endpoint.
func NewHandler(
	userService *user.Service,
	followingService *corefollowing.Service,
	noteRepo repository.NoteRepository,
	idGen id.Generator,
) *Handler {
	return &Handler{
		userService:      userService,
		followingService: followingService,
		noteRepo:         noteRepo,
		idGen:            idGen,
	}
}

// SetChartHook attaches a ChartHook invoked after Show successfully
// resolves a profile.
func (h *Handler) SetChartHook(c ChartHook) {
	h.chartHook = c
}

// ShowRequest is the request body for users/show.
type ShowRequest struct {
	UserID   *string `json:"userId"`
	Username *string `json:"username"`
	Host     *string `json:"host"`
}

// Show handles POST /api/users/show.
func (h *Handler) Show(c echo.Context) error {
	var req ShowRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}

	if req.UserID == nil && req.Username == nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "userId or username is required.",
				"code":    "INVALID_PARAM",
				"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
			},
		})
	}

	var (
		bundle *user.UserWithProfile
		err    error
	)
	if req.UserID != nil {
		bundle, err = h.userService.ShowByID(*req.UserID)
	} else {
		bundle, err = h.userService.ShowByUsername(*req.Username, req.Host)
	}

	if err != nil {
		// Service.ShowByID/ShowByUsernameはErrUserNotFoundのみ返す
		return noSuchUser(c)
	}

	// チャート集計はベストエフォート。匿名訪問者は visitor key として
	// リモートホスト名を使う (簡易実装; 認証済みなら viewer id を渡す)。
	if h.chartHook != nil {
		viewerID := ""
		visitorKey := ""
		if viewer := middleware.GetUser(c); viewer != nil {
			viewerID = viewer.ID
		} else {
			visitorKey = c.Request().RemoteAddr
		}
		h.chartHook.OnUserShow(bundle.User.ID, viewerID, visitorKey)
	}

	detailed := entity.PackUserDetailed(bundle.User, bundle.Profile, h.idGen)

	// リモートユーザーの場合、Instance情報を付与
	if bundle.User.Host != nil && h.instanceRepo != nil {
		if inst, err := h.instanceRepo.FindByHost(*bundle.User.Host); err == nil {
			detailed.Instance = &entity.InstanceLite{
				Name:            inst.Name,
				SoftwareName:    inst.SoftwareName,
				SoftwareVersion: inst.SoftwareVersion,
				IconURL:         inst.IconURL,
				FaviconURL:      inst.FaviconURL,
				ThemeColor:      inst.ThemeColor,
			}
		}
	}

	// viewerがログインしている場合、viewer依存フィールドを追加
	if viewer := middleware.GetUser(c); viewer != nil && viewer.ID != bundle.User.ID {
		if h.followingRepo != nil {
			isFollowing, _ := h.followingRepo.Exists(viewer.ID, bundle.User.ID)
			isFollowed, _ := h.followingRepo.Exists(bundle.User.ID, viewer.ID)
			detailed.IsFollowing = &isFollowing
			detailed.IsFollowed = &isFollowed
			// notify/withReplies はフォロー関係レコードから取得
			if f, err := h.followingRepo.FindByPair(viewer.ID, bundle.User.ID); err == nil {
				detailed.Notify = f.Notify
				wr := f.WithReplies
				detailed.WithReplies = &wr
			}
		}
		if h.blockingRepo != nil {
			isBlocking, _ := h.blockingRepo.Exists(viewer.ID, bundle.User.ID)
			isBlocked, _ := h.blockingRepo.Exists(bundle.User.ID, viewer.ID)
			detailed.IsBlocking = &isBlocking
			detailed.IsBlocked = &isBlocked
		}
		if h.mutingRepo != nil {
			isMuted, _ := h.mutingRepo.Exists(viewer.ID, bundle.User.ID)
			detailed.IsMuted = &isMuted
		}
		if h.renoteMutingRepo != nil {
			isRenoteMuted, _ := h.renoteMutingRepo.Exists(viewer.ID, bundle.User.ID)
			detailed.IsRenoteMuted = &isRenoteMuted
		}
		if h.followRequestRepo != nil {
			hasPendingFrom, _ := h.followRequestRepo.Exists(viewer.ID, bundle.User.ID)
			hasPendingTo, _ := h.followRequestRepo.Exists(bundle.User.ID, viewer.ID)
			detailed.HasPendingFollowRequestFromYou = &hasPendingFrom
			detailed.HasPendingFollowRequestToYou = &hasPendingTo
		}
		if h.memoRepo != nil {
			if memo, err := h.memoRepo.FindByPair(viewer.ID, bundle.User.ID); err == nil {
				detailed.Memo = &memo.Memo
			}
		}
	}

	return c.JSON(http.StatusOK, detailed)
}

// SearchRequest is the request body for users/search.
type SearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Search handles POST /api/users/search.
func (h *Handler) Search(c echo.Context) error {
	var req SearchRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	users, err := h.userService.Search(req.Query, req.Limit, req.Offset)
	if err != nil {
		return internalError(c)
	}

	out := make([]entity.UserDetailed, 0, len(users))
	for _, u := range users {
		profile := h.userService.GetProfile(u.ID)
		out = append(out, entity.PackUserDetailed(u, profile))
	}
	return c.JSON(http.StatusOK, out)
}

// NotesRequest is the request body for users/notes.
type NotesRequest struct {
	UserID  string `json:"userId"`
	Limit   int    `json:"limit"`
	SinceID string `json:"sinceId"`
	UntilID string `json:"untilId"`
}

// Notes handles POST /api/users/notes.
func (h *Handler) Notes(c echo.Context) error {
	var req NotesRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	if _, err := h.userService.ShowByID(req.UserID); err != nil {
		return noSuchUser(c)
	}

	notes, err := h.noteRepo.ListByUserID(req.UserID, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		return internalError(c)
	}

	out := make([]entity.NoteEntity, 0, len(notes))
	for _, n := range notes {
		out = append(out, entity.PackNote(n, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// FollowersRequest is the request body for users/followers and users/following.
type FollowersRequest struct {
	UserID  string `json:"userId"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	SinceID string `json:"sinceId"`
	UntilID string `json:"untilId"`
}

// Followers handles POST /api/users/followers.
func (h *Handler) Followers(c echo.Context) error {
	return h.listRelations(c, true)
}

// Following handles POST /api/users/following.
func (h *Handler) Following(c echo.Context) error {
	return h.listRelations(c, false)
}

func (h *Handler) listRelations(c echo.Context, followers bool) error {
	var req FollowersRequest
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	if _, err := h.userService.ShowByID(req.UserID); err != nil {
		return noSuchUser(c)
	}

	var (
		rows []relationItem
		err  error
	)
	if followers {
		rows, err = h.collectFollowers(req)
	} else {
		rows, err = h.collectFollowing(req)
	}
	if err != nil {
		return internalError(c)
	}

	return c.JSON(http.StatusOK, rows)
}

// relationItem represents a single entry in followers/following lists.
type relationItem struct {
	ID         string               `json:"id"`
	FollowerID string               `json:"followerId"`
	FolloweeID string               `json:"followeeId"`
	Follower   *entity.UserDetailed `json:"follower,omitempty"`
	Followee   *entity.UserDetailed `json:"followee,omitempty"`
}

func (h *Handler) collectFollowers(req FollowersRequest) ([]relationItem, error) {
	rows, err := h.followingService.ListReceivedFollowing(req.UserID, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	out := make([]relationItem, 0, len(rows))
	for _, f := range rows {
		// カーソルベースページネーション
		if req.SinceID != "" && f.ID <= req.SinceID {
			continue
		}
		if req.UntilID != "" && f.ID >= req.UntilID {
			continue
		}
		item := relationItem{ID: f.ID, FollowerID: f.FollowerID, FolloweeID: f.FolloweeID}
		if b, err := h.userService.ShowByID(f.FollowerID); err == nil {
			d := entity.PackUserDetailed(b.User, b.Profile)
			item.Follower = &d
		}
		out = append(out, item)
	}
	return out, nil
}

func (h *Handler) collectFollowing(req FollowersRequest) ([]relationItem, error) {
	rows, err := h.followingService.ListSentFollowing(req.UserID, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	out := make([]relationItem, 0, len(rows))
	for _, f := range rows {
		if req.SinceID != "" && f.ID <= req.SinceID {
			continue
		}
		if req.UntilID != "" && f.ID >= req.UntilID {
			continue
		}
		item := relationItem{ID: f.ID, FollowerID: f.FollowerID, FolloweeID: f.FolloweeID}
		if b, err := h.userService.ShowByID(f.FolloweeID); err == nil {
			d := entity.PackUserDetailed(b.User, b.Profile)
			item.Followee = &d
		}
		out = append(out, item)
	}
	return out, nil
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

func invalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]any{
		"error": map[string]any{
			"message": "Invalid param.",
			"code":    "INVALID_PARAM",
			"id":      "3d81ceae-475f-4600-b2a8-2bc116157532",
		},
	})
}

func noSuchUser(c echo.Context) error {
	return c.JSON(http.StatusNotFound, map[string]any{
		"error": map[string]any{
			"message": "No such user.",
			"code":    "NO_SUCH_USER",
			"id":      "4362f8dc-731f-4ad8-a694-be5a88922a24",
		},
	})
}
