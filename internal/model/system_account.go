package model

// SystemAccount represents the `system_account` table.
// Maps a system account type (e.g. "actor", "relay", "proxy") to a user record.
type SystemAccount struct {
	ID     string `gorm:"column:id;type:varchar(32);primaryKey"`
	UserID string `gorm:"column:userId;type:varchar(32);not null"`
	Type   string `gorm:"column:type;type:varchar(256);not null;uniqueIndex"`
}

func (SystemAccount) TableName() string { return "system_account" }
