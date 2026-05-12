-- upstream Misskey #17034 (= 2026.5.0 / triage follow-up): avatar_decoration に
-- category column を追加し、管理画面でカテゴリ分類できるようにする。nullable
-- なので既存行の影響なし。
ALTER TABLE "avatar_decoration" ADD COLUMN "category" varchar(128);
