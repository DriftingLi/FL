-- 000060: 存量题归属回填（#2598 / 规格块 C2 的落地前提）
--
-- 背景：#2598 起题库写路径前置归属谓词——「只有创建者本人（或管理员）能改这道题」。
-- 但 `000001_baseline.up.sql` 里 477 道种子题的 `created_by` **全部为 NULL**
-- （created_by_type 一律 'admin'，表示系统导入），全仓此前没有任何回填 `question.created_by`
-- 的迁移。若不做回填就上线，讲师会对**全部**存量题失去写权——规格 C2 原文假设
-- 「NULL 是少数历史遗留、比例待生产库现测」，实测是 100%，假设不成立。
--
-- 处置：把这些「系统导入、无具体作者」的题归给库中**最早创建的那位讲师**
-- （tutor_id 最小者）。理由：
--   - 这些题是课程配套题库，实际维护者是讲师而非管理员；
--   - 取「最早创建」而不是「任意一位」，是为了**确定性**——同一份数据无论重跑几次、
--     在哪个库上跑，结果都一样（ORDER BY tutor_id LIMIT 1）；
--   - 同一批题归给同一个人，而不是按 id 分散给多个讲师：分散需要一条没有事实依据的分配
--     规则（比如取模），那比「都归最早那位」更不可解释，且会让「谁该改这道题」变成猜谜。
--
-- created_by_type 同步改为 'tutor'：`created_by` 与 `created_by_type` 是同一个事实的两半，
-- 只改一半会留下「作者 id 是讲师、类型却是 admin」的自相矛盾行，后续任何按类型做的
-- 统计或分流都会读到脏数据。
--
-- 幂等：WHERE created_by IS NULL 保证重跑零影响（已回填的行不再命中）。
-- 无讲师时**不回填**：此时库里没人能接手这些题，保持 NULL（= 只有管理员可写）比
-- 硬塞一个不存在的 id 诚实；管理员仍可通过管理端维护，等第一位讲师建号后再由
-- 运维决定是否补跑（本迁移在迁移表里已记过，重跑不会自动发生——这是刻意的：
-- 自动重跑会在「第一位讲师建号」的那一刻把全部题库判给他，那不是数据迁移该有的副作用）。

UPDATE question
SET created_by      = (SELECT tutor_id FROM tutor ORDER BY tutor_id ASC LIMIT 1),
    created_by_type = 'tutor'
WHERE created_by IS NULL
  AND EXISTS (SELECT 1 FROM tutor);

COMMENT ON COLUMN question.created_by IS
  '题目创建者账号 id（#2598 写隔离的判据：只有本人或管理员可改；NULL = 作者未知，仅管理员可写）';
