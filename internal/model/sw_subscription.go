package model

// SwSubscription represents the `sw_subscription` table for Web Push.
type SwSubscription struct {
	ID              string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID          string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	Endpoint        string `gorm:"column:endpoint;type:varchar(512);not null" json:"endpoint"`
	Auth            string `gorm:"column:auth;type:varchar(256);not null" json:"auth"`
	PublicKey       string `gorm:"column:publickey;type:varchar(128);not null" json:"publickey"`
	SendReadMessage bool   `gorm:"column:sendReadMessage;default:false" json:"sendReadMessage"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (SwSubscription) TableName() string { return "sw_subscription" }
