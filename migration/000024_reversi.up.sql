CREATE TABLE IF NOT EXISTS "reversi_game" (
    "id" varchar(32) PRIMARY KEY,
    "startedAt" timestamp with time zone,
    "endedAt" timestamp with time zone,
    "user1Id" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "user2Id" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "user1Ready" boolean NOT NULL DEFAULT false,
    "user2Ready" boolean NOT NULL DEFAULT false,
    "black" integer,
    "isStarted" boolean NOT NULL DEFAULT false,
    "isEnded" boolean NOT NULL DEFAULT false,
    "winnerId" varchar(32),
    "surrenderedUserId" varchar(32),
    "timeoutUserId" varchar(32),
    "timeLimitForEachTurn" smallint NOT NULL DEFAULT 90,
    "logs" jsonb NOT NULL DEFAULT '[]',
    "map" varchar(64)[] NOT NULL DEFAULT '{}',
    "bw" varchar(32) NOT NULL DEFAULT '',
    "noIrregularRules" boolean NOT NULL DEFAULT false,
    "isLlotheo" boolean NOT NULL DEFAULT false,
    "canPutEverywhere" boolean NOT NULL DEFAULT false,
    "loopedBoard" boolean NOT NULL DEFAULT false,
    "form1" jsonb,
    "form2" jsonb,
    "crc32" varchar(32),
    "federationId" varchar(256)
);
CREATE INDEX IF NOT EXISTS "IDX_reversi_game_user1Id" ON "reversi_game" ("user1Id");
CREATE INDEX IF NOT EXISTS "IDX_reversi_game_user2Id" ON "reversi_game" ("user2Id");
