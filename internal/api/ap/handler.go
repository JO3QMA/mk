// Package ap provides ActivityPub resource endpoints for users and notes.
package ap

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/repository"
)

// Handler handles ActivityPub resource endpoints.
type Handler struct {
	renderer     *activitypub.Renderer
	userService  *coreuser.Service
	queryService *corenote.QueryService
	keypairRepo  repository.UserKeypairRepository
	idGen        id.Generator
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
	return c.JSONBlob(http.StatusOK, mustMarshal(person))
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
	return c.JSONBlob(http.StatusOK, mustMarshal(note))
}
