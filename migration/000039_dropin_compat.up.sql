-- Phase 13-2 (#367) drop-in 互換: TS-A → mk-A 切替で発覚した欠損カラム補填。
--
-- mk-go の internal/model/note.go は note テーブルに `pageCount` smallint
-- (Misskey TS 側で 1755168347001_PageCountInNote として 2025-08 に追加) が
-- 存在することを前提にしている。一方、それより古い TS バックエンド (例えば
-- misskey/misskey:2025.2.1) で初期化された DB にはこのカラムが無い。
--
-- 000001_initial.up.sql は CREATE TABLE IF NOT EXISTS で note テーブルを作る
-- が、既に TS-create された note テーブルに対しては no-op となるため
-- pageCount カラムは追加されない。drop-in 切替時に mk-go が
-- "column \"pageCount\" of relation \"note\" does not exist" で 500 を返す
-- 原因になっていた。
--
-- ここでは ALTER TABLE ... ADD COLUMN IF NOT EXISTS で冪等に追加する。

ALTER TABLE "note" ADD COLUMN IF NOT EXISTS "pageCount" smallint NOT NULL DEFAULT 0;

-- renoteChannelId: TS 側で channel-renote 機能向けに追加された列。古い TS
-- にはまだ無いため drop-in 切替で mk-go の INSERT が失敗する。
ALTER TABLE "note" ADD COLUMN IF NOT EXISTS "renoteChannelId" varchar(32);
