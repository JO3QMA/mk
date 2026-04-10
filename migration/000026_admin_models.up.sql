CREATE TABLE IF NOT EXISTS "ad" (
    "id" varchar(32) PRIMARY KEY,
    "expiresAt" timestamp with time zone NOT NULL,
    "startsAt" timestamp with time zone NOT NULL DEFAULT now(),
    "place" varchar(32) NOT NULL,
    "priority" varchar(32) NOT NULL DEFAULT 'middle',
    "ratio" integer NOT NULL DEFAULT 1,
    "url" varchar(1024) NOT NULL,
    "imageUrl" varchar(1024) NOT NULL,
    "memo" varchar(8192) NOT NULL DEFAULT '',
    "dayOfWeek" integer NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS "registration_ticket" (
    "id" varchar(32) PRIMARY KEY,
    "code" varchar(64) NOT NULL UNIQUE,
    "expiresAt" timestamp with time zone,
    "createdById" varchar(32) REFERENCES "user"("id") ON DELETE CASCADE,
    "usedById" varchar(32) REFERENCES "user"("id") ON DELETE CASCADE,
    "usedAt" timestamp with time zone,
    "pendingUserId" varchar(32) REFERENCES "user"("id") ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS "relay" (
    "id" varchar(32) PRIMARY KEY,
    "inbox" varchar(512) NOT NULL UNIQUE,
    "status" varchar(32) NOT NULL DEFAULT 'requesting'
);
