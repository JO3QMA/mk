package l10n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolve_ProfileLang(t *testing.T) {
	ja := "ja-JP"
	assert.Equal(t, "ja", Resolve(&ja, []string{"en-US"}))
}

func TestResolve_MetaLangsFallback(t *testing.T) {
	assert.Equal(t, "ja", Resolve(nil, []string{"ja-JP"}))
}

func TestResolve_EnglishDefault(t *testing.T) {
	assert.Equal(t, "en", Resolve(nil, nil))
	unknown := "fr-FR"
	assert.Equal(t, "en", Resolve(&unknown, []string{"de-DE"}))
}

func TestResolveFromHeader_MatchesInstanceLang(t *testing.T) {
	got := ResolveFromHeader("ja-JP,en;q=0.8", []string{"en-US", "ja-JP"})
	assert.Equal(t, "ja", got)
}

func TestResolveFromHeader_FallsBackToMeta(t *testing.T) {
	got := ResolveFromHeader("fr-FR", []string{"ja-JP"})
	assert.Equal(t, "ja", got)
}
