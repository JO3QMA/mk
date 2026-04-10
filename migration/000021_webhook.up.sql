CREATE TABLE IF NOT EXISTS "webhook" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "name" varchar(128) NOT NULL,
    "on" varchar(128)[] NOT NULL DEFAULT '{}',
    "url" varchar(1024) NOT NULL,
    "secret" varchar(1024) NOT NULL DEFAULT '',
    "active" boolean NOT NULL DEFAULT true,
    "latestSentAt" timestamp with time zone,
    "latestStatus" integer
);
CREATE INDEX IF NOT EXISTS "IDX_webhook_userId" ON "webhook" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_webhook_active" ON "webhook" ("active");
