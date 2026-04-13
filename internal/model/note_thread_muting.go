package model

// NoteThreadMuting represents the `note_thread_muting` table.
// Allows users to mute specific note threads.
type NoteThreadMuting struct {
	ID       string `gorm:"column:id;type:varchar(32);primaryKey"`
	UserID   string `gorm:"column:userId;type:varchar(32);not null"`
	ThreadID string `gorm:"column:threadId;type:varchar(256);not null"`
}

func (NoteThreadMuting) TableName() string { return "note_thread_muting" }
