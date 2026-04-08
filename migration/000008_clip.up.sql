-- clip: ユーザーが note を集めるブックマーク/コレクション (Misskey 互換)
CREATE TABLE IF NOT EXISTS "clip" (
    "id" varchar(32) PRIMARY KEY,
    "lastClippedAt" timestamp with time zone,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "name" varchar(128) NOT NULL,
    "isPublic" boolean NOT NULL DEFAULT false,
    "description" varchar(2048),
    "notesCount" integer NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS "IDX_clip_userId" ON "clip" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_clip_lastClippedAt" ON "clip" ("lastClippedAt");

-- clip_note: clip に追加された note の中間テーブル
CREATE TABLE IF NOT EXISTS "clip_note" (
    "id" varchar(32) PRIMARY KEY,
    "noteId" varchar(32) NOT NULL REFERENCES "note"("id") ON DELETE CASCADE,
    "clipId" varchar(32) NOT NULL REFERENCES "clip"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "IDX_clip_note_noteId" ON "clip_note" ("noteId");
CREATE INDEX IF NOT EXISTS "IDX_clip_note_clipId" ON "clip_note" ("clipId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_clip_note_pair" ON "clip_note" ("noteId", "clipId");
