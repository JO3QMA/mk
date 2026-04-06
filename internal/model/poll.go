package model

import (
	"time"

	"github.com/lib/pq"
)

// Poll represents the `poll` table.
type Poll struct {
	NoteID         string         `gorm:"column:noteId;type:varchar(32);primaryKey" json:"noteId"`
	ExpiresAt      *time.Time     `gorm:"column:expiresAt;type:timestamp with time zone" json:"expiresAt"`
	Multiple       bool           `gorm:"column:multiple;not null" json:"multiple"`
	Choices        pq.StringArray `gorm:"column:choices;type:varchar(256)[];default:'{}'" json:"choices"`
	Votes          pq.Int64Array  `gorm:"column:votes;type:integer[]" json:"votes"`
	NoteVisibility NoteVisibility `gorm:"column:noteVisibility;type:note_visibility_enum" json:"noteVisibility"`
	// Denormalized fields
	UserID    string  `gorm:"column:userId;type:varchar(32)" json:"userId"`
	UserHost  *string `gorm:"column:userHost;type:varchar(128)" json:"userHost"`
	ChannelID *string `gorm:"column:channelId;type:varchar(32)" json:"channelId"`

	// Relations
	Note *Note `gorm:"foreignKey:NoteID" json:"note,omitempty"`
}

func (Poll) TableName() string { return "poll" }
