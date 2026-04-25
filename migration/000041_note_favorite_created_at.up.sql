-- Misskey 本家互換のため note_favorite に createdAt を追加 (#424)。
-- /api/i/favorites の response shape に createdAt を含める要件で、UI の
-- お気に入り一覧で個々の registered timestamp を expose するため。
-- 既存行には NOW() を埋めるが、本家データに揃えたい場合は admin 操作で
-- 上書き可能。default を残しておくと挿入側で明示しなくても良くなり
-- 既存 INSERT の互換性も保たれる。
ALTER TABLE "note_favorite"
    ADD COLUMN IF NOT EXISTS "createdAt" timestamp with time zone NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS "IDX_note_favorite_createdAt" ON "note_favorite" ("createdAt");
