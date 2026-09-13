// Package l10n resolves locale tags for outbound email copy.
package l10n

import (
	"strings"

	"github.com/shiroha-a/mk/internal/model"
)

// Resolve picks the email locale from a user profile lang, then instance langs,
// then English.
func Resolve(profileLang *string, metaLangs []string) string {
	if profileLang != nil && *profileLang != "" {
		if lang := normalizeKnown(*profileLang); lang != "" {
			return lang
		}
	}
	for _, l := range metaLangs {
		if lang := normalizeKnown(l); lang != "" {
			return lang
		}
	}
	return "en"
}

// ResolveFromHeader matches Accept-Language against instance langs, then falls
// back to Resolve(nil, metaLangs).
func ResolveFromHeader(acceptLanguage string, metaLangs []string) string {
	if acceptLanguage != "" {
		for _, part := range strings.Split(acceptLanguage, ",") {
			tag := strings.TrimSpace(strings.Split(part, ";")[0])
			if tag == "" || tag == "*" {
				continue
			}
			want := normalizeKnown(tag)
			if want == "" {
				continue
			}
			for _, ml := range metaLangs {
				if normalizeKnown(ml) == want {
					return want
				}
			}
		}
	}
	return Resolve(nil, metaLangs)
}

// LangsFromMeta returns meta.langs when meta is non-nil.
func LangsFromMeta(meta *model.Meta) []string {
	if meta == nil {
		return nil
	}
	return meta.Langs
}

func normalizeKnown(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		tag = tag[:i]
	}
	switch tag {
	case "ja":
		return "ja"
	case "en":
		return "en"
	default:
		return ""
	}
}
