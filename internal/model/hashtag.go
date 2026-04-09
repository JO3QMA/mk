package model

import "github.com/lib/pq"

// Hashtag represents the `hashtag` table.
type Hashtag struct {
	ID                        string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	Name                      string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	MentionedUsersCount       int            `gorm:"column:mentionedUsersCount;default:0" json:"mentionedUsersCount"`
	MentionedLocalUsersCount  int            `gorm:"column:mentionedLocalUsersCount;default:0" json:"mentionedLocalUsersCount"`
	MentionedRemoteUsersCount int            `gorm:"column:mentionedRemoteUsersCount;default:0" json:"mentionedRemoteUsersCount"`
	AttachedUsersCount        int            `gorm:"column:attachedUsersCount;default:0" json:"attachedUsersCount"`
	AttachedLocalUsersCount   int            `gorm:"column:attachedLocalUsersCount;default:0" json:"attachedLocalUsersCount"`
	AttachedRemoteUsersCount  int            `gorm:"column:attachedRemoteUsersCount;default:0" json:"attachedRemoteUsersCount"`
	MentionedUserIds          pq.StringArray `gorm:"column:mentionedUserIds;type:varchar(32)[];default:'{}'" json:"-"`
	MentionedLocalUserIds     pq.StringArray `gorm:"column:mentionedLocalUserIds;type:varchar(32)[];default:'{}'" json:"-"`
	MentionedRemoteUserIds    pq.StringArray `gorm:"column:mentionedRemoteUserIds;type:varchar(32)[];default:'{}'" json:"-"`
	AttachedUserIds           pq.StringArray `gorm:"column:attachedUserIds;type:varchar(32)[];default:'{}'" json:"-"`
	AttachedLocalUserIds      pq.StringArray `gorm:"column:attachedLocalUserIds;type:varchar(32)[];default:'{}'" json:"-"`
	AttachedRemoteUserIds     pq.StringArray `gorm:"column:attachedRemoteUserIds;type:varchar(32)[];default:'{}'" json:"-"`
}

func (Hashtag) TableName() string { return "hashtag" }
