-- channel: ユーザー作成のトピックスペース (Misskey 互換)
CREATE TABLE IF NOT EXISTS "channel" (
    "id" varchar(32) PRIMARY KEY,
    "lastNotedAt" timestamp with time zone,
    "userId" varchar(32) REFERENCES "user"("id") ON DELETE SET NULL,
    "name" varchar(128) NOT NULL,
    "description" varchar(2048),
    "bannerId" varchar(32),
    "pinnedNoteIds" varchar(128)[] NOT NULL DEFAULT '{}',
    "color" varchar(16) NOT NULL DEFAULT '#86b300',
    "isArchived" boolean NOT NULL DEFAULT false,
    "notesCount" integer NOT NULL DEFAULT 0,
    "usersCount" integer NOT NULL DEFAULT 0,
    "isSensitive" boolean NOT NULL DEFAULT false,
    "allowRenoteToExternal" boolean NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS "IDX_channel_lastNotedAt" ON "channel" ("lastNotedAt");
CREATE INDEX IF NOT EXISTS "IDX_channel_userId" ON "channel" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_channel_isArchived" ON "channel" ("isArchived");
CREATE INDEX IF NOT EXISTS "IDX_channel_notesCount" ON "channel" ("notesCount");
CREATE INDEX IF NOT EXISTS "IDX_channel_usersCount" ON "channel" ("usersCount");

-- channel_following: チャンネルをフォローしているユーザーの関係表
CREATE TABLE IF NOT EXISTS "channel_following" (
    "id" varchar(32) PRIMARY KEY,
    "followeeId" varchar(32) NOT NULL REFERENCES "channel"("id") ON DELETE CASCADE,
    "followerId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_channel_following_followeeId" ON "channel_following" ("followeeId");
CREATE INDEX IF NOT EXISTS "IDX_channel_following_followerId" ON "channel_following" ("followerId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_channel_following_pair" ON "channel_following" ("followerId", "followeeId");
