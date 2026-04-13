package channels

import "encoding/json"

// noteFilter provides client-controlled filtering for timeline channels.
// タイムラインチャンネルのconnectパラメータで指定されるフィルタ条件。
type noteFilter struct {
	WithRenotes bool `json:"withRenotes"`
	WithReplies bool `json:"withReplies"`
	WithFiles   bool `json:"withFiles"`
}

// defaultNoteFilter returns a filter with default values matching TS behavior.
func defaultNoteFilter() noteFilter {
	return noteFilter{
		WithRenotes: true,
		WithReplies: false,
		WithFiles:   false,
	}
}

// parseNoteFilter parses filter parameters from the connect params JSON.
func parseNoteFilter(params json.RawMessage) noteFilter {
	f := defaultNoteFilter()
	if len(params) > 0 {
		_ = json.Unmarshal(params, &f)
	}
	return f
}

// notePayload is the minimal structure needed for filtering decisions.
type notePayload struct {
	Text     *string  `json:"text"`
	RenoteID *string  `json:"renoteId"`
	ReplyID  *string  `json:"replyId"`
	FileIDs  []string `json:"fileIds"`
}

// shouldEmit returns true if the note passes all filter conditions.
func (f *noteFilter) shouldEmit(payload []byte) bool {
	var note notePayload
	if err := json.Unmarshal(payload, &note); err != nil {
		// パース失敗時はそのまま送信
		return true
	}

	// 純リノート（テキストなし + renoteIdあり）をフィルタ
	if !f.WithRenotes && note.Text == nil && note.RenoteID != nil {
		return false
	}

	// リプライをフィルタ
	if !f.WithReplies && note.ReplyID != nil {
		return false
	}

	// ファイル付きのみモード
	if f.WithFiles && len(note.FileIDs) == 0 {
		return false
	}

	return true
}
