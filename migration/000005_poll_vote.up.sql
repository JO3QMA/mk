-- poll_vote
CREATE TABLE IF NOT EXISTS "poll_vote" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "noteId" varchar(32) NOT NULL REFERENCES "note"("id") ON DELETE CASCADE,
    "choice" integer NOT NULL,
    "createdAt" timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS "IDX_poll_vote_userId" ON "poll_vote" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_poll_vote_noteId" ON "poll_vote" ("noteId");
CREATE UNIQUE INDEX IF NOT EXISTS "IDX_poll_vote_userId_noteId_choice" ON "poll_vote" ("userId", "noteId", "choice");
