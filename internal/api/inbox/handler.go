// Package inbox provides the ActivityPub inbox endpoint.
package inbox

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/model"
)

// HostBlockChecker reports whether a host is on the blocked list. Used by the
// inbox handler to reject activities from blocked instances.
type HostBlockChecker interface {
	IsBlocked(host string) bool
}

// InstanceTracker is invoked after a successfully verified inbound request so
// that the instance row's latestRequestReceivedAt can be updated. パッケージ
// 間の循環依存を避けるため interface で受け取る。
type InstanceTracker interface {
	MarkRequestReceived(host string) error
}

// Handler accepts incoming activities and dispatches them to the federation
// processor after verifying their HTTP signature.
type Handler struct {
	resolver        *federation.Resolver
	processor       *federation.Processor
	hostBlocker     HostBlockChecker
	instanceTracker InstanceTracker
}

// NewHandler constructs a Handler.
func NewHandler(resolver *federation.Resolver, processor *federation.Processor) *Handler {
	return &Handler{resolver: resolver, processor: processor}
}

// SetHostBlockChecker attaches a HostBlockChecker. 設定されると、シグネチャ
// 検証成功後の actor が属するホストが blocked リストに含まれる場合は 403 を
// 返して以降の処理をスキップする。
func (h *Handler) SetHostBlockChecker(c HostBlockChecker) {
	h.hostBlocker = c
}

// SetInstanceTracker attaches an InstanceTracker. 設定されると、署名検証に
// 成功するたびに対応 instance row の latestRequestReceivedAt が更新される。
func (h *Handler) SetInstanceTracker(t InstanceTracker) {
	h.instanceTracker = t
}

// Inbox handles POST /inbox and POST /users/:id/inbox.
//
// Userごとのinboxであっても処理は同じ (現状のシンプル実装ではshared inboxと
// 同様にactivityをdispatchするだけ)。
func (h *Handler) Inbox(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// Echo の Request からはホストヘッダが空のことがあるため、明示的に補完する。
	if c.Request().Header.Get("Host") == "" {
		c.Request().Header.Set("Host", c.Request().Host)
	}

	actor, err := h.verifySignature(c.Request())
	if err != nil {
		slog.Warn("inbox signature verification failed", "err", err)
		return c.NoContent(http.StatusUnauthorized)
	}

	if h.isHostBlocked(actor) {
		return c.NoContent(http.StatusForbidden)
	}

	h.touchInstance(actor)

	if err := h.processor.Process(body); err != nil {
		if errors.Is(err, federation.ErrUnsupportedActivity) {
			// 未対応typeでも 202 Accepted を返す。
			return c.NoContent(http.StatusAccepted)
		}
		slog.Warn("inbox process failed", "err", err)
		return c.NoContent(http.StatusBadRequest)
	}
	return c.NoContent(http.StatusAccepted)
}

// verifySignature parses the Signature header, resolves the actor, and
// validates the request signature against the actor's stored public key.
// 戻り値の actor は後続の host block チェックに再利用される。
func (h *Handler) verifySignature(req *http.Request) (*model.User, error) {
	parsed, err := activitypub.ParseSignatureHeader(req.Header.Get("Signature"))
	if err != nil {
		return nil, err
	}
	actorURI := activitypub.ResolveKeyURL(parsed.KeyID)
	actor, err := h.resolver.ResolveActor(actorURI)
	if err != nil {
		return nil, err
	}
	pem, err := h.resolver.PublicKeyForActor(actor.ID)
	if err != nil {
		return nil, err
	}
	if err := activitypub.VerifyRequest(req, pem); err != nil {
		return nil, err
	}
	return actor, nil
}

// isHostBlocked reports whether the actor's host is on the blocked list.
// hostBlocker が未設定 / actor がローカル / Host nil なら false。
func (h *Handler) isHostBlocked(actor *model.User) bool {
	if h.hostBlocker == nil || actor == nil || actor.Host == nil {
		return false
	}
	return h.hostBlocker.IsBlocked(*actor.Host)
}

// touchInstance is a best-effort hook into the InstanceTracker. tracker が
// 未設定 / actor がローカル / Host nil の場合は no-op。エラーは握りつぶす。
func (h *Handler) touchInstance(actor *model.User) {
	if h.instanceTracker == nil || actor == nil || actor.Host == nil {
		return
	}
	_ = h.instanceTracker.MarkRequestReceived(*actor.Host)
}
