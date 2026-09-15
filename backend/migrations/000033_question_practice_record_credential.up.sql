-- #1007 / ADR-0051 练习记录的证件分区在写入时冻结：question_practice_record 落 credential_id。
--
-- 背景：练习记录一直无分区列，三个读面各走各的 —— practice-stats 读时 JOIN question 按题目当前归属过滤，
-- /practice-mode/history 与 legacy /practice-mode/stats 完全不分区。生产实测的缺陷形态：某学员（当前证件=
-- 低压电工）15 条记录全在别的证件下 → 可达的「数据报告」页总览报 0、by_type 明细报 15，同页自相矛盾。
--
-- 决定（ADR-0051）：学员行为事实的证件分区在写入时冻结 —— 记录落「作答那一刻的当前证件」，读面按记录上的
-- 分区过滤，nil = 不分区（未选证件时看全部）。练习进度（#414）与模考记录（#1005）已是此形态。
--
-- 存量回填：按题目反推（练习记录必然指向一道题，题目分区单归属）。生产实测 22 行、0 行落在未分区题目上，
-- 故无兜底分支；题目被删时记录随题级联删除（baseline DDL: ON DELETE CASCADE），不存在无主行。
-- 函数保留不删：幂等（无待回填行时零影响），pg 契约测试可插旧样本后重放验证。

ALTER TABLE question_practice_record ADD COLUMN IF NOT EXISTS credential_id INT REFERENCES credential(id) ON DELETE SET NULL;
COMMENT ON COLUMN question_practice_record.credential_id IS '该次作答所属的目标证件分区（#1007 / ADR-0051）；作答那一刻的当前证件，写入时冻结；NULL = 未分区（未选证件时作答，兼容存量）';

-- 三个读面的查询形态：WHERE student_id + credential_id（+ 可选 type/日期），ORDER BY created_at DESC
CREATE INDEX IF NOT EXISTS idx_qpr_student_credential ON question_practice_record (student_id, credential_id, created_at DESC);

CREATE OR REPLACE FUNCTION backfill_qpr_credential() RETURNS void AS $$
BEGIN
  UPDATE question_practice_record r
  SET credential_id = q.credential_id
  FROM question q
  WHERE q.id = r.question_id
    AND r.credential_id IS NULL
    AND q.credential_id IS NOT NULL;
END;
$$ LANGUAGE plpgsql;

SELECT backfill_qpr_credential();
