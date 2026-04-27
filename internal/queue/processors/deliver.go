// Package processors contains task handlers used by the queue worker.
// Handlers are driver-neutral: they accept a driver.Task and return
// driver.SkipRetry to suppress retries.
package processors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
)

// HTTPSigner abstracts the signed POST capability of activitypub.Client so
// that DeliverProcessor can be unit-tested without an actual HTTP client.
type HTTPSigner interface {
	PostSigned(url string, body []byte, key *activitypub.PrivateKey) (*http.Response, error)
}

// ResponseHook is invoked after each HTTP attempt so the instance metadata
// (isNotResponding / notRespondingSince) can be kept up to date. パッケージ間
// の循環依存を避けるため interface で受け取る。実装は core/instance.Service。
type ResponseHook interface {
	RecordResponseSuccess(host string) error
	RecordResponseError(host string) error
}

// ChartHook is invoked after each HTTP attempt so the chart subsystem
// can record ApRequest / Federation / Instance metrics. パッケージ間の
// 循環依存を避けるため interface で受け取る。実装は core/chart/charthook。
type ChartHook interface {
	OnDelivered(host string, succeeded bool)
}

// SuspendedChecker reports whether delivery to a host should be skipped
// based on meta.deliverSuspendedSoftware.
type SuspendedChecker interface {
	IsSuspended(host string) bool
}

// DeliverProcessor handles ap:deliver tasks by posting the activity body to
// the recipient inbox with an HTTP signature.
type DeliverProcessor struct {
	signer           HTTPSigner
	responseHook     ResponseHook
	chartHook        ChartHook
	suspendedChecker SuspendedChecker
}

// NewDeliverProcessor constructs a DeliverProcessor.
func NewDeliverProcessor(signer HTTPSigner) *DeliverProcessor {
	return &DeliverProcessor{signer: signer}
}

// SetSuspendedChecker attaches a checker for deliverSuspendedSoftware.
func (p *DeliverProcessor) SetSuspendedChecker(c SuspendedChecker) {
	p.suspendedChecker = c
}

// SetResponseHook attaches a ResponseHook used to update instance health flags.
func (p *DeliverProcessor) SetResponseHook(h ResponseHook) {
	p.responseHook = h
}

// SetChartHook attaches a ChartHook invoked after each delivery attempt.
func (p *DeliverProcessor) SetChartHook(h ChartHook) {
	p.chartHook = h
}

// hostFromInbox returns the host portion of an inbox URL, or "" if the URL is
// not parseable. ResponseHook 通知用に共通化する。
func hostFromInbox(inbox string) string {
	u, err := url.Parse(inbox)
	if err != nil {
		return ""
	}
	return u.Host
}

// recordSuccess is a best-effort wrapper that fires both the response
// hook and the chart hook for a successful inbox POST.
func (p *DeliverProcessor) recordSuccess(inbox string) {
	host := hostFromInbox(inbox)
	if p.responseHook != nil && host != "" {
		_ = p.responseHook.RecordResponseSuccess(host)
	}
	if p.chartHook != nil {
		p.chartHook.OnDelivered(host, true)
	}
}

// recordError is a best-effort wrapper that fires both the response
// hook and the chart hook for a failed inbox POST.
func (p *DeliverProcessor) recordError(inbox string) {
	host := hostFromInbox(inbox)
	if p.responseHook != nil && host != "" {
		_ = p.responseHook.RecordResponseError(host)
	}
	if p.chartHook != nil {
		p.chartHook.OnDelivered(host, false)
	}
}

// Handle dispatches a single deliver task. The driver runtime invokes
// this for every dequeued task.
func (p *DeliverProcessor) Handle(_ context.Context, t driver.Task) error {
	payload, err := queue.DecodeDeliverPayload(t.Payload())
	if err != nil {
		// payload が壊れているジョブは何度リトライしても無意味なのでスキップ。
		return fmt.Errorf("decode deliver payload: %w: %w", err, driver.SkipRetry)
	}

	key, err := activitypub.NewPrivateKey(payload.KeyID, payload.KeyPEM)
	if err != nil {
		// 鍵が壊れているジョブも同様にスキップする。
		return fmt.Errorf("parse private key: %w: %w", err, driver.SkipRetry)
	}

	// deliverSuspendedSoftware: 対象インスタンスの software がリストに該当すればスキップ
	if p.suspendedChecker != nil {
		host := hostFromInbox(payload.Inbox)
		if host != "" && p.suspendedChecker.IsSuspended(host) {
			slog.Info("ap deliver: skip (software suspended)", "inbox", payload.Inbox)
			return nil
		}
	}

	resp, err := p.signer.PostSigned(payload.Inbox, payload.Body, key)
	if err != nil {
		// ネットワークエラーや接続失敗はリトライする。
		slog.Warn("ap deliver: post failed",
			"inbox", payload.Inbox, "err", err)
		p.recordError(payload.Inbox)
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		p.recordSuccess(payload.Inbox)
		return nil
	case resp.StatusCode == http.StatusGone,
		resp.StatusCode == http.StatusNotFound:
		// 410 / 404: 受信側がもう存在しない。リトライしても無駄なのでスキップ。
		// 「応答した」事自体は事実なので isNotResponding は解除する。
		slog.Info("ap deliver: target gone",
			"inbox", payload.Inbox, "status", resp.StatusCode)
		p.recordSuccess(payload.Inbox)
		return fmt.Errorf("target gone (%d): %w", resp.StatusCode, driver.SkipRetry)
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// その他の4xxは受信側の不正リクエスト扱い。HTTP として応答が返って
		// きているので isNotResponding 状態は解除する。
		slog.Warn("ap deliver: client error",
			"inbox", payload.Inbox, "status", resp.StatusCode)
		p.recordSuccess(payload.Inbox)
		return fmt.Errorf("client error (%d): %w", resp.StatusCode, driver.SkipRetry)
	default:
		// 5xx は受信側の一時的な障害。リトライさせる + 不調状態としてマーク。
		slog.Warn("ap deliver: server error",
			"inbox", payload.Inbox, "status", resp.StatusCode)
		p.recordError(payload.Inbox)
		return errors.New("server error: " + resp.Status)
	}
}
