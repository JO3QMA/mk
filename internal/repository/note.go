package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// NoteRepository provides data access for notes.
type NoteRepository interface {
	Create(note *model.Note) error
	FindByID(id string) (*model.Note, error)
	FindByIDWithUser(id string) (*model.Note, error)
	FindByURI(uri string) (*model.Note, error)
	Delete(note *model.Note) error
	Update(note *model.Note, column string, value any) error
	UpdateFields(noteID string, fields map[string]any) error
	IncrementCount(noteID, column string, delta int) error
	IncrementReaction(noteID, reaction string, delta int) error
	ListByUserID(userID string, untilID, sinceID string, limit int) ([]*model.Note, error)
	ListByChannelID(channelID string, untilID, sinceID string, limit int) ([]*model.Note, error)
	FindManyByIDsWithUser(ids []string) ([]*model.Note, error)
	ListRenotesOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error)
	ListRepliesOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error)
	ListChildrenOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error)
	Search(query string, untilID, sinceID string, limit int) ([]*model.Note, error)
}

type noteRepository struct {
	db *gorm.DB
}

// NewNoteRepository creates a new NoteRepository.
func NewNoteRepository(db *gorm.DB) NoteRepository {
	return &noteRepository{db: db}
}

func (r *noteRepository) Create(note *model.Note) error {
	return r.db.Create(note).Error
}

func (r *noteRepository) FindByID(id string) (*model.Note, error) {
	var note model.Note
	if err := r.db.First(&note, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *noteRepository) FindByIDWithUser(id string) (*model.Note, error) {
	var note model.Note
	if err := r.db.Preload("User").First(&note, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

// FindByURI looks up a note by its ActivityPub URI. リモート由来の note は
// uri 列に作成元のIRIが入っているため、配信や inbox 処理での重複検出に使う。
func (r *noteRepository) FindByURI(uri string) (*model.Note, error) {
	var note model.Note
	if err := r.db.Where("uri = ?", uri).First(&note).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *noteRepository) Delete(note *model.Note) error {
	return r.db.Delete(note).Error
}

func (r *noteRepository) Update(note *model.Note, column string, value any) error {
	return r.db.Model(note).Update(column, value).Error
}

// UpdateFields applies a map of column → value updates to the note row keyed
// by id. Update (single column) は既存呼び出し互換のため残す。
func (r *noteRepository) UpdateFields(noteID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.Note{}).Where("id = ?", noteID).Updates(fields).Error
}

// IncrementCount adjusts a counter column on the note row by delta.
// 集計列の更新はGORMのUpdateColumnでSQL式を直接適用する。
func (r *noteRepository) IncrementCount(noteID, column string, delta int) error {
	return r.db.Model(&model.Note{}).
		Where("id = ?", noteID).
		UpdateColumn(column, gorm.Expr("\""+column+"\" + ?", delta)).Error
}

// IncrementReaction increments (or decrements when delta<0) the value of a
// reaction key inside the note.reactions JSONB object. 0以下になったキーは
// JSONBオブジェクトから完全に削除される。
//
// 結果値が正なら jsonb_set でカウントを上書き、0以下なら "reactions - key" で
// キーごと削除する。 CASE 式で1クエリにまとめている。
func (r *noteRepository) IncrementReaction(noteID, reaction string, delta int) error {
	expr := "CASE WHEN COALESCE((reactions->>?)::int, 0) + ? > 0 " +
		"THEN jsonb_set(reactions, ?, to_jsonb(COALESCE((reactions->>?)::int, 0) + ?)::jsonb, true) " +
		"ELSE reactions - ?::text END"
	pathArr := "{" + reaction + "}"
	return r.db.Model(&model.Note{}).
		Where("id = ?", noteID).
		UpdateColumn("reactions",
			gorm.Expr(expr, reaction, delta, pathArr, reaction, delta, reaction)).Error
}

// ListByUserID returns the user's notes ordered by id DESC, optionally
// constrained by sinceID/untilID for keyset pagination.
func (r *noteRepository) ListByUserID(userID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	var notes []*model.Note
	q := r.db.Preload("User").Where("\"userId\" = ?", userID)
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if err := q.Order("id DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListByChannelID returns notes posted to the channel ordered by id DESC.
func (r *noteRepository) ListByChannelID(channelID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	var notes []*model.Note
	q := r.db.Preload("User").Where("\"channelId\" = ?", channelID)
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if limit <= 0 {
		limit = 30
	}
	if err := q.Order("id DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListRenotesOf returns notes whose renoteId equals noteID, ordered by id DESC.
// テキストやファイルを伴わない pure renote だけでなく quote renote も含む。
func (r *noteRepository) ListRenotesOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	var notes []*model.Note
	q := r.db.Preload("User").Where("\"renoteId\" = ?", noteID)
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if err := q.Order("id DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListRepliesOf returns notes whose replyId equals noteID, ordered by id DESC.
// すべてのユーザーからの返信を返す(ミュート判定はServiceで行う)。
func (r *noteRepository) ListRepliesOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	var notes []*model.Note
	q := r.db.Preload("User").Where("\"replyId\" = ?", noteID)
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if err := q.Order("id DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListChildrenOf returns notes that are either replies or quote-renotes of the given noteID.
// notes/childrenでスレッドツリーの直下を取得するために使用する。
func (r *noteRepository) ListChildrenOf(noteID string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	var notes []*model.Note
	q := r.db.Preload("User").
		Where("\"replyId\" = ? OR \"renoteId\" = ?", noteID, noteID)
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if err := q.Order("id DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// Search returns notes whose text matches the given query (case-insensitive substring search).
// Phase 4でMeilisearchへ置き換える前提のシンプルなDB検索。
func (r *noteRepository) Search(query string, untilID, sinceID string, limit int) ([]*model.Note, error) {
	var notes []*model.Note
	q := r.db.Preload("User").
		Where("text ILIKE ?", "%"+query+"%").
		Where("visibility IN ?", []string{
			string(model.NoteVisibilityPublic),
			string(model.NoteVisibilityHome),
		})
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if err := q.Order("id DESC").Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// FindManyByIDsWithUser returns the requested notes preserving the order of `ids`.
// Notes that are not found are simply omitted from the result.
func (r *noteRepository) FindManyByIDsWithUser(ids []string) ([]*model.Note, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var notes []*model.Note
	if err := r.db.Preload("User").Where("id IN ?", ids).Find(&notes).Error; err != nil {
		return nil, err
	}
	// idsの順序を保つため、idでマップ化してから並び替える
	byID := make(map[string]*model.Note, len(notes))
	for _, n := range notes {
		byID[n.ID] = n
	}
	ordered := make([]*model.Note, 0, len(ids))
	for _, id := range ids {
		if n, ok := byID[id]; ok {
			ordered = append(ordered, n)
		}
	}
	return ordered, nil
}
