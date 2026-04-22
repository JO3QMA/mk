package federation

import (
	"github.com/shiroha-a/mk/internal/activitypub"
	corenote "github.com/shiroha-a/mk/internal/core/note"
)

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

// ExtractMentionTags exposes the unexported extractMentionTags for external tests (#397).
var ExtractMentionTags = extractMentionTags

// MergeMentionIDs exposes the unexported mergeMentionIDs for external tests (#397).
var MergeMentionIDs = mergeMentionIDs

// ResolveMentionedUserIDs exposes the unexported resolveMentionedUserIDs for external tests.
func (r *Resolver) ResolveMentionedUserIDs(hrefs []string) []string {
	return r.resolveMentionedUserIDs(hrefs)
}

// ResolveTextMentionUserIDs exposes the unexported resolveTextMentionUserIDs for external tests.
func (r *Resolver) ResolveTextMentionUserIDs(mentions []corenote.Mention) []string {
	return r.resolveTextMentionUserIDs(mentions)
}
