package model

import "time"

// UserIP represents the `user_ip` table.
// sign-in 成功時に IP アドレスを記録する。enableIpLogging が true のときのみ使用。
type UserIP struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"column:createdAt;autoCreateTime" json:"createdAt"`
	UserID    string    `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	IP        string    `gorm:"column:ip;type:varchar(128);not null" json:"ip"`
}

func (UserIP) TableName() string { return "user_ip" }
