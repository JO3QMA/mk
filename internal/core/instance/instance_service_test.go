package instance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/core/instance"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService(t *testing.T) (*instance.Service, *testutil.MockInstanceRepository, *testutil.MockMetaRepository) {
	t.Helper()
	repo := testutil.NewMockInstanceRepository()
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{}
	idGen, _ := id.NewGenerator("aidx")
	return instance.NewService(repo, metaRepo, idGen), repo, metaRepo
}

func TestService_RegisterFromHost_New(t *testing.T) {
	svc, repo, _ := newService(t)
	got, err := svc.RegisterFromHost("alpha.example")
	require.NoError(t, err)
	assert.Equal(t, "alpha.example", got.Host)
	assert.Equal(t, 1, got.UsersCount)
	assert.NotEmpty(t, repo.Instances["alpha.example"])
}

func TestService_RegisterFromHost_Existing(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{
		ID: "i1", Host: "alpha.example", UsersCount: 5,
	}
	got, err := svc.RegisterFromHost("alpha.example")
	require.NoError(t, err)
	assert.Equal(t, 5, got.UsersCount)
}

func TestService_RegisterFromHost_EmptyHost(t *testing.T) {
	svc, _, _ := newService(t)
	_, err := svc.RegisterFromHost("")
	assert.Error(t, err)
}

func TestService_RegisterFromHost_CreateError(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.CreateErr = errors.New("create failed")
	_, err := svc.RegisterFromHost("beta.example")
	assert.Error(t, err)
}

func TestService_MarkRequestReceived(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{ID: "i1", Host: "alpha.example"}
	require.NoError(t, svc.MarkRequestReceived("alpha.example"))
	require.NotNil(t, repo.Instances["alpha.example"].LatestRequestReceivedAt)
}

func TestService_MarkRequestReceived_UnknownHost(t *testing.T) {
	svc, _, _ := newService(t)
	require.NoError(t, svc.MarkRequestReceived("missing.example"))
}

func TestService_MarkRequestReceived_EmptyHost(t *testing.T) {
	svc, _, _ := newService(t)
	require.NoError(t, svc.MarkRequestReceived(""))
}

func TestService_RecordResponseError(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{ID: "i1", Host: "alpha.example"}
	require.NoError(t, svc.RecordResponseError("alpha.example"))
	assert.True(t, repo.Instances["alpha.example"].IsNotResponding)
	require.NotNil(t, repo.Instances["alpha.example"].NotRespondingSince)
}

func TestService_RecordResponseError_AlreadyNotResponding(t *testing.T) {
	svc, repo, _ := newService(t)
	since := time.Now().Add(-1 * time.Hour)
	repo.Instances["alpha.example"] = &model.Instance{
		ID: "i1", Host: "alpha.example",
		IsNotResponding: true, NotRespondingSince: &since,
	}
	require.NoError(t, svc.RecordResponseError("alpha.example"))
	assert.Equal(t, &since, repo.Instances["alpha.example"].NotRespondingSince)
}

func TestService_RecordResponseError_UnknownHost(t *testing.T) {
	svc, _, _ := newService(t)
	require.NoError(t, svc.RecordResponseError("missing.example"))
}

func TestService_RecordResponseError_EmptyHost(t *testing.T) {
	svc, _, _ := newService(t)
	require.NoError(t, svc.RecordResponseError(""))
}

func TestService_RecordResponseSuccess(t *testing.T) {
	svc, repo, _ := newService(t)
	since := time.Now().Add(-1 * time.Hour)
	repo.Instances["alpha.example"] = &model.Instance{
		ID: "i1", Host: "alpha.example",
		IsNotResponding: true, NotRespondingSince: &since,
	}
	require.NoError(t, svc.RecordResponseSuccess("alpha.example"))
	assert.False(t, repo.Instances["alpha.example"].IsNotResponding)
}

func TestService_RecordResponseSuccess_UnknownHost(t *testing.T) {
	svc, _, _ := newService(t)
	require.NoError(t, svc.RecordResponseSuccess("missing.example"))
}

func TestService_RecordResponseSuccess_EmptyHost(t *testing.T) {
	svc, _, _ := newService(t)
	require.NoError(t, svc.RecordResponseSuccess(""))
}

func TestService_IsBlocked(t *testing.T) {
	svc, _, metaRepo := newService(t)
	metaRepo.Meta.BlockedHosts = pq.StringArray{"bad.example"}
	assert.True(t, svc.IsBlocked("bad.example"))
	assert.False(t, svc.IsBlocked("good.example"))
	assert.False(t, svc.IsBlocked(""))
}

func TestService_IsBlocked_MetaError(t *testing.T) {
	svc, _, metaRepo := newService(t)
	metaRepo.Meta = nil
	assert.False(t, svc.IsBlocked("any.example"))
}

func TestService_IsSilenced(t *testing.T) {
	svc, _, metaRepo := newService(t)
	metaRepo.Meta.SilencedHosts = pq.StringArray{"quiet.example"}
	assert.True(t, svc.IsSilenced("quiet.example"))
	assert.False(t, svc.IsSilenced("loud.example"))
	assert.False(t, svc.IsSilenced(""))
}

func TestService_IsSilenced_MetaError(t *testing.T) {
	svc, _, metaRepo := newService(t)
	metaRepo.Meta = nil
	assert.False(t, svc.IsSilenced("any.example"))
}

func TestService_Suspend(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{ID: "i1", Host: "alpha.example"}
	require.NoError(t, svc.Suspend("alpha.example", model.SuspensionStateManuallySuspended))
	assert.Equal(t, model.SuspensionStateManuallySuspended, repo.Instances["alpha.example"].SuspensionState)
}

func TestService_Suspend_NotFound(t *testing.T) {
	svc, _, _ := newService(t)
	err := svc.Suspend("missing.example", model.SuspensionStateManuallySuspended)
	assert.ErrorIs(t, err, instance.ErrInstanceNotFound)
}

func TestService_Suspend_EmptyHost(t *testing.T) {
	svc, _, _ := newService(t)
	err := svc.Suspend("", model.SuspensionStateManuallySuspended)
	assert.Error(t, err)
}

func TestService_UpdateModerationNote(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{ID: "i1", Host: "alpha.example"}
	require.NoError(t, svc.UpdateModerationNote("alpha.example", "spam"))
	assert.Equal(t, "spam", repo.Instances["alpha.example"].ModerationNote)
}

func TestService_UpdateModerationNote_NotFound(t *testing.T) {
	svc, _, _ := newService(t)
	err := svc.UpdateModerationNote("missing.example", "x")
	assert.ErrorIs(t, err, instance.ErrInstanceNotFound)
}

func TestService_UpdateModerationNote_EmptyHost(t *testing.T) {
	svc, _, _ := newService(t)
	err := svc.UpdateModerationNote("", "x")
	assert.Error(t, err)
}

func TestService_FindByHost(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{ID: "i1", Host: "alpha.example"}
	got, err := svc.FindByHost("alpha.example")
	require.NoError(t, err)
	assert.Equal(t, "i1", got.ID)
}

func TestService_FindByHost_NotFound(t *testing.T) {
	svc, _, _ := newService(t)
	_, err := svc.FindByHost("missing.example")
	assert.ErrorIs(t, err, instance.ErrInstanceNotFound)
}

func TestService_List(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{ID: "i1", Host: "alpha.example"}
	repo.Instances["beta.example"] = &model.Instance{ID: "i2", Host: "beta.example"}
	rows, err := svc.List(model.InstanceListFilter{})
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

// stubMetadataFetcher records the hosts it was asked to fetch.
type stubMetadataFetcher struct {
	hosts []string
	err   error
}

func (s *stubMetadataFetcher) Fetch(host string) error {
	s.hosts = append(s.hosts, host)
	return s.err
}

func TestService_RegisterFromHost_TriggersMetadataFetch(t *testing.T) {
	svc, _, _ := newService(t)
	fetcher := &stubMetadataFetcher{}
	svc.SetMetadataFetcher(fetcher)
	_, err := svc.RegisterFromHost("alpha.example")
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha.example"}, fetcher.hosts)
}

func TestService_RegisterFromHost_MetadataFetcherErrorIgnored(t *testing.T) {
	svc, _, _ := newService(t)
	fetcher := &stubMetadataFetcher{err: errors.New("net down")}
	svc.SetMetadataFetcher(fetcher)
	_, err := svc.RegisterFromHost("alpha.example")
	// fetcher エラーは握りつぶされる
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha.example"}, fetcher.hosts)
}

func TestService_RegisterFromHost_NoFetchOnExisting(t *testing.T) {
	svc, repo, _ := newService(t)
	repo.Instances["alpha.example"] = &model.Instance{ID: "i1", Host: "alpha.example"}
	fetcher := &stubMetadataFetcher{}
	svc.SetMetadataFetcher(fetcher)
	_, err := svc.RegisterFromHost("alpha.example")
	require.NoError(t, err)
	// 既存行に対しては fetch しない
	assert.Empty(t, fetcher.hosts)
}

// FetchMetadataService が MetadataFetcher interface を実装していることを確認。
// 配線時に router.go でこの代入が成立する必要がある。
func TestFetchMetadataService_ImplementsMetadataFetcher(t *testing.T) {
	var _ instance.MetadataFetcher = (*instance.FetchMetadataService)(nil)
}

func TestService_SetClock(t *testing.T) {
	svc, repo, _ := newService(t)
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return fixed })
	svc.SetClock(nil) // nil 渡しは無視
	got, err := svc.RegisterFromHost("alpha.example")
	require.NoError(t, err)
	assert.Equal(t, fixed, got.FirstRetrievedAt)
	assert.Equal(t, fixed, repo.Instances["alpha.example"].FirstRetrievedAt)
}
