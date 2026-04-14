package admin_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
)

var adminUser = &model.User{ID: "admin1"}

type stubIPRepo struct{}

func (s *stubIPRepo) Upsert(_, _ string) error                            { return nil }
func (s *stubIPRepo) ListByUser(_ string, _ int) ([]*model.UserIP, error) { return nil, nil }

// stubEmojiImportEnqueuer records the last payload and optionally returns err.
type stubEmojiImportEnqueuer struct {
	lastUserID string
	lastFileID string
	err        error
}

func (s *stubEmojiImportEnqueuer) EnqueueImportCustomEmojis(p queue.ImportCustomEmojisPayload) error {
	if s.err != nil {
		return s.err
	}
	s.lastUserID = p.UserID
	s.lastFileID = p.FileID
	return nil
}

// assertError is a trivial error used to exercise the enqueue-failure branch.
type assertError struct{}

func (assertError) Error() string { return "stub enqueue failure" }

func TestSetDriveFileRepo(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetDriveFileRepo(testutil.NewMockDriveFileRepository())
}

func TestSetAdminDB(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAdminDB(nil)
}

// --- accounts ---
func TestAccountsDelete(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AccountsDelete, `{}`, adminUser).Code)
}
func TestAccountsFindByEmail_Empty(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, doPost(h.AccountsFindByEmail, `{}`, adminUser).Code)
}
func TestAccountsFindByEmail_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNotFound, doPost(h.AccountsFindByEmail, `{"email":"ghost@example.com"}`, adminUser).Code)
}

// --- ad ---
func TestAdCreate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AdCreate, `{}`, adminUser).Code)
}
func TestAdDelete(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AdDelete, `{}`, adminUser).Code)
}
func TestAdList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.AdList, `{}`, adminUser).Code)
}
func TestAdUpdate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AdUpdate, `{}`, adminUser).Code)
}

// --- avatar-decorations ---
func TestAvatarDecorationsCreate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AvatarDecorationsCreate, `{}`, adminUser).Code)
}
func TestAvatarDecorationsDelete(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AvatarDecorationsDelete, `{}`, adminUser).Code)
}
func TestAvatarDecorationsList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.AvatarDecorationsList, `{}`, adminUser).Code)
}
func TestAvatarDecorationsUpdate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AvatarDecorationsUpdate, `{}`, adminUser).Code)
}

// --- captcha ---
func TestCaptchaCurrent(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.CaptchaCurrent, `{}`, adminUser).Code)
}
func TestCaptchaSave(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.CaptchaSave, `{}`, adminUser).Code)
}

// --- abuse-report/notification-recipient ---
func TestAbuseReportNotificationRecipientCreate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AbuseReportNotificationRecipientCreate, `{}`, adminUser).Code)
}
func TestAbuseReportNotificationRecipientDelete(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AbuseReportNotificationRecipientDelete, `{}`, adminUser).Code)
}
func TestAbuseReportNotificationRecipientList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.AbuseReportNotificationRecipientList, `{}`, adminUser).Code)
}
func TestAbuseReportNotificationRecipientShow(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNotFound, doPost(h.AbuseReportNotificationRecipientShow, `{}`, adminUser).Code)
}
func TestAbuseReportNotificationRecipientUpdate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.AbuseReportNotificationRecipientUpdate, `{}`, adminUser).Code)
}

// --- single endpoints ---
func TestDeleteAccountAdmin(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.DeleteAccount, `{}`, adminUser).Code)
}
func TestDeleteAllFilesOfUser(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.DeleteAllFilesOfUser, `{}`, adminUser).Code)
}
func TestForwardAbuseUserReport(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.ForwardAbuseUserReport, `{}`, adminUser).Code)
}
func TestGetIndexStats(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.GetIndexStats, `{}`, adminUser).Code)
}
func TestGetTableStats(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.GetTableStats, `{}`, adminUser).Code)
}
func TestGetUserIPs_NilRepo(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.GetUserIPs, `{}`, adminUser).Code)
}

func TestGetUserIPs_InvalidParam(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetUserIPRepo(&stubIPRepo{})
	assert.Equal(t, http.StatusBadRequest, doPost(h.GetUserIPs, `{}`, adminUser).Code)
}

func TestGetUserIPs_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetUserIPRepo(&stubIPRepo{})
	rec := doPost(h.GetUserIPs, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
}
func TestResetPasswordAdmin_Empty(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, doPost(h.ResetPassword, `{}`, adminUser).Code)
}
func TestResetPasswordAdmin_Success(t *testing.T) {
	h, userRepo, _, _ := newTestHandler(t)
	userRepo.Users["u1"] = &model.User{ID: "u1", Username: "testuser"}
	userRepo.Profiles["u1"] = &model.UserProfile{UserID: "u1"}
	rec := doPost(h.ResetPassword, `{"userId":"u1"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "password")
}
func TestSendEmail(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.SendEmail, `{}`, adminUser).Code)
}
func TestServerInfo_Disabled(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.ServerInfo, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	// enableServerMachineStats = false (デフォルト) なので Empty() が返る
	assert.Contains(t, rec.Body.String(), `"name":"?"`)
}
func TestUnsetUserAvatar(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.UnsetUserAvatar, `{}`, adminUser).Code)
}
func TestUnsetUserBanner(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.UnsetUserBanner, `{}`, adminUser).Code)
}
func TestUpdateAbuseUserReport(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.UpdateAbuseUserReport, `{}`, adminUser).Code)
}
func TestUpdateProxyAccount(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.UpdateProxyAccount, `{}`, adminUser).Code)
}
func TestUpdateUserNote(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.UpdateUserNote, `{}`, adminUser).Code)
}

// --- drive ---
func TestDriveCleanRemoteFiles(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.DriveCleanRemoteFiles, `{}`, adminUser).Code)
}
func TestDriveCleanup(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.DriveCleanup, `{}`, adminUser).Code)
}
func TestDriveFilesAdmin(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.DriveFiles, `{}`, adminUser).Code)
}
func TestDriveShowFile_Empty(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, doPost(h.DriveShowFile, `{}`, adminUser).Code)
}
func TestDriveShowFile_NotFound(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNotFound, doPost(h.DriveShowFile, `{"fileId":"ghost"}`, adminUser).Code)
}

// --- emoji bulk ops ---
func TestEmojiAddAliasesBulk(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiAddAliasesBulk, `{}`, adminUser).Code)
}
func TestEmojiCopy(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiCopy, `{}`, adminUser).Code)
}
func TestEmojiDeleteBulk(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiDeleteBulk, `{}`, adminUser).Code)
}
func TestEmojiImportZip_NoFileID(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	// fileId missing → 400 InvalidParam
	assert.Equal(t, http.StatusBadRequest, doPost(h.EmojiImportZip, `{}`, adminUser).Code)
}

func TestEmojiImportZip_MalformedJSON(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusBadRequest, doPost(h.EmojiImportZip, `not-json`, adminUser).Code)
}

func TestEmojiImportZip_NoEnqueuer(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	// enqueuer not set → 204 (no-op fallback so tests without worker still pass)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiImportZip, `{"fileId":"f1"}`, adminUser).Code)
}

func TestEmojiImportZip_NilUser(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetEmojiImportEnqueuer(&stubEmojiImportEnqueuer{})
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiImportZip, `{"fileId":"f1"}`, nil).Code)
}

func TestEmojiImportZip_Success(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	stub := &stubEmojiImportEnqueuer{}
	h.SetEmojiImportEnqueuer(stub)
	rec := doPost(h.EmojiImportZip, `{"fileId":"f1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "f1", stub.lastFileID)
	assert.Equal(t, "admin1", stub.lastUserID)
}

func TestEmojiImportZip_EnqueueFailure(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetEmojiImportEnqueuer(&stubEmojiImportEnqueuer{err: assertError{}})
	rec := doPost(h.EmojiImportZip, `{"fileId":"f1"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
func TestEmojiListRemote(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.EmojiListRemote, `{}`, adminUser).Code)
}
func TestEmojiRemoveAliasesBulk(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiRemoveAliasesBulk, `{}`, adminUser).Code)
}
func TestEmojiSetAliasesBulk(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiSetAliasesBulk, `{}`, adminUser).Code)
}
func TestEmojiSetCategoryBulk(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiSetCategoryBulk, `{}`, adminUser).Code)
}
func TestEmojiSetLicenseBulk(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.EmojiSetLicenseBulk, `{}`, adminUser).Code)
}

// --- federation ---
func TestFederationDeleteAllFiles(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationDeleteAllFiles, `{}`, adminUser).Code)
}
func TestFederationRefreshRemoteInstanceMetadata(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationRefreshRemoteInstanceMetadata, `{}`, adminUser).Code)
}
func TestFederationRemoveAllFollowing(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationRemoveAllFollowing, `{}`, adminUser).Code)
}
func TestFederationUpdateInstance(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.FederationUpdateInstance, `{}`, adminUser).Code)
}

// --- invite ---
func TestInviteCreate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.InviteCreate, `{}`, adminUser).Code)
}
func TestInviteList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.InviteList, `{}`, adminUser).Code)
}

// --- promo ---
func TestPromoCreate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.PromoCreate, `{}`, adminUser).Code)
}

// --- queue ---
func TestQueueClear(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.QueueClear, `{}`, adminUser).Code)
}
func TestQueueDeliverDelayed(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.QueueDeliverDelayed, `{}`, adminUser).Code)
}
func TestQueueInboxDelayed(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.QueueInboxDelayed, `{}`, adminUser).Code)
}
func TestQueueJobs(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.QueueJobs, `{}`, adminUser).Code)
}
func TestQueuePromoteJobs(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.QueuePromoteJobs, `{}`, adminUser).Code)
}
func TestQueueQueueStats(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.QueueQueueStats, `{}`, adminUser).Code)
}
func TestQueueQueues(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.QueueQueues, `{}`, adminUser).Code)
}
func TestQueueRemoveJob(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.QueueRemoveJob, `{}`, adminUser).Code)
}
func TestQueueRetryJob(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.QueueRetryJob, `{}`, adminUser).Code)
}
func TestQueueShowJob(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNotFound, doPost(h.QueueShowJob, `{}`, adminUser).Code)
}
func TestQueueShowJobLogs(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.QueueShowJobLogs, `{}`, adminUser).Code)
}
func TestQueueStatsAdmin(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := doPost(h.QueueStats, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "deliver")
}

// --- relays ---

// fakeRelaySvc records calls + returns configurable results for the
// admin/relays handlers.
type fakeRelaySvc struct {
	added   []string
	removed []string
	listed  int
	retRel  *model.Relay
	retList []*model.Relay
	err     error
}

func (f *fakeRelaySvc) Add(_ context.Context, inbox string) (*model.Relay, error) {
	f.added = append(f.added, inbox)
	return f.retRel, f.err
}
func (f *fakeRelaySvc) Remove(_ context.Context, id string) error {
	f.removed = append(f.removed, id)
	return f.err
}
func (f *fakeRelaySvc) List(_ context.Context) ([]*model.Relay, error) {
	f.listed++
	return f.retList, f.err
}

func TestRelaysAdd(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.RelaysAdd, `{}`, adminUser).Code)
}

func TestRelaysAdd_WithService(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{retRel: &model.Relay{ID: "rel1", Inbox: "https://r.example/inbox", Status: "requesting"}}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysAdd, `{"inbox":"https://r.example/inbox"}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"https://r.example/inbox"}, svc.added)
}

func TestRelaysAdd_ServiceError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{err: errors.New("boom")}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysAdd, `{"inbox":"https://r.example/inbox"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRelaysList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.RelaysList, `{}`, adminUser).Code)
}

func TestRelaysList_WithService(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{retList: []*model.Relay{{ID: "a"}, {ID: "b"}}}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysList, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, svc.listed)
}

func TestRelaysList_WithService_NilSlice(t *testing.T) {
	// list が nil を返しても [] を返す (本家互換)
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{retList: nil}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysList, `{}`, adminUser)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestRelaysRemove(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.RelaysRemove, `{}`, adminUser).Code)
}

func TestRelaysRemove_WithService(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysRemove, `{"id":"rel1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, []string{"rel1"}, svc.removed)
}

func TestRelaysRemove_WithService_NoID(t *testing.T) {
	// id も inbox も無い場合はスキップして 204 を返す
	h, _, _, _ := newTestHandler(t)
	svc := &fakeRelaySvc{}
	h.SetRelayService(svc)
	rec := doPost(h.RelaysRemove, `{}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, svc.removed)
}

// --- system-webhook ---
func TestSystemWebhookCreate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.SystemWebhookCreate, `{}`, adminUser).Code)
}
func TestSystemWebhookDelete(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.SystemWebhookDelete, `{}`, adminUser).Code)
}
func TestSystemWebhookList(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusOK, doPost(h.SystemWebhookList, `{}`, adminUser).Code)
}
func TestSystemWebhookShow(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNotFound, doPost(h.SystemWebhookShow, `{}`, adminUser).Code)
}
func TestSystemWebhookTest(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.SystemWebhookTest, `{}`, adminUser).Code)
}
func TestSystemWebhookUpdate(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	assert.Equal(t, http.StatusNoContent, doPost(h.SystemWebhookUpdate, `{}`, adminUser).Code)
}
