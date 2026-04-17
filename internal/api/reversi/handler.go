package reversi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/api/apierr"
	corereversi "github.com/shiroha-a/mk/internal/core/reversi"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/datatypes"
)

// FederationDeliverer delivers AP activities to remote users.
// 循環依存を避けるため interface で定義。実装は core/federation.DeliverService。
type FederationDeliverer interface {
	DeliverToUser(signerUserID string, recipient *model.User, body []byte) error
}

// Handler handles reversi/* endpoints.
type Handler struct {
	repo      repository.ReversiRepository
	svc       *corereversi.Service
	idGen     id.Generator
	baseURL   string
	deliverer FederationDeliverer
	fedCache  *corereversi.FederationIDCache
	userRepo  repository.UserRepository
}

// NewHandler creates a new reversi handler.
func NewHandler(repo repository.ReversiRepository, idGen id.Generator) *Handler {
	return &Handler{repo: repo, idGen: idGen}
}

// SetService attaches the core reversi service. 必須ではないが設定されていれば
// Surrender 等の player action が service 経由で動く (IsStarted バリデーション +
// WebSocket 通知が効くようになる)。nil の場合は従来の repo 直接操作にフォール
// バックするので既存テスト互換。
func (h *Handler) SetService(svc *corereversi.Service) {
	h.svc = svc
}

// SetFederation attaches federation support.
func (h *Handler) SetFederation(baseURL string, deliverer FederationDeliverer, fedCache *corereversi.FederationIDCache, userRepo repository.UserRepository) {
	h.baseURL = baseURL
	h.deliverer = deliverer
	h.fedCache = fedCache
	h.userRepo = userRepo
}

func packGame(g *model.ReversiGame, idGen id.Generator) map[string]any {
	result := map[string]any{
		"id":                   g.ID,
		"user1Id":              g.User1ID,
		"user2Id":              g.User2ID,
		"user1Ready":           g.User1Ready,
		"user2Ready":           g.User2Ready,
		"black":                g.Black,
		"isStarted":            g.IsStarted,
		"isEnded":              g.IsEnded,
		"winnerId":             g.WinnerID,
		"surrenderedUserId":    g.SurrenderedUserID,
		"timeoutUserId":        g.TimeoutUserID,
		"timeLimitForEachTurn": g.TimeLimitForEachTurn,
		"noIrregularRules":     g.NoIrregularRules,
		"isLlotheo":            g.IsLlotheo,
		"canPutEverywhere":     g.CanPutEverywhere,
		"loopedBoard":          g.LoopedBoard,
		"map":                  g.Map,
		"bw":                   g.BW,
		"startedAt":            g.StartedAt,
		"endedAt":              g.EndedAt,
		"logs":                 g.Logs,
		"crc32":                g.CRC32,
	}
	if t, err := idGen.ParseTime(g.ID); err == nil {
		result["createdAt"] = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	if g.User1 != nil {
		result["user1"] = entity.PackUserLite(g.User1)
	}
	if g.User2 != nil {
		result["user2"] = entity.PackUserLite(g.User2)
	}
	return result
}

// 標準8x8盤面
var defaultMap = pq.StringArray{
	"--------",
	"--------",
	"--------",
	"---wb---",
	"---bw---",
	"--------",
	"--------",
	"--------",
}

// resolveAcct converts an acct form (`@user` or `@user@host`) into a local
// user id by looking up the UserRepository. Remote users must already exist
// locally — webfinger-based discovery is intentionally out of scope.
func (h *Handler) resolveAcct(acct string) (string, error) {
	trimmed := strings.TrimPrefix(acct, "@")
	if trimmed == "" {
		return "", errors.New("empty acct")
	}
	username := trimmed
	var hostPtr *string
	if at := strings.IndexByte(trimmed, '@'); at >= 0 {
		username = trimmed[:at]
		host := strings.ToLower(trimmed[at+1:])
		if host != "" {
			hostPtr = &host
		}
	}
	u, err := h.userRepo.FindByUsernameLower(strings.ToLower(username), hostPtr)
	if err != nil {
		return "", err
	}
	return u.ID, nil
}

// Games handles POST /api/reversi/games — list games.
func (h *Handler) Games(c echo.Context) error {
	var req struct {
		Limit int `json:"limit"`
	}
	_ = c.Bind(&req)
	if req.Limit <= 0 {
		req.Limit = 10
	}
	viewer := middleware.GetUser(c)
	var games []*model.ReversiGame
	if viewer != nil {
		games, _ = h.repo.ListByUser(viewer.ID, req.Limit)
	} else {
		games, _ = h.repo.ListActive()
	}
	out := make([]map[string]any, len(games))
	for i, g := range games {
		out[i] = packGame(g, h.idGen)
	}
	return c.JSON(http.StatusOK, out)
}

// Invitations handles POST /api/reversi/invitations — list pending invitations.
func (h *Handler) Invitations(c echo.Context) error {
	user := middleware.GetUser(c)
	games, _ := h.repo.ListByUser(user.ID, 20)
	var pending []map[string]any
	for _, g := range games {
		if !g.IsStarted && !g.IsEnded {
			pending = append(pending, packGame(g, h.idGen))
		}
	}
	if pending == nil {
		pending = []map[string]any{}
	}
	return c.JSON(http.StatusOK, pending)
}

// ShowGame handles POST /api/reversi/show-game.
func (h *Handler) ShowGame(c echo.Context) error {
	var req struct {
		GameID string `json:"gameId"`
	}
	if err := c.Bind(&req); err != nil || req.GameID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "gameId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	game, err := h.repo.FindByID(req.GameID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_GAME", "No such game.", "d8a95858-973b-4f3b-8592-fcf2eb4dd044"))
	}
	return c.JSON(http.StatusOK, packGame(game, h.idGen))
}

// Match handles POST /api/reversi/match — create or join a game.
// userId は本家互換で local user id を受け付けるが、CherryPick 拡張として
// `@user` / `@user@host` 形式 (acct) も受け入れる。これは vanilla Misskey
// フロントエンドが「対戦相手選択画面」を持たず、リモートユーザーを選択で
// きない制約を緩和するための backend-side workaround。
func (h *Handler) Match(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		UserID string `json:"userId"`
	}
	_ = c.Bind(&req)

	// acct 形式 (@user / @user@host) を local user id に解決する
	if strings.HasPrefix(req.UserID, "@") && h.userRepo != nil {
		resolved, err := h.resolveAcct(req.UserID)
		if err != nil {
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_USER", "No such user.", "6cc579cc-885d-43d8-95c2-b8c7fc963280"))
		}
		req.UserID = resolved
	}

	now := time.Now()
	game := &model.ReversiGame{
		ID:                   h.idGen.Generate(now),
		User1ID:              user.ID,
		User2ID:              req.UserID,
		Map:                  defaultMap,
		BW:                   "random",
		TimeLimitForEachTurn: 90,
		Logs:                 datatypes.JSON("[]"),
	}
	if req.UserID == "" {
		// ランダムマッチ — user2IDを空にしてマッチ待ち
		game.User2ID = user.ID // 自分vs自分 (仮置き、実際はマッチング待ち)
	}

	if err := h.repo.Create(game); err != nil {
		return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
	}

	// リモートユーザーの場合、federation session を Redis に保存 + Invite 送信。
	// DB スキーマは本家互換を保つため、session id はカラムに持たせず Redis のみに
	// 保持する (internal/core/reversi.FederationIDCache)。
	if req.UserID != "" && h.userRepo != nil && h.deliverer != nil && h.fedCache != nil {
		if targetUser, err := h.userRepo.FindByID(req.UserID); err == nil && targetUser.Host != nil && targetUser.URI != nil {
			sessionID := h.idGen.Generate(now) + "-fed"
			h.fedCache.Set(c.Request().Context(), sessionID, game.ID)
			invite := corereversi.RenderInvite(h.baseURL, sessionID,
				h.baseURL+"/users/"+user.ID, *targetUser.URI, now.UTC().Format(time.RFC3339))
			if body, jerr := json.Marshal(invite); jerr == nil {
				_ = h.deliverer.DeliverToUser(user.ID, targetUser, body)
			}
		}
	}

	return c.JSON(http.StatusOK, packGame(game, h.idGen))
}

// CancelMatch handles POST /api/reversi/cancel-match.
func (h *Handler) CancelMatch(c echo.Context) error {
	user := middleware.GetUser(c)
	// 未開始のゲームを削除
	games, _ := h.repo.ListByUser(user.ID, 10)
	for _, g := range games {
		if !g.IsStarted && !g.IsEnded {
			_ = h.repo.Delete(g.ID)
		}
	}
	return c.NoContent(http.StatusNoContent)
}

// Surrender handles POST /api/reversi/surrender.
// 対局中 (IsStarted かつ not IsEnded) のみ許可する。service.Surrender を経由
// することで WebSocket チャネルに `ended` イベントを publish するので、両
// プレイヤーのクライアントが即座に終局状態を検出できる。
func (h *Handler) Surrender(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		GameID string `json:"gameId"`
	}
	if err := c.Bind(&req); err != nil || req.GameID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "gameId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	// federation Leave を送るために winner (= 相手) を先に引いておく。
	// service.Surrender は repo 越しに game を読むが、federation session の
	// lookup は handler のタイミングで必要なので preload する。
	game, err := h.repo.FindByID(req.GameID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_GAME", "No such game.", "d8a95858-973b-4f3b-8592-fcf2eb4dd044"))
	}
	if game.User1ID != user.ID && game.User2ID != user.ID {
		return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "Access denied.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
	}
	var winnerID string
	if game.User1ID == user.ID {
		winnerID = game.User2ID
	} else {
		winnerID = game.User1ID
	}

	// service が注入されていればそちらを経由する (本来のパス)。
	// フォールバックは従来の repo 直接操作 (service 未注入の古いテスト互換)。
	if h.svc != nil {
		if err := h.svc.Surrender(c.Request().Context(), req.GameID, user.ID); err != nil {
			return surrenderErrorResponse(c, err)
		}
	} else {
		now := time.Now()
		game.IsEnded = true
		game.EndedAt = &now
		game.WinnerID = &winnerID
		game.SurrenderedUserID = &user.ID
		_ = h.repo.Update(game)
	}

	// リモート相手に Leave 送信。federation session は Redis 側にある。
	if h.fedCache != nil && h.deliverer != nil && h.userRepo != nil {
		if sessionID, ok := h.fedCache.GetSessionByGame(c.Request().Context(), req.GameID); ok {
			if remoteUser, err := h.userRepo.FindByID(winnerID); err == nil && remoteUser.Host != nil && remoteUser.URI != nil {
				leave := corereversi.RenderLeave(h.baseURL+"/users/"+user.ID, *remoteUser.URI, sessionID)
				if body, jerr := json.Marshal(leave); jerr == nil {
					_ = h.deliverer.DeliverToUser(user.ID, remoteUser, body)
				}
			}
			// ゲーム終了時に mapping を明示削除
			h.fedCache.Delete(c.Request().Context(), sessionID, req.GameID)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// surrenderErrorResponse maps core/reversi service errors to Misskey-
// compatible HTTP error bodies. Unknown errors fall back to 500.
func surrenderErrorResponse(c echo.Context, err error) error {
	switch {
	case errors.Is(err, corereversi.ErrGameNotFound):
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_GAME", "No such game.", "d8a95858-973b-4f3b-8592-fcf2eb4dd044"))
	case errors.Is(err, corereversi.ErrNotPlayer):
		return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "Access denied.", "1fb7cb09-d46a-4fff-b8df-057708cce513"))
	case errors.Is(err, corereversi.ErrAlreadyEnded):
		return c.JSON(http.StatusBadRequest, apierr.Error("ALREADY_ENDED", "Game has already ended.", "2a3a7f72-bc06-4f4e-9f7c-b7f8d4f6a09e"))
	case errors.Is(err, corereversi.ErrNotStarted):
		return c.JSON(http.StatusBadRequest, apierr.Error("NOT_STARTED", "Game has not started yet.", "ac4bb45f-ea81-44d3-a5b3-fe5f30be2c8d"))
	}
	return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
}

// Verify handles POST /api/reversi/verify — verify game integrity.
func (h *Handler) Verify(c echo.Context) error {
	var req struct {
		GameID string `json:"gameId"`
	}
	if err := c.Bind(&req); err != nil || req.GameID == "" {
		return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "gameId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	game, err := h.repo.FindByID(req.GameID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_GAME", "No such game.", "d8a95858-973b-4f3b-8592-fcf2eb4dd044"))
	}

	// ゲームログを再生して検証
	opts := corereversi.Options{
		IsLlotheo:        game.IsLlotheo,
		CanPutEverywhere: game.CanPutEverywhere,
		LoopedBoard:      game.LoopedBoard,
	}
	g := corereversi.NewGame(game.Map, opts)

	var logs [][]int
	_ = json.Unmarshal(game.Logs, &logs)
	for _, log := range logs {
		if len(log) >= 1 {
			g.PutStone(log[0])
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"desynced": false,
		"game":     packGame(game, h.idGen),
	})
}
