-- 真题套卷模型（ADR-0022）：套卷 + 卷题关联 + 模拟考归卷 + 来源标记标签语义

-- 来源标记标签（如真题）：题目不参与公共抽题池，标签不出现在专项练习列表
ALTER TABLE question_tag
    ADD COLUMN IF NOT EXISTS is_source_tag BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN question_tag.is_source_tag IS '来源标记标签（如真题）：true 时题目不进顺序/随机/专项练习与模拟考抽题池，标签不出现在专项练习标签列表';
UPDATE question_tag SET is_source_tag = TRUE WHERE code = 'real_exam';

-- 套卷表：内容由导入工具按来源文件 upsert（幂等键 credential_id + source_ref）
CREATE TABLE IF NOT EXISTS real_exam_paper (
    paper_id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    credential_id INT NOT NULL REFERENCES credential(id) ON DELETE CASCADE,
    title         VARCHAR(200) NOT NULL,
    year          INT,
    source        VARCHAR(100),
    duration_minutes INT NOT NULL DEFAULT 90,
    level_id      INT REFERENCES course_level(level_id) ON DELETE SET NULL,
    source_ref    VARCHAR(255) NOT NULL,
    question_count INT NOT NULL DEFAULT 0,
    status        SMALLINT NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (credential_id, source_ref)
);
CREATE INDEX IF NOT EXISTS idx_real_exam_paper_credential ON real_exam_paper (credential_id, status);
COMMENT ON TABLE real_exam_paper IS '真题套卷表（导入工具按真题源文件生成）';
COMMENT ON COLUMN real_exam_paper.level_id IS '难度（复用课程等级字典），导入暂无数据可空';
COMMENT ON COLUMN real_exam_paper.source_ref IS '来源文件相对路径（导入幂等键）';
COMMENT ON COLUMN real_exam_paper.status IS '状态：1-上架，0-下架';

-- 卷题关联：跨卷同题干在导入去重后折叠为同一题，套卷与题目为多对多
CREATE TABLE IF NOT EXISTS real_exam_paper_question (
    paper_id    INT NOT NULL REFERENCES real_exam_paper(paper_id) ON DELETE CASCADE,
    question_id INT NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    order_num   INT NOT NULL DEFAULT 0,
    PRIMARY KEY (paper_id, question_id)
);
CREATE INDEX IF NOT EXISTS idx_real_exam_paper_question_question ON real_exam_paper_question (question_id);
COMMENT ON TABLE real_exam_paper_question IS '真题卷-题目关联（order_num 维持卷内题序）';

-- 模拟考记录归卷（真题卷开考的整卷考试）
ALTER TABLE mock_exam
    ADD COLUMN IF NOT EXISTS paper_id INT REFERENCES real_exam_paper(paper_id) ON DELETE SET NULL;
COMMENT ON COLUMN mock_exam.paper_id IS '来源真题卷（按卷开考时写入，随机模拟考为 NULL）';
