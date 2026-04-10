CREATE TABLE IF NOT EXISTS "sw_subscription" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "endpoint" varchar(512) NOT NULL,
    "auth" varchar(256) NOT NULL,
    "publickey" varchar(128) NOT NULL,
    "sendReadMessage" boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS "IDX_sw_subscription_userId" ON "sw_subscription" ("userId");
