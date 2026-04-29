package processors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
)

// FederationProcessor is the narrow surface of federation.Processor that
// InboxProcessor consumes (#534). 切り出すことで unit test が
// federation.NewProcessor(...) の重い依存 chain (resolver / following /
// reaction / userRepo / noteRepo) を組まなくても済む。
type FederationProcessor interface {
	Process(body []byte) error
}

// InboxProcessor handles ap:inbox tasks by replaying the inbound AP
// activity through federation.Processor (#534). The HTTP handler has
// already verified the signature and committed instance / chart hooks
// synchronously, so the worker only needs to apply the activity to local
// state.
//
// 各 activity handler は冪等であることを前提とする (Misskey TS と同じ
// 戦略)。順序保証や per-actor lock は持たず、後着の Update が一時的に
// 旧 state を上書きする可能性は許容する設計 (#534 issue body 参照)。
type InboxProcessor struct {
	processor FederationProcessor
}

// NewInboxProcessor constructs an InboxProcessor wrapping the supplied
// federation.Processor.
func NewInboxProcessor(p FederationProcessor) *InboxProcessor {
	return &InboxProcessor{processor: p}
}

// Handle dispatches a single inbox task. driver runtime invokes this for
// every dequeued task.
//
// payload decode 失敗は再 retry しても無意味なので driver.SkipRetry で
// 確定 fail にする。federation.ErrUnsupportedActivity は handler 不在
// (= 受け付けたが何もしない) 扱いで成功扱い。それ以外の処理エラーは
// driver の retry policy (inboxJobMaxAttempts) に任せる。
func (p *InboxProcessor) Handle(_ context.Context, t driver.Task) error {
	payload, err := queue.DecodeInboxPayload(t.Payload())
	if err != nil {
		return fmt.Errorf("decode inbox payload: %w: %w", err, driver.SkipRetry)
	}
	if err := p.processor.Process(payload.Body); err != nil {
		if errors.Is(err, federation.ErrUnsupportedActivity) {
			// 未対応 type は HTTP 同期処理時と同じく成功扱い (sender に
			// retry 要求しない)。
			slog.Debug("inbox: unsupported activity, dropped", "host", payload.Host)
			return nil
		}
		return fmt.Errorf("process inbox activity: %w", err)
	}
	return nil
}
