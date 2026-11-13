-- #2497 单挂挂接（spec B2）：文件行带章节归属，NULL=在库未挂；
-- 挂接/换挂/解挂语义见 internal/tutorfile/attach 一处，删除随引用走（B5）由该列判定。
-- FK ON DELETE SET NULL 沿 chapter_file 先例（000001:262）：章节删除时挂接行自动回库，
-- 不产生反向悬空引用（被删章的文件仍归导师、可再挂/可删）。
ALTER TABLE tutor_file ADD COLUMN IF NOT EXISTS chapter_id INT REFERENCES chapter(chapter_id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_tutor_file_chapter ON tutor_file (chapter_id);
