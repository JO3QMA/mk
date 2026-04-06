package model

// UserKeypair represents the `user_keypair` table.
type UserKeypair struct {
	UserID     string `gorm:"column:userId;type:varchar(32);primaryKey" json:"userId"`
	PublicKey  string `gorm:"column:publicKey;type:varchar(4096);not null" json:"publicKey"`
	PrivateKey string `gorm:"column:privateKey;type:varchar(4096);not null" json:"-"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (UserKeypair) TableName() string { return "user_keypair" }
