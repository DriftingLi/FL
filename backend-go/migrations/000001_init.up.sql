-- 叉车维修培训系统 PostgreSQL 初始化迁移
-- 由 MySQL init.sql + Python 迁移脚本合并翻译而来
-- 类型映射: AUTO_INCREMENT -> GENERATED ALWAYS AS IDENTITY
--          DATETIME -> TIMESTAMPTZ
--          JSON -> JSONB
--          TINYINT -> SMALLINT / BOOLEAN
--          DECIMAL -> NUMERIC

-- 1. 学员表
CREATE TABLE student (
    student_id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username        VARCHAR(50)  NOT NULL UNIQUE,
    password        VARCHAR(255) NOT NULL,
    name            VARCHAR(50)  NOT NULL,
    status          SMALLINT     NOT NULL DEFAULT 1,
    level           VARCHAR(20)  NOT NULL DEFAULT 'beginner',
    level_updated_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_student_username ON student (username);
COMMENT ON TABLE  student IS '学员表';
COMMENT ON COLUMN student.student_id IS '学员ID';
COMMENT ON COLUMN student.password IS '密码（BCrypt加密）';
COMMENT ON COLUMN student.status IS '状态：1-正常，0-禁用';
COMMENT ON COLUMN student.level IS '等级：beginner/intermediate/advanced';

-- 2. 管理员表
CREATE TABLE admin (
    admin_id    INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username    VARCHAR(50)  NOT NULL UNIQUE,
    password    VARCHAR(255) NOT NULL,
    name        VARCHAR(50)  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
COMMENT ON TABLE admin IS '管理员表';

-- 3. 导师表
CREATE TABLE tutor (
    tutor_id    INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username    VARCHAR(50)  NOT NULL UNIQUE,
    password    VARCHAR(255) NOT NULL,
    name        VARCHAR(50)  NOT NULL,
    status      SMALLINT     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
COMMENT ON TABLE tutor IS '导师表';

-- 4. 课程表
CREATE TABLE course (
    course_id   INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    category    VARCHAR(20)  NOT NULL,
    description TEXT,
    cover_image VARCHAR(255),
    duration    INT          NOT NULL DEFAULT 0,
    status      SMALLINT     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_course_category ON course (category);
CREATE INDEX idx_course_status   ON course (status);
COMMENT ON TABLE  course IS '课程表';
COMMENT ON COLUMN course.category IS '分类：CATEGORY_01-基础理论, CATEGORY_02-安全规范, CATEGORY_03-实操技能, CATEGORY_04-进阶提升';
COMMENT ON COLUMN course.status   IS '状态：1-上架，0-下架';

-- 5. 章节表
CREATE TABLE chapter (
    chapter_id   INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    course_id    INT           NOT NULL REFERENCES course(course_id) ON DELETE CASCADE,
    title        VARCHAR(200)  NOT NULL,
    content      TEXT,
    content_url  VARCHAR(255),
    content_type VARCHAR(20)   NOT NULL DEFAULT 'text',
    file_url     VARCHAR(500),
    description  TEXT,
    duration     INT           NOT NULL DEFAULT 0,
    order_num    INT           NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_chapter_course_id ON chapter (course_id);
COMMENT ON TABLE chapter IS '章节表';
COMMENT ON COLUMN chapter.content_type IS '内容类型：text/document/slide';

-- 6. 章节文件表
CREATE TABLE chapter_file (
    file_id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    chapter_id   INT           REFERENCES chapter(chapter_id) ON DELETE SET NULL,
    file_url     VARCHAR(500)  NOT NULL,
    file_name    VARCHAR(255),
    content_type VARCHAR(50)   NOT NULL DEFAULT 'document',
    file_size    BIGINT        NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_chapter_file_chapter ON chapter_file (chapter_id);
COMMENT ON TABLE chapter_file IS '章节文件表';

-- 7. 学习记录表
CREATE TABLE study_record (
    record_id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id     INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    course_id      INT          NOT NULL REFERENCES course(course_id) ON DELETE CASCADE,
    chapter_id     INT,
    study_duration INT          NOT NULL DEFAULT 0,
    progress       NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    study_date     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_study_record_student_course_chapter ON study_record (student_id, course_id, chapter_id);
CREATE INDEX idx_study_record_study_date             ON study_record (study_date);
COMMENT ON TABLE study_record IS '学习记录表';

-- 8. 考核记录表
CREATE TABLE exam_record (
    exam_id   INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    course_id  INT          NOT NULL REFERENCES course(course_id) ON DELETE CASCADE,
    score      NUMERIC(5,2),
    answers    JSONB,
    exam_date  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_exam_record_student_course ON exam_record (student_id, course_id);
COMMENT ON TABLE exam_record IS '考核记录表';

-- 9. AI 生成记录表
CREATE TABLE ai_generation_log (
    log_id          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         INT          NOT NULL,
    user_type       VARCHAR(20)  NOT NULL,
    generation_type VARCHAR(20)  NOT NULL,
    input_params    JSONB,
    output_result   TEXT,
    status          SMALLINT     NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_log_user_id         ON ai_generation_log (user_id);
CREATE INDEX idx_ai_log_generation_type ON ai_generation_log (generation_type);
COMMENT ON TABLE ai_generation_log IS 'AI生成记录表';

-- 10. 知识点表
CREATE TABLE knowledge_point (
    id          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    level       VARCHAR(20)  NOT NULL,
    parent_id   INT          REFERENCES knowledge_point(id) ON DELETE SET NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_kp_level  ON knowledge_point (level);
CREATE INDEX idx_kp_parent ON knowledge_point (parent_id);
COMMENT ON TABLE knowledge_point IS '知识点表';

-- 11. 题目表
CREATE TABLE question (
    id                  INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type                VARCHAR(20)  NOT NULL,
    level               VARCHAR(20)  NOT NULL,
    content             TEXT         NOT NULL,
    options             JSONB,
    answer              VARCHAR(255) NOT NULL,
    explanation         TEXT,
    image_url           VARCHAR(500),
    reference_answer    TEXT,
    scoring_criteria    TEXT,
    score               INT          NOT NULL DEFAULT 0,
    knowledge_point_id  INT          REFERENCES knowledge_point(id) ON DELETE SET NULL,
    status              VARCHAR(20)  NOT NULL DEFAULT 'draft',
    created_by          INT,
    created_by_type     VARCHAR(20)  NOT NULL DEFAULT 'tutor',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_question_type             ON question (type);
CREATE INDEX idx_question_level            ON question (level);
CREATE INDEX idx_question_status           ON question (status);
CREATE INDEX idx_question_knowledge_point  ON question (knowledge_point_id);
COMMENT ON TABLE  question IS '题目表';
COMMENT ON COLUMN question.type IS '题型：single_choice/multi_choice/true_false/fault_image/short_answer';
COMMENT ON COLUMN question.status IS '状态：draft/pending/published';

-- 12. 考试场次表
CREATE TABLE exam_session (
    id              INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            VARCHAR(200) NOT NULL,
    level           VARCHAR(20)  NOT NULL,
    start_time      TIMESTAMPTZ  NOT NULL,
    end_time        TIMESTAMPTZ  NOT NULL,
    duration        INT          NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'upcoming',
    created_by      INT,
    question_config JSONB,
    total_score     INT          NOT NULL DEFAULT 0,
    pass_score      INT          NOT NULL DEFAULT 60,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_exam_session_level  ON exam_session (level);
CREATE INDEX idx_exam_session_status ON exam_session (status);
COMMENT ON TABLE exam_session IS '考试场次表';

-- 13. 考试参与记录表
CREATE TABLE exam_participant (
    id               INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    exam_session_id  INT          NOT NULL REFERENCES exam_session(id) ON DELETE CASCADE,
    student_id       INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    status           VARCHAR(20)  NOT NULL DEFAULT 'not_started',
    start_time       TIMESTAMPTZ,
    submit_time      TIMESTAMPTZ,
    remaining_time   INT          NOT NULL DEFAULT 0,
    score            NUMERIC(5,2),
    objective_score  NUMERIC(5,2),
    subjective_score NUMERIC(5,2),
    is_passed        BOOLEAN      NOT NULL DEFAULT FALSE,
    answers_snapshot JSONB,
    question_ids     JSONB,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_exam_participant_session_student ON exam_participant (exam_session_id, student_id);
CREATE INDEX idx_exam_participant_student                ON exam_participant (student_id);
COMMENT ON TABLE exam_participant IS '考试参与记录表';

-- 14. 考试答题记录表
CREATE TABLE exam_answer (
    id                  INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    exam_participant_id INT          NOT NULL REFERENCES exam_participant(id) ON DELETE CASCADE,
    question_id         INT          NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    user_answer         TEXT,
    is_correct          BOOLEAN,
    score               NUMERIC(5,2) NOT NULL DEFAULT 0,
    grader_id           INT,
    graded_at           TIMESTAMPTZ,
    grading_comment     TEXT,
    ai_score            NUMERIC(5,2),
    ai_comment          TEXT,
    ai_graded_at        TIMESTAMPTZ
);
CREATE INDEX idx_exam_answer_participant ON exam_answer (exam_participant_id);
CREATE INDEX idx_exam_answer_question    ON exam_answer (question_id);
COMMENT ON TABLE exam_answer IS '考试答题记录表';

-- 15. 题库练习记录表
CREATE TABLE question_practice_record (
    id            INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id    INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    question_id   INT          NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    level         VARCHAR(20)  NOT NULL,
    is_correct    BOOLEAN      NOT NULL DEFAULT FALSE,
    practice_type VARCHAR(20)  NOT NULL DEFAULT 'free',
    user_answer   TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_qpr_student  ON question_practice_record (student_id);
CREATE INDEX idx_qpr_question ON question_practice_record (question_id);
CREATE INDEX idx_qpr_created  ON question_practice_record (created_at);
COMMENT ON TABLE question_practice_record IS '题库练习记录表';

-- 17. 错题记录表
CREATE TABLE wrong_question (
    id             INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id     INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    question_id    INT          NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    wrong_count    INT          NOT NULL DEFAULT 1,
    last_wrong_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    is_removed     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_wrong_question_student_question ON wrong_question (student_id, question_id);
CREATE INDEX        idx_wrong_question_student          ON wrong_question (student_id);
COMMENT ON TABLE wrong_question IS '错题记录表';

-- 18. 模拟考试表
CREATE TABLE mock_exam (
    id            INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id    INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    level         VARCHAR(20)  NOT NULL,
    question_ids  JSONB,
    answers       JSONB,
    start_time    TIMESTAMPTZ,
    submit_time   TIMESTAMPTZ,
    remaining_time INT         NOT NULL DEFAULT 0,
    duration      INT          NOT NULL DEFAULT 90,
    score         NUMERIC(5,2),
    status        VARCHAR(20)  NOT NULL DEFAULT 'not_started',
    result        JSONB,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_mock_exam_student ON mock_exam (student_id);
CREATE INDEX idx_mock_exam_level   ON mock_exam (level);
COMMENT ON TABLE mock_exam IS '模拟考试表';

-- 19. 异步任务表
CREATE TABLE async_task (
    id         INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_type  VARCHAR(50)  NOT NULL,
    status     VARCHAR(20)  NOT NULL DEFAULT 'pending',
    payload    JSONB,
    result     JSONB,
    error      TEXT,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_async_task_status ON async_task (status);
COMMENT ON TABLE async_task IS '异步任务表';
