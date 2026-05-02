package signup_test

import (
	"sync"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/signup"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// integrationDB は本ファイル専用 (testcontainers / 既存 PostgreSQL に接続)。
// 他の core/signup mock test と並走しても DSN は共有なので問題なし。
func integrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testutil.OpenTestDB()
	if err != nil {
		t.Skipf("PostgreSQL test DB unavailable: %v", err)
	}
	testutil.ApplyMigrations(db)
	return db
}

// cleanupSignupRows は本ファイルで作った行を毎テスト後にきれいに掃く。
// 互いの test が前ステップの pending / user / ticket を引き継がないように
// suffix prefix で絞り込んでいる (it_ で始まる行を全削除)。
func cleanupSignupRows(t *testing.T, db *gorm.DB, prefix string) {
	t.Helper()
	db.Exec(`DELETE FROM "user_pending" WHERE id LIKE ? OR username LIKE ?`, prefix+"%", prefix+"%")
	db.Exec(`DELETE FROM "user_profile" WHERE "userId" IN (SELECT id FROM "user" WHERE id LIKE ? OR "usernameLower" LIKE ?)`, prefix+"%", prefix+"%")
	db.Exec(`DELETE FROM "user_keypair" WHERE "userId" LIKE ?`, prefix+"%")
	db.Exec(`DELETE FROM "user" WHERE id LIKE ? OR "usernameLower" LIKE ?`, prefix+"%", prefix+"%")
	db.Exec(`DELETE FROM "registration_ticket" WHERE id LIKE ? OR code LIKE ?`, prefix+"%", prefix+"%")
}

func newTxService(t *testing.T, db *gorm.DB) *signup.Service {
	t.Helper()
	idGen, _ := id.NewGenerator("aidx")

	// real repos (mock ではなく本物の GORM 経由で挙動確認)
	userRepo := repository.NewUserRepository(db)
	pendingRepo := repository.NewUserPendingRepository(db)
	ticketRepo := repository.NewRegistrationTicketRepository(db)

	// Meta は seed が無いと FindByUsernameLower で参照される PreservedUsernames
	// が空 default になり問題ないので、空 meta repo を mock で渡す。
	metaRepo := testutil.NewMockMetaRepository()
	metaRepo.Meta = &model.Meta{ID: "x"}

	svc := signup.NewService(userRepo, metaRepo, idGen)
	svc.SetUserPendingRepo(pendingRepo)
	svc.SetTicketRepo(ticketRepo)
	svc.SetDB(db)
	return svc
}

// invitation 経由で pending を作成し、PromotePending が ticket を消費する
// 一連の流れを real DB 上で検証する (#600 item 2 + #604 happy path)。
func TestPromotePending_TxConsumesInvitationTicket(t *testing.T) {
	db := integrationDB(t)
	const prefix = "itinv_"
	defer cleanupSignupRows(t, db, prefix)

	svc := newTxService(t, db)

	// 招待 ticket seed
	ticket := &model.RegistrationTicket{
		ID:   prefix + "tkt1",
		Code: prefix + "code1",
	}
	require.NoError(t, db.Create(ticket).Error)

	row, err := svc.CreatePending(prefix+"alice", "alice@it.example", "pw123", &ticket.ID)
	require.NoError(t, err)

	result, err := svc.PromotePending(row.Code)
	require.NoError(t, err)
	require.NotNil(t, result.User)
	assert.True(t, result.InvitationTicketConsumed, "tx 経路では Service 側で MarkUsed 済")

	// ticket が消費済 (usedById = 新 user, usedAt が set)
	var consumed model.RegistrationTicket
	require.NoError(t, db.Where("id = ?", ticket.ID).First(&consumed).Error)
	require.NotNil(t, consumed.UsedByID)
	assert.Equal(t, result.User.ID, *consumed.UsedByID)
	require.NotNil(t, consumed.UsedAt)

	// pending row が消えている
	var lingering model.UserPending
	err = db.Where("id = ?", row.ID).First(&lingering).Error
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// 同じ pending code で並列 2 PromotePending を試行すると、片方は user 作成
// 成功 / もう片方は ErrInvitationAlreadyUsed (もしくは ErrUsernameAlreadyExists)
// で拒否される (#604 race fix)。
func TestPromotePending_ConcurrentPromotesYieldOneUser(t *testing.T) {
	db := integrationDB(t)
	const prefix = "itrace_"
	defer cleanupSignupRows(t, db, prefix)

	// 同一 pending を 2 つ作って同じ ticket を共有させる concurrent シナリオ
	svcA := newTxService(t, db)
	svcB := newTxService(t, db)

	ticket := &model.RegistrationTicket{
		ID:   prefix + "tkt",
		Code: prefix + "code",
	}
	require.NoError(t, db.Create(ticket).Error)

	rowA, err := svcA.CreatePending(prefix+"userA", "a@it.example", "pw", &ticket.ID)
	require.NoError(t, err)
	rowB, err := svcB.CreatePending(prefix+"userB", "b@it.example", "pw", &ticket.ID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	var errA, errB error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errA = svcA.PromotePending(rowA.Code)
	}()
	go func() {
		defer wg.Done()
		_, errB = svcB.PromotePending(rowB.Code)
	}()
	wg.Wait()

	// 1 件だけ成功、もう片方は ErrInvitationAlreadyUsed
	successes := 0
	if errA == nil {
		successes++
	}
	if errB == nil {
		successes++
	}
	assert.Equal(t, 1, successes, "ticket lock により 1 件だけ成功する")

	failed := errA
	if errB != nil {
		failed = errB
	}
	assert.ErrorIs(t, failed, signup.ErrInvitationAlreadyUsed)

	// user は 1 件だけ作成されている
	var users []model.User
	require.NoError(t, db.Where(`"usernameLower" LIKE ?`, prefix+"%").Find(&users).Error)
	assert.Len(t, users, 1)
}

// username 衝突で tx が rollback すると user / profile が一切残らないこと。
// (#600 item 2 partial failure rollback)。
func TestPromotePending_UsernameConflictRollsBack(t *testing.T) {
	db := integrationDB(t)
	const prefix = "itroll_"
	defer cleanupSignupRows(t, db, prefix)

	svc := newTxService(t, db)
	idGen, _ := id.NewGenerator("aidx")

	// 既存 user を seed (同名で衝突)
	existing := &model.User{
		ID:                idGen.Generate(time.Now()),
		Username:          prefix + "dup",
		UsernameLower:     prefix + "dup",
		AvatarDecorations: datatypes.JSON([]byte("[]")),
	}
	require.NoError(t, db.Create(existing).Error)

	// pending 作成 (CreatePending の事前チェックを回避するため直接 INSERT)
	pending := &model.UserPending{
		ID:       idGen.Generate(time.Now()),
		Code:     prefix + "code",
		Username: prefix + "dup",
		Email:    "dup@it.example",
		Password: "$2a$10$fakehash",
	}
	require.NoError(t, db.Create(pending).Error)

	_, err := svc.PromotePending(pending.Code)
	require.ErrorIs(t, err, signup.ErrUsernameAlreadyExists)

	// 失敗後、新 user 行は作成されていない (existing 1 件のみ)
	var users []model.User
	require.NoError(t, db.Where(`"usernameLower" = ?`, prefix+"dup").Find(&users).Error)
	assert.Len(t, users, 1, "rollback で新 user は作成されていない")
	assert.Equal(t, existing.ID, users[0].ID)

	// pending row も消えていない (rollback)
	var lingering model.UserPending
	require.NoError(t, db.Where("id = ?", pending.ID).First(&lingering).Error)
}

// 非招待 pending を tx 経路で promote する場合、ticket 経路はスキップされ
// InvitationTicketConsumed = false で返る。
func TestPromotePending_TxNoInvitation(t *testing.T) {
	db := integrationDB(t)
	const prefix = "itnoinv_"
	defer cleanupSignupRows(t, db, prefix)

	svc := newTxService(t, db)

	row, err := svc.CreatePending(prefix+"plain", "p@it.example", "pw", nil)
	require.NoError(t, err)

	result, err := svc.PromotePending(row.Code)
	require.NoError(t, err)
	assert.Nil(t, result.InvitationTicketID)
	assert.False(t, result.InvitationTicketConsumed, "非招待では consumed flag は false")
}

// tx 経路で keypair repo が wire されていれば federation 用 keypair も
// 同 tx で作成される (RSA は遅いので別テスト)。
func TestPromotePending_TxCreatesKeypair(t *testing.T) {
	db := integrationDB(t)
	const prefix = "itkp_"
	defer cleanupSignupRows(t, db, prefix)

	svc := newTxService(t, db)
	keypairRepo := repository.NewUserKeypairRepository(db)
	svc.SetKeypairRepo(keypairRepo)

	row, err := svc.CreatePending(prefix+"kp", "kp@it.example", "pw", nil)
	require.NoError(t, err)
	result, err := svc.PromotePending(row.Code)
	require.NoError(t, err)

	kp, err := keypairRepo.FindByUserID(result.User.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, kp.PublicKey)
	assert.NotEmpty(t, kp.PrivateKey)
}

// tx 経路で webhook hook が wire されていれば commit 後に発火する。
func TestPromotePending_TxFiresWebhook(t *testing.T) {
	db := integrationDB(t)
	const prefix = "itwh_"
	defer cleanupSignupRows(t, db, prefix)

	svc := newTxService(t, db)
	hook := &integrationHook{}
	svc.SetWebhookHook(hook)

	row, err := svc.CreatePending(prefix+"wh", "wh@it.example", "pw", nil)
	require.NoError(t, err)
	_, err = svc.PromotePending(row.Code)
	require.NoError(t, err)
	assert.Equal(t, 1, hook.calls)
}

type integrationHook struct{ calls int }

func (h *integrationHook) OnUserCreated(_ *model.User) { h.calls++ }

// pending row 作成後に ticket を delete すると、PromotePending tx 内の
// FindByIDForUpdateTx が NotFound になり ErrInvitationAlreadyUsed として
// 返る (defensive な race / admin-side ticket revoke 対応)。
func TestPromotePending_TxTicketRevoked(t *testing.T) {
	db := integrationDB(t)
	const prefix = "itrev_"
	defer cleanupSignupRows(t, db, prefix)

	svc := newTxService(t, db)

	ticket := &model.RegistrationTicket{
		ID:   prefix + "tkt",
		Code: prefix + "code",
	}
	require.NoError(t, db.Create(ticket).Error)

	row, err := svc.CreatePending(prefix+"u", "u@it.example", "pw", &ticket.ID)
	require.NoError(t, err)

	// admin が ticket を revoke する状況を再現
	require.NoError(t, db.Where("id = ?", ticket.ID).Delete(&model.RegistrationTicket{}).Error)

	_, err = svc.PromotePending(row.Code)
	assert.ErrorIs(t, err, signup.ErrInvitationAlreadyUsed)
}
