-- ============================================================
-- 000003 question_tag 退役 category 列（考点模块为死字段，全库无消费方）
-- 决策：标签的「考点模块」category 自引入起仅存储、无任何过滤/分组/展示
-- 消费（学员端练习按 tag id），按死字段退役；代码层同步移除
-- （model.Category、service 读写、前端表单）。
-- ============================================================

ALTER TABLE question_tag DROP COLUMN IF EXISTS category;
