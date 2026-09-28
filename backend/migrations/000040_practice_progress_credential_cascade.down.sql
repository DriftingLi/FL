-- 回滚 #1360：把 practice_progress.credential_id 的外键退回 000013 的形态（匿名动作 = NO ACTION）。
--
-- 已知代价（与所有「删过父行」的回滚同形，写明白不藏）：CASCADE 删掉的练习进度分区无法由本文件
-- 恢复——回滚只还原**约束**，不还原数据。这是「回滚 = 退回旧代码」的固有语义，旧代码本来也不会
-- 带着这些分区（它压根删不掉证件）。
--
-- 约束名沿用 000040 up 建出来的那个名字，形状与 000013 的列级 REFERENCES 逐字等价
-- （PG 对匿名列级 FK 也是 <table>_<column>_fkey 命名），故 down 后再 up 一次不会留下两个约束。

ALTER TABLE practice_progress
    DROP CONSTRAINT IF EXISTS practice_progress_credential_id_fkey;

ALTER TABLE practice_progress
    ADD CONSTRAINT practice_progress_credential_id_fkey
    FOREIGN KEY (credential_id) REFERENCES credential(id);
