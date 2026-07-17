-- 000002_question_reject_reason.up.sql
-- 题目表新增驳回理由字段（审核流程：导师提交 pending → 管理员发布/驳回 → 驳回填理由回退 draft）
ALTER TABLE question ADD COLUMN IF NOT EXISTS reject_reason TEXT;
COMMENT ON COLUMN question.reject_reason IS '驳回理由（管理员驳回时填写，导师修改重提后清空）';
