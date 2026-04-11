// Package ap provides ActivityPub resource endpoints for users and notes.
package ap

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// RemoteFetcher fetches remote ActivityPub objects.
type RemoteFetcher interface {
	FetchObject(uri string) ([]byte, error)
}

// RemoteResolver resolves remote actors and notes.
type RemoteResolver interface {
	ResolveActor(uri string) (*model.User, error)
	ResolveNote(uri string) (*model.Note, error)
}

// Handler handles ActivityPub resource endpoints.
type Handler struct {
	renderer       *activitypub.Renderer
	userService    *coreuser.Service
	queryService   *corenote.QueryService
	keypairRepo    repository.UserKeypairRepository
	idGen          id.Generator
	remoteFetcher  RemoteFetcher
	remoteResolver RemoteResolver
}

// SetRemote attaches remote AP fetcher and resolver.
func (h *Handler) SetRemote(fetcher RemoteFetcher, resolver RemoteResolver) {
	h.remoteFetcher = fetcher
	h.remoteResolver = resolver
}

// NewHandler constructs a Handler.
func NewHandler(
	renderer *activitypub.Renderer,
	userService *coreuser.Service,
	queryService *corenote.QueryService,
	keypairRepo repository.UserKeypairRepository,
	idGen id.Generator,
) *Handler {
	return &Handler{
		renderer:     renderer,
		userService:  userService,
		queryService: queryService,
		keypairRepo:  keypairRepo,
		idGen:        idGen,
	}
}

// User handles GET /users/:id.
//
// Acceptヘッダ application/activity+json (または HTML 経由でない場合) なら
// AS Person を返す。HTMLリクエストは将来のWebUIでハンドルする想定だが、現状
// はリダイレクトせず常にAP表現を返す。
func (h *Handler) User(c echo.Context) error {
	id := c.Param("id")
	bundle, err := h.userService.ShowByID(id)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	// リモートユーザーへのリダイレクト相当は将来対応
	if bundle.User.Host != nil {
		return c.NoContent(http.StatusNotFound)
	}
	keypair, err := h.keypairRepo.FindByUserID(bundle.User.ID)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	person := h.renderer.RenderPerson(bundle.User, keypair.PublicKey)
	return writeActivityJSON(c, person)
}

// UserByAcct handles GET /@:acct with ActivityPub content negotiation.
// When the Accept header prefers application/activity+json (which is how
// other AP implementations resolve actors that they discovered via
// WebFinger or a raw link), return the Person document. Otherwise fall
// through to the HTML frontend by returning echo.ErrNotFound so the
// catch-all route can handle it.
func (h *Handler) UserByAcct(c echo.Context) error {
	if !wantsActivityJSON(c.Request().Header.Get("Accept")) {
		return echo.ErrNotFound
	}
	acct := c.Param("acct")
	// /@alice or /@alice@host 形式。ローカルのみ扱う。
	username := acct
	if idx := strings.Index(acct, "@"); idx >= 0 {
		username = acct[:idx]
	}
	bundle, err := h.userService.ShowByUsername(username, nil)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	if bundle.User.Host != nil {
		return c.NoContent(http.StatusNotFound)
	}
	keypair, err := h.keypairRepo.FindByUserID(bundle.User.ID)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	person := h.renderer.RenderPerson(bundle.User, keypair.PublicKey)
	return writeActivityJSON(c, person)
}

// writeActivityJSON serializes v and writes it with the ActivityPub
// content type. Remote implementations (Misskey, Mastodon, ...) check
// Content-Type before treating a response as an AP document, so a plain
// application/json would cause them to reject it.
func writeActivityJSON(c echo.Context, v any) error {
	return c.Blob(http.StatusOK, `application/activity+json; charset=utf-8`, mustMarshal(v))
}

// wantsActivityJSON reports whether the caller prefers an AP document.
// Any occurrence of application/activity+json or application/ld+json in
// the Accept header is treated as a positive signal.
func wantsActivityJSON(accept string) bool {
	return strings.Contains(accept, "application/activity+json") ||
		strings.Contains(accept, "application/ld+json")
}

// APIGet handles POST /api/ap/get — Admin専用。URIからActivityPubオブジェクトを取得。
func (h *Handler) APIGet(c echo.Context) error {
	var req struct {
		URI string `json:"uri"`
	}
	if err := c.Bind(&req); err != nil || req.URI == "" {
		return c.JSON(http.StatusBadRequest, apAPIError("INVALID_PARAM", "uri is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	// ローカルURIからオブジェクトを解決
	obj, err := h.resolveLocal(req.URI)
	if err == nil {
		return c.JSON(http.StatusOK, obj)
	}

	// リモートフェッチ
	if h.remoteFetcher != nil {
		data, err := h.remoteFetcher.FetchObject(req.URI)
		if err == nil {
			var parsed map[string]any
			if json.Unmarshal(data, &parsed) == nil {
				return c.JSON(http.StatusOK, parsed)
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]any{})
}

// APIShow handles POST /api/ap/show — URIからUser/Noteを解決して返す。
func (h *Handler) APIShow(c echo.Context) error {
	var req struct {
		URI string `json:"uri"`
	}
	if err := c.Bind(&req); err != nil || req.URI == "" {
		return c.JSON(http.StatusBadRequest, apAPIError("INVALID_PARAM", "uri is required.", "ed1d7571-a3ac-4370-899c-0dbe5e230cc8"))
	}

	// ローカルのノートURIかチェック (/notes/ を含む)
	if noteID := extractLocalID(req.URI, "/notes/"); noteID != "" {
		n, err := h.queryService.Show(nil, noteID)
		if err == nil && n.UserHost == nil {
			return c.JSON(http.StatusOK, map[string]any{
				"type":   "Note",
				"object": packNoteForAPI(n),
			})
		}
	}

	// ローカルのユーザーURIかチェック (/users/ を含む)
	if userID := extractLocalID(req.URI, "/users/"); userID != "" {
		bundle, err := h.userService.ShowByID(userID)
		if err == nil && bundle.User.Host == nil {
			return c.JSON(http.StatusOK, map[string]any{
				"type":   "User",
				"object": packUserForAPI(bundle.User),
			})
		}
	}

	// リモートオブジェクトをフェッチしてType判定
	// Note の場合は ResolveNote でローカルDBに取り込み、local ID を返す。
	// これにより後続の notes/reactions/create などが local note を見つけられる。
	if h.remoteFetcher != nil {
		if data, err := h.remoteFetcher.FetchObject(req.URI); err == nil {
			var parsed map[string]any
			if json.Unmarshal(data, &parsed) == nil {
				t, _ := parsed["type"].(string)
				switch t {
				case "Note", "Article", "Question":
					if h.remoteResolver != nil {
						if remoteNote, err := h.remoteResolver.ResolveNote(req.URI); err == nil {
							return c.JSON(http.StatusOK, map[string]any{
								"type":   "Note",
								"object": packNoteForAPI(remoteNote),
							})
						}
					}
					// Resolver 無し or ResolveNote 失敗時は raw AP JSON を返す
					return c.JSON(http.StatusOK, map[string]any{
						"type":   "Note",
						"object": parsed,
					})
				case "Person", "Service", "Application", "Organization", "Group":
					if h.remoteResolver != nil {
						if remoteUser, err := h.remoteResolver.ResolveActor(req.URI); err == nil {
							return c.JSON(http.StatusOK, map[string]any{
								"type":   "User",
								"object": packUserForAPI(remoteUser),
							})
						}
					}
				}
			}
		}
	}

	// フェッチ失敗 or Type 不明の場合は ResolveActor を試す (webfinger 経由の
	// /@user URL に対する fetch が失敗するケースがあるため)。
	if h.remoteResolver != nil {
		if remoteUser, err := h.remoteResolver.ResolveActor(req.URI); err == nil {
			return c.JSON(http.StatusOK, map[string]any{
				"type":   "User",
				"object": packUserForAPI(remoteUser),
			})
		}
	}

	return c.JSON(http.StatusNotFound, apAPIError("NO_SUCH_OBJECT", "No such object.", "dc94d745-1262-4e63-a17d-fecaa57efc82"))
}

// resolveLocal attempts to resolve a local URI to an AP object.
func (h *Handler) resolveLocal(uri string) (any, error) {
	if noteID := extractLocalID(uri, "/notes/"); noteID != "" {
		n, err := h.queryService.Show(nil, noteID)
		if err != nil {
			return nil, err
		}
		return h.renderer.RenderNote(n, h.idGen), nil
	}
	if userID := extractLocalID(uri, "/users/"); userID != "" {
		bundle, err := h.userService.ShowByID(userID)
		if err != nil {
			return nil, err
		}
		keypair, err := h.keypairRepo.FindByUserID(bundle.User.ID)
		if err != nil {
			return nil, err
		}
		return h.renderer.RenderPerson(bundle.User, keypair.PublicKey), nil
	}
	return nil, http.ErrNotSupported
}

func apAPIError(code, message, id string) map[string]any {
	return map[string]any{
		"error": map[string]any{"message": message, "code": code, "id": id},
	}
}

func extractLocalID(uri, pathPrefix string) string {
	// URI末尾の /notes/{id} や /users/{id} からIDを抽出
	idx := len(uri) - 1
	for idx >= 0 && uri[idx] != '/' {
		idx--
	}
	if idx < 0 {
		return ""
	}
	// pathPrefixが含まれているか確認
	prefixIdx := -1
	for i := 0; i+len(pathPrefix) <= len(uri); i++ {
		if uri[i:i+len(pathPrefix)] == pathPrefix {
			prefixIdx = i
			break
		}
	}
	if prefixIdx < 0 {
		return ""
	}
	return uri[prefixIdx+len(pathPrefix):]
}

func packNoteForAPI(n *model.Note) map[string]any {
	result := map[string]any{
		"id":         n.ID,
		"text":       n.Text,
		"userId":     n.UserID,
		"visibility": n.Visibility,
	}
	if n.User != nil {
		result["user"] = packUserForAPI(n.User)
	}
	return result
}

func packUserForAPI(u *model.User) map[string]any {
	if u == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":       u.ID,
		"username": u.Username,
		"name":     u.Name,
		"host":     u.Host,
	}
}

// APNotes handles POST /api/ap/notes — stub returning empty array.
func (h *Handler) APNotes(c echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}

// Note handles GET /notes/:id.
func (h *Handler) Note(c echo.Context) error {
	noteID := c.Param("id")
	// 公開ノートのみAPでフェッチ可能 (非ログインから取得されるため viewer=nil)
	n, err := h.queryService.Show(nil, noteID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	// リモートノートはホスト元へリダイレクトすべきだが現状は404
	if n.UserHost != nil {
		return c.NoContent(http.StatusNotFound)
	}
	note := h.renderer.RenderNote(n, h.idGen)
	return writeActivityJSON(c, note)
}
