package model

// ChannelMuting represents the `channel_muting` table.
type ChannelMuting struct {
	ID        string `gorm:"column:id;type:varchar(32);primaryKey"`
	UserID    string `gorm:"column:userId;type:varchar(32);not null"`
	ChannelID string `gorm:"column:channelId;type:varchar(32);not null"`
}

func (ChannelMuting) TableName() string { return "channel_muting" }
