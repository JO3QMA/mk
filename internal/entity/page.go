package entity

import (
	"encoding/json"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
)

// PackPage converts a model.Page into the map shape returned by /api/pages/*
// and embedded as UserDetailed.pinnedPage. Returns nil when p is nil so
// callers can assign the result directly to a nilable field.
// When an idGen is supplied, createdAt is derived from the aidx ID and
// rendered in the same "2006-01-02T15:04:05.000Z" format used by other
// entity packers (PackNote / PackDriveFile etc). updatedAt is rendered with
// the same format for consistency.
func PackPage(p *model.Page, idGens ...id.Generator) map[string]any {
	if p == nil {
		return nil
	}
	const tsFormat = "2006-01-02T15:04:05.000Z"
	out := map[string]any{
		"id":                  p.ID,
		"updatedAt":           p.UpdatedAt.UTC().Format(tsFormat),
		"title":               p.Title,
		"name":                p.Name,
		"summary":             p.Summary,
		"alignCenter":         p.AlignCenter,
		"hideTitleWhenPinned": p.HideTitleWhenPinned,
		"font":                p.Font,
		"userId":              p.UserID,
		"eyeCatchingImageId":  p.EyeCatchingImageID,
		"content":             rawJSONBytes(p.Content),
		"variables":           rawJSONBytes(p.Variables),
		"script":              p.Script,
		"visibility":          string(p.Visibility),
		"likedCount":          p.LikedCount,
	}
	if len(idGens) > 0 && idGens[0] != nil {
		if t, err := idGens[0].ParseTime(p.ID); err == nil {
			out["createdAt"] = t.UTC().Format(tsFormat)
		}
	}
	return out
}

// rawJSONBytes returns the raw JSON bytes as json.RawMessage so JSON encoders
// emit jsonb column content verbatim. Empty bytes become nil (JSON null).
func rawJSONBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}
