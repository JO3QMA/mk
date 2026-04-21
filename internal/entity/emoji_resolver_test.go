package entity

import (
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubEmojiLookup implements EmojiLookup for testing purposes.
type stubEmojiLookup struct {
	// host文字列("" = local) → 返却する絵文字一覧
	data  map[string][]*model.Emoji
	calls []struct {
		names []string
		host  *string
	}
}

func (s *stubEmojiLookup) FindManyByNamesAndHost(names []string, host *string) ([]*model.Emoji, error) {
	// 呼び出し記録を保存(assertionで使う)
	cp := make([]string, len(names))
	copy(cp, names)
	s.calls = append(s.calls, struct {
		names []string
		host  *string
	}{names: cp, host: host})

	h := ""
	if host != nil {
		h = *host
	}
	return s.data[h], nil
}

// strPtr is a helper that returns a pointer to a string literal.
func strPtr(s string) *string { return &s }

func TestNewEmojiResolver_BatchResolve(t *testing.T) {
	remoteHost := "remote.example"
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			"remote.example": {
				{ID: "e1", Name: "thumbsup", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/thumbsup.png", OriginalURL: "https://remote.example/emoji/thumbsup_orig.png"},
				{ID: "e2", Name: "wave", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/wave.png", OriginalURL: "https://remote.example/emoji/wave_orig.png"},
			},
		},
	}

	notes := []*model.Note{
		{
			ID:       "n1",
			UserID:   "u1",
			UserHost: &remoteHost,
			Emojis:   pq.StringArray{"thumbsup", "wave"},
		},
		{
			ID:       "n2",
			UserID:   "u1",
			UserHost: &remoteHost,
			Emojis:   pq.StringArray{"thumbsup"}, // 重複する名前はbatchで一度だけfetch
		},
	}

	r := NewEmojiResolver(lookup, notes)

	// host別に1回だけbatch fetchされる
	require.Len(t, lookup.calls, 1)
	assert.ElementsMatch(t, []string{"thumbsup", "wave"}, lookup.calls[0].names)
	require.NotNil(t, lookup.calls[0].host)
	assert.Equal(t, "remote.example", *lookup.calls[0].host)

	// cacheにURLが格納されている
	assert.Equal(t, "https://remote.example/emoji/thumbsup.png", r.cache["thumbsup@remote.example"])
	assert.Equal(t, "https://remote.example/emoji/wave.png", r.cache["wave@remote.example"])
}

func TestPopulateNoteEmojis(t *testing.T) {
	remoteHost := "remote.example"
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			"remote.example": {
				{ID: "e1", Name: "fire", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/fire.webp", OriginalURL: "https://remote.example/emoji/fire.png"},
				{ID: "e2", Name: "star", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/star.webp", OriginalURL: "https://remote.example/emoji/star.png"},
			},
		},
	}

	note := &model.Note{
		ID:       "n1",
		UserID:   "u1",
		UserHost: &remoteHost,
		Emojis:   pq.StringArray{"fire", "star"},
	}

	r := NewEmojiResolver(lookup, []*model.Note{note})
	entity := &NoteEntity{}
	r.PopulateNoteEmojis(note, entity)

	// note.Emojisに対応するURLがentity.Emojisに設定される
	require.NotNil(t, entity.Emojis)
	assert.Equal(t, "https://remote.example/emoji/fire.webp", entity.Emojis["fire"])
	assert.Equal(t, "https://remote.example/emoji/star.webp", entity.Emojis["star"])
}

func TestPopulateNoteEmojis_Empty(t *testing.T) {
	lookup := &stubEmojiLookup{data: map[string][]*model.Emoji{}}

	note := &model.Note{
		ID:     "n1",
		UserID: "u1",
		Emojis: pq.StringArray{}, // 空の絵文字リスト
	}

	r := NewEmojiResolver(lookup, []*model.Note{note})
	entity := &NoteEntity{Emojis: map[string]string{"existing": "value"}}

	r.PopulateNoteEmojis(note, entity)

	// 空emojisの場合、entity.Emojisは変更されない
	assert.Equal(t, map[string]string{"existing": "value"}, entity.Emojis)
}

func TestPopulateUserEmojis(t *testing.T) {
	remoteHost := "remote.example"
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			"remote.example": {
				{ID: "e1", Name: "cat", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/cat.png", OriginalURL: "https://remote.example/emoji/cat_orig.png"},
			},
		},
	}

	user := &model.User{
		ID:       "u1",
		Username: "alice",
		Host:     &remoteHost,
		Emojis:   pq.StringArray{"cat"},
	}

	// note経由でuserの絵文字もcacheに読み込ませる
	note := &model.Note{
		ID:       "n1",
		UserID:   "u1",
		UserHost: &remoteHost,
		User:     user,
		Emojis:   pq.StringArray{},
	}

	r := NewEmojiResolver(lookup, []*model.Note{note})
	lite := &UserLite{}
	r.PopulateUserEmojis(user, lite)

	// user.Emojisに対応するURLがlite.Emojisに設定される
	require.NotNil(t, lite.Emojis)
	assert.Equal(t, "https://remote.example/emoji/cat.png", lite.Emojis["cat"])
}

func TestPopulateUserEmojis_Empty(t *testing.T) {
	lookup := &stubEmojiLookup{data: map[string][]*model.Emoji{}}

	user := &model.User{
		ID:       "u1",
		Username: "bob",
		Emojis:   pq.StringArray{}, // 空の絵文字リスト
	}

	r := NewEmojiResolver(lookup, []*model.Note{})
	lite := &UserLite{Emojis: map[string]string{"existing": "value"}}
	r.PopulateUserEmojis(user, lite)

	// 空emojisの場合、lite.Emojisは変更されない
	assert.Equal(t, map[string]string{"existing": "value"}, lite.Emojis)
}

func TestEmojiResolver_NilLookup(t *testing.T) {
	remoteHost := "remote.example"
	note := &model.Note{
		ID:       "n1",
		UserID:   "u1",
		UserHost: &remoteHost,
		Emojis:   pq.StringArray{"fire"},
	}

	// lookup==nilでも空cacheのresolverが返りpanicしない
	r := NewEmojiResolver(nil, []*model.Note{note})
	require.NotNil(t, r)
	assert.Empty(t, r.cache)

	// PopulateNoteEmojisはno-op
	entity := &NoteEntity{Emojis: map[string]string{"keep": "this"}}
	r.PopulateNoteEmojis(note, entity)
	assert.Equal(t, map[string]string{"keep": "this"}, entity.Emojis)

	// PopulateUserEmojisもno-op
	user := &model.User{
		ID:       "u1",
		Username: "alice",
		Host:     &remoteHost,
		Emojis:   pq.StringArray{"fire"},
	}
	lite := &UserLite{Emojis: map[string]string{"keep": "this"}}
	r.PopulateUserEmojis(user, lite)
	assert.Equal(t, map[string]string{"keep": "this"}, lite.Emojis)
}

func TestEmojiResolver_NilReceiver(t *testing.T) {
	var r *EmojiResolver

	// nilレシーバのPopulateNoteEmojisはpanicしない
	note := &model.Note{
		ID:     "n1",
		UserID: "u1",
		Emojis: pq.StringArray{"fire"},
	}
	entity := &NoteEntity{}
	assert.NotPanics(t, func() {
		r.PopulateNoteEmojis(note, entity)
	})

	// nilレシーバのPopulateUserEmojisもpanicしない
	user := &model.User{
		ID:       "u1",
		Username: "alice",
		Emojis:   pq.StringArray{"fire"},
	}
	lite := &UserLite{}
	assert.NotPanics(t, func() {
		r.PopulateUserEmojis(user, lite)
	})
}

func TestEmojiResolver_LocalEmojis(t *testing.T) {
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			// host=="" はローカル絵文字
			"": {
				{ID: "e1", Name: "local_emoji", Host: nil, PublicURL: "https://local.example/emoji/local_emoji.png", OriginalURL: "https://local.example/emoji/local_emoji_orig.png"},
			},
		},
	}

	// UserHost==nil (ローカルユーザーの投稿)
	note := &model.Note{
		ID:       "n1",
		UserID:   "u1",
		UserHost: nil,
		Emojis:   pq.StringArray{"local_emoji"},
	}

	r := NewEmojiResolver(lookup, []*model.Note{note})

	// ローカル絵文字のfetchではhost=nilが渡される
	require.Len(t, lookup.calls, 1)
	assert.Nil(t, lookup.calls[0].host)

	// cacheに "name@" (空host) のキーで格納される
	assert.Equal(t, "https://local.example/emoji/local_emoji.png", r.cache["local_emoji@"])

	// PopulateNoteEmojisでローカル絵文字が解決される
	entity := &NoteEntity{}
	r.PopulateNoteEmojis(note, entity)
	require.NotNil(t, entity.Emojis)
	assert.Equal(t, "https://local.example/emoji/local_emoji.png", entity.Emojis["local_emoji"])
}

func TestEmojiResolver_PublicURLFallback(t *testing.T) {
	remoteHost := "remote.example"
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			"remote.example": {
				// PublicURLが空の場合はOriginalURLにフォールバック
				{ID: "e1", Name: "rare", Host: strPtr("remote.example"), PublicURL: "", OriginalURL: "https://remote.example/emoji/rare_orig.png"},
			},
		},
	}

	note := &model.Note{
		ID:       "n1",
		UserID:   "u1",
		UserHost: &remoteHost,
		Emojis:   pq.StringArray{"rare"},
	}

	r := NewEmojiResolver(lookup, []*model.Note{note})

	// PublicURLが空なのでOriginalURLが使われる
	assert.Equal(t, "https://remote.example/emoji/rare_orig.png", r.cache["rare@remote.example"])

	entity := &NoteEntity{}
	r.PopulateNoteEmojis(note, entity)
	require.NotNil(t, entity.Emojis)
	assert.Equal(t, "https://remote.example/emoji/rare_orig.png", entity.Emojis["rare"])
}

func TestEmojiResolver_MultipleHosts(t *testing.T) {
	hostA := "alpha.example"
	hostB := "beta.example"
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			"alpha.example": {
				{ID: "e1", Name: "smile", Host: strPtr("alpha.example"), PublicURL: "https://alpha.example/emoji/smile.png", OriginalURL: "https://alpha.example/emoji/smile_orig.png"},
			},
			"beta.example": {
				{ID: "e2", Name: "smile", Host: strPtr("beta.example"), PublicURL: "https://beta.example/emoji/smile.png", OriginalURL: "https://beta.example/emoji/smile_orig.png"},
				{ID: "e3", Name: "wave", Host: strPtr("beta.example"), PublicURL: "https://beta.example/emoji/wave.png", OriginalURL: "https://beta.example/emoji/wave_orig.png"},
			},
		},
	}

	notes := []*model.Note{
		{
			ID:       "n1",
			UserID:   "u1",
			UserHost: &hostA,
			Emojis:   pq.StringArray{"smile"},
		},
		{
			ID:       "n2",
			UserID:   "u2",
			UserHost: &hostB,
			Emojis:   pq.StringArray{"smile", "wave"},
		},
	}

	r := NewEmojiResolver(lookup, notes)

	// host別に2回batch fetchが呼ばれる
	require.Len(t, lookup.calls, 2)

	// 同名の絵文字でもhost別に異なるURLが解決される
	assert.Equal(t, "https://alpha.example/emoji/smile.png", r.cache["smile@alpha.example"])
	assert.Equal(t, "https://beta.example/emoji/smile.png", r.cache["smile@beta.example"])
	assert.Equal(t, "https://beta.example/emoji/wave.png", r.cache["wave@beta.example"])

	// note1のemojisはalpha.exampleから解決される
	entityA := &NoteEntity{}
	r.PopulateNoteEmojis(notes[0], entityA)
	require.NotNil(t, entityA.Emojis)
	assert.Equal(t, "https://alpha.example/emoji/smile.png", entityA.Emojis["smile"])

	// note2のemojisはbeta.exampleから解決される
	entityB := &NoteEntity{}
	r.PopulateNoteEmojis(notes[1], entityB)
	require.NotNil(t, entityB.Emojis)
	assert.Equal(t, "https://beta.example/emoji/smile.png", entityB.Emojis["smile"])
	assert.Equal(t, "https://beta.example/emoji/wave.png", entityB.Emojis["wave"])
}

func TestNewEmojiResolver_NilNotesInSlice(t *testing.T) {
	lookup := &stubEmojiLookup{data: map[string][]*model.Emoji{}}

	// nilを含むスライスでもpanicしない
	notes := []*model.Note{nil, nil}
	r := NewEmojiResolver(lookup, notes)
	require.NotNil(t, r)
	assert.Empty(t, r.cache)
	// nilのnoteはスキップされるのでfetchは呼ばれない
	assert.Empty(t, lookup.calls)
}

func TestNewEmojiResolver_UserEmojisCollectedFromNoteUser(t *testing.T) {
	remoteHost := "remote.example"
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			"remote.example": {
				{ID: "e1", Name: "note_emoji", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/note_emoji.png", OriginalURL: ""},
				{ID: "e2", Name: "user_emoji", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/user_emoji.png", OriginalURL: ""},
			},
		},
	}

	user := &model.User{
		ID:       "u1",
		Username: "alice",
		Host:     &remoteHost,
		Emojis:   pq.StringArray{"user_emoji"},
	}

	note := &model.Note{
		ID:       "n1",
		UserID:   "u1",
		UserHost: &remoteHost,
		Emojis:   pq.StringArray{"note_emoji"},
		User:     user,
	}

	r := NewEmojiResolver(lookup, []*model.Note{note})

	// noteのemojisとuserのemojisが共にcacheに格納される
	assert.Equal(t, "https://remote.example/emoji/note_emoji.png", r.cache["note_emoji@remote.example"])
	assert.Equal(t, "https://remote.example/emoji/user_emoji.png", r.cache["user_emoji@remote.example"])

	// 1回のbatch fetchで両方取得されている
	require.Len(t, lookup.calls, 1)
	assert.ElementsMatch(t, []string{"note_emoji", "user_emoji"}, lookup.calls[0].names)
}

func TestPopulateNoteEmojis_NilNoteOrEntity(t *testing.T) {
	lookup := &stubEmojiLookup{data: map[string][]*model.Emoji{}}
	r := NewEmojiResolver(lookup, []*model.Note{})

	// note==nilでもpanicしない
	entity := &NoteEntity{}
	assert.NotPanics(t, func() {
		r.PopulateNoteEmojis(nil, entity)
	})

	// entity==nilでもpanicしない
	note := &model.Note{ID: "n1", UserID: "u1", Emojis: pq.StringArray{"x"}}
	assert.NotPanics(t, func() {
		r.PopulateNoteEmojis(note, nil)
	})
}

func TestPopulateUserEmojis_NilUserOrLite(t *testing.T) {
	lookup := &stubEmojiLookup{data: map[string][]*model.Emoji{}}
	r := NewEmojiResolver(lookup, []*model.Note{})

	// user==nilでもpanicしない
	lite := &UserLite{}
	assert.NotPanics(t, func() {
		r.PopulateUserEmojis(nil, lite)
	})

	// lite==nilでもpanicしない
	user := &model.User{ID: "u1", Username: "alice", Emojis: pq.StringArray{"x"}}
	assert.NotPanics(t, func() {
		r.PopulateUserEmojis(user, nil)
	})
}

func TestPopulateNoteEmojis_UnresolvedEmojiExcluded(t *testing.T) {
	remoteHost := "remote.example"
	lookup := &stubEmojiLookup{
		data: map[string][]*model.Emoji{
			"remote.example": {
				// "known"のみ返却、"unknown"は含まない
				{ID: "e1", Name: "known", Host: strPtr("remote.example"), PublicURL: "https://remote.example/emoji/known.png", OriginalURL: ""},
			},
		},
	}

	note := &model.Note{
		ID:       "n1",
		UserID:   "u1",
		UserHost: &remoteHost,
		Emojis:   pq.StringArray{"known", "unknown"},
	}

	r := NewEmojiResolver(lookup, []*model.Note{note})
	entity := &NoteEntity{}
	r.PopulateNoteEmojis(note, entity)

	// cacheに存在する絵文字のみがentity.Emojisに含まれる
	require.NotNil(t, entity.Emojis)
	assert.Equal(t, "https://remote.example/emoji/known.png", entity.Emojis["known"])
	_, exists := entity.Emojis["unknown"]
	assert.False(t, exists, "unknown emoji should not be in entity.Emojis")
}
