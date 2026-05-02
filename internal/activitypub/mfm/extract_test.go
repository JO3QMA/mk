package mfm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectEmojiCodes_Empty(t *testing.T) {
	assert.Nil(t, CollectEmojiCodes())
	assert.Nil(t, CollectEmojiCodes(""))
	assert.Nil(t, CollectEmojiCodes("", "", ""))
}

func TestCollectEmojiCodes_PlainText(t *testing.T) {
	assert.Nil(t, CollectEmojiCodes("hello world"))
}

func TestCollectEmojiCodes_Single(t *testing.T) {
	assert.Equal(t, []string{"foo"}, CollectEmojiCodes(":foo:"))
	assert.Equal(t, []string{"foo"}, CollectEmojiCodes("hello :foo: world"))
}

func TestCollectEmojiCodes_Multiple_DedupAndOrder(t *testing.T) {
	got := CollectEmojiCodes(":foo: :bar: :foo: :baz:")
	assert.Equal(t, []string{"foo", "bar", "baz"}, got)
}

func TestCollectEmojiCodes_NestedInsideMarkup(t *testing.T) {
	// MFM の bold / italic / fn 等の中にあっても拾えること
	got := CollectEmojiCodes("**:foo:** *:bar:* $[x2 :baz:]")
	assert.ElementsMatch(t, []string{"foo", "bar", "baz"}, got)
}

func TestCollectEmojiCodes_AcrossMultipleTexts(t *testing.T) {
	// text + cw のような複数 source からの dedup
	got := CollectEmojiCodes("body :foo:", "cw :bar:", "more :foo: :baz:")
	assert.Equal(t, []string{"foo", "bar", "baz"}, got)
}

func TestCollectEmojiCodes_IgnoresUnicodeEmoji(t *testing.T) {
	// Unicode emoji (😀) は NodeUnicodeEmoji なので含めない
	assert.Nil(t, CollectEmojiCodes("hello 😀 world"))
}

func TestCollectEmojiCodes_IgnoresMalformed(t *testing.T) {
	// 閉じ \`:\` が無いものは emoji code として parse されない
	assert.Nil(t, CollectEmojiCodes("hello :foo bar"))
	// 英数字 + _ + - 以外は emoji name として parse されない (parser.go::tryEmojiCode)
	assert.Nil(t, CollectEmojiCodes(":foo bar:"))
}
