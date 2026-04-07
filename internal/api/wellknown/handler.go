// Package wellknown provides /.well-known/* discovery endpoints.
package wellknown

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/core/user"
)

// Handler handles webfinger / host-meta / nodeinfo discovery.
type Handler struct {
	urls        *activitypub.URLBuilder
	userService *user.Service
	host        string
}

// NewHandler constructs a Handler.
func NewHandler(urls *activitypub.URLBuilder, userService *user.Service, host string) *Handler {
	return &Handler{urls: urls, userService: userService, host: host}
}

// Webfinger handles GET /.well-known/webfinger.
//
// クエリパラメータ resource は acct:username@host または actor URI 形式。
// マッチするローカルユーザーが存在しない場合は404を返す。
func (h *Handler) Webfinger(c echo.Context) error {
	resource := c.QueryParam("resource")
	if resource == "" {
		return c.NoContent(http.StatusBadRequest)
	}

	username, ok := h.parseResource(resource)
	if !ok {
		return c.NoContent(http.StatusBadRequest)
	}

	bundle, err := h.userService.ShowByUsername(username, nil)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	uri := h.urls.UserURI(bundle.User.ID)
	resp := map[string]any{
		"subject": "acct:" + bundle.User.Username + "@" + h.host,
		"links": []map[string]any{
			{
				"rel":  "self",
				"type": "application/activity+json",
				"href": uri,
			},
			{
				"rel":  "http://webfinger.net/rel/profile-page",
				"type": "text/html",
				"href": uri,
			},
		},
	}
	return c.JSON(http.StatusOK, resp)
}

// HostMeta handles GET /.well-known/host-meta.
func (h *Handler) HostMeta(c echo.Context) error {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0">
  <Link rel="lrdd" type="application/xrd+xml" template="https://` + h.host + `/.well-known/webfinger?resource={uri}"/>
</XRD>`
	return c.Blob(http.StatusOK, "application/xrd+xml; charset=utf-8", []byte(xml))
}

// NodeInfoDiscovery handles GET /.well-known/nodeinfo.
func (h *Handler) NodeInfoDiscovery(c echo.Context) error {
	resp := map[string]any{
		"links": []map[string]any{
			{
				"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.1",
				"href": "https://" + h.host + "/nodeinfo/2.1",
			},
		},
	}
	return c.JSON(http.StatusOK, resp)
}

// parseResource extracts the local username from a webfinger resource string.
// 受け付ける形式: acct:user@host, acct:user, https://host/users/<id>
func (h *Handler) parseResource(resource string) (string, bool) {
	if acct, ok := strings.CutPrefix(resource, "acct:"); ok {
		parts := strings.Split(acct, "@")
		switch len(parts) {
		case 1:
			return parts[0], true
		case 2:
			if parts[1] != h.host {
				return "", false
			}
			return parts[0], true
		}
		return "", false
	}
	if strings.HasPrefix(resource, "https://") || strings.HasPrefix(resource, "http://") {
		u, err := url.Parse(resource)
		if err != nil {
			return "", false
		}
		if u.Host != h.host {
			return "", false
		}
		// expecting /users/<id>
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) == 2 && parts[0] == "users" {
			return parts[1], true
		}
		return "", false
	}
	return "", false
}
