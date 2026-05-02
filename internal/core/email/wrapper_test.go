package email_test

import (
	"strings"
	"testing"

	coreemail "github.com/shiroha-a/mk/internal/core/email"
	"github.com/stretchr/testify/assert"
)

func TestWrapHTML_BasicShape(t *testing.T) {
	got := coreemail.WrapHTML(coreemail.HTMLWrapInput{
		SiteName: "ExampleSite",
		SiteURL:  "https://example.test",
		LogoURL:  "https://example.test/logo.png",
		Subject:  "Hello",
		BodyHTML: "<p>Hi there</p>",
	})
	// 必須要素
	assert.True(t, strings.HasPrefix(got, "<!doctype html>"), "doctype が先頭")
	assert.Contains(t, got, "<title>Hello</title>")
	assert.Contains(t, got, "<h1 style=\"margin:0 0 1em 0\">Hello</h1>")
	assert.Contains(t, got, "<p>Hi there</p>", "BodyHTML はそのまま埋まる (caller responsibility)")
	assert.Contains(t, got, "https://example.test/logo.png")
	assert.Contains(t, got, "alt=\"ExampleSite\"")
	assert.Contains(t, got, "href=\"https://example.test\"")
}

// SiteName 空 → "Misskey" にフォールバック
func TestWrapHTML_SiteNameDefault(t *testing.T) {
	got := coreemail.WrapHTML(coreemail.HTMLWrapInput{
		Subject:  "Hi",
		BodyHTML: "x",
	})
	assert.NotContains(t, got, "<img ", "LogoURL 空ならロゴ画像は埋まらない")
	assert.Contains(t, got, ">Misskey<", "site name デフォルトは Misskey")
}

// Subject の HTML escape (XSS 防御)
func TestWrapHTML_SubjectEscaped(t *testing.T) {
	got := coreemail.WrapHTML(coreemail.HTMLWrapInput{
		Subject:  `<script>alert(1)</script>`,
		BodyHTML: "x",
	})
	assert.NotContains(t, got, "<script>alert(1)</script>")
	assert.Contains(t, got, "&lt;script&gt;alert(1)&lt;/script&gt;")
}

// SiteName / LogoURL も escape される
func TestWrapHTML_AttributesEscaped(t *testing.T) {
	got := coreemail.WrapHTML(coreemail.HTMLWrapInput{
		SiteName: `My"Site`,
		LogoURL:  `https://example.test/logo".png`,
		Subject:  "Hi",
		BodyHTML: "x",
	})
	assert.Contains(t, got, "&#34;Site")
	assert.Contains(t, got, "logo&#34;.png")
}

func TestLinkText(t *testing.T) {
	text, html := coreemail.LinkText("Welcome!", "Click here", "https://example.test/c?token=abc&x=1")
	assert.Equal(t, "Welcome!\nhttps://example.test/c?token=abc&x=1", text)
	// HTML 側は & が escape される
	assert.Contains(t, html, "https://example.test/c?token=abc&amp;x=1")
	assert.Contains(t, html, "Click here")
	assert.Contains(t, html, "<p>Welcome!</p>")
}
