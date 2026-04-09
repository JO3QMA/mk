-- note_favorite: ノートのお気に入り (Misskey 互換)
CREATE TABLE IF NOT EXISTS "note_favorite" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "noteId" varchar(32) NOT NULL REFERENCES "note"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_note_favorite_userId" ON "note_favorite" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_note_favorite_noteId" ON "note_favorite" ("noteId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_note_favorite_pair" ON "note_favorite" ("userId", "noteId");
