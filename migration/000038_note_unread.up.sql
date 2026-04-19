-- note_unread: per-user per-note unread tracking for specified-visibility
-- and mention notes. Matches the TS schema's note_unread table so
-- /api/i can surface hasUnreadSpecifiedNotes / hasUnreadMentions without
-- scanning the Redis notification stream.
CREATE TABLE IF NOT EXISTS "note_unread" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "noteId" varchar(32) NOT NULL REFERENCES "note"("id") ON DELETE CASCADE,
    "noteUserId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "isSpecified" boolean NOT NULL DEFAULT FALSE,
    "isMentioned" boolean NOT NULL DEFAULT FALSE
);

CREATE UNIQUE INDEX IF NOT EXISTS "UQ_note_unread_userId_noteId"
    ON "note_unread" ("userId", "noteId");
CREATE INDEX IF NOT EXISTS "IDX_note_unread_userId"
    ON "note_unread" ("userId");
-- 本家同様に partial index で hasUnreadSpecifiedNotes / hasUnreadMentions
-- 判定を効率化する。
CREATE INDEX IF NOT EXISTS "IDX_note_unread_userId_isSpecified"
    ON "note_unread" ("userId") WHERE "isSpecified" = true;
CREATE INDEX IF NOT EXISTS "IDX_note_unread_userId_isMentioned"
    ON "note_unread" ("userId") WHERE "isMentioned" = true;
