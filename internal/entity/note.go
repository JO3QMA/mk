package entity

import (
	"encoding/json"
	"regexp"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/datatypes"
)

// localEmojiPattern matches `:name:` without `@host` suffix.
var localEmojiPattern = regexp.MustCompile(`^:([\w+\-]+):$`)

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
		Reactions:          normalizeReactionKeys(n.Reactions),
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
		VisibleUserIDs:     visibleUserIDs,
		Mentions:           mentions,
		HasPoll:            n.HasPoll,
	}

	if n.User != nil {
		entity.User = PackUserLite(n.User)
	}

	return entity
}

// PackNotes packs a slice of notes and populates UserLite.Instance for remote
// users in a single batch fetch via lookup. lookup == nil keeps Instance as
// nil (convenient for handlers not yet wired or for contexts where instance
// embed is unnecessary).
//
// reply / renote の User も instance resolver の対象に含めるため、ここで
// collectUsers により集約してから resolver を作る。
func PackNotes(notes []*model.Note, idGen id.Generator, lookup InstanceLookup) []NoteEntity {
	resolver := NewInstanceResolver(lookup, collectNoteUsers(notes)...)
	out := make([]NoteEntity, 0, len(notes))
	for _, n := range notes {
		packed := PackNote(n, idGen)
		resolver.FillUserLite(&packed.User)
		// reply / renote は PackNote 呼び出し側が別途 populate する運用のため
		// ここでは top-level note のみ埋める。
		out = append(out, packed)
	}
	return out
}

// PackNoteWithInstance is a single-note convenience wrapper: pack + populate.
func PackNoteWithInstance(n *model.Note, idGen id.Generator, lookup InstanceLookup) NoteEntity {
	packed := PackNote(n, idGen)
	resolver := NewInstanceResolver(lookup, collectNoteUsers([]*model.Note{n})...)
	resolver.FillUserLite(&packed.User)
	return packed
}

// collectNoteUsers extracts User pointers from a slice of notes for instance
// resolution. 現状は note.User のみ (reply/renote は NoteEntity で別埋めする
// 運用)。
func collectNoteUsers(notes []*model.Note) []*model.User {
	users := make([]*model.User, 0, len(notes))
	for _, n := range notes {
		if n == nil || n.User == nil {
			continue
		}
		users = append(users, n.User)
	}
	return users
}

// NormalizeReactionKey converts a legacy `:name:` key to the canonical
// `:name@.:` form used by the frontend. Remote and non-custom reactions
// are returned unchanged.
func NormalizeReactionKey(key string) string {
	if m := localEmojiPattern.FindStringSubmatch(key); m != nil {
		return ":" + m[1] + "@.:"
	}
	return key
}

// normalizeReactionKeys rewrites reaction JSONB keys so that legacy
// `:name:` entries are merged into `:name@.:`.
// TS時代のレコードとmk時代のレコードが同一キーに集約される。
func normalizeReactionKeys(raw datatypes.JSON) datatypes.JSON {
	if len(raw) == 0 {
		return raw
	}
	var m map[string]float64
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	normalized := make(map[string]float64, len(m))
	for k, v := range m {
		nk := NormalizeReactionKey(k)
		normalized[nk] += v
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return raw
	}
	return data
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
