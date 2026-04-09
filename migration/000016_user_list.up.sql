-- user_list: ユーザーリスト (Misskey 互換)
CREATE TABLE IF NOT EXISTS "user_list" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "name" varchar(128) NOT NULL,
    "isPublic" boolean NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS "IDX_user_list_userId" ON "user_list" ("userId");

-- user_list_membership: リストメンバー
CREATE TABLE IF NOT EXISTS "user_list_membership" (
    "id" varchar(32) PRIMARY KEY,
    "userListId" varchar(32) NOT NULL REFERENCES "user_list"("id") ON DELETE CASCADE,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_user_list_membership_userListId" ON "user_list_membership" ("userListId");
CREATE INDEX IF NOT EXISTS "IDX_user_list_membership_userId" ON "user_list_membership" ("userId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_user_list_membership_pair" ON "user_list_membership" ("userListId", "userId");
