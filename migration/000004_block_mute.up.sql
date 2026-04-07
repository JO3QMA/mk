-- blocking
CREATE TABLE IF NOT EXISTS "blocking" (
    "id" varchar(32) PRIMARY KEY,
    "blockerId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "blockeeId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_blocking_blockerId" ON "blocking" ("blockerId");
CREATE INDEX IF NOT EXISTS "IDX_blocking_blockeeId" ON "blocking" ("blockeeId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_blocking_blockerId_blockeeId" ON "blocking" ("blockerId", "blockeeId");

-- muting
CREATE TABLE IF NOT EXISTS "muting" (
    "id" varchar(32) PRIMARY KEY,
    "muterId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "muteeId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "expiresAt" timestamp with time zone
);

CREATE INDEX IF NOT EXISTS "IDX_muting_muterId" ON "muting" ("muterId");
CREATE INDEX IF NOT EXISTS "IDX_muting_muteeId" ON "muting" ("muteeId");
CREATE INDEX IF NOT EXISTS "IDX_muting_expiresAt" ON "muting" ("expiresAt");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_muting_muterId_muteeId" ON "muting" ("muterId", "muteeId");

-- renote_muting
CREATE TABLE IF NOT EXISTS "renote_muting" (
    "id" varchar(32) PRIMARY KEY,
    "muterId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "muteeId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_renote_muting_muterId" ON "renote_muting" ("muterId");
CREATE INDEX IF NOT EXISTS "IDX_renote_muting_muteeId" ON "renote_muting" ("muteeId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_renote_muting_muterId_muteeId" ON "renote_muting" ("muterId", "muteeId");
