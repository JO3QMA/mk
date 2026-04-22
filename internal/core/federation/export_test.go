package federation

import "github.com/shiroha-a/mk/internal/activitypub"

// ExtractEmojiTags exposes the unexported extractEmojiTags for external tests.
var ExtractEmojiTags = extractEmojiTags

// UpsertEmojis exposes the unexported upsertEmojis method for external tests.
func (r *Resolver) UpsertEmojis(tags []activitypub.EmojiTag, host string) []string {
	return r.upsertEmojis(tags, host)
}

// ExtractAttachments exposes the unexported extractAttachments for external tests (#378).
var ExtractAttachments = extractAttachments

// UpsertAttachments exposes the unexported upsertAttachments method for external tests.
func (r *Resolver) UpsertAttachments(docs []activitypub.Document, userID, host *string) []string {
	return r.upsertAttachments(docs, userID, host)
}

// CollectAttachedFileTypes exposes the unexported collectAttachedFileTypes for external tests.
func (r *Resolver) CollectAttachedFileTypes(fileIDs []string) []string {
	return r.collectAttachedFileTypes(fileIDs)
}
