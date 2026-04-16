package federation

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/model"
)

// hostPageRequest is the common request body for the three per-host listing
// endpoints (federation/followers, federation/following, federation/users).
type hostPageRequest struct {
	Host   string `json:"host"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Followers handles POST /api/federation/followers.
//
// 指定リモートホストに属するユーザーが、このインスタンスのローカルユーザーを
// フォローしている関係の一覧を返す。Misskey 本家互換。
func (h *Handler) Followers(c echo.Context) error {
	req, err := bindHostPage(c)
	if err != nil {
		return err
	}
	if h.followingRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.followingRepo.ListFollowersByHost(req.Host, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, packFollowings(rows))
}

// Following handles POST /api/federation/following.
//
// このインスタンスのローカルユーザーが、指定リモートホストに属するユーザーを
// フォローしている関係の一覧を返す。
func (h *Handler) Following(c echo.Context) error {
	req, err := bindHostPage(c)
	if err != nil {
		return err
	}
	if h.followingRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	rows, err := h.followingRepo.ListFollowingByHost(req.Host, req.Limit, req.Offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.JSON(http.StatusOK, packFollowings(rows))
}

// bindHostPage parses + validates the common per-host page request, returning
// a normalized body (limit defaulted to 30, clamped to 100).
func bindHostPage(c echo.Context) (hostPageRequest, error) {
	var req hostPageRequest
	if err := c.Bind(&req); err != nil || req.Host == "" {
		return req, apierr.JSONInvalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 30
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	return req, nil
}

// Users handles POST /api/federation/users.
//
// 指定リモートホストに属するユーザーの一覧を返す。
func (h *Handler) Users(c echo.Context) error {
	var req hostPageRequest
	if err := c.Bind(&req); err != nil || req.Host == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	users, err := h.userRepo.ListUsers(model.UserListFilter{
		Origin:   "remote",
		Hostname: req.Host,
		Limit:    limit,
		Offset:   req.Offset,
	})
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, map[string]any{
			"id":        u.ID,
			"username":  u.Username,
			"host":      u.Host,
			"name":      u.Name,
			"avatarUrl": u.AvatarURL,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// Stats handles POST /api/federation/stats.
//
// 連合統計: followers / following がもっとも多いリモートインスタンス上位
// N 件と、そのほかのインスタンスをまとめた残り総計を返す。
func (h *Handler) Stats(c echo.Context) error {
	var req struct {
		Limit int `json:"limit"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 10
	}

	empty := map[string]any{
		"topSubInstances":     []any{},
		"otherFollowersCount": 0,
		"topPubInstances":     []any{},
		"otherFollowingCount": 0,
	}
	if h.svc == nil {
		return c.JSON(http.StatusOK, empty)
	}

	subs, err := h.svc.List(model.InstanceListFilter{SortBy: "-followers", Limit: req.Limit})
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	pubs, err := h.svc.List(model.InstanceListFilter{SortBy: "-following", Limit: req.Limit})
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	totalFollowers, totalFollowing, err := totalCounts(h.svc)
	if err != nil {
		return apierr.JSONInternalError(c)
	}

	topSubFollowers := 0
	topSub := make([]map[string]any, 0, len(subs))
	for _, inst := range subs {
		topSubFollowers += inst.FollowersCount
		topSub = append(topSub, instanceToMap(inst))
	}
	topPubFollowing := 0
	topPub := make([]map[string]any, 0, len(pubs))
	for _, inst := range pubs {
		topPubFollowing += inst.FollowingCount
		topPub = append(topPub, instanceToMap(inst))
	}

	return c.JSON(http.StatusOK, map[string]any{
		"topSubInstances":     topSub,
		"otherFollowersCount": totalFollowers - topSubFollowers,
		"topPubInstances":     topPub,
		"otherFollowingCount": totalFollowing - topPubFollowing,
	})
}

// UpdateRemoteUser handles POST /api/federation/update-remote-user.
//
// 対象のリモートユーザーを ActivityPub actor fetch で強制更新する。管理者用。
func (h *Handler) UpdateRemoteUser(c echo.Context) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := c.Bind(&req); err != nil || req.UserID == "" {
		return apierr.JSONInvalidParam(c)
	}
	if h.userRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	user, err := h.userRepo.FindByID(req.UserID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_USER", "No such user.", "ae1bd95a-1a3b-4d4e-9c47-7e76a2020f8e"))
	}
	if user.URI == nil || *user.URI == "" {
		// ローカルユーザーは対象外。
		return c.NoContent(http.StatusNoContent)
	}
	if h.resolver == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if _, err := h.resolver.ForceResolveActor(*user.URI); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}

// totalCounts sums followersCount / followingCount across every instance row.
// Scans the whole instance table in pages; OK for current scale (a few
// thousand rows) and matches upstream Misskey's aggregation approach.
func totalCounts(svc interface {
	List(filter model.InstanceListFilter) ([]*model.Instance, error)
}) (int, int, error) {
	totalFollowers := 0
	totalFollowing := 0
	offset := 0
	for {
		page, err := svc.List(model.InstanceListFilter{Limit: 100, Offset: offset})
		if err != nil {
			return 0, 0, err
		}
		if len(page) == 0 {
			break
		}
		for _, inst := range page {
			totalFollowers += inst.FollowersCount
			totalFollowing += inst.FollowingCount
		}
		if len(page) < 100 {
			break
		}
		offset += 100
	}
	return totalFollowers, totalFollowing, nil
}

func packFollowings(rows []*model.Following) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		out = append(out, map[string]any{
			"id":           f.ID,
			"followerId":   f.FollowerID,
			"followeeId":   f.FolloweeID,
			"followerHost": f.FollowerHost,
			"followeeHost": f.FolloweeHost,
			"withReplies":  f.WithReplies,
		})
	}
	return out
}
