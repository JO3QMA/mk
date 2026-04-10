CREATE TABLE IF NOT EXISTS "note_draft" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "text" text,
    "cw" varchar(512),
    "visibility" varchar(32) NOT NULL DEFAULT 'public',
    "localOnly" boolean NOT NULL DEFAULT false,
    "reactionAcceptance" varchar(64),
    "fileIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "visibleUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "replyId" varchar(32),
    "renoteId" varchar(32),
    "channelId" varchar(32),
    "hashtag" varchar(128),
    "hasPoll" boolean NOT NULL DEFAULT false,
    "pollChoices" varchar(256)[] NOT NULL DEFAULT '{}',
    "pollMultiple" boolean NOT NULL DEFAULT false,
    "pollExpiresAt" timestamp with time zone,
    "scheduledAt" timestamp with time zone,
    "isActuallyScheduled" boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS "IDX_note_draft_userId" ON "note_draft" ("userId");

CREATE TABLE IF NOT EXISTS "note_thread_muting" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "threadId" varchar(256) NOT NULL,
    UNIQUE ("userId", "threadId")
);
CREATE INDEX IF NOT EXISTS "IDX_note_thread_muting_userId" ON "note_thread_muting" ("userId");
