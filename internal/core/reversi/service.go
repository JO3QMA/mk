package reversi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// --- Federation ID cache (existing) ---

// FederationIDCache manages federationId ↔ gameId mapping in Redis.
type FederationIDCache struct {
	redis redis.Cmdable
}

// NewFederationIDCache creates a new cache.
func NewFederationIDCache(r redis.Cmdable) *FederationIDCache {
	return &FederationIDCache{redis: r}
}

// Set stores a federationId → gameId mapping.
func (c *FederationIDCache) Set(ctx context.Context, federationID, gameID string) {
	if c.redis == nil {
		return
	}
	c.redis.Set(ctx, "reversi:federationId:"+federationID, gameID, 5*time.Minute)
}

// Get retrieves a gameId from a federationId.
func (c *FederationIDCache) Get(ctx context.Context, federationID string) (string, error) {
	if c.redis == nil {
		return "", redis.Nil
	}
	return c.redis.Get(ctx, "reversi:federationId:"+federationID).Result()
}

// ValidUpdateKeys lists the keys that can be changed via federation Update.
var ValidUpdateKeys = map[string]bool{
	"map":                  true,
	"bw":                   true,
	"isLlotheo":            true,
	"canPutEverywhere":     true,
	"loopedBoard":          true,
	"timeLimitForEachTurn": true,
	"noIrregularRules":     true,
}

// IsValidUpdateKey checks if a key is allowed for federation settings updates.
func IsValidUpdateKey(key string) bool {
	return ValidUpdateKeys[key]
}

// --- Errors returned by Service methods ---

var (
	// ErrGameNotFound is returned when the referenced game does not exist.
	ErrGameNotFound = errors.New("reversi game not found")
	// ErrNotPlayer is returned when a non-player attempts a player-only action.
	ErrNotPlayer = errors.New("user is not a player in this game")
	// ErrAlreadyStarted is returned when attempting pre-start changes on a
	// running game.
	ErrAlreadyStarted = errors.New("reversi game already started")
	// ErrAlreadyEnded is returned when attempting any action on a concluded game.
	ErrAlreadyEnded = errors.New("reversi game already ended")
	// ErrNotStarted is returned when attempting in-game actions on a pre-start
	// game.
	ErrNotStarted = errors.New("reversi game not yet started")
	// ErrNotYourTurn is returned when a player tries to move out of turn.
	ErrNotYourTurn = errors.New("not your turn")
	// ErrInvalidMove is returned when a move violates game rules.
	ErrInvalidMove = errors.New("invalid move")
	// ErrInvalidSetting is returned when an updateSettings key/value is refused.
	ErrInvalidSetting = errors.New("invalid setting")
)

// --- PubSub publisher interface ---

// GamePublisher is the minimal interface the Service uses to push events to
// the reversi WebSocket channel. 循環依存を避けるため interface で受ける。
type GamePublisher interface {
	PublishGameEvent(gameID, eventType string, body any)
}

// --- Core service ---

// Service manages reversi game state transitions and publishes updates to
// the matching WebSocket channel.
type Service struct {
	repo      repository.ReversiRepository
	publisher GamePublisher
	redis     redis.Cmdable
	fedCache  *FederationIDCache
}

// NewService constructs a Service with the required dependencies.
func NewService(
	repo repository.ReversiRepository,
	publisher GamePublisher,
	r redis.Cmdable,
) *Service {
	return &Service{repo: repo, publisher: publisher, redis: r}
}

// SetFederationCache attaches a FederationIDCache so that federated
// Invite/Join/Leave messages can resolve the game id.
func (s *Service) SetFederationCache(c *FederationIDCache) {
	s.fedCache = c
}

// --- Queries ---

// Get fetches a game by id.
func (s *Service) Get(_ context.Context, gameID string) (*model.ReversiGame, error) {
	game, err := s.repo.FindByID(gameID)
	if err != nil {
		return nil, ErrGameNotFound
	}
	return game, nil
}

// --- Pre-start transitions ---

// UpdateReady toggles the per-user ready flag. When both players are ready,
// StartGame is fired automatically.
func (s *Service) UpdateReady(ctx context.Context, gameID, userID string, ready bool) error {
	game, err := s.Get(ctx, gameID)
	if err != nil {
		return err
	}
	if game.IsStarted {
		return ErrAlreadyStarted
	}
	if game.IsEnded {
		return ErrAlreadyEnded
	}
	if !isPlayer(game, userID) {
		return ErrNotPlayer
	}
	if game.User1ID == userID {
		game.User1Ready = ready
	} else {
		game.User2Ready = ready
	}
	if err := s.repo.Update(game); err != nil {
		return err
	}
	s.publish(gameID, "changeReadyStates", map[string]any{
		"user1": game.User1Ready,
		"user2": game.User2Ready,
	})
	if game.User1Ready && game.User2Ready {
		return s.StartGame(ctx, game)
	}
	return nil
}

// UpdateSettings mutates a single pre-start setting (map, bw, isLlotheo, etc.)
// and broadcasts the change so both clients can re-render.
func (s *Service) UpdateSettings(ctx context.Context, gameID, userID, key string, value json.RawMessage) error {
	game, err := s.Get(ctx, gameID)
	if err != nil {
		return err
	}
	if game.IsStarted || game.IsEnded {
		return ErrAlreadyStarted
	}
	if !isPlayer(game, userID) {
		return ErrNotPlayer
	}
	if !IsValidUpdateKey(key) {
		return ErrInvalidSetting
	}
	if err := applySetting(game, key, value); err != nil {
		return err
	}
	// 設定変更は ready 状態をリセットする (本家挙動)
	game.User1Ready = false
	game.User2Ready = false
	if err := s.repo.Update(game); err != nil {
		return err
	}
	var valueAny any
	_ = json.Unmarshal(value, &valueAny)
	s.publish(gameID, "updateSettings", map[string]any{
		"userId": userID,
		"key":    key,
		"value":  valueAny,
	})
	s.publish(gameID, "changeReadyStates", map[string]any{
		"user1": false, "user2": false,
	})
	return nil
}

// CancelGame aborts a pre-start game. Both players receive `canceled` and the
// game row is deleted.
func (s *Service) CancelGame(ctx context.Context, gameID, userID string) error {
	game, err := s.Get(ctx, gameID)
	if err != nil {
		return err
	}
	if game.IsStarted {
		return ErrAlreadyStarted
	}
	if !isPlayer(game, userID) {
		return ErrNotPlayer
	}
	if err := s.repo.Delete(gameID); err != nil {
		return err
	}
	s.publish(gameID, "canceled", map[string]any{"userId": userID})
	return nil
}

// --- Start ---

// StartGame transitions the game to in-progress state. color 割り当ては
// game.BW ("random" | "1" | "2") に従い、Redis に最初のターンタイマーをセット
// してから `started` イベントを publish する。
func (s *Service) StartGame(ctx context.Context, game *model.ReversiGame) error {
	if game.IsStarted {
		return ErrAlreadyStarted
	}
	if game.IsEnded {
		return ErrAlreadyEnded
	}

	black := pickBlack(game.BW)
	game.Black = &black
	game.IsStarted = true
	now := time.Now()
	game.StartedAt = &now

	if err := s.repo.Update(game); err != nil {
		return err
	}
	// 初期ターンタイマー: logsCount=0 (まだ一手も置かれていない)
	s.setTurnTimer(ctx, game.ID, 0, game.TimeLimitForEachTurn)
	s.publish(game.ID, "started", map[string]any{"game": packGame(game)})
	return nil
}

// --- In-game actions ---

// PutStone validates and applies a move, publishes the log, and ends the game
// if the engine reports terminal state.
func (s *Service) PutStone(ctx context.Context, gameID, userID string, pos int) error {
	game, err := s.Get(ctx, gameID)
	if err != nil {
		return err
	}
	if !game.IsStarted {
		return ErrNotStarted
	}
	if game.IsEnded {
		return ErrAlreadyEnded
	}
	if !isPlayer(game, userID) {
		return ErrNotPlayer
	}
	engine, err := EngineFromGame(game)
	if err != nil {
		return err
	}
	myColor, err := playerColor(game, userID)
	if err != nil {
		return err
	}
	if engine.Turn == nil || *engine.Turn != myColor {
		return ErrNotYourTurn
	}
	if !engine.CanPut(myColor, pos) {
		return ErrInvalidMove
	}
	engine.PutStone(pos)

	// logs: [[pos, color], ...]
	var logs [][]int
	if len(game.Logs) > 0 {
		_ = json.Unmarshal(game.Logs, &logs)
	}
	colorInt := 1
	if !myColor {
		colorInt = 2
	}
	logs = append(logs, []int{pos, colorInt})
	raw, _ := json.Marshal(logs)
	game.Logs = raw

	if engine.Turn == nil {
		// ゲーム終了
		if err := s.finalizeGame(ctx, game, engine); err != nil {
			return err
		}
	} else {
		if err := s.repo.Update(game); err != nil {
			return err
		}
		s.setTurnTimer(ctx, game.ID, len(logs), game.TimeLimitForEachTurn)
	}

	s.publish(gameID, "log", map[string]any{
		"pos":       pos,
		"color":     colorInt,
		"player":    myColor,
		"time":      time.Now().UnixMilli(),
		"operation": "put",
	})
	if engine.Turn == nil {
		s.publish(gameID, "ended", map[string]any{
			"winnerId": game.WinnerID,
			"game":     packGame(game),
		})
	}
	return nil
}

// Surrender marks the opposing user as the winner and ends the game.
func (s *Service) Surrender(ctx context.Context, gameID, userID string) error {
	game, err := s.Get(ctx, gameID)
	if err != nil {
		return err
	}
	// PutStone 等と同じ順序でガードする。未スタートの対局に対する surrender は
	// 「勝ち逃げ」と区別が付かなくなるので、開始前は拒否する。
	if !game.IsStarted {
		return ErrNotStarted
	}
	if game.IsEnded {
		return ErrAlreadyEnded
	}
	if !isPlayer(game, userID) {
		return ErrNotPlayer
	}
	winnerID := opponent(game, userID)
	now := time.Now()
	game.IsEnded = true
	game.EndedAt = &now
	game.WinnerID = &winnerID
	game.SurrenderedUserID = &userID
	if err := s.repo.Update(game); err != nil {
		return err
	}
	s.publish(gameID, "ended", map[string]any{
		"winnerId": winnerID,
		"reason":   "surrender",
		"game":     packGame(game),
	})
	return nil
}

// CheckTimeout is invoked when a client claims the opponent's turn timer has
// expired. If the matching Redis key is gone, the opponent wins by timeout.
func (s *Service) CheckTimeout(ctx context.Context, gameID string) error {
	game, err := s.Get(ctx, gameID)
	if err != nil {
		return err
	}
	if !game.IsStarted || game.IsEnded {
		return nil
	}
	var logs [][]int
	if len(game.Logs) > 0 {
		_ = json.Unmarshal(game.Logs, &logs)
	}
	exists, err := s.turnTimerExists(ctx, gameID, len(logs))
	if err != nil {
		return err
	}
	if exists {
		// タイマー未期限: タイムアウト主張は無効。
		return nil
	}
	// タイムアウト発生: 現在のターンプレイヤー (logs 末尾 + 1) を敗者にする。
	engine, err := EngineFromGame(game)
	if err != nil {
		return err
	}
	if engine.Turn == nil {
		return nil
	}
	loserID := playerIDForColor(game, *engine.Turn)
	if loserID == "" {
		return nil
	}
	winnerID := opponent(game, loserID)
	now := time.Now()
	game.IsEnded = true
	game.EndedAt = &now
	game.WinnerID = &winnerID
	game.TimeoutUserID = &loserID
	if err := s.repo.Update(game); err != nil {
		return err
	}
	s.publish(gameID, "ended", map[string]any{
		"winnerId": winnerID,
		"reason":   "timeout",
		"game":     packGame(game),
	})
	return nil
}

// --- helpers ---

func (s *Service) publish(gameID, eventType string, body any) {
	if s.publisher == nil {
		return
	}
	s.publisher.PublishGameEvent(gameID, eventType, body)
}

func (s *Service) setTurnTimer(ctx context.Context, gameID string, turnIndex, seconds int) {
	if s.redis == nil || seconds <= 0 {
		return
	}
	key := turnTimerKey(gameID, turnIndex)
	s.redis.Set(ctx, key, "1", time.Duration(seconds)*time.Second)
}

func (s *Service) turnTimerExists(ctx context.Context, gameID string, turnIndex int) (bool, error) {
	if s.redis == nil {
		return true, nil
	}
	key := turnTimerKey(gameID, turnIndex)
	n, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func turnTimerKey(gameID string, turnIndex int) string {
	return fmt.Sprintf("reversi:game:turnTimer:%s:%d", gameID, turnIndex)
}

func (s *Service) finalizeGame(_ context.Context, game *model.ReversiGame, engine *Game) error {
	now := time.Now()
	game.IsEnded = true
	game.EndedAt = &now
	crc := strconv.FormatUint(uint64(engine.CalcCRC32()), 10)
	game.CRC32 = &crc
	if winnerColor := engine.Winner(); winnerColor != nil {
		winnerID := playerIDForColor(game, *winnerColor)
		if winnerID != "" {
			game.WinnerID = &winnerID
		}
	}
	return s.repo.Update(game)
}

// isPlayer returns true when userID matches one of the game's two players.
func isPlayer(game *model.ReversiGame, userID string) bool {
	return game.User1ID == userID || game.User2ID == userID
}

// opponent returns the other player's id given userID.
func opponent(game *model.ReversiGame, userID string) string {
	if game.User1ID == userID {
		return game.User2ID
	}
	return game.User1ID
}

// playerColor computes a user's stone color from game.Black (1 | 2).
func playerColor(game *model.ReversiGame, userID string) (Color, error) {
	if game.Black == nil {
		return false, errors.New("game.Black not set")
	}
	user1IsBlack := *game.Black == 1
	switch userID {
	case game.User1ID:
		return colorFromBool(user1IsBlack), nil
	case game.User2ID:
		return colorFromBool(!user1IsBlack), nil
	}
	return false, ErrNotPlayer
}

// playerIDForColor is the inverse of playerColor — returns the user id that
// plays the given color.
func playerIDForColor(game *model.ReversiGame, color Color) string {
	if game.Black == nil {
		return ""
	}
	user1IsBlack := *game.Black == 1
	if color == Black {
		if user1IsBlack {
			return game.User1ID
		}
		return game.User2ID
	}
	if user1IsBlack {
		return game.User2ID
	}
	return game.User1ID
}

func colorFromBool(black bool) Color {
	if black {
		return Black
	}
	return White
}

// pickBlack returns 1 or 2 per the BW setting, defaulting to random.
func pickBlack(bw string) int {
	switch bw {
	case "1":
		return 1
	case "2":
		return 2
	}
	// "random" など
	//nolint:gosec // G404 — UX randomness, non-cryptographic is fine
	if rand.Intn(2) == 0 {
		return 1
	}
	return 2
}

// applySetting writes the typed raw JSON value into the matching game column.
func applySetting(game *model.ReversiGame, key string, raw json.RawMessage) error {
	switch key {
	case "map":
		var m []string
		if err := json.Unmarshal(raw, &m); err != nil {
			return ErrInvalidSetting
		}
		game.Map = m
	case "bw":
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return ErrInvalidSetting
		}
		game.BW = fmt.Sprint(v)
	case "isLlotheo":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return ErrInvalidSetting
		}
		game.IsLlotheo = b
	case "canPutEverywhere":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return ErrInvalidSetting
		}
		game.CanPutEverywhere = b
	case "loopedBoard":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return ErrInvalidSetting
		}
		game.LoopedBoard = b
	case "timeLimitForEachTurn":
		var n int
		if err := json.Unmarshal(raw, &n); err != nil {
			return ErrInvalidSetting
		}
		if n < 0 {
			return ErrInvalidSetting
		}
		game.TimeLimitForEachTurn = n
	case "noIrregularRules":
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return ErrInvalidSetting
		}
		game.NoIrregularRules = b
	default:
		return ErrInvalidSetting
	}
	return nil
}

// EngineFromGame restores a reversi.Game engine from the persisted state by
// replaying the move log against the original map.
func EngineFromGame(game *model.ReversiGame) (*Game, error) {
	opts := Options{
		IsLlotheo:        game.IsLlotheo,
		CanPutEverywhere: game.CanPutEverywhere,
		LoopedBoard:      game.LoopedBoard,
	}
	engine := NewGame([]string(game.Map), opts)
	if len(game.Logs) == 0 {
		return engine, nil
	}
	var logs [][]int
	if err := json.Unmarshal(game.Logs, &logs); err != nil {
		return nil, fmt.Errorf("decode logs: %w", err)
	}
	for _, entry := range logs {
		if len(entry) >= 1 {
			engine.PutStone(entry[0])
		}
	}
	return engine, nil
}

// packGame is a minimal JSON projection used by event bodies. 詳細ペイロード
// は api ハンドラ側の packGame を使うため、ここでは stream 配信に必要な最小の
// フィールドのみ含める。
func packGame(game *model.ReversiGame) map[string]any {
	return map[string]any{
		"id":                   game.ID,
		"user1Id":              game.User1ID,
		"user2Id":              game.User2ID,
		"user1Ready":           game.User1Ready,
		"user2Ready":           game.User2Ready,
		"black":                game.Black,
		"isStarted":            game.IsStarted,
		"isEnded":              game.IsEnded,
		"winnerId":             game.WinnerID,
		"surrenderedUserId":    game.SurrenderedUserID,
		"timeoutUserId":        game.TimeoutUserID,
		"timeLimitForEachTurn": game.TimeLimitForEachTurn,
		"logs":                 game.Logs,
		"map":                  game.Map,
		"bw":                   game.BW,
		"isLlotheo":            game.IsLlotheo,
		"canPutEverywhere":     game.CanPutEverywhere,
		"loopedBoard":          game.LoopedBoard,
		"noIrregularRules":     game.NoIrregularRules,
		"startedAt":            game.StartedAt,
		"endedAt":              game.EndedAt,
	}
}
