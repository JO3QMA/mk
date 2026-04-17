-- TS側はmigration 1580148575182 (appId) / 1696569742153 (score) でDROP済み。
-- Go側 000033 で追加したが、既存TSインスタンスのDBには存在しないため
-- スキーマ整合のために削除する。
ALTER TABLE "note" DROP COLUMN IF EXISTS "appId";
ALTER TABLE "note" DROP COLUMN IF EXISTS "score";
