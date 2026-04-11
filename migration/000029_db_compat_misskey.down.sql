-- Phase: Misskey DB スキーマ完全互換化 のロールバック。

-- 5. migrations テーブル (TypeORM 互換) を削除
DROP TABLE IF EXISTS "migrations";

-- 4. 欠落テーブル 5 つを削除 (依存順に FK 子 → 親)
DROP TABLE IF EXISTS "promo_read";
DROP TABLE IF EXISTS "promo_note";
DROP TABLE IF EXISTS "user_memo";
DROP TABLE IF EXISTS "user_ip";
DROP TABLE IF EXISTS "user_security_key";

-- 3. meta に追加した 58 カラムを除去
ALTER TABLE "meta" ALTER COLUMN "feedbackUrl" DROP DEFAULT;
ALTER TABLE "meta" ALTER COLUMN "repositoryUrl" DROP DEFAULT;

ALTER TABLE "meta"
    DROP COLUMN IF EXISTS "clientOptions",
    DROP COLUMN IF EXISTS "deliverSuspendedSoftware",
    DROP COLUMN IF EXISTS "ugcVisibilityForVisitor",
    DROP COLUMN IF EXISTS "inquiryUrl",
    DROP COLUMN IF EXISTS "googleAnalyticsMeasurementId",
    DROP COLUMN IF EXISTS "singleUserMode",
    DROP COLUMN IF EXISTS "remoteNotesCleaningExpiryDaysForEachNotes",
    DROP COLUMN IF EXISTS "remoteNotesCleaningMaxProcessingDurationInMinutes",
    DROP COLUMN IF EXISTS "enableRemoteNotesCleaning",
    DROP COLUMN IF EXISTS "allowExternalApRedirect",
    DROP COLUMN IF EXISTS "enableReactionsBuffering",
    DROP COLUMN IF EXISTS "showRoleBadgesOfRemoteUsers",
    DROP COLUMN IF EXISTS "enableServerMachineStats",
    DROP COLUMN IF EXISTS "enableStatsForFederatedInstances",
    DROP COLUMN IF EXISTS "enableChartsForFederatedInstances",
    DROP COLUMN IF EXISTS "enableChartsForRemoteUser",
    DROP COLUMN IF EXISTS "enableIpLogging",
    DROP COLUMN IF EXISTS "enableIdenticonGeneration",
    DROP COLUMN IF EXISTS "deeplIsPro",
    DROP COLUMN IF EXISTS "deeplAuthKey",
    DROP COLUMN IF EXISTS "prohibitedWordsForNameOfUser",
    DROP COLUMN IF EXISTS "preservedUsernames",
    DROP COLUMN IF EXISTS "bannedEmailDomains",
    DROP COLUMN IF EXISTS "manifestJsonOverride",
    DROP COLUMN IF EXISTS "notesPerOneAd",
    DROP COLUMN IF EXISTS "perUserListTimelineCacheMax",
    DROP COLUMN IF EXISTS "perUserHomeTimelineCacheMax",
    DROP COLUMN IF EXISTS "perRemoteUserUserTimelineCacheMax",
    DROP COLUMN IF EXISTS "perLocalUserUserTimelineCacheMax",
    DROP COLUMN IF EXISTS "urlPreviewUserAgent",
    DROP COLUMN IF EXISTS "urlPreviewSummaryProxyUrl",
    DROP COLUMN IF EXISTS "urlPreviewRequireContentLength",
    DROP COLUMN IF EXISTS "urlPreviewMaximumContentLength",
    DROP COLUMN IF EXISTS "urlPreviewTimeout",
    DROP COLUMN IF EXISTS "urlPreviewAllowRedirect",
    DROP COLUMN IF EXISTS "urlPreviewEnabled",
    DROP COLUMN IF EXISTS "mediaSilencedHosts",
    DROP COLUMN IF EXISTS "enableSensitiveMediaDetectionForVideos",
    DROP COLUMN IF EXISTS "setSensitiveFlagAutomatically",
    DROP COLUMN IF EXISTS "sensitiveMediaDetectionSensitivity",
    DROP COLUMN IF EXISTS "sensitiveMediaDetection",
    DROP COLUMN IF EXISTS "truemailAuthKey",
    DROP COLUMN IF EXISTS "truemailInstance",
    DROP COLUMN IF EXISTS "enableTruemailApi",
    DROP COLUMN IF EXISTS "verifymailAuthKey",
    DROP COLUMN IF EXISTS "enableVerifymailApi",
    DROP COLUMN IF EXISTS "enableActiveEmailValidation",
    DROP COLUMN IF EXISTS "enableTestcaptcha",
    DROP COLUMN IF EXISTS "mcaptchaInstanceUrl",
    DROP COLUMN IF EXISTS "mcaptchaSecretKey",
    DROP COLUMN IF EXISTS "mcaptchaSitekey",
    DROP COLUMN IF EXISTS "enableMcaptcha",
    DROP COLUMN IF EXISTS "defaultDarkTheme",
    DROP COLUMN IF EXISTS "defaultLightTheme",
    DROP COLUMN IF EXISTS "infoImageUrl",
    DROP COLUMN IF EXISTS "notFoundImageUrl",
    DROP COLUMN IF EXISTS "serverErrorImageUrl",
    DROP COLUMN IF EXISTS "app512IconUrl",
    DROP COLUMN IF EXISTS "app192IconUrl",
    DROP COLUMN IF EXISTS "mascotImageUrl";

-- 2. user_profile に追加した 3 カラムを除去
ALTER TABLE "user_profile"
    DROP COLUMN IF EXISTS "room",
    DROP COLUMN IF EXISTS "clientData",
    DROP COLUMN IF EXISTS "twoFactorBackupSecret";

-- 1. poll_vote.createdAt を復元 (ロールバック用)。
-- 元のスキーマは NOT NULL DEFAULT now() だったので同じに戻す。
ALTER TABLE "poll_vote" ADD COLUMN IF NOT EXISTS "createdAt" timestamp with time zone NOT NULL DEFAULT now();
