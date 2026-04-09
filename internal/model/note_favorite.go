package model

// NoteFavorite represents the `note_favorite` table.
type NoteFavorite struct {
	ID     string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	NoteID string `gorm:"column:noteId;type:varchar(32);not null" json:"noteId"`

	Note *Note `gorm:"foreignKey:NoteID" json:"note,omitempty"`
}

func (NoteFavorite) TableName() string { return "note_favorite" }
