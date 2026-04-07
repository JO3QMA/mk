package queue

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

// TaskTypeDeliver is the asynq task type used for outbound ActivityPub
// delivery jobs.
const TaskTypeDeliver = "ap:deliver"

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
