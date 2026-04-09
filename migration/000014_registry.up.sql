-- registry_item: ユーザー設定の永続化 (Misskey 互換)
CREATE TABLE IF NOT EXISTS "registry_item" (
    "id" varchar(32) PRIMARY KEY,
    "updatedAt" timestamp with time zone NOT NULL DEFAULT now(),
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "key" varchar(1024) NOT NULL,
    "value" jsonb DEFAULT '{}',
    "scope" varchar(1024)[] NOT NULL DEFAULT '{}',
    "domain" varchar(512)
);

CREATE INDEX IF NOT EXISTS "IDX_registry_item_userId" ON "registry_item" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_registry_item_scope" ON "registry_item" ("scope");
CREATE INDEX IF NOT EXISTS "IDX_registry_item_domain" ON "registry_item" ("domain");
