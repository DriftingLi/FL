-- 站内信收件人身份空间与章节归属（ADR-0095，#2537）。
--
-- 一、notifications 收件人从「只能是学员」放开为「学员或讲师，恰好其一」：
--     user_id 改为可空、新增 tutor_id、CHECK num_nonnulls 兜底。
--     存量通知全部是学员的（user_id 非空、tutor_id 为空），恰好是新 CHECK 的合法形态，
--     故本迁移**纯 DDL、零回填**——约束在迁移完成瞬间即对存量行成立。
--
-- 二、chapter 获得稳定归属（作者讲师）：讲师侧写、首次落定后不改（ADR-0095 决策二）。

ALTER TABLE notifications ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS tutor_id INTEGER REFERENCES tutor(tutor_id) ON DELETE CASCADE;

-- 恰好一列非空：一条站内信只发给一个人，收件人身份二选一（ADR-0095）。
-- num_nonnulls 是 PG 内置函数，与「user_id NOT NULL 或 tutor_id NOT NULL 且不同时非空」等价且更强。
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS chk_notifications_recipient;
ALTER TABLE notifications
    ADD CONSTRAINT chk_notifications_recipient CHECK (num_nonnulls(user_id, tutor_id) = 1);

-- 讲师信箱读面索引：按收件人 + 时间倒序（与学员侧 idx_notifications_user_created 同形）。
CREATE INDEX IF NOT EXISTS idx_notifications_tutor_created
    ON notifications (tutor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_tutor_unread
    ON notifications (tutor_id) WHERE is_read = FALSE;

-- 章节归属：可空（存量章节作者未知），删讲师置空（内容资产不随身份删除消失）。
ALTER TABLE chapter
    ADD COLUMN IF NOT EXISTS tutor_id INTEGER REFERENCES tutor(tutor_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_chapter_tutor ON chapter (tutor_id);

COMMENT ON COLUMN notifications.tutor_id IS '讲师收件人（与 user_id 恰好一列非空，ADR-0095）';
COMMENT ON COLUMN chapter.tutor_id IS '章节作者讲师（首次落定后不改；空=作者未知，ADR-0095）';
