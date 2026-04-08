// Package channel provides the user-facing channel CRUD, follow, and timeline
// services. Misskey 互換のローカル限定機能で、ActivityPub 連携は持たない。
package channel

import (
	"errors"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by Service.
var (
	// ErrChannelNotFound is returned when the requested channel does not exist.
	ErrChannelNotFound = errors.New("channel not found")
	// ErrChannelNameRequired is returned when name is empty on Create / Update.
	ErrChannelNameRequired = errors.New("channel name is required")
	// ErrAccessDenied is returned when a user attempts to update a channel
	// they do not own.
	ErrAccessDenied = errors.New("not the owner of this channel")
	// ErrAlreadyFollowing is returned when a user already follows the channel.
	ErrAlreadyFollowing = errors.New("already following this channel")
	// ErrNotFollowing is returned when there is no follow record to delete.
	ErrNotFollowing = errors.New("not following this channel")
)

// Service provides channel CRUD / follow / timeline operations.
type Service struct {
	repo          repository.ChannelRepository
	followingRepo repository.ChannelFollowingRepository
	noteRepo      repository.NoteRepository
	idGen         id.Generator
	clock         func() time.Time
}

// NewService constructs a channel Service. noteRepo は Timeline / OnNotePosted
// で使用する。
func NewService(
	repo repository.ChannelRepository,
	followingRepo repository.ChannelFollowingRepository,
	noteRepo repository.NoteRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		repo:          repo,
		followingRepo: followingRepo,
		noteRepo:      noteRepo,
		idGen:         idGen,
		clock:         time.Now,
	}
}

// SetClock overrides the time source. Intended for tests.
func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.clock = now
	}
}

// CreateInput is the parameter set for Service.Create.
type CreateInput struct {
	OwnerID     string
	Name        string
	Description *string
	Color       string
	IsSensitive bool
}

// Create persists a new channel and returns it. Color が空文字なら Misskey
// 既定の `#86b300` を入れる。
func (s *Service) Create(in CreateInput) (*model.Channel, error) {
	if in.Name == "" {
		return nil, ErrChannelNameRequired
	}
	if in.OwnerID == "" {
		return nil, errors.New("ownerId is required")
	}
	owner := in.OwnerID
	now := s.clock()
	color := in.Color
	if color == "" {
		color = "#86b300"
	}
	c := &model.Channel{
		ID:                    s.idGen.Generate(now),
		Name:                  in.Name,
		Description:           in.Description,
		Color:                 color,
		UserID:                &owner,
		IsSensitive:           in.IsSensitive,
		AllowRenoteToExternal: true,
	}
	if err := s.repo.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Show returns the channel by id.
func (s *Service) Show(channelID string) (*model.Channel, error) {
	c, err := s.repo.FindByID(channelID)
	if err != nil {
		return nil, ErrChannelNotFound
	}
	return c, nil
}

// UpdateInput holds the editable fields of a channel.
type UpdateInput struct {
	Name        *string
	Description **string
	Color       *string
	IsArchived  *bool
	IsSensitive *bool
}

// Update applies the non-nil fields to a channel owned by ownerID.
func (s *Service) Update(ownerID, channelID string, in UpdateInput) (*model.Channel, error) {
	c, err := s.repo.FindByID(channelID)
	if err != nil {
		return nil, ErrChannelNotFound
	}
	if c.UserID == nil || *c.UserID != ownerID {
		return nil, ErrAccessDenied
	}
	fields := map[string]any{}
	if in.Name != nil {
		if *in.Name == "" {
			return nil, ErrChannelNameRequired
		}
		fields["name"] = *in.Name
	}
	if in.Description != nil {
		fields["description"] = *in.Description
	}
	if in.Color != nil && *in.Color != "" {
		fields["color"] = *in.Color
	}
	if in.IsArchived != nil {
		fields["isArchived"] = *in.IsArchived
	}
	if in.IsSensitive != nil {
		fields["isSensitive"] = *in.IsSensitive
	}
	if err := s.repo.UpdateFields(channelID, fields); err != nil {
		return nil, err
	}
	return s.repo.FindByID(channelID)
}

// Follow records that user follows the channel.
func (s *Service) Follow(userID, channelID string) error {
	if _, err := s.repo.FindByID(channelID); err != nil {
		return ErrChannelNotFound
	}
	if exists, err := s.followingRepo.Exists(userID, channelID); err != nil {
		return err
	} else if exists {
		return ErrAlreadyFollowing
	}
	f := &model.ChannelFollowing{
		ID:         s.idGen.Generate(s.clock()),
		FollowerID: userID,
		FolloweeID: channelID,
	}
	if err := s.followingRepo.Create(f); err != nil {
		return err
	}
	_ = s.repo.IncrementCount(channelID, "usersCount", 1)
	return nil
}

// Unfollow removes the follow relationship.
func (s *Service) Unfollow(userID, channelID string) error {
	f, err := s.followingRepo.FindByPair(userID, channelID)
	if err != nil {
		return ErrNotFollowing
	}
	if err := s.followingRepo.Delete(f); err != nil {
		return err
	}
	_ = s.repo.IncrementCount(channelID, "usersCount", -1)
	return nil
}

// ListFollowed returns the channels followed by the user. 結果には channel の
// 実体を解決して返す (followingRepo の戻り値は中間テーブル)。
func (s *Service) ListFollowed(userID string, limit, offset int) ([]*model.Channel, error) {
	rows, err := s.followingRepo.ListFollowed(userID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Channel, 0, len(rows))
	for _, f := range rows {
		c, err := s.repo.FindByID(f.FolloweeID)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// ListOwned returns channels owned by the user. クエリ / アーカイブ条件は
// repository に丸投げする。
func (s *Service) ListOwned(userID string, limit, offset int) ([]*model.Channel, error) {
	return s.repo.List(model.ChannelListFilter{OwnerID: userID, Limit: limit, Offset: offset})
}

// ListFeatured returns the most active channels (notes count desc).
func (s *Service) ListFeatured(limit, offset int) ([]*model.Channel, error) {
	return s.repo.List(model.ChannelListFilter{
		SortBy: "-notesCount",
		Limit:  limit,
		Offset: offset,
	})
}

// Search returns channels whose name matches the query.
func (s *Service) Search(query string, limit, offset int) ([]*model.Channel, error) {
	return s.repo.List(model.ChannelListFilter{
		Query:  query,
		Limit:  limit,
		Offset: offset,
	})
}

// Timeline returns notes posted to the channel, newest first.
func (s *Service) Timeline(channelID, untilID, sinceID string, limit int) ([]*model.Note, error) {
	if _, err := s.repo.FindByID(channelID); err != nil {
		return nil, ErrChannelNotFound
	}
	return s.noteRepo.ListByChannelID(channelID, untilID, sinceID, limit)
}

// OnNotePosted updates lastNotedAt and increments notesCount when a note is
// successfully posted to the channel. Best-effort: errors are ignored.
//
// 呼び出し元: NoteCreateService 経由 (ChannelHook interface 経由で疎結合)。
func (s *Service) OnNotePosted(channelID string) {
	now := s.clock()
	_ = s.repo.UpdateFields(channelID, map[string]any{"lastNotedAt": &now})
	_ = s.repo.IncrementCount(channelID, "notesCount", 1)
}
