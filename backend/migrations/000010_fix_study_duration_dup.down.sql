-- 000010_fix_study_duration_dup.down.sql
-- 回滚说明：历史重复累加的 study_duration 无法精确恢复（原始值已被覆盖），
-- 本迁移为不可逆的数据修复，down 仅做占位。
-- 如需回滚代码层修改，请手动 git revert 相关 commit；数据不回滚。
SELECT 1;
