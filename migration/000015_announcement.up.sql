-- announcement: サーバーお知らせ (Misskey 互換)
CREATE TABLE IF NOT EXISTS "announcement" (
    "id" varchar(32) PRIMARY KEY,
    "updatedAt" timestamp with time zone,
    "title" varchar(256) NOT NULL,
    "text" varchar(8192) NOT NULL,
    "imageUrl" varchar(1024),
    "icon" varchar(256) NOT NULL DEFAULT 'info',
    "display" varchar(256) NOT NULL DEFAULT 'normal',
    "needConfirmationToRead" boolean NOT NULL DEFAULT false,
    "isActive" boolean NOT NULL DEFAULT true,
    "forExistingUsers" boolean NOT NULL DEFAULT false,
    "silence" boolean NOT NULL DEFAULT false,
    "userId" varchar(32) REFERENCES "user"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_announcement_isActive" ON "announcement" ("isActive");

-- announcement_read: ユーザーごとの既読管理
CREATE TABLE IF NOT EXISTS "announcement_read" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "announcementId" varchar(32) NOT NULL REFERENCES "announcement"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_announcement_read_userId" ON "announcement_read" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_announcement_read_announcementId" ON "announcement_read" ("announcementId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_announcement_read_pair" ON "announcement_read" ("userId", "announcementId");
