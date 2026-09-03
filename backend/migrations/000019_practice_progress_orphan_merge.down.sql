-- #510 回滚：000019 只做数据归并 + 函数定义，数据不可逆。回滚仅删函数，
-- 不还原数据（已并入桶行，恢复孤儿行无意义且违背 #504 语义）。
DROP FUNCTION IF EXISTS merge_orphan_practice_progress();
