-- user_publickey: リモートユーザーの公開鍵を永続化するテーブル (TS版 1000000000000-Init.js 準拠)
CREATE TABLE IF NOT EXISTS "user_publickey" (
    "userId" varchar(32) NOT NULL PRIMARY KEY REFERENCES "user"("id") ON DELETE CASCADE,
    "keyId" varchar(256) NOT NULL,
    "keyPem" varchar(4096) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_user_publickey_keyId" ON "user_publickey" ("keyId");
