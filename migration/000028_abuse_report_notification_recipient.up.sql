CREATE TABLE IF NOT EXISTS "abuse_report_notification_recipient" (
    "id" varchar(32) PRIMARY KEY,
    "isActive" boolean NOT NULL DEFAULT true,
    "name" varchar(255) NOT NULL,
    "method" varchar(64) NOT NULL DEFAULT 'email',
    "userId" varchar(32) REFERENCES "user"("id") ON DELETE SET NULL,
    "systemWebhookId" varchar(32) REFERENCES "system_webhook"("id") ON DELETE SET NULL
);
