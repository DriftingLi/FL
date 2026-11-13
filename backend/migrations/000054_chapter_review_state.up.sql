-- #2498 章节先审后发状态机（spec C1/C4）：working 草稿留在 chapter 行，
-- 在审快照（slot='pending'）与已发布槽（slot='published'）各占一行——
-- 过审＝pending→published 原子改槽（C5 旧版在切换前保留），驳回＝删 pending（弃快照）。
-- 冻结单元＝正文 + 文件挂接清单（C3；清单在 chapter_file_slot）。
ALTER TABLE chapter ADD COLUMN IF NOT EXISTS review_status TEXT NOT NULL DEFAULT 'draft';
ALTER TABLE chapter ADD COLUMN IF NOT EXISTS reject_reason TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS chapter_content_slot (
    slot         TEXT NOT NULL,
    chapter_id   INT  NOT NULL REFERENCES chapter(chapter_id) ON DELETE CASCADE,
    content      TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT 'text',
    file_url     TEXT NOT NULL DEFAULT '',
    submitted_by INT  NOT NULL DEFAULT 0,
    frozen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (slot, chapter_id)
);
COMMENT ON TABLE chapter_content_slot IS '章节内容版本槽（在审快照 pending / 已发布槽 published）';

CREATE TABLE IF NOT EXISTS chapter_file_slot (
    slot         TEXT NOT NULL,
    chapter_id   INT  NOT NULL REFERENCES chapter(chapter_id) ON DELETE CASCADE,
    source       TEXT NOT NULL,
    file_id      INT  NOT NULL,
    file_url     TEXT NOT NULL DEFAULT '',
    file_name    TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT 'document',
    file_size    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (slot, chapter_id, source, file_id)
);
COMMENT ON TABLE chapter_file_slot IS '章节文件挂接清单槽（提交审核时冻结，学员读 published）';
CREATE INDEX IF NOT EXISTS idx_chapter_file_slot_file ON chapter_file_slot (source, file_id);

-- C12 存量一次性豁免：上线时存量章节视为已过审——置已发布槽、不入队列。
-- 回填做成可重放函数（沿 000032 backfill_mock_exam_credential / 000033 backfill_qpr_credential
-- 先例）：迁移内调用一次；契约测试插入「迁移前形状」样本后重放验证三源冻结语义与幂等。
CREATE OR REPLACE FUNCTION backfill_chapter_review_state() RETURNS void AS $$
BEGIN
  UPDATE chapter SET review_status = 'published' WHERE review_status <> 'published';

  INSERT INTO chapter_content_slot (slot, chapter_id, content, content_type, file_url, frozen_at)
  SELECT 'published', c.chapter_id, COALESCE(c.content, ''), COALESCE(c.content_type, ''),
         COALESCE(c.file_url, ''), now()
  FROM chapter c
  WHERE NOT EXISTS (SELECT 1 FROM chapter_content_slot s WHERE s.slot = 'published' AND s.chapter_id = c.chapter_id);

  -- 存量文件清单按 live 读面三源冻结（source 判别沿 #2497）：
  -- chapter_file 行 / tutor_file 挂接行 / legacy 折叠（无 chapter_file 行且 chapter.file_url 非空）。
  -- 历史行的 URL/名称/类型列可能为 NULL（迁移 000001 的 chapter.file_url 可空），槽表这三列
  -- 是 NOT NULL ⇒ 一律 COALESCE，否则空库重建 baseline 会因存量 NULL 直接失败。
  INSERT INTO chapter_file_slot (slot, chapter_id, source, file_id, file_url, file_name, content_type, file_size, created_at)
  SELECT 'published', cf.chapter_id, 'chapter_file', cf.file_id, COALESCE(cf.file_url, ''),
         COALESCE(cf.file_name, ''), COALESCE(cf.content_type, 'document'), COALESCE(cf.file_size, 0),
         COALESCE(cf.created_at, now())
  FROM chapter_file cf
  WHERE cf.chapter_id IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM chapter_file_slot s WHERE s.slot = 'published' AND s.chapter_id = cf.chapter_id AND s.source = 'chapter_file' AND s.file_id = cf.file_id);

  INSERT INTO chapter_file_slot (slot, chapter_id, source, file_id, file_url, file_name, content_type, file_size, created_at)
  SELECT 'published', tf.chapter_id, 'tutor_file', tf.file_id, COALESCE(tf.file_url, ''),
         COALESCE(tf.file_name, ''), COALESCE(tf.content_type, 'document'), COALESCE(tf.file_size, 0),
         COALESCE(tf.created_at, now())
  FROM tutor_file tf
  WHERE tf.chapter_id IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM chapter_file_slot s WHERE s.slot = 'published' AND s.chapter_id = tf.chapter_id AND s.source = 'tutor_file' AND s.file_id = tf.file_id);

  INSERT INTO chapter_file_slot (slot, chapter_id, source, file_id, file_url, file_name, content_type, file_size, created_at)
  SELECT 'published', c.chapter_id, 'legacy', 0, c.file_url,
         regexp_replace(c.file_url, '^.*/', ''), COALESCE(NULLIF(c.content_type, ''), 'document'), 0,
         COALESCE(c.created_at, now())
  FROM chapter c
  WHERE c.file_url IS NOT NULL AND c.file_url <> ''
    AND NOT EXISTS (SELECT 1 FROM chapter_file cf WHERE cf.chapter_id = c.chapter_id)
    AND NOT EXISTS (SELECT 1 FROM chapter_file_slot s WHERE s.slot = 'published' AND s.chapter_id = c.chapter_id AND s.source = 'legacy');
END;
$$ LANGUAGE plpgsql;

SELECT backfill_chapter_review_state();
