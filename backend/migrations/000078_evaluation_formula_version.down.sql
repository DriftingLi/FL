-- 回滚后应用将所有记录按旧公式解释。
ALTER TABLE evaluations DROP COLUMN formula_version;
