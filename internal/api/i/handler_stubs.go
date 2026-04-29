package i

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	coreemail "github.com/shiroha-a/mk/internal/core/email"
	"github.com/shiroha-a/mk/internal/core/move"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

// generateVerifyCode returns a random 16-char hex code for email verification.
func generateVerifyCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// verifyURL builds the email verification URL.
func (h *Handler) verifyURL(code string) string {
	base := h.serverURL
	if base == "" {
		base = "https://localhost"
	}
	return base + "/verify-email/" + code
}

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

// SigninHistory handles POST /api/i/signin-history.
func (h *Handler) SigninHistory(c echo.Context) error {
	if h.signinRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	var req struct {
		Limit   *int    `json:"limit"`
		SinceID *string `json:"sinceId"`
		UntilID *string `json:"untilId"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid param.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
	}

	limit := 10
	if req.Limit != nil {
		limit = *req.Limit
		if limit < 1 {
			limit = 1
		}
		if limit > 100 {
			limit = 100
		}
	}
	sinceID := ""
	if req.SinceID != nil {
		sinceID = *req.SinceID
	}
	untilID := ""
	if req.UntilID != nil {
		untilID = *req.UntilID
	}

	rows, err := h.signinRepo.ListByUserID(u.ID, limit, untilID, sinceID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	result := make([]map[string]any, len(rows))
	for i, s := range rows {
		entry := map[string]any{
			"id":      s.ID,
			"ip":      s.IP,
			"headers": s.Headers,
			"success": s.Success,
		}
		if t, err := h.idGen.ParseTime(s.ID); err == nil {
			entry["createdAt"] = t.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		result[i] = entry
	}
	return c.JSON(http.StatusOK, result)
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

// UpdateEmail handles POST /api/i/update-email.
// 本家 Misskey と同じフロー:
//  1. パスワード検証 (必須)
//  2. email が null → emailRequiredForSignup ならエラー、そうでなければクリア
//  3. email が非null → フォーマット + banned + active/API 検証
//  4. profile の email / emailVerified / emailVerifyCode を更新
//  5. 新 email があれば確認メールを送信
func (h *Handler) UpdateEmail(c echo.Context) error {
	u := middleware.GetUser(c)
	var req struct {
		Password string  `json:"password"`
		Email    *string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return apierr.JSONInvalidParam(c)
	}

	profile := h.userService.GetProfile(u.ID)
	if profile == nil || profile.Password == nil {
		return apierr.JSONInternalError(c)
	}

	// パスワード検証
	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, apierr.Error("INCORRECT_PASSWORD", "Incorrect password.", "e86c14a4-0da8-4571-8f36-8a2e9f9b3a00"))
	}

	fields := map[string]any{
		"emailVerified":   false,
		"emailVerifyCode": nil,
	}

	if req.Email == nil {
		// email クリア。emailRequiredForSignup 時はクリア不可。
		if h.metaRepo != nil {
			if m, err := h.metaRepo.Fetch(); err == nil && m.EmailRequiredForSignup {
				return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Email is required.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
			}
		}
		fields["email"] = nil
	} else {
		addr := *req.Email
		// email validation (banned + format + active/API)
		if h.metaRepo != nil {
			if m, err := h.metaRepo.Fetch(); err == nil {
				svc := coreemail.NewService(m)
				if verr := svc.Validate(c.Request().Context(), addr); verr != nil {
					return c.JSON(http.StatusBadRequest, apierr.Error("UNAVAILABLE", "Email is not available.", "a]504947-b888-4a99-9f62-8c4a0f3a3dab"))
				}
			}
		}
		fields["email"] = addr

		// 確認コード生成 + メール送信
		code := generateVerifyCode()
		fields["emailVerifyCode"] = code

		if h.emailSender != nil {
			go h.emailSender(addr, "Verify your email",
				"Click the link to verify your email:\n"+h.verifyURL(code))
		}
	}

	if err := h.userService.UpdateProfileFields(u.ID, fields); err != nil {
		return apierr.JSONInternalError(c)
	}

	return h.Me(c)
}

// VerifyEmail handles POST /api/verify-email.
// emailVerifyCode が一致すれば emailVerified を true にする。
func (h *Handler) VerifyEmail(c echo.Context) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&req); err != nil || req.Code == "" {
		return apierr.JSONInvalidParam(c)
	}

	profile, err := h.userService.FindProfileByVerifyCode(req.Code)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_CODE", "No such code.", "1e53842e-b7f4-4e1c-8f1e-8d0a2d9b0c7e"))
	}

	if verr := h.userService.UpdateProfileFields(profile.UserID, map[string]any{
		"emailVerified":   true,
		"emailVerifyCode": nil,
	}); verr != nil {
		return apierr.JSONInternalError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

// Move handles POST /api/i/move.
//
// Mastodon / Misskey 互換のアカウント引越し。moveToAccount には移行先ユーザーの
// canonical URI (例: https://other.example/users/abc) を渡す。処理内容:
//
//  1. パスワード照合 (不正 token による乗っ取り防止)
//  2. 既に movedToUri を持っていれば ALREADY_MOVED
//  3. URI 解決して dst ユーザーを取得 (失敗時は NO_SUCH_USER)
//  4. dst.alsoKnownAs に自分の URI が含まれているかを検証
//     (相互指定されていなければ DESTINATION_ACCOUNT_FORBIDS)
//  5. movedToUri / movedAt / alsoKnownAs を DB に書き込み
//  6. Move activity を follower inbox 群に配送
//
// 本家では moveToAccount に "@user@host" 形式も受け付けるが、こちらは
// outbound WebFinger を持たないため URI のみを受け付ける。フロント側も
// リモート actor 解決後の URI を渡す運用で対応可能。
//
// 本家は secure: true フラグで session 検証を行うが、我々は secure flag の枠
// 組みを持たないため delete-account / change-password と同じく password
// パラメータで直接照合する。アカウント移行は不可逆なので password 必須を
// 崩してはいけない。
func (h *Handler) Move(c echo.Context) error {
	me := middleware.GetUser(c)
	var req struct {
		MoveToAccount string `json:"moveToAccount"`
		Password      string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.MoveToAccount == "" || req.Password == "" {
		return apierr.JSONInvalidParam(c)
	}
	profile := h.userService.GetProfile(me.ID)
	if profile == nil || profile.Password == nil {
		return c.JSON(http.StatusForbidden, apierr.Error(
			"ACCESS_DENIED", "No password set.",
			"1fb7cb09-d46a-4fff-b8df-057708cce513",
		))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*profile.Password), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusForbidden, apierr.Error(
			"INCORRECT_PASSWORD", "Incorrect password.",
			"932c904e-9460-45b7-9ce6-7ed33be7eb2c",
		))
	}
	if h.mover == nil {
		return c.JSON(http.StatusNotImplemented, apierr.Error(
			"INTERNAL_ERROR",
			"Account move is not wired up.",
			"5d37dbcb-891e-41ca-a3d6-e690c97775ac",
		))
	}
	err := h.mover.Move(me, req.MoveToAccount)
	switch {
	case err == nil:
		return h.Me(c)
	case errors.Is(err, move.ErrNoSuchUser):
		return c.JSON(http.StatusNotFound, apierr.Error(
			"NO_SUCH_USER", "No such user.",
			"fcd2eef9-a9b2-4c4f-8624-038099e90aa5",
		))
	case errors.Is(err, move.ErrAlreadyMoved):
		return c.JSON(http.StatusBadRequest, apierr.Error(
			"ALREADY_MOVED",
			"Account was already moved to another account.",
			"b234a14e-9ebe-4581-8000-074b3c215962",
		))
	case errors.Is(err, move.ErrDestinationForbids):
		return c.JSON(http.StatusBadRequest, apierr.Error(
			"DESTINATION_ACCOUNT_FORBIDS",
			"Destination account doesn't have proper 'Known As' alias, or has already moved.",
			"b5c90186-4ab0-49c8-9bba-a1f766282ba4",
		))
	case errors.Is(err, move.ErrRemoteSourceForbidden):
		return c.JSON(http.StatusBadRequest, apierr.Error(
			"INVALID_PARAM", "Remote users cannot initiate account move.",
			"3d81ceae-475f-4600-b2a8-2bc116157532",
		))
	default:
		return apierr.JSONInternalError(c)
	}
}

// paginationFromRequest reads optional limit / offset / sinceId / untilId
// JSON fields, with Misskey-friendly defaults (limit=10, offset=0,
// limit clamped to 100). Cursor 指定時は frontend Paginator が untilId
// / sinceId を投げてくる経路 (#493)。
func paginationFromRequest(c echo.Context) (limit, offset int, sinceID, untilID string) {
	var req struct {
		Limit   *int   `json:"limit"`
		Offset  *int   `json:"offset"`
		SinceID string `json:"sinceId"`
		UntilID string `json:"untilId"`
	}
	_ = c.Bind(&req)
	limit = 10
	if req.Limit != nil {
		limit = *req.Limit
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	if req.Offset != nil && *req.Offset > 0 {
		offset = *req.Offset
	}
	return limit, offset, req.SinceID, req.UntilID
}

// GalleryLikes handles POST /api/i/gallery/likes.
// 自分が like した gallery_post を返却する。
//
// upstream Misskey TS は `{id, post: GalleryPost}` を返し、frontend は
// `item.post` で MkGalleryPostPreview を描画する。mk-go は元々 `postId`
// しか返しておらず frontend が空表示になっていた (#493 関連)。
// 同じ shape に揃えるため like 行ごとに対応する gallery_post を fetch
// して埋める。
func (h *Handler) GalleryLikes(c echo.Context) error {
	if h.galleryRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	limit, offset, sinceID, untilID := paginationFromRequest(c)
	likes, err := h.galleryRepo.ListLikesByUser(u.ID, sinceID, untilID, limit, offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	postIDs := make([]string, 0, len(likes))
	for _, l := range likes {
		postIDs = append(postIDs, l.PostID)
	}
	posts, err := h.galleryRepo.FindPostsByIDs(postIDs)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	postByID := make(map[string]*model.GalleryPost, len(posts))
	for _, p := range posts {
		postByID[p.ID] = p
	}
	out := make([]map[string]any, 0, len(likes))
	for _, l := range likes {
		// upstream Misskey TS は post が削除済の like row を Promise.all
		// + pack の null フィルタで弾く。frontend の MkGalleryPostPreview
		// も it.post 必須なので、ここで skip して整合させる。
		p, ok := postByID[l.PostID]
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"id":   l.ID,
			"post": packGalleryPost(p, h.idGen),
		})
	}
	return c.JSON(http.StatusOK, out)
}

// packGalleryPost は upstream の GalleryPost shape (createdAt / user 含む)
// に揃えてレスポンス用の map を返す。
func packGalleryPost(p *model.GalleryPost, idGen id.Generator) map[string]any {
	const tsFormat = "2006-01-02T15:04:05.000Z"
	createdAt := ""
	if idGen != nil {
		if t, err := idGen.ParseTime(p.ID); err == nil {
			createdAt = t.UTC().Format(tsFormat)
		}
	}
	resp := map[string]any{
		"id":          p.ID,
		"createdAt":   createdAt,
		"updatedAt":   p.UpdatedAt.UTC().Format(tsFormat),
		"title":       p.Title,
		"description": p.Description,
		"userId":      p.UserID,
		"fileIds":     []string(p.FileIDs),
		"files":       []any{},
		"isSensitive": p.IsSensitive,
		"likedCount":  p.LikedCount,
		"tags":        []string(p.Tags),
	}
	if p.User != nil {
		resp["user"] = entity.PackUserLite(p.User)
	}
	return resp
}

// GalleryPosts handles POST /api/i/gallery/posts.
// 自分が投稿した gallery_post を返却する。
func (h *Handler) GalleryPosts(c echo.Context) error {
	if h.galleryRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	limit, offset, sinceID, untilID := paginationFromRequest(c)
	posts, err := h.galleryRepo.ListByUser(u.ID, sinceID, untilID, limit, offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(posts))
	for _, p := range posts {
		out = append(out, packGalleryPost(p, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// PageLikes handles POST /api/i/page-likes.
// 自分が like した page 一覧を返却する。
func (h *Handler) PageLikes(c echo.Context) error {
	if h.pageLikeRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	limit, offset, _, _ := paginationFromRequest(c)
	likes, err := h.pageLikeRepo.ListByUser(u.ID, limit, offset)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(likes))
	for _, l := range likes {
		out = append(out, map[string]any{
			"id":     l.ID,
			"pageId": l.PageID,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// registryScopeDomainRequest is the canonical input shape for registry
// read endpoints. scope は []string、domain は *string (省略可)。
type registryScopeDomainRequest struct {
	Scope  []string `json:"scope"`
	Domain *string  `json:"domain"`
}

// normalizeRegistryScope treats a nil Scope as an empty slice. JSON で
// `"scope"` が省略されると req.Scope は nil になるが、repository 側で
// pq.StringArray(nil) は SQL NULL にシリアライズされ `scope = NULL` で
// どのレコードにも一致しなくなる (registry_item のデフォルト値は
// `'{}'`)。他の registry* ハンドラ (RegistryGet / RegistryGetAll /
// RegistrySet 等) と挙動を揃えるため nil → 空配列に寄せる。
func normalizeRegistryScope(scope []string) []string {
	if scope == nil {
		return []string{}
	}
	return scope
}

// RegistryGetDetail handles POST /api/i/registry/get-detail.
// 指定の (key, scope, domain) に該当する RegistryItem を返す。
func (h *Handler) RegistryGetDetail(c echo.Context) error {
	if h.registryRepo == nil {
		return c.JSON(http.StatusOK, map[string]any{})
	}
	u := middleware.GetUser(c)
	var req struct {
		Key string `json:"key"`
		registryScopeDomainRequest
	}
	if err := c.Bind(&req); err != nil || req.Key == "" {
		return apierr.JSONInvalidParam(c)
	}
	req.Scope = normalizeRegistryScope(req.Scope)
	item, err := h.registryRepo.Get(u.ID, req.Key, req.Scope, req.Domain)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_KEY", "No such key.", "7f5e1e4a-1e2a-41b9-80e3-8a0d6e8aa8b1"))
	}
	return c.JSON(http.StatusOK, map[string]any{
		"updatedAt": item.UpdatedAt,
		"value":     item.Value,
		"scope":     []string(item.Scope),
		"domain":    item.Domain,
	})
}

// RegistryKeys handles POST /api/i/registry/keys.
// 指定 scope+domain 下の key 一覧を返す (本家互換で配列)。
func (h *Handler) RegistryKeys(c echo.Context) error {
	if h.registryRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	var req registryScopeDomainRequest
	_ = c.Bind(&req)
	req.Scope = normalizeRegistryScope(req.Scope)
	keysMap, err := h.registryRepo.KeysWithType(u.ID, req.Scope, req.Domain)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	keys := make([]string, 0, len(keysMap))
	for k := range keysMap {
		keys = append(keys, k)
	}
	return c.JSON(http.StatusOK, keys)
}

// RegistryScopesWithDomain handles POST /api/i/registry/scopes-with-domain.
// ユーザーが保存している (scope, domain) の distinct 一覧を返す。
func (h *Handler) RegistryScopesWithDomain(c echo.Context) error {
	if h.registryRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	u := middleware.GetUser(c)
	pairs, err := h.registryRepo.ScopesWithDomain(u.ID)
	if err != nil {
		return apierr.JSONInternalError(c)
	}
	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]any{
			"scope":  p.Scope,
			"domain": p.Domain,
		})
	}
	return c.JSON(http.StatusOK, out)
}
