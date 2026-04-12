// Package urlpreview fetches and parses URL metadata (OGP, Twitter card)
// for link preview cards. Mirrors the original Misskey's summaly behavior.
package urlpreview

// Result is the URL preview response shape matching Misskey's SummalyResult.
type Result struct {
	Title       *string      `json:"title"`
	Description *string      `json:"description"`
	Thumbnail   *string      `json:"thumbnail"`
	Icon        *string      `json:"icon"`
	Sitename    *string      `json:"sitename"`
	URL         string       `json:"url"`
	Player      PlayerResult `json:"player"`
	Sensitive   bool         `json:"sensitive"`
	ActivityPub *string      `json:"activityPub"`
}

// PlayerResult holds embedded player (oEmbed) info.
type PlayerResult struct {
	URL    *string  `json:"url"`
	Width  *int     `json:"width"`
	Height *int     `json:"height"`
	Allow  []string `json:"allow"`
}
