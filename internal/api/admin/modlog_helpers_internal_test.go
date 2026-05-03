package admin

// Internal (package admin) test for modlog helpers. Lets us hit the
// nil-target defensive branch of logUserActionWithUser directly without
// going through a handler. Phase 1 review iter 2 #1 follow-up.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/core/moderationlog"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/testutil"
)

// newCtx builds a minimal echo.Context for helper invocation. The
// helpers only touch c.Request().Context() so a stub request is enough.
func newCtx() echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec)
}

func TestLogUserActionWithUser_NilTargetIsNoop(t *testing.T) {
	repo := testutil.NewMockModerationLogRepository()
	gen, err := id.NewGenerator("aidx")
	if err != nil {
		t.Fatalf("idgen: %v", err)
	}
	h := &Handler{modLogService: moderationlog.New(repo, gen)}

	// nil target — must early-return without writing anything.
	h.logUserActionWithUser(newCtx(), moderationlog.LogSuspend, nil)

	if got := repo.Snapshot(); len(got) != 0 {
		t.Errorf("expected no log writes, got %d", len(got))
	}
}
