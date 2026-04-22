package repository

// paginationOrder returns the ORDER BY clause that matches the upstream
// Misskey QueryService.makePaginationQuery behaviour: sinceID-only queries
// flip to ASC so that cursor-based fetching in the newer direction keeps
// sequential ordering; every other combination (untilID only, both, neither)
// stays DESC.
//
// The idColumn argument is concatenated verbatim into the ORDER BY, so it
// should be a ready-to-use column reference such as "id" or `"note"."id"`.
// Example usage:
//
//	q := r.db.Order(paginationOrder(sinceID, untilID, "id"))
func paginationOrder(sinceID, untilID, idColumn string) string {
	if sinceID != "" && untilID == "" {
		return idColumn + " ASC"
	}
	return idColumn + " DESC"
}
