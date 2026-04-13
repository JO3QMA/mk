package model

// ChannelFavorite represents the `channel_favorite` table.
type ChannelFavorite struct {
	ID        string `gorm:"column:id;type:varchar(32);primaryKey"`
	UserID    string `gorm:"column:userId;type:varchar(32);not null"`
	ChannelID string `gorm:"column:channelId;type:varchar(32);not null"`
}

func (ChannelFavorite) TableName() string { return "channel_favorite" }
