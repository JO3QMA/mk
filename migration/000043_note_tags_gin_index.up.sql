-- #655: hashtags/trend は note.tags 配列を unnest して直近 200 分の
-- 投稿を集計する。tags && ARRAY[...] / unnest("tags") の両方を高速化
-- するため GIN インデックスを追加する。
CREATE INDEX IF NOT EXISTS "IDX_note_tags" ON "note" USING gin ("tags");
