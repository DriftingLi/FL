-- ADR-0017：学习位置与章节完成状态（移动端 continue-learning 数据源）。
-- 复用 study_record 双轨记录模型，不建新表：
--   章节级记录（chapter_id IS NOT NULL）+ video_position：该章节最后播放位置（秒）；
--   课程级记录（chapter_id IS NULL）+ last_chapter_id / last_studied_at：最后学习章节与时间戳。
-- 章节完成仍以 progress >= 100 为单一事实源（时长自动完成 / completed 显式完成收敛于此）。
ALTER TABLE study_record ADD COLUMN IF NOT EXISTS video_position INT NOT NULL DEFAULT 0;
ALTER TABLE study_record ADD COLUMN IF NOT EXISTS last_chapter_id INT;
ALTER TABLE study_record ADD COLUMN IF NOT EXISTS last_studied_at TIMESTAMPTZ;
