package model

import (
	"time"

	"github.com/lib/pq"
)

// Emoji represents the `emoji` table.
type Emoji struct {
	ID                                      string         `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	UpdatedAt                               *time.Time     `gorm:"column:updatedAt;type:timestamp with time zone" json:"updatedAt"`
	Name                                    string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	Host                                    *string        `gorm:"column:host;type:varchar(128)" json:"host"`
	Category                                *string        `gorm:"column:category;type:varchar(128)" json:"category"`
	OriginalURL                             string         `gorm:"column:originalUrl;type:varchar(512);not null" json:"originalUrl"`
	PublicURL                               string         `gorm:"column:publicUrl;type:varchar(512);default:''" json:"publicUrl"`
	URI                                     *string        `gorm:"column:uri;type:varchar(512)" json:"uri"`
	Type                                    *string        `gorm:"column:type;type:varchar(64)" json:"type"`
	Aliases                                 pq.StringArray `gorm:"column:aliases;type:varchar(128)[];default:'{}'" json:"aliases"`
	License                                 *string        `gorm:"column:license;type:varchar(1024)" json:"license"`
	LocalOnly                               bool           `gorm:"column:localOnly;default:false" json:"localOnly"`
	IsSensitive                             bool           `gorm:"column:isSensitive;default:false" json:"isSensitive"`
	RoleIDsThatCanBeUsedThisEmojiAsReaction pq.StringArray `gorm:"column:roleIdsThatCanBeUsedThisEmojiAsReaction;type:varchar(128)[];default:'{}'" json:"roleIdsThatCanBeUsedThisEmojiAsReaction"`
}

func (Emoji) TableName() string { return "emoji" }
