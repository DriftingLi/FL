-- #2070（ADR-0089）：反馈小域——学员面向平台的单向消息。
--
-- 一张表、两个状态（pending / handled）、四个端点（学员提交 + 本人历史；管理端列表 + 标记已处理）。
-- **刻意不做**：工单号 / 分配 / 优先级 / SLA / 状态流转历史 / 回复线程（ADR-0089 决策 1 与「备选」）。
--
-- 与相邻面的边界（同一份 ADR）：不公开发布（论坛）、不是下发通道（站内信）、不是编辑内容
-- （内容精选）、不进治理队列（举报）。
--
-- images 与论坛图片同前缀 images/forum/（ADR-0066 决策 5：不新建上传通道）⇒ 论坛的悬空图片
-- 清理必须把本表的图算进引用集，否则 24h 后会被静默删掉（见 internal/forum/image_service.go）。

CREATE TABLE feedback (
    id         BIGSERIAL    PRIMARY KEY,
    user_id    INTEGER      NOT NULL REFERENCES hrwai_users(id) ON DELETE CASCADE,
    content    TEXT         NOT NULL,
    images     JSONB        NOT NULL DEFAULT '[]'::jsonb,
    contact    VARCHAR(100) NOT NULL DEFAULT '',
    status     VARCHAR(16)  NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'handled')),
    handled_at TIMESTAMPTZ,
    handled_by INTEGER      REFERENCES admin(admin_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 本人历史：按人 + 时间倒序（学员面分页的唯一查询）。
CREATE INDEX idx_feedback_user ON feedback (user_id, created_at DESC);
-- 管理端队列：按状态 + 时间倒序（pending 优先看，handled 归档查看）。
CREATE INDEX idx_feedback_status ON feedback (status, created_at DESC);

COMMENT ON TABLE feedback IS '学员反馈（ADR-0089）：单向消息，两态 pending/handled，无工单流';
COMMENT ON COLUMN feedback.content IS '反馈正文（1-1000 字，去空白后非空）';
COMMENT ON COLUMN feedback.images IS '图片 URL 数组（0-3 张，仅本站 images/forum/ 前缀）';
COMMENT ON COLUMN feedback.contact IS '联系方式（可空、不强制格式：手机/微信/QQ/邮箱都可能）';
COMMENT ON COLUMN feedback.status IS '状态：pending 待处理 / handled 已处理（标记幂等，离开 pending 后不再变）';
COMMENT ON COLUMN feedback.handled_by IS '处置管理员（admin_id；管理员账号删除时置空，处置事实仍由 handled_at 承载）';
