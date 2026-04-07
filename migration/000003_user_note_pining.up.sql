-- user_note_pining
CREATE TABLE IF NOT EXISTS "user_note_pining" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "noteId" varchar(32) NOT NULL REFERENCES "note"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_user_note_pining_userId" ON "user_note_pining" ("userId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_user_note_pining_userId_noteId" ON "user_note_pining" ("userId", "noteId");
