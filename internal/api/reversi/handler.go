package reversi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	corereversi "github.com/shiroha-a/mk/internal/core/reversi"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"gorm.io/datatypes"
)

// Handler handles reversi/* endpoints.
type Handler struct {
	repo  repository.ReversiRepository
	idGen id.Generator
}

// NewHandler creates a new reversi handler.
func NewHandler(repo repository.ReversiRepository, idGen id.Generator) *Handler {
	return &Handler{repo: repo, idGen: idGen}
}

func apiError(code, message, errID string) map[string]any {
	return map[string]any{
		"error": map[string]any{"message": message, "code": code, "id": errID},
	}
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
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "gameId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	game, err := h.repo.FindByID(req.GameID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_GAME", "No such game.", "d8a95858-973b-4f3b-8592-fcf2eb4dd044"))
	}
	return c.JSON(http.StatusOK, packGame(game, h.idGen))
}

// Match handles POST /api/reversi/match — create or join a game.
func (h *Handler) Match(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		UserID string `json:"userId"`
	}
	_ = c.Bind(&req)

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
		return c.JSON(http.StatusInternalServerError, apiError("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
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
func (h *Handler) Surrender(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		GameID string `json:"gameId"`
	}
	if err := c.Bind(&req); err != nil || req.GameID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "gameId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	game, err := h.repo.FindByID(req.GameID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_GAME", "No such game.", "d8a95858-973b-4f3b-8592-fcf2eb4dd044"))
	}
	if game.User1ID != user.ID && game.User2ID != user.ID {
		return c.JSON(http.StatusForbidden, apiError("ACCESS_DENIED", "Access denied.", "00000000-0000-0000-0000-000000000000"))
	}

	// 相手が勝者
	var winnerID string
	if game.User1ID == user.ID {
		winnerID = game.User2ID
	} else {
		winnerID = game.User1ID
	}
	now := time.Now()
	game.IsEnded = true
	game.EndedAt = &now
	game.WinnerID = &winnerID
	game.SurrenderedUserID = &user.ID
	_ = h.repo.Update(game)

	return c.NoContent(http.StatusNoContent)
}

// Verify handles POST /api/reversi/verify — verify game integrity.
func (h *Handler) Verify(c echo.Context) error {
	var req struct {
		GameID string `json:"gameId"`
	}
	if err := c.Bind(&req); err != nil || req.GameID == "" {
		return c.JSON(http.StatusBadRequest, apiError("INVALID_PARAM", "gameId is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}
	game, err := h.repo.FindByID(req.GameID)
	if err != nil {
		return c.JSON(http.StatusNotFound, apiError("NO_SUCH_GAME", "No such game.", "d8a95858-973b-4f3b-8592-fcf2eb4dd044"))
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
