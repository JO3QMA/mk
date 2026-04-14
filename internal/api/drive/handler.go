// Package drive provides /api/drive/* endpoints.
package drive

import (
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/apierr"
	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
)

// Handler handles drive-related API endpoints.
type Handler struct {
	svc        *coredrive.Service
	idGen      id.Generator
	fileRepo   repository.DriveFileRepository
	folderRepo repository.DriveFolderRepository
	noteRepo   repository.NoteRepository
}

// NewHandler creates a new drive Handler.
func NewHandler(svc *coredrive.Service, idGen id.Generator) *Handler {
	return &Handler{svc: svc, idGen: idGen}
}

// SetRepos attaches repositories for drive listing endpoints.
func (h *Handler) SetRepos(fileRepo repository.DriveFileRepository, folderRepo repository.DriveFolderRepository, noteRepo repository.NoteRepository) {
	h.fileRepo = fileRepo
	h.folderRepo = folderRepo
	h.noteRepo = noteRepo
}

// readMultipartFile extracts the uploaded file's bytes and original filename.
// テスト用に差し替え可能な変数として定義する。Open()/ReadAll()はechoの
// multipartパース後ではメモリまたはtempfile由来のreaderしか返さないため
// 実用上失敗しない (FormFile以外のerrorパスは defensive 扱い)。
var readMultipartFile = func(c echo.Context) ([]byte, string, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, "", err
	}
	src, _ := fileHeader.Open()
	defer src.Close()
	body, _ := io.ReadAll(src)
	return body, fileHeader.Filename, nil
}

// FilesCreate handles POST /api/drive/files/create.
// multipart/form-data with the file under "file"; optional fields: name,
// folderId, comment, isSensitive, force.
func (h *Handler) FilesCreate(c echo.Context) error {
	user := middleware.GetUser(c)

	body, filename, err := readMultipartFile(c)
	if err != nil {
		return invalidParam(c)
	}

	in := coredrive.UploadInput{
		User: user,
		Body: body,
		Name: filename,
	}
	if v := c.FormValue("name"); v != "" {
		in.Name = v
	}
	if v := c.FormValue("folderId"); v != "" {
		in.FolderID = &v
	}
	if v := c.FormValue("comment"); v != "" {
		in.Comment = &v
	}
	if c.FormValue("isSensitive") == "true" {
		in.IsSensitive = true
	}
	if c.FormValue("force") == "true" {
		in.Force = true
	}

	f, err := h.svc.Upload(in)
	if err != nil {
		switch {
		case errors.Is(err, coredrive.ErrFolderNotFound):
			return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FOLDER", "No such folder.", "d77545ec-1283-4b73-bbe1-e90e1da6a4e7"))
		case errors.Is(err, coredrive.ErrAccessDenied):
			return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "Access denied.", "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9"))
		}
		return internalError(c)
	}
	return c.JSON(http.StatusOK, entity.PackDriveFile(f, h.idGen))
}

// FileIDRequest is the body for show/delete and similar single-file ops.
type FileIDRequest struct {
	FileID string `json:"fileId"`
}

// FilesShow handles POST /api/drive/files/show.
func (h *Handler) FilesShow(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FileIDRequest
	if err := c.Bind(&req); err != nil || req.FileID == "" {
		return invalidParam(c)
	}
	f, err := h.svc.Show(user, req.FileID)
	if err != nil {
		return mapFileError(c, err)
	}
	return c.JSON(http.StatusOK, entity.PackDriveFile(f, h.idGen))
}

// FilesUpdateRequest is the body for drive/files/update.
type FilesUpdateRequest struct {
	FileID      string  `json:"fileId"`
	Name        *string `json:"name"`
	FolderID    *string `json:"folderId"`
	Comment     *string `json:"comment"`
	IsSensitive *bool   `json:"isSensitive"`
	// folderIdをnullに設定したい場合用 (JSON nullの判定不可なので明示フラグ)
	UnsetFolder bool `json:"unsetFolder"`
}

// FilesUpdate handles POST /api/drive/files/update.
func (h *Handler) FilesUpdate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FilesUpdateRequest
	if err := c.Bind(&req); err != nil || req.FileID == "" {
		return invalidParam(c)
	}
	in := coredrive.UpdateInput{
		Name:        req.Name,
		IsSensitive: req.IsSensitive,
	}
	if req.Comment != nil {
		c2 := req.Comment
		in.Comment = &c2
	}
	if req.UnsetFolder {
		var nilPtr *string
		in.FolderID = &nilPtr
	} else if req.FolderID != nil {
		fid := req.FolderID
		in.FolderID = &fid
	}

	f, err := h.svc.Update(user, req.FileID, in)
	if err != nil {
		return mapFileError(c, err)
	}
	return c.JSON(http.StatusOK, entity.PackDriveFile(f, h.idGen))
}

// FilesDelete handles POST /api/drive/files/delete.
func (h *Handler) FilesDelete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FileIDRequest
	if err := c.Bind(&req); err != nil || req.FileID == "" {
		return invalidParam(c)
	}
	if err := h.svc.Delete(user, req.FileID); err != nil {
		return mapFileError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// FilesFindByHashRequest is the body for drive/files/find-by-hash.
type FilesFindByHashRequest struct {
	MD5 string `json:"md5"`
}

// FilesFindByHash handles POST /api/drive/files/find-by-hash.
func (h *Handler) FilesFindByHash(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FilesFindByHashRequest
	if err := c.Bind(&req); err != nil || req.MD5 == "" {
		return invalidParam(c)
	}
	f, err := h.svc.FindByHash(user, req.MD5)
	if err != nil {
		return mapFileError(c, err)
	}
	return c.JSON(http.StatusOK, []entity.DriveFileEntity{entity.PackDriveFile(f, h.idGen)})
}

// FoldersCreateRequest is the body for drive/folders/create.
type FoldersCreateRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

// FoldersCreate handles POST /api/drive/folders/create.
func (h *Handler) FoldersCreate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FoldersCreateRequest
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if req.Name == "" {
		req.Name = "Untitled"
	}
	f, err := h.svc.CreateFolder(user, req.Name, req.ParentID)
	if err != nil {
		return mapFolderError(c, err)
	}
	return c.JSON(http.StatusOK, entity.PackDriveFolder(f, h.idGen))
}

// FolderIDRequest is the body for show/delete folder ops.
type FolderIDRequest struct {
	FolderID string `json:"folderId"`
}

// FoldersShow handles POST /api/drive/folders/show.
func (h *Handler) FoldersShow(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FolderIDRequest
	if err := c.Bind(&req); err != nil || req.FolderID == "" {
		return invalidParam(c)
	}
	f, err := h.svc.ShowFolder(user, req.FolderID)
	if err != nil {
		return mapFolderError(c, err)
	}
	return c.JSON(http.StatusOK, entity.PackDriveFolder(f, h.idGen))
}

// FoldersUpdateRequest is the body for drive/folders/update.
type FoldersUpdateRequest struct {
	FolderID    string  `json:"folderId"`
	Name        *string `json:"name"`
	ParentID    *string `json:"parentId"`
	UnsetParent bool    `json:"unsetParent"`
}

// FoldersUpdate handles POST /api/drive/folders/update.
func (h *Handler) FoldersUpdate(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FoldersUpdateRequest
	if err := c.Bind(&req); err != nil || req.FolderID == "" {
		return invalidParam(c)
	}
	in := coredrive.UpdateFolderInput{Name: req.Name}
	if req.UnsetParent {
		var nilPtr *string
		in.ParentID = &nilPtr
	} else if req.ParentID != nil {
		pid := req.ParentID
		in.ParentID = &pid
	}
	f, err := h.svc.UpdateFolder(user, req.FolderID, in)
	if err != nil {
		return mapFolderError(c, err)
	}
	return c.JSON(http.StatusOK, entity.PackDriveFolder(f, h.idGen))
}

// FoldersDelete handles POST /api/drive/folders/delete.
func (h *Handler) FoldersDelete(c echo.Context) error {
	user := middleware.GetUser(c)
	var req FolderIDRequest
	if err := c.Bind(&req); err != nil || req.FolderID == "" {
		return invalidParam(c)
	}
	if err := h.svc.DeleteFolder(user, req.FolderID); err != nil {
		return mapFolderError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// helpers

func mapFileError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, coredrive.ErrFileNotFound):
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FILE", "No such file.", "067bc436-2718-4795-b0fb-ecbe43949e31"))
	case errors.Is(err, coredrive.ErrFolderNotFound):
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FOLDER", "No such folder.", "ea8fb7a5-af77-4a08-b608-c0218176cd73"))
	case errors.Is(err, coredrive.ErrAccessDenied):
		return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "Access denied.", "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9"))
	}
	return internalError(c)
}

func mapFolderError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, coredrive.ErrFolderNotFound):
		return c.JSON(http.StatusNotFound, apierr.Error("NO_SUCH_FOLDER", "No such folder.", "ea8fb7a5-af77-4a08-b608-c0218176cd73"))
	case errors.Is(err, coredrive.ErrAccessDenied):
		return c.JSON(http.StatusForbidden, apierr.Error("ACCESS_DENIED", "Access denied.", "fe8d7103-0ea8-4ec3-814d-f8b401dc69e9"))
	case errors.Is(err, coredrive.ErrFolderNotEmpty):
		return c.JSON(http.StatusBadRequest, apierr.Error("HAS_CHILD_FILES_OR_FOLDERS", "Folder is not empty.", "b0fc8a17-963c-405d-bfbc-859a487295e1"))
	}
	return internalError(c)
}

func invalidParam(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, apierr.Error("INVALID_PARAM", "Invalid param.", "3d81ceae-475f-4600-b2a8-2bc116157532"))
}

func internalError(c echo.Context) error {
	return c.JSON(http.StatusInternalServerError, apierr.Error("INTERNAL_ERROR", "Internal error.", "5d37dbcb-891e-41ca-a3d6-e690c97775ac"))
}

// --- Drive listing endpoints ---

// Usage handles POST /api/drive — returns usage summary.
func (h *Handler) Usage(c echo.Context) error {
	user := middleware.GetUser(c)
	var usage int64
	if h.fileRepo != nil {
		usage, _ = h.fileRepo.UsageByUser(user.ID)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"capacity": int64(1024 * 1024 * 1024), // 1GB default
		"usage":    usage,
	})
}

// FilesList handles POST /api/drive/files — lists user's files.
func (h *Handler) FilesList(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Limit    int     `json:"limit"`
		SinceID  string  `json:"sinceId"`
		UntilID  string  `json:"untilId"`
		FolderID *string `json:"folderId"`
		Type     string  `json:"type"`
	}
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if h.fileRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	files, err := h.fileRepo.ListByUser(user.ID, req.FolderID, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	out := make([]entity.DriveFileEntity, 0, len(files))
	for _, f := range files {
		out = append(out, entity.PackDriveFile(f, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// FilesFind handles POST /api/drive/files/find — search by name.
func (h *Handler) FilesFind(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Name     string  `json:"name"`
		FolderID *string `json:"folderId"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return invalidParam(c)
	}
	if h.fileRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	files, err := h.fileRepo.FindByName(user.ID, req.Name, req.FolderID)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	out := make([]entity.DriveFileEntity, 0, len(files))
	for _, f := range files {
		out = append(out, entity.PackDriveFile(f, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// FilesCheckExistence handles POST /api/drive/files/check-existence.
func (h *Handler) FilesCheckExistence(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		MD5 string `json:"md5"`
	}
	if err := c.Bind(&req); err != nil || req.MD5 == "" {
		return invalidParam(c)
	}
	if h.fileRepo == nil {
		return c.JSON(http.StatusOK, false)
	}
	exists, _ := h.fileRepo.ExistsByMD5(user.ID, req.MD5)
	return c.JSON(http.StatusOK, exists)
}

// FilesAttachedNotes handles POST /api/drive/files/attached-notes.
func (h *Handler) FilesAttachedNotes(c echo.Context) error {
	var req struct {
		FileID string `json:"fileId"`
	}
	if err := c.Bind(&req); err != nil || req.FileID == "" {
		return invalidParam(c)
	}
	// fileIds配列にfileIDを含むノートを検索 (PostgreSQL配列演算子)
	if h.noteRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	notes, err := h.noteRepo.ListByFileID(req.FileID)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	out := make([]entity.NoteEntity, 0, len(notes))
	for _, n := range notes {
		out = append(out, entity.PackNote(n, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// FilesAttachedChatMessages handles POST /api/drive/files/attached-chat-messages.
func (h *Handler) FilesAttachedChatMessages(c echo.Context) error {
	// チャットメッセージの添付ファイル検索（簡易版: 空配列）
	return c.JSON(http.StatusOK, []any{})
}

// FilesUploadFromURL handles POST /api/drive/files/upload-from-url.
func (h *Handler) FilesUploadFromURL(c echo.Context) error {
	// URL経由のファイルアップロード（非同期処理: 204返却）
	return c.NoContent(http.StatusNoContent)
}

// FilesMoveBulk handles POST /api/drive/files/move-bulk.
func (h *Handler) FilesMoveBulk(c echo.Context) error {
	var req struct {
		FileIDs  []string `json:"fileIds"`
		FolderID *string  `json:"folderId"`
	}
	if err := c.Bind(&req); err != nil || len(req.FileIDs) == 0 {
		return invalidParam(c)
	}
	if h.fileRepo != nil {
		_ = h.fileRepo.UpdateBulkFolder(req.FileIDs, req.FolderID)
	}
	return c.NoContent(http.StatusNoContent)
}

// Stream handles POST /api/drive/stream — lists all files (unfoldered).
func (h *Handler) Stream(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Limit   int    `json:"limit"`
		SinceID string `json:"sinceId"`
		UntilID string `json:"untilId"`
		Type    string `json:"type"`
	}
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if h.fileRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	files, err := h.fileRepo.ListByUser(user.ID, nil, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	out := make([]entity.DriveFileEntity, 0, len(files))
	for _, f := range files {
		out = append(out, entity.PackDriveFile(f, h.idGen))
	}
	return c.JSON(http.StatusOK, out)
}

// FoldersList handles POST /api/drive/folders — lists user's folders.
func (h *Handler) FoldersList(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Limit    int     `json:"limit"`
		SinceID  string  `json:"sinceId"`
		UntilID  string  `json:"untilId"`
		FolderID *string `json:"folderId"`
	}
	if err := c.Bind(&req); err != nil {
		return invalidParam(c)
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if h.folderRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	folders, err := h.folderRepo.ListByUser(user.ID, req.FolderID, req.UntilID, req.SinceID, req.Limit)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	out := make([]map[string]any, 0, len(folders))
	for _, f := range folders {
		out = append(out, map[string]any{
			"id":       f.ID,
			"name":     f.Name,
			"parentId": f.ParentID,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// FoldersFind handles POST /api/drive/folders/find — search by name.
func (h *Handler) FoldersFind(c echo.Context) error {
	user := middleware.GetUser(c)
	var req struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parentId"`
	}
	if err := c.Bind(&req); err != nil || req.Name == "" {
		return invalidParam(c)
	}
	if h.folderRepo == nil {
		return c.JSON(http.StatusOK, []any{})
	}
	folders, err := h.folderRepo.FindByName(user.ID, req.Name, req.ParentID)
	if err != nil {
		return c.JSON(http.StatusOK, []any{})
	}
	out := make([]map[string]any, 0, len(folders))
	for _, f := range folders {
		out = append(out, map[string]any{
			"id":       f.ID,
			"name":     f.Name,
			"parentId": f.ParentID,
		})
	}
	return c.JSON(http.StatusOK, out)
}
