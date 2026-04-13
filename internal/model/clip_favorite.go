package model

// ClipFavorite represents the `clip_favorite` table.
type ClipFavorite struct {
	ID     string `gorm:"column:id;type:varchar(32);primaryKey"`
	UserID string `gorm:"column:userId;type:varchar(32);not null"`
	ClipID string `gorm:"column:clipId;type:varchar(32);not null"`
}

func (ClipFavorite) TableName() string { return "clip_favorite" }
