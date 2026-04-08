-- Phase 4 Step Q (4.7) — Charts
-- 12 chart kinds × {hour, day} = 24 tables. Column naming follows the
-- Misskey TypeScript convention: dot-notation keys are flattened with
-- a triple-underscore prefix and a single-underscore delimiter, and
-- uniqueIncrement keys have a parallel `unique_temp___<name>` varchar[]
-- column for the in-progress set. The day tables mirror the hour tables
-- column-for-column but get their own SERIAL sequence and indexes.

----------------------------------------------------------------------
-- federation (ungrouped)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__federation" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "unique_temp___deliveredInstances" varchar[] NOT NULL DEFAULT '{}',
    "___deliveredInstances" smallint NOT NULL DEFAULT 0,
    "unique_temp___inboxInstances" varchar[] NOT NULL DEFAULT '{}',
    "___inboxInstances" smallint NOT NULL DEFAULT 0,
    "unique_temp___stalled" varchar[] NOT NULL DEFAULT '{}',
    "___stalled" smallint NOT NULL DEFAULT 0,
    "___sub" smallint NOT NULL DEFAULT 0,
    "___pub" smallint NOT NULL DEFAULT 0,
    "___pubsub" smallint NOT NULL DEFAULT 0,
    "___subActive" smallint NOT NULL DEFAULT 0,
    "___pubActive" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__federation_date" ON "__chart__federation" ("date");

CREATE TABLE IF NOT EXISTS "__chart_day__federation" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "unique_temp___deliveredInstances" varchar[] NOT NULL DEFAULT '{}',
    "___deliveredInstances" smallint NOT NULL DEFAULT 0,
    "unique_temp___inboxInstances" varchar[] NOT NULL DEFAULT '{}',
    "___inboxInstances" smallint NOT NULL DEFAULT 0,
    "unique_temp___stalled" varchar[] NOT NULL DEFAULT '{}',
    "___stalled" smallint NOT NULL DEFAULT 0,
    "___sub" smallint NOT NULL DEFAULT 0,
    "___pub" smallint NOT NULL DEFAULT 0,
    "___pubsub" smallint NOT NULL DEFAULT 0,
    "___subActive" smallint NOT NULL DEFAULT 0,
    "___pubActive" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__federation_date" ON "__chart_day__federation" ("date");

----------------------------------------------------------------------
-- notes (ungrouped)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__notes" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___local_total" integer NOT NULL DEFAULT 0,
    "___local_inc" integer NOT NULL DEFAULT 0,
    "___local_dec" integer NOT NULL DEFAULT 0,
    "___local_diffs_normal" integer NOT NULL DEFAULT 0,
    "___local_diffs_reply" integer NOT NULL DEFAULT 0,
    "___local_diffs_renote" integer NOT NULL DEFAULT 0,
    "___local_diffs_withFile" integer NOT NULL DEFAULT 0,
    "___remote_total" integer NOT NULL DEFAULT 0,
    "___remote_inc" integer NOT NULL DEFAULT 0,
    "___remote_dec" integer NOT NULL DEFAULT 0,
    "___remote_diffs_normal" integer NOT NULL DEFAULT 0,
    "___remote_diffs_reply" integer NOT NULL DEFAULT 0,
    "___remote_diffs_renote" integer NOT NULL DEFAULT 0,
    "___remote_diffs_withFile" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__notes_date" ON "__chart__notes" ("date");

CREATE TABLE IF NOT EXISTS "__chart_day__notes" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___local_total" integer NOT NULL DEFAULT 0,
    "___local_inc" integer NOT NULL DEFAULT 0,
    "___local_dec" integer NOT NULL DEFAULT 0,
    "___local_diffs_normal" integer NOT NULL DEFAULT 0,
    "___local_diffs_reply" integer NOT NULL DEFAULT 0,
    "___local_diffs_renote" integer NOT NULL DEFAULT 0,
    "___local_diffs_withFile" integer NOT NULL DEFAULT 0,
    "___remote_total" integer NOT NULL DEFAULT 0,
    "___remote_inc" integer NOT NULL DEFAULT 0,
    "___remote_dec" integer NOT NULL DEFAULT 0,
    "___remote_diffs_normal" integer NOT NULL DEFAULT 0,
    "___remote_diffs_reply" integer NOT NULL DEFAULT 0,
    "___remote_diffs_renote" integer NOT NULL DEFAULT 0,
    "___remote_diffs_withFile" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__notes_date" ON "__chart_day__notes" ("date");

----------------------------------------------------------------------
-- users (ungrouped)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__users" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___local_total" integer NOT NULL DEFAULT 0,
    "___local_inc" smallint NOT NULL DEFAULT 0,
    "___local_dec" smallint NOT NULL DEFAULT 0,
    "___remote_total" integer NOT NULL DEFAULT 0,
    "___remote_inc" smallint NOT NULL DEFAULT 0,
    "___remote_dec" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__users_date" ON "__chart__users" ("date");

CREATE TABLE IF NOT EXISTS "__chart_day__users" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___local_total" integer NOT NULL DEFAULT 0,
    "___local_inc" smallint NOT NULL DEFAULT 0,
    "___local_dec" smallint NOT NULL DEFAULT 0,
    "___remote_total" integer NOT NULL DEFAULT 0,
    "___remote_inc" smallint NOT NULL DEFAULT 0,
    "___remote_dec" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__users_date" ON "__chart_day__users" ("date");

----------------------------------------------------------------------
-- drive (ungrouped) — counters and kilobyte sizes
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__drive" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___local_incCount" integer NOT NULL DEFAULT 0,
    "___local_incSize" integer NOT NULL DEFAULT 0,
    "___local_decCount" integer NOT NULL DEFAULT 0,
    "___local_decSize" integer NOT NULL DEFAULT 0,
    "___remote_incCount" integer NOT NULL DEFAULT 0,
    "___remote_incSize" integer NOT NULL DEFAULT 0,
    "___remote_decCount" integer NOT NULL DEFAULT 0,
    "___remote_decSize" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__drive_date" ON "__chart__drive" ("date");

CREATE TABLE IF NOT EXISTS "__chart_day__drive" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___local_incCount" integer NOT NULL DEFAULT 0,
    "___local_incSize" integer NOT NULL DEFAULT 0,
    "___local_decCount" integer NOT NULL DEFAULT 0,
    "___local_decSize" integer NOT NULL DEFAULT 0,
    "___remote_incCount" integer NOT NULL DEFAULT 0,
    "___remote_incSize" integer NOT NULL DEFAULT 0,
    "___remote_decCount" integer NOT NULL DEFAULT 0,
    "___remote_decSize" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__drive_date" ON "__chart_day__drive" ("date");

----------------------------------------------------------------------
-- instance (grouped by host)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__instance" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___requests_failed" smallint NOT NULL DEFAULT 0,
    "___requests_succeeded" smallint NOT NULL DEFAULT 0,
    "___requests_received" smallint NOT NULL DEFAULT 0,
    "___notes_total" integer NOT NULL DEFAULT 0,
    "___notes_inc" integer NOT NULL DEFAULT 0,
    "___notes_dec" integer NOT NULL DEFAULT 0,
    "___notes_diffs_normal" integer NOT NULL DEFAULT 0,
    "___notes_diffs_reply" integer NOT NULL DEFAULT 0,
    "___notes_diffs_renote" integer NOT NULL DEFAULT 0,
    "___notes_diffs_withFile" integer NOT NULL DEFAULT 0,
    "___users_total" integer NOT NULL DEFAULT 0,
    "___users_inc" smallint NOT NULL DEFAULT 0,
    "___users_dec" smallint NOT NULL DEFAULT 0,
    "___following_total" integer NOT NULL DEFAULT 0,
    "___following_inc" smallint NOT NULL DEFAULT 0,
    "___following_dec" smallint NOT NULL DEFAULT 0,
    "___followers_total" integer NOT NULL DEFAULT 0,
    "___followers_inc" smallint NOT NULL DEFAULT 0,
    "___followers_dec" smallint NOT NULL DEFAULT 0,
    "___drive_totalFiles" integer NOT NULL DEFAULT 0,
    "___drive_incFiles" integer NOT NULL DEFAULT 0,
    "___drive_decFiles" integer NOT NULL DEFAULT 0,
    "___drive_incUsage" integer NOT NULL DEFAULT 0,
    "___drive_decUsage" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__instance_date_group" ON "__chart__instance" ("date", "group");

CREATE TABLE IF NOT EXISTS "__chart_day__instance" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___requests_failed" smallint NOT NULL DEFAULT 0,
    "___requests_succeeded" smallint NOT NULL DEFAULT 0,
    "___requests_received" smallint NOT NULL DEFAULT 0,
    "___notes_total" integer NOT NULL DEFAULT 0,
    "___notes_inc" integer NOT NULL DEFAULT 0,
    "___notes_dec" integer NOT NULL DEFAULT 0,
    "___notes_diffs_normal" integer NOT NULL DEFAULT 0,
    "___notes_diffs_reply" integer NOT NULL DEFAULT 0,
    "___notes_diffs_renote" integer NOT NULL DEFAULT 0,
    "___notes_diffs_withFile" integer NOT NULL DEFAULT 0,
    "___users_total" integer NOT NULL DEFAULT 0,
    "___users_inc" smallint NOT NULL DEFAULT 0,
    "___users_dec" smallint NOT NULL DEFAULT 0,
    "___following_total" integer NOT NULL DEFAULT 0,
    "___following_inc" smallint NOT NULL DEFAULT 0,
    "___following_dec" smallint NOT NULL DEFAULT 0,
    "___followers_total" integer NOT NULL DEFAULT 0,
    "___followers_inc" smallint NOT NULL DEFAULT 0,
    "___followers_dec" smallint NOT NULL DEFAULT 0,
    "___drive_totalFiles" integer NOT NULL DEFAULT 0,
    "___drive_incFiles" integer NOT NULL DEFAULT 0,
    "___drive_decFiles" integer NOT NULL DEFAULT 0,
    "___drive_incUsage" integer NOT NULL DEFAULT 0,
    "___drive_decUsage" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__instance_date_group" ON "__chart_day__instance" ("date", "group");

----------------------------------------------------------------------
-- apRequest (ungrouped) → table __chart__ap_request
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__ap_request" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___deliverFailed" integer NOT NULL DEFAULT 0,
    "___deliverSucceeded" integer NOT NULL DEFAULT 0,
    "___inboxReceived" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__ap_request_date" ON "__chart__ap_request" ("date");

CREATE TABLE IF NOT EXISTS "__chart_day__ap_request" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___deliverFailed" integer NOT NULL DEFAULT 0,
    "___deliverSucceeded" integer NOT NULL DEFAULT 0,
    "___inboxReceived" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__ap_request_date" ON "__chart_day__ap_request" ("date");

----------------------------------------------------------------------
-- activeUsers (ungrouped) — uniqueIncrement / intersection heavy
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__active_users" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___readWrite" integer NOT NULL DEFAULT 0,
    "unique_temp___read" varchar[] NOT NULL DEFAULT '{}',
    "___read" integer NOT NULL DEFAULT 0,
    "unique_temp___write" varchar[] NOT NULL DEFAULT '{}',
    "___write" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredWithinWeek" varchar[] NOT NULL DEFAULT '{}',
    "___registeredWithinWeek" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredWithinMonth" varchar[] NOT NULL DEFAULT '{}',
    "___registeredWithinMonth" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredWithinYear" varchar[] NOT NULL DEFAULT '{}',
    "___registeredWithinYear" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredOutsideWeek" varchar[] NOT NULL DEFAULT '{}',
    "___registeredOutsideWeek" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredOutsideMonth" varchar[] NOT NULL DEFAULT '{}',
    "___registeredOutsideMonth" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredOutsideYear" varchar[] NOT NULL DEFAULT '{}',
    "___registeredOutsideYear" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__active_users_date" ON "__chart__active_users" ("date");

CREATE TABLE IF NOT EXISTS "__chart_day__active_users" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "___readWrite" integer NOT NULL DEFAULT 0,
    "unique_temp___read" varchar[] NOT NULL DEFAULT '{}',
    "___read" integer NOT NULL DEFAULT 0,
    "unique_temp___write" varchar[] NOT NULL DEFAULT '{}',
    "___write" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredWithinWeek" varchar[] NOT NULL DEFAULT '{}',
    "___registeredWithinWeek" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredWithinMonth" varchar[] NOT NULL DEFAULT '{}',
    "___registeredWithinMonth" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredWithinYear" varchar[] NOT NULL DEFAULT '{}',
    "___registeredWithinYear" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredOutsideWeek" varchar[] NOT NULL DEFAULT '{}',
    "___registeredOutsideWeek" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredOutsideMonth" varchar[] NOT NULL DEFAULT '{}',
    "___registeredOutsideMonth" integer NOT NULL DEFAULT 0,
    "unique_temp___registeredOutsideYear" varchar[] NOT NULL DEFAULT '{}',
    "___registeredOutsideYear" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__active_users_date" ON "__chart_day__active_users" ("date");

----------------------------------------------------------------------
-- perUserNotes (grouped by userId)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__per_user_notes" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___total" integer NOT NULL DEFAULT 0,
    "___inc" smallint NOT NULL DEFAULT 0,
    "___dec" smallint NOT NULL DEFAULT 0,
    "___diffs_normal" smallint NOT NULL DEFAULT 0,
    "___diffs_reply" smallint NOT NULL DEFAULT 0,
    "___diffs_renote" smallint NOT NULL DEFAULT 0,
    "___diffs_withFile" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__per_user_notes_date_group" ON "__chart__per_user_notes" ("date", "group");

CREATE TABLE IF NOT EXISTS "__chart_day__per_user_notes" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___total" integer NOT NULL DEFAULT 0,
    "___inc" smallint NOT NULL DEFAULT 0,
    "___dec" smallint NOT NULL DEFAULT 0,
    "___diffs_normal" smallint NOT NULL DEFAULT 0,
    "___diffs_reply" smallint NOT NULL DEFAULT 0,
    "___diffs_renote" smallint NOT NULL DEFAULT 0,
    "___diffs_withFile" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__per_user_notes_date_group" ON "__chart_day__per_user_notes" ("date", "group");

----------------------------------------------------------------------
-- perUserDrive (grouped by userId)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__per_user_drive" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___totalCount" integer NOT NULL DEFAULT 0,
    "___totalSize" integer NOT NULL DEFAULT 0,
    "___incCount" smallint NOT NULL DEFAULT 0,
    "___incSize" integer NOT NULL DEFAULT 0,
    "___decCount" smallint NOT NULL DEFAULT 0,
    "___decSize" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__per_user_drive_date_group" ON "__chart__per_user_drive" ("date", "group");

CREATE TABLE IF NOT EXISTS "__chart_day__per_user_drive" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___totalCount" integer NOT NULL DEFAULT 0,
    "___totalSize" integer NOT NULL DEFAULT 0,
    "___incCount" smallint NOT NULL DEFAULT 0,
    "___incSize" integer NOT NULL DEFAULT 0,
    "___decCount" smallint NOT NULL DEFAULT 0,
    "___decSize" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__per_user_drive_date_group" ON "__chart_day__per_user_drive" ("date", "group");

----------------------------------------------------------------------
-- perUserFollowing (grouped by userId)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__per_user_following" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___local_followings_total" integer NOT NULL DEFAULT 0,
    "___local_followings_inc" smallint NOT NULL DEFAULT 0,
    "___local_followings_dec" smallint NOT NULL DEFAULT 0,
    "___local_followers_total" integer NOT NULL DEFAULT 0,
    "___local_followers_inc" smallint NOT NULL DEFAULT 0,
    "___local_followers_dec" smallint NOT NULL DEFAULT 0,
    "___remote_followings_total" integer NOT NULL DEFAULT 0,
    "___remote_followings_inc" smallint NOT NULL DEFAULT 0,
    "___remote_followings_dec" smallint NOT NULL DEFAULT 0,
    "___remote_followers_total" integer NOT NULL DEFAULT 0,
    "___remote_followers_inc" smallint NOT NULL DEFAULT 0,
    "___remote_followers_dec" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__per_user_following_date_group" ON "__chart__per_user_following" ("date", "group");

CREATE TABLE IF NOT EXISTS "__chart_day__per_user_following" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___local_followings_total" integer NOT NULL DEFAULT 0,
    "___local_followings_inc" smallint NOT NULL DEFAULT 0,
    "___local_followings_dec" smallint NOT NULL DEFAULT 0,
    "___local_followers_total" integer NOT NULL DEFAULT 0,
    "___local_followers_inc" smallint NOT NULL DEFAULT 0,
    "___local_followers_dec" smallint NOT NULL DEFAULT 0,
    "___remote_followings_total" integer NOT NULL DEFAULT 0,
    "___remote_followings_inc" smallint NOT NULL DEFAULT 0,
    "___remote_followings_dec" smallint NOT NULL DEFAULT 0,
    "___remote_followers_total" integer NOT NULL DEFAULT 0,
    "___remote_followers_inc" smallint NOT NULL DEFAULT 0,
    "___remote_followers_dec" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__per_user_following_date_group" ON "__chart_day__per_user_following" ("date", "group");

----------------------------------------------------------------------
-- perUserPv (grouped by userId)
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__per_user_pv" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "unique_temp___upv_user" varchar[] NOT NULL DEFAULT '{}',
    "___upv_user" smallint NOT NULL DEFAULT 0,
    "___pv_user" smallint NOT NULL DEFAULT 0,
    "unique_temp___upv_visitor" varchar[] NOT NULL DEFAULT '{}',
    "___upv_visitor" smallint NOT NULL DEFAULT 0,
    "___pv_visitor" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__per_user_pv_date_group" ON "__chart__per_user_pv" ("date", "group");

CREATE TABLE IF NOT EXISTS "__chart_day__per_user_pv" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "unique_temp___upv_user" varchar[] NOT NULL DEFAULT '{}',
    "___upv_user" smallint NOT NULL DEFAULT 0,
    "___pv_user" smallint NOT NULL DEFAULT 0,
    "unique_temp___upv_visitor" varchar[] NOT NULL DEFAULT '{}',
    "___upv_visitor" smallint NOT NULL DEFAULT 0,
    "___pv_visitor" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__per_user_pv_date_group" ON "__chart_day__per_user_pv" ("date", "group");

----------------------------------------------------------------------
-- perUserReactions (grouped by userId) → table __chart__per_user_reaction
----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS "__chart__per_user_reaction" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___local_count" smallint NOT NULL DEFAULT 0,
    "___remote_count" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart__per_user_reaction_date_group" ON "__chart__per_user_reaction" ("date", "group");

CREATE TABLE IF NOT EXISTS "__chart_day__per_user_reaction" (
    "id" SERIAL PRIMARY KEY,
    "date" integer NOT NULL,
    "group" varchar(128) NOT NULL,
    "___local_count" smallint NOT NULL DEFAULT 0,
    "___remote_count" smallint NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "IDX___chart_day__per_user_reaction_date_group" ON "__chart_day__per_user_reaction" ("date", "group");
