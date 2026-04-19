package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// AntennaNoteUnreadRepository tracks per-user-per-note antenna unreads.
type AntennaNoteUnreadRepository interface {
	Create(m *model.AntennaNoteUnread) error
	// HasAnyByUser reports whether the user has any unread antenna notes.
	// Used by /api/i to populate hasUnreadAntenna.
	HasAnyByUser(userID string) (bool, error)
	// DeleteByAntennaUser removes all unread rows for a given (user, antenna)
	// pair. Used when a user reads an antenna timeline.
	DeleteByAntennaUser(userID, antennaID string) error
}

type antennaNoteUnreadRepository struct{ db *gorm.DB }

// NewAntennaNoteUnreadRepository constructs a repository backed by db.
func NewAntennaNoteUnreadRepository(db *gorm.DB) AntennaNoteUnreadRepository {
	return &antennaNoteUnreadRepository{db: db}
}

func (r *antennaNoteUnreadRepository) Create(m *model.AntennaNoteUnread) error {
	// (userId, antennaId, noteId) の unique 制約を持つので重複はskipして
	// ログにも残さない (best-effort insert)。
	return r.db.Where(`"userId" = ? AND "antennaId" = ? AND "noteId" = ?`, m.UserID, m.AntennaID, m.NoteID).
		FirstOrCreate(m).Error
}

func (r *antennaNoteUnreadRepository) HasAnyByUser(userID string) (bool, error) {
	// SELECT EXISTS(...) は 1 行見つかった時点で short-circuit するため
	// COUNT(*) より効率的 (未読が多いユーザーで効く)。
	var exists bool
	err := r.db.Raw(
		`SELECT EXISTS(SELECT 1 FROM "antenna_note_unread" WHERE "userId" = ?)`,
		userID,
	).Scan(&exists).Error
	return exists, err
}

func (r *antennaNoteUnreadRepository) DeleteByAntennaUser(userID, antennaID string) error {
	return r.db.Where(`"userId" = ? AND "antennaId" = ?`, userID, antennaID).
		Delete(&model.AntennaNoteUnread{}).Error
}

// ChannelNoteUnreadRepository tracks per-follower-per-note channel unreads.
type ChannelNoteUnreadRepository interface {
	Create(m *model.ChannelNoteUnread) error
	// HasAnyByUser reports whether the user has any unread channel notes.
	HasAnyByUser(userID string) (bool, error)
	// DeleteByChannelUser removes all unread rows for a given (user, channel)
	// pair. Used when a user reads a channel timeline.
	DeleteByChannelUser(userID, channelID string) error
}

type channelNoteUnreadRepository struct{ db *gorm.DB }

// NewChannelNoteUnreadRepository constructs a repository backed by db.
func NewChannelNoteUnreadRepository(db *gorm.DB) ChannelNoteUnreadRepository {
	return &channelNoteUnreadRepository{db: db}
}

func (r *channelNoteUnreadRepository) Create(m *model.ChannelNoteUnread) error {
	return r.db.Where(`"userId" = ? AND "channelId" = ? AND "noteId" = ?`, m.UserID, m.ChannelID, m.NoteID).
		FirstOrCreate(m).Error
}

func (r *channelNoteUnreadRepository) HasAnyByUser(userID string) (bool, error) {
	var exists bool
	err := r.db.Raw(
		`SELECT EXISTS(SELECT 1 FROM "channel_note_unread" WHERE "userId" = ?)`,
		userID,
	).Scan(&exists).Error
	return exists, err
}

func (r *channelNoteUnreadRepository) DeleteByChannelUser(userID, channelID string) error {
	return r.db.Where(`"userId" = ? AND "channelId" = ?`, userID, channelID).
		Delete(&model.ChannelNoteUnread{}).Error
}
