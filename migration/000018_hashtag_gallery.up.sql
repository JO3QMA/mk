-- hashtag: ハッシュタグ (Misskey 互換)
CREATE TABLE IF NOT EXISTS "hashtag" (
    "id" varchar(32) PRIMARY KEY,
    "name" varchar(128) NOT NULL,
    "mentionedUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "mentionedUsersCount" integer NOT NULL DEFAULT 0,
    "mentionedLocalUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "mentionedLocalUsersCount" integer NOT NULL DEFAULT 0,
    "mentionedRemoteUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "mentionedRemoteUsersCount" integer NOT NULL DEFAULT 0,
    "attachedUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "attachedUsersCount" integer NOT NULL DEFAULT 0,
    "attachedLocalUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "attachedLocalUsersCount" integer NOT NULL DEFAULT 0,
    "attachedRemoteUserIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "attachedRemoteUsersCount" integer NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS "IDX_hashtag_name" ON "hashtag" ("name");

-- gallery_post: ギャラリー投稿 (Misskey 互換)
CREATE TABLE IF NOT EXISTS "gallery_post" (
    "id" varchar(32) PRIMARY KEY,
    "updatedAt" timestamp with time zone NOT NULL DEFAULT now(),
    "title" varchar(256) NOT NULL,
    "description" varchar(2048),
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "fileIds" varchar(32)[] NOT NULL DEFAULT '{}',
    "isSensitive" boolean NOT NULL DEFAULT false,
    "likedCount" integer NOT NULL DEFAULT 0,
    "tags" varchar(128)[] NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS "IDX_gallery_post_userId" ON "gallery_post" ("userId");
CREATE INDEX IF NOT EXISTS "IDX_gallery_post_tags" ON "gallery_post" ("tags");

-- gallery_like: ギャラリーいいね
CREATE TABLE IF NOT EXISTS "gallery_like" (
    "id" varchar(32) PRIMARY KEY,
    "userId" varchar(32) NOT NULL REFERENCES "user"("id") ON DELETE CASCADE,
    "postId" varchar(32) NOT NULL REFERENCES "gallery_post"("id") ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS "IDX_gallery_like_pair" ON "gallery_like" ("userId", "postId");
