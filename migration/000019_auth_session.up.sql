CREATE TABLE IF NOT EXISTS "app" (
    "id" varchar(32) PRIMARY KEY,
    "createdAt" timestamp with time zone NOT NULL DEFAULT now(),
    "userId" varchar(32) REFERENCES "user"("id") ON DELETE SET NULL,
    "secret" varchar(64) NOT NULL,
    "name" varchar(128) NOT NULL,
    "description" varchar(512) NOT NULL DEFAULT '',
    "permission" varchar(64)[] NOT NULL DEFAULT '{}',
    "callbackUrl" varchar(512)
);
CREATE INDEX IF NOT EXISTS "IDX_app_secret" ON "app" ("secret");
CREATE INDEX IF NOT EXISTS "IDX_app_userId" ON "app" ("userId");

CREATE TABLE IF NOT EXISTS "auth_session" (
    "id" varchar(32) PRIMARY KEY,
    "createdAt" timestamp with time zone NOT NULL DEFAULT now(),
    "token" varchar(128) NOT NULL,
    "userId" varchar(32) REFERENCES "user"("id") ON DELETE CASCADE,
    "appId" varchar(32) NOT NULL REFERENCES "app"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "IDX_auth_session_token" ON "auth_session" ("token");
