package model

// UserMemo represents the `user_memo` table.
// ユーザーが他のユーザーに対して付ける個人メモ。管理者用ではなく一般ユーザー
// 機能 (show-user / update-memo)。
type UserMemo struct {
	ID           string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID       string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	TargetUserID string `gorm:"column:targetUserId;type:varchar(32);not null" json:"targetUserId"`
	Memo         string `gorm:"column:memo;type:varchar(2048);not null" json:"memo"`
}

func (UserMemo) TableName() string { return "user_memo" }
