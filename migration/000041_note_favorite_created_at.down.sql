-- Revert #424 follow-up: drop createdAt column from note_favorite.
DROP INDEX IF EXISTS "IDX_note_favorite_createdAt";
ALTER TABLE "note_favorite" DROP COLUMN IF EXISTS "createdAt";
