package i

import (
	"errors"
	"net/http"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/stretchr/testify/assert"
)

// stubTransferEnqueuer records calls and optionally returns an error.
type stubTransferEnqueuer struct {
	exportCalls []queue.ExportPayload
	importCalls []queue.ImportPayload
	exportErr   error
	importErr   error
}

func (s *stubTransferEnqueuer) EnqueueExport(p queue.ExportPayload) error {
	if s.exportErr != nil {
		return s.exportErr
	}
	s.exportCalls = append(s.exportCalls, p)
	return nil
}

func (s *stubTransferEnqueuer) EnqueueImport(p queue.ImportPayload) error {
	if s.importErr != nil {
		return s.importErr
	}
	s.importCalls = append(s.importCalls, p)
	return nil
}

func newTransferHandler() (*Handler, *stubTransferEnqueuer) {
	h, _, _, _ := newTestHandler(&testing.T{})
	enq := &stubTransferEnqueuer{}
	h.SetTransferEnqueuer(enq)
	return h, enq
}

// --- Export handlers ---

func TestExport_NotesNoAuthIfEnqueuerMissing(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := post(h.ExportNotes, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestExport_AllTypesEnqueue(t *testing.T) {
	cases := []struct {
		name    string
		handler func(h *Handler) func(c any) error
	}{}
	// map each endpoint to its expected payload type
	h, enq := newTransferHandler()
	user := &model.User{ID: "u1"}

	type ex struct {
		name string
		fn   func(c *Handler) func() error
		kind string
	}

	// Use a simple test invocation approach: call handler via post helper.
	assertOK := func(recCode int) {
		assert.Equal(t, http.StatusNoContent, recCode)
	}

	// notes
	assertOK(post(h.ExportNotes, `{}`, user).Code)
	// following
	assertOK(post(h.ExportFollowing, `{}`, user).Code)
	// blocking
	assertOK(post(h.ExportBlocking, `{}`, user).Code)
	// mute
	assertOK(post(h.ExportMute, `{}`, user).Code)
	// favorites
	assertOK(post(h.ExportFavorites, `{}`, user).Code)
	// user-lists
	assertOK(post(h.ExportUserLists, `{}`, user).Code)
	// antennas
	assertOK(post(h.ExportAntennas, `{}`, user).Code)
	// clips
	assertOK(post(h.ExportClips, `{}`, user).Code)

	assert.Len(t, enq.exportCalls, 8)
	// verify the types are distinct
	seen := make(map[string]bool)
	for _, c := range enq.exportCalls {
		seen[c.Type] = true
		assert.Equal(t, "u1", c.UserID)
	}
	assert.Len(t, seen, 8)
	_ = cases
}

func TestExport_EnqueuerFailure(t *testing.T) {
	h, enq := newTransferHandler()
	enq.exportErr = errors.New("enq boom")
	rec := post(h.ExportNotes, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Import handlers ---

func TestImport_AllTypesEnqueue(t *testing.T) {
	h, enq := newTransferHandler()
	user := &model.User{ID: "u1"}

	assertOK := func(recCode int) {
		assert.Equal(t, http.StatusNoContent, recCode)
	}

	assertOK(post(h.ImportFollowing, `{"fileId":"f1"}`, user).Code)
	assertOK(post(h.ImportBlocking, `{"fileId":"f1"}`, user).Code)
	assertOK(post(h.ImportMuting, `{"fileId":"f1"}`, user).Code)
	assertOK(post(h.ImportUserLists, `{"fileId":"f1"}`, user).Code)
	assertOK(post(h.ImportAntennas, `{"fileId":"f1"}`, user).Code)

	assert.Len(t, enq.importCalls, 5)
	for _, c := range enq.importCalls {
		assert.Equal(t, "u1", c.UserID)
		assert.Equal(t, "f1", c.FileID)
	}
}

func TestImport_MissingFileID(t *testing.T) {
	h, _ := newTransferHandler()
	rec := post(h.ImportFollowing, `{}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestImport_NoEnqueuer(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	rec := post(h.ImportFollowing, `{"fileId":"f1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestImport_EnqueuerError(t *testing.T) {
	h, enq := newTransferHandler()
	enq.importErr = errors.New("enq boom")
	rec := post(h.ImportFollowing, `{"fileId":"f1"}`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestImport_InvalidJSON(t *testing.T) {
	h, _ := newTransferHandler()
	rec := post(h.ImportFollowing, `not json`, &model.User{ID: "u1"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
