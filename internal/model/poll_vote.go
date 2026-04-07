package model

import "time"

// PollVote represents the `poll_vote` table.
type PollVote struct {
	ID        string    `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID    string    `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	NoteID    string    `gorm:"column:noteId;type:varchar(32);not null" json:"noteId"`
	Choice    int       `gorm:"column:choice;type:integer;not null" json:"choice"`
	CreatedAt time.Time `gorm:"column:createdAt;type:timestamp with time zone;not null" json:"createdAt"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Note *Note `gorm:"foreignKey:NoteID" json:"note,omitempty"`
}

func (PollVote) TableName() string { return "poll_vote" }
