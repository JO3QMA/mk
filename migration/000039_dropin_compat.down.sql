-- 000039 の down: drop-in 互換のために追加した列を取り除く。本家 TS が
-- 同名 migration を後から回せば再追加される。
ALTER TABLE "note" DROP COLUMN IF EXISTS "pageCount";
ALTER TABLE "note" DROP COLUMN IF EXISTS "renoteChannelId";
