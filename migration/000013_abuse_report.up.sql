-- abuse_user_report: ユーザー通報 (Misskey 互換)
CREATE TABLE IF NOT EXISTS "abuse_user_report" (
    "id" varchar(32) PRIMARY KEY,
    "targetUserId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "reporterId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "assigneeId" varchar(32) REFERENCES "user"("id") ON DELETE SET NULL,
    "resolved" boolean NOT NULL DEFAULT false,
    "forwarded" boolean NOT NULL DEFAULT false,
    "comment" varchar(2048) NOT NULL DEFAULT '',
    "moderationNote" varchar(8192) NOT NULL DEFAULT '',
    "resolvedAs" varchar(16),
    "targetUserHost" varchar(128),
    "reporterHost" varchar(128)
);

CREATE INDEX IF NOT EXISTS "IDX_abuse_user_report_targetUserId" ON "abuse_user_report" ("targetUserId");
CREATE INDEX IF NOT EXISTS "IDX_abuse_user_report_reporterId" ON "abuse_user_report" ("reporterId");
CREATE INDEX IF NOT EXISTS "IDX_abuse_user_report_resolved" ON "abuse_user_report" ("resolved");

-- moderation_log: モデレーション操作ログ (Misskey 互換)
CREATE TABLE IF NOT EXISTS "moderation_log" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "type" varchar(128) NOT NULL,
    "info" jsonb NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS "IDX_moderation_log_userId" ON "moderation_log" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_moderation_log_type" ON "moderation_log" ("type");
