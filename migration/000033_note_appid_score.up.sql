-- Add appId (source app) and score (internal ranking value) columns to note table.
ALTER TABLE "note" ADD COLUMN IF NOT EXISTS "appId" varchar(32);
ALTER TABLE "note" ADD COLUMN IF NOT EXISTS "score" integer NOT NULL DEFAULT 0;
