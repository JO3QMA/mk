ALTER TABLE "user_list_membership" DROP COLUMN IF EXISTS "userListUserId";
ALTER TABLE "user_list_membership" DROP COLUMN IF EXISTS "withReplies";
ALTER TABLE "note_draft" DROP COLUMN IF EXISTS "pollExpiredAfter";
ALTER TABLE "antenna" DROP COLUMN IF EXISTS "excludeNotesInSensitiveChannel";
ALTER TABLE "ad" DROP COLUMN IF EXISTS "isSensitive";

DROP TABLE IF EXISTS "system_account";
DROP TABLE IF EXISTS "retention_aggregation";
DROP TABLE IF EXISTS "user_list_favorite";
DROP TABLE IF EXISTS "clip_favorite";
DROP TABLE IF EXISTS "channel_muting";
DROP TABLE IF EXISTS "channel_favorite";
DROP TABLE IF EXISTS "used_username";
