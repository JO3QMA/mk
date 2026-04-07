-- follow_request
CREATE TABLE IF NOT EXISTS "follow_request" (
    "id" varchar(32) PRIMARY KEY,
    "followeeId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "followerId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "requestId" varchar(128),
    "withReplies" boolean NOT NULL DEFAULT false,
    "followerHost" varchar(128),
    "followerInbox" varchar(512),
    "followerSharedInbox" varchar(512),
    "followeeHost" varchar(128),
    "followeeInbox" varchar(512),
    "followeeSharedInbox" varchar(512)
);

CREATE INDEX IF NOT EXISTS "IDX_follow_request_followeeId" ON "follow_request" ("followeeId");
CREATE INDEX IF NOT EXISTS "IDX_follow_request_followerId" ON "follow_request" ("followerId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_follow_request_followerId_followeeId" ON "follow_request" ("followerId", "followeeId");
