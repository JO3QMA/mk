CREATE TABLE IF NOT EXISTS "chat_room" (
    "id" varchar(32) PRIMARY KEY,
    "name" varchar(256) NOT NULL,
    "ownerId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "description" varchar(2048) NOT NULL DEFAULT '',
    "isArchived" boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS "IDX_chat_room_ownerId" ON "chat_room" ("ownerId");

CREATE TABLE IF NOT EXISTS "chat_message" (
    "id" varchar(32) PRIMARY KEY,
    "fromUserId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "toUserId" varchar(32) REFERENCES "user"("id") ON DELETE CASCADE,
    "toRoomId" varchar(32) REFERENCES "chat_room"("id") ON DELETE CASCADE,
    "text" varchar(4096),
    "uri" varchar(512),
    "reads" varchar(32)[] NOT NULL DEFAULT '{}',
    "fileId" varchar(32),
    "reactions" varchar(1024)[] NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS "IDX_chat_message_fromUserId" ON "chat_message" ("fromUserId");
CREATE INDEX IF NOT EXISTS "IDX_chat_message_toUserId" ON "chat_message" ("toUserId");
CREATE INDEX IF NOT EXISTS "IDX_chat_message_toRoomId" ON "chat_message" ("toRoomId");

CREATE TABLE IF NOT EXISTS "chat_room_membership" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "roomId" varchar(32) NOT NULL REFERENCES "chat_room"("id") ON DELETE CASCADE,
    "isMuted" boolean NOT NULL DEFAULT false,
    UNIQUE ("userId", "roomId")
);
CREATE INDEX IF NOT EXISTS "IDX_chat_room_membership_userId" ON "chat_room_membership" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_chat_room_membership_roomId" ON "chat_room_membership" ("roomId");

CREATE TABLE IF NOT EXISTS "chat_room_invitation" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "roomId" varchar(32) NOT NULL REFERENCES "chat_room"("id") ON DELETE CASCADE,
    "ignored" boolean NOT NULL DEFAULT false,
    UNIQUE ("userId", "roomId")
);
CREATE INDEX IF NOT EXISTS "IDX_chat_room_invitation_userId" ON "chat_room_invitation" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_chat_room_invitation_roomId" ON "chat_room_invitation" ("roomId");
