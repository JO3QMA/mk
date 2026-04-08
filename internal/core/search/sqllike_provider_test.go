package search

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSQLLikeProviderForTest() (*SQLLikeProvider, *testutil.MockNoteRepository) {
	repo := testutil.NewMockNoteRepository()
	return NewSQLLikeProvider(repo, nil), repo
}

func TestSQLLikeProvider_IndexAndUnindexAreNoOps(t *testing.T) {
	p, _ := newSQLLikeProviderForTest()
	require.NoError(t, p.IndexNote(&model.Note{ID: "n1"}))
	require.NoError(t, p.UnindexNote(&model.Note{ID: "n1"}))
}

func TestSQLLikeProvider_SearchNoteHappyPath(t *testing.T) {
	p, repo := newSQLLikeProviderForTest()
	hello := "Hello, world"
	repo.Notes["n1"] = &model.Note{
		ID:         "n1",
		UserID:     "u1",
		Visibility: model.NoteVisibilityPublic,
		Text:       &hello,
	}
	other := "completely different"
	repo.Notes["n2"] = &model.Note{
		ID:         "n2",
		UserID:     "u1",
		Visibility: model.NoteVisibilityPublic,
		Text:       &other,
	}

	out, err := p.SearchNote(nil, "hello", SearchOpts{}, Pagination{Limit: 10})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "n1", out[0].ID)
}

func TestSQLLikeProvider_EmptyQueryRejected(t *testing.T) {
	p, _ := newSQLLikeProviderForTest()
	_, err := p.SearchNote(nil, "", SearchOpts{}, Pagination{Limit: 10})
	require.ErrorIs(t, err, ErrEmptyQuery)
}

func TestSQLLikeProvider_DefaultLimitWhenZero(t *testing.T) {
	p, repo := newSQLLikeProviderForTest()
	hello := "hello"
	for i := range 12 {
		id := string(rune('a'+i)) + "1"
		repo.Notes[id] = &model.Note{
			ID:         id,
			UserID:     "u",
			Visibility: model.NoteVisibilityPublic,
			Text:       &hello,
		}
	}
	out, err := p.SearchNote(nil, "hello", SearchOpts{}, Pagination{}) // Limit==0
	require.NoError(t, err)
	// MockNoteRepository.SearchByFilter のデフォルト Limit は 10。
	assert.Len(t, out, 10)
}

// stubFailingNoteRepo wraps the in-memory mock and forces SearchByFilter to fail
// so we can verify the provider propagates repo errors.
type stubFailingNoteRepo struct {
	*testutil.MockNoteRepository
}

func (s *stubFailingNoteRepo) SearchByFilter(_ model.NoteSearchFilter) ([]*model.Note, error) {
	return nil, errors.New("boom")
}

func TestSQLLikeProvider_RepoErrorPropagated(t *testing.T) {
	repo := &stubFailingNoteRepo{MockNoteRepository: testutil.NewMockNoteRepository()}
	p := NewSQLLikeProvider(repo, nil)
	_, err := p.SearchNote(nil, "hello", SearchOpts{}, Pagination{Limit: 10})
	require.Error(t, err)
}

func TestSQLLikeProvider_VisibilityFilterDropsHidden(t *testing.T) {
	p, repo := newSQLLikeProviderForTest()
	hello := "hello"
	repo.Notes["n1"] = &model.Note{
		ID:         "n1",
		UserID:     "author",
		Visibility: model.NoteVisibilityPublic,
		Text:       &hello,
	}
	// Mock returns this row because it is public/home, but the post-filter
	// in core/note.CanSeeNote should keep it for an anonymous viewer.
	out, err := p.SearchNote(nil, "hello", SearchOpts{}, Pagination{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, out, 1)
}
