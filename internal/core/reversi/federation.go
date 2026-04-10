package reversi

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
	ID        string   `json:"id,omitempty"`
	Type      string   `json:"type"`
	Actor     string   `json:"actor"`
	Object    any      `json:"object"`
	To        string   `json:"to,omitempty"`
	CC        []string `json:"cc,omitempty"`
	Published string   `json:"published,omitempty"`
}

// RenderInvite creates an Invite activity for a reversi game.
func RenderInvite(baseURL, sessionID, actorURI, targetURI string, published string) APActivity {
	game := NewAPGame(sessionID)
	return APActivity{
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
		ID:        baseURL + "/games/" + GameTypeUUID + "/" + sessionID + "/activity",
		Type:      "Join",
		Actor:     actorURI,
		Object:    game,
		To:        targetURI,
		CC:        []string{},
		Published: published,
	}
}

// RenderUpdate creates an Update activity for game state changes.
func RenderUpdate(actorURI, targetURI string, state APGameState) APActivity {
	game := APGame{
		Type:         "Game",
		GameTypeUUID: GameTypeUUID,
		ExtentFlags:  []string{},
		GameState:    state,
	}
	return APActivity{
		Type:   "Update",
		Actor:  actorURI,
		Object: game,
		To:     targetURI,
		CC:     []string{},
	}
}

// RenderLeave creates a Leave activity for surrendering or cancelling.
func RenderLeave(actorURI, targetURI string, sessionID string) APActivity {
	game := NewAPGame(sessionID)
	return APActivity{
		Type:   "Leave",
		Actor:  actorURI,
		Object: game,
		To:     targetURI,
		CC:     []string{},
	}
}

// RenderUndo creates an Undo activity wrapping another activity.
func RenderUndo(actorURI string, original APActivity) APActivity {
	return APActivity{
		Type:   "Undo",
		Actor:  actorURI,
		Object: original,
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
