-- flash: AI Script ベースのインタラクティブコンテンツ (Misskey Play 互換)
CREATE TABLE IF NOT EXISTS "flash" (
    "id" varchar(32) PRIMARY KEY,
    "updatedAt" timestamp with time zone NOT NULL DEFAULT now(),
    "title" varchar(256) NOT NULL,
    "summary" varchar(1024) NOT NULL DEFAULT '',
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "script" varchar(65536) NOT NULL,
    "permissions" varchar(64)[] NOT NULL DEFAULT '{}',
    "likedCount" integer NOT NULL DEFAULT 0,
    "visibility" varchar(32) NOT NULL DEFAULT 'public'
);

CREATE INDEX IF NOT EXISTS "IDX_flash_userId" ON "flash" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_flash_updatedAt" ON "flash" ("updatedAt");
CREATE INDEX IF NOT EXISTS "IDX_flash_likedCount" ON "flash" ("likedCount");

-- flash_like: Flash へのいいね中間表
CREATE TABLE IF NOT EXISTS "flash_like" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "flashId" varchar(32) NOT NULL REFERENCES "flash"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_flash_like_userId" ON "flash_like" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_flash_like_flashId" ON "flash_like" ("flashId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_flash_like_pair" ON "flash_like" ("userId", "flashId");
