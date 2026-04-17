package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/gorm"
)

// UserListFavoriteRepository is the interface for user list favorite operations.
type UserListFavoriteRepository interface {
	Create(fav *model.UserListFavorite) error
	Delete(userID, listID string) error
	ListByUser(userID string) ([]*model.UserListFavorite, error)
	Exists(userID, listID string) (bool, error)
}

// SetUserListFavoriteRepo attaches a UserListFavoriteRepository.
func (h *Handler) SetUserListFavoriteRepo(r UserListFavoriteRepository) {
	h.userListFavoriteRepo = r
}

// SetUserListRepo attaches a UserListRepository for list update endpoints.
func (h *Handler) SetUserListRepo(r repository.UserListRepository) {
	h.userListRepo = r
}

// Achievements handles POST /api/users/achievements.
func (h *Handler) Achievements(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	bundle, err := h.userService.ShowByID(req.UserID)
	if err != nil {
		return apierr.JSONNoSuchUser(c)
	}
	if bundle.Profile == nil || bundle.Profile.Achievements == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	var achievements []any
	_ = json.Unmarshal(bundle.Profile.Achievements, &achievements)
	return c.JSON(http.StatusOK, achievements)
}

// Clips handles POST /api/users/clips.
//
// 非所有者視点では public のみ返す。LIMIT は絞り込み後に適用されるよう SQL
// 側で WHERE isPublic = true を通す (ListPublicByUser) — 古い post-fetch
// filter 方式だと private が多いユーザーで空の結果を返してしまう bug になる。
func (h *Handler) Clips(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.clipRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	viewer := middleware.GetUser(c)
	isSelf := viewer != nil && viewer.ID == req.UserID
	var rows []*model.Clip
	var err error
	if isSelf {
		rows, err = h.clipRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	} else {
		rows, err = h.clipRepo.ListPublicByUser(req.UserID, req.Limit, req.Offset)
	}
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, cl := range rows {
		out = append(out, map[string]any{
			"id":            cl.ID,
			"userId":        cl.UserID,
			"name":          cl.Name,
			"description":   cl.Description,
			"isPublic":      cl.IsPublic,
			"notesCount":    cl.NotesCount,
			"lastClippedAt": cl.LastClippedAt,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// Flashs handles POST /api/users/flashs.
func (h *Handler) Flashs(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.flashRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	viewer := middleware.GetUser(c)
	isSelf := viewer != nil && viewer.ID == req.UserID
	var rows []*model.Flash
	var err error
	if isSelf {
		rows, err = h.flashRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	} else {
		rows, err = h.flashRepo.ListPublicByUser(req.UserID, req.Limit, req.Offset)
	}
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		out = append(out, map[string]any{
			"id":          f.ID,
			"updatedAt":   f.UpdatedAt,
			"title":       f.Title,
			"summary":     f.Summary,
			"userId":      f.UserID,
			"script":      f.Script,
			"permissions": []string(f.Permissions),
			"likedCount":  f.LikedCount,
			"visibility":  f.Visibility,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// GalleryPosts handles POST /api/users/gallery/posts.
func (h *Handler) GalleryPosts(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.galleryRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	rows, err := h.galleryRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	// GalleryPost に visibility 概念はなく常に公開扱い。
	out := make([]map[string]any, 0, len(rows))
	for _, g := range rows {
		out = append(out, map[string]any{
			"id":          g.ID,
			"updatedAt":   g.UpdatedAt,
			"userId":      g.UserID,
			"title":       g.Title,
			"description": g.Description,
			"fileIds":     []string(g.FileIDs),
			"tags":        []string(g.Tags),
			"isSensitive": g.IsSensitive,
			"likedCount":  g.LikedCount,
			"files":       []any{},
		})
	}
	return c.JSON(http.StatusOK, out)
}

// Pages handles POST /api/users/pages.
func (h *Handler) Pages(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.pageRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	clampListLimit(&req.Limit)
	viewer := middleware.GetUser(c)
	isSelf := viewer != nil && viewer.ID == req.UserID
	var rows []*model.Page
	var err error
	if isSelf {
		rows, err = h.pageRepo.ListByUser(req.UserID, req.Limit, req.Offset)
	} else {
		rows, err = h.pageRepo.ListPublicByUser(req.UserID, req.Limit, req.Offset)
	}
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		out = append(out, map[string]any{
			"id":        p.ID,
			"updatedAt": p.UpdatedAt,
			"userId":    p.UserID,
			"title":     p.Title,
			"name":      p.Name,
			"summary":   p.Summary,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// clampListLimit normalises a user-supplied limit to 1..100 with a default of 10.
func clampListLimit(limit *int) {
	if *limit <= 0 {
		*limit = 10
	}
	if *limit > 100 {
		*limit = 100
	}
}

// GetFrequentlyRepliedUsers handles POST /api/users/get-frequently-replied-users.
//
// 指定 userId が最近返信している相手上位 limit 件を weight = count / peak
// (最大件数で正規化) 付きで返す。Misskey 本家と同じレスポンス shape。
func (h *Handler) GetFrequentlyRepliedUsers(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	clampListLimit(&req.Limit)
	if _, err := h.userService.ShowByID(req.UserID); err != nil {
		return apierr.JSONNoSuchUser(c)
	}
	rows, err := h.noteRepo.CountReplyTargets(req.UserID, req.Limit)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	var peak int64
	for _, r := range rows {
		if r.Count > peak {
			peak = r.Count
		}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		bundle, err := h.userService.ShowByID(r.UserID)
		if err != nil {
			continue
		}
		// peak>0 は上で rows が非空なら必ず真だが、念のためガード。
		weight := 0.0
		if peak > 0 {
			weight = float64(r.Count) / float64(peak)
		}
		out = append(out, map[string]any{
			"user":   entity.PackUserDetailed(bundle.User, bundle.Profile, h.idGen),
			"weight": weight,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// birthdayRange は月日指定の単発 (Month/Day) もしくは範囲 (Begin/End) を
// JSON で受け取るための共通形。oneOf は Bind では表現できないため、
// すべて optional にして後段で検証する。
type birthdayRange struct {
	Month *int `json:"month,omitempty"`
	Day   *int `json:"day,omitempty"`
	Begin *struct {
		Month int `json:"month"`
		Day   int `json:"day"`
	} `json:"begin,omitempty"`
	End *struct {
		Month int `json:"month"`
		Day   int `json:"day"`
	} `json:"end,omitempty"`
}

func isValidMMDD(m, d int) bool {
	return m >= 1 && m <= 12 && d >= 1 && d <= 31
}

// GetFollowingUsersByBirthday handles POST /api/users/get-following-users-by-birthday.
//
// 認証ユーザーの followee のうち誕生日 (月日) が指定範囲に入る者を返す。
// 単発指定 ({month, day}) と範囲指定 ({begin, end}) の 2 形式に対応し、
// 範囲指定で begin > end の場合は年跨ぎ (例: 12/25..1/5) として扱う。
func (h *Handler) GetFollowingUsersByBirthday(c echo.Context) error {
	viewer := middleware.GetUser(c)
	var req struct {
		Limit    int            `json:"limit"`
		Offset   int            `json:"offset"`
		Birthday *birthdayRange `json:"birthday"`
	}
	if err := c.Bind(&req); err != nil || req.Birthday == nil {
		return apierr.JSONInvalidParam(c)
	}
	clampListLimit(&req.Limit)
	var begin, end int
	switch {
	case req.Birthday.Begin != nil && req.Birthday.End != nil:
		if !isValidMMDD(req.Birthday.Begin.Month, req.Birthday.Begin.Day) ||
			!isValidMMDD(req.Birthday.End.Month, req.Birthday.End.Day) {
			return apierr.JSONInvalidParam(c)
		}
		begin = req.Birthday.Begin.Month*100 + req.Birthday.Begin.Day
		end = req.Birthday.End.Month*100 + req.Birthday.End.Day
	case req.Birthday.Month != nil && req.Birthday.Day != nil:
		if !isValidMMDD(*req.Birthday.Month, *req.Birthday.Day) {
			return apierr.JSONInvalidParam(c)
		}
		begin = *req.Birthday.Month*100 + *req.Birthday.Day
		end = begin
	default:
		return apierr.JSONInvalidParam(c)
	}
	if h.followingRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.followingRepo.ListFollowingByBirthday(viewer.ID, begin, end, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	// 本家は「今日以降の最寄りの誕生日の日付」を "YYYY-MM-DD" で返す。
	now := time.Now()
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		bundle, err := h.userService.ShowByID(r.FolloweeID)
		if err != nil {
			continue
		}
		birthday := nextBirthdayDate(r.Birthday, now)
		out = append(out, map[string]any{
			"id":       r.FolloweeID,
			"birthday": birthday,
			"user":     entity.PackUserLite(bundle.User),
		})
	}
	return c.JSON(http.StatusOK, out)
}

// nextBirthdayDate returns the bday in "YYYY-MM-DD" form adjusted so that it is
// not earlier than "today". If bday's (month, day) is before today, the year is
// bumped by one — same semantics as Misskey 本家。
func nextBirthdayDate(bday string, now time.Time) string {
	if len(bday) != 10 {
		return bday
	}
	mm := int(bday[5]-'0')*10 + int(bday[6]-'0')
	dd := int(bday[8]-'0')*10 + int(bday[9]-'0')
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	bdayDate := time.Date(now.Year(), time.Month(mm), dd, 0, 0, 0, 0, now.Location())
	if bdayDate.Before(today) {
		bdayDate = bdayDate.AddDate(1, 0, 0)
	}
	return bdayDate.Format("2006-01-02")
}

// UserRecommendation handles POST /api/users/recommendation.
//
// 認証ユーザーがまだフォローしていないローカルのアクティブユーザーを
// followersCount 降順で返す (onboarding 向け)。Misskey 本家互換で 7 日以内に
// 更新があったユーザーに絞る。
func (h *Handler) UserRecommendation(c echo.Context) error {
	viewer := middleware.GetUser(c)
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	clampListLimit(&req.Limit)
	users, err := h.userService.ListRecommendations(viewer.ID, time.Now().AddDate(0, 0, -7), req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]entity.UserDetailed, 0, len(users))
	for _, u := range users {
		profile := h.userService.GetProfile(u.ID)
		out = append(out, entity.PackUserDetailed(u, profile, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// UsersBulk handles POST /api/users — bulk user lookup.
//
// Misskey 本家は userIds を最大 100 件に制限している。未知 ID は無視される
// (他の ID の結果は返す)。
func (h *Handler) UsersBulk(c echo.Context) error {
	var req struct {
		UserIDs []string `json:"userIds"`
	}
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	if len(req.UserIDs) == 0 {
		return c.JSON(http.StatusOK, []any{})
	}
	if len(req.UserIDs) > 100 {
		req.UserIDs = req.UserIDs[:100]
	}
	out := make([]entity.UserLite, 0, len(req.UserIDs))
	for _, uid := range req.UserIDs {
		if bundle, err := h.userService.ShowByID(uid); err == nil {
			out = append(out, entity.PackUserLite(bundle.User))
		}
	}
	return c.JSON(http.StatusOK, out)
}

// ListsCreateFromPublic handles POST /api/users/lists/create-from-public.
//
// 公開済みの UserList (listId) から名前を引き継いだ新しい list (name) を作って、
// 元 list のメンバーをそのままコピーする。メンバー追加で一部失敗しても残りは
// 続行する (1 件のブロックや重複で全体を失敗させないほうが UX 上望ましい)。
func (h *Handler) ListsCreateFromPublic(c echo.Context) error {
	viewer := middleware.GetUser(c)
	var req struct {
		Name   string `json:"name"`
		ListID string `json:"listId"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" || req.ListID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userListRepo == nil {
		return apierr.JSONInternalError(c)
	}
	src, err := h.userListRepo.FindByID(req.ListID)
	if err != nil || !src.IsPublic {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_LIST", "No such list.", "9292f798-6175-4f7d-93f4-b6742279667d"))
	}
	now := time.Now()
	newList := &model.UserList{
		ID:       h.idGen.Generate(now),
		UserID:   viewer.ID,
		Name:     req.Name,
		IsPublic: false,
	}
	if err := h.userListRepo.Create(newList); err != nil {
		return apierr.JSONInternalError(c)
	}
	members, err := h.userListRepo.ListMembers(req.ListID)
	if err != nil {
		// list 自体は既に作成済みなので、メンバーコピー失敗時も新 list を返す。
		return c.JSON(http.StatusOK, newList)
	}
	for _, m := range members {
		mb := &model.UserListMembership{
			ID:         h.idGen.Generate(time.Now()),
			UserListID: newList.ID,
			UserID:     m.UserID,
		}
		_ = h.userListRepo.AddMember(mb)
	}
	return c.JSON(http.StatusOK, newList)
}

// ListsFavorite handles POST /api/users/lists/favorite.
func (h *Handler) ListsFavorite(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		ListID string `json:"listId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userListFavoriteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	already, _ := h.userListFavoriteRepo.Exists(user.ID, req.ListID)
	if already {
		return c.NoContent(http.StatusNoContent)
	}
	fav := &model.UserListFavorite{
		ID:         h.idGen.Generate(time.Now()),
		UserID:     user.ID,
		UserListID: req.ListID,
	}
	if err := h.userListFavoriteRepo.Create(fav); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListsUnfavorite handles POST /api/users/lists/unfavorite.
func (h *Handler) ListsUnfavorite(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		ListID string `json:"listId"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userListFavoriteRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := h.userListFavoriteRepo.Delete(user.ID, req.ListID); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListsUpdate handles POST /api/users/lists/update.
func (h *Handler) ListsUpdate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		ListID   string `json:"listId"`
		Name     string `json:"name"`
		IsPublic *bool  `json:"isPublic"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userListRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	list, err := h.userListRepo.FindByID(req.ListID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_LIST", "No such list.", "796666fe-3dff-4d39-becb-8a5932c1d5b7"))
	}
	// 所有権チェック
	if list.UserID != user.ID {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_LIST", "No such list.", "796666fe-3dff-4d39-becb-8a5932c1d5b7"))
	}
	fields := map[string]any{}
	if req.Name != "" {
		fields["name"] = req.Name
		list.Name = req.Name
	}
	if req.IsPublic != nil {
		fields["isPublic"] = *req.IsPublic
		list.IsPublic = *req.IsPublic
	}
	if len(fields) > 0 {
		if err := h.userListRepo.UpdateList(req.ListID, fields); err != nil {
			return apierr.JSONInternalError(c)
		}
	}
	return c.JSON(http.StatusOK, list)
}

// ListsUpdateMembership handles POST /api/users/lists/update-membership.
func (h *Handler) ListsUpdateMembership(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		ListID      string `json:"listId"`
		UserID      string `json:"userId"`
		WithReplies bool   `json:"withReplies"`
	}
	if err := c.Bind(&req); err != nil || req.ListID == "" || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userListRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	list, err := h.userListRepo.FindByID(req.ListID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_LIST", "No such list.", "7f44670e-ab16-43b8-b4c1-ccd2ee89cc02"))
	}
	// 所有権チェック
	if list.UserID != user.ID {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_LIST", "No such list.", "7f44670e-ab16-43b8-b4c1-ccd2ee89cc02"))
	}
	if err := h.userListRepo.UpdateMembership(req.ListID, req.UserID, req.WithReplies); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_USER", "No such user.", "588e7f72-c744-4a61-b180-d354e912bda2"))
		}
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListsGetMemberships handles POST /api/users/lists/get-memberships.
//
// 認証ユーザが所有する UserList のうち、指定された userId を member として含む
// ものの一覧を返す。Misskey 本家互換。認証は router の RequireAuth middleware
// が 401 CREDENTIAL_REQUIRED で弾くため viewer は常に非 nil。
func (h *Handler) ListsGetMemberships(c echo.Context) error {
	viewer := middleware.GetUser(c)
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userListRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	lists, err := h.userListRepo.ListsContainingMember(viewer.ID, req.UserID)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(lists))
	for _, l := range lists {
		out = append(out, map[string]any{
			"id":       l.ID,
			"name":     l.Name,
			"userId":   l.UserID,
			"isPublic": l.IsPublic,
		})
	}
	return c.JSON(http.StatusOK, out)
}
