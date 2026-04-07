// Package processors contains asynq task handlers used by the queue worker.
package processors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/queue"
)

// HTTPSigner abstracts the signed POST capability of activitypub.Client so
// that DeliverProcessor can be unit-tested without an actual HTTP client.
type HTTPSigner interface {
	PostSigned(url string, body []byte, key *activitypub.PrivateKey) (*http.Response, error)
}

// DeliverProcessor handles ap:deliver tasks by posting the activity body to
// the recipient inbox with an HTTP signature.
type DeliverProcessor struct {
	signer HTTPSigner
}

// NewDeliverProcessor constructs a DeliverProcessor.
func NewDeliverProcessor(signer HTTPSigner) *DeliverProcessor {
	return &DeliverProcessor{signer: signer}
}

// Handle dispatches a single deliver task. The asynq runtime invokes this for
// every dequeued task.
func (p *DeliverProcessor) Handle(_ context.Context, t *asynq.Task) error {
	payload, err := queue.DecodeDeliverPayload(t.Payload())
	if err != nil {
		// payload が壊れているジョブは何度リトライしても無意味なのでスキップ。
		return fmt.Errorf("decode deliver payload: %w: %w", err, asynq.SkipRetry)
	}

	key, err := activitypub.NewPrivateKey(payload.KeyID, payload.KeyPEM)
	if err != nil {
		// 鍵が壊れているジョブも同様にスキップする。
		return fmt.Errorf("parse private key: %w: %w", err, asynq.SkipRetry)
	}

	resp, err := p.signer.PostSigned(payload.Inbox, payload.Body, key)
	if err != nil {
		// ネットワークエラーや接続失敗はリトライする。
		slog.Warn("ap deliver: post failed",
			"inbox", payload.Inbox, "err", err)
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusGone,
		resp.StatusCode == http.StatusNotFound:
		// 410 / 404: 受信側がもう存在しない。リトライしても無駄なのでスキップ。
		slog.Info("ap deliver: target gone",
			"inbox", payload.Inbox, "status", resp.StatusCode)
		return fmt.Errorf("target gone (%d): %w", resp.StatusCode, asynq.SkipRetry)
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// その他の4xxは受信側の不正リクエスト扱い。リトライしてもまず通らないので
		// スキップしてログに残す。
		slog.Warn("ap deliver: client error",
			"inbox", payload.Inbox, "status", resp.StatusCode)
		return fmt.Errorf("client error (%d): %w", resp.StatusCode, asynq.SkipRetry)
	default:
		// 5xx は受信側の一時的な障害。リトライさせる。
		slog.Warn("ap deliver: server error",
			"inbox", payload.Inbox, "status", resp.StatusCode)
		return errors.New("server error: " + resp.Status)
	}
}
