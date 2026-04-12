package urlpreview

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

// ParseHTML extracts OGP and Twitter card metadata from HTML content.
// 本家 Misskey の summaly と同じ優先順位: og:* > twitter:* > <title>/<link>。
func ParseHTML(r io.Reader, pageURL string) *Result {
	doc, err := html.Parse(r)
	if err != nil {
		return &Result{URL: pageURL, Player: PlayerResult{Allow: []string{}}}
	}

	meta := extractMeta(doc)

	title := firstNonEmpty(meta["og:title"], meta["twitter:title"], meta["title"])
	desc := firstNonEmpty(meta["og:description"], meta["twitter:description"], meta["description"])
	thumb := firstNonEmpty(meta["og:image"], meta["twitter:image"], meta["twitter:image:src"])
	icon := meta["icon"]
	sitename := firstNonEmpty(meta["og:site_name"])
	apID := firstNonEmpty(meta["ap:id"])

	result := &Result{
		URL:    firstNonEmpty(meta["og:url"], pageURL),
		Player: PlayerResult{Allow: []string{}},
	}
	if title != "" {
		result.Title = &title
	}
	if desc != "" {
		result.Description = &desc
	}
	if thumb != "" {
		result.Thumbnail = &thumb
	}
	if icon != "" {
		result.Icon = &icon
	}
	if sitename != "" {
		result.Sitename = &sitename
	}
	if apID != "" {
		result.ActivityPub = &apID
	}
	return result
}

type metaMap map[string]string

func extractMeta(n *html.Node) metaMap {
	m := make(metaMap)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				key, val := metaKeyVal(n)
				if key != "" && val != "" {
					m[key] = val
				}
			case "title":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					m["title"] = strings.TrimSpace(n.FirstChild.Data)
				}
			case "link":
				rel, href := attrVal(n, "rel"), attrVal(n, "href")
				if (rel == "icon" || rel == "shortcut icon") && href != "" {
					m["icon"] = href
				}
				// alternate + application/activity+json → AP ID
				if rel == "alternate" && strings.Contains(attrVal(n, "type"), "activity+json") && href != "" {
					m["ap:id"] = href
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return m
}

func metaKeyVal(n *html.Node) (string, string) {
	name := attrVal(n, "property")
	if name == "" {
		name = attrVal(n, "name")
	}
	content := attrVal(n, "content")
	return strings.ToLower(name), content
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
