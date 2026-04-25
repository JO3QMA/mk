package retention

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// --- stubs ---

type stubUserRepo struct {
	// idGen.Generate(t) には random suffix が含まれて呼び出しごとに値が
	// 変わるので、cursor 一致比較ではなく「直近登録ユーザー一覧」を直接
	// 持たせる。
	registered []string
	active     []string
}

func (s *stubUserRepo) ListLocalUserIDsRegisteredAfter(_ string) ([]string, error) {
	return append([]string(nil), s.registered...), nil
}

func (s *stubUserRepo) ListLocalUserIDsActiveSince(_ time.Time) ([]string, error) {
	return append([]string(nil), s.active...), nil
}

// 残りの UserRepository メソッドは aggregation で使わないので no-op で
// 満たす。テスト側は service.go が呼ぶメソッドだけ正しく動けば十分。
func (s *stubUserRepo) Create(*model.User) error                                 { return nil }
func (s *stubUserRepo) FindByID(string) (*model.User, error)                     { return nil, nil }
func (s *stubUserRepo) FindByToken(string) (*model.User, error)                  { return nil, nil }
func (s *stubUserRepo) FindByURI(string) (*model.User, error)                    { return nil, nil }
func (s *stubUserRepo) FindByUsernameLower(string, *string) (*model.User, error) { return nil, nil }
func (s *stubUserRepo) FindProfileByUserID(string) (*model.UserProfile, error)   { return nil, nil }
func (s *stubUserRepo) IncrementFollowingCount(string, int) error                { return nil }
func (s *stubUserRepo) IncrementFollowersCount(string, int) error                { return nil }
func (s *stubUserRepo) SearchByUsername(string, int, int) ([]*model.User, error) { return nil, nil }
func (s *stubUserRepo) UpdateUser(string, map[string]any) error                  { return nil }
func (s *stubUserRepo) UpdateProfile(string, map[string]any) error               { return nil }
func (s *stubUserRepo) CreateProfile(*model.UserProfile) error                   { return nil }
func (s *stubUserRepo) ListUsers(model.UserListFilter) ([]*model.User, error)    { return nil, nil }
func (s *stubUserRepo) ListRemoteInboxes() ([]string, error)                     { return nil, nil }
func (s *stubUserRepo) FindProfileByVerifyCode(string) (*model.UserProfile, error) {
	return nil, nil
}
func (s *stubUserRepo) FindProfileByEmail(string) (*model.UserProfile, error) { return nil, nil }
func (s *stubUserRepo) CountOnlineUsers() (int64, error)                      { return 0, nil }
func (s *stubUserRepo) CountLocalUsers() (int64, error)                       { return 0, nil }
func (s *stubUserRepo) CountLocalUsersActiveSince(time.Time) (int64, error)   { return 0, nil }
func (s *stubUserRepo) ListUserRecommendations(string, time.Time, int, int) ([]*model.User, error) {
	return nil, nil
}

type stubRetentionRepo struct {
	rows            map[string]*model.RetentionAggregation // dateKey -> row
	insertErr       error
	updateCalls     int
	insertedDateKey string
}

func newStubRetentionRepo() *stubRetentionRepo {
	return &stubRetentionRepo{rows: map[string]*model.RetentionAggregation{}}
}

func (s *stubRetentionRepo) ListRecent(int) ([]*model.RetentionAggregation, error) {
	return nil, nil
}

func (s *stubRetentionRepo) ListSince(_ time.Time) ([]*model.RetentionAggregation, error) {
	out := make([]*model.RetentionAggregation, 0, len(s.rows))
	for _, r := range s.rows {
		out = append(out, r)
	}
	return out, nil
}

func (s *stubRetentionRepo) FindByDateKey(dateKey string) (*model.RetentionAggregation, error) {
	if r, ok := s.rows[dateKey]; ok {
		return r, nil
	}
	return nil, nil
}

func (s *stubRetentionRepo) Insert(row *model.RetentionAggregation) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	if _, exists := s.rows[row.DateKey]; exists {
		return repository.ErrDuplicateKey
	}
	s.rows[row.DateKey] = row
	s.insertedDateKey = row.DateKey
	return nil
}

func (s *stubRetentionRepo) Update(id string, updatedAt time.Time, data datatypes.JSON) error {
	s.updateCalls++
	for _, r := range s.rows {
		if r.ID == id {
			r.UpdatedAt = updatedAt
			r.Data = data
			return nil
		}
	}
	return nil
}

// --- tests ---

func TestService_Aggregate_InsertsTodayRow(t *testing.T) {
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	idGen, err := id.NewGenerator("aidx")
	require.NoError(t, err)

	users := &stubUserRepo{registered: []string{"u1", "u2"}}
	retentions := newStubRetentionRepo()

	svc := NewService(users, retentions, idGen)
	svc.SetClock(func() time.Time { return now })

	require.NoError(t, svc.Aggregate(context.Background()))

	row := retentions.rows["2026-4-25"]
	require.NotNil(t, row, "today's row must be inserted")
	assert.Equal(t, 2, row.UsersCount)
	assert.Equal(t, pq.StringArray{"u1", "u2"}, row.UserIDs)
}

func TestService_Aggregate_DuplicateDateKeyIsSkipped(t *testing.T) {
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	idGen, err := id.NewGenerator("aidx")
	require.NoError(t, err)

	retentions := newStubRetentionRepo()
	// today already inserted by another worker
	retentions.rows["2026-4-25"] = &model.RetentionAggregation{
		ID:      "existing",
		DateKey: "2026-4-25",
		Data:    datatypes.JSON([]byte("{}")),
	}

	svc := NewService(&stubUserRepo{}, retentions, idGen)
	svc.SetClock(func() time.Time { return now })

	require.NoError(t, svc.Aggregate(context.Background()))
	// Insert は ErrDuplicateKey で吸収されるので Update は走らない (今日の
	// 行に対する self-skip だけが起きるため)。
	assert.Equal(t, 0, retentions.updateCalls)
}

func TestService_Aggregate_UpdatesPastCohorts(t *testing.T) {
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	idGen, err := id.NewGenerator("aidx")
	require.NoError(t, err)

	users := &stubUserRepo{
		registered: []string{"newcomer"},
		active:     []string{"u_yesterday_active", "newcomer"},
	}
	retentions := newStubRetentionRepo()
	// Yesterday の cohort: 2 ユーザー登録、うち 1 人だけ今日アクティブ
	retentions.rows["2026-4-24"] = &model.RetentionAggregation{
		ID:        "row-yesterday",
		DateKey:   "2026-4-24",
		UserIDs:   pq.StringArray{"u_yesterday_active", "u_yesterday_dropped"},
		Data:      datatypes.JSON([]byte("{}")),
		CreatedAt: now.Add(-24 * time.Hour),
	}

	svc := NewService(users, retentions, idGen)
	svc.SetClock(func() time.Time { return now })

	require.NoError(t, svc.Aggregate(context.Background()))

	yesterday := retentions.rows["2026-4-24"]
	require.NotNil(t, yesterday)
	var data map[string]int
	require.NoError(t, json.Unmarshal(yesterday.Data, &data))
	assert.Equal(t, 1, data["2026-4-25"], "1 of 2 yesterday-cohort users active today")
}

func TestService_Aggregate_DoesNotUpdateTodayCohortItself(t *testing.T) {
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	idGen, err := id.NewGenerator("aidx")
	require.NoError(t, err)

	users := &stubUserRepo{
		registered: []string{"newbie"},
		active:     []string{"newbie"},
	}
	retentions := newStubRetentionRepo()

	svc := NewService(users, retentions, idGen)
	svc.SetClock(func() time.Time { return now })

	require.NoError(t, svc.Aggregate(context.Background()))

	row := retentions.rows["2026-4-25"]
	require.NotNil(t, row)
	// Today's data starts as `{}` and stays empty — service skips self-update.
	assert.Equal(t, "{}", string(row.Data))
}
