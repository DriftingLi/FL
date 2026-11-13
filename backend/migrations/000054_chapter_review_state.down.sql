-- #2498 回退：删版本槽两表与章节审核列（存量豁免的已发布槽随之丢弃——回退即回到无审核语义）。
DROP FUNCTION IF EXISTS backfill_chapter_review_state();
DROP TABLE IF EXISTS chapter_file_slot;
DROP TABLE IF EXISTS chapter_content_slot;
ALTER TABLE chapter DROP COLUMN IF EXISTS reject_reason;
ALTER TABLE chapter DROP COLUMN IF EXISTS review_status;
