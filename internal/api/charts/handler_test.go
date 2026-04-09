package charts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shiroha-a/mk/internal/core/chart"
	corecharts "github.com/shiroha-a/mk/internal/core/chart/charts"
	"github.com/shiroha-a/mk/internal/model"
)

// fakeRepo mirrors the package-internal fake from
// internal/core/chart/charts/fake_repo_test.go but lives here so the
// handler tests can build a real *chart.Chart and exercise the engine
// end-to-end without spinning up postgres.
type fakeRepo struct {
	mu     sync.Mutex
	nextID int64
	hour   map[string][]*chart.Row
	day    map[string][]*chart.Row
	failOn map[string]error // method name -> error to return
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		hour:   make(map[string][]*chart.Row),
		day:    make(map[string][]*chart.Row),
		failOn: make(map[string]error),
	}
}

func (r *fakeRepo) tableFor(span chart.Span) map[string][]*chart.Row {
	if span == chart.SpanDay {
		return r.day
	}
	return r.hour
}

func (r *fakeRepo) FindCurrent(_ context.Context, span chart.Span, group string, ts int64) (*chart.Row, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failOn["FindCurrent"]; err != nil {
		return nil, err
	}
	for _, row := range r.tableFor(span)[group] {
		if row.Date == ts {
			return row, nil
		}
	}
	return nil, chart.ErrRowNotFound
}

func (r *fakeRepo) FindLatest(_ context.Context, span chart.Span, group string) (*chart.Row, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failOn["FindLatest"]; err != nil {
		return nil, err
	}
	rows := r.tableFor(span)[group]
	if len(rows) == 0 {
		return nil, chart.ErrRowNotFound
	}
	latest := rows[0]
	for _, row := range rows[1:] {
		if row.Date > latest.Date {
			latest = row
		}
	}
	return latest, nil
}

func (r *fakeRepo) FindBefore(_ context.Context, span chart.Span, group string, ts int64) (*chart.Row, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failOn["FindBefore"]; err != nil {
		return nil, err
	}
	var best *chart.Row
	for _, row := range r.tableFor(span)[group] {
		if row.Date < ts {
			if best == nil || row.Date > best.Date {
				best = row
			}
		}
	}
	if best == nil {
		return nil, chart.ErrRowNotFound
	}
	return best, nil
}

func (r *fakeRepo) FindRange(_ context.Context, span chart.Span, group string, gt, lt int64) ([]*chart.Row, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failOn["FindRange"]; err != nil {
		return nil, err
	}
	var out []*chart.Row
	for _, row := range r.tableFor(span)[group] {
		if row.Date >= gt && row.Date <= lt {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, nil
}

func (r *fakeRepo) Insert(_ context.Context, span chart.Span, group string, ts int64, cols map[string]any) (*chart.Row, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	row := &chart.Row{
		ID:   r.nextID,
		Date: ts,
		Cols: make(map[string]any, len(cols)),
	}
	if group != "" {
		row.Group = sql.NullString{Valid: true, String: group}
	}
	maps.Copy(row.Cols, cols)
	r.tableFor(span)[group] = append(r.tableFor(span)[group], row)
	return row, nil
}

func (r *fakeRepo) ApplyDeltas(_ context.Context, span chart.Span, id int64, deltas map[string]int64, uniqueAppends map[string][]string, setInts map[string]int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.findByID(span, id)
	if row == nil {
		return fmt.Errorf("fakeRepo: row %d not found", id)
	}
	for k, v := range deltas {
		row.Cols[k] = toInt64(row.Cols[k]) + v
	}
	for k, items := range uniqueAppends {
		key := k + ":unique"
		cur, _ := row.Cols[key].([]string)
		row.Cols[key] = append(cur, items...)
	}
	for k, v := range setInts {
		row.Cols[k] = v
	}
	return nil
}

func (r *fakeRepo) SetColumns(_ context.Context, span chart.Span, id int64, cols map[string]int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.findByID(span, id)
	if row == nil {
		return fmt.Errorf("fakeRepo: row %d not found", id)
	}
	for k, v := range cols {
		row.Cols[k] = v
	}
	return nil
}

func (r *fakeRepo) ResetUniqueTempColumns(_ context.Context, span chart.Span, gt, lt int64, columns []string) error {
	return nil
}

func (r *fakeRepo) findByID(span chart.Span, id int64) *chart.Row {
	for _, rows := range r.tableFor(span) {
		for _, row := range rows {
			if row.ID == id {
				return row
			}
		}
	}
	return nil
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	default:
		return 0
	}
}

// fakeClock returns a fixed UTC instant.
type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

// --- helpers ----------------------------------------------------------------

func newEngine(t *testing.T, schema chart.Schema) (*chart.Chart, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	c, err := chart.New(chart.Config{
		Schema: schema,
		Repo:   repo,
		Lock:   chart.NewMemoryLocker(),
		Clock:  fakeClock{now: time.Date(2026, 4, 9, 12, 30, 0, 0, time.UTC)},
	})
	require.NoError(t, err)
	return c, repo
}

func newHandlerWithAllCharts(t *testing.T) (*Handler, Charts) {
	t.Helper()
	notes, _ := newEngine(t, corecharts.SchemaNotes())
	users, _ := newEngine(t, corecharts.SchemaUsers())
	drive, _ := newEngine(t, corecharts.SchemaDrive())
	federation, _ := newEngine(t, corecharts.SchemaFederation())
	instance, _ := newEngine(t, corecharts.SchemaInstance())
	apReq, _ := newEngine(t, corecharts.SchemaApRequest())
	active, _ := newEngine(t, corecharts.SchemaActiveUsers())
	puNotes, _ := newEngine(t, corecharts.SchemaPerUserNotes())
	puDrive, _ := newEngine(t, corecharts.SchemaPerUserDrive())
	puFollow, _ := newEngine(t, corecharts.SchemaPerUserFollowing())
	puPv, _ := newEngine(t, corecharts.SchemaPerUserPv())
	puReact, _ := newEngine(t, corecharts.SchemaPerUserReaction())
	c := Charts{
		Notes:            notes,
		Users:            users,
		Drive:            drive,
		Federation:       federation,
		Instance:         instance,
		ApRequest:        apReq,
		ActiveUsers:      active,
		PerUserNotes:     puNotes,
		PerUserDrive:     puDrive,
		PerUserFollowing: puFollow,
		PerUserPv:        puPv,
		PerUserReaction:  puReact,
	}
	h := NewHandler(c, fakeClock{now: time.Date(2026, 4, 9, 12, 30, 0, 0, time.UTC)})
	return h, c
}

func newReq(t *testing.T, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// --- instance-wide endpoints ------------------------------------------------

func TestNewHandler_NilClockUsesSystem(t *testing.T) {
	// nil clock を渡しても panic せず offset 計算が成立する。
	notes, _ := newEngine(t, corecharts.SchemaNotes())
	h := NewHandler(Charts{Notes: notes}, nil)
	c, rec := newReq(t, `{"span":"hour","offset":1}`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestInstanceEndpoints_HappyPath(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	cases := []struct {
		name string
		fn   echo.HandlerFunc
		body string
	}{
		{"notes", h.Notes, `{"span":"hour"}`},
		{"users", h.Users, `{"span":"day","limit":7}`},
		{"drive", h.Drive, `{"span":"hour","limit":24,"offset":0}`},
		{"federation", h.Federation, `{"span":"day"}`},
		{"apRequest", h.ApRequest, `{"span":"hour"}`},
		{"activeUsers", h.ActiveUsers, `{"span":"day"}`},
		{"instance", h.Instance, `{"span":"hour","host":"example.com"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newReq(t, tc.body)
			require.NoError(t, tc.fn(c))
			assert.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
			var out map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		})
	}
}

func TestInstanceEndpoint_HostRequired(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{"span":"hour"}`)
	require.NoError(t, h.Instance(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInstanceEndpoint_NilChartUnavailable(t *testing.T) {
	h := NewHandler(Charts{}, fakeClock{now: time.Now().UTC()})
	c, rec := newReq(t, `{"span":"hour"}`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestInstanceEndpoint_BadJSON(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{not json`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInstanceEndpoint_RepoError(t *testing.T) {
	notes, repo := newEngine(t, corecharts.SchemaNotes())
	repo.failOn["FindRange"] = errors.New("boom")
	h := NewHandler(Charts{Notes: notes}, fakeClock{now: time.Now().UTC()})
	c, rec := newReq(t, `{"span":"hour"}`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- per-user endpoints -----------------------------------------------------

func TestPerUserEndpoints_HappyPath(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	cases := []struct {
		name string
		fn   echo.HandlerFunc
	}{
		{"notes", h.UserNotes},
		{"drive", h.UserDrive},
		{"following", h.UserFollowing},
		{"pv", h.UserPv},
		{"reactions", h.UserReactions},
	}
	body := `{"span":"day","limit":3,"userId":"alice"}`
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newReq(t, body)
			require.NoError(t, tc.fn(c))
			assert.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		})
	}
}

func TestPerUserEndpoint_UserIDRequired(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{"span":"day"}`)
	require.NoError(t, h.UserNotes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPerUserEndpoint_BadJSON(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{not json`)
	require.NoError(t, h.UserNotes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPerUserEndpoint_NilChartUnavailable(t *testing.T) {
	h := NewHandler(Charts{}, fakeClock{now: time.Now().UTC()})
	c, rec := newReq(t, `{"span":"day","userId":"alice"}`)
	require.NoError(t, h.UserNotes(c))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestPerUserEndpoint_BadSpan(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{"span":"week","userId":"alice"}`)
	require.NoError(t, h.UserNotes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPerUserEndpoint_RepoError(t *testing.T) {
	puNotes, repo := newEngine(t, corecharts.SchemaPerUserNotes())
	repo.failOn["FindRange"] = errors.New("boom")
	h := NewHandler(Charts{PerUserNotes: puNotes}, fakeClock{now: time.Now().UTC()})
	c, rec := newReq(t, `{"span":"day","userId":"alice"}`)
	require.NoError(t, h.UserNotes(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- shared validation ------------------------------------------------------

func TestParseRequest_BadSpan(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{"span":"week"}`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestParseRequest_LimitOutOfRange(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	for _, body := range []string{
		`{"span":"hour","limit":0}`,
		`{"span":"hour","limit":501}`,
		`{"span":"hour","limit":-1}`,
	} {
		c, rec := newReq(t, body)
		require.NoError(t, h.Notes(c))
		assert.Equalf(t, http.StatusBadRequest, rec.Code, "body=%s", body)
	}
}

func TestParseRequest_NegativeOffset(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{"span":"hour","offset":-3}`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestParseRequest_OffsetCursorDay(t *testing.T) {
	// offset > 0 のとき cursor が span 単位で過去にずれることを
	// fakeClock とともに確認する。実際の値は handler 内部に閉じている
	// が、リクエスト自体が 200 を返せばパスを通っている。
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{"span":"day","offset":2,"limit":5}`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestParseRequest_OffsetCursorHour(t *testing.T) {
	h, _ := newHandlerWithAllCharts(t)
	c, rec := newReq(t, `{"span":"hour","offset":1,"limit":5}`)
	require.NoError(t, h.Notes(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestResponseShape_NestedObject(t *testing.T) {
	// 1 件 commit して Save した上でハンドラを叩き、
	// nested object 形式で返ることを確認する。
	notes, _ := newEngine(t, corecharts.SchemaNotes())
	wrapper := corecharts.NewNotesChart(notes)
	require.NoError(t, wrapper.Update(&model.Note{ID: "n1"}, true))
	require.NoError(t, notes.Save(context.Background()))

	h := NewHandler(Charts{Notes: notes}, fakeClock{now: time.Date(2026, 4, 9, 12, 30, 0, 0, time.UTC)})
	c, rec := newReq(t, `{"span":"hour","limit":1}`)
	require.NoError(t, h.Notes(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	local, ok := out["local"].(map[string]any)
	require.True(t, ok, "expected nested local map: %s", rec.Body.String())
	totals, ok := local["total"].([]any)
	require.True(t, ok)
	require.Len(t, totals, 1)
	// 直近バケットに 1 件記録されているはず
	assert.EqualValues(t, 1, totals[0])
}
