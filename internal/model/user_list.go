package model

// UserList represents the `user_list` table.
type UserList struct {
	ID       string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserID   string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`
	Name     string `gorm:"column:name;type:varchar(128);not null" json:"name"`
	IsPublic bool   `gorm:"column:isPublic;default:false" json:"isPublic"`
}

func (UserList) TableName() string { return "user_list" }

// UserListMembership represents the `user_list_membership` table.
type UserListMembership struct {
	ID         string `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UserListID string `gorm:"column:userListId;type:varchar(32);not null" json:"userListId"`
	UserID     string `gorm:"column:userId;type:varchar(32);not null" json:"userId"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (UserListMembership) TableName() string { return "user_list_membership" }
