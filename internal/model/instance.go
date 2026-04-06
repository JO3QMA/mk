package model

import "time"

// SuspensionState represents the suspension state of a remote instance.
type SuspensionState string

const (
	SuspensionStateNone                          SuspensionState = "none"
	SuspensionStateManuallySuspended             SuspensionState = "manuallySuspended"
	SuspensionStateGoneSuspended                 SuspensionState = "goneSuspended"
	SuspensionStateAutoSuspendedForNotResponding SuspensionState = "autoSuspendedForNotResponding"
)

// Instance represents the `instance` table.
type Instance struct {
	ID                      string          `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	FirstRetrievedAt        time.Time       `gorm:"column:firstRetrievedAt;type:timestamp with time zone;not null" json:"firstRetrievedAt"`
	Host                    string          `gorm:"column:host;type:varchar(128);uniqueIndex;not null" json:"host"`
	UsersCount              int             `gorm:"column:usersCount;default:0" json:"usersCount"`
	NotesCount              int             `gorm:"column:notesCount;default:0" json:"notesCount"`
	FollowingCount          int             `gorm:"column:followingCount;default:0" json:"followingCount"`
	FollowersCount          int             `gorm:"column:followersCount;default:0" json:"followersCount"`
	LatestRequestReceivedAt *time.Time      `gorm:"column:latestRequestReceivedAt;type:timestamp with time zone" json:"latestRequestReceivedAt"`
	IsNotResponding         bool            `gorm:"column:isNotResponding;default:false" json:"isNotResponding"`
	NotRespondingSince      *time.Time      `gorm:"column:notRespondingSince;type:timestamp with time zone" json:"notRespondingSince"`
	SuspensionState         SuspensionState `gorm:"column:suspensionState;type:varchar(64);default:'none'" json:"suspensionState"`
	SoftwareName            *string         `gorm:"column:softwareName;type:varchar(64)" json:"softwareName"`
	SoftwareVersion         *string         `gorm:"column:softwareVersion;type:varchar(64)" json:"softwareVersion"`
	OpenRegistrations       *bool           `gorm:"column:openRegistrations" json:"openRegistrations"`
	Name                    *string         `gorm:"column:name;type:varchar(256)" json:"name"`
	Description             *string         `gorm:"column:description;type:varchar(4096)" json:"description"`
	MaintainerName          *string         `gorm:"column:maintainerName;type:varchar(128)" json:"maintainerName"`
	MaintainerEmail         *string         `gorm:"column:maintainerEmail;type:varchar(256)" json:"maintainerEmail"`
	IconURL                 *string         `gorm:"column:iconUrl;type:varchar(256)" json:"iconUrl"`
	FaviconURL              *string         `gorm:"column:faviconUrl;type:varchar(256)" json:"faviconUrl"`
	ThemeColor              *string         `gorm:"column:themeColor;type:varchar(64)" json:"themeColor"`
	InfoUpdatedAt           *time.Time      `gorm:"column:infoUpdatedAt;type:timestamp with time zone" json:"infoUpdatedAt"`
	ModerationNote          string          `gorm:"column:moderationNote;type:varchar(16384);default:''" json:"moderationNote"`
}

func (Instance) TableName() string { return "instance" }
