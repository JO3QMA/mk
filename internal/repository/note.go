package repository

import (
	"strconv"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// aidxTime2000Ms は aidx ID のタイムスタンプ部分 (最初の8文字 base36) の
// epoch 起点 (2000-01-01T00:00:00Z in ms since Unix epoch)。
// misc/id パッケージに同値の非公開定数があるが、本ファイル内で時刻→ID 先頭
// 変換に使うため独自に定義する。
const aidxTime2000Ms int64 = 946684800000

// aidxCutoffID は与えられた time に対応する最小のaidx ID文字列を返す。
// aidxは「時刻base36(8) + nodeID(4) + counter(4)」の 16 文字で、先頭 8 文字が
// ms-since-2000 を base36 で表したもの。そのため lexicographic 比較で時刻順に
// 並ぶ。時刻以降の全IDを 「id >= aidxCutoffID(t)」 で、時刻以前を
// 「id < aidxCutoffID(t)」 で拾える。
func aidxCutoffID(t time.Time) string {
	ms := t.UnixMilli() - aidxTime2000Ms
	if ms < 0 {
		ms = 0
	}
	prefix := strconv.FormatInt(ms, 36)
	if len(prefix) < 8 {
		prefix = strings.Repeat("0", 8-len(prefix)) + prefix
	} else if len(prefix) > 8 {
		// 想定外 (2089年以降) だが安全側で末尾8文字を使う
		prefix = prefix[len(prefix)-8:]
	}
	return prefix + "00000000"
}

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
	// DeleteExpiredRemoteNotes deletes up to batchSize remote notes older
	// than expiryDays in a single DELETE statement. Returns the count
	// actually removed in this batch. Callers drive the loop themselves so
	// cancellation checkpoints and sleep pacing live in the processor
	// (see CleanRemoteNotesProcessor).
	DeleteExpiredRemoteNotes(expiryDays, batchSize int) (int64, error)
	// DeleteByUserBatch deletes up to batchSize notes authored by userID in a
	// single DELETE statement. Returns the count actually removed. Callers
	// drive the loop themselves so that cancellation checkpoints and sleep
	// pacing live in the processor (see DeleteAccountProcessor).
	DeleteByUserBatch(userID string, batchSize int) (int64, error)
	// ListByUserList returns notes authored by members of the given user list,
	// ordered by id DESC with keyset pagination. Channel notes are excluded.
	ListByUserList(listID string, limit int, sinceID, untilID string) ([]*model.Note, error)
	// CountReplyTargets returns the users that userID most frequently replies
	// to, ordered by reply count descending. Used by
	// users/get-frequently-replied-users.
	CountReplyTargets(userID string, limit int) ([]model.ReplyTargetCount, error)
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

// ListByUserID returns the user's notes ordered by id, optionally
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
	if err := q.Order(paginationOrder(sinceID, untilID, "id")).Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListByChannelID returns notes posted to the channel.
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
	if err := q.Order(paginationOrder(sinceID, untilID, "id")).Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListRenotesOf returns notes whose renoteId equals noteID.
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
	if err := q.Order(paginationOrder(sinceID, untilID, "id")).Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// ListRepliesOf returns notes whose replyId equals noteID.
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
	if err := q.Order(paginationOrder(sinceID, untilID, "id")).Limit(limit).Find(&notes).Error; err != nil {
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
	if err := q.Order(paginationOrder(sinceID, untilID, "id")).Limit(limit).Find(&notes).Error; err != nil {
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
	if err := q.Order(paginationOrder(f.SinceID, f.UntilID, "id")).Limit(limit).Find(&notes).Error; err != nil {
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
		Order(paginationOrder(sinceID, untilID, "id")).Limit(limit)
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
		Order(paginationOrder(sinceID, untilID, "id")).Limit(limit)
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
		Order(paginationOrder(sinceID, untilID, `"id"`)).Limit(limit)
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
		Order(paginationOrder(sinceID, untilID, `"id"`)).Limit(limit)
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
		Order(paginationOrder(sinceID, untilID, `"id"`)).Limit(limit)
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

// DeleteExpiredRemoteNotes はリモートノート (userHost IS NOT NULL) のうち
// expiryDays より前に作成されたものを最大 batchSize 件削除し、実際に消えた
// 行数を返す。ループは呼び出し側 (CleanRemoteNotesProcessor) が sleep / ctx
// cancellation 付きで回す。
// ON DELETE CASCADE が reactions / replies 等に効く前提。
// misskey-go の note テーブルには createdAt カラムが無い (aidx ID から時刻を
// 導出する設計) ため、ID 文字列の lexicographic 比較で時刻切り捨てを行う。
func (r *noteRepository) DeleteExpiredRemoteNotes(expiryDays, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 100
	}
	cutoffID := aidxCutoffID(time.Now().Add(-time.Duration(expiryDays) * 24 * time.Hour))
	res := r.db.Exec(`
		DELETE FROM "note" WHERE id IN (
			SELECT id FROM "note"
			WHERE "userHost" IS NOT NULL
			  AND id < ?
			LIMIT ?
		)`, cutoffID, batchSize)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *noteRepository) DeleteByUserBatch(userID string, batchSize int) (int64, error) {
	if userID == "" {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	res := r.db.Exec(`
		DELETE FROM "note" WHERE id IN (
			SELECT id FROM "note"
			WHERE "userId" = ?
			LIMIT ?
		)`, userID, batchSize)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// ListByUserList returns notes authored by members of the given user list,
// ordered by id DESC with keyset pagination. user_list_membership テーブルと
// INNER JOIN してメンバーのノートだけを返す。channel ノートは除外する (TS互換)。
func (r *noteRepository) ListByUserList(listID string, limit int, sinceID, untilID string) ([]*model.Note, error) {
	if limit <= 0 {
		limit = 10
	}
	q := r.db.Preload("User").
		Joins(`INNER JOIN "user_list_membership" m ON m."userId" = "note"."userId" AND m."userListId" = ?`, listID).
		Where(`"note"."channelId" IS NULL`).
		Where(`"note"."visibility" IN ('public','home','followers')`)
	if sinceID != "" {
		q = q.Where(`"note"."id" > ?`, sinceID)
	}
	if untilID != "" {
		q = q.Where(`"note"."id" < ?`, untilID)
	}
	var notes []*model.Note
	if err := q.Order(paginationOrder(sinceID, untilID, `"note"."id"`)).Limit(limit).Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *noteRepository) CountReplyTargets(userID string, limit int) ([]model.ReplyTargetCount, error) {
	if limit <= 0 {
		limit = 10
	}
	var rows []model.ReplyTargetCount
	// replyUserIdがNULLのもの (通常起こり得ないが防御)と自己返信は集計から除外する。
	err := r.db.Model(&model.Note{}).
		Select(`"replyUserId", COUNT(*) AS count`).
		Where(`"userId" = ? AND "replyId" IS NOT NULL AND "replyUserId" IS NOT NULL AND "replyUserId" <> ?`, userID, userID).
		Group(`"replyUserId"`).
		Order(`count DESC`).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
