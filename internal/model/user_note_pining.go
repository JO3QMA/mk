package model

// UserNotePining represents the `user_note_pining` table.
// Used by i/pin and i/unpin to track which notes a user has pinned to their profile.
type UserNotePining struct {
	ID     string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	NoteID string `gorm:"column:noteId;type:varchar(32);not null" json:"noteId"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Note *Note `gorm:"foreignKey:NoteID" json:"note,omitempty"`
}

func (UserNotePining) TableName() string { return "user_note_pining" }
