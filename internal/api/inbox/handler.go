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
)

// Handler accepts incoming activities and dispatches them to the federation
// processor after verifying their HTTP signature.
type Handler struct {
	resolver  *federation.Resolver
	processor *federation.Processor
}

// NewHandler constructs a Handler.
func NewHandler(resolver *federation.Resolver, processor *federation.Processor) *Handler {
	return &Handler{resolver: resolver, processor: processor}
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

	if err := h.verifySignature(c.Request()); err != nil {
		slog.Warn("inbox signature verification failed", "err", err)
		return c.NoContent(http.StatusUnauthorized)
	}

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
func (h *Handler) verifySignature(req *http.Request) error {
	parsed, err := activitypub.ParseSignatureHeader(req.Header.Get("Signature"))
	if err != nil {
		return err
	}
	actorURI := activitypub.ResolveKeyURL(parsed.KeyID)
	actor, err := h.resolver.ResolveActor(actorURI)
	if err != nil {
		return err
	}
	pem, err := h.resolver.PublicKeyForActor(actor.ID)
	if err != nil {
		return err
	}
	return activitypub.VerifyRequest(req, pem)
}
