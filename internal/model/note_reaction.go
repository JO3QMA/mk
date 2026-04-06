package model

// NoteReaction represents the `note_reaction` table.
type NoteReaction struct {
	ID       string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID   string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	NoteID   string `gorm:"column:noteId;type:varchar(32);not null" json:"noteId"`
	Reaction string `gorm:"column:reaction;type:varchar(260);not null" json:"reaction"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Note *Note `gorm:"foreignKey:NoteID" json:"note,omitempty"`
}

func (NoteReaction) TableName() string { return "note_reaction" }
