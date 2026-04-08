package antenna

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

var (
	testRedis *testutil.TestRedis
	idGen     id.Generator
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	tr, err := testutil.SetupRedis(ctx)
	if err != nil {
		log.Fatalf("redis setup failed: %v", err)
	}
	testRedis = tr
	idGen, _ = id.NewGenerator("aidx")
	code := m.Run()
	testRedis.Teardown(ctx)
	os.Exit(code)
}

func newSvc(t *testing.T) (*Service, *testutil.MockAntennaRepository) {
	t.Helper()
	testRedis.FlushAll(context.Background())
	repo := testutil.NewMockAntennaRepository()
	return NewService(repo, testutil.NewMockUserRepository(), testRedis.Client, idGen), repo
}

func closedClient(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	_ = c.Close()
	return c
}

// --- Create ----------------------------------------------------------------

func TestCreate_HappyPath(t *testing.T) {
	svc, repo := newSvc(t)
	a, err := svc.Create(CreateInput{
		OwnerID:  "u1",
		Name:     "alpha",
		Src:      model.AntennaSourceAll,
		Keywords: [][]string{{"misskey"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "alpha", a.Name)
	assert.Equal(t, model.AntennaSourceAll, a.Src)
	assert.Len(t, repo.Antennas, 1)
}

func TestCreate_NameRequired(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Create(CreateInput{OwnerID: "u1", Src: model.AntennaSourceAll})
	assert.ErrorIs(t, err, ErrAntennaNameRequired)
}

func TestCreate_OwnerRequired(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Create(CreateInput{Name: "alpha", Src: model.AntennaSourceAll})
	assert.Error(t, err)
}

func TestCreate_InvalidSource(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Create(CreateInput{OwnerID: "u1", Name: "alpha", Src: "bogus"})
	assert.ErrorIs(t, err, ErrInvalidSource)
}

func TestCreate_ListSourceRejected(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Create(CreateInput{OwnerID: "u1", Name: "alpha", Src: model.AntennaSourceList})
	assert.ErrorIs(t, err, ErrInvalidSource)
}

func TestCreate_RepoError(t *testing.T) {
	svc, repo := newSvc(t)
	repo.CreateErr = errors.New("boom")
	_, err := svc.Create(CreateInput{OwnerID: "u1", Name: "alpha", Src: model.AntennaSourceAll})
	assert.Error(t, err)
}

// --- Show ------------------------------------------------------------------

func TestShow_HappyPath(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1", Name: "alpha"}
	got, err := svc.Show("u1", "a1")
	require.NoError(t, err)
	assert.Equal(t, "alpha", got.Name)
}

func TestShow_NotFound(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Show("u1", "missing")
	assert.ErrorIs(t, err, ErrAntennaNotFound)
}

func TestShow_AccessDenied(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	_, err := svc.Show("u2", "a1")
	assert.ErrorIs(t, err, ErrAccessDenied)
}

// --- Update ----------------------------------------------------------------

func TestUpdate_HappyPath(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{
		ID: "a1", UserID: "u1", Name: "alpha", Src: model.AntennaSourceAll,
		Keywords: datatypes.JSON([]byte("[]")), ExcludeKeywords: datatypes.JSON([]byte("[]")),
	}
	newName := "alpha-v2"
	src := model.AntennaSourceUsers
	users := []string{"alice"}
	keywords := [][]string{{"foo"}}
	exclude := [][]string{{"bar"}}
	caseS := true
	exB := true
	wR := true
	wF := true
	lo := true
	active := false
	got, err := svc.Update("u1", "a1", UpdateInput{
		Name:            &newName,
		Src:             &src,
		Users:           &users,
		Keywords:        &keywords,
		ExcludeKeywords: &exclude,
		CaseSensitive:   &caseS,
		ExcludeBots:     &exB,
		WithReplies:     &wR,
		WithFile:        &wF,
		LocalOnly:       &lo,
		IsActive:        &active,
	})
	require.NoError(t, err)
	assert.Equal(t, "alpha-v2", got.Name)
}

func TestUpdate_NotFound(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Update("u1", "missing", UpdateInput{})
	assert.ErrorIs(t, err, ErrAntennaNotFound)
}

func TestUpdate_AccessDenied(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "other"}
	_, err := svc.Update("u1", "a1", UpdateInput{})
	assert.ErrorIs(t, err, ErrAccessDenied)
}

func TestUpdate_NameEmpty(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	empty := ""
	_, err := svc.Update("u1", "a1", UpdateInput{Name: &empty})
	assert.ErrorIs(t, err, ErrAntennaNameRequired)
}

func TestUpdate_InvalidSource(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	bogus := model.AntennaSource("bogus")
	_, err := svc.Update("u1", "a1", UpdateInput{Src: &bogus})
	assert.ErrorIs(t, err, ErrInvalidSource)
}

func TestUpdate_RepoError(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	repo.UpdateErr = errors.New("boom")
	name := "x"
	_, err := svc.Update("u1", "a1", UpdateInput{Name: &name})
	assert.Error(t, err)
}

// --- Delete ----------------------------------------------------------------

func TestDelete_HappyPath(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	require.NoError(t, svc.Delete("u1", "a1"))
	assert.Empty(t, repo.Antennas)
}

func TestDelete_NotFound(t *testing.T) {
	svc, _ := newSvc(t)
	err := svc.Delete("u1", "missing")
	assert.ErrorIs(t, err, ErrAntennaNotFound)
}

func TestDelete_AccessDenied(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "other"}
	err := svc.Delete("u1", "a1")
	assert.ErrorIs(t, err, ErrAccessDenied)
}

// failingDeleteRepo causes Delete to fail.
type failingDeleteRepo struct {
	*testutil.MockAntennaRepository
}

func (r *failingDeleteRepo) Delete(_ *model.Antenna) error { return errors.New("boom") }

func TestDelete_RepoError(t *testing.T) {
	mock := testutil.NewMockAntennaRepository()
	mock.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	repo := &failingDeleteRepo{MockAntennaRepository: mock}
	svc := NewService(repo, testutil.NewMockUserRepository(), testRedis.Client, idGen)
	err := svc.Delete("u1", "a1")
	assert.Error(t, err)
}

// --- ListByUser ------------------------------------------------------------

func TestListByUser(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	repo.Antennas["a2"] = &model.Antenna{ID: "a2", UserID: "u1"}
	rows, err := svc.ListByUser("u1")
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

// --- Notes -----------------------------------------------------------------

func TestNotes_HappyPath(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	require.NoError(t, svc.pushNote(context.Background(), "a1", "n1", time.Now()))
	require.NoError(t, svc.pushNote(context.Background(), "a1", "n2", time.Now().Add(time.Millisecond)))
	rows, err := svc.Notes(context.Background(), "u1", "a1", 10)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestNotes_LimitClamping(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	rows, err := svc.Notes(context.Background(), "u1", "a1", -1)
	require.NoError(t, err)
	assert.Empty(t, rows)
	rows, err = svc.Notes(context.Background(), "u1", "a1", 9999)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestNotes_NotFound(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Notes(context.Background(), "u1", "missing", 10)
	assert.ErrorIs(t, err, ErrAntennaNotFound)
}

func TestNotes_RedisError(t *testing.T) {
	repo := testutil.NewMockAntennaRepository()
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	svc := NewService(repo, testutil.NewMockUserRepository(), closedClient(t), idGen)
	_, err := svc.Notes(context.Background(), "u1", "a1", 10)
	assert.Error(t, err)
}

func TestNotes_SkipsBadValue(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = &model.Antenna{ID: "a1", UserID: "u1"}
	// noteId フィールドが無いエントリを混ぜる
	require.NoError(t, testRedis.Client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: streamKey("a1"),
		ID:     "1-0",
		Values: map[string]any{"other": "x"},
	}).Err())
	rows, err := svc.Notes(context.Background(), "u1", "a1", 10)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// --- OnNoteCreated + matchNote --------------------------------------------

func makeAntenna(t *testing.T, id, userID string, kw [][]string, mods ...func(*model.Antenna)) *model.Antenna {
	t.Helper()
	keywordsJSON := []byte("[]")
	if len(kw) > 0 {
		raw, err := json.Marshal(kw)
		require.NoError(t, err)
		keywordsJSON = raw
	}
	a := &model.Antenna{
		ID:              id,
		UserID:          userID,
		Name:            id,
		Src:             model.AntennaSourceAll,
		Keywords:        datatypes.JSON(keywordsJSON),
		ExcludeKeywords: datatypes.JSON([]byte("[]")),
		IsActive:        true,
	}
	for _, m := range mods {
		m(a)
	}
	return a
}

func TestOnNoteCreated_HappyPath(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", [][]string{{"misskey"}})

	text := "hello misskey world"
	n := &model.Note{ID: "n1", UserID: "author", Text: &text, Visibility: model.NoteVisibilityPublic}
	author := &model.User{ID: "author", Username: "alice"}
	svc.OnNoteCreated(n, author)

	rows, err := svc.Notes(context.Background(), "u1", "a1", 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"n1"}, rows)
}

func TestOnNoteCreated_NilArgsAreNoOp(t *testing.T) {
	svc, _ := newSvc(t)
	svc.OnNoteCreated(nil, &model.User{})
	svc.OnNoteCreated(&model.Note{}, nil)
}

func TestOnNoteCreated_RepoErrorIsNoOp(t *testing.T) {
	repo := testutil.NewMockAntennaRepository()
	svc := NewService(&listFailRepo{repo}, testutil.NewMockUserRepository(), testRedis.Client, idGen)
	text := "hi"
	svc.OnNoteCreated(&model.Note{ID: "n1", Text: &text}, &model.User{ID: "u1"})
}

// listFailRepo causes ListAllActive to fail.
type listFailRepo struct {
	*testutil.MockAntennaRepository
}

func (r *listFailRepo) ListAllActive() ([]*model.Antenna, error) {
	return nil, errors.New("boom")
}

func TestOnNoteCreated_NoMatchSkipped(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", [][]string{{"missing"}})

	text := "hello world"
	svc.OnNoteCreated(
		&model.Note{ID: "n1", Text: &text},
		&model.User{ID: "author", Username: "alice"},
	)

	rows, err := svc.Notes(context.Background(), "u1", "a1", 10)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// --- matchNote each filter ------------------------------------------------

func TestMatchNote_LocalOnlyRejectsRemote(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.LocalOnly = true
	})
	host := "remote.example"
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{Host: &host}))
}

func TestMatchNote_ExcludeBotsRejectsBot(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.ExcludeBots = true
	})
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{IsBot: true}))
}

func TestMatchNote_WithFileRequiresAttachment(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.WithFile = true
	})
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{}))
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{FileIDs: []string{"f1"}}, &model.User{}))
}

func TestMatchNote_WithRepliesFalseRejectsReplies(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil)
	parent := "p1"
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{ReplyID: &parent}, &model.User{}))
}

func TestMatchNote_WithRepliesTrueAllowsReplies(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.WithReplies = true
	})
	parent := "p1"
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{ReplyID: &parent}, &model.User{}))
}

func TestMatchNote_UsersSourceWhitelist(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.Src = model.AntennaSourceUsers
		a.Users = []string{"alice"}
	})
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{Username: "alice"}))
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{Username: "bob"}))
}

func TestMatchNote_UsersBlacklist(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.Src = model.AntennaSourceUsersBlacklist
		a.Users = []string{"alice"}
	})
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{Username: "alice"}))
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{Username: "bob"}))
}

func TestMatchNote_KeywordsCaseSensitiveMiss(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", [][]string{{"Misskey"}}, func(a *model.Antenna) {
		a.CaseSensitive = true
	})
	text := "this is misskey"
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &text}, &model.User{}))
}

func TestMatchNote_KeywordsCaseSensitiveHit(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", [][]string{{"Misskey"}}, func(a *model.Antenna) {
		a.CaseSensitive = true
	})
	text := "this is Misskey"
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &text}, &model.User{}))
}

func TestMatchNote_KeywordsAndOr(t *testing.T) {
	svc, repo := newSvc(t)
	// (foo AND bar) OR baz
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", [][]string{{"foo", "bar"}, {"baz"}})
	hit1 := "foo bar"
	hit2 := "baz only"
	miss := "foo only"
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &hit1}, &model.User{}))
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &hit2}, &model.User{}))
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &miss}, &model.User{}))
}

func TestMatchNote_ExcludeKeywords(t *testing.T) {
	svc, repo := newSvc(t)
	exclude := []byte(`[["spam"]]`)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.ExcludeKeywords = datatypes.JSON(exclude)
	})
	dirty := "this is spam"
	clean := "this is fine"
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &dirty}, &model.User{}))
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &clean}, &model.User{}))
}

func TestMatchNote_BadKeywordsJSONTreatedAsEmpty(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.Keywords = datatypes.JSON([]byte("{not json"))
	})
	// 不正 JSON は emptyMatches=true として扱うので keyword フィルタは pass
	// (matchKeywords が true を返す)
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{}))
}

func TestMatchNote_NilKeywordsTreatedAsEmpty(t *testing.T) {
	svc, repo := newSvc(t)
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.Keywords = nil // empty raw → emptyMatches=true
	})
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{}, &model.User{}))
}

func TestMatchNote_EmptyInnerGroupSkipped(t *testing.T) {
	svc, repo := newSvc(t)
	// 外側 OR の中に空 group + 実 group。空 group は skip され実 group が
	// マッチしなければ false
	repo.Antennas["a1"] = makeAntenna(t, "a1", "u1", nil, func(a *model.Antenna) {
		a.Keywords = datatypes.JSON([]byte(`[[],["foo"]]`))
	})
	miss := "no match here"
	require.False(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &miss}, &model.User{}))
	hit := "foo here"
	require.True(t, svc.matchNote(repo.Antennas["a1"], &model.Note{Text: &hit}, &model.User{}))
}

func TestNoteText_CWAndText(t *testing.T) {
	cw := "warning"
	text := "body"
	n := &model.Note{CW: &cw, Text: &text}
	got := noteText(n)
	assert.Contains(t, got, "warning")
	assert.Contains(t, got, "body")
}

// normalizeKeywords drops empty rows
func TestNormalizeKeywords(t *testing.T) {
	got := normalizeKeywords([][]string{
		{"a", "b"},
		{},
		{"   ", "c"},
		{""},
	})
	assert.Equal(t, [][]string{{"a", "b"}, {"c"}}, got)
}

// --- SetClock --------------------------------------------------------------

func TestSetClock(t *testing.T) {
	svc, _ := newSvc(t)
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return fixed })
	svc.SetClock(nil)
	a, err := svc.Create(CreateInput{OwnerID: "u1", Name: "alpha", Src: model.AntennaSourceAll})
	require.NoError(t, err)
	assert.Equal(t, fixed, a.LastUsedAt)
}
