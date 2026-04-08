-- page_visibility enum
CREATE TYPE page_visibility_enum AS ENUM ('public', 'followers', 'specified');

-- page: ユーザー作成のページ (Misskey 互換)
CREATE TABLE IF NOT EXISTS "page" (
    "id" varchar(32) PRIMARY KEY,
    "updatedAt" timestamp with time zone NOT NULL DEFAULT now(),
    "title" varchar(256) NOT NULL,
    "name" varchar(256) NOT NULL,
    "summary" varchar(256),
    "alignCenter" boolean NOT NULL DEFAULT false,
    "hideTitleWhenPinned" boolean NOT NULL DEFAULT false,
    "font" varchar(32) NOT NULL DEFAULT 'sans-serif',
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "eyeCatchingImageId" varchar(32),
    "content" jsonb NOT NULL DEFAULT '[]',
    "variables" jsonb NOT NULL DEFAULT '[]',
    "script" varchar(16384) NOT NULL DEFAULT '',
    "visibility" page_visibility_enum NOT NULL DEFAULT 'public',
    "visibleUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "likedCount" integer NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS "IDX_page_userId" ON "page" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_page_name" ON "page" ("name");
CREATE INDEX IF NOT EXISTS "IDX_page_updatedAt" ON "page" ("updatedAt");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_page_userId_name" ON "page" ("userId", "name");

-- page_like: ページへのいいね中間表
CREATE TABLE IF NOT EXISTS "page_like" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "pageId" varchar(32) NOT NULL REFERENCES "page"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_page_like_userId" ON "page_like" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_page_like_pageId" ON "page_like" ("pageId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_page_like_pair" ON "page_like" ("userId", "pageId");
