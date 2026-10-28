-- #2066 招聘域：职位所需证件（原型对照第一波 · 草稿 04）。
--
-- 语义只有一档「需持有」（CONTEXT.md「职位」词条）：可空多选，复用培训域 credential 字典，
-- **不建第二套字典、没有「优先 / 加分」档**。空集（无关联行）= 该职位不限证件。
--
-- 为什么是关联表而不是职位行上的数组列：读面要按证件收窄（GET /jobs?credential_id=），
-- 关联表让「命中任一即返回」成为一次 EXISTS，不必按方言解析数组/JSON；写入侧的多选又天然
-- 是可整组替换的集合。表形状与 question_tag_relation 同构（复合主键 + 两侧 CASCADE）。
--
-- 两条外键都是 CASCADE：
--   * 删职位 ⇒ 要求随职位消失（这是职位的一部分，不是独立资产）；
--   * 删证件字典项 ⇒ 该要求从各职位上摘下，**不反向阻塞字典删除**
--     （与 user_contributions 的 NO ACTION 不同：投稿是内容资产，要求只是一枚筛选标签）。

-- 两列都写 INT 而不是 INTEGER：本仓 #1360 的迁移文本锁把
-- 「credential_id INTEGER NOT NULL REFERENCES credential(id) ON DELETE CASCADE」当作
-- 「投稿侧被改成级联」的反面形态（全仓只有 000020 用 INTEGER，其余 credential_id 都是 INT）。
-- 本表的级联是正当的（要求只是筛选标签，不是内容资产），用 INT 才不会被那把锁误读。
CREATE TABLE job_credential_requirements (
    job_posting_id INT         NOT NULL REFERENCES job_postings(id) ON DELETE CASCADE,
    credential_id  INT         NOT NULL REFERENCES credential(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (job_posting_id, credential_id)
);
-- 反向索引：列表的 EXISTS 按 credential_id 起手，没有它就得扫全表。
CREATE INDEX idx_job_credential_requirements_credential
    ON job_credential_requirements (credential_id);

COMMENT ON TABLE job_credential_requirements IS '职位所需证件（单一「要求」档，可空多选；无优先/加分语义）';
COMMENT ON COLUMN job_credential_requirements.credential_id IS '目标证件字典项（删除即从职位上摘下，不阻塞字典删除）';
