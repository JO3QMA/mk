package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func cleanupNote(t *testing.T, id string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "poll" WHERE "noteId" = ?`, id)
	testDB.Exec(`DELETE FROM "note" WHERE id = ?`, id)
}

func TestNoteRepository_CreateAndFindByID(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_ncf_1", "noteuser1")
	defer cleanupUser(t, user.ID)

	text := "Hello, world!"
	note := &model.Note{
		ID:         "n_ncf_1",
		UserID:     user.ID,
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	found, err := repo.FindByID(note.ID)
	require.NoError(t, err)
	assert.Equal(t, note.ID, found.ID)
	assert.Equal(t, &text, found.Text)
}

func TestNoteRepository_FindByIDWithUser(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nfu_1", "noteuser2")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_nfu_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityHome,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	found, err := repo.FindByIDWithUser(note.ID)
	require.NoError(t, err)
	assert.NotNil(t, found.User)
	assert.Equal(t, user.ID, found.User.ID)
	assert.Equal(t, "noteuser2", found.User.Username)
}

func TestNoteRepository_UpdateFields(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nuf_1", "noteuser_uf")
	defer cleanupUser(t, user.ID)

	text := "original"
	note := &model.Note{
		ID:         "n_nuf_1",
		UserID:     user.ID,
		Text:       &text,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	updated := "edited"
	cw := "spoiler"
	require.NoError(t, repo.UpdateFields(note.ID, map[string]any{
		"text": &updated,
		"cw":   &cw,
	}))

	got, err := repo.FindByID(note.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Text)
	assert.Equal(t, "edited", *got.Text)
	require.NotNil(t, got.CW)
	assert.Equal(t, "spoiler", *got.CW)
}

func TestNoteRepository_UpdateFields_NoOp(t *testing.T) {
	repo := NewNoteRepository(testDB)
	require.NoError(t, repo.UpdateFields("any", nil))
}

func TestNoteRepository_ListByChannelID(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nlc_1", "noteuser_lc")
	defer cleanupUser(t, user.ID)

	chRepo := NewChannelRepository(testDB)
	uid := user.ID
	ch := newTestChannel("ch_lc_1", "list-by-channel", &uid)
	require.NoError(t, chRepo.Create(ch))
	defer cleanupChannel(t, ch.ID)

	cid := ch.ID
	for _, id := range []string{"n_lc_1", "n_lc_2", "n_lc_3"} {
		note := &model.Note{
			ID:         id,
			UserID:     user.ID,
			Visibility: model.NoteVisibilityPublic,
			Reactions:  datatypes.JSON([]byte("{}")),
			ChannelID:  &cid,
		}
		require.NoError(t, repo.Create(note))
		defer cleanupNote(t, note.ID)
	}

	rows, err := repo.ListByChannelID(ch.ID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, rows, 3)

	rows, err = repo.ListByChannelID(ch.ID, "n_lc_3", "", 10)
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	rows, err = repo.ListByChannelID(ch.ID, "", "n_lc_1", 10)
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	rows, err = repo.ListByChannelID(ch.ID, "", "", 0) // 0 → default 30
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestNoteRepository_FindByURI(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nuri_1", "noteuser_uri")
	defer cleanupUser(t, user.ID)

	uri := "https://remote.example/notes/n_nuri_1"
	note := &model.Note{
		ID:         "n_nuri_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
		URI:        &uri,
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	found, err := repo.FindByURI(uri)
	require.NoError(t, err)
	assert.Equal(t, note.ID, found.ID)

	// 存在しない URI なら error
	_, err = repo.FindByURI("https://remote.example/notes/missing")
	assert.Error(t, err)
}

func TestNoteRepository_Delete(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nd_1", "noteuser3")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_nd_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))

	require.NoError(t, repo.Delete(note))

	_, err := repo.FindByID(note.ID)
	assert.Error(t, err)
}

func TestNoteRepository_Update(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_nu_1", "noteuser4")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_nu_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	require.NoError(t, repo.Update(note, "hasPoll", true))

	found, err := repo.FindByID(note.ID)
	require.NoError(t, err)
	assert.True(t, found.HasPoll)
}

func TestNoteRepository_FindByID_NotFound(t *testing.T) {
	repo := NewNoteRepository(testDB)

	_, err := repo.FindByID("nonexistent_note")
	assert.Error(t, err)
}

func TestNoteRepository_FindByIDWithUser_NotFound(t *testing.T) {
	repo := NewNoteRepository(testDB)

	_, err := repo.FindByIDWithUser("nonexistent_note")
	assert.Error(t, err)
}

func TestNoteRepository_ListByUserID(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_lst_1", "listuser")
	defer cleanupUser(t, user.ID)

	for _, id := range []string{"n_lst_1", "n_lst_2", "n_lst_3"} {
		note := &model.Note{
			ID:         id,
			UserID:     user.ID,
			Visibility: model.NoteVisibilityPublic,
			Reactions:  datatypes.JSON([]byte("{}")),
		}
		require.NoError(t, repo.Create(note))
		defer cleanupNote(t, id)
	}

	// 全件 (id降順)
	out, err := repo.ListByUserID(user.ID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 3)
	assert.Equal(t, "n_lst_3", out[0].ID)

	// untilIDで絞り込み
	out, err = repo.ListByUserID(user.ID, "n_lst_3", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 2)

	// sinceIDで絞り込み
	out, err = repo.ListByUserID(user.ID, "", "n_lst_1", 10)
	require.NoError(t, err)
	assert.Len(t, out, 2)
}

func TestNoteRepository_FindManyByIDsWithUser(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_lst_2", "listuser2")
	defer cleanupUser(t, user.ID)

	for _, id := range []string{"n_fm_1", "n_fm_2"} {
		note := &model.Note{
			ID:         id,
			UserID:     user.ID,
			Visibility: model.NoteVisibilityPublic,
			Reactions:  datatypes.JSON([]byte("{}")),
		}
		require.NoError(t, repo.Create(note))
		defer cleanupNote(t, id)
	}

	out, err := repo.FindManyByIDsWithUser([]string{"n_fm_2", "n_fm_1", "ghost"})
	require.NoError(t, err)
	assert.Len(t, out, 2)
	// 順序がidsの順序を保つ
	assert.Equal(t, "n_fm_2", out[0].ID)
	assert.Equal(t, "n_fm_1", out[1].ID)

	// 空配列
	out, err = repo.FindManyByIDsWithUser(nil)
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestNoteRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := testDB.WithContext(ctx)
	repo := NewNoteRepository(db)

	_, err := repo.ListByUserID("a", "", "", 10)
	assert.Error(t, err)

	_, err = repo.ListByChannelID("a", "", "", 10)
	assert.Error(t, err)

	_, err = repo.FindManyByIDsWithUser([]string{"a"})
	assert.Error(t, err)

	_, err = repo.ListRenotesOf("a", "", "", 10)
	assert.Error(t, err)

	_, err = repo.ListRepliesOf("a", "", "", 10)
	assert.Error(t, err)

	_, err = repo.ListChildrenOf("a", "", "", 10)
	assert.Error(t, err)

	_, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "a", Limit: 10})
	assert.Error(t, err)

	err = repo.IncrementCount("a", "renoteCount", 1)
	assert.Error(t, err)

	err = repo.IncrementReaction("a", "x", 1)
	assert.Error(t, err)
}

func TestNoteRepository_IncrementReaction(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_irx_1", "irxuser")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_irx_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	require.NoError(t, repo.IncrementReaction(note.ID, "👍", 2))
	require.NoError(t, repo.IncrementReaction(note.ID, "❤", 1))

	found, err := repo.FindByID(note.ID)
	require.NoError(t, err)
	assert.Contains(t, string(found.Reactions), "👍")
	assert.Contains(t, string(found.Reactions), "❤")

	// 0以下になったキーは削除される
	require.NoError(t, repo.IncrementReaction(note.ID, "👍", -2))
	found, err = repo.FindByID(note.ID)
	require.NoError(t, err)
	assert.NotContains(t, string(found.Reactions), "👍")
}

func TestNoteRepository_IncrementCount(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_inc_1", "incuser")
	defer cleanupUser(t, user.ID)

	note := &model.Note{
		ID:         "n_inc_1",
		UserID:     user.ID,
		Visibility: model.NoteVisibilityPublic,
		Reactions:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(note))
	defer cleanupNote(t, note.ID)

	require.NoError(t, repo.IncrementCount(note.ID, "renoteCount", 2))
	require.NoError(t, repo.IncrementCount(note.ID, "repliesCount", 3))

	found, err := repo.FindByID(note.ID)
	require.NoError(t, err)
	assert.Equal(t, int16(2), found.RenoteCount)
	assert.Equal(t, int16(3), found.RepliesCount)
}

func TestNoteRepository_ListRenotesOf(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_lr_1", "lruser")
	defer cleanupUser(t, user.ID)

	// 元ノート
	parent := &model.Note{
		ID: "n_lr_parent", UserID: user.ID, Visibility: model.NoteVisibilityPublic,
		Reactions: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(parent))
	defer cleanupNote(t, parent.ID)

	// 3件のrenote
	parentID := parent.ID
	for _, id := range []string{"n_lr_r1", "n_lr_r2", "n_lr_r3"} {
		n := &model.Note{
			ID: id, UserID: user.ID, Visibility: model.NoteVisibilityPublic,
			RenoteID:  &parentID,
			Reactions: datatypes.JSON([]byte("{}")),
		}
		require.NoError(t, repo.Create(n))
		defer cleanupNote(t, id)
	}

	out, err := repo.ListRenotesOf(parent.ID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 3)
	assert.Equal(t, "n_lr_r3", out[0].ID)

	out, err = repo.ListRenotesOf(parent.ID, "n_lr_r3", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 2)

	out, err = repo.ListRenotesOf(parent.ID, "", "n_lr_r1", 10)
	require.NoError(t, err)
	assert.Len(t, out, 2)
}

func TestNoteRepository_ListRepliesOf(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_lp_1", "lpuser")
	defer cleanupUser(t, user.ID)

	parent := &model.Note{
		ID: "n_lp_parent", UserID: user.ID, Visibility: model.NoteVisibilityPublic,
		Reactions: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(parent))
	defer cleanupNote(t, parent.ID)

	parentID := parent.ID
	for _, id := range []string{"n_lp_r1", "n_lp_r2"} {
		n := &model.Note{
			ID: id, UserID: user.ID, Visibility: model.NoteVisibilityPublic,
			ReplyID:   &parentID,
			Reactions: datatypes.JSON([]byte("{}")),
		}
		require.NoError(t, repo.Create(n))
		defer cleanupNote(t, id)
	}

	out, err := repo.ListRepliesOf(parent.ID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 2)

	out, err = repo.ListRepliesOf(parent.ID, "n_lp_r2", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)

	out, err = repo.ListRepliesOf(parent.ID, "", "n_lp_r1", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestNoteRepository_ListChildrenOf(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "u_lc_1", "lcuser")
	defer cleanupUser(t, user.ID)

	parent := &model.Note{
		ID: "n_lc_parent", UserID: user.ID, Visibility: model.NoteVisibilityPublic,
		Reactions: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(parent))
	defer cleanupNote(t, parent.ID)

	parentID := parent.ID
	// 1件はreply, 1件はquote renote (IDの辞書順を昇順 c1 < c2 にしておく)
	reply := &model.Note{
		ID: "n_lc_c1", UserID: user.ID, Visibility: model.NoteVisibilityPublic,
		ReplyID:   &parentID,
		Reactions: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(reply))
	defer cleanupNote(t, reply.ID)

	quote := &model.Note{
		ID: "n_lc_c2", UserID: user.ID, Visibility: model.NoteVisibilityPublic,
		RenoteID:  &parentID,
		Reactions: datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, repo.Create(quote))
	defer cleanupNote(t, quote.ID)

	out, err := repo.ListChildrenOf(parent.ID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 2)

	out, err = repo.ListChildrenOf(parent.ID, "n_lc_c2", "", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "n_lc_c1", out[0].ID)

	out, err = repo.ListChildrenOf(parent.ID, "", "n_lc_c1", 10)
	require.NoError(t, err)
	assert.Len(t, out, 1)
	assert.Equal(t, "n_lc_c2", out[0].ID)
}

func TestNoteRepository_SearchByFilter(t *testing.T) {
	repo := NewNoteRepository(testDB)
	localUser := insertTestUser(t, "u_se_1", "seuser")
	defer cleanupUser(t, localUser.ID)
	otherLocal := insertTestUser(t, "u_se_2", "seuser2")
	defer cleanupUser(t, otherLocal.ID)
	remoteHost := "remote.example"
	remoteUser := insertRemoteTestUser(t, "u_se_r", "seremote", remoteHost)
	defer cleanupUser(t, remoteUser.ID)

	channelID := "ch_se_1"
	hello := "Hello world this is searchable"
	other := "completely different"
	private := "Hello but private"
	helloChannel := "Hello in channel"
	helloRemote := "Hello from a remote instance"

	notes := []*model.Note{
		{ID: "n_se_1", UserID: localUser.ID, Text: &hello, Visibility: model.NoteVisibilityPublic, Reactions: datatypes.JSON([]byte("{}"))},
		{ID: "n_se_2", UserID: localUser.ID, Text: &other, Visibility: model.NoteVisibilityPublic, Reactions: datatypes.JSON([]byte("{}"))},
		{ID: "n_se_3", UserID: localUser.ID, Text: &private, Visibility: model.NoteVisibilityFollowers, Reactions: datatypes.JSON([]byte("{}"))},
		{ID: "n_se_4", UserID: otherLocal.ID, Text: &hello, Visibility: model.NoteVisibilityHome, Reactions: datatypes.JSON([]byte("{}"))},
		{ID: "n_se_5", UserID: localUser.ID, Text: &helloChannel, Visibility: model.NoteVisibilityPublic, ChannelID: &channelID, Reactions: datatypes.JSON([]byte("{}"))},
		{ID: "n_se_6", UserID: remoteUser.ID, Text: &helloRemote, Visibility: model.NoteVisibilityPublic, UserHost: &remoteHost, Reactions: datatypes.JSON([]byte("{}"))},
	}
	for _, n := range notes {
		require.NoError(t, repo.Create(n))
		defer cleanupNote(t, n.ID)
	}

	// 基本: 部分一致 + visibility フィルタ。followers (n_se_3) は除外。
	out, err := repo.SearchByFilter(model.NoteSearchFilter{Query: "hello", Limit: 10})
	require.NoError(t, err)
	got := idsOf(out)
	assert.ElementsMatch(t, []string{"n_se_1", "n_se_4", "n_se_5", "n_se_6"}, got)

	// userId フィルタ
	out, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "hello", UserID: localUser.ID, Limit: 10})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n_se_1", "n_se_5"}, idsOf(out))

	// channelId フィルタ
	out, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "hello", ChannelID: channelID, Limit: 10})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n_se_5"}, idsOf(out))

	// host フィルタ "." → ローカル限定
	out, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "hello", Host: ".", Limit: 10})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n_se_1", "n_se_4", "n_se_5"}, idsOf(out))

	// host フィルタ — 特定ホスト
	out, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "hello", Host: remoteHost, Limit: 10})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n_se_6"}, idsOf(out))

	// untilID / sinceID の分岐
	out, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "hello", UntilID: "n_se_4", Limit: 10})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n_se_1"}, idsOf(out))

	out, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "hello", SinceID: "n_se_4", Limit: 10})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n_se_5", "n_se_6"}, idsOf(out))

	// Limit デフォルト (0 → 10) を踏むケース
	out, err = repo.SearchByFilter(model.NoteSearchFilter{Query: "hello"})
	require.NoError(t, err)
	assert.Len(t, out, 4)
}

// idsOf is a tiny helper to extract note IDs for assertion comparisons.
func idsOf(notes []*model.Note) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.ID)
	}
	return out
}

func TestNoteRepository_ListFeatured(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "feat_u", "featuser")
	defer cleanupUser(t, user.ID)

	n := &model.Note{ID: "feat_n1", UserID: user.ID, Visibility: "public", RenoteCount: 10}
	require.NoError(t, testDB.Create(n).Error)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, n.ID)

	notes, err := repo.ListFeatured(10, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, notes)

	// default limit
	notes, err = repo.ListFeatured(0, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, notes)

	// offset
	notes, err = repo.ListFeatured(10, 1)
	require.NoError(t, err)
	_ = notes
}

func TestNoteRepository_ListFeatured_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewNoteRepository(testDB.WithContext(ctx))
	_, err := repo.ListFeatured(10, 0)
	assert.Error(t, err)
}

func TestNoteRepository_FindRenoteByUser(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "unrn_u", "unrnuser")
	defer cleanupUser(t, user.ID)

	orig := &model.Note{ID: "unrn_orig", UserID: user.ID, Visibility: "public"}
	require.NoError(t, testDB.Create(orig).Error)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, orig.ID)

	renoteID := orig.ID
	rn := &model.Note{ID: "unrn_rn", UserID: user.ID, RenoteID: &renoteID, Visibility: "public"}
	require.NoError(t, testDB.Create(rn).Error)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, rn.ID)

	found, err := repo.FindRenoteByUser(user.ID, orig.ID)
	require.NoError(t, err)
	assert.Equal(t, rn.ID, found.ID)
}

func TestNoteRepository_FindRenoteByUser_NotFound(t *testing.T) {
	repo := NewNoteRepository(testDB)
	_, err := repo.FindRenoteByUser("ghost", "ghost")
	assert.Error(t, err)
}

func TestNoteRepository_ListMentions(t *testing.T) {
	repo := NewNoteRepository(testDB)
	user := insertTestUser(t, "ment_u", "mentuser")
	mentionee := insertTestUser(t, "ment_m", "mentionee")
	defer cleanupUser(t, user.ID)
	defer cleanupUser(t, mentionee.ID)

	n := &model.Note{ID: "ment_n1", UserID: user.ID, Visibility: "public", Mentions: []string{mentionee.ID}}
	require.NoError(t, testDB.Create(n).Error)
	defer testDB.Exec(`DELETE FROM "note" WHERE id = ?`, n.ID)

	notes, err := repo.ListMentions(mentionee.ID, 10, "", "")
	require.NoError(t, err)
	assert.NotEmpty(t, notes)

	// with cursors
	notes, err = repo.ListMentions(mentionee.ID, 10, "", "zzz")
	require.NoError(t, err)
	assert.NotEmpty(t, notes)

	notes, err = repo.ListMentions(mentionee.ID, 10, "000", "")
	require.NoError(t, err)
	assert.NotEmpty(t, notes)
}

func TestNoteRepository_ListMentions_DefaultLimit(t *testing.T) {
	repo := NewNoteRepository(testDB)
	notes, err := repo.ListMentions("nobody", 0, "", "")
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestNoteRepository_ListMentions_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewNoteRepository(testDB.WithContext(ctx))
	_, err := repo.ListMentions("x", 10, "", "")
	assert.Error(t, err)
}
