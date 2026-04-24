package reversi

import (
	"strings"

	"github.com/google/uuid"
	"github.com/shiroha-a/mk/internal/model"
)

// PendingGameLookup is the minimal interface for finding a pending reversi
// game between two users. repository.ReversiRepository がそのまま満たす。
type PendingGameLookup interface {
	ListByUser(userID string, limit int) ([]*model.ReversiGame, error)
}

// FindPendingInvitation returns the latest pending (not started / ended)
// reversi_game row where User1=inviterID and User2=inviteeID. 招待の受諾
// (/api/reversi/match) 経路と、連合経路 (handleReversiInvite の 5 秒再送
// dedup) の両方から使う共有ヘルパー。ListByUser は id DESC なので最初に
// ヒットしたものを返すだけで最新が取れる。
func FindPendingInvitation(repo PendingGameLookup, inviteeID, inviterID string) *model.ReversiGame {
	games, err := repo.ListByUser(inviteeID, 20)
	if err != nil {
		return nil
	}
	for _, g := range games {
		if g.IsStarted || g.IsEnded {
			continue
		}
		if g.User1ID == inviterID && g.User2ID == inviteeID {
			return g
		}
	}
	return nil
}

// GameTypeUUID is the unique identifier for the reversi game type in ActivityPub.
// CherryPick互換の固定UUID。
const GameTypeUUID = "1c086295-25e3-4b82-b31e-3e3959906312"

// APGame represents an ActivityPub Game object for reversi.
type APGame struct {
	Type         string      `json:"type"`
	GameTypeUUID string      `json:"game_type_uuid"`
	ExtentFlags  []string    `json:"extent_flags"`
	GameState    APGameState `json:"game_state"`
}

// APGameState represents the state payload within an APGame.
type APGameState struct {
	GameSessionID string `json:"game_session_id"`
	Type          string `json:"type,omitempty"`  // settings, ready_states, putstone
	Key           string `json:"key,omitempty"`   // 設定変更キー
	Value         any    `json:"value,omitempty"` // 設定変更値
	Ready         *bool  `json:"ready,omitempty"` // 準備完了フラグ
	Pos           *int   `json:"pos,omitempty"`   // 石配置位置
}

// NewAPGame creates a new APGame with the given session ID.
func NewAPGame(sessionID string) APGame {
	return APGame{
		Type:         "Game",
		GameTypeUUID: GameTypeUUID,
		ExtentFlags:  []string{},
		GameState: APGameState{
			GameSessionID: sessionID,
		},
	}
}

// APActivity represents a generic ActivityPub activity for reversi.
type APActivity struct {
	Context   any      `json:"@context,omitempty"`
	ID        string   `json:"id,omitempty"`
	Type      string   `json:"type"`
	Actor     string   `json:"actor"`
	Object    any      `json:"object"`
	To        string   `json:"to,omitempty"`
	CC        []string `json:"cc,omitempty"`
	Published string   `json:"published,omitempty"`
}

// defaultContext は activitypub パッケージが activity 一般に付けるのと同じ
// JSON-LD @context。CherryPick の inbox はこれが無い reversi activity を
// 弾く可能性があるため Render 時に必ず埋める (#417 P1)。
var defaultContext = []any{
	"https://www.w3.org/ns/activitystreams",
	"https://w3id.org/security/v1",
	map[string]any{
		"Key":               "sec:Key",
		"sensitive":         "as:sensitive",
		"Hashtag":           "as:Hashtag",
		"quoteUrl":          "as:quoteUrl",
		"toot":              "http://joinmastodon.org/ns#",
		"Emoji":             "toot:Emoji",
		"featured":          "toot:featured",
		"discoverable":      "toot:discoverable",
		"schema":            "http://schema.org#",
		"PropertyValue":     "schema:PropertyValue",
		"value":             "schema:value",
		"misskey":           "https://misskey-hub.net/ns#",
		"_misskey_content":  "misskey:_misskey_content",
		"_misskey_quote":    "misskey:_misskey_quote",
		"_misskey_reaction": "misskey:_misskey_reaction",
		"_misskey_votes":    "misskey:_misskey_votes",
		"isCat":             "misskey:isCat",
	},
}

// RenderInvite creates an Invite activity for a reversi game.
func RenderInvite(baseURL, sessionID, actorURI, targetURI string, published string) APActivity {
	game := NewAPGame(sessionID)
	return APActivity{
		Context:   defaultContext,
		ID:        baseURL + "/games/" + GameTypeUUID + "/" + sessionID + "/activity",
		Type:      "Invite",
		Actor:     actorURI,
		Object:    game,
		To:        targetURI,
		CC:        []string{},
		Published: published,
	}
}

// RenderJoin creates a Join activity for accepting a reversi invitation.
func RenderJoin(baseURL, sessionID, actorURI, targetURI string, published string) APActivity {
	game := NewAPGame(sessionID)
	return APActivity{
		Context:   defaultContext,
		ID:        baseURL + "/games/" + GameTypeUUID + "/" + sessionID + "/activity",
		Type:      "Join",
		Actor:     actorURI,
		Object:    game,
		To:        targetURI,
		CC:        []string{},
		Published: published,
	}
}

// activityID builds a per-activity id so that CherryPick's
// InboxProcessorService does not silent-drop the payload. 同じ actor が複数
// 回 Update/Leave/Undo を送るので session id ベースの決定的 id は衝突する。
// 本家 addContext と同じくランダム UUID ベースにする。
func activityID(baseURL string) string {
	return baseURL + "/activities/" + uuid.NewString()
}

// RenderUpdate creates an Update activity for game state changes.
// baseURL はローカルインスタンスのベース URL (id フィールド生成用)。
func RenderUpdate(baseURL, actorURI, targetURI string, state APGameState) APActivity {
	game := APGame{
		Type:         "Game",
		GameTypeUUID: GameTypeUUID,
		ExtentFlags:  []string{},
		GameState:    state,
	}
	return APActivity{
		Context: defaultContext,
		ID:      activityID(baseURL),
		Type:    "Update",
		Actor:   actorURI,
		Object:  game,
		To:      targetURI,
		CC:      []string{},
	}
}

// RenderLeave creates a Leave activity for surrendering or cancelling.
func RenderLeave(baseURL, actorURI, targetURI, sessionID string) APActivity {
	game := NewAPGame(sessionID)
	return APActivity{
		Context: defaultContext,
		ID:      activityID(baseURL),
		Type:    "Leave",
		Actor:   actorURI,
		Object:  game,
		To:      targetURI,
		CC:      []string{},
	}
}

// RenderUndo creates an Undo activity wrapping another activity.
func RenderUndo(baseURL, actorURI string, original APActivity) APActivity {
	return APActivity{
		Context: defaultContext,
		ID:      activityID(baseURL),
		Type:    "Undo",
		Actor:   actorURI,
		Object:  original,
	}
}

// APReactionActivity represents an EmojiReaction activity sent against a
// reversi game. Object is a URI string (game session) per CherryPick spec,
// not the nested Game object used by Invite/Update/Leave/Undo.
//
// 別 struct にしているのは Object フィールドの型が違う (URI string) のと
// `_misskey_reaction` / `tag` 等の reversi reaction 専用フィールドを持つため。
type APReactionActivity struct {
	Context         any      `json:"@context,omitempty"`
	ID              string   `json:"id,omitempty"`
	Type            string   `json:"type"`
	Actor           string   `json:"actor"`
	Object          string   `json:"object"`
	Content         string   `json:"content,omitempty"`
	MisskeyReaction string   `json:"_misskey_reaction,omitempty"`
	To              string   `json:"to,omitempty"`
	CC              []string `json:"cc,omitempty"`
}

// GameSessionURI builds the canonical URI used by CherryPick to identify a
// reversi game session in EmojiReaction activities. CherryPick の
// `/games/{UUID}/{sessionID}` パターンをそのまま生成する。inbox 側の
// ParseGameSessionURI と対称。
func GameSessionURI(baseURL, sessionID string) string {
	return baseURL + "/games/" + GameTypeUUID + "/" + sessionID
}

// ParseGameSessionURI extracts the reversi session id from a CherryPick-style
// `/games/{UUID}/{sessionID}` URI. URI が reversi game セッションの形でなけ
// れば空文字を返す (= 通常 Like / EmojiReaction として処理させる)。
func ParseGameSessionURI(uri string) string {
	const marker = "/games/" + GameTypeUUID + "/"
	idx := strings.Index(uri, marker)
	if idx < 0 {
		return ""
	}
	return uri[idx+len(marker):]
}

// RenderReversiReaction builds an EmojiReaction activity addressed to the
// remote opponent for a live reversi game session. CherryPick はこれを
// `reacted` WS event の federation に使う (#417 P5)。
//
//	emoji  : `:name:` 形式 (custom) か Unicode 1 文字 (default)
//	tags   : custom emoji の場合の `tag` 配列 (省略可)。プロトコル上必須では
//	         ない (受け側はローカルにある絵文字定義で fallback できる)。
func RenderReversiReaction(baseURL, sessionID, actorURI, targetURI, emoji string) APReactionActivity {
	return APReactionActivity{
		Context:         defaultContext,
		ID:              activityID(baseURL),
		Type:            "EmojiReaction",
		Actor:           actorURI,
		Object:          GameSessionURI(baseURL, sessionID),
		Content:         emoji,
		MisskeyReaction: emoji,
		To:              targetURI,
		CC:              []string{},
	}
}

// IsReversiGame checks if an object is a reversi Game by UUID.
func IsReversiGame(obj map[string]any) bool {
	if obj["type"] != "Game" {
		return false
	}
	return obj["game_type_uuid"] == GameTypeUUID
}

// ParseGameState extracts the game state from an AP Game object.
func ParseGameState(obj map[string]any) *APGameState {
	stateRaw, ok := obj["game_state"].(map[string]any)
	if !ok {
		return nil
	}
	state := &APGameState{}
	if v, ok := stateRaw["game_session_id"].(string); ok {
		state.GameSessionID = v
	}
	if v, ok := stateRaw["type"].(string); ok {
		state.Type = v
	}
	if v, ok := stateRaw["key"].(string); ok {
		state.Key = v
	}
	if v, ok := stateRaw["value"]; ok {
		state.Value = v
	}
	if v, ok := stateRaw["ready"].(bool); ok {
		state.Ready = &v
	}
	if v, ok := stateRaw["pos"].(float64); ok {
		pos := int(v)
		state.Pos = &pos
	}
	return state
}
