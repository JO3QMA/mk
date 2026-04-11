package webpush

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/repository"
)

// NoteRepoPacker adapts (NoteRepository + id.Generator) into the
// notification.NotePacker interface. The adapter re-uses entity.PackNote so
// that the Web Push payload matches the shape returned by /api/i/notifications.
type NoteRepoPacker struct {
	repo  repository.NoteRepository
	idGen id.Generator
}

// NewNoteRepoPacker constructs a NoteRepoPacker.
func NewNoteRepoPacker(repo repository.NoteRepository, idGen id.Generator) *NoteRepoPacker {
	return &NoteRepoPacker{repo: repo, idGen: idGen}
}

// PackNoteByID implements notification.NotePacker.
func (p *NoteRepoPacker) PackNoteByID(noteID string) (map[string]any, bool) {
	if p == nil || p.repo == nil {
		return nil, false
	}
	n, err := p.repo.FindByIDWithUser(noteID)
	if err != nil {
		return nil, false
	}
	return toMap(entity.PackNote(n, p.idGen))
}

// UserRepoPacker adapts UserRepository into notification.UserPacker using
// entity.PackUserLite.
type UserRepoPacker struct {
	repo repository.UserRepository
}

// NewUserRepoPacker constructs a UserRepoPacker.
func NewUserRepoPacker(repo repository.UserRepository) *UserRepoPacker {
	return &UserRepoPacker{repo: repo}
}

// PackUserByID implements notification.UserPacker.
func (p *UserRepoPacker) PackUserByID(userID string) (map[string]any, bool) {
	if p == nil || p.repo == nil {
		return nil, false
	}
	u, err := p.repo.FindByID(userID)
	if err != nil {
		return nil, false
	}
	return toMap(entity.PackUserLite(u))
}

// toMap round-trips any JSON-serializable value into a map. Used because the
// notification hook expects a map[string]any and the entity helpers return
// concrete structs.
func toMap(v any) (map[string]any, bool) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}
