package chart

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// ErrRowNotFound is returned by Repository helpers when a lookup did not
// match any row. The chart engine treats this case the same as "no
// records yet" and falls back to interpolation logic, so callers must
// not surface this error to API responses.
var ErrRowNotFound = errors.New("chart: row not found")

// Row is the in-memory representation of a single chart record. The
// Cols map contains both `___<name>` integer columns (int64) and
// `unique_temp___<name>` varchar[] columns ([]string). Callers read
// values via the dot-notation key, which Repository implementations
// translate to/from raw column names.
type Row struct {
	ID    int64
	Date  int64
	Group sql.NullString
	// Cols maps the dot-notation column name to the stored value. Int
	// columns are int64, unique-temp columns are []string. Empty maps
	// are valid for newly-created buckets that have not yet been
	// updated.
	Cols map[string]any
}

// Repository abstracts the SQL access for one chart's `__chart__<name>`
// and `__chart_day__<name>` tables. The interface intentionally exposes
// span as a parameter so a single implementation can serve both spans.
type Repository interface {
	// FindCurrent returns the row with exactly the given timestamp, or
	// ErrRowNotFound when no row exists yet for that bucket.
	FindCurrent(ctx context.Context, span Span, group string, ts int64) (*Row, error)
	// FindLatest returns the most recent row (regardless of timestamp)
	// for the given group, or ErrRowNotFound when the group has never
	// been written. Used by claimCurrentLog to seed accumulate columns.
	FindLatest(ctx context.Context, span Span, group string) (*Row, error)
	// FindBefore returns the most recent row strictly before the given
	// timestamp. Used by GetChart to anchor interpolation when the
	// requested range starts in a gap.
	FindBefore(ctx context.Context, span Span, group string, ts int64) (*Row, error)
	// FindRange returns all rows whose timestamp lies in [gt, lt]
	// inclusive, ordered date DESC. Used by GetChart for the bulk of
	// the result.
	FindRange(ctx context.Context, span Span, group string, gt, lt int64) ([]*Row, error)
	// Insert creates a new row with the given timestamp/group and
	// initial column values. Returns the persisted row including the
	// generated id.
	Insert(ctx context.Context, span Span, group string, ts int64, cols map[string]any) (*Row, error)
	// ApplyDeltas applies the given int deltas (positive or negative)
	// and unique-temp appends to an existing row. Implementations must
	// use server-side arithmetic so concurrent writers do not lose
	// updates. The `setInts` map overrides absolute values for
	// uniqueIncrement / intersection columns where the engine has
	// already computed cardinality.
	ApplyDeltas(ctx context.Context, span Span, id int64, deltas map[string]int64, uniqueAppends map[string][]string, setInts map[string]int64) error
	// SetColumns replaces the integer values of the named columns
	// directly. Used by Tick() to overwrite tickMajor/tickMinor results.
	SetColumns(ctx context.Context, span Span, id int64, cols map[string]int64) error
	// ResetUniqueTempColumns clears the listed unique-temp columns to
	// the empty array for all rows whose timestamp falls in (gt, lt).
	// Used by Clean().
	ResetUniqueTempColumns(ctx context.Context, span Span, gt, lt int64, columns []string) error
}

// gormRepository is the production Repository backed by a *gorm.DB.
// One instance is constructed per chart and embeds the chart's Schema
// so it can translate dot-notation keys into the SQL column names.
type gormRepository struct {
	db     *gorm.DB
	schema Schema
}

// NewRepository builds a Repository for the given Schema. The schema is
// used to know the hour/day table names and to translate column names.
func NewRepository(db *gorm.DB, schema Schema) Repository {
	return &gormRepository{db: db, schema: schema}
}

func (r *gormRepository) tableFor(span Span) string {
	if span == SpanDay {
		return DayTableName(r.schema.Name)
	}
	return HourTableName(r.schema.Name)
}

// scanRow maps a single map[string]any returned by gorm raw query into
// a Row. Integer column values can come back as int64, int32 or
// []byte depending on the driver, so we coerce them defensively.
func (r *gormRepository) scanRow(raw map[string]any) *Row {
	row := &Row{Cols: make(map[string]any, len(raw))}
	for k, v := range raw {
		switch k {
		case "id":
			row.ID = toInt64(v)
		case "date":
			row.Date = toInt64(v)
		case "group":
			if v == nil {
				row.Group = sql.NullString{Valid: false}
			} else {
				row.Group = sql.NullString{Valid: true, String: fmt.Sprint(v)}
			}
		default:
			if name, ok := fromColumnName(k); ok {
				row.Cols[name] = v
				continue
			}
			if strings.HasPrefix(k, uniqueTempPrefix) {
				rest := k[len(uniqueTempPrefix):]
				name := strings.ReplaceAll(rest, columnDelimiter, ".")
				row.Cols[name+":unique"] = v
			}
		}
	}
	return row
}

// toInt64 coerces a database driver value (int / int64 / float64 / []byte)
// into int64. Defaults to zero on unknown types.
func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case []byte:
		var n int64
		fmt.Sscan(string(x), &n)
		return n
	default:
		return 0
	}
}

// findOne runs a parameterised SELECT * query and returns the first row
// or ErrRowNotFound. The callers always sort/filter so this helper does
// not impose ordering.
func (r *gormRepository) findOne(ctx context.Context, span Span, where string, args []any, order string) (*Row, error) {
	table := r.tableFor(span)
	q := fmt.Sprintf(`SELECT * FROM "%s" WHERE %s ORDER BY %s LIMIT 1`, table, where, order)
	var rows []map[string]any
	if err := r.db.WithContext(ctx).Raw(q, args...).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrRowNotFound
	}
	return r.scanRow(rows[0]), nil
}

func (r *gormRepository) FindCurrent(ctx context.Context, span Span, group string, ts int64) (*Row, error) {
	if r.schema.Grouped {
		return r.findOne(ctx, span, `"date" = ? AND "group" = ?`, []any{ts, group}, `"date" DESC`)
	}
	return r.findOne(ctx, span, `"date" = ?`, []any{ts}, `"date" DESC`)
}

func (r *gormRepository) FindLatest(ctx context.Context, span Span, group string) (*Row, error) {
	if r.schema.Grouped {
		return r.findOne(ctx, span, `"group" = ?`, []any{group}, `"date" DESC`)
	}
	return r.findOne(ctx, span, `1 = 1`, nil, `"date" DESC`)
}

func (r *gormRepository) FindBefore(ctx context.Context, span Span, group string, ts int64) (*Row, error) {
	if r.schema.Grouped {
		return r.findOne(ctx, span, `"date" < ? AND "group" = ?`, []any{ts, group}, `"date" DESC`)
	}
	return r.findOne(ctx, span, `"date" < ?`, []any{ts}, `"date" DESC`)
}

func (r *gormRepository) FindRange(ctx context.Context, span Span, group string, gt, lt int64) ([]*Row, error) {
	table := r.tableFor(span)
	var (
		q    string
		args []any
	)
	if r.schema.Grouped {
		q = fmt.Sprintf(`SELECT * FROM "%s" WHERE "date" BETWEEN ? AND ? AND "group" = ? ORDER BY "date" DESC`, table)
		args = []any{gt, lt, group}
	} else {
		q = fmt.Sprintf(`SELECT * FROM "%s" WHERE "date" BETWEEN ? AND ? ORDER BY "date" DESC`, table)
		args = []any{gt, lt}
	}
	var raws []map[string]any
	if err := r.db.WithContext(ctx).Raw(q, args...).Find(&raws).Error; err != nil {
		return nil, err
	}
	out := make([]*Row, len(raws))
	for i, raw := range raws {
		out[i] = r.scanRow(raw)
	}
	return out, nil
}

func (r *gormRepository) Insert(ctx context.Context, span Span, group string, ts int64, cols map[string]any) (*Row, error) {
	table := r.tableFor(span)
	colNames := []string{`"date"`}
	placeholders := []string{"?"}
	args := []any{ts}
	if r.schema.Grouped {
		colNames = append(colNames, `"group"`)
		placeholders = append(placeholders, "?")
		args = append(args, group)
	}
	// 列順を安定させて debugging しやすくする
	keys := make([]string, 0, len(cols))
	for k := range cols {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		v := cols[name]
		def, ok := r.schema.columnByName(name)
		if !ok {
			continue
		}
		colNames = append(colNames, fmt.Sprintf(`"%s"`, toColumnName(name)))
		placeholders = append(placeholders, "?")
		args = append(args, v)
		if def.UniqueIncrement {
			// uniqueIncrement 列は temp 配列も明示的に空で初期化する
			// (DEFAULT を持っているのでなくても良いが、明示することで Insert
			// 後の Row 状態と DB 状態をズラさない)
			colNames = append(colNames, fmt.Sprintf(`"%s"`, toUniqueTempColumnName(name)))
			placeholders = append(placeholders, "'{}'::varchar[]")
		}
	}
	q := fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s) RETURNING *`,
		table, strings.Join(colNames, ","), strings.Join(placeholders, ","),
	)
	var rows []map[string]any
	if err := r.db.WithContext(ctx).Raw(q, args...).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("chart: insert returned no row for %s", table)
	}
	return r.scanRow(rows[0]), nil
}

func (r *gormRepository) ApplyDeltas(ctx context.Context, span Span, id int64, deltas map[string]int64, uniqueAppends map[string][]string, setInts map[string]int64) error {
	if len(deltas) == 0 && len(uniqueAppends) == 0 && len(setInts) == 0 {
		return nil
	}
	table := r.tableFor(span)
	sets := make([]string, 0, len(deltas)+len(uniqueAppends)+len(setInts))
	args := make([]any, 0, len(deltas)+len(uniqueAppends)+len(setInts)+1)
	for _, name := range sortedKeysInt64(deltas) {
		col := toColumnName(name)
		sets = append(sets, fmt.Sprintf(`"%s" = "%s" + ?`, col, col))
		args = append(args, deltas[name])
	}
	for _, name := range sortedKeysStrings(uniqueAppends) {
		col := toUniqueTempColumnName(name)
		sets = append(sets, fmt.Sprintf(`"%s" = array_cat("%s", ?::varchar[])`, col, col))
		args = append(args, pgArrayLiteral(uniqueAppends[name]))
	}
	for _, name := range sortedKeysInt64(setInts) {
		col := toColumnName(name)
		sets = append(sets, fmt.Sprintf(`"%s" = ?`, col))
		args = append(args, setInts[name])
	}
	args = append(args, id)
	q := fmt.Sprintf(`UPDATE "%s" SET %s WHERE "id" = ?`, table, strings.Join(sets, ","))
	return r.db.WithContext(ctx).Exec(q, args...).Error
}

func (r *gormRepository) SetColumns(ctx context.Context, span Span, id int64, cols map[string]int64) error {
	if len(cols) == 0 {
		return nil
	}
	table := r.tableFor(span)
	sets := make([]string, 0, len(cols))
	args := make([]any, 0, len(cols)+1)
	for _, name := range sortedKeysInt64(cols) {
		sets = append(sets, fmt.Sprintf(`"%s" = ?`, toColumnName(name)))
		args = append(args, cols[name])
	}
	args = append(args, id)
	q := fmt.Sprintf(`UPDATE "%s" SET %s WHERE "id" = ?`, table, strings.Join(sets, ","))
	return r.db.WithContext(ctx).Exec(q, args...).Error
}

func (r *gormRepository) ResetUniqueTempColumns(ctx context.Context, span Span, gt, lt int64, columns []string) error {
	if len(columns) == 0 {
		return nil
	}
	table := r.tableFor(span)
	sets := make([]string, 0, len(columns))
	for _, name := range columns {
		sets = append(sets, fmt.Sprintf(`"%s" = '{}'::varchar[]`, toUniqueTempColumnName(name)))
	}
	q := fmt.Sprintf(`UPDATE "%s" SET %s WHERE "date" > ? AND "date" < ?`, table, strings.Join(sets, ","))
	return r.db.WithContext(ctx).Exec(q, gt, lt).Error
}

// pgArrayLiteral builds a PostgreSQL array literal string for a slice of
// strings. Values are wrapped in double quotes and embedded characters
// are escaped to keep the string parseable. Used by ApplyDeltas to feed
// the unique-temp `array_cat` operands.
func pgArrayLiteral(values []string) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, v := range values {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		// escape backslash and double-quote
		for _, r := range v {
			if r == '\\' || r == '"' {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func sortedKeysInt64(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeysStrings(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
