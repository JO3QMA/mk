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
	SearchByFilter(filter model.NoteSearchFilter) ([]*model.Note, error)
	ListFeatured(limit, offset int) ([]*model.Note, error)
	FindRenoteByUser(userID, renoteID string) (*model.Note, error)
	ListMentions(userID string, limit int, sinceID, untilID string) ([]*model.Note, error)
	SearchByTag(tag string, limit int, sinceID, untilID string) ([]*model.Note, error)
	ListByFileID(fileID string) ([]*model.Note, error)
	IncrementUserNotesCount(userID string, delta int) error
	ListHomeTimeline(userID string, limit int, sinceID, untilID string, filter model.TimelineDBFilter) ([]*model.Note, error)
	ListLocalTimeline(limit int, sinceID, untilID string, filter model.TimelineDBFilter) ([]*model.Note, error)
	ListGlobalTimeline(limit int, sinceID, untilID string, filter model.TimelineDBFilter) ([]*model.Note, error)
	// DeleteExpiredRemoteNotes deletes remote notes older than expiryDays
	// in batches of batchSize. Returns the total count of deleted notes.
	DeleteExpiredRemoteNotes(expiryDays, batchSize int) (int64, error)
	// DeleteByUser deletes every note authored by userID in chunks of
	// batchSize rows. Returns the total count. Designed for background
	// cascade deletion of user accounts so a single long transaction never
	// blocks the whole notes table.
	DeleteByUser(userID string, batchSize int) (int64, error)
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

// SearchByFilter returns notes matching the filter criteria.
// 検索バックエンド (core/search.SQLLikeProvider) から呼ばれる。
// テキストは ILIKE 部分一致、可視性は public/home に限定する。
func (r *noteRepository) SearchByFilter(f model.NoteSearchFilter) ([]*model.Note, error) {
	var notes []*model.Note
	q := r.db.Preload("User").
		Where("text ILIKE ?", "%"+f.Query+"%").
		Where("visibility IN ?", []string{
			string(model.NoteVisibilityPublic),
			string(model.NoteVisibilityHome),
		})
	if f.UserID != "" {
		q = q.Where("\"userId\" = ?", f.UserID)
	}
	if f.ChannelID != "" {
		q = q.Where("\"channelId\" = ?", f.ChannelID)
	}
	if f.Host != "" {
		// "." はローカル限定 (userHost が NULL のもの)
		if f.Host == "." {
			q = q.Where("\"userHost\" IS NULL")
		} else {
			q = q.Where("\"userHost\" = ?", f.Host)
		}
	}
	if f.UntilID != "" {
		q = q.Where("id < ?", f.UntilID)
	}
	if f.SinceID != "" {
		q = q.Where("id > ?", f.SinceID)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 10
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

func (r *noteRepository) ListFeatured(limit, offset int) ([]*model.Note, error) {
	if limit <= 0 {
		limit = 10
	}
	q := r.db.Preload("User").
		Where("visibility = 'public'").
		Order("(\"renoteCount\" + \"repliesCount\") DESC, id DESC").
		Limit(limit)
	if offset > 0 {
		q = q.Offset(offset)
	}
	var notes []*model.Note
	if err := q.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *noteRepository) FindRenoteByUser(userID, renoteID string) (*model.Note, error) {
	var note model.Note
	if err := r.db.Where("\"userId\" = ? AND \"renoteId\" = ? AND text IS NULL", userID, renoteID).
		Order("id DESC").First(&note).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *noteRepository) ListMentions(userID string, limit int, sinceID, untilID string) ([]*model.Note, error) {
	if limit <= 0 {
		limit = 10
	}
	q := r.db.Preload("User").
		Where("mentions @> ARRAY[?]::varchar[]", userID).
		Order("id DESC").Limit(limit)
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	var notes []*model.Note
	if err := q.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *noteRepository) SearchByTag(tag string, limit int, sinceID, untilID string) ([]*model.Note, error) {
	if limit <= 0 {
		limit = 10
	}
	q := r.db.Preload("User").
		Where("tags @> ARRAY[?]::varchar[]", tag).
		Order("id DESC").Limit(limit)
	if sinceID != "" {
		q = q.Where("id > ?", sinceID)
	}
	if untilID != "" {
		q = q.Where("id < ?", untilID)
	}
	var notes []*model.Note
	if err := q.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *noteRepository) ListByFileID(fileID string) ([]*model.Note, error) {
	var notes []*model.Note
	if err := r.db.Preload("User").Where(`"fileIds" @> ARRAY[?]::varchar[]`, fileID).Order(`"id" DESC`).Limit(20).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *noteRepository) IncrementUserNotesCount(userID string, delta int) error {
	return r.db.Exec(`UPDATE "user" SET "notesCount" = "notesCount" + ? WHERE "id" = ?`, delta, userID).Error
}

// applyTimelineFilter adds common filter conditions to a GORM query builder.
func applyTimelineFilter(q *gorm.DB, f model.TimelineDBFilter) *gorm.DB {
	if f.WithFiles {
		q = q.Where(`"fileIds" != '{}'`)
	}
	if f.WithRenotes != nil && !*f.WithRenotes {
		// pure renote (テキストもファイルもない renote) を除外
		q = q.Where(`NOT ("renoteId" IS NOT NULL AND text IS NULL AND "fileIds" = '{}')`)
	}
	if f.WithReplies != nil && !*f.WithReplies {
		if f.ViewerID != "" {
			// 自分への返信は残す
			q = q.Where(`("replyId" IS NULL OR "replyUserId" = ?)`, f.ViewerID)
		} else {
			q = q.Where(`"replyId" IS NULL`)
		}
	}
	if f.IncludeMyRenotes != nil && !*f.IncludeMyRenotes && f.ViewerID != "" {
		q = q.Where(`NOT ("renoteId" IS NOT NULL AND text IS NULL AND "fileIds" = '{}' AND "userId" = ?)`, f.ViewerID)
	}
	if f.IncludeRenotedMyNotes != nil && !*f.IncludeRenotedMyNotes && f.ViewerID != "" {
		q = q.Where(`NOT ("renoteId" IS NOT NULL AND text IS NULL AND "fileIds" = '{}' AND "renoteUserId" = ?)`, f.ViewerID)
	}
	if f.IncludeLocalRenotes != nil && !*f.IncludeLocalRenotes {
		q = q.Where(`NOT ("renoteId" IS NOT NULL AND text IS NULL AND "fileIds" = '{}' AND "renoteUserHost" IS NULL)`)
	}
	if len(f.MutedChannelIDs) > 0 {
		// channel_mutingに登録されたチャンネルのノートはタイムラインから除外する。
		// GORMは連続.WhereをANDで繋ぐがraw SQL側のORはデフォルトでは
		// 括弧で囲まれないので、明示的に囲まないとAND側の他フィルタを
		// バイパスしてしまう (SQL優先順位: AND > OR)。
		q = q.Where(`("channelId" IS NULL OR "channelId" NOT IN ?)`, f.MutedChannelIDs)
	}
	return q
}

// ListHomeTimeline returns notes by the user and users they follow.
// DBフォールバック用。Redisが空のときに使う。
func (r *noteRepository) ListHomeTimeline(userID string, limit int, sinceID, untilID string, filter model.TimelineDBFilter) ([]*model.Note, error) {
	q := r.db.Preload("User").
		Where(`("userId" = ? OR "userId" IN (SELECT "followeeId" FROM "following" WHERE "followerId" = ?)) AND "visibility" IN ('public','home','followers')`, userID, userID).
		Order(`"id" DESC`).Limit(limit)
	if sinceID != "" {
		q = q.Where(`"id" > ?`, sinceID)
	}
	if untilID != "" {
		q = q.Where(`"id" < ?`, untilID)
	}
	q = applyTimelineFilter(q, filter)
	var notes []*model.Note
	if err := q.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListLocalTimeline returns public/home notes by local users.
func (r *noteRepository) ListLocalTimeline(limit int, sinceID, untilID string, filter model.TimelineDBFilter) ([]*model.Note, error) {
	q := r.db.Preload("User").
		Where(`"userHost" IS NULL AND "visibility" = 'public' AND "channelId" IS NULL`).
		Order(`"id" DESC`).Limit(limit)
	if sinceID != "" {
		q = q.Where(`"id" > ?`, sinceID)
	}
	if untilID != "" {
		q = q.Where(`"id" < ?`, untilID)
	}
	q = applyTimelineFilter(q, filter)
	var notes []*model.Note
	if err := q.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListGlobalTimeline returns all public notes.
// channelId IS NULL でチャンネルノートを除外する (TS互換)。
func (r *noteRepository) ListGlobalTimeline(limit int, sinceID, untilID string, filter model.TimelineDBFilter) ([]*model.Note, error) {
	q := r.db.Preload("User").
		Where(`"visibility" = 'public' AND "channelId" IS NULL`).
		Order(`"id" DESC`).Limit(limit)
	if sinceID != "" {
		q = q.Where(`"id" > ?`, sinceID)
	}
	if untilID != "" {
		q = q.Where(`"id" < ?`, untilID)
	}
	q = applyTimelineFilter(q, filter)
	var notes []*model.Note
	if err := q.Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// DeleteExpiredRemoteNotes はリモート���ート (userHost IS NOT NULL) のうち
// expiryDays より前に作���されたものを batchSize 件ずつ削除する。
// ON DELETE CASCADE が reactions / replies 等に効く前提。
func (r *noteRepository) DeleteExpiredRemoteNotes(expiryDays, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 100
	}
	var total int64
	for {
		res := r.db.Exec(`
			DELETE FROM "note" WHERE id IN (
				SELECT id FROM "note"
				WHERE "userHost" IS NOT NULL
				  AND "createdAt" < NOW() - INTERVAL '1 day' * ?
				LIMIT ?
			)`, expiryDays, batchSize)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(batchSize) {
			break
		}
	}
	return total, nil
}

func (r *noteRepository) DeleteByUser(userID string, batchSize int) (int64, error) {
	if userID == "" {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	var total int64
	for {
		res := r.db.Exec(`
			DELETE FROM "note" WHERE id IN (
				SELECT id FROM "note"
				WHERE "userId" = ?
				LIMIT ?
			)`, userID, batchSize)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(batchSize) {
			break
		}
	}
	return total, nil
}
