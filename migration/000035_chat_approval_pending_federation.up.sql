-- chat_approval: 1対1チャット許可管理 (User.chatScope と組み合わせて使う)
CREATE TABLE IF NOT EXISTS "chat_approval" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "otherId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "IDX_chat_approval_userId" ON "chat_approval" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_chat_approval_otherId" ON "chat_approval" ("otherId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_chat_approval_userId_otherId" ON "chat_approval" ("userId", "otherId");

-- user_pending: 登録待機テーブル (招待制登録時のメール確認待ち)
CREATE TABLE IF NOT EXISTS "user_pending" (
    "id" varchar(32) PRIMARY KEY,
    "code" varchar(128) NOT NULL UNIQUE,
    "username" varchar(128) NOT NULL,
    "email" varchar(128) NOT NULL,
    "password" varchar(128) NOT NULL
);

-- chat_message: CherryPick連合用カラム追加
ALTER TABLE "chat_message" ADD COLUMN IF NOT EXISTS "emojis" varchar(128)[] NOT NULL DEFAULT '{}';
ALTER TABLE "chat_message" ADD COLUMN IF NOT EXISTS "isDelivering" boolean NOT NULL DEFAULT false;
ALTER TABLE "chat_message" ADD COLUMN IF NOT EXISTS "isDeliverFailed" boolean NOT NULL DEFAULT false;
