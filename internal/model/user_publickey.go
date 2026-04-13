package model

// UserPublickey stores the public key of a remote user for HTTP Signature
// verification. TS版の user_publickey テーブルに対応する。ローカルユーザーの
// 鍵ペア (user_keypair) とは別。
type UserPublickey struct {
	UserID string `gorm:"column:userId;type:varchar(32);primaryKey"`
	KeyID  string `gorm:"column:keyId;type:varchar(256);not null"`
	KeyPEM string `gorm:"column:keyPem;type:varchar(4096);not null"`
}

// TableName returns the database table name.
func (UserPublickey) TableName() string { return "user_publickey" }
