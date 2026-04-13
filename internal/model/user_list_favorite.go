package model

// UserListFavorite represents the `user_list_favorite` table.
type UserListFavorite struct {
	ID         string `gorm:"column:id;type:varchar(32);primaryKey"`
	UserID     string `gorm:"column:userId;type:varchar(32);not null"`
	UserListID string `gorm:"column:userListId;type:varchar(32);not null"`
}

func (UserListFavorite) TableName() string { return "user_list_favorite" }
