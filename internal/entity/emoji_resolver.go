package entity

import "github.com/shiroha-a/mk/internal/model"

// EmojiLookup is the minimal interface required to batch-fetch emoji rows
// for populating UserLite.Emojis and NoteEntity.Emojis. 循環依存を避ける
// ため interface で受け取る (実装は repository.EmojiRepository)。
type EmojiLookup interface {
	FindManyByNamesAndHost(names []string, host *string) ([]*model.Emoji, error)
}

// EmojiResolver caches (name, host) → URL lookups so that packers can
// populate Emojis maps without repeating DB queries. InstanceResolverと同
// パターンで、1回のhost別batch fetch後にO(1)参照する。
//
// nilレシーバは常にno-opを返す (EmojiLookupが未配線な呼出し元向け)。
type EmojiResolver struct {
	cache map[string]string // "name@host" → url
}

// NewEmojiResolver collects unique (name, host) pairs from the notes and
// their authors, then batch-fetches matching emoji rows grouped by host.
//
// lookup==nilなら空cacheのresolverを返す (PopulateXxx は全てno-op)。
func NewEmojiResolver(lookup EmojiLookup, notes []*model.Note) *EmojiResolver {
	r := &EmojiResolver{cache: map[string]string{}}
	if lookup == nil {
		return r
	}
	// host別に絵文字名を集約
	hostNames := map[string]map[string]struct{}{}
	addNames := func(names []string, host *string) {
		if len(names) == 0 {
			return
		}
		h := ""
		if host != nil {
			h = *host
		}
		if hostNames[h] == nil {
			hostNames[h] = map[string]struct{}{}
		}
		for _, n := range names {
			hostNames[h][n] = struct{}{}
		}
	}
	for _, n := range notes {
		if n == nil {
			continue
		}
		addNames(n.Emojis, n.UserHost)
		if n.User != nil {
			addNames(n.User.Emojis, n.User.Host)
		}
	}
	// hostごとにbatch fetch
	for host, nameSet := range hostNames {
		names := make([]string, 0, len(nameSet))
		for n := range nameSet {
			names = append(names, n)
		}
		var hostPtr *string
		if host != "" {
			hostPtr = &host
		}
		emojis, err := lookup.FindManyByNamesAndHost(names, hostPtr)
		if err != nil {
			continue
		}
		for _, e := range emojis {
			url := e.PublicURL
			if url == "" {
				url = e.OriginalURL
			}
			r.cache[e.Name+"@"+host] = url
		}
	}
	return r
}

// PopulateNoteEmojis resolves emoji names stored in note.Emojis to URLs
// and sets entity.Emojis. no-opの場合entity.Emojisは変更しない
// (PackNoteが設定した空mapを維持する)。
func (r *EmojiResolver) PopulateNoteEmojis(note *model.Note, entity *NoteEntity) {
	if r == nil || note == nil || entity == nil || len(note.Emojis) == 0 {
		return
	}
	host := ""
	if note.UserHost != nil {
		host = *note.UserHost
	}
	emojis := make(map[string]string, len(note.Emojis))
	for _, name := range note.Emojis {
		if url, ok := r.cache[name+"@"+host]; ok {
			emojis[name] = url
		}
	}
	if len(emojis) > 0 {
		entity.Emojis = emojis
	}
}

// PopulateUserEmojis resolves emoji names stored in user.Emojis to URLs
// and sets lite.Emojis.
func (r *EmojiResolver) PopulateUserEmojis(user *model.User, lite *UserLite) {
	if r == nil || user == nil || lite == nil || len(user.Emojis) == 0 {
		return
	}
	host := ""
	if user.Host != nil {
		host = *user.Host
	}
	emojis := make(map[string]string, len(user.Emojis))
	for _, name := range user.Emojis {
		if url, ok := r.cache[name+"@"+host]; ok {
			emojis[name] = url
		}
	}
	if len(emojis) > 0 {
		lite.Emojis = emojis
	}
}
