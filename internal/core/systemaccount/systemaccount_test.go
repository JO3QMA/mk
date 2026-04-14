package systemaccount_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/systemaccount"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// saMock wraps testutil.MockSystemAccountRepository to translate the
// testutil-level ErrNotFound sentinel into the gorm.ErrRecordNotFound that
// the production gorm repository would return. Service 側が gorm の
// sentinel で分岐するため必須のアダプタ。
type saMock struct {
	*testutil.MockSystemAccountRepository
}

func newSAMock() *saMock {
	return &saMock{MockSystemAccountRepository: testutil.NewMockSystemAccountRepository()}
}

func (s *saMock) FindByType(typ string) (*model.SystemAccount, error) {
	sa, err := s.MockSystemAccountRepository.FindByType(typ)
	if errors.Is(err, testutil.ErrNotFound) {
		return nil, gorm.ErrRecordNotFound
	}
	return sa, err
}

// saFailCreate は Create だけ常にエラーを返す SystemAccountRepository モック。
type saFailCreate struct {
	*saMock
	err error
}

func (f *saFailCreate) Create(sa *model.SystemAccount) error { return f.err }

// keypairFailCreate は Create が常にエラーを返す UserKeypairRepository モック。
type keypairFailCreate struct {
	*testutil.MockUserKeypairRepository
	err error
}

func (k *keypairFailCreate) Create(kp *model.UserKeypair) error { return k.err }

// userFailProfile は CreateProfile だけエラーを返す。
type userFailProfile struct {
	*testutil.MockUserRepository
	err error
}

func (u *userFailProfile) CreateProfile(p *model.UserProfile) error { return u.err }

func newService(t *testing.T) (*systemaccount.Service, *testutil.MockUserRepository, *testutil.MockUserKeypairRepository, *saMock) {
	t.Helper()
	userRepo := testutil.NewMockUserRepository()
	keypairRepo := testutil.NewMockUserKeypairRepository()
	saRepo := newSAMock()
	idGen, err := id.NewGenerator("aidx")
	require.NoError(t, err)
	svc := systemaccount.NewService(userRepo, keypairRepo, saRepo, idGen)
	svc.SetClock(func() time.Time { return time.Unix(1_700_000_000, 0).UTC() })
	return svc, userRepo, keypairRepo, saRepo
}

func TestFetch_CreatesRowsOnFirstCall(t *testing.T) {
	svc, userRepo, keypairRepo, saRepo := newService(t)

	user, err := svc.Fetch("relay")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "relay.actor", user.Username)
	assert.True(t, user.IsBot)
	assert.False(t, user.IsExplorable)
	assert.Nil(t, user.Host) // 必ずローカル

	// 各 repo に行が作られていることを確認
	require.Len(t, userRepo.Users, 1)
	require.Len(t, keypairRepo.Keypairs, 1)
	require.Len(t, saRepo.Accounts, 1)
	kp := keypairRepo.Keypairs[user.ID]
	require.NotNil(t, kp)
	assert.Contains(t, kp.PrivateKey, "PRIVATE KEY")
	assert.Contains(t, kp.PublicKey, "PUBLIC KEY")
}

func TestFetch_ReturnsExistingOnSecondCall(t *testing.T) {
	svc, userRepo, _, _ := newService(t)

	user1, err := svc.Fetch("relay")
	require.NoError(t, err)
	user2, err := svc.Fetch("relay")
	require.NoError(t, err)
	assert.Equal(t, user1.ID, user2.ID)
	// user はひとつだけ
	assert.Len(t, userRepo.Users, 1)
}

func TestFetch_EmptyKindRejected(t *testing.T) {
	svc, _, _, _ := newService(t)
	_, err := svc.Fetch("")
	assert.Error(t, err)
}

func TestFetch_ErrorBubblesFromSystemAccountCreate(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	keypairRepo := testutil.NewMockUserKeypairRepository()
	saRepo := &saFailCreate{
		saMock: newSAMock(),
		err:    errors.New("db down"),
	}
	idGen, _ := id.NewGenerator("aidx")
	svc := systemaccount.NewService(userRepo, keypairRepo, saRepo, idGen)

	_, err := svc.Fetch("relay")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestFetch_ErrorBubblesFromKeypairCreate(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	keypairRepo := &keypairFailCreate{
		MockUserKeypairRepository: testutil.NewMockUserKeypairRepository(),
		err:                       errors.New("keypair db down"),
	}
	saRepo := newSAMock()
	idGen, _ := id.NewGenerator("aidx")
	svc := systemaccount.NewService(userRepo, keypairRepo, saRepo, idGen)

	_, err := svc.Fetch("relay")
	assert.Error(t, err)
}

func TestFetch_ErrorBubblesFromProfileCreate(t *testing.T) {
	userRepo := &userFailProfile{
		MockUserRepository: testutil.NewMockUserRepository(),
		err:                errors.New("profile db down"),
	}
	keypairRepo := testutil.NewMockUserKeypairRepository()
	saRepo := newSAMock()
	idGen, _ := id.NewGenerator("aidx")
	svc := systemaccount.NewService(userRepo, keypairRepo, saRepo, idGen)

	_, err := svc.Fetch("relay")
	assert.Error(t, err)
}

// saFailLookup は FindByType で gorm.ErrRecordNotFound 以外のエラーを返して
// その分岐がきちんと上位に伝わることを確認する。
type saFailLookup struct {
	*saMock
	err error
}

func (f *saFailLookup) FindByType(typ string) (*model.SystemAccount, error) { return nil, f.err }

func TestFetch_ErrorBubblesFromUnexpectedLookup(t *testing.T) {
	userRepo := testutil.NewMockUserRepository()
	keypairRepo := testutil.NewMockUserKeypairRepository()
	saRepo := &saFailLookup{
		saMock: newSAMock(),
		err:    errors.New("other db error"),
	}
	idGen, _ := id.NewGenerator("aidx")
	svc := systemaccount.NewService(userRepo, keypairRepo, saRepo, idGen)

	_, err := svc.Fetch("relay")
	require.Error(t, err)
	assert.NotErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSetClock_NilIgnored(t *testing.T) {
	svc, _, _, _ := newService(t)
	// nil を渡しても panic しない
	svc.SetClock(nil)
	_, err := svc.Fetch("relay")
	require.NoError(t, err)
}
