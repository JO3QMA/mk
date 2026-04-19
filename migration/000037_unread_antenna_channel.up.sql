-- antenna_note_unread: per-user per-note antenna 未読レコード。
-- antenna owner (antenna.userId) が未読の note を追跡。
-- Note / Antenna / User いずれかが削除されたらCASCADE で消える。
CREATE TABLE IF NOT EXISTS "antenna_note_unread" (
    "id" varchar(32) PRIMARY KEY,
    "antennaId" varchar(32) NOT NULL REFERENCES "antenna"("id") ON DELETE CASCADE,
    "noteId" varchar(32) NOT NULL REFERENCES "note"("id") ON DELETE CASCADE,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS "UQ_antenna_note_unread_triple"
    ON "antenna_note_unread" ("userId", "antennaId", "noteId");
CREATE INDEX IF NOT EXISTS "IDX_antenna_note_unread_userId"
    ON "antenna_note_unread" ("userId");

-- channel_note_unread: per-follower per-note channel 未読レコード。
-- channel follower が未読の note を追跡。
CREATE TABLE IF NOT EXISTS "channel_note_unread" (
    "id" varchar(32) PRIMARY KEY,
    "channelId" varchar(32) NOT NULL REFERENCES "channel"("id") ON DELETE CASCADE,
    "noteId" varchar(32) NOT NULL REFERENCES "note"("id") ON DELETE CASCADE,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS "UQ_channel_note_unread_triple"
    ON "channel_note_unread" ("userId", "channelId", "noteId");
CREATE INDEX IF NOT EXISTS "IDX_channel_note_unread_userId"
    ON "channel_note_unread" ("userId");
