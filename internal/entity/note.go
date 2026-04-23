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
	Emojis             map[string]string `json:"emojis"`
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

// maxNoteEmbedDepth caps how many levels of Renote / Reply are expanded when
// packing. repository 側の preloadNoteRelations は 1 段しか preload しない
// ので通常 1 で十分だが、テストや将来の preload 変更で n.Renote.Renote などが
// 意図せず非 nil になった場合でも応答を bounded に保つためのガード (#416
// Devin review)。
const maxNoteEmbedDepth = 1

// PackNote converts a model.Note to a NoteEntity. Renote / Reply の embed は
// maxNoteEmbedDepth で制限する。
func PackNote(n *model.Note, idGen id.Generator) NoteEntity {
	return packNoteAtDepth(n, idGen, 0)
}

func packNoteAtDepth(n *model.Note, idGen id.Generator, depth int) NoteEntity {
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

	// Renote / Reply の target は repository 層で Preload("Renote.User") /
	// Preload("Reply.User") されている前提で 1 段だけ展開する。preload が
	// 無ければ n.Renote == nil になり、フロントエンドはこれを「削除された投稿」
	// として描画する (renoteId だけが入ってる状態と区別するため #416)。
	if depth < maxNoteEmbedDepth {
		if n.Renote != nil {
			r := packNoteAtDepth(n.Renote, idGen, depth+1)
			entity.Renote = &r
		}
		if n.Reply != nil {
			r := packNoteAtDepth(n.Reply, idGen, depth+1)
			entity.Reply = &r
		}
	}

	return entity
}

// PackNotes packs a slice of notes and populates UserLite.Instance for remote
// users in a single batch fetch via lookup. lookup == nil keeps Instance as
// nil (convenient for handlers not yet wired or for contexts where instance
// embed is unnecessary).
//
// flattenNotesPlusRelations で top-level + Renote/Reply の target note を
// 1 まとめにしてから resolver を作る。CollectNoteAuthors も flatten 済みの
// スライスから author を拾うので、埋め込み note の remote user にも Instance /
// emoji が正しく載る。
func PackNotes(notes []*model.Note, idGen id.Generator, instLookup InstanceLookup, emojiLookup EmojiLookup) []NoteEntity {
	flat := flattenNotesPlusRelations(notes)
	instResolver := NewInstanceResolver(instLookup, CollectNoteAuthors(flat)...)
	emojiResolver := NewEmojiResolver(emojiLookup, flat)
	out := make([]NoteEntity, 0, len(notes))
	for _, n := range notes {
		packed := PackNote(n, idGen)
		applyNoteResolvers(n, &packed, instResolver, emojiResolver)
		out = append(out, packed)
	}
	return out
}

// PackNoteWithInstance is a single-note convenience wrapper: pack + populate.
//
// **Single-note only.** Each call spins up a fresh InstanceResolver (1 DB
// query via lookup.FindManyByHosts). For a slice of notes, call `PackNotes`
// instead — calling this in a loop produces N+1 queries.
func PackNoteWithInstance(n *model.Note, idGen id.Generator, instLookup InstanceLookup, emojiLookup EmojiLookup) NoteEntity {
	packed := PackNote(n, idGen)
	flat := flattenNotesPlusRelations([]*model.Note{n})
	instResolver := NewInstanceResolver(instLookup, CollectNoteAuthors(flat)...)
	emojiResolver := NewEmojiResolver(emojiLookup, flat)
	applyNoteResolvers(n, &packed, instResolver, emojiResolver)
	return packed
}

// applyNoteResolvers fills Instance + emojis on the packed entity and its
// embedded renote/reply children. Preload は 1 段だけなので Renote.Renote や
// Reply.Reply は常に nil で、深い再帰には成らない。
func applyNoteResolvers(n *model.Note, e *NoteEntity, instResolver *InstanceResolver, emojiResolver *EmojiResolver) {
	instResolver.FillUserLite(&e.User)
	emojiResolver.PopulateNoteEmojis(n, e)
	if n.User != nil {
		emojiResolver.PopulateUserEmojis(n.User, &e.User)
	}
	if n.Renote != nil && e.Renote != nil {
		applyNoteResolvers(n.Renote, e.Renote, instResolver, emojiResolver)
	}
	if n.Reply != nil && e.Reply != nil {
		applyNoteResolvers(n.Reply, e.Reply, instResolver, emojiResolver)
	}
}

// flattenNotesPlusRelations returns notes plus any preloaded Renote/Reply
// targets (1 level deep, since GORM only preloads the relations we ask for).
// resolver 構築時に embed 先 note の author / emoji も拾うために使う。
func flattenNotesPlusRelations(notes []*model.Note) []*model.Note {
	// 最悪ケース (全 note が Renote + Reply 両方を持つ) で *3 要素になるので
	// 容量もそれに合わせる。多くの note は片方以下なので余剰確保だが、
	// timeline fetch 30 件 × 2 の再アロケートよりは安上がり。
	flat := make([]*model.Note, 0, len(notes)*3)
	for _, n := range notes {
		if n == nil {
			continue
		}
		flat = append(flat, n)
		if n.Renote != nil {
			flat = append(flat, n.Renote)
		}
		if n.Reply != nil {
			flat = append(flat, n.Reply)
		}
	}
	return flat
}

// CollectNoteAuthors returns the author `User` pointer of each note that has
// one preloaded. Used by packers and handlers when building an
// InstanceResolver over a pre-fetched slice of notes.
//
// note.User のみを拾う。reply/renote の author も含めたい場合は呼び出し側で
// flattenNotesPlusRelations を事前適用してから渡すこと (PackNotes はそうして
// いる)。
func CollectNoteAuthors(notes []*model.Note) []*model.User {
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
