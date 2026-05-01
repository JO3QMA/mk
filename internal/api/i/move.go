package i

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	"github.com/shiroha-a/mk/internal/core/move"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"golang.org/x/crypto/bcrypt"
)

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
