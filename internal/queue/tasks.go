package queue

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

// TaskTypeDeliver is the asynq task type used for outbound ActivityPub
// delivery jobs.
const TaskTypeDeliver = "ap:deliver"

// TaskTypeExport is the asynq task type for data export jobs.
const TaskTypeExport = "export"

// TaskTypeImport is the asynq task type for data import jobs.
const TaskTypeImport = "import"

// TaskTypeWebPush is the asynq task type for Web Push delivery jobs.
const TaskTypeWebPush = "webpush:notify"

// TaskTypeImportCustomEmojis is the asynq task type for admin/emoji/import-zip
// processing jobs. 本家 Misskey の importCustomEmojis queue job と同名。
const TaskTypeImportCustomEmojis = "importCustomEmojis"

// TaskTypeUserWebhook is the asynq task type for user webhook delivery jobs.
const TaskTypeUserWebhook = "webhook:user"

// TaskTypeSystemWebhook is the asynq task type for system webhook delivery jobs.
const TaskTypeSystemWebhook = "webhook:system"

// TaskTypeCleanRemoteNotes is the asynq task type for the periodic remote
// notes cleaning job. ペイロードなし (meta から設定を読む)。
const TaskTypeCleanRemoteNotes = "maintenance:cleanRemoteNotes"

// TaskTypeReactionFlush is the asynq task type for flushing buffered
// reaction counts from Redis to the database.
const TaskTypeReactionFlush = "maintenance:reactionFlush"

// TaskTypeChartTick is the asynq task type for the hourly tick-charts
// job. Mirrors upstream `tickCharts` (cron `55 * * * *`).
const TaskTypeChartTick = "chart:tick"

// TaskTypeChartResync is the asynq task type for the daily resync-charts
// job. Mirrors upstream `resyncCharts` (cron `0 0 * * *`).
const TaskTypeChartResync = "chart:resync"

// TaskTypeChartClean is the asynq task type for the daily clean-charts
// job. Mirrors upstream `cleanCharts` (cron `0 0 * * *`).
const TaskTypeChartClean = "chart:clean"

// TaskTypeDeleteAccount is the asynq task type for the background cascade
// deletion of a user account's related rows (notes / drive files / follow
// graph entries etc.). Mirrors upstream `deleteAccount` queue job.
const TaskTypeDeleteAccount = "maintenance:deleteAccount"

// TaskTypeInstanceRefresh is the asynq task type for the periodic
// remote-instance metadata refresh (#393). Registered by
// `Scheduler.RegisterInstanceRefreshJob` at `0 3 * * *` UTC and handled by
// `processors.InstanceRefreshProcessor`.
const TaskTypeInstanceRefresh = "maintenance:instanceRefresh"

// TaskTypeRetentionAggregate is the asynq task type for the daily
// retention aggregation (#421). Registered by
// `Scheduler.RegisterRetentionJob` at `0 0 * * *` UTC and handled by
// `processors.RetentionAggregateProcessor`. Mirrors upstream
// AggregateRetentionProcessorService.
const TaskTypeRetentionAggregate = "maintenance:retentionAggregate"

// DeliverPayload is the body of a deliver task. すべてJSONで安全に
// シリアライズできる型のみを保持する。
type DeliverPayload struct {
	// Inbox is the absolute URL of the recipient inbox to POST to.
	Inbox string `json:"inbox"`
	// Body is the JSON-serialized AP activity to deliver.
	Body []byte `json:"body"`
	// KeyID is the HTTP Signature keyId
	// (e.g. https://example.com/users/u1#main-key).
	KeyID string `json:"keyId"`
	// KeyPEM is the PEM-encoded RSA private key for signing the request.
	KeyPEM string `json:"keyPem"`
}

// NewDeliverTask serializes the payload into an asynq.Task ready to enqueue.
// DeliverPayload はすべて marshal 可能な型 (string と []byte) のみで構成される
// ため json.Marshal は失敗しない。エラー戻り値を取らない方が呼び出し側を簡潔
// にできる。
func NewDeliverTask(payload DeliverPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeDeliver, body)
}

// DecodeDeliverPayload extracts a DeliverPayload from a task body.
func DecodeDeliverPayload(body []byte) (DeliverPayload, error) {
	var p DeliverPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return DeliverPayload{}, err
	}
	return p, nil
}

// ExportPayload is the body of an export task.
type ExportPayload struct {
	UserID string `json:"userId"`
	Type   string `json:"type"` // notes, following, blocking, mute, favorites, user-lists, antennas, clips
}

// NewExportTask creates an asynq.Task for data export.
func NewExportTask(payload ExportPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeExport, body)
}

// DecodeExportPayload extracts an ExportPayload from a task body.
func DecodeExportPayload(body []byte) (ExportPayload, error) {
	var p ExportPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return ExportPayload{}, err
	}
	return p, nil
}

// ImportPayload is the body of an import task.
type ImportPayload struct {
	UserID string `json:"userId"`
	Type   string `json:"type"` // following, blocking, muting, user-lists, antennas
	FileID string `json:"fileId"`
}

// NewImportTask creates an asynq.Task for data import.
func NewImportTask(payload ImportPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeImport, body)
}

// DecodeImportPayload extracts an ImportPayload from a task body.
func DecodeImportPayload(body []byte) (ImportPayload, error) {
	var p ImportPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return ImportPayload{}, err
	}
	return p, nil
}

// ImportCustomEmojisPayload is the body of an emoji-zip import task.
// UserID は import を発行した管理者 (= drive 上のアップロード所有者)。
type ImportCustomEmojisPayload struct {
	UserID string `json:"userId"`
	FileID string `json:"fileId"`
}

// NewImportCustomEmojisTask creates an asynq.Task for emoji-zip imports.
func NewImportCustomEmojisTask(payload ImportCustomEmojisPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeImportCustomEmojis, body)
}

// DecodeImportCustomEmojisPayload extracts an ImportCustomEmojisPayload.
func DecodeImportCustomEmojisPayload(body []byte) (ImportCustomEmojisPayload, error) {
	var p ImportCustomEmojisPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return ImportCustomEmojisPayload{}, err
	}
	return p, nil
}

// WebPushPayload is the body of a Web Push delivery task. The Body field holds
// the already-truncated notification payload; the processor does not inspect
// it and simply forwards it to subscribers.
type WebPushPayload struct {
	UserID string          `json:"userId"`
	Type   string          `json:"type"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// NewWebPushTask serializes the payload into an asynq.Task ready to enqueue.
func NewWebPushTask(payload WebPushPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeWebPush, body)
}

// DecodeWebPushPayload extracts a WebPushPayload from a task body.
func DecodeWebPushPayload(body []byte) (WebPushPayload, error) {
	var p WebPushPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return WebPushPayload{}, err
	}
	return p, nil
}

// WebhookPayload carries a single webhook delivery job. Body is the
// pre-marshalled event payload envelope (see core/webhook for the exact
// Misskey-compatible structure); the processor forwards it to the endpoint
// URL without further interpretation.
type WebhookPayload struct {
	WebhookID string          `json:"webhookId"`
	UserID    string          `json:"userId,omitempty"` // user webhooks only
	EventType string          `json:"eventType"`
	Body      json.RawMessage `json:"body"`
}

// NewUserWebhookTask serializes the payload into an asynq.Task for user
// webhook delivery.
func NewUserWebhookTask(payload WebhookPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeUserWebhook, body)
}

// NewSystemWebhookTask serializes the payload into an asynq.Task for system
// webhook delivery.
func NewSystemWebhookTask(payload WebhookPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeSystemWebhook, body)
}

// DecodeWebhookPayload extracts a WebhookPayload from a task body. The same
// encoder/decoder is reused for both user and system webhook tasks since the
// wire format is identical.
func DecodeWebhookPayload(body []byte) (WebhookPayload, error) {
	var p WebhookPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return WebhookPayload{}, err
	}
	return p, nil
}

// DeleteAccountPayload carries the userID whose related rows should be
// cascade-deleted by the background processor.
type DeleteAccountPayload struct {
	UserID string `json:"userId"`
}

// NewDeleteAccountTask serializes a DeleteAccountPayload into an asynq.Task.
func NewDeleteAccountTask(payload DeleteAccountPayload) *asynq.Task {
	body, _ := json.Marshal(payload)
	return asynq.NewTask(TaskTypeDeleteAccount, body)
}

// DecodeDeleteAccountPayload extracts a DeleteAccountPayload from a task body.
func DecodeDeleteAccountPayload(body []byte) (DeleteAccountPayload, error) {
	var p DeleteAccountPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return DeleteAccountPayload{}, err
	}
	return p, nil
}
