-- user_pending: 招待制 + email 確認制の併用運用で「1 招待で複数 pending を
-- 作って全部 promote すると複数アカウント作れる」 gap を塞ぐため、pending
-- 行に invitation_ticket_id を持たせて PromotePending 時に MarkUsed する。
ALTER TABLE "user_pending" ADD COLUMN IF NOT EXISTS "invitationTicketId" varchar(32);
