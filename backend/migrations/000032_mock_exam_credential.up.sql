-- #1003 模考历史按证件分区：mock_exam 落 credential_id。
--
-- 背景：模考抽题自 #702 起按当前证件分区（sampleQuestions 传证件），但**记录本身没落分区**，
-- 而历史读面（GetHistory）只按 student_id + status 过滤 —— 切到别的证件后仍能看到全部证件的
-- 模考记录，与 CONTEXT.md「当前证件 = 全局过滤器」的领域约定不符。
--
-- 存量回填两阶段（函数保留不删：幂等——无待回填行时零影响，pg 契约测试可插旧样本后重放验证）：
--   阶段一（精确）按试卷题目反推：开考即按单证件抽题，故一份卷子的题目集内证件唯一，
--     COUNT(DISTINCT credential_id) = 1 才落；跨证件/未分区题目一律留 NULL；
--   阶段二（兜底）题目反推不出的剩余行（题目已删 / 未分区池开考）挂学员当前证件 ——
--     与 000013 练习进度回填「其余证件分区从其视角重新起算」同口径；学员无当前证件则留 NULL。
--
-- 读面口径：非 NULL 证件按当前证件严格分区；NULL = 未分区（未选证件时开考 / 无法归属的存量），
-- 未选证件的读面不分区、看全部 —— 与错题本、题库池的既有口径一致，不会出现「历史凭空消失」。

ALTER TABLE mock_exam ADD COLUMN IF NOT EXISTS credential_id INT REFERENCES credential(id) ON DELETE SET NULL;
COMMENT ON COLUMN mock_exam.credential_id IS '本次模拟考试所属的目标证件分区（#1003）；NULL = 未分区（未选证件时开考，兼容存量）';

-- 历史列表查询：WHERE student_id + status + credential_id，ORDER BY created_at DESC
CREATE INDEX IF NOT EXISTS idx_mock_exam_student_credential ON mock_exam (student_id, credential_id, created_at DESC);

CREATE OR REPLACE FUNCTION backfill_mock_exam_credential() RETURNS void AS $$
BEGIN
  -- 阶段一：按试卷题目反推证件（题目集内证件唯一才落）
  UPDATE mock_exam m
  SET credential_id = sub.cred
  FROM (
    SELECT m2.id AS exam_id, MIN(q.credential_id) AS cred
    FROM mock_exam m2
    CROSS JOIN LATERAL jsonb_array_elements_text(m2.question_ids) AS t(qid)
    JOIN question q ON q.id = t.qid::int
    WHERE m2.credential_id IS NULL AND q.credential_id IS NOT NULL
    GROUP BY m2.id
    HAVING COUNT(DISTINCT q.credential_id) = 1
  ) sub
  WHERE m.id = sub.exam_id;

  -- 阶段二：其余仍为 NULL 的行挂学员当前证件（无当前证件的学员留 NULL）
  UPDATE mock_exam m
  SET credential_id = u.current_credential_id
  FROM hrwai_users u
  WHERE m.student_id = u.id
    AND m.credential_id IS NULL
    AND u.current_credential_id IS NOT NULL;
END;
$$ LANGUAGE plpgsql;

SELECT backfill_mock_exam_credential();
