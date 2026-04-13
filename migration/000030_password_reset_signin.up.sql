-- password_reset_request テーブル (TS版: 1619942102890-password-reset.js 準拠)
CREATE TABLE IF NOT EXISTS "password_reset_request" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "token" varchar(256) NOT NULL,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_password_reset_request_token" ON "password_reset_request" ("token");
CREATE INDEX IF NOT EXISTS "IDX_password_reset_request_userId" ON "password_reset_request" ("userId");

-- signin テーブル
CREATE TABLE IF NOT EXISTS "signin" (
    "id" varchar(32) NOT NULL PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "ip" varchar(128) NOT NULL DEFAULT '',
    "headers" jsonb NOT NULL DEFAULT '{}',
    "success" boolean NOT NULL DEFAULT true
);
CREATE INDEX IF NOT EXISTS "IDX_signin_userId" ON "signin" ("userId");
