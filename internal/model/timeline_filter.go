package model

// TimelineDBFilter holds filtering conditions for timeline DB fallback queries.
// *bool フィールドは nil のときデフォルト値として扱う (WithRenotes nil=true 等)。
type TimelineDBFilter struct {
	WithFiles             bool
	WithRenotes           *bool    // nil=true
	WithReplies           *bool    // nil=false
	IncludeMyRenotes      *bool    // nil=true
	IncludeRenotedMyNotes *bool    // nil=true
	IncludeLocalRenotes   *bool    // nil=true
	ViewerID              string   // home/hybridフィルタで使用
	MutedChannelIDs       []string // 指定があれば channelId が IN (...) のノートを除外
}
