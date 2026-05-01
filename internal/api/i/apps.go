package i

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Apps handles POST /api/i/apps.
// Owned OAuth2 application registration/management は本家 Misskey でも
// 現状未使用 (app 登録 UI が削除されている) ので常に空配列を返す。
// OAuth owner-apps listing を復活させる場合は別 issue で扱う。
func (h *Handler) Apps(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// AuthorizedApps handles POST /api/i/authorized-apps.
// 本家 Misskey の実装に合わせて、自分に紐づく access_token を返す。
// name / description / iconUrl / permission / lastUsedAt を含む。
func (h *Handler) AuthorizedApps(c echo.Context) error {
	if h.accessTokenRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	tokens, err := h.accessTokenRepo.ListByUserID(u.ID)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		entry := map[string]any{
			"id":          t.ID,
			"name":        t.Name,
			"description": t.Description,
			"iconUrl":     t.IconURL,
			"permission":  []string(t.Permission),
			"lastUsedAt":  t.LastUsedAt,
		}
		if ct, err := h.idGen.ParseTime(t.ID); err == nil {
			entry["createdAt"] = ct.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		out = append(out, entry)
	}
	return c.JSON(http.StatusOK, out)
}

// RevokeToken handles POST /api/i/revoke-token.
// TS互換: tokenId (ID) または token (ハッシュ文字列) のどちらかで指定可。
// 指定 access_token が自分の所有であれば削除する。
func (h *Handler) RevokeToken(c echo.Context) error {
	if h.accessTokenRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	u := middleware.GetUser(c)
	var req struct {
		TokenID string `json:"tokenId"`
		Token   string `json:"token"`
	}
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}
	if req.TokenID == "" && req.Token == "" {
		return apierr.JSONInvalidParam(c)
	}
	var tokenID string
	if req.TokenID != "" {
		tok, err := h.accessTokenRepo.FindByID(req.TokenID)
		if err != nil {
			return c.NoContent(http.StatusNoContent)
		}
		if tok.UserID != u.ID {
			return apierr.JSONAccessDenied(c)
		}
		tokenID = tok.ID
	} else {
		// DBはSHA-256ハッシュを格納しているため、生tokenをハッシュ化して検索
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Token)))
		tok, err := h.accessTokenRepo.FindByHash(hash)
		if err != nil {
			return c.NoContent(http.StatusNoContent)
		}
		if tok.UserID != u.ID {
			return apierr.JSONAccessDenied(c)
		}
		tokenID = tok.ID
	}
	if err := h.accessTokenRepo.DeleteByID(tokenID); err != nil {
		return apierr.JSONInternalError(c)
	}
	return c.NoContent(http.StatusNoContent)
}
