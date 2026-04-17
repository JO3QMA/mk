-- 欠損テーブル + カラム追加 (Issue #113)

-- used_username: ユーザー名再利用防止
CREATE TABLE IF NOT EXISTS "used_username" (
    "username" varchar(128) NOT NULL PRIMARY KEY,
    "createdAt" timestamp with time zone NOT NULL DEFAULT now()
);

-- channel_favorite: チャンネルお気に入り
CREATE TABLE IF NOT EXISTS "channel_favorite" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "channelId" varchar(32) NOT NULL REFERENCES "channel"("id") ON DELETE CASCADE,
    UNIQUE ("userId", "channelId")
);
CREATE INDEX IF NOT EXISTS "IDX_channel_favorite_userId" ON "channel_favorite" ("userId");

-- channel_muting: チャンネルミュート
CREATE TABLE IF NOT EXISTS "channel_muting" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "channelId" varchar(32) NOT NULL REFERENCES "channel"("id") ON DELETE CASCADE,
    UNIQUE ("userId", "channelId")
);
CREATE INDEX IF NOT EXISTS "IDX_channel_muting_userId" ON "channel_muting" ("userId");

-- clip_favorite: クリップお気に入り
CREATE TABLE IF NOT EXISTS "clip_favorite" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "clipId" varchar(32) NOT NULL REFERENCES "clip"("id") ON DELETE CASCADE,
    UNIQUE ("userId", "clipId")
);
CREATE INDEX IF NOT EXISTS "IDX_clip_favorite_userId" ON "clip_favorite" ("userId");

-- user_list_favorite: ユーザーリストお気に入り
CREATE TABLE IF NOT EXISTS "user_list_favorite" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "userListId" varchar(32) NOT NULL REFERENCES "user_list"("id") ON DELETE CASCADE,
    UNIQUE ("userId", "userListId")
);
CREATE INDEX IF NOT EXISTS "IDX_user_list_favorite_userId" ON "user_list_favorite" ("userId");

-- retention_aggregation: リテンション統計
CREATE TABLE IF NOT EXISTS "retention_aggregation" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "createdAt" timestamp with time zone NOT NULL DEFAULT now(),
    "updatedAt" timestamp with time zone NOT NULL DEFAULT now(),
    "userIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "data" jsonb NOT NULL DEFAULT '{}',
    "dateKey" varchar(64) NOT NULL UNIQUE,
    "usersCount" integer NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS "IDX_retention_aggregation_createdAt" ON "retention_aggregation" ("createdAt");

-- system_account: システムアカウント (instance.actor, relay 等)
CREATE TABLE IF NOT EXISTS "system_account" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "type" varchar(256) NOT NULL UNIQUE
);
CREATE INDEX IF NOT EXISTS "IDX_system_account_userId" ON "system_account" ("userId");

-- 欠損カラム追加
ALTER TABLE "ad" ADD COLUMN IF NOT EXISTS "isSensitive" boolean NOT NULL DEFAULT false;
ALTER TABLE "antenna" ADD COLUMN IF NOT EXISTS "excludeNotesInSensitiveChannel" boolean NOT NULL DEFAULT false;
ALTER TABLE "note_draft" ADD COLUMN IF NOT EXISTS "pollExpiredAfter" bigint;
ALTER TABLE "user_list_membership" ADD COLUMN IF NOT EXISTS "withReplies" boolean NOT NULL DEFAULT false;
ALTER TABLE "user_list_membership" ADD COLUMN IF NOT EXISTS "userListUserId" varchar(32);
