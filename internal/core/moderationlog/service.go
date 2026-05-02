package moderationlog

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"gorm.io/datatypes"
)

// Service writes moderation log rows asynchronously.
//
// The Misskey TS counterpart (packages/backend/src/core/ModerationLogService.ts)
// is invoked fire-and-forget by callers without await. We follow the same
// pattern: Log returns immediately, the actual DB insert happens in a
// recovered goroutine, and any failure is observed via slog.Warn rather
// than propagated to the API caller. Audit logging must never block or
// fail the moderation action itself.
type Service struct {
	repo  repository.ModerationLogRepository
	idGen id.Generator
}

// New constructs a Service. Either dependency may be nil during tests or
// partial wiring; in that case Log degrades to a silent no-op.
func New(repo repository.ModerationLogRepository, idGen id.Generator) *Service {
	return &Service{repo: repo, idGen: idGen}
}

// Log records a moderation action performed by moderatorID. The info map is
// JSON-encoded into moderation_log.info; key names must match Misskey TS
// (e.g. "userId", "userUsername", "userHost") because the shared frontend
// reads them by exact field name.
//
// Log never blocks and never returns errors. nil receiver / unwired deps /
// JSON encoding failures / repo errors all surface as warn-level logs.
func (s *Service) Log(moderatorID string, t LogType, info map[string]any) {
	if s == nil || s.repo == nil || s.idGen == nil {
		return
	}
	if info == nil {
		info = map[string]any{}
	}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		slog.Warn("moderation log: marshal info failed",
			"type", t, "moderatorId", moderatorID, "err", err)
		return
	}
	entry := &model.ModerationLog{
		ID:     s.idGen.Generate(time.Now()),
		UserID: moderatorID,
		Type:   string(t),
		Info:   datatypes.JSON(infoBytes),
	}
	safeGo(func() {
		if err := s.repo.Create(entry); err != nil {
			slog.Warn("moderation log: write failed",
				"type", t, "moderatorId", moderatorID, "err", err)
		}
	})
}

// safeGo runs fn in a goroutine, recovering from panics to keep audit-log
// writes from taking down the request handler thread. Mirrors the private
// helper of the same name in internal/core/note/note_create_service.go;
// duplicated here intentionally to avoid leaking goroutine helpers across
// package boundaries.
func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Warn("moderation log: goroutine panic", "panic", r)
			}
		}()
		fn()
	}()
}
