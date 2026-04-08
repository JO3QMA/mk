// Package chart implements the Misskey-compatible chart engine that
// aggregates time-series statistics into per-hour and per-day buckets.
//
// The engine is intentionally a faithful port of the upstream
// `core/chart/core.ts`: same column-name encoding (`___local_total`),
// same uniqueIncrement / intersection semantics, and the same buffer
// + 20-minute save loop strategy. Concrete chart definitions live in
// `internal/core/chart/charts/*` and are wired up by router.go.
package chart

import (
	"errors"
	"strings"
)

// Span identifies the bucket granularity of a chart record.
type Span string

const (
	// SpanHour buckets data into one record per UTC hour.
	SpanHour Span = "hour"
	// SpanDay buckets data into one record per UTC day.
	SpanDay Span = "day"
)

// ColRange describes the integer width of a chart column.
// Range は本家の `range: 'big' | 'medium' | 'small'` に対応し、
// 生成される SQL カラムの型 (bigint / integer / smallint) を決める。
type ColRange string

const (
	// RangeMedium is the default integer width and maps to PostgreSQL `integer`.
	RangeMedium ColRange = ""
	// RangeBig maps to PostgreSQL `bigint` and is used for cumulative totals
	// that may exceed 2^31.
	RangeBig ColRange = "big"
	// RangeSmall maps to PostgreSQL `smallint` and is used for delta counters
	// that never exceed 2^15.
	RangeSmall ColRange = "small"
)

// ColumnDef describes one logical column of a chart schema.
type ColumnDef struct {
	// Name is the dot-notation key (e.g. "local.diffs.normal"). It is the
	// API-facing identifier and is also encoded into the underlying SQL
	// column name via toColumnName().
	Name string
	// Accumulate, when true, copies the previous bucket's value forward
	// when a new bucket is created. Used for "running total" columns.
	Accumulate bool
	// Range chooses the SQL column type. RangeMedium is the default.
	Range ColRange
	// UniqueIncrement, when true, tracks a set of distinct string values
	// per bucket. The Chart engine bakes the set into a cardinality count
	// column on Save(); the raw set lives in a `unique_temp___<name>`
	// varchar[] column.
	UniqueIncrement bool
	// IntersectionOf names other UniqueIncrement columns whose sets are
	// intersected to produce this column's cardinality. The intersection
	// is also baked on Save().
	IntersectionOf []string
}

// Schema defines a chart: its name, whether it is grouped (per-user,
// per-host etc.), and the ordered list of columns.
type Schema struct {
	// Name is camelCase (e.g. "perUserNotes"). The underlying SQL table
	// names are derived from snake_case form: `__chart__per_user_notes`.
	Name string
	// Grouped indicates that the chart has a `group` varchar column
	// (e.g. user id, host) and that Commit/Tick/GetChart take a non-empty
	// group key.
	Grouped bool
	// Columns is the ordered list of column definitions. Order matters
	// for predictable SQL DDL generation.
	Columns []ColumnDef
}

// columnByName looks up a ColumnDef by its dot-notation Name. The lookup
// is linear because the schema column count is small (under 30 in
// practice).
func (s Schema) columnByName(name string) (ColumnDef, bool) {
	for _, c := range s.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return ColumnDef{}, false
}

// uniqueColumnNames returns the dot-notation names of all UniqueIncrement
// columns. Used by Clean() to know which `unique_temp___*` columns to
// reset.
func (s Schema) uniqueColumnNames() []string {
	out := make([]string, 0, len(s.Columns))
	for _, c := range s.Columns {
		if c.UniqueIncrement {
			out = append(out, c.Name)
		}
	}
	return out
}

// toColumnName encodes a dot-notation key into the SQL column name used
// by the chart tables. The encoding mirrors the upstream rule:
//
//	"local.diffs.normal" -> "___local_diffs_normal"
//
// The triple-underscore prefix matches the upstream `___` constant and
// keeps chart columns visually distinct from migration metadata columns
// such as `id` and `date`.
func toColumnName(name string) string {
	return columnPrefix + strings.ReplaceAll(name, ".", columnDelimiter)
}

// toUniqueTempColumnName encodes the temporary varchar[] column that
// stores raw unique values for a UniqueIncrement column.
func toUniqueTempColumnName(name string) string {
	return uniqueTempPrefix + strings.ReplaceAll(name, ".", columnDelimiter)
}

// fromColumnName reverses toColumnName by stripping the `___` prefix and
// replacing the column delimiter with dots. Used while reading rows back
// out of the database.
func fromColumnName(col string) (string, bool) {
	if !strings.HasPrefix(col, columnPrefix) {
		return "", false
	}
	rest := col[len(columnPrefix):]
	return strings.ReplaceAll(rest, columnDelimiter, "."), true
}

// HourTableName returns the SQL hour-bucket table name for the given
// chart name (camelCase). The result matches Misskey's TS implementation
// exactly: `notes` -> `__chart__notes`, `perUserNotes` -> `__chart__per_user_notes`.
func HourTableName(name string) string {
	return "__chart__" + camelToSnake(name)
}

// DayTableName returns the SQL day-bucket table name for the given chart
// name. `notes` -> `__chart_day__notes`.
func DayTableName(name string) string {
	return "__chart_day__" + camelToSnake(name)
}

// camelToSnake converts a camelCase identifier to snake_case using the
// same rule as Misskey TS: every uppercase letter is replaced with
// `_<lowercase>`. Note that this does not handle leading capitals (we
// never use them for chart names).
func camelToSnake(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SQLTypeOf returns the PostgreSQL column type for a given range. Used by
// the migration generator and by repository helpers when constructing
// CREATE TABLE statements in tests.
func SQLTypeOf(r ColRange) string {
	switch r {
	case RangeBig:
		return "bigint"
	case RangeSmall:
		return "smallint"
	default:
		return "integer"
	}
}

// Diff is a single Commit() payload: dot-notation key -> int delta or
// []string of unique values to add. The engine accepts either type per
// key but enforces consistency at Save() time via the schema definition.
type Diff map[string]any

// Result is the response shape returned by GetChart(). Each dot-notation
// key maps to a slice of length `amount` ordered oldest-first.
type Result map[string][]int64

// ErrSchemaName is returned when a Schema is constructed with an empty
// or invalid name.
var ErrSchemaName = errors.New("chart: schema name must be non-empty")

// columnPrefix and columnDelimiter mirror the upstream `___` and `_`
// encoding constants.
const (
	columnPrefix     = "___"
	uniqueTempPrefix = "unique_temp___"
	columnDelimiter  = "_"
)
