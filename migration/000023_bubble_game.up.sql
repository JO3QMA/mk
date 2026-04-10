CREATE TABLE IF NOT EXISTS "bubble_game_record" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "seededAt" timestamp with time zone NOT NULL,
    "seed" varchar(1024) NOT NULL,
    "gameVersion" integer NOT NULL,
    "gameMode" varchar(128) NOT NULL,
    "score" integer NOT NULL,
    "logs" jsonb NOT NULL DEFAULT '[]',
    "isVerified" boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS "IDX_bubble_game_record_userId" ON "bubble_game_record" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_bubble_game_record_seededAt" ON "bubble_game_record" ("seededAt");
CREATE INDEX IF NOT EXISTS "IDX_bubble_game_record_score" ON "bubble_game_record" ("score");
