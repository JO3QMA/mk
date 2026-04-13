package timeline

import "github.com/shiroha-a/mk/internal/model"

// TimelineFilter holds filtering options for timeline queries.
// *bool フィールドは nil のときデフォルト値として扱う。
type TimelineFilter struct {
	WithFiles             bool  // trueならファイル付きノートのみ
	WithRenotes           *bool // nil=true。falseでpure renote除外
	WithReplies           *bool // nil=false。local/hybridのみ
	IncludeMyRenotes      *bool // nil=true。home/hybridのみ
	IncludeRenotedMyNotes *bool // nil=true。home/hybridのみ
	IncludeLocalRenotes   *bool // nil=true。home/hybridのみ
	AllowPartial          bool  // trueならRedis結果が不足でもDBフォールバックしない
}

// boolDefault returns *b if non-nil, else def.
func boolDefault(b *bool, def bool) bool {
	if b != nil {
		return *b
	}
	return def
}

// isPureRenote returns true if the note is a pure renote (no text, no files).
func isPureRenote(n *model.Note) bool {
	return n.RenoteID != nil && n.Text == nil && len(n.FileIDs) == 0
}

// ApplyFilter filters notes in-memory according to the given TimelineFilter.
// viewerID is the currently authenticated user's ID (empty string if anonymous).
func ApplyFilter(notes []*model.Note, viewerID string, f TimelineFilter) []*model.Note {
	withRenotes := boolDefault(f.WithRenotes, true)
	withReplies := boolDefault(f.WithReplies, false)
	includeMyRenotes := boolDefault(f.IncludeMyRenotes, true)
	includeRenotedMyNotes := boolDefault(f.IncludeRenotedMyNotes, true)
	includeLocalRenotes := boolDefault(f.IncludeLocalRenotes, true)

	out := make([]*model.Note, 0, len(notes))
	for _, n := range notes {
		if f.WithFiles && len(n.FileIDs) == 0 {
			continue
		}
		if !withRenotes && isPureRenote(n) {
			continue
		}
		// withReplies=false: 他人への返信を除外 (自分への返信は残す)
		if !withReplies && n.ReplyID != nil {
			if viewerID == "" || (n.ReplyUserID != nil && *n.ReplyUserID != viewerID) {
				continue
			}
		}
		if isPureRenote(n) {
			// includeMyRenotes=false: 自分がした pure renote を除外
			if !includeMyRenotes && viewerID != "" && n.UserID == viewerID {
				continue
			}
			// includeRenotedMyNotes=false: 自分のノートの pure renote を除外
			if !includeRenotedMyNotes && viewerID != "" && n.RenoteUserID != nil && *n.RenoteUserID == viewerID {
				continue
			}
			// includeLocalRenotes=false: ローカルユーザーの pure renote を除外
			if !includeLocalRenotes && n.RenoteUserHost == nil {
				continue
			}
		}
		out = append(out, n)
	}
	return out
}
