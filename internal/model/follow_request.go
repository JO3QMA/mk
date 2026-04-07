package model

// FollowRequest represents the `follow_request` table.
// Lockedユーザーへのフォロー、またはRemote由来のFollow Activity受信時に作成される
type FollowRequest struct {
	ID          string  `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	FolloweeID  string  `gorm:"column:followeeId;type:varchar(32);not null" json:"followeeId"`
	FollowerID  string  `gorm:"column:followerId;type:varchar(32);not null" json:"followerId"`
	RequestID   *string `gorm:"column:requestId;type:varchar(128)" json:"requestId"`
	WithReplies bool    `gorm:"column:withReplies;default:false" json:"withReplies"`
	// Denormalized fields
	FollowerHost        *string `gorm:"column:followerHost;type:varchar(128)" json:"followerHost"`
	FollowerInbox       *string `gorm:"column:followerInbox;type:varchar(512)" json:"followerInbox"`
	FollowerSharedInbox *string `gorm:"column:followerSharedInbox;type:varchar(512)" json:"followerSharedInbox"`
	FolloweeHost        *string `gorm:"column:followeeHost;type:varchar(128)" json:"followeeHost"`
	FolloweeInbox       *string `gorm:"column:followeeInbox;type:varchar(512)" json:"followeeInbox"`
	FolloweeSharedInbox *string `gorm:"column:followeeSharedInbox;type:varchar(512)" json:"followeeSharedInbox"`

	// Relations
	Followee *User `gorm:"foreignKey:FolloweeID" json:"followee,omitempty"`
	Follower *User `gorm:"foreignKey:FollowerID" json:"follower,omitempty"`
}

func (FollowRequest) TableName() string { return "follow_request" }
