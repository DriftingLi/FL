-- 回滚：恢复 specialty 列（数据不恢复，需人工迁移）；岗位字典表删除。
ALTER TABLE job_cards ADD COLUMN expected_specialty_id INTEGER;
ALTER TABLE job_cards ADD COLUMN expected_specialty_extra VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE job_cards DROP COLUMN IF EXISTS expected_position_id;
ALTER TABLE job_cards DROP COLUMN IF EXISTS expected_position_extra;

ALTER TABLE job_postings ADD COLUMN specialty_id INTEGER;
ALTER TABLE job_postings DROP COLUMN IF EXISTS position_id;

DROP TABLE IF EXISTS positions;
