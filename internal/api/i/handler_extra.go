package i

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

// ChangePassword handles POST /api/i/change-password.
func (h *Handler) ChangePassword(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.Bind(&req); err != nil || req.CurrentPassword == "" || req.NewPassword == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "currentPassword and newPassword are required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	profile := h.userService.GetProfile(u.ID)
	if profile == nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "No password set.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.CurrentPassword)); err != nil {
		return c.JSON(http.StatusForbidden, apierr.Error("INCORRECT_PASSWORD", "Incorrect password.", "932c904e-9460-45b7-9ce6-7ed33be7eb2c"))
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	hashStr := string(hash)
	if err := h.userService.UpdateProfileFields(u.ID, map[string]any{"password": hashStr}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteAccount handles POST /api/i/delete-account.
func (h *Handler) DeleteAccount(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Password == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "password is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	profile := h.userService.GetProfile(u.ID)
	if profile == nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "No password set.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, apierr.Error("INCORRECT_PASSWORD", "Incorrect password.", "932c904e-9460-45b7-9ce6-7ed33be7eb2c"))
	}

	// isSuspended + isDeleted を true に設定 (論理削除)
	if err := h.userService.UpdateUserFields(u.ID, map[string]any{
		"isSuspended": true,
		"isDeleted":   true,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	return c.NoContent(http.StatusNoContent)
}

// Favorites handles POST /api/i/favorites.
func (h *Handler) Favorites(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid parameters.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}
	if h.favoriteRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	favs, err := h.favoriteRepo.ListByUser(u.ID, req.Limit, req.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	// 複数の favorite note について remote user の instance を 1 回の batch
	// fetch で resolve する。
	notes := make([]*model.Note, 0, len(favs))
	for _, f := range favs {
		if f.Note != nil {
			notes = append(notes, f.Note)
		}
	}
	resolver := entity.NewInstanceResolver(h.instanceLookup(), collectNoteAuthors(notes)...)

	result := make([]map[string]any, 0, len(favs))
	for _, f := range favs {
		item := map[string]any{
			"id":     f.ID,
			"noteId": f.NoteID,
		}
		if f.Note != nil {
			pn := entity.PackNote(f.Note, h.idGen)
			resolver.FillUserLite(&pn.User)
			item["note"] = pn
		}
		result = append(result, item)
	}
	return c.JSON(http.StatusOK, result)
}

// collectNoteAuthors returns the author User pointer of each note that has
// one preloaded. Used to build an entity.InstanceResolver over a set of
// notes without pulling in the full model package in entity.
func collectNoteAuthors(notes []*model.Note) []*model.User {
	out := make([]*model.User, 0, len(notes))
	for _, n := range notes {
		if n != nil && n.User != nil {
			out = append(out, n.User)
		}
	}
	return out
}

// NotificationsGrouped handles POST /api/i/notifications-grouped.
// フロントエンドがブート直後に呼ぶ。簡易版として空配列を返す。
func (h *Handler) NotificationsGrouped(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// RegenerateToken handles POST /api/i/regenerate-token.
func (h *Handler) RegenerateToken(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Password == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "password is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	profile := h.userService.GetProfile(u.ID)
	if profile == nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "No password set.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, apierr.Error("INCORRECT_PASSWORD", "Incorrect password.", "932c904e-9460-45b7-9ce6-7ed33be7eb2c"))
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	newToken := hex.EncodeToString(b)

	if err := h.userService.UpdateUserFields(u.ID, map[string]any{"token": newToken}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}
	// TS本家 regenerate-token.ts:60 と同じく、token再生成成功後にmainへ
	// publishする。body無し(type のみで、他セッションはtoken無効化を
	// 検知してログアウトする用途)。
	if h.mainStreamPublisher != nil {
		h.mainStreamPublisher.PublishMainEvent(u.ID, "myTokenRegenerated", nil)
	}
	return c.JSON(http.StatusOK, map[string]any{"token": newToken})
}

// ClaimAchievement handles POST /api/i/claim-achievement.
// ユーザーの実績を記録する。既に獲得済みの場合は何もしない。
func (h *Handler) ClaimAchievement(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "name is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	profile := h.userService.GetProfile(u.ID)
	if profile == nil {
		return c.NoContent(http.StatusNoContent)
	}

	// 既存の実績をパース
	var achievements []map[string]any
	if profile.Achievements != nil {
		_ = json.Unmarshal(profile.Achievements, &achievements)
	}

	// 既に獲得済みか確認
	for _, a := range achievements {
		if a["name"] == req.Name {
			return c.NoContent(http.StatusNoContent)
		}
	}

	// 新しい実績を追加
	achievements = append(achievements, map[string]any{
		"name":       req.Name,
		"unlockedAt": time.Now().UnixMilli(),
	})
	data, _ := json.Marshal(achievements)

	if err := h.userService.UpdateProfileFields(u.ID, map[string]any{"achievements": string(data)}); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	return c.NoContent(http.StatusNoContent)
}
