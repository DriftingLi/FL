-- #414 答题进度按目标证件分桶：practice_progress 增加证件归属列，
-- 唯一键从 (student_id, practice_mode) 扩为 (student_id, practice_mode, credential_id)。
--
-- 回填取舍（#414 PR 声明）：存量每学员每模式仅一行，只能归到该学员【当前】证件
-- （hrwai_users.current_credential_id），其余证件分区从其视角重新起算——唯一可行解。
-- 当前证件为 NULL 的存量行保持 NULL（无分区），由 partial 唯一索引兜底（NULL 不判重）。

ALTER TABLE practice_progress ADD COLUMN credential_id INT REFERENCES credential(id);

UPDATE practice_progress pp
SET credential_id = u.current_credential_id
FROM hrwai_users u
WHERE pp.student_id = u.id AND u.current_credential_id IS NOT NULL;

COMMENT ON COLUMN practice_progress.credential_id IS '练习进度归属的目标证件分区（#414；NULL=未分区，兼容未预筛选学员）';

-- 重建唯一约束：原表级 UNIQUE (student_id, practice_mode) 约束名由 PG 自动生成
ALTER TABLE practice_progress DROP CONSTRAINT practice_progress_student_id_practice_mode_key;

-- 证件分区唯一（NULL 不参与判重）
CREATE UNIQUE INDEX uq_practice_progress_cred
  ON practice_progress (student_id, practice_mode, credential_id)
  WHERE credential_id IS NOT NULL;

-- 未分区兜底：保持旧口径唯一（同学员同模式仅一行）
CREATE UNIQUE INDEX uq_practice_progress_nocred
  ON practice_progress (student_id, practice_mode)
  WHERE credential_id IS NULL;
