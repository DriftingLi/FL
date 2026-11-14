-- 回退 000056：站内信收件人身份空间与章节归属（ADR-0095，#2537）。
--
-- 顺序与 up 相反。回退前须知：若已有讲师收件通知（tutor_id 非空），
-- 恢复 user_id NOT NULL 会失败——那是**正确的失败**（数据形态已不兼容旧约束），
-- 需先人工决定那些通知的去向，不要靠改约束强删。

DROP INDEX IF EXISTS idx_chapter_tutor;
ALTER TABLE chapter DROP COLUMN IF EXISTS tutor_id;

DROP INDEX IF EXISTS idx_notifications_tutor_unread;
DROP INDEX IF EXISTS idx_notifications_tutor_created;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS chk_notifications_recipient;
ALTER TABLE notifications DROP COLUMN IF EXISTS tutor_id;
ALTER TABLE notifications ALTER COLUMN user_id SET NOT NULL;
