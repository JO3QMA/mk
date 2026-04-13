package model

// PasswordResetRequest represents the `password_reset_request` table.
type PasswordResetRequest struct {
	ID     string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	Token  string `gorm:"column:token;type:varchar(256);not null" json:"token"`
	UserID string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
}

func (PasswordResetRequest) TableName() string { return "password_reset_request" }
