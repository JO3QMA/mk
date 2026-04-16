-- meta.proxyAccountId: admin/update-proxy-account が書き込む、インスタンスが
-- プロキシ用途で使うユーザーの ID を保持する。Misskey 本家の meta.proxyAccountId
-- と互換。
ALTER TABLE "meta" ADD COLUMN IF NOT EXISTS "proxyAccountId" varchar(32);
