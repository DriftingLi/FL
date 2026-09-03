-- #510 断点回跳修复的存量归并：顺序练习 NULL 孤儿行并回当前证件桶。
--
-- 背景（#505 修复前）：保存进度走 POST /practice-mode/progress，其 credential_id 只从
-- JSON body 解析，而前端从未携带——#414 分桶后，已预筛选学员的顺序练习游标不断写进
-- credential_id IS NULL 的孤儿行，其证件桶行游标冻结在 000013 回填时的旧值（断点回跳）。
--
-- 归并语义（与 #504 游标只进不退方向一致）：
--   - 仅处理「学员有 current_credential_id」的 sequential 孤儿行；
--   - 游标取 max（孤儿行是后写真实进度，桶行冻结旧值，max 必然拿到最新断点）；
--   - answers_state 按题目 ID 合并：桶行为底、孤儿行覆盖同键（孤儿后写为准）；
--   - total 取 max、question_ids 保留桶行（顺序练习恒用现集合，000013 后 ResumeSet
--     已按现池刷新，桶行顺序即最新）；
--   - 合并后删除孤儿行（uq_practice_progress_nocred 不再拦截）；
--   - 无 current_credential 学员的 NULL 行是唯一合法行，原样不动。
--
-- 归并逻辑收敛为存储函数 merge_orphan_practice_progress()，迁移执行一次；
-- 函数保留不删——幂等（无孤儿时零影响），pg 契约测试可插旧样本后重放验证归并行为。

CREATE OR REPLACE FUNCTION merge_orphan_practice_progress() RETURNS void AS $$
BEGIN
  -- 1) 桶行存在：并入 max 游标与合并 answers（桶底 + 孤儿覆盖同键）
  UPDATE practice_progress b
  SET current_index = o.merged_index,
      total = o.merged_total,
      answers_state = o.merged_answers,
      updated_at = now()
  FROM (
    SELECT o.student_id, u.current_credential_id AS cred_id,
           GREATEST(o.current_index, b2.current_index) AS merged_index,
           COALESCE(b2.answers_state, '{}'::jsonb) || COALESCE(o.answers_state, '{}'::jsonb) AS merged_answers,
           GREATEST(o.total, b2.total) AS merged_total
    FROM practice_progress o
    JOIN hrwai_users u ON u.id = o.student_id AND u.current_credential_id IS NOT NULL
    JOIN practice_progress b2
      ON b2.student_id = o.student_id
     AND b2.practice_mode = o.practice_mode
     AND b2.credential_id = u.current_credential_id
    WHERE o.practice_mode = 'sequential' AND o.credential_id IS NULL
  ) o
  WHERE b.student_id = o.student_id
    AND b.practice_mode = 'sequential'
    AND b.credential_id = o.cred_id;

  -- 2) 桶行不存在（孤儿行是该学员唯一行）：直接改挂证件分区
  UPDATE practice_progress o
  SET credential_id = u.current_credential_id
  FROM hrwai_users u
  WHERE o.student_id = u.id
    AND u.current_credential_id IS NOT NULL
    AND o.practice_mode = 'sequential'
    AND o.credential_id IS NULL
    AND NOT EXISTS (
      SELECT 1 FROM practice_progress b
      WHERE b.student_id = o.student_id
        AND b.practice_mode = 'sequential'
        AND b.credential_id = u.current_credential_id
    );

  -- 3) 删除已并入桶的孤儿行
  DELETE FROM practice_progress o
  USING hrwai_users u
  WHERE o.student_id = u.id
    AND u.current_credential_id IS NOT NULL
    AND o.practice_mode = 'sequential'
    AND o.credential_id IS NULL
    AND EXISTS (
      SELECT 1 FROM practice_progress b
      WHERE b.student_id = o.student_id
        AND b.practice_mode = 'sequential'
        AND b.credential_id = u.current_credential_id
    );
END;
$$ LANGUAGE plpgsql;

SELECT merge_orphan_practice_progress();
