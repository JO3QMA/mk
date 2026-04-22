package repository

// paginationOrder returns the ORDER BY direction clause matching the upstream
// Misskey QueryService.makePaginationQuery behaviour: sinceID-only queries
// flip to ASC so that cursor-based fetching in the newer direction keeps
// sequential ordering; every other combination (untilID only, both, neither)
// stays DESC.
//
// 利用側は自分のテーブル名や quoting 付きカラム名と組み合わせて使う。例:
//
//	q := r.db.Order("note." + paginationOrder(sinceID, untilID, "id"))
//
// idColumn には quote済みのカラム表記を渡してそのまま連結する (e.g.
// `"id"` や `"note"."id"`)。
func paginationOrder(sinceID, untilID, idColumn string) string {
	if sinceID != "" && untilID == "" {
		return idColumn + " ASC"
	}
	return idColumn + " DESC"
}
