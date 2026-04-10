CREATE TABLE IF NOT EXISTS "avatar_decoration" (
    "id" varchar(32) PRIMARY KEY,
    "updatedAt" timestamp with time zone,
    "url" varchar(1024) NOT NULL,
    "name" varchar(256) NOT NULL,
    "description" varchar(2048) NOT NULL DEFAULT '',
    "roleIdsThatCanBeUsedThisDecoration" varchar(128)[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS "system_webhook" (
    "id" varchar(32) PRIMARY KEY,
    "isActive" boolean NOT NULL DEFAULT true,
    "updatedAt" timestamp with time zone NOT NULL DEFAULT now(),
    "latestSentAt" timestamp with time zone,
    "latestStatus" integer,
    "name" varchar(255) NOT NULL,
    "on" varchar(128)[] NOT NULL DEFAULT '{}',
    "url" varchar(1024) NOT NULL,
    "secret" varchar(1024) NOT NULL DEFAULT ''
);
