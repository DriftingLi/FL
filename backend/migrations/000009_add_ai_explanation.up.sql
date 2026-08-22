-- 题目 AI 解析缓存（spec #284，ticket #289）
ALTER TABLE question ADD COLUMN IF NOT EXISTS ai_explanation TEXT NOT NULL DEFAULT '';
