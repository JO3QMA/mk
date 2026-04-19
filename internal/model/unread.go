package model

// AntennaNoteUnread represents the `antenna_note_unread` table tracking
// which antenna notes a user has not yet read. One row per (user, antenna,
// note) triple; deleted when the user marks the antenna as read.
type AntennaNoteUnread struct {
	ID        string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	AntennaID string `gorm:"column:antennaId;type:varchar(32);not null" json:"antennaId"`
	NoteID    string `gorm:"column:noteId;type:varchar(32);not null" json:"noteId"`
	UserID    string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
}

// TableName overrides GORM's default pluralisation.
func (AntennaNoteUnread) TableName() string { return "antenna_note_unread" }

// ChannelNoteUnread represents the `channel_note_unread` table tracking
// which channel notes a follower has not yet read. One row per (user,
// channel, note) triple; deleted when the user reads the channel timeline.
type ChannelNoteUnread struct {
	ID        string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	ChannelID string `gorm:"column:channelId;type:varchar(32);not null" json:"channelId"`
	NoteID    string `gorm:"column:noteId;type:varchar(32);not null" json:"noteId"`
	UserID    string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
}

// TableName overrides GORM's default pluralisation.
func (ChannelNoteUnread) TableName() string { return "channel_note_unread" }
