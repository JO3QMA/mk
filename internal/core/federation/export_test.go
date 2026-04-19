package federation

import "github.com/shiroha-a/mk/internal/activitypub"

// ExtractEmojiTags exposes the unexported extractEmojiTags for external tests.
var ExtractEmojiTags = extractEmojiTags

// UpsertEmojis exposes the unexported upsertEmojis method for external tests.
func (r *Resolver) UpsertEmojis(tags []activitypub.EmojiTag, host string) []string {
	return r.upsertEmojis(tags, host)
}
