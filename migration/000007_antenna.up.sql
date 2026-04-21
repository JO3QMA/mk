-- antenna_src enum: フィードのソースを表す。
-- TS Misskey が同名 enum を既に作っているケース (drop-in 切替 #367) でも
-- migrate up が失敗しないよう duplicate_object を無視する。
DO $$ BEGIN
    CREATE TYPE antenna_src_enum AS ENUM ('home', 'all', 'users', 'list', 'users_blacklist');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- antenna: ユーザー作成のキーワード/ユーザベースカスタムフィード (Misskey 互換)
CREATE TABLE IF NOT EXISTS "antenna" (
    "id" varchar(32) PRIMARY KEY,
    "lastUsedAt" timestamp with time zone NOT NULL DEFAULT now(),
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "name" varchar(128) NOT NULL,
    "src" antenna_src_enum NOT NULL,
    "userListId" varchar(32),
    "users" varchar(1024)[] NOT NULL DEFAULT '{}',
    "keywords" jsonb NOT NULL DEFAULT '[]',
    "excludeKeywords" jsonb NOT NULL DEFAULT '[]',
    "caseSensitive" boolean NOT NULL DEFAULT false,
    "excludeBots" boolean NOT NULL DEFAULT false,
    "withReplies" boolean NOT NULL DEFAULT false,
    "withFile" boolean NOT NULL DEFAULT false,
    "expression" varchar(2048),
    "isActive" boolean NOT NULL DEFAULT true,
    "localOnly" boolean NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS "IDX_antenna_userId" ON "antenna" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_antenna_lastUsedAt" ON "antenna" ("lastUsedAt");
CREATE INDEX IF NOT EXISTS "IDX_antenna_isActive" ON "antenna" ("isActive");
