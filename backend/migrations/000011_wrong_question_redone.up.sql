-- 错题重做标记（spec 修复：重做做对后标记为已重做，批量移出）
ALTER TABLE wrong_question ADD COLUMN IF NOT EXISTS is_redone BOOLEAN NOT NULL DEFAULT FALSE;
