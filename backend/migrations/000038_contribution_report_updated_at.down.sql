-- 回滚 #1099 的补列：把 contribution_report 退回 000020 建表时的形状（没有 updated_at）。
--
-- 注意：若该列是历史路径建的（而非本迁移），回滚会一并删掉它——这与 000020 的建表形状
-- 一致，也正是「回滚 = 退回旧代码」的语义（旧代码的 model 仍然要这一列，属于已知的回滚代价）。

ALTER TABLE IF EXISTS contribution_report
    DROP COLUMN IF EXISTS updated_at;
