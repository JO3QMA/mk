package model

// Blocking represents the `blocking` table.
type Blocking struct {
	ID        string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	BlockerID string `gorm:"column:blockerId;type:varchar(32);not null" json:"blockerId"`
	BlockeeID string `gorm:"column:blockeeId;type:varchar(32);not null" json:"blockeeId"`

	// Relations
	Blocker *User `gorm:"foreignKey:BlockerID" json:"blocker,omitempty"`
	Blockee *User `gorm:"foreignKey:BlockeeID" json:"blockee,omitempty"`
}

func (Blocking) TableName() string { return "blocking" }
