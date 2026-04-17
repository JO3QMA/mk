ALTER TABLE "chat_message" DROP COLUMN IF EXISTS "isDeliverFailed";
ALTER TABLE "chat_message" DROP COLUMN IF EXISTS "isDelivering";
ALTER TABLE "chat_message" DROP COLUMN IF EXISTS "emojis";
DROP TABLE IF EXISTS "user_pending";
DROP TABLE IF EXISTS "chat_approval";
