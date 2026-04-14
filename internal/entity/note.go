package entity

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/datatypes"
)

// NoteEntity is the note representation returned by API endpoints.
type NoteEntity struct {
	ID                 string            `json:"id"`
	CreatedAt          string            `json:"createdAt"`
	UserID             string            `json:"userId"`
	User               UserLite          `json:"user"`
	Text               *string           `json:"text"`
	CW                 *string           `json:"cw"`
	Visibility         string            `json:"visibility"`
	LocalOnly          bool              `json:"localOnly"`
	ReactionAcceptance *string           `json:"reactionAcceptance"`
	Reactions          datatypes.JSON    `json:"reactions"`
	ReactionCount      int               `json:"reactionCount"`
	ReactionEmojis     map[string]string `json:"reactionEmojis"`
	RenoteCount        int16             `json:"renoteCount"`
	RepliesCount       int16             `json:"repliesCount"`
	ClippedCount       int               `json:"clippedCount"`
	URI                *string           `json:"uri,omitempty"`
	URL                *string           `json:"url,omitempty"`
	ReplyID            *string           `json:"replyId"`
	RenoteID           *string           `json:"renoteId"`
	Reply              *NoteEntity       `json:"reply,omitempty"`
	Renote             *NoteEntity       `json:"renote,omitempty"`
	FileIDs            []string          `json:"fileIds"`
	Files              []any             `json:"files"`
	Tags               []string          `json:"tags,omitempty"`
	Poll               *PollEntity       `json:"poll,omitempty"`
	Emojis             map[string]string `json:"emojis,omitempty"`
	ChannelID          *string           `json:"channelId,omitempty"`
	Channel            *ChannelLite      `json:"channel,omitempty"`
	AppID              *string           `json:"appId,omitempty"`
	VisibleUserIDs     []string          `json:"visibleUserIds"`
	Mentions           []string          `json:"mentions"`
	HasPoll            bool              `json:"hasPoll"`
	MyReaction         *string           `json:"myReaction,omitempty"`
	IsHidden           bool              `json:"isHidden,omitempty"`
}

// ChannelLite is the minimal channel info embedded in NoteEntity.
type ChannelLite struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Color                 string `json:"color"`
	IsSensitive           bool   `json:"isSensitive"`
	AllowRenoteToExternal bool   `json:"allowRenoteToExternal"`
}

// PollEntity is the poll representation in a note.
type PollEntity struct {
	ExpiresAt *string      `json:"expiresAt"`
	Multiple  bool         `json:"multiple"`
	Choices   []PollChoice `json:"choices"`
}

// PollChoice represents a single poll choice.
type PollChoice struct {
	Text    string `json:"text"`
	Votes   int    `json:"votes"`
	IsVoted bool   `json:"isVoted"`
}

// PackNote converts a model.Note to a NoteEntity.
func PackNote(n *model.Note, idGen id.Generator) NoteEntity {
	createdAt := ""
	if t, err := idGen.ParseTime(n.ID); err == nil {
		createdAt = t.UTC().Format("2006-01-02T15:04:05.000Z")
	}

	fileIDs := make([]string, 0)
	if n.FileIDs != nil {
		fileIDs = n.FileIDs
	}

	visibleUserIDs := make([]string, 0)
	if n.VisibleUserIDs != nil {
		visibleUserIDs = n.VisibleUserIDs
	}

	mentions := make([]string, 0)
	if n.Mentions != nil {
		mentions = n.Mentions
	}

	entity := NoteEntity{
		ID:                 n.ID,
		CreatedAt:          createdAt,
		UserID:             n.UserID,
		Text:               n.Text,
		CW:                 n.CW,
		Visibility:         string(n.Visibility),
		LocalOnly:          n.LocalOnly,
		ReactionAcceptance: n.ReactionAcceptance,
		Reactions:          n.Reactions,
		ReactionCount:      sumReactions(n.Reactions),
		ReactionEmojis:     make(map[string]string),
		RenoteCount:        n.RenoteCount,
		RepliesCount:       n.RepliesCount,
		ClippedCount:       0,
		URI:                n.URI,
		URL:                n.URL,
		ReplyID:            n.ReplyID,
		RenoteID:           n.RenoteID,
		FileIDs:            fileIDs,
		Files:              []any{},
		Tags:               n.Tags,
		Emojis:             make(map[string]string),
		ChannelID:          n.ChannelID,
		AppID:              n.AppID,
		VisibleUserIDs:     visibleUserIDs,
		Mentions:           mentions,
		HasPoll:            n.HasPoll,
	}

	if n.User != nil {
		entity.User = PackUserLite(n.User)
	}

	return entity
}

// sumReactions decodes the reactions JSONB and sums all values.
func sumReactions(raw datatypes.JSON) int {
	if len(raw) == 0 {
		return 0
	}
	var m map[string]float64
	if err := json.Unmarshal(raw, &m); err != nil {
		return 0
	}
	total := 0
	for _, v := range m {
		total += int(v)
	}
	return total
}
