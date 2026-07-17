-- ============================================================
-- 叉车维修培训系统 PostgreSQL 初始化 baseline（squash 000001~000017）
-- 由 17 个迁移文件 squash 而成，等价于原 17 个迁移按序执行后的最终态
-- 类型映射: AUTO_INCREMENT -> GENERATED ALWAYS AS IDENTITY
--          DATETIME -> TIMESTAMPTZ
--          JSON -> JSONB
--          TINYINT -> SMALLINT / BOOLEAN
--          DECIMAL -> NUMERIC
-- ============================================================

-- =====================================================
-- Part 1: 学员/题库/考试相关表（来自 000001 + 000013）
-- =====================================================

-- 1. 学员表（含 000013 的 phone/email/company 字段）
CREATE TABLE student (
    student_id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username        VARCHAR(50)  NOT NULL UNIQUE,
    password        VARCHAR(255) NOT NULL,
    name            VARCHAR(50)  NOT NULL,
    status          SMALLINT     NOT NULL DEFAULT 1,
    phone           VARCHAR(20)  NOT NULL,
    email           VARCHAR(100),
    company         VARCHAR(100),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_student_username ON student (username);
CREATE UNIQUE INDEX idx_student_phone ON student (phone);
COMMENT ON TABLE  student IS '学员表';
COMMENT ON COLUMN student.student_id IS '学员ID';
COMMENT ON COLUMN student.password IS '密码（BCrypt加密）';
COMMENT ON COLUMN student.status IS '状态：1-正常，0-禁用';
COMMENT ON COLUMN student.phone   IS '手机号（注册时必填，同时作为自动生成的用户名）';
COMMENT ON COLUMN student.email   IS '邮箱（选填）';
COMMENT ON COLUMN student.company IS '单位/公司（选填）';

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
    category    VARCHAR(32),
    parent_id   INT          REFERENCES knowledge_point(id) ON DELETE SET NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_kp_category ON knowledge_point (category);
CREATE INDEX idx_kp_parent   ON knowledge_point (parent_id);
COMMENT ON TABLE  knowledge_point IS '知识点表';
COMMENT ON COLUMN knowledge_point.category IS '课程分类：CATEGORY_01-基础理论, CATEGORY_02-安全规范, CATEGORY_03-实操技能, CATEGORY_04-进阶提升';

-- 11. 题目表
CREATE TABLE question (
    id                  INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type                VARCHAR(20)  NOT NULL,
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
    reject_reason       TEXT,
    created_by          INT,
    created_by_type     VARCHAR(20)  NOT NULL DEFAULT 'tutor',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_question_type             ON question (type);
CREATE INDEX idx_question_status           ON question (status);
CREATE INDEX idx_question_knowledge_point  ON question (knowledge_point_id);
COMMENT ON TABLE  question IS '题目表';
COMMENT ON COLUMN question.type IS '题型：single_choice/multi_choice/true_false/fault_image/short_answer';
COMMENT ON COLUMN question.status IS '状态：draft/pending/published';
COMMENT ON COLUMN question.reject_reason IS '驳回理由（管理员驳回时填写，导师修改重提后清空）';

-- 12. 考试场次表
CREATE TABLE exam_session (
    id              INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            VARCHAR(200) NOT NULL,
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

-- 20. 练习进度表（断点续练游标 + 答题状态持久化，来自 000002）
CREATE TABLE practice_progress (
    id            INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    student_id    INT          NOT NULL REFERENCES student(student_id) ON DELETE CASCADE,
    practice_mode VARCHAR(32)  NOT NULL,
    question_ids  JSONB        NOT NULL,
    current_index INT          NOT NULL DEFAULT 0,
    total         INT          NOT NULL DEFAULT 0,
    answers_state JSONB        NOT NULL DEFAULT '{}'::jsonb,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (student_id, practice_mode)
);
CREATE INDEX idx_practice_progress_student ON practice_progress (student_id);
COMMENT ON TABLE practice_progress IS '练习进度表（断点续练游标 + 答题状态持久化）';
COMMENT ON COLUMN practice_progress.answers_state IS '每题作答状态 JSONB，key 为 question_id，value 含 user_answer/is_correct/correct_answer/explanation 等';

-- =====================================================
-- Part 2: 残值评估相关表（最终态）
-- =====================================================

-- 1. 品牌表（最终态：删除了 tier/models/brand_type 列）
CREATE TABLE brands (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(50) UNIQUE NOT NULL,
    k_brand     DECIMAL(4,2) NOT NULL,
    is_active   BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMP DEFAULT NOW()
);

-- 2. 车型表（含 000007 的 earliest_factory_year）
CREATE TABLE vehicle_types (
    id                      SERIAL PRIMARY KEY,
    name                    VARCHAR(50) UNIQUE NOT NULL,
    power_type              VARCHAR(20) NOT NULL,
    earliest_factory_year   INTEGER NOT NULL DEFAULT 1980
);

-- 3. 系列表（含 000009 的 earliest_factory_year）
CREATE TABLE series (
    id                      SERIAL PRIMARY KEY,
    brand                   VARCHAR(50) NOT NULL,
    name                    VARCHAR(50) NOT NULL,
    earliest_factory_year   INTEGER NOT NULL DEFAULT 2000,
    UNIQUE(brand, name)
);

-- 4. 吨位表
CREATE TABLE tonnages (
    id      SERIAL PRIMARY KEY,
    value   DECIMAL(5,2) UNIQUE NOT NULL
);

-- 5. 门架类型表
CREATE TABLE mast_types (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(50) UNIQUE NOT NULL
);

-- 6. 门架高度表
CREATE TABLE mast_heights (
    id          SERIAL PRIMARY KEY,
    value_mm    INTEGER UNIQUE NOT NULL
);

-- 7. 电池类型表
CREATE TABLE battery_types (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(50) UNIQUE NOT NULL
);

-- 8. 车况评级表
CREATE TABLE condition_ratings (
    id                  SERIAL PRIMARY KEY,
    rating              VARCHAR(10) UNIQUE NOT NULL,
    label               VARCHAR(20) NOT NULL,
    base_coefficient    DECIMAL(5,4) NOT NULL
);

-- 9. 区域系数表
CREATE TABLE region_coefficients (
    id              SERIAL PRIMARY KEY,
    province        VARCHAR(50) NOT NULL,
    city            VARCHAR(50) NOT NULL,
    coefficient     DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    UNIQUE(province, city)
);

-- 10. 传动类型表（来自 000010）
CREATE TABLE transmission_types (
    id    SERIAL PRIMARY KEY,
    name  VARCHAR(50) UNIQUE NOT NULL
);

-- 11. 发动机类型表（来自 000010）
CREATE TABLE engine_types (
    id    SERIAL PRIMARY KEY,
    name  VARCHAR(50) UNIQUE NOT NULL
);

-- 12. 系列配置选项表（来自 000010）
CREATE TABLE series_config_options (
    id           SERIAL PRIMARY KEY,
    brand        VARCHAR(50) NOT NULL,
    series       VARCHAR(50) NOT NULL,
    dimension    VARCHAR(20) NOT NULL,   -- 'transmission' / 'engine' / 'battery'
    option_name  VARCHAR(50) NOT NULL,
    UNIQUE (brand, series, dimension, option_name)
);
CREATE INDEX idx_sco_lookup ON series_config_options(brand, series);

-- 13. 车辆原价表（最终态：7字段唯一约束 + earliest_factory_year，无 battery_type/brand_type/is_active）
CREATE TABLE original_prices (
    id                      BIGSERIAL PRIMARY KEY,
    brand                   VARCHAR(50) NOT NULL,
    vehicle_type            VARCHAR(50) NOT NULL,
    series                  VARCHAR(50) NOT NULL,
    tonnage                 DECIMAL(5,2) NOT NULL,
    config_type             VARCHAR(50) NOT NULL,
    mast_type               VARCHAR(50) NOT NULL,
    mast_height_mm          INTEGER NOT NULL,
    original_price          DECIMAL(12,2) NOT NULL,
    earliest_factory_year   INTEGER NOT NULL DEFAULT 2000,
    updated_at              TIMESTAMP DEFAULT NOW(),
    CONSTRAINT original_prices_7field_unique
        UNIQUE (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm)
);
CREATE INDEX idx_original_prices_lookup ON original_prices(
    brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm
);

-- 14. 评估记录表（最终态：无 battery_type/brand_type）
CREATE TABLE evaluations (
    id                          BIGSERIAL PRIMARY KEY,
    brand                       VARCHAR(50) NOT NULL,
    vehicle_type                VARCHAR(50) NOT NULL,
    series                      VARCHAR(50) NOT NULL,
    tonnage                     DECIMAL(5,2) NOT NULL,
    config_type                 VARCHAR(50) NOT NULL,
    mast_type                   VARCHAR(50) NOT NULL,
    mast_height_mm              INTEGER NOT NULL,
    factory_year                INTEGER NOT NULL,
    sale_year                   INTEGER NOT NULL,
    usage_hours                 INTEGER NOT NULL,
    original_paint              BOOLEAN NOT NULL,
    province                    VARCHAR(50) NOT NULL,
    city                        VARCHAR(50) NOT NULL,
    has_license_plate           BOOLEAN NOT NULL,
    has_registration_certificate BOOLEAN NOT NULL,
    has_maintenance_records     BOOLEAN NOT NULL,
    condition_rating            VARCHAR(10) NOT NULL,
    original_price              DECIMAL(12,2) NOT NULL,
    k_time                      DECIMAL(6,4),
    k_hours                     DECIMAL(6,4),
    k_brand                     DECIMAL(6,4),
    k_condition                 DECIMAL(6,4),
    k_market                    DECIMAL(6,4),
    estimated_value             DECIMAL(12,2) NOT NULL,
    confidence_low              DECIMAL(12,2),
    confidence_high             DECIMAL(12,2),
    report_pdf_path             VARCHAR(255),
    created_at                  TIMESTAMP DEFAULT NOW(),
    updated_at                  TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_evaluations_created ON evaluations(created_at DESC);
CREATE INDEX idx_evaluations_brand ON evaluations(brand);

-- 15. 电池评估主表（来自 000002）
CREATE TABLE battery_evaluations (
    id                BIGSERIAL PRIMARY KEY,
    battery_type      VARCHAR(20) NOT NULL,
    battery_model     VARCHAR(100),
    cycle_count       INTEGER NOT NULL,
    rul_cycles        INTEGER NOT NULL,
    soh_percent       DECIMAL(5,2) NOT NULL,
    confidence        DECIMAL(4,3) NOT NULL,
    confidence_low    INTEGER,
    confidence_high   INTEGER,
    feature_importance JSONB,
    report_pdf_path   VARCHAR(255),
    created_at        TIMESTAMP DEFAULT NOW(),
    updated_at        TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_battery_evals_type    ON battery_evaluations(battery_type);
CREATE INDEX idx_battery_evals_created ON battery_evaluations(created_at DESC);

-- 16. 周期特征表（来自 000002）
CREATE TABLE battery_cycle_features (
    id              BIGSERIAL PRIMARY KEY,
    evaluation_id   BIGINT NOT NULL REFERENCES battery_evaluations(id) ON DELETE CASCADE,
    cycle_index     INTEGER NOT NULL,
    feature_vector  JSONB NOT NULL,
    raw_stats       JSONB NOT NULL,
    soh_at_cycle    DECIMAL(5,2) NOT NULL,
    UNIQUE(evaluation_id, cycle_index)
);
CREATE INDEX idx_battery_features_eval ON battery_cycle_features(evaluation_id);

-- 17. 系数配置表（来自 000002）
CREATE TABLE coefficient_configs (
    id          SERIAL PRIMARY KEY,
    key         VARCHAR(50) UNIQUE NOT NULL,
    value       DECIMAL(10,6) NOT NULL,
    description VARCHAR(255),
    updated_at  TIMESTAMP DEFAULT NOW()
);

-- =====================================================
-- Part 3: 种子数据
-- =====================================================

-- 3.1 brands（14 个品牌：000005 的 13 个 + 000008 的中力）
INSERT INTO brands (name, k_brand, is_active) VALUES
  ('林德',     1.10, TRUE),
  ('丰田',     1.08, TRUE),
  ('永恒力',   1.06, TRUE),
  ('斗山',     0.98, TRUE),
  ('海斯特',   0.96, TRUE),
  ('凯斯',     0.95, TRUE),
  ('合力',     1.00, TRUE),
  ('杭叉',     1.00, TRUE),
  ('比亚迪',   1.02, TRUE),
  ('龙工',     0.94, TRUE),
  ('柳工',     0.93, TRUE),
  ('中联重科', 0.92, TRUE),
  ('宝骊',     0.88, TRUE),
  ('中力',     0.86, TRUE)
ON CONFLICT DO NOTHING;

-- 3.2 vehicle_types（6 个车型，含 earliest_factory_year）
INSERT INTO vehicle_types (name, power_type, earliest_factory_year) VALUES
  ('电动平衡重式',   'electric',   1995),
  ('电动前移式',     'electric',   2000),
  ('电动托盘搬运车', 'electric',   1998),
  ('电动堆高车',     'electric',   2002),
  ('内燃平衡重式',   'combustion', 1985),
  ('内燃重型叉车',   'combustion', 1990)
ON CONFLICT (name) DO NOTHING;

-- 3.3 series（69 个 series，含 earliest_factory_year，已应用 000012 合并规则与"无"→"其它"）
INSERT INTO series (brand, name, earliest_factory_year) VALUES
  -- 林德
  ('林德', 'E系列', 2015), ('林德', 'Xi系列', 2018), ('林德', 'H系列', 2006),
  ('林德', 'T-MATIC', 2010), ('林德', 'L系列', 2015), ('林德', 'R系列', 2010),
  ('林德', '其它', 1980),
  -- 丰田
  ('丰田', '8FBE系列', 2013), ('丰田', '8FD系列', 2010), ('丰田', 'Traigo系列', 2016),
  ('丰田', '7系列（7FD）', 1999), ('丰田', 'Z系列（本土化）', 2010),
  ('丰田', '8系列（8FBN）', 2011), ('丰田', '8系列（8FBR）', 2015),
  ('丰田', '其它', 1980),
  -- 永恒力
  ('永恒力', 'EFG系列', 2010), ('永恒力', 'ETV系列', 2012), ('永恒力', 'ERIC系列', 2014),
  ('永恒力', '其它', 1980),
  -- 合力
  ('合力', 'A系列', 2005), ('合力', 'X系列', 2015), ('合力', 'CPCD系列', 1998),
  ('合力', 'CPD系列', 2005), ('合力', 'K系列（K1）', 2011), ('合力', 'K2系列（大力士）', 2022),
  ('合力', 'G2系列', 2020), ('合力', 'G3系列', 2023), ('合力', 'H3系列', 2020),
  ('合力', 'H4系列', 2023), ('合力', 'K3系列（锂电专用）', 2025), ('合力', '前移式', 2022),
  ('合力', '其它', 1980),
  -- 杭叉
  ('杭叉', 'A系列', 2008), ('杭叉', 'XH系列', 2018), ('杭叉', 'XC系列', 2016),
  ('杭叉', 'XE系列', 2017), ('杭叉', 'XF系列', 2019), ('杭叉', 'XA系列', 2020),
  ('杭叉', 'J系列（大吨位）', 2020), ('杭叉', 'X系列', 2019), ('杭叉', 'A系列（经济型）', 2018),
  ('杭叉', '其它', 1980),
  -- 比亚迪
  ('比亚迪', 'CPD系列', 2013), ('比亚迪', 'CPD系列（大吨位）', 2018), ('比亚迪', 'S系列', 2018),
  ('比亚迪', '其它', 1980),
  -- 斗山
  ('斗山', 'B30S系列', 2008), ('斗山', 'BR系列', 2010), ('斗山', '其它', 1980),
  -- 海斯特
  ('海斯特', 'H系列', 2006), ('海斯特', 'J系列', 2010), ('海斯特', '其它', 1980),
  -- 凯斯
  ('凯斯', '其它', 1980),
  -- 龙工
  ('龙工', 'A系列', 2008), ('龙工', 'LG系列', 2010), ('龙工', 'CPCD系列', 2012),
  ('龙工', 'N系列', 2018), ('龙工', 'CDD系列', 2019), ('龙工', '其它', 1980),
  -- 柳工
  ('柳工', 'CLG系列', 2012), ('柳工', 'E系列', 2020), ('柳工', '其它', 1980),
  -- 中联重科
  ('中联重科', 'FD系列', 2010), ('中联重科', '其它', 1980),
  -- 宝骊
  ('宝骊', 'KBE系列', 2008), ('宝骊', '其它', 1980),
  -- 中力
  ('中力', 'EPT系列', 2013), ('中力', 'ES系列', 2015), ('中力', '其它', 1980)
ON CONFLICT (brand, name) DO NOTHING;

-- 3.4 tonnages（29 个吨位）
INSERT INTO tonnages (value) VALUES
  (1.0), (1.4), (1.5), (1.6), (1.8), (2.0), (2.5), (2.8), (3.0), (3.2),
  (3.5), (3.8), (4.0), (4.5), (5.0), (5.5), (6.0), (6.5), (7.0), (7.5),
  (8.0), (8.5), (10.0), (12.0), (14.0), (16.0), (20.0), (25.0), (32.0)
ON CONFLICT (value) DO NOTHING;

-- 3.5 mast_types（8 个）
INSERT INTO mast_types (name) VALUES
  ('两级门架'), ('三级门架'), ('四级门架'), ('无'),
  ('两级标准门架'), ('两级宽视野门架'), ('三级全自由门架'), ('四级HD门架')
ON CONFLICT (name) DO NOTHING;

-- 3.6 mast_heights（16 个）
INSERT INTO mast_heights (value_mm) VALUES
  (0), (2500), (2900), (3000), (3250), (3500), (4000), (4500),
  (5000), (6000), (7000), (8000), (9500), (10000), (11300), (12000)
ON CONFLICT (value_mm) DO NOTHING;

-- 3.7 battery_types（4 个）
INSERT INTO battery_types (name) VALUES
  ('磷酸铁锂(LFP)'), ('三元锂(NCM)'), ('铅酸'), ('无')
ON CONFLICT (name) DO NOTHING;

-- 3.8 condition_ratings（5 个）
INSERT INTO condition_ratings (rating, label, base_coefficient) VALUES
  ('A', '优秀', 1.00),
  ('B', '良好', 0.90),
  ('C', '一般', 0.78),
  ('D', '较差', 0.65),
  ('E', '差',   0.50)
ON CONFLICT (rating) DO NOTHING;

-- 3.9 region_coefficients（12 个）
INSERT INTO region_coefficients (province, city, coefficient) VALUES
  ('上海', '上海', 0.98),
  ('江苏', '苏州', 0.98),
  ('江苏', '南京', 0.99),
  ('浙江', '杭州', 0.98),
  ('浙江', '宁波', 0.99),
  ('广东', '广州', 0.99),
  ('广东', '深圳', 0.97),
  ('北京', '北京', 1.02),
  ('四川', '成都', 1.05),
  ('湖北', '武汉', 1.04),
  ('山东', '青岛', 1.00),
  ('河南', '郑州', 1.05)
ON CONFLICT (province, city) DO NOTHING;

-- 3.10 transmission_types（5 个）
INSERT INTO transmission_types (name) VALUES
  ('手波'), ('自波'), ('无级变速'), ('无'), ('静压传动')
ON CONFLICT (name) DO NOTHING;

-- 3.11 engine_types（4 个）
INSERT INTO engine_types (name) VALUES
  ('国产发动机'), ('进口发动机'), ('混合动力'), ('无')
ON CONFLICT (name) DO NOTHING;

-- 3.12 series_config_options（已应用 000012 合并规则与"无"→"其它"）

-- 林德
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('林德','E系列','battery','磷酸铁锂(LFP)'),
  ('林德','E系列','battery','铅酸'),
  ('林德','E系列','battery','无'),
  ('林德','Xi系列','battery','磷酸铁锂(LFP)'),
  ('林德','Xi系列','battery','无'),
  ('林德','H系列','transmission','手波'),
  ('林德','H系列','transmission','自波'),
  ('林德','H系列','transmission','无'),
  ('林德','H系列','engine','进口发动机'),
  ('林德','H系列','engine','无'),
  ('林德','T-MATIC','transmission','自波'),
  ('林德','T-MATIC','transmission','无'),
  ('林德','T-MATIC','engine','进口发动机'),
  ('林德','T-MATIC','engine','无'),
  ('林德','L系列','battery','铅酸'),
  ('林德','L系列','battery','无'),
  ('林德','R系列','battery','铅酸'),
  ('林德','R系列','battery','无')
ON CONFLICT DO NOTHING;

-- 丰田
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('丰田','8FBE系列','battery','磷酸铁锂(LFP)'),
  ('丰田','8FBE系列','battery','铅酸'),
  ('丰田','8FBE系列','battery','无'),
  ('丰田','8FD系列','transmission','手波'),
  ('丰田','8FD系列','transmission','自波'),
  ('丰田','8FD系列','transmission','无'),
  ('丰田','8FD系列','engine','进口发动机'),
  ('丰田','8FD系列','engine','无'),
  ('丰田','Traigo系列','battery','磷酸铁锂(LFP)'),
  ('丰田','Traigo系列','battery','无'),
  ('丰田','7系列（7FD）','transmission','自波'),
  ('丰田','7系列（7FD）','transmission','无'),
  ('丰田','7系列（7FD）','engine','进口发动机'),
  ('丰田','7系列（7FD）','engine','无'),
  ('丰田','Z系列（本土化）','transmission','手波'),
  ('丰田','Z系列（本土化）','transmission','自波'),
  ('丰田','Z系列（本土化）','transmission','无'),
  ('丰田','Z系列（本土化）','engine','进口发动机'),
  ('丰田','Z系列（本土化）','engine','无'),
  ('丰田','8系列（8FBN）','battery','铅酸'),
  ('丰田','8系列（8FBN）','battery','无'),
  ('丰田','8系列（8FBE）','battery','铅酸'),
  ('丰田','8系列（8FBE）','battery','无'),
  ('丰田','8系列（8FBR）','battery','铅酸'),
  ('丰田','8系列（8FBR）','battery','无')
ON CONFLICT DO NOTHING;

-- 永恒力
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('永恒力','EFG系列','battery','磷酸铁锂(LFP)'),
  ('永恒力','EFG系列','battery','铅酸'),
  ('永恒力','EFG系列','battery','无'),
  ('永恒力','ETV系列','battery','磷酸铁锂(LFP)'),
  ('永恒力','ETV系列','battery','铅酸'),
  ('永恒力','ETV系列','battery','无'),
  ('永恒力','ERIC系列','battery','铅酸'),
  ('永恒力','ERIC系列','battery','磷酸铁锂(LFP)'),
  ('永恒力','ERIC系列','battery','无')
ON CONFLICT DO NOTHING;

-- 合力
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('合力','A系列','transmission','手波'),
  ('合力','A系列','transmission','自波'),
  ('合力','A系列','transmission','无级变速'),
  ('合力','A系列','transmission','无'),
  ('合力','A系列','engine','国产发动机'),
  ('合力','A系列','engine','进口发动机'),
  ('合力','A系列','engine','无'),
  ('合力','X系列','battery','磷酸铁锂(LFP)'),
  ('合力','X系列','battery','无'),
  ('合力','CPCD系列','transmission','手波'),
  ('合力','CPCD系列','transmission','自波'),
  ('合力','CPCD系列','transmission','无级变速'),
  ('合力','CPCD系列','transmission','无'),
  ('合力','CPCD系列','engine','国产发动机'),
  ('合力','CPCD系列','engine','进口发动机'),
  ('合力','CPCD系列','engine','无'),
  ('合力','CPD系列','battery','磷酸铁锂(LFP)'),
  ('合力','CPD系列','battery','铅酸'),
  ('合力','CPD系列','battery','无'),
  ('合力','K系列（K1）','transmission','手波'),
  ('合力','K系列（K1）','transmission','自波'),
  ('合力','K系列（K1）','transmission','无级变速'),
  ('合力','K系列（K1）','transmission','无'),
  ('合力','K系列（K1）','engine','国产发动机'),
  ('合力','K系列（K1）','engine','进口发动机'),
  ('合力','K系列（K1）','engine','无'),
  ('合力','K2系列（大力士）','transmission','自波'),
  ('合力','K2系列（大力士）','transmission','无'),
  ('合力','K2系列（大力士）','engine','国产发动机'),
  ('合力','K2系列（大力士）','engine','进口发动机'),
  ('合力','K2系列（大力士）','engine','无'),
  ('合力','G2系列','transmission','自波'),
  ('合力','G2系列','transmission','无'),
  ('合力','G2系列','engine','国产发动机'),
  ('合力','G2系列','engine','进口发动机'),
  ('合力','G2系列','engine','无'),
  ('合力','G3系列','battery','磷酸铁锂(LFP)'),
  ('合力','G3系列','battery','无'),
  ('合力','H3系列','battery','铅酸'),
  ('合力','H3系列','battery','无'),
  ('合力','H4系列','battery','磷酸铁锂(LFP)'),
  ('合力','H4系列','battery','无'),
  ('合力','K3系列（锂电专用）','battery','磷酸铁锂(LFP)'),
  ('合力','K3系列（锂电专用）','battery','无'),
  ('合力','前移式','battery','铅酸'),
  ('合力','前移式','battery','无')
ON CONFLICT DO NOTHING;

-- 杭叉
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('杭叉','A系列','transmission','手波'),
  ('杭叉','A系列','transmission','自波'),
  ('杭叉','A系列','transmission','无'),
  ('杭叉','A系列','engine','国产发动机'),
  ('杭叉','A系列','engine','进口发动机'),
  ('杭叉','A系列','engine','无'),
  ('杭叉','A系列','battery','铅酸'),
  ('杭叉','A系列','battery','无'),
  ('杭叉','XH系列','battery','磷酸铁锂(LFP)'),
  ('杭叉','XH系列','battery','无'),
  ('杭叉','XC系列','battery','磷酸铁锂(LFP)'),
  ('杭叉','XC系列','battery','无'),
  ('杭叉','XE系列','battery','磷酸铁锂(LFP)'),
  ('杭叉','XE系列','battery','铅酸'),
  ('杭叉','XE系列','battery','无'),
  ('杭叉','XF系列','transmission','手波'),
  ('杭叉','XF系列','transmission','自波'),
  ('杭叉','XF系列','transmission','无'),
  ('杭叉','XF系列','engine','国产发动机'),
  ('杭叉','XF系列','engine','进口发动机'),
  ('杭叉','XF系列','engine','无'),
  ('杭叉','XA系列','transmission','自波'),
  ('杭叉','XA系列','transmission','无'),
  ('杭叉','XA系列','engine','国产发动机'),
  ('杭叉','XA系列','engine','进口发动机'),
  ('杭叉','XA系列','engine','无'),
  ('杭叉','J系列（大吨位）','battery','铅酸'),
  ('杭叉','J系列（大吨位）','battery','无'),
  ('杭叉','X系列','battery','铅酸'),
  ('杭叉','X系列','battery','无'),
  ('杭叉','A系列（经济型）','battery','铅酸'),
  ('杭叉','A系列（经济型）','battery','无')
ON CONFLICT DO NOTHING;

-- 比亚迪
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('比亚迪','CPD系列','battery','磷酸铁锂(LFP)'),
  ('比亚迪','CPD系列','battery','无'),
  ('比亚迪','CPD系列（大吨位）','battery','磷酸铁锂(LFP)'),
  ('比亚迪','CPD系列（大吨位）','battery','无'),
  ('比亚迪','S系列','battery','铅酸'),
  ('比亚迪','S系列','battery','无')
ON CONFLICT DO NOTHING;

-- 斗山
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('斗山','B30S系列','transmission','手波'),
  ('斗山','B30S系列','transmission','自波'),
  ('斗山','B30S系列','transmission','无'),
  ('斗山','B30S系列','engine','进口发动机'),
  ('斗山','B30S系列','engine','无'),
  ('斗山','BR系列','transmission','手波'),
  ('斗山','BR系列','transmission','自波'),
  ('斗山','BR系列','transmission','无'),
  ('斗山','BR系列','engine','进口发动机'),
  ('斗山','BR系列','engine','无')
ON CONFLICT DO NOTHING;

-- 海斯特
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('海斯特','H系列','transmission','手波'),
  ('海斯特','H系列','transmission','自波'),
  ('海斯特','H系列','transmission','无'),
  ('海斯特','H系列','engine','进口发动机'),
  ('海斯特','H系列','engine','无'),
  ('海斯特','J系列','transmission','手波'),
  ('海斯特','J系列','transmission','自波'),
  ('海斯特','J系列','transmission','无'),
  ('海斯特','J系列','engine','进口发动机'),
  ('海斯特','J系列','engine','无')
ON CONFLICT DO NOTHING;

-- 龙工
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('龙工','A系列','transmission','手波'),
  ('龙工','A系列','transmission','自波'),
  ('龙工','A系列','transmission','无'),
  ('龙工','A系列','engine','国产发动机'),
  ('龙工','A系列','engine','无'),
  ('龙工','LG系列','transmission','手波'),
  ('龙工','LG系列','transmission','自波'),
  ('龙工','LG系列','transmission','无'),
  ('龙工','LG系列','engine','国产发动机'),
  ('龙工','LG系列','engine','无'),
  ('龙工','CPCD系列','transmission','手波'),
  ('龙工','CPCD系列','transmission','自波'),
  ('龙工','CPCD系列','transmission','无'),
  ('龙工','CPCD系列','engine','国产发动机'),
  ('龙工','CPCD系列','engine','无'),
  ('龙工','N系列','battery','铅酸'),
  ('龙工','N系列','battery','无'),
  ('龙工','CDD系列','battery','铅酸'),
  ('龙工','CDD系列','battery','无')
ON CONFLICT DO NOTHING;

-- 柳工
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('柳工','CLG系列','transmission','手波'),
  ('柳工','CLG系列','transmission','自波'),
  ('柳工','CLG系列','transmission','无'),
  ('柳工','CLG系列','engine','国产发动机'),
  ('柳工','CLG系列','engine','无'),
  ('柳工','CLG系列','battery','铅酸'),
  ('柳工','CLG系列','battery','无'),
  ('柳工','E系列','transmission','自波'),
  ('柳工','E系列','transmission','无'),
  ('柳工','E系列','engine','国产发动机'),
  ('柳工','E系列','engine','无')
ON CONFLICT DO NOTHING;

-- 中联重科
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('中联重科','FD系列','transmission','手波'),
  ('中联重科','FD系列','transmission','自波'),
  ('中联重科','FD系列','transmission','无'),
  ('中联重科','FD系列','engine','国产发动机'),
  ('中联重科','FD系列','engine','无')
ON CONFLICT DO NOTHING;

-- 宝骊
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('宝骊','KBE系列','battery','磷酸铁锂(LFP)'),
  ('宝骊','KBE系列','battery','铅酸'),
  ('宝骊','KBE系列','battery','无')
ON CONFLICT DO NOTHING;

-- 中力
INSERT INTO series_config_options (brand, series, dimension, option_name) VALUES
  ('中力','EPT系列','battery','磷酸铁锂(LFP)'),
  ('中力','EPT系列','battery','铅酸'),
  ('中力','EPT系列','battery','无'),
  ('中力','ES系列','battery','磷酸铁锂(LFP)'),
  ('中力','ES系列','battery','铅酸'),
  ('中力','ES系列','battery','无'),
  ('中力','其它','battery','铅酸'),
  ('中力','其它','battery','磷酸铁锂(LFP)'),
  ('中力','其它','battery','无')
ON CONFLICT DO NOTHING;

-- 3.13 coefficient_configs（12 个参数：000005 + 000015，description 用 000014 之后的版本）
INSERT INTO coefficient_configs (key, value, description) VALUES
  ('lambda_electric',     0.120000, '电动叉车时间衰减系数 λ（每年衰减率，值越大残值随年限下降越快，建议 0.10~0.15）'),
  ('lambda_combustion',   0.100000, '内燃叉车时间衰减系数 λ（每年衰减率，值越大残值随年限下降越快，建议 0.08~0.12）'),
  ('annual_usage_hours',  1750.000000, '年度标准使用小时数（行业平均年工时，用于计算使用强度比值，电动一般 1500~2000，内燃一般 1200~1800）'),
  ('confidence_range',    0.100000, '残值置信区间幅度 ±（如 0.10 表示残值上下浮动 10%，值越大区间越宽）'),
  ('k_hours_ratio_low',   0.700000, '使用强度比值下限阈值（实际工时/标准工时 < 此值时 Kh=1.10，使用强度低，保值加成）'),
  ('k_hours_ratio_mid',   1.000000, '使用强度比值中段阈值（比值在 low 与此值之间时 Kh=1.00，正常使用）'),
  ('k_hours_ratio_high',  1.300000, '使用强度比值上段阈值（比值在 mid 与此值之间时 Kh=0.95，使用强度偏高）'),
  ('k_hours_ratio_max',   1.600000, '使用强度比值上限阈值（比值在 high 与此值之间时 Kh=0.90，重型使用；超过此值 Kh=0.85）'),
  ('kc_paint_bonus',                  0.020000, '原厂漆 Kc 加成（绝对值，叠加在车况评级 base 上；建议 0.01~0.05；保养类小幅加成）'),
  ('kc_maintenance_bonus',            0.020000, '维保记录 Kc 加成（绝对值，叠加在 base 上；建议 0.01~0.05；保养类小幅加成）'),
  ('kc_no_license_penalty_pct',       0.100000, '缺车牌 Kc 扣减比例（乘性因子，0.10 表示扣减 10%；证件类影响大，建议 0.08~0.15；缺双证时复合放大）'),
  ('kc_no_registration_penalty_pct',  0.100000, '缺登记证 Kc 扣减比例（乘性因子，0.10 表示扣减 10%；证件类影响大，建议 0.08~0.15；缺双证时复合放大）')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, description = EXCLUDED.description, updated_at = NOW();

-- =====================================================
-- 3.14 original_prices（取 000011 数据，去掉 brand_type/is_active，应用 000012 系列重命名与"无"→"其它"）
-- =====================================================

-- ----- 4.1 电动平衡重式 -----

-- 林德 E系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('林德', '电动平衡重式', 'E系列', 1.6, '磷酸铁锂(LFP)', '两级门架', 3000, 266500),
  ('林德', '电动平衡重式', 'E系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 280000),
  ('林德', '电动平衡重式', 'E系列', 2.0, '铅酸', '两级门架', 3000, 240000),
  ('林德', '电动平衡重式', 'E系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 295000),
  ('林德', '电动平衡重式', 'E系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 302200)
ON CONFLICT DO NOTHING;

-- 林德 Xi系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('林德', '电动平衡重式', 'Xi系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 255000),
  ('林德', '电动平衡重式', 'Xi系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 275000)
ON CONFLICT DO NOTHING;

-- 丰田 8FBE系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('丰田', '电动平衡重式', '8FBE系列', 2.0, '铅酸', '两级门架', 3000, 210000),
  ('丰田', '电动平衡重式', '8FBE系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 245000),
  ('丰田', '电动平衡重式', '8FBE系列', 3.0, '铅酸', '三级门架', 4500, 265000),
  ('丰田', '电动平衡重式', '8FBE系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 300000)
ON CONFLICT DO NOTHING;

-- 永恒力 EFG系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('永恒力', '电动平衡重式', 'EFG系列', 1.5, '铅酸', '两级门架', 3000, 158000),
  ('永恒力', '电动平衡重式', 'EFG系列', 2.5, '铅酸', '两级门架', 3000, 185000),
  ('永恒力', '电动平衡重式', 'EFG系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 215000),
  ('永恒力', '电动平衡重式', 'EFG系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 245000)
ON CONFLICT DO NOTHING;

-- 合力 X系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '电动平衡重式', 'X系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 88000),
  ('合力', '电动平衡重式', 'X系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 105000)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '电动平衡重式', 'CPD系列', 1.5, '铅酸', '两级门架', 3000, 98000),
  ('合力', '电动平衡重式', 'CPD系列', 2.0, '铅酸', '两级门架', 3000, 115000),
  ('合力', '电动平衡重式', 'CPD系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 132000),
  ('合力', '电动平衡重式', 'CPD系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 128000),
  ('合力', '电动平衡重式', 'CPD系列', 3.5, '磷酸铁锂(LFP)', '三级门架', 4500, 165000)
ON CONFLICT DO NOTHING;

-- 杭叉 XH系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '电动平衡重式', 'XH系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 92000),
  ('杭叉', '电动平衡重式', 'XH系列', 2.5, '磷酸铁锂(LFP)', '三级门架', 4500, 125000)
ON CONFLICT DO NOTHING;

-- 杭叉 XC系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '电动平衡重式', 'XC系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 75000),
  ('杭叉', '电动平衡重式', 'XC系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 88000)
ON CONFLICT DO NOTHING;

-- 杭叉 XE系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '电动平衡重式', 'XE系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 78000),
  ('杭叉', '电动平衡重式', 'XE系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 95000)
ON CONFLICT DO NOTHING;

-- 比亚迪 CPD系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('比亚迪', '电动平衡重式', 'CPD系列', 1.6, '磷酸铁锂(LFP)', '两级门架', 3000, 92000),
  ('比亚迪', '电动平衡重式', 'CPD系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 105000),
  ('比亚迪', '电动平衡重式', 'CPD系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 118000),
  ('比亚迪', '电动平衡重式', 'CPD系列', 3.0, '磷酸铁锂(LFP)', '三级门架', 4500, 138000)
ON CONFLICT DO NOTHING;

-- 宝骊 KBE系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('宝骊', '电动平衡重式', 'KBE系列', 2.0, '铅酸', '两级门架', 3000, 62000),
  ('宝骊', '电动平衡重式', 'KBE系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3000, 78000),
  ('宝骊', '电动平衡重式', 'KBE系列', 2.5, '磷酸铁锂(LFP)', '两级门架', 3000, 88000)
ON CONFLICT DO NOTHING;

-- ----- 4.2 电动前移式 -----

-- 丰田 Traigo系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('丰田', '电动前移式', 'Traigo系列', 2.0, '磷酸铁锂(LFP)', '三级门架', 6000, 265000),
  ('丰田', '电动前移式', 'Traigo系列', 2.5, '磷酸铁锂(LFP)', '三级门架', 6000, 295000)
ON CONFLICT DO NOTHING;

-- 永恒力 ETV系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('永恒力', '电动前移式', 'ETV系列', 2.0, '铅酸', '三级门架', 5000, 229600),
  ('永恒力', '电动前移式', 'ETV系列', 2.5, '磷酸铁锂(LFP)', '三级门架', 6000, 280000)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（电动前移式）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '电动前移式', 'CPD系列', 2.0, '铅酸', '三级门架', 6000, 135000)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（电动前移式）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '电动前移式', 'A系列', 2.0, '铅酸', '三级门架', 6000, 150000)
ON CONFLICT DO NOTHING;

-- 比亚迪 CPD系列（电动前移式）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('比亚迪', '电动前移式', 'CPD系列', 1.5, '磷酸铁锂(LFP)', '三级门架', 5500, 120000)
ON CONFLICT DO NOTHING;

-- ----- 4.3 电动托盘搬运车 -----

-- 中力 EPT系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('中力', '电动托盘搬运车', 'EPT系列', 1.5, '铅酸', '无', 0, 9900),
  ('中力', '电动托盘搬运车', 'EPT系列', 2.0, '铅酸', '无', 0, 15000),
  ('中力', '电动托盘搬运车', 'EPT系列', 2.0, '磷酸铁锂(LFP)', '无', 0, 17000),
  ('中力', '电动托盘搬运车', 'EPT系列', 3.0, '磷酸铁锂(LFP)', '无', 0, 22000)
ON CONFLICT DO NOTHING;

-- 中力 其它（原"无"系列，已重命名）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('中力', '电动托盘搬运车', '其它', 1.5, '铅酸', '无', 0, 9500),
  ('中力', '电动托盘搬运车', '其它', 2.0, '铅酸', '无', 0, 14000)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（电动托盘搬运车）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '电动托盘搬运车', 'A系列', 2.0, '磷酸铁锂(LFP)', '无', 0, 28000)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（电动托盘搬运车）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '电动托盘搬运车', 'CPD系列', 2.0, '铅酸', '无', 0, 25000)
ON CONFLICT DO NOTHING;

-- 永恒力 EFG系列（电动托盘搬运车）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('永恒力', '电动托盘搬运车', 'EFG系列', 1.5, '铅酸', '无', 0, 35000)
ON CONFLICT DO NOTHING;

-- ----- 4.4 电动堆高车 -----

-- 中力 ES系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('中力', '电动堆高车', 'ES系列', 1.5, '铅酸', '两级门架', 3000, 21800),
  ('中力', '电动堆高车', 'ES系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 25000),
  ('中力', '电动堆高车', 'ES系列', 2.0, '铅酸', '两级门架', 3500, 52000)
ON CONFLICT DO NOTHING;

-- 永恒力 ERIC系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('永恒力', '电动堆高车', 'ERIC系列', 1.5, '铅酸', '两级门架', 3000, 45000),
  ('永恒力', '电动堆高车', 'ERIC系列', 1.5, '磷酸铁锂(LFP)', '两级门架', 3000, 55000),
  ('永恒力', '电动堆高车', 'ERIC系列', 2.0, '铅酸', '两级门架', 3500, 62000)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（电动堆高车）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '电动堆高车', 'A系列', 2.0, '磷酸铁锂(LFP)', '两级门架', 3300, 55000)
ON CONFLICT DO NOTHING;

-- 合力 CPD系列（电动堆高车）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '电动堆高车', 'CPD系列', 1.5, '铅酸', '两级门架', 3000, 32000)
ON CONFLICT DO NOTHING;

-- ----- 4.5 内燃平衡重式 -----

-- 林德 H系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('林德', '内燃平衡重式', 'H系列', 2.0, '手波/进口发动机', '两级门架', 3000, 155000),
  ('林德', '内燃平衡重式', 'H系列', 2.5, '手波/进口发动机', '两级门架', 3000, 168000),
  ('林德', '内燃平衡重式', 'H系列', 2.5, '自波/进口发动机', '两级门架', 3000, 192000),
  ('林德', '内燃平衡重式', 'H系列', 3.5, '手波/进口发动机', '三级门架', 4000, 195000)
ON CONFLICT DO NOTHING;

-- 林德 T-MATIC
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('林德', '内燃平衡重式', 'T-MATIC', 2.0, '自波/进口发动机', '两级门架', 3000, 178000),
  ('林德', '内燃平衡重式', 'T-MATIC', 3.0, '自波/进口发动机', '三级门架', 4500, 215000)
ON CONFLICT DO NOTHING;

-- 丰田 8FD系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('丰田', '内燃平衡重式', '8FD系列', 2.5, '手波/进口发动机', '两级门架', 3000, 168000),
  ('丰田', '内燃平衡重式', '8FD系列', 2.5, '自波/进口发动机', '两级门架', 3000, 192000),
  ('丰田', '内燃平衡重式', '8FD系列', 3.5, '手波/进口发动机', '三级门架', 4000, 195000)
ON CONFLICT DO NOTHING;

-- 合力 A系列（内燃平衡重式）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级门架', 3000, 72000),
  ('合力', '内燃平衡重式', 'A系列', 2.5, '手波/国产发动机', '两级门架', 3000, 80000),
  ('合力', '内燃平衡重式', 'A系列', 2.5, '自波/国产发动机', '两级门架', 3000, 92000),
  ('合力', '内燃平衡重式', 'A系列', 3.5, '手波/国产发动机', '三级门架', 4000, 102000)
ON CONFLICT DO NOTHING;

-- 合力 CPCD系列（内燃平衡重式）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '内燃平衡重式', 'CPCD系列', 2.0, '手波/国产发动机', '两级门架', 3000, 75000),
  ('合力', '内燃平衡重式', 'CPCD系列', 2.5, '手波/国产发动机', '两级门架', 3000, 82000),
  ('合力', '内燃平衡重式', 'CPCD系列', 2.5, '手波/进口发动机', '两级门架', 3000, 105000),
  ('合力', '内燃平衡重式', 'CPCD系列', 3.5, '手波/国产发动机', '三级门架', 4000, 105000)
ON CONFLICT DO NOTHING;

-- 杭叉 A系列（内燃平衡重式）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级门架', 3000, 68000),
  ('杭叉', '内燃平衡重式', 'A系列', 2.5, '手波/国产发动机', '两级门架', 3000, 75000),
  ('杭叉', '内燃平衡重式', 'A系列', 2.5, '自波/国产发动机', '两级门架', 3000, 86000),
  ('杭叉', '内燃平衡重式', 'A系列', 3.5, '手波/国产发动机', '三级门架', 4000, 95000)
ON CONFLICT DO NOTHING;

-- 杭叉 XF系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('杭叉', '内燃平衡重式', 'XF系列', 2.5, '手波/国产发动机', '两级门架', 3000, 82000),
  ('杭叉', '内燃平衡重式', 'XF系列', 2.5, '手波/进口发动机', '两级门架', 3000, 105000),
  ('杭叉', '内燃平衡重式', 'XF系列', 3.5, '手波/国产发动机', '三级门架', 4000, 102000)
ON CONFLICT DO NOTHING;

-- 斗山 B30S系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('斗山', '内燃平衡重式', 'B30S系列', 2.5, '手波/进口发动机', '两级门架', 3000, 145000),
  ('斗山', '内燃平衡重式', 'B30S系列', 3.0, '手波/进口发动机', '三级门架', 4500, 165000)
ON CONFLICT DO NOTHING;

-- 斗山 BR系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('斗山', '内燃平衡重式', 'BR系列', 2.5, '手波/进口发动机', '两级门架', 3000, 148000),
  ('斗山', '内燃平衡重式', 'BR系列', 3.0, '手波/进口发动机', '三级门架', 4500, 168000)
ON CONFLICT DO NOTHING;

-- 海斯特 H系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('海斯特', '内燃平衡重式', 'H系列', 2.5, '手波/进口发动机', '两级门架', 3000, 142000),
  ('海斯特', '内燃平衡重式', 'H系列', 3.0, '手波/进口发动机', '三级门架', 4500, 162000),
  ('海斯特', '内燃平衡重式', 'H系列', 3.0, '自波/进口发动机', '三级门架', 4500, 185000)
ON CONFLICT DO NOTHING;

-- 海斯特 J系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('海斯特', '内燃平衡重式', 'J系列', 3.0, '手波/进口发动机', '三级门架', 4500, 158000),
  ('海斯特', '内燃平衡重式', 'J系列', 5.0, '手波/进口发动机', '三级门架', 4500, 220000)
ON CONFLICT DO NOTHING;

-- 龙工 A系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('龙工', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级门架', 3000, 58000),
  ('龙工', '内燃平衡重式', 'A系列', 3.0, '手波/国产发动机', '两级门架', 3000, 72000)
ON CONFLICT DO NOTHING;

-- 龙工 LG系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('龙工', '内燃平衡重式', 'LG系列', 2.0, '手波/国产发动机', '两级门架', 3000, 62000),
  ('龙工', '内燃平衡重式', 'LG系列', 2.5, '手波/国产发动机', '两级门架', 3000, 68000),
  ('龙工', '内燃平衡重式', 'LG系列', 5.0, '手波/国产发动机', '三级门架', 4500, 128000)
ON CONFLICT DO NOTHING;

-- 柳工 CLG系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('柳工', '内燃平衡重式', 'CLG系列', 2.5, '手波/国产发动机', '两级门架', 3000, 65000),
  ('柳工', '内燃平衡重式', 'CLG系列', 3.0, '手波/国产发动机', '两级门架', 3000, 75000)
ON CONFLICT DO NOTHING;

-- ----- 4.6 内燃重型叉车 -----

-- 合力 CPCD系列（大吨位）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('合力', '内燃重型叉车', 'CPCD系列', 10.0, '手波/国产发动机', '三级门架', 4500, 380000),
  ('合力', '内燃重型叉车', 'CPCD系列', 15.0, '手波/国产发动机', '三级门架', 4500, 450000)
ON CONFLICT DO NOTHING;

-- 龙工 LG系列（大吨位）
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('龙工', '内燃重型叉车', 'LG系列', 12.0, '手波/国产发动机', '三级门架', 4500, 350000)
ON CONFLICT DO NOTHING;

-- 中联重科 FD系列
INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, original_price) VALUES
  ('中联重科', '内燃重型叉车', 'FD系列', 10.0, '手波/国产发动机', '三级门架', 4500, 365000),
  ('中联重科', '内燃重型叉车', 'FD系列', 16.0, '手波/国产发动机', '三级门架', 4500, 520000)
ON CONFLICT DO NOTHING;

-- =====================================================
-- 5. Excel 价格对照表数据（92 行，已应用 000012 系列重命名）
-- =====================================================

INSERT INTO original_prices (
    brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm, original_price
) VALUES
  -- ========== 合力 ==========
  ('合力', '内燃平衡重式', 'K系列（K1）', 2.0, '手波/国产发动机', '两级标准门架', 3000, 50000),
  ('合力', '内燃平衡重式', 'K系列（K1）', 3.0, '手波/国产发动机', '两级标准门架', 3000, 57000),
  ('合力', '内燃平衡重式', 'K系列（K1）', 3.0, '自波/国产发动机', '两级标准门架', 3000, 63000),
  ('合力', '内燃平衡重式', 'K系列（K1）', 3.0, '手波/国产发动机', '三级全自由门架', 4500, 65000),
  ('合力', '内燃平衡重式', 'K系列（K1）', 3.5, '自波/国产发动机', '三级全自由门架', 4500, 70000),
  ('合力', '内燃平衡重式', 'K系列（K1）', 3.5, '无级变速/国产发动机', '两级标准门架', 3000, 100000),
  ('合力', '内燃平衡重式', 'K2系列（大力士）', 5.0, '自波/国产发动机', '两级标准门架', 3000, 122500),
  ('合力', '内燃平衡重式', 'K2系列（大力士）', 5.0, '自波/国产发动机', '三级全自由门架', 4500, 132500),
  ('合力', '内燃平衡重式', 'K2系列（大力士）', 7.0, '自波/国产发动机', '两级标准门架', 3000, 170000),
  ('合力', '内燃平衡重式', 'K2系列（大力士）', 10.0, '自波/国产发动机', '两级标准门架', 3000, 235000),
  ('合力', '内燃平衡重式', 'G2系列', 20.0, '自波/国产发动机', '两级标准门架', 3000, 500000),
  ('合力', '电动平衡重式', 'H3系列', 1.5, '铅酸', '两级标准门架', 3000, 82500),
  ('合力', '电动平衡重式', 'H3系列', 2.0, '铅酸', '两级标准门架', 3000, 92500),
  ('合力', '电动平衡重式', 'H3系列', 3.0, '铅酸', '两级标准门架', 3000, 115000),
  ('合力', '电动平衡重式', 'H3系列', 3.0, '铅酸', '三级全自由门架', 4500, 125000),
  ('合力', '电动平衡重式', 'H4系列', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 132500),
  ('合力', '电动平衡重式', 'H4系列', 3.5, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 152500),
  ('合力', '电动平衡重式', 'K3系列（锂电专用）', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 147500),
  ('合力', '电动平衡重式', 'G3系列', 6.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 240000),
  ('合力', '电动前移式', '前移式', 1.5, '铅酸', '三级门架', 6000, 165000),
  ('合力', '电动前移式', '前移式', 2.0, '铅酸', '四级门架', 9500, 240000),

  -- ========== 杭叉 ==========
  ('杭叉', '内燃平衡重式', 'A系列', 2.0, '手波/国产发动机', '两级标准门架', 3000, 46000),
  ('杭叉', '内燃平衡重式', 'A系列', 3.0, '手波/国产发动机', '两级标准门架', 3000, 54000),
  ('杭叉', '内燃平衡重式', 'A系列', 3.0, '自波/国产发动机', '两级标准门架', 3000, 60500),
  ('杭叉', '内燃平衡重式', 'A系列', 3.0, '自波/国产发动机', '三级全自由门架', 4500, 63000),
  ('杭叉', '内燃平衡重式', 'A系列', 3.5, '自波/国产发动机', '三级全自由门架', 4500, 70000),
  ('杭叉', '内燃平衡重式', 'A系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 112500),
  ('杭叉', '内燃平衡重式', 'A系列', 7.0, '自波/国产发动机', '两级标准门架', 3000, 160000),
  ('杭叉', '内燃平衡重式', 'XA系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 120000),
  ('杭叉', '内燃平衡重式', 'XA系列', 10.0, '自波/国产发动机', '两级标准门架', 3000, 215000),
  ('杭叉', '电动平衡重式', 'A系列', 1.5, '铅酸', '两级标准门架', 3000, 77500),
  ('杭叉', '电动平衡重式', 'A系列', 2.0, '铅酸', '两级标准门架', 3000, 87500),
  ('杭叉', '电动平衡重式', 'A系列', 3.0, '铅酸', '两级标准门架', 3000, 105000),
  ('杭叉', '电动平衡重式', 'A系列', 3.0, '铅酸', '三级全自由门架', 4500, 115000),
  ('杭叉', '电动平衡重式', 'XC系列', 2.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 105000),
  ('杭叉', '电动平衡重式', 'XC系列', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 122500),
  ('杭叉', '电动平衡重式', 'XC系列', 3.5, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 142500),
  ('杭叉', '电动平衡重式', 'XH系列', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 147500),
  ('杭叉', '电动平衡重式', 'J系列（大吨位）', 5.0, '铅酸', '两级标准门架', 3000, 200000),
  ('杭叉', '电动前移式', 'X系列', 1.5, '铅酸', '三级门架', 6000, 155000),
  ('杭叉', '电动前移式', 'X系列', 2.0, '铅酸', '四级门架', 10000, 225000),
  ('杭叉', '电动堆高车', 'A系列（经济型）', 1.5, '铅酸', '两级门架', 3000, 30000),
  ('杭叉', '电动堆高车', 'A系列', 2.0, '铅酸', '三级门架', 4500, 65000),

  -- ========== 丰田 ==========
  ('丰田', '内燃平衡重式', '7系列（7FD）', 3.0, '自波/进口发动机', '两级标准门架', 3000, 200000),
  ('丰田', '内燃平衡重式', '8FD系列', 3.0, '自波/进口发动机', '两级标准门架', 3000, 225000),
  ('丰田', '内燃平衡重式', '8FD系列', 3.0, '自波/进口发动机', '三级全自由门架', 4500, 245000),
  ('丰田', '内燃平衡重式', '8FD系列', 5.0, '自波/进口发动机', '两级标准门架', 3000, 315000),
  ('丰田', '内燃平衡重式', 'Z系列（本土化）', 3.0, '手波/进口发动机', '两级标准门架', 3000, 135000),
  ('丰田', '内燃平衡重式', 'Z系列（本土化）', 3.0, '自波/进口发动机', '三级全自由门架', 4500, 155000),
  ('丰田', '电动平衡重式', '8系列（8FBN）', 1.5, '铅酸', '两级标准门架', 3000, 220000),
  ('丰田', '电动平衡重式', '8系列（8FBN）', 2.0, '铅酸', '两级标准门架', 3000, 255000),
  ('丰田', '电动平衡重式', '8系列（8FBN）', 3.0, '铅酸', '两级标准门架', 3000, 290000),
  ('丰田', '电动平衡重式', '8系列（8FBN）', 3.0, '铅酸', '三级全自由门架', 4500, 310000),
  ('丰田', '电动平衡重式', '8FBE系列', 1.5, '铅酸', '两级标准门架', 3000, 200000),
  ('丰田', '电动前移式', '8系列（8FBR）', 1.5, '铅酸', '三级门架', 6000, 305000),
  ('丰田', '电动前移式', '8系列（8FBR）', 2.0, '铅酸', '四级门架', 11300, 385000),

  -- ========== 林德 ==========
  ('林德', '内燃平衡重式', 'H系列', 3.0, '无级变速/进口发动机', '两级宽视野门架', 3000, 275000),
  ('林德', '内燃平衡重式', 'H系列', 3.0, '无级变速/进口发动机', '三级全自由门架', 4500, 305000),
  ('林德', '内燃平衡重式', 'H系列', 5.0, '无级变速/进口发动机', '两级宽视野门架', 3000, 385000),
  ('林德', '电动平衡重式', 'E系列', 1.6, '磷酸铁锂(LFP)', '两级宽视野门架', 3250, 240000),
  ('林德', '电动平衡重式', 'E系列', 3.0, '磷酸铁锂(LFP)', '两级宽视野门架', 3250, 310000),
  ('林德', '电动平衡重式', 'E系列', 3.0, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 340000),
  ('林德', '电动平衡重式', 'E系列', 5.0, '铅酸', '两级宽视野门架', 3250, 440000),
  ('林德', '电动前移式', 'R系列', 1.6, '铅酸', '三级门架', 6000, 330000),
  ('林德', '电动前移式', 'R系列', 2.0, '铅酸', '四级HD门架', 12000, 450000),
  ('林德', '电动堆高车', 'L系列', 1.6, '铅酸', '三级门架', 4500, 140000),

  -- ========== 龙工 ==========
  ('龙工', '内燃平衡重式', 'CPCD系列', 3.0, '手波/国产发动机', '两级标准门架', 3000, 48500),
  ('龙工', '内燃平衡重式', 'CPCD系列', 3.0, '自波/国产发动机', '两级标准门架', 3000, 54000),
  ('龙工', '内燃平衡重式', 'CPCD系列', 3.5, '自波/国产发动机', '三级门架', 4500, 63000),
  ('龙工', '内燃平衡重式', 'CPCD系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 97500),
  ('龙工', '内燃平衡重式', 'CPCD系列', 10.0, '自波/国产发动机', '三级门架', 4500, 195000),
  ('龙工', '电动平衡重式', 'N系列', 1.5, '铅酸', '两级标准门架', 3000, 67500),
  ('龙工', '电动平衡重式', 'N系列', 2.0, '铅酸', '两级标准门架', 3000, 77500),
  ('龙工', '电动平衡重式', 'N系列', 3.0, '铅酸', '三级全自由门架', 4500, 100000),
  ('龙工', '电动堆高车', 'CDD系列', 1.5, '铅酸', '两级门架', 3000, 21500),
  ('龙工', '电动堆高车', 'CDD系列', 2.0, '铅酸', '三级门架', 4500, 47500),

  -- ========== 柳工 ==========
  ('柳工', '内燃平衡重式', 'CLG系列', 3.0, '手波/国产发动机', '两级标准门架', 3000, 51500),
  ('柳工', '内燃平衡重式', 'CLG系列', 3.0, '自波/国产发动机', '三级全自由门架', 4500, 60000),
  ('柳工', '内燃平衡重式', 'CLG系列', 3.5, '自波/国产发动机', '三级门架', 4500, 52500),
  ('柳工', '内燃平衡重式', 'E系列', 5.0, '自波/国产发动机', '两级标准门架', 3000, 110000),
  ('柳工', '内燃平衡重式', 'E系列', 7.0, '自波/国产发动机', '两级标准门架', 3000, 152500),
  ('柳工', '电动平衡重式', 'CLG系列', 2.0, '铅酸', '两级标准门架', 3000, 77500),
  ('柳工', '电动平衡重式', 'CLG系列', 3.0, '铅酸', '三级门架', 4500, 100000),

  -- ========== 比亚迪 ==========
  ('比亚迪', '电动平衡重式', 'CPD系列', 1.5, '磷酸铁锂(LFP)', '两级标准门架', 3000, 62500),
  ('比亚迪', '电动平衡重式', 'CPD系列', 2.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 72500),
  ('比亚迪', '电动平衡重式', 'CPD系列', 3.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 90000),
  ('比亚迪', '电动平衡重式', 'CPD系列', 3.0, '磷酸铁锂(LFP)', '三级全自由门架', 4500, 100000),
  ('比亚迪', '电动平衡重式', 'CPD系列', 3.5, '磷酸铁锂(LFP)', '两级标准门架', 3000, 105000),
  ('比亚迪', '电动平衡重式', 'CPD系列（大吨位）', 5.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 165000),
  ('比亚迪', '电动平衡重式', 'CPD系列（大吨位）', 7.0, '磷酸铁锂(LFP)', '两级标准门架', 3000, 240000),
  ('比亚迪', '电动堆高车', 'S系列', 1.5, '铅酸', '两级门架', 3000, 24000),
  ('比亚迪', '电动堆高车', 'S系列', 2.0, '铅酸', '三级门架', 4500, 52500)
ON CONFLICT DO NOTHING;

-- =====================================================
-- 5.1 earliest_factory_year 回填（用 series 表数据）
-- =====================================================
UPDATE original_prices op
SET earliest_factory_year = s.earliest_factory_year
FROM series s
WHERE s.brand = op.brand AND s.name = op.series;

-- =====================================================
-- 6. "无" 配置兜底记录
--    对每个 (brand, vehicle_type, series) 组合，若尚无含 "无" 的 config_type 记录，
--    则补一条最低原价的兜底记录，保证级联过滤在 config_type/mast_type/mast_height 步骤不断链。
--    电动 series：config_type = '无'
--    内燃 series：config_type = '无/无'
-- =====================================================
INSERT INTO original_prices (
    brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm, original_price
)
SELECT
    op.brand, op.vehicle_type, op.series, MIN(op.tonnage),
    CASE WHEN op.config_type LIKE '%/%' THEN '无/无' ELSE '无' END,
    '无', 0, MIN(op.original_price)
FROM original_prices op
WHERE NOT EXISTS (
    SELECT 1 FROM original_prices p2
    WHERE p2.brand = op.brand
      AND p2.vehicle_type = op.vehicle_type
      AND p2.series = op.series
      AND p2.config_type = (CASE WHEN op.config_type LIKE '%/%' THEN '无/无' ELSE '无' END)
)
GROUP BY op.brand, op.vehicle_type, op.series, (CASE WHEN op.config_type LIKE '%/%' THEN '无/无' ELSE '无' END)
ON CONFLICT DO NOTHING;

-- 兜底记录的 earliest_factory_year 回填
UPDATE original_prices op
SET earliest_factory_year = s.earliest_factory_year
FROM series s
WHERE s.brand = op.brand AND s.name = op.series
  AND op.config_type IN ('无', '无/无');

-- =====================================================
-- Part 4: N1 题库种子数据（course/knowledge_point/chapter/question）
-- =====================================================

-- ================================================================
-- 叉车N1司机证理论培训 - 数据库种子数据 v2
-- ================================================================
-- 自动生成，请勿手工修改
-- 数据源：.workbuddy/叉车N1司机证理论考试题库.md
-- 生成器：.trae/docs/N1题库导入脚本/gen_n1_seed.py
--
-- 设计要点：
--   1. 沿用现有 JSON 题库已定义的 6 门课程（不新建课程）
--   2. 6 门课程分布在 4 个 category：
--      - CATEGORY_01 基础理论：课程1 叉车基础知识概述、课程2 液压系统原理与维护
--      - CATEGORY_02 安全规范：课程3 叉车安全操作规范
--      - CATEGORY_03 实操技能：课程4 日常检查与保养指南、课程5 货叉操作技能训练
--      - CATEGORY_04 进阶提升：课程6 故障诊断与排除进阶
--   3. 6 门课程 × 3 章节 = 18 章节（含 Markdown 正文）
--   4. 6 个知识点（与课程一一对应）
--   5. 题目筛选：剔除纯法规/证件管理类，保留结构原理/维护/故障/安全操作 4 类
--   6. level 按内容难度判定：记忆→beginner，理解→intermediate，应用→advanced
--   7. 每知识点 3 档 level 全覆盖
--
-- 执行方法：
--   psql "postgres://用户名:密码@主机:5432/数据库名" -f seed_n1.sql
--
-- 幂等：可重复执行
-- ================================================================


-- ============================================================
-- 1. 课程（沿用现有 JSON 题库 6 门课程，分布在 4 个 category 下）
-- ============================================================
INSERT INTO course (course_id, name, category, description, cover_image, duration, status, created_at) OVERRIDING SYSTEM VALUE VALUES (1, '叉车基础知识概述', 'CATEGORY_01', '叉车分类、基本结构、主要技术参数等入门知识，为后续维修与操作学习奠定基础。', NULL, 120, 1, now()) ON CONFLICT (course_id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description, duration = EXCLUDED.duration, status = EXCLUDED.status;
INSERT INTO course (course_id, name, category, description, cover_image, duration, status, created_at) OVERRIDING SYSTEM VALUE VALUES (2, '液压系统原理与维护', 'CATEGORY_04', '液压传动原理、液压元件结构、液压系统维护与故障排除，进阶技术内容。', NULL, 150, 1, now()) ON CONFLICT (course_id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description, duration = EXCLUDED.duration, status = EXCLUDED.status;
INSERT INTO course (course_id, name, category, description, cover_image, duration, status, created_at) OVERRIDING SYSTEM VALUE VALUES (3, '叉车安全操作规范', 'CATEGORY_02', '叉车起步、行驶、转弯、坡道、停车、会车等环节的安全操作规程。', NULL, 120, 1, now()) ON CONFLICT (course_id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description, duration = EXCLUDED.duration, status = EXCLUDED.status;
INSERT INTO course (course_id, name, category, description, cover_image, duration, status, created_at) OVERRIDING SYSTEM VALUE VALUES (4, '日常检查与保养指南', 'CATEGORY_03', '出车前检查项目、十字作业法、液压与制动系统保养、电瓶与轮胎维护。', NULL, 120, 1, now()) ON CONFLICT (course_id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description, duration = EXCLUDED.duration, status = EXCLUDED.status;
INSERT INTO course (course_id, name, category, description, cover_image, duration, status, created_at) OVERRIDING SYSTEM VALUE VALUES (5, '货叉操作技能训练', 'CATEGORY_03', '货物叉取、起升、堆垛、装卸等作业环节的标准操作与安全要求。', NULL, 120, 1, now()) ON CONFLICT (course_id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description, duration = EXCLUDED.duration, status = EXCLUDED.status;
INSERT INTO course (course_id, name, category, description, cover_image, duration, status, created_at) OVERRIDING SYSTEM VALUE VALUES (6, '故障诊断与排除进阶', 'CATEGORY_04', '叉车制动、液压、转向系统故障诊断与排除，应急处置与突发情况应对。', NULL, 150, 1, now()) ON CONFLICT (course_id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description, duration = EXCLUDED.duration, status = EXCLUDED.status;

-- ============================================================
-- 2. 知识点（6 个，与 6 门课程对应，category 与 course.category 保持一致）
-- ============================================================
INSERT INTO knowledge_point (id, name, category, parent_id, description, created_at) OVERRIDING SYSTEM VALUE VALUES (1, '叉车结构与基础', 'CATEGORY_01', NULL, '对应课程1：叉车分类、基本结构、技术参数等基础知识', now()) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description;
INSERT INTO knowledge_point (id, name, category, parent_id, description, created_at) OVERRIDING SYSTEM VALUE VALUES (2, '液压与动力系统', 'CATEGORY_04', NULL, '对应课程2：液压传动原理、液压元件、动力传递路线', now()) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description;
INSERT INTO knowledge_point (id, name, category, parent_id, description, created_at) OVERRIDING SYSTEM VALUE VALUES (3, '安全操作规程', 'CATEGORY_02', NULL, '对应课程3：起步、行驶、转弯、坡道、停车、会车规程', now()) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description;
INSERT INTO knowledge_point (id, name, category, parent_id, description, created_at) OVERRIDING SYSTEM VALUE VALUES (4, '日常检查与保养', 'CATEGORY_03', NULL, '对应课程4：出车前检查、十字作业法、液压制动保养、电瓶轮胎维护', now()) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description;
INSERT INTO knowledge_point (id, name, category, parent_id, description, created_at) OVERRIDING SYSTEM VALUE VALUES (5, '货叉作业技能', 'CATEGORY_03', NULL, '对应课程5：货物叉取、起升、堆垛、装卸、视线盲区', now()) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description;
INSERT INTO knowledge_point (id, name, category, parent_id, description, created_at) OVERRIDING SYSTEM VALUE VALUES (6, '故障诊断与排除', 'CATEGORY_04', NULL, '对应课程6：制动/液压/转向故障诊断、应急处置', now()) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, description = EXCLUDED.description;

-- ============================================================
-- 3. 章节（6 门课程 × 3 章节 = 18 章节，含 Markdown 正文 + 扩充内容）
-- ============================================================
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (1, 1, '第一章 叉车分类与型号', '## 第一章 叉车分类与型号

叉车（Forklift Truck）是工业搬运车辆，主要用于成件托盘货物的装卸、堆垛和短距离运输。

### 按动力方式分类
- **内燃叉车**：柴油、汽油、液化石油气（LPG）动力，功率大、续航长
- **电动叉车**：蓄电池驱动，噪音低、无污染，适合室内作业
- **手动液压叉车**：人力操作，适用于轻型短距离搬运

### 按结构形式分类
- 平衡重式叉车（最常见，车体后部配重）
- 前移式叉车（门架可前后移动，适合窄通道）
- 插腿式叉车（前伸支腿提供稳定性）
- 侧面叉车（货叉在侧面，适合长条物料）
- 托盘堆垛车（专用于托盘堆垛）

### 按额定起重量分类
- 轻型：1 吨以下（电商物流中心）
- 中型：1～5 吨（一般工厂仓库，2.0～2.5 吨最常见）
- 重型：5～10 吨（建材、机械等重工业）
- 超重型：10 吨以上（港口、冶金）

### 本章重点
叉车按动力分为内燃/电动/手动；按结构分为平衡重/前移/插腿/侧面；2.0～2.5 吨级是工厂仓库最常见规格。


---

### 技术参数速查表

| 叉车类型 | 额定起重量 | 动力方式 | 适用场景 |
|---------|----------|---------|---------|
| 轻型 | ≤1吨 | 电动 | 电商物流、轻型仓储 |
| 中型 | 1-5吨 | 内燃/电动 | 工厂仓库（2.0-2.5吨最常见） |
| 重型 | 5-10吨 | 内燃 | 建材、机械等重工业 |
| 超重型 | >10吨 | 内燃 | 港口、冶金 |

| 结构类型 | 特点 | 适用场景 |
|---------|------|---------|
| 平衡重式 | 后部配重，最常见 | 通用搬运 |
| 前移式 | 门架前后移动 | 窄通道仓储 |
| 插腿式 | 前伸支腿 | 低层堆垛 |
| 侧面叉车 | 货叉在侧面 | 长条物料 |
| 托盘堆垛车 | 步行/站立操作 | 托盘堆垛 |

### 常见误区辨析
- **误区**：电动叉车比内燃叉车力量小 → **事实**：同吨位级别两者额定起重量相同，区别在于续航和适用环境
- **误区**：平衡重越重越好 → **事实**：平衡重需与额定起重量匹配，过重会影响转向灵活性和后轮承压
', NULL, 'text', NULL, NULL, 40, 1, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (2, 1, '第二章 叉车基本结构组成', '## 第二章 叉车基本结构组成

叉车主要由七大系统组成：动力系统、传动系统、转向系统、制动系统、液压系统、工作装置、电气系统。

### 动力与传动系统
- 内燃叉车动力传递：发动机 → 液力变矩器 → 变速箱 → 传动轴 → 驱动桥 → 驱动轮
- 电动叉车动力传递：蓄电池 → 电机控制器 → 行走电机 → 驱动桥 → 驱动轮
- 变速箱功能：变换速度和扭矩，实现前进后退

### 液压系统（核心工作系统）
- 以液压油为工作介质，利用液体压力能传递动力
- 核心原理：帕斯卡定律
- 工作流程：吸油 → 压油 → 控制 → 执行 → 回油
- 主要驱动工作装置（货叉升降、门架倾斜），不驱动车轮行驶

### 工作装置
- 门架：前倾角 3°～6°（便于叉取），后倾角 6°～12°（防止货物滑落）
- 货叉：直接接触货物的部件
- 自由起升高度：门架高度不增加时货叉的最大起升高度（集装箱作业关键参数）

### 安全装置
- 护顶架：防止上方坠落物伤害操作人员
- 安全带：防止侧翻时驾驶员被甩出

### 本章重点
叉车七大系统；液压系统基于帕斯卡定律；门架后倾角 6°～12°防滑落；护顶架防坠落物。


---

### 叉车七大系统对照表

| 系统 | 功能 | 核心部件 | 常见故障 |
|------|------|---------|---------|
| 动力系统 | 提供动力 | 发动机/蓄电池 | 无法启动、动力不足 |
| 传动系统 | 传递动力 | 变速箱、传动轴、驱动桥 | 异响、跳挡 |
| 转向系统 | 控制方向 | 方向盘、转向器、转向油缸 | 转向沉重、跑偏 |
| 制动系统 | 减速停车 | 制动踏板、制动总泵/分泵、制动片 | 失灵、发软、偏刹 |
| 液压系统 | 驱动工作装置 | 液压泵、换向阀、起升油缸、倾斜油缸 | 不起升、下落、油温高 |
| 工作装置 | 装卸货物 | 门架、货叉、链条 | 变形、磨损、异响 |
| 电气系统 | 照明信号 | 灯光、喇叭、仪表、传感器 | 灯不亮、喇叭不响 |

### 门架角度参数

| 参数 | 角度范围 | 用途 |
|------|---------|------|
| 前倾角 | 3°-6° | 便于叉取和卸货 |
| 后倾角 | 6°-12° | 行驶时防止货物滑落 |
| 最大倾斜角 | 10° | 超过此角度有侧翻风险 |
', NULL, 'text', NULL, NULL, 40, 2, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (3, 1, '第三章 叉车主要技术参数', '## 第三章 叉车主要技术参数

理解叉车技术参数是正确选型和安全作业的基础。

### 起重量参数
- **额定起重量**：规定条件下允许起升的最大货物质量（最重要的性能参数）
- **载荷中心距**：货物质心到货叉垂直段前表面的水平距离，标准值 500mm（针对 2～5 吨叉车）
- **载荷曲线图**：当载荷中心距增大时，实际起重量需相应降低

### 速度与爬坡能力
- 内燃叉车满载行驶速度：15～20 km/h
- 电动叉车满载行驶速度：10～14 km/h
- 满载爬坡度：不低于 15%
- 空载爬坡度：不低于 20%

### 转向与起升
- 最小转弯半径：
  - 1.5 吨电动叉车约 1500～1800mm
  - 2.5 吨内燃叉车约 2100～2400mm
  - 前移式叉车约 1400～1600mm（窄通道优势）
- 起升高度：
  - 标准起升 3000mm：二节门架
  - 起升 5000mm：二节门架
  - 起升 7000mm 以上：三节门架

### 厂内限速规定
- 厂区主干道：10 km/h
- 仓库车间内：5 km/h
- 载货行驶货叉离地：10～20 cm，门架后倾

### 本章重点
额定起重量是最重要参数；载荷中心距标准 500mm；转弯半径决定窄通道通过能力；厂内限速 5/10 km/h。


---

### 技术参数速查表

| 参数 | 标准值 | 说明 |
|------|--------|------|
| 额定起重量 | 1-10吨 | 标准载荷中心距下的最大允许重量 |
| 载荷中心距 | 500mm | 货物质心到货叉垂直段前表面的水平距离 |
| 内燃满载速度 | 15-20 km/h | 平坦路面满载最高行驶速度 |
| 电动满载速度 | 10-14 km/h | 电动叉车满载最高行驶速度 |
| 满载爬坡度 | ≥15% | 满载时最大爬坡能力 |
| 空载爬坡度 | ≥20% | 空载时最大爬坡能力 |
| 厂区主干道限速 | 10 km/h | 厂区主干道限速规定 |
| 仓库车间限速 | 5 km/h | 仓库车间内限速规定 |
| 载货行驶货叉离地 | 10-20 cm | 载货行驶时货叉离地高度 |
| 转弯半径 | 1.5-2.4m | 最小转弯半径（视吨位而定） |
| 标准起升高度 | 3000mm | 二节门架标准起升 |
| 链条下垂量 | 1-2 cm | 起升链条正常松紧度 |

### 载荷曲线使用说明
当载荷中心距从500mm增加到600mm时，2.5吨叉车的实际安全起重量降至约2.1吨。
**务必查阅车身铭牌上的载荷曲线图**，不可仅凭额定起重量判断。
', NULL, 'text', NULL, NULL, 40, 3, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (4, 2, '第一章 液压传动基础原理', '## 第一章 液压传动基础原理

液压系统是叉车工作装置的核心驱动系统，理解其原理是维修保养的基础。

### 液压传动基本概念
- 以**液压油**为工作介质
- 利用液体的**压力能**传递动力和运动
- 核心原理：**帕斯卡定律**（密闭液体各方向压强相等）

### 液压系统五大组成
1. **动力元件**（液压泵）：将机械能转为液压能
2. **控制元件**（液压阀）：控制压力、流量、方向
3. **执行元件**（液压缸/液压马达）：将液压能转为机械能
4. **辅助元件**（油箱、滤油器、管路、蓄能器）
5. **工作介质**（液压油）

### 工作流程
- **吸油阶段**：液压泵从油箱吸入液压油
- **压油阶段**：液压泵将油液加压输出
- **控制阶段**：换向阀控制油液流向
- **执行阶段**：压力油驱动油缸动作
- **回油阶段**：工作后的油液返回油箱

### 关键参数
- 额定工作压力：14～17.5 MPa
- 正常油温：30℃～60℃（最高不超过 80℃）
- 常用液压油：L-HM46 抗磨液压油（运动粘度 46 mm²/s @40℃）

### 本章重点
帕斯卡定律；五大组成（动力/控制/执行/辅助/介质）；额定压力 14～17.5 MPa；油温 30～60℃。


---

### 液压系统参数速查表

| 参数 | 数值 | 说明 |
|------|------|------|
| 额定工作压力 | 14-17.5 MPa | 安全阀调定压力 |
| 正常油温 | 30-55°C | 最高不超过80°C |
| 液压油型号 | L-HM46 | 抗磨液压油，运动粘度46mm²/s@40°C |
| 换油周期 | 2000工作小时/1年 | 以先到者为准 |
| 滤芯更换 | 同步换油 | 更换液压油时必须同步更换滤芯 |

### 液压系统五大组成

| 组成 | 元件 | 作用 |
|------|------|------|
| 动力元件 | 液压泵 | 将机械能转为液压能 |
| 控制元件 | 液压阀 | 控制压力、流量、方向 |
| 执行元件 | 油缸/马达 | 将液压能转为机械能 |
| 辅助元件 | 油箱、滤油器、管路 | 储油、过滤、连接 |
| 工作介质 | 液压油 | 传递压力能 |
', NULL, 'text', NULL, NULL, 50, 1, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (5, 2, '第二章 液压元件结构与原理', '## 第二章 液压元件结构与原理

### 液压泵类型
- **齿轮泵**：结构简单、价格低廉、工作可靠，但流量脉动大（叉车最常用）
- **叶片泵**：流量均匀、噪音低，但结构复杂
- **柱塞泵**：压力高、效率高，但价格昂贵

### 液压阀
- **溢流阀**：限制系统最高压力，起安全保护作用（压力超过设定值时溢流）
- **液控单向阀（液压锁）**：安装在起升油缸进油路，防止货叉意外下落
- **多路换向阀**：中位时液压泵卸荷，减少能量消耗和系统发热

### 液压缸
- 起升油缸：驱动货叉升降
- 倾斜油缸：驱动门架前后倾斜

### 转向系统
- 全液压转向：转向盘 → 转向器 → 转向油缸 → 转向轮
- 发动机熄火时可人力转向（应急功能）

### 本章重点
齿轮泵最常用但脉动大；溢流阀限压保护；液控单向阀（液压锁）防货叉下落；多路阀中位卸荷。


---

### 液压泵类型对比表

| 泵类型 | 优点 | 缺点 | 适用场景 |
|--------|------|------|---------|
| 齿轮泵 | 结构简单、价格低、可靠 | 流量脉动大、噪音高 | 叉车（最常用） |
| 叶片泵 | 流量均匀、噪音低 | 结构复杂、价格高 | 精密设备 |
| 柱塞泵 | 压力高、效率高 | 价格昂贵、维护成本高 | 高压系统 |

### 液压阀功能对照表

| 阀类型 | 功能 | 故障影响 |
|--------|------|---------|
| 溢流阀（安全阀） | 限制系统最高压力 | 压力过低→起升无力；过高→管路爆裂 |
| 液控单向阀（液压锁） | 防止货叉自然下落 | 失效→货叉自动下沉 |
| 多路换向阀 | 控制油液流向，中位卸荷 | 卡滞→不起升；内泄→动作无力 |

### 关键安全提醒
- **严禁私自调高溢流阀压力**，否则管路爆裂、油缸损坏
- **维修前必须停机泄压**，高压油液可达14-17.5MPa，喷溅可致重伤
- 溢流阀压力调定需在专业试验台上进行
', NULL, 'text', NULL, NULL, 50, 2, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (6, 2, '第三章 液压系统维护与故障', '## 第三章 液压系统维护与故障

### 液压油维护
- 油位检查：油位低于油标规定时加注
- 油质检查：变质（发黑、浑浊、泡沫、异味）应及时更换
- 加注要求：原厂型号、标准油位、过滤无杂质、停机冷却后加注
- 严禁混用不同型号液压油

### 常见故障诊断
#### 货叉不起升
原因排查：液压油不足 → 液压泵故障 → 溢流阀调定压力过低 → 换向阀卡滞 → 起升油缸内泄

#### 货叉自然下落
原因：液控单向阀（液压锁）失效、油缸内泄、管路渗漏

#### 油温过高
原因：油液不足、散热不良、系统内泄、长时间超负荷作业

#### 系统噪声大
原因：吸油管路漏气、油液泡沫、滤油器堵塞、泵磨损

### 维修安全规程
- **维修前必须停机泄压**，防止高压油液喷溅伤人
- 拆卸元件前先释放系统压力
- 严禁带压作业，严禁私自调高溢流阀压力
- 维修后启动前检查油位、排除空气

### 本章重点
液压油变质四判断（黑/浑/泡/味）；货叉不起升五原因；维修前必停机泄压；禁止私调溢流阀。


---

### 液压油变质判断表

| 外观表现 | 原因 | 处理方式 |
|---------|------|---------|
| 颜色发黑 | 氧化变质 | 立即更换 |
| 浑浊有杂质 | 含水 | 立即更换 |
| 泡沫过多 | 空气混入 | 检查管路密封后更换 |
| 异味粘稠 | 高温分解 | 立即更换 |

### 液压系统常见故障速查表

| 故障现象 | 可能原因 | 排除方法 |
|---------|---------|---------|
| 货叉不起升 | ①液压油不足 ②泵故障 ③溢流阀压力低 ④换向阀卡滞 ⑤油缸内泄 | 按顺序排查 |
| 货叉自然下落 | ①液压锁失效 ②油缸内泄 ③管路渗漏 | 更换液压锁/密封件 |
| 油温过高 | ①油量不足 ②散热器堵塞 ③系统内泄 ④长时间超负荷 | 补油/清理散热/停机降温 |
| 系统噪声大 | ①吸油管漏气 ②油液泡沫 ③滤油器堵塞 ④泵磨损 | 检查密封/换油/清洗/换泵 |
| 起升无力 | ①溢流阀压力低 ②油缸内泄 ③泵磨损 | 调整压力/换密封/换泵 |

### 液压油加注四要求
1. 使用原厂型号（L-HM46）
2. 油位在标准区间
3. 过滤无杂质
4. 停机冷却后加注
- **严禁混用不同型号液压油**
', NULL, 'text', NULL, NULL, 50, 3, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (7, 3, '第一章 起步与行驶安全', '## 第一章 起步与行驶安全

### 起步操作规程
1. 观察四周环境，确认无人员和障碍物
2. 鸣笛示意，提醒周围人员
3. 系好安全带（如有配备）
4. 缓慢起步，避免急加速

### 行驶速度规定
- 厂区主干道：限速 10 km/h
- 仓库车间内：限速 5 km/h
- 弯道、路口、狭窄通道：减速慢行
- 与前车保持 3 米以上安全距离

### 载货行驶要求
- 货叉离地 10～20 cm，门架后倾
- 货物不得超高遮挡视线
- 大件货物倒车行驶（有人指挥）
- 严禁超载、偏载、超速

### 禁止行为
- 严禁酒后驾驶、疲劳驾驶
- 严禁行驶中接打电话
- 严禁货叉上载人
- 严禁急刹车、急转弯
- 严禁空挡溜车

### 本章重点
厂内限速 5/10 km/h；载货行驶货叉离地 10～20 cm；门架后倾；保持 3 米车距。


---

### 厂区行驶速度规定

| 区域 | 限速 | 说明 |
|------|------|------|
| 仓库车间内 | 5 km/h | 人员密集、空间狭小 |
| 厂区主干道 | 10 km/h | 视线较好但需注意交叉路口 |
| 十字路口 | 5 km/h以下 | 一慢二看三通过 |
| 倒车 | 3 km/h | 视野受限，低速确保安全 |
| 载货行驶 | 适当降低 | 货物增加制动距离 |

### 载货行驶三要素
1. **货叉离地10-20cm**：过低刮碰地面，过高影响稳定
2. **门架后倾**：后倾角6°-12°，防止货物滑落
3. **保持3米以上安全距离**：前车紧急制动时有足够反应时间

### 严禁行为清单
- 酒后驾驶、疲劳驾驶
- 行驶中接打电话
- 货叉上载人
- 急刹车、急转弯
- 空挡溜车
- 超载、偏载、超速
', NULL, 'text', NULL, NULL, 40, 1, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (8, 3, '第二章 坡道与转弯作业', '## 第二章 坡道与转弯作业

### 坡道作业规程
- **上坡**：正向行驶（货叉朝前）
- **下坡**：倒车行驶（货叉朝后，防止货物前倾）
- 严禁坡道掉头、转弯
- 严禁空挡溜车下坡
- 坡道停车：脚刹 + 手刹 + 落叉 + 挂挡

### 转弯作业规程
- 提前减速、鸣笛、慢行
- 注意内侧盲区，防止刮碰
- 转弯半径越小越易侧翻，重载转弯尤其危险
- 严禁高速转弯

### 倒车作业
- 倒车前观察后方
- 必要时下车查看或有人指挥
- 倒车时低速慢行
- 倒车过程中注意盲区

### 视线不良情况
- 货物超高遮挡视线：倒车行驶
- 拐角盲区：鸣笛低速通过
- 雨雪大雾：停止作业或加强照明
- 视线受阻：必须有人指挥

### 本章重点
上坡正向、下坡倒车；坡道禁止掉头转弯；转弯提前减速鸣笛；视线不良必须有人指挥。


---

### 坡道作业操作速查表

| 操作场景 | 正确做法 | 禁止行为 |
|---------|---------|---------|
| 上坡 | 正向行驶（货叉朝前） | 中途换挡 |
| 下坡 | 倒车行驶（货叉朝后） | 正向冲坡、空挡溜车 |
| 坡道停车 | 脚刹+手刹+落叉+垫三角木 | 仅靠手刹 |
| 坡道转弯 | 禁止——驶至平坦区域后操作 | 坡道掉头/转弯 |

### 转弯安全参数

| 参数 | 数值 | 说明 |
|------|------|------|
| 最大允许爬坡角度 | 10%-15% | 满载时的爬坡能力 |
| 转弯提前减速距离 | 5米以上 | 根据速度和载重判断 |
| 重载转弯速度 | 比空载更低 | 重载离心力大，侧翻风险高 |

### 倒车操作规范
1. 倒车前观察后方环境
2. 提前鸣笛警示
3. 低速行驶（不超过3km/h）
4. 后方视线受阻时必须有人指挥
5. 禁止高速倒车、不观察盲区
', NULL, 'text', NULL, NULL, 40, 2, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (9, 3, '第三章 停车与会车规范', '## 第三章 停车与会车规范

### 停车规范
- 停放指定区域，禁止占用：
  - 消防通道
  - 路口
  - 坡道
  - 盲区
- 停车步骤：
  1. 货叉完全落地
  2. 拉紧手刹（驻车制动）
  3. 挂空挡
  4. 断电熄火
  5. 拔下钥匙
- 驾驶员离车必须拔钥匙

### 会车规则
- 靠右减速
- 窄道一方停车礼让
- 空载让重载
- 小车让大车
- 支线让干线
- 下坡车让上坡车

### 超车规定
- 厂区内原则上禁止超车
- 必须超车时：鸣笛示意、确认安全、左侧超车
- 弯道、坡道、路口、窄道严禁超车

### 装卸场地通行
- 按规定路线行驶
- 注意行人警示标志
- 倒车时鸣笛警示
- 装卸月台靠近时减速

### 本章重点
停车五步（落叉/手刹/空挡/熄火/拔钥）；会车靠右减速；空载让重载；下坡让上坡。


---

### 停车规范五步法

| 步骤 | 操作 | 目的 |
|------|------|------|
| ① | 货叉完全落地 | 防止意外下落伤人 |
| ② | 拉紧手刹（驻车制动） | 防止溜车 |
| ③ | 挂空挡 | 防止意外启动 |
| ④ | 断电熄火 | 切断动力 |
| ⑤ | 拔下钥匙 | 防止未授权使用 |

### 禁止停车位置

| 禁停区域 | 原因 |
|---------|------|
| 消防通道 | 阻碍应急救援 |
| 路口拐角 | 影响其他车辆视线 |
| 坡道斜坡 | 溜车风险 |
| 人员密集通道 | 阻碍通行 |
| 盲区位置 | 易被碰撞 |

### 会车避让规则表

| 规则 | 说明 |
|------|------|
| 靠右减速 | 基本原则 |
| 空载让重载 | 重载制动距离长 |
| 小车让大车 | 大车视野差、制动距离长 |
| 支线让干线 | 干线车辆速度快 |
| 下坡让上坡 | 上坡车爬坡中途停车困难 |
| 窄道一方停车 | 禁止互不相让抢行 |
| 同向行驶间距≥5米 | 防止追尾 |
', NULL, 'text', NULL, NULL, 40, 3, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (10, 4, '第一章 出车前必检项目', '## 第一章 出车前必检项目

### 十字作业法
叉车每班次出车前必须执行"十字作业法"：**清洁、润滑、紧固、调整、防腐**

### 必检项目清单
#### 1. 轮胎
- 气压：符合规定值
- 花纹磨损：深度不足需更换
- 鼓包、开裂、异物：立即更换
- 轮毂螺栓：紧固无松动

#### 2. 制动系统
- 刹车踏板自由行程：符合规定
- 制动液位：在 MIN-MAX 之间
- 刹车效果：低速测试
- 手刹效能：坡道测试

#### 3. 转向系统
- 方向盘自由转动量：不超过规定值
- 转向灵活性：左右转动顺畅
- 转向油位：液面正常
- 转向节：无异响

#### 4. 液压系统
- 液压油位：油标规定范围
- 管路渗漏：无渗漏
- 油温：常温启动
- 工作压力：空载测试

#### 5. 灯光喇叭
- 前照灯、尾灯、转向灯、警示灯、倒车灯
- 喇叭：声响正常

#### 6. 链条
- 松紧度：下垂量 1～2 cm
- 变形、锈蚀：无
- 润滑：定期加油

### 检查记录
- 每班次填写《叉车点检表》
- 发现异常立即上报
- 严禁带病作业

### 本章重点
十字作业法（清洁/润滑/紧固/调整/防腐）；链条下垂 1～2 cm；制动液位 MIN-MAX；带病作业严禁。


---

### 出车前必检项目清单

| 检查项目 | 检查内容 | 判定标准 |
|---------|---------|---------|
| 轮胎 | 气压、花纹、鼓包、开裂 | 花纹深度≥1.6mm，无鼓包开裂 |
| 制动 | 踏板行程、制动液位、制动效果 | 液位在MIN-MAX之间，制动有效 |
| 转向 | 方向盘自由量、转向灵活性 | 自由量不超标，转向顺畅无异响 |
| 液压 | 油位、管路渗漏、油温 | 油位正常，无渗漏 |
| 灯光 | 前照灯、尾灯、转向灯、警示灯、倒车灯 | 全部功能正常 |
| 喇叭 | 声响 | 声响正常 |
| 链条 | 松紧度、变形、锈蚀 | 下垂量1-2cm，无变形锈蚀 |
| 护顶架 | 完整性、紧固 | 无变形、螺栓紧固 |
| 安全带 | 完好性、锁扣功能 | 无断裂、锁扣有效 |

### 十字作业法详解

| 步骤 | 内容 | 频率 |
|------|------|------|
| 清洁 | 清除车身、底盘、链条油污杂物 | 每班次 |
| 润滑 | 链条加注黄油、转向节润滑 | 每日/每周 |
| 紧固 | 检查轮胎螺栓、管路接头 | 每日 |
| 调整 | 制动踏板行程、链条松紧度 | 按需 |
| 防腐 | 补漆防锈、电瓶接线柱涂抹凡士林 | 每周/每月 |
', NULL, 'text', NULL, NULL, 40, 1, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (11, 4, '第二章 液压与制动系统保养', '## 第二章 液压与制动系统保养

### 液压系统保养
#### 液压油更换
- 更换周期：按厂家规定（一般 2000 工作小时或 1 年）
- 油质判断：
  - 发黑：氧化变质
  - 浑浊：含水
  - 泡沫：空气混入
  - 异味：高温分解
- 加注要求：原厂型号、标准油位、过滤无杂质、停机冷却后加注
- 严禁混用不同型号液压油

#### 滤油器维护
- 定期清洗或更换滤芯
- 堵塞会导致吸油阻力增大、泵噪声大
- 更换液压油时同步更换滤芯

#### 管路检查
- 检查橡胶管老化、开裂、鼓包
- 检查接头松动、渗漏
- 发现问题及时更换

### 制动系统保养
#### 刹车油更换
- 更换周期：按厂家规定
- 油液型号：同型号（DOT3/DOT4）
- 更换后必须**排空空气**
- 更换后测试制动效果

#### 制动片检查
- 厚度不足需更换
- 异常磨损需排查原因
- 严禁制动片磨损到金属

#### 制动系统故障判断
- 制动发软：管路有空气
- 制动偏刹：左右制动片磨损不均
- 制动卡顿：制动钳卡滞
- 制动异响：制动片磨损到极限

### 安全阀维护
- 工作压力由安全阀控制
- **严禁私自调高溢流阀压力**
- 维修需专业人员在试验台上调整

### 本章重点
液压油变质四判断（黑/浑/泡/味）；刹车油更换后排空气；禁止私调溢流阀；滤芯同步更换。


---

### 液压油维护周期表

| 项目 | 周期 | 要求 |
|------|------|------|
| 油位检查 | 每班次 | 油位在油标规定范围 |
| 油质检查 | 每周 | 观察颜色、浑浊度、泡沫、异味 |
| 滤芯更换 | 2000小时/1年 | 与液压油同步更换 |
| 液压油更换 | 2000小时/1年 | 使用L-HM46，停机冷却后加注 |
| 管路检查 | 每周 | 检查橡胶管老化、开裂、鼓包、接头松动 |

### 制动系统保养周期表

| 项目 | 周期 | 要求 |
|------|------|------|
| 制动液位检查 | 每班次 | 液位在MIN-MAX之间 |
| 制动片厚度 | 每月 | 磨损不超过原厚10% |
| 刹车油更换 | 按厂家规定 | 同型号(DOT3/DOT4)，排空空气 |
| 制动效果测试 | 每班次 | 低速测试制动灵敏度 |

### 安全阀维护红线
- **严禁私自调高溢流阀压力**
- 工作压力由安全阀控制（额定14-17.5MPa）
- 维修需专业人员在试验台上调整
- 私调导致管路爆裂、油缸损坏，后果严重
', NULL, 'text', NULL, NULL, 40, 2, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (12, 4, '第三章 电瓶与轮胎维护', '## 第三章 电瓶与轮胎维护

### 蓄电池维护（电动叉车）
#### 电解液检查
- 液面应高于极板 10～15 mm
- 液面低于极板时补充**蒸馏水**（严禁自来水、电解液、矿泉水）
- 补水后静置 30 分钟再充电

#### 充电规范
- 充电环境通风、远离明火
- 充电温度：5～30 ℃
- 严禁过充（导致电池鼓包、发热）
- 严禁亏电长期停放（导致极板硫化）
- 充电时打开电池盖排气

#### 电池寿命管理
- 循环寿命：约 1500 次
- 鼓包原因：过充过热
- 硫化原因：长期亏电
- 极板活性物质脱落：过充大电流

#### 电池安全
- 严禁明火靠近
- 电池着火严禁用水扑救（应使用干粉灭火器）
- 电解液溅到皮肤：立即用大量清水冲洗

### 轮胎维护
#### 气压管理
- 按规定气压充气
- 气压过高：胎面中部磨损、易爆胎
- 气压过低：胎肩磨损、油耗增加、转向沉重

#### 磨损检查
- 花纹深度不足 1.6 mm 需更换
- 异常磨损需排查原因（四轮定位、轴承、气压）
- 局部磨损严重需调位

#### 轮胎损坏判断
- 鼓包：帘布层断裂，立即更换
- 开裂：老化或外伤，及时更换
- 异物刺入：拔出后补胎或更换
- 胎侧损伤：必须更换（不可补）

### 内燃叉车发动机维护
- 机油作用：润滑、冷却、密封、防锈、缓冲
- 水温正常：80～90 ℃
- 冷车启动：怠速预热 3～5 分钟
- 排气颜色判断：
  - 黑烟：燃烧不充分
  - 蓝烟：烧机油
  - 白烟：含水或冷启动

### 本章重点
电瓶补水加蒸馏水；充电通风远离明火；电池着火禁用水；花纹不足 1.6 mm 更换；水温 80～90 ℃。


---

### 蓄电池维护速查表

| 项目 | 正常标准 | 异常处理 |
|------|---------|---------|
| 电解液液面 | 高于极板10-15mm | 补充蒸馏水（非自来水/电解液） |
| 补水后静置 | 30分钟 | 静置后方可充电 |
| 充电温度 | 5-30°C | 超出范围停止充电 |
| 充电环境 | 通风、远离明火 | 氢气易爆，严禁烟火 |
| 循环寿命 | 约1500次 | 到期评估更换 |

### 电池故障诊断表

| 故障现象 | 可能原因 | 预防/处理 |
|---------|---------|---------|
| 电池鼓包 | 过充过热 | 避免过充，检查充电器 |
| 极板硫化 | 亏电长期停放 | 定期补电，不可亏电存放 |
| 活性物质脱落 | 过充大电流 | 使用标准充电电流 |
| 续航变短(冬季) | 低温化学反应减速 | 室内充电保温 |
| 突然断电 | 接线松动/熔断丝 | 紧固接线/更换熔断丝 |
| 电池着火 | 短路/过充 | 干粉灭火器，**严禁用水** |

### 轮胎检查标准

| 检查项 | 正常标准 | 更换标准 |
|--------|---------|---------|
| 气压 | 符合规定值 | 过高→胎面中部磨损；过低→胎肩磨损 |
| 花纹深度 | ≥1.6mm | <1.6mm需更换 |
| 胎面鼓包 | 无 | 鼓包=帘布层断裂，立即更换 |
| 胎侧开裂 | 无 | 老化或外伤，及时更换 |
| 胎侧损伤 | 无 | 必须更换（不可补） |

### 内燃叉车排气颜色诊断

| 排气颜色 | 原因 | 处理 |
|---------|------|------|
| 黑烟 | 燃烧不充分（空滤堵塞/喷油多） | 清洁/更换空滤，检查喷油器 |
| 蓝烟 | 烧机油（活塞环磨损/气门油封失效） | 检修发动机 |
| 白烟 | 含水或冷启动正常雾化 | 冬季正常，持续白烟检查汽缸垫 |
', NULL, 'text', NULL, NULL, 40, 3, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (13, 5, '第一章 货物叉取与起升', '## 第一章 货物叉取与起升

### 叉取前准备
- 检查货物重量（不得超过额定起重量）
- 检查货物包装完整性
- 调整货叉间距（与托盘适配、对称居中）
- 确认货物重心位置

### 叉取操作
1. 货叉水平插入托盘
2. 插入深度：**托盘的 2/3 以上**
3. 微微起升（10 cm）确认稳定
4. 门架后倾至最大角度
5. 缓慢后退离开货位

### 起升操作
- 平稳缓慢升降，禁止猛升猛降
- 起升后稍作停顿检查平稳性
- 起升过程中注意上方障碍物
- 货叉下严禁人员停留、穿行

### 货物稳定性判断
- 货物重心居中
- 捆绑牢固、无松动
- 包装完整、无破损
- 偏载、散装货物需特殊处理

### 禁止叉运的货物
- 深埋固定重物
- 松散散料（无托盘）
- 超长超限货物（需特殊装置）
- 不明重量货物
- 易燃易爆危险品（需防爆叉车）

### 本章重点
货叉插入 2/3 以上；起升后门架后倾；货叉下严禁站人；重心居中、捆绑牢固。


---

### 货物叉取操作步骤

| 步骤 | 操作 | 要点 |
|------|------|------|
| ①调整 | 货叉间距与托盘适配、对称居中 | 禁止一宽一窄 |
| ②插入 | 货叉水平插入托盘 | 插入深度≥托盘2/3 |
| ③确认 | 微微起升10cm检查稳定 | 货物无倾斜、无松动 |
| ④后倾 | 门架后倾至最大角度 | 后倾角6°-12° |
| ⑤离开 | 缓慢后退离开货位 | 低速、观察四周 |

### 货物稳定性判断

| 检查项 | 合格标准 | 不合格处理 |
|--------|---------|-----------|
| 重心位置 | 居中 | 偏载→重新摆放或拒绝叉运 |
| 捆绑状态 | 牢固无松动 | 松散→重新捆绑后起升 |
| 包装完整 | 无破损 | 破损→加固包装或拒运 |
| 重量判断 | 不超额定起重量 | 超重→分拆或换大车 |

### 禁止叉运的货物清单
- 深埋固定重物（会损坏货叉和门架）
- 松散散料（无托盘，易散落）
- 超长超限货物（需特殊装置）
- 不明重量货物（有超载风险）
- 易燃易爆危险品（需防爆叉车）
', NULL, 'text', NULL, NULL, 40, 1, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (14, 5, '第二章 堆垛与装卸作业', '## 第二章 堆垛与装卸作业

### 堆垛作业规程
1. 垂直对位货架
2. 低速缓慢升降
3. 到位后门架前倾至水平
4. 缓慢下降至货架
5. 货叉退出托盘
6. 货叉下降至离地 10～20 cm

### 货物码放原则
- **上轻下重**：重物在下，轻物在上
- **居中码放**：不偏不倚
- **整齐稳固**：堆叠整齐
- **高度限制**：不超过规定高度
- **通道留出**：便于取放

### 高位作业安全
- 高位作业禁止人员靠近
- 禁止高空急转、急落
- 禁止大幅度倾斜
- 起升最大倾斜角度 10 度
- 高位作业需有人指挥

### 装卸车辆作业
- 货车制动可靠（垫块）
- 货叉与车厢对正
- 装卸顺序：先外后内、先下后上
- 月台靠近时减速
- 装卸完毕确认货叉清空

### 危险情况处理
- 货物倾斜：立即停止、缓慢放下
- 货物滑落：远离作业区、清理后重新作业
- 货叉损坏：立即停用、更换
- 货架损坏：停止作业、上报

### 本章重点
上轻下重居中码放；高位作业禁止急转急落；最大倾斜 10 度；月台靠近减速。


---

### 堆垛作业操作步骤

| 步骤 | 操作 | 要点 |
|------|------|------|
| ①对位 | 垂直对位货架 | 靠近货架、低速 |
| ②升降 | 低速缓慢升降到目标位置 | 禁止猛升猛降 |
| ③前倾 | 门架前倾至水平 | 前倾角3°-6° |
| ④落货 | 缓慢下降至货架 | 确认货物平稳 |
| ⑤退出 | 货叉退出托盘 | 低速、确认货叉清空 |
| ⑥复位 | 货叉下降至离地10-20cm | 门架后倾，准备行驶 |

### 货物码放原则

| 原则 | 说明 | 违反后果 |
|------|------|---------|
| 上轻下重 | 重物在下、轻物在上 | 上重下轻→倒塌风险 |
| 居中码放 | 不偏不倚 | 偏载→侧翻风险 |
| 整齐稳固 | 堆叠整齐 | 不稳→运输中倒塌 |
| 高度限制 | 不超过3-4层 | 过高→倒塌风险 |
| 通道留出 | 便于取放 | 无通道→取放困难 |

### 高位作业安全参数
- 最大倾斜角度：10度
- 高位作业必须有人指挥
- 禁止高空急转、急落、大幅度倾斜
- 起升最大高度参考载荷曲线图
', NULL, 'text', NULL, NULL, 40, 2, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (15, 5, '第三章 视线盲区与安全作业', '## 第三章 视线盲区与安全作业

### 叉车视线盲区
- **前方盲区**：货物超高时遮挡前方视线
- **后方盲区**：倒车时后方盲区大
- **侧方盲区**：转弯时内侧盲区
- **高位盲区**：高位起升时上方盲区

### 视线不良应对
#### 货物超高遮挡视线
- 倒车低速行驶
- 必要时有人指挥
- 严禁强行前进

#### 拐角盲区
- 鸣笛低速通过
- 减速慢行
- 注意突然出现的人员

#### 倒车作业
- 倒车前观察后方
- 必要时下车查看
- 有人指挥配合
- 低速慢行

### 作业区域安全
#### 人员管理
- 作业区设置警示标志
- 无关人员禁止进入
- 行人优先礼让
- 严禁货叉上载人

#### 环境要求
- 雨雪大雾停止作业
- 昏暗库房加强照明
- 积水深坑绕行
- 结冰路面停止作业

#### 特殊作业
- 狭窄通道：低速慢行、注意刮碰
- 高位货架：有人指挥、确认对位
- 危险品：使用防爆叉车、专人监护
- 集装箱作业：注意自由起升高度

### 应急处置
- 货物倾倒：立即停车、远离现场、上报处理
- 人员受伤：立即救人、保护现场、报告急救
- 车辆故障：停止作业、设立警示、上报维修

### 本章重点
货物超高倒车行驶；拐角鸣笛低速；雨雪结冰停止作业；作业区设置警示标志。


---

### 叉车盲区分布表

| 盲区类型 | 产生原因 | 应对措施 |
|---------|---------|---------|
| 前方盲区 | 货物超高遮挡视线 | 倒车低速行驶，有人指挥 |
| 后方盲区 | 倒车时后方视野受限 | 提前鸣笛、观察、有人指挥 |
| 侧方盲区 | 转弯时内侧内轮差 | 提前减速、注意内侧 |
| 高位盲区 | 高位起升时上方不可见 | 确认上方无障碍物 |

### 特殊环境作业规定

| 环境 | 规定 | 原因 |
|------|------|------|
| 雨雪天气 | 减速慢行、避免急刹急转 | 地面湿滑、制动距离变长 |
| 结冰路面 | 停止室外作业 | 侧滑失控风险极大 |
| 大雾天气 | 停止作业或加强照明 | 能见度不足 |
| 昏暗库房 | 加强照明 | 视线不清易碰撞 |
| 狭窄通道 | 低速慢行、注意刮碰 | 空间受限 |
| 人员密集区 | 设置警示标志、禁止穿行 | 行人安全优先 |

### 货物超高应对方案
| 情况 | 处理方式 |
|------|---------|
| 货物超高遮挡前方视线 | 倒车低速行驶 |
| 后方视线也被遮挡 | 必须有人指挥 |
| 无法安全通过 | 分拆搬运或换用前移式叉车 |
', NULL, 'text', NULL, NULL, 40, 3, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (16, 6, '第一章 制动系统故障诊断', '## 第一章 制动系统故障诊断

### 叉车制动系统组成
- 行车制动（脚制动）：制动踏板 → 制动总泵 → 制动分泵 → 制动蹄片 → 制动鼓
- 驻车制动（手制动）：手制动杆 → 机械拉线 → 制动蹄片锁紧
- 两套独立系统，互不影响

### 常见故障诊断
#### 制动失灵（紧急情况）
- 现象：踩下制动踏板无制动效果
- 应急处置：
  1. 松油门、降低速度
  2. 利用发动机牵阻减速
  3. 低速摩擦障碍物减速（护栏、路沿）
  4. 平稳靠边停机
  5. **禁止急打方向**（防止侧翻）
- 故障原因：制动液不足、制动管路破裂、制动总泵失效

#### 制动发软
- 原因：制动管路有空气
- 排除：排气作业（排出管路空气）

#### 制动偏刹
- 现象：制动时车辆跑偏
- 原因：左右制动片磨损不均、制动分泵卡滞
- 排除：调整或更换制动片

#### 制动卡顿
- 现象：松开制动后制动片不回位
- 原因：制动钳卡滞、回位弹簧失效
- 排除：清洗或更换制动钳

#### 制动异响
- 现象：制动时金属摩擦声
- 原因：制动片磨损到极限
- 排除：立即更换制动片

### 制动液维护
- 型号：DOT3 / DOT4（同型号不可混用）
- 更换周期：按厂家规定
- 更换后必须排空空气
- 液位：MIN-MAX 之间

### 本章重点
制动失灵松油门低速摩擦减速、禁急打方向；制动发软需排气；偏刹调制动片；异响换片。


---

### 制动系统故障诊断表

| 故障现象 | 可能原因 | 排除方法 | 紧急程度 |
|---------|---------|---------|---------|
| 制动失灵 | 制动液不足/管路破裂/总泵失效 | 松油门→摩擦减速→靠边停机 | ⚠️紧急 |
| 制动发软 | 管路有空气 | 排气作业 | 高 |
| 制动偏刹 | 左右制动片磨损不均/分泵卡滞 | 调整或更换制动片 | 高 |
| 制动卡顿 | 制动钳卡滞/回位弹簧失效 | 清洗或更换制动钳 | 中 |
| 制动异响 | 制动片磨损到极限 | 立即更换制动片 | 高 |
| 制动踏板行程过大 | 间隙过大/制动片磨损 | 调整间隙或更换制动片 | 中 |

### 制动失灵应急处置流程
```
发现制动失灵
    ↓
松油门降低速度
    ↓
利用发动机牵阻减速（降挡）
    ↓
低速摩擦障碍物减速（护栏/路沿）
    ↓
平稳靠边停机
    ↓
熄火、设置警示、报修
    ↓
⚠️ 全程禁止急打方向（防侧翻）
```

### 刹车油更换要点
1. 型号：DOT3 或 DOT4（**不可混用**）
2. 更换后必须**排空油路空气**
3. 更换后必须**测试制动效果**
4. 排气顺序：先远后近（右后→左后→右前→左前）
', NULL, 'text', NULL, NULL, 50, 1, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (17, 6, '第二章 液压系统故障排除', '## 第二章 液压系统故障排除

### 货叉不起升故障
#### 故障原因排查（按顺序）
1. **液压油不足**：检查油位、补充液压油
2. **液压泵故障**：泵磨损、内泄、不泵油
3. **溢流阀调定压力过低**：调整或更换溢流阀
4. **换向阀卡滞**：清洗或更换换向阀
5. **起升油缸内泄**：更换油缸密封件

### 货叉自然下落故障
- 原因：液控单向阀（液压锁）失效
- 排除：更换液压锁
- 次要原因：油缸内泄、管路渗漏

### 油温过高故障
- 原因：
  - 油液不足
  - 散热器堵塞
  - 系统内泄
  - 长时间超负荷作业
- 排除：补充油液、清理散热器、检修内泄、停机降温

### 系统噪声大
- 原因：
  - 吸油管路漏气
  - 油液泡沫
  - 滤油器堵塞
  - 液压泵磨损
- 排除：检查管路密封、更换油液、清洗滤芯、更换泵

### 起升无力
- 原因：溢流阀压力调定过低、油缸内泄、泵磨损
- 排除：调整压力、更换密封、更换泵

### 门架自动下滑
- 原因：油缸内泄、液压锁失效
- 排除：更换密封件、更换液压锁

### 维修安全规程
- **维修前必须停机泄压**
- 严禁带压拆卸元件
- 严禁私自调高溢流阀压力
- 维修后启动前检查油位、排除空气
- 高压油液喷溅可致重伤

### 本章重点
货叉不起升五原因排查（油/泵/阀/换向/缸）；油温过高四原因；维修前必停机泄压。


---

### 液压系统故障诊断流程

#### 货叉不起升排查
```
货叉不起升
    ↓
① 检查液压油位 → 不足则补充
    ↓
② 检查液压泵 → 泵磨损/内泄则更换
    ↓
③ 检查溢流阀压力 → 过低则调整或更换
    ↓
④ 检查换向阀 → 卡滞则清洗或更换
    ↓
⑤ 检查起升油缸 → 内泄则更换密封件
```

#### 液压系统故障速查表

| 故障现象 | 首查原因 | 次查原因 | 排除方法 |
|---------|---------|---------|---------|
| 货叉不起升 | 液压油不足 | 泵/阀/缸故障 | 补油→按流程排查 |
| 货叉自然下落 | 液压锁失效 | 油缸内泄/管路渗漏 | 更换液压锁/密封件 |
| 油温过高 | 油量不足 | 散热器堵塞/内泄/超负荷 | 补油/清理/停机降温 |
| 系统噪声大 | 吸油管漏气 | 油液泡沫/滤芯堵/泵磨损 | 检查密封/换油/清洗/换泵 |
| 起升无力 | 溢流阀压力低 | 油缸内泄/泵磨损 | 调整压力/换密封/换泵 |
| 门架自动下滑 | 油缸内泄 | 液压锁失效 | 更换密封件/液压锁 |

### 维修安全红线
- **维修前必须停机泄压**，高压油液可达14-17.5MPa
- **严禁带压拆卸油管**，高压油液喷溅可致重伤
- **严禁私自调高溢流阀压力**
- 维修后启动前检查油位、排除空气
- 拆卸元件前先释放系统压力
', NULL, 'text', NULL, NULL, 50, 2, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;
INSERT INTO chapter (chapter_id, course_id, title, content, content_url, content_type, file_url, description, duration, order_num, created_at) OVERRIDING SYSTEM VALUE VALUES (18, 6, '第三章 应急处置与突发情况', '## 第三章 应急处置与突发情况

### 叉车侧翻应急处置
#### 侧翻瞬间（关键 3 秒）
- **紧握方向盘**
- **身体向内倾**（向叉车倾翻的反方向）
- **禁止跳车**
- **禁止松手下车**

#### 侧翻原因
- 高速转弯
- 重载急转
- 货物偏载
- 路面倾斜
- 轮胎爆胎

### 突发天气应对
#### 暴雨
- 就近安全位置停靠
- 禁止高速冲回库房
- 雨后检查制动效能

#### 雨雪湿滑
- 减速慢行
- 避免急刹急转
- 积水深坑绕行
- 增加安全距离

#### 结冰
- 停止作业
- 必须作业时加装防滑链

#### 大雾
- 停止作业或加强照明
- 必要时有人指挥

### 火灾应急处置
1. 立即停机、断电、熄火
2. 切断电源（电瓶叉车）
3. 使用灭火器扑救
4. 电瓶叉车**严禁用水扑救**（用干粉灭火器）
5. 火势不可控立即撤离、报警

### 伤人事故处理
1. 立即停机、保护现场
2. 先救人后保物
3. 拨打 120 急救
4. 上报管理部门
5. 配合事故调查

### 制动失灵应急处置
- 松油门降低速度
- 利用发动机牵阻减速
- 低速摩擦障碍物减速
- 平稳靠边停机
- **禁止急打方向**

### 转向失灵应急处置
- 立即松油门
- 利用人力转向（应急功能）
- 缓慢减速停车
- 严禁急刹

### 本章重点
侧翻紧握方向盘身体向内倾、禁跳车；电瓶着火禁用水；制动失灵松油门低速摩擦、禁急打方向。


---

### 叉车侧翻应急处置

#### 侧翻瞬间操作（关键3秒）
```
感觉到侧翻
    ↓
① 紧握方向盘（固定身体位置）
    ↓
② 身体向内倾（向倾翻反方向）
    ↓
③ 禁止跳车！禁止松手！
    ↓
等待车辆稳定后从安全侧逃生
```

| 错误做法 | 后果 |
|---------|------|
| 跳车逃生 | 被车辆压砸，死亡率极高 |
| 松手下车 | 无法控制身体，被甩出车外 |
| 往外跳 | 被护顶架和地面夹击 |

#### 侧翻常见原因

| 原因 | 预防措施 |
|------|---------|
| 高速转弯 | 提前减速，遵守限速 |
| 重载急转 | 重载转弯比空载更危险，减速至最低 |
| 货物偏载 | 确保货物重心居中 |
| 路面倾斜 | 避免在坡道/不平路面转弯 |
| 轮胎爆胎 | 出车前检查轮胎气压和完好性 |

### 突发天气应对速查表

| 天气 | 处理方式 | 禁止行为 |
|------|---------|---------|
| 暴雨 | 就近安全位置停靠 | 高速冲回库房 |
| 雨雪湿滑 | 减速慢行、避免急刹急转 | 急刹车 |
| 积水 | 绕行深坑 | 涉水超过轮胎1/2 |
| 结冰 | 停止室外作业 | 强行作业 |
| 大雾 | 停止作业或加强照明 | 凭经验盲开 |

### 火灾应急处置流程
```
发现火情
    ↓
立即停机、断电、熄火
    ↓
切断电源（电瓶叉车）
    ↓
使用灭火器扑救
    ↓
电瓶叉车→干粉灭火器（⚠️严禁用水）
    ↓
火势不可控→立即撤离、报警
```

### 事故处理基本原则
1. **先救人后保物**——人员生命优先
2. **保护现场**——便于事故调查
3. **及时上报**——配合管理部门处理
4. **拨打120急救**——如有人员受伤
', NULL, 'text', NULL, NULL, 50, 3, now()) ON CONFLICT (chapter_id) DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content, duration = EXCLUDED.duration, order_num = EXCLUDED.order_num;

-- ============================================================
-- 4. 题目筛选与分类
-- ============================================================
-- 筛选规则：剔除纯法规/证件管理类（与维修技术无关），保留结构原理/维护/故障/安全操作 4 类
-- 知识点分配：按题干关键词匹配到 6 个知识点（对应 6 门课程）
-- 难度判定：记忆→beginner，理解→intermediate，应用→advanced（每知识点 3 档全覆盖）

-- 清空已有 N1 题目（避免重复导入）

-- 输入题目：503 道；剔除法规类：26 道；保留：477 道
-- level 分布：beginner=381, intermediate=41, advanced=55
-- 知识点 × level 分布：
--   kp1 叉车结构与基础: beg=143, int=25, adv=11
--   kp2 液压与动力系统: beg=11, int=5, adv=9
--   kp3 安全操作规程: beg=124, int=2, adv=10
--   kp4 日常检查与保养: beg=44, int=5, adv=7
--   kp5 货叉作业技能: beg=53, int=3, adv=3
--   kp6 故障诊断与排除: beg=6, int=1, adv=15

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, 'true_false', '叉车属于场（厂）内专用机动车辆，属于特种设备。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第1题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(2, 'true_false', '无证人员可以在老司机陪同下临时驾驶叉车。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第3题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(3, 'true_false', '叉车作业时可以载人行驶、站在货叉上作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第4题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(4, 'true_false', '载货行驶时货叉应离地10–20cm，门架后倾。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第5题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(5, 'true_false', '仓库、车间内叉车行驶限速5km/h以内。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第6题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(6, 'true_false', '叉车可以超载、偏载、单边叉货。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第7题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(7, 'true_false', '叉车转弯时应提前减速、鸣笛、禁止急转弯。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第8题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(8, 'true_false', '坡道上行驶严禁熄火、空挡溜车。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第9题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(9, 'true_false', '叉车下坡应倒车慢行，禁止正向冲坡下坡。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第10题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(10, 'true_false', '叉车作业前只需检查燃油，不用检查制动和转向。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第11题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(11, 'true_false', '出车前应检查轮胎、刹车、灯光、链条、液压有无渗漏。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第12题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(12, 'true_false', '叉车起升货物时应平稳起升，禁止猛升猛降。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第13题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(13, 'true_false', '货物超高、超宽、超长可以强行搬运。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第14题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(14, 'true_false', '夜间作业必须开启叉车照明灯。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第15题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(15, 'true_false', '叉车行驶中可以随意变道、穿插人流。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第16题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(16, 'true_false', '发现叉车制动失灵应立即靠边停车，熄火报修。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第17题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(17, 'true_false', '电瓶叉车可以在易燃易爆场所普通使用，不用防爆型。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第18题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(18, 'true_false', '内燃叉车可以在密闭库房长时间怠速运转。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第19题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(19, 'true_false', '叉车停车后必须落叉落地、拉手刹、断电熄火。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第20题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(20, 'true_false', '叉车可以用货叉顶推、拖拽其他车辆。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第21题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(21, 'true_false', '作业现场视线受阻应鸣笛、低速慢行、有人指挥。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第22题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(22, 'true_false', '叉车链条松动、变形可继续作业不用检修。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉出现变形弯曲必须直接报废更换，不可敲打校正或加热掰直。校正会破坏货叉的热处理结构，承重能力大幅下降，使用中可能突然断裂造成事故。（题库第23题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(23, 'true_false', '液压系统漏油不影响正常作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第24题）', NULL, NULL, NULL, 2, 2, 'published', NULL, 'admin', now(), now()),
(24, 'true_false', '叉车行驶时人员可以趴在货叉、站在托盘上。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第27题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(25, 'true_false', '交叉路口叉车应减速鸣笛，礼让行人与车辆。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第28题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(26, 'true_false', '雨雪天地面湿滑应减速慢行，禁止急刹急转弯。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第29题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(27, 'true_false', '叉车起升重物时，严禁人员在货叉下方站立穿行。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第30题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(28, 'true_false', '叉车可以在坡道上转弯。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第31题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(29, 'true_false', '货物摆放重心偏后不影响叉车行驶安全。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第32题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(30, 'true_false', '叉车行驶时门架应尽量前倾。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第33题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(31, 'true_false', '叉车装卸货物时应拉手刹、挂空挡。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第34题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(32, 'true_false', '可以用叉车牵引故障机动车上路行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第35题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(33, 'true_false', '电瓶叉车充电时应远离明火、保持通风。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第36题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(34, 'true_false', '叉车喇叭损坏可以继续作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第37题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(35, 'true_false', '叉车轮胎花纹磨损严重仍可正常行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】轮胎检查标准：气压符合规定值（过高致胎面中部磨损易爆胎，过低致胎肩磨损转向沉重）、花纹深度不足1.6mm需更换、鼓包（帘布层断裂）立即更换、开裂（老化或外伤）及时更换。（题库第38题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(36, 'true_false', '酒后严禁驾驶叉车作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第39题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(37, 'true_false', '疲劳状态下可以勉强操作叉车。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第40题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(38, 'true_false', '叉车作业时可以接打电话。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车作业时严禁接打电话、与他人闲聊、注意力分散。作业中应集中精力，保持对周围环境的持续观察。边打电话边操作是严重违章行为。（题库第41题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(39, 'true_false', '发现地面有油污应及时清理，防止打滑。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第42题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(40, 'true_false', '两台叉车可以同距离并排高速行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第43题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(41, 'true_false', '叉车倒车时必须观察后方、鸣笛示意。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第44题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(42, 'true_false', '叉车货叉开裂、变形可以继续使用。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第45题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(43, 'true_false', '更换叉车液压油无需停机泄压。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第46题）', NULL, NULL, NULL, 2, 2, 'published', NULL, 'admin', now(), now()),
(44, 'true_false', '叉车属于特种设备，由市场监督管理部门监管。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第47题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(45, 'true_false', '叉车可以在人行道上行驶作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第49题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(46, 'true_false', '作业结束后不必清理车辆，直接熄火即可。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第50题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(47, 'true_false', '叉车起步前应观察四周、鸣笛示意。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第51题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(48, 'true_false', '载物过高遮挡视线应倒车低速行驶。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第52题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(49, 'true_false', '可以利用叉车惯性滑行取货卸货。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁利用叉车惯性滑行取货卸货、用货叉顶推或拖拽其他车辆/设备、用叉车冲撞货物进行对位、用货叉撬动深埋固定重物。这些行为会损坏货叉、门架、液压系统，且极易引发事故。（题库第53题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(50, 'true_false', '叉车行驶遇行人应停车礼让。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第54题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(51, 'true_false', '叉车门架升降时有异响可继续作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】门架不能升降故障大概率是液压系统故障。门架导轨干涩卡顿应加注润滑黄油，不可强行操作或加水润滑。门架升降异响应立即停机检查。（题库第55题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(52, 'true_false', '低温天气启动叉车应怠速预热。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】内燃叉车正常水温80-90°C。冷车启动应怠速预热3-5分钟，使机油充分润滑各部件。熄火前应怠速降温片刻，避免高温骤冷损坏发动机。怠速时间不宜过长，否则积碳增多、缸壁润滑不良。（题库第56题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(53, 'true_false', '叉车可以叉运埋在地下的重物。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第57题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(54, 'true_false', '托盘破损变形仍可正常叉运货物。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉出现变形弯曲必须直接报废更换，不可敲打校正或加热掰直。校正会破坏货叉的热处理结构，承重能力大幅下降，使用中可能突然断裂造成事故。（题库第58题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(55, 'true_false', '叉车限速标志应严格遵守。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第59题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(56, 'true_false', '可以在叉车货叉之间站人作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第60题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(57, 'true_false', '内燃叉车机油不足仍可短时作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】制动液不足会造成制动失灵。制动液位应在MIN-MAX之间，低于MIN线说明系统有泄漏或制动片磨损严重，需立即检修。（题库第61题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(58, 'true_false', '叉车制动踏板自由行程过大应及时调整。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第62题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(59, 'true_false', '雨天行驶应避免急刹车，防止侧滑。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第63题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(60, 'true_false', '叉车可以在斜坡上横向行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第64题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(61, 'true_false', '货物捆绑不牢固禁止起升转运。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货物捆绑不牢固禁止起升转运。捆绑松散存在的风险：运输途中散落、货物倾倒、砸伤人员、损坏设备。必须确认捆绑牢固后方可叉运。（题库第65题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(62, 'true_false', '叉车维修时应切断电源、拉手刹、垫好车轮。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车维修作业安全要求：切断电源熄火、垫牢车轮防滑、货叉支撑固定、禁止明火检修。严禁带电拆卸元件、带压拆卸油管、私自改装限速、擅自拆除防护装置。（题库第66题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(63, 'true_false', '无证上岗发生事故由个人自行承担责任。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第67题）', NULL, NULL, NULL, 2, 6, 'published', NULL, 'admin', now(), now()),
(64, 'true_false', '企业无需对叉车司机做定期安全培训。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第68题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(65, 'true_false', '叉车灯光不全不影响白天作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第69题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(66, 'true_false', '起升货物离地后应稍作停顿，检查平稳性。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第70题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(67, 'true_false', '可以用货叉挑翻稳固的货物。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第71题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(68, 'true_false', '叉车行驶时货叉离地面越高越不安全。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第72题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(69, 'true_false', '仓库通道狭窄应低速慢行、谨慎会车。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第73题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(70, 'true_false', '叉车可以搭载随行人员同行。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第74题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(71, 'true_false', '液压油温过高应停机降温，禁止继续作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第75题）', NULL, NULL, NULL, 2, 2, 'published', NULL, 'admin', now(), now()),
(72, 'true_false', '叉车转向沉重应立即停机检查。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第76题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(73, 'true_false', '超载一点点货物可以勉强叉运。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第77题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(74, 'true_false', '叉车作业完毕应停放在指定停车区域。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第78题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(75, 'true_false', '可以随意改动叉车限速、液压安全阀参数。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第79题）', NULL, NULL, NULL, 2, 2, 'published', NULL, 'admin', now(), now()),
(76, 'true_false', '叉车驾驶员应穿戴劳保用品，严禁穿拖鞋作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车驾驶员必须穿戴劳保用品：劳保鞋（防砸防滑）、工作服、防护手套。严禁穿拖鞋、短裤、赤膊作业。严禁使用化纤防滑手套（易产生静电火花）。（题库第80题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(77, 'true_false', '电瓶叉车电池缺水可以继续充电使用。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第81题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(78, 'true_false', '叉车转弯半径越小越容易侧翻。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第82题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(79, 'true_false', '夜间作业照明不良应停止作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第83题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(80, 'true_false', '可以在叉车下方检修、停留。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第84题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(81, 'true_false', '叉车会车时应靠右减速，保持安全间距。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第85题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(82, 'true_false', '货物重心偏向一侧容易造成叉车侧翻。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第86题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(83, 'true_false', '叉车怠速时可以长时间原地等待作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第87题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(84, 'true_false', '遇到突发障碍物可以急打方向盘避让。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第88题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(85, 'true_false', '操作人员身体不适不得驾驶叉车。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第90题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(86, 'true_false', '可以用叉车冲撞货物进行对位。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁利用叉车惯性滑行取货卸货、用货叉顶推或拖拽其他车辆/设备、用叉车冲撞货物进行对位、用货叉撬动深埋固定重物。这些行为会损坏货叉、门架、液压系统，且极易引发事故。（题库第91题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(87, 'true_false', '叉车链条松紧度应定期检查调整。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第92题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(88, 'true_false', '粉尘环境作业应做好防尘防护。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第93题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(89, 'true_false', '叉车倒车速度可以比前进速度更快。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车倒车速度不得超过3km/h。倒车时驾驶员视野受限，后方盲区大，低速行驶是保证安全的关键。（题库第94题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(90, 'true_false', '防爆叉车可以在易燃易爆危险区域作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第95题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(91, 'true_false', '普通电瓶叉车可代替防爆叉车使用。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第96题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(92, 'true_false', '坡道上行可以中途换挡变速。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第97题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(93, 'true_false', '叉车停车可随意堵在通道路口。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第98题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(94, 'true_false', '作业前检查发现隐患必须排除后方可作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第99题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(95, 'true_false', '熟练司机可以不按操作规程操作叉车。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第100题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(96, 'true_false', '叉车行驶中严禁换挡滑行。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第101题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(97, 'true_false', '货叉高低不平禁止叉运货物。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第102题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(98, 'true_false', '可以在松软地面高速行驶叉车。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第103题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(99, 'true_false', '叉车起升油缸漏油应立即停用检修。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】起升油缸漏油最直接影响是起升无力或货叉自动下沉。漏油会导致液压系统压力不足，应立即停用检修，更换油缸密封件。（题库第104题）', NULL, NULL, NULL, 2, 2, 'published', NULL, 'admin', now(), now()),
(100, 'true_false', '交叉路口叉车优先先行，不用礼让行人。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第105题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(101, 'true_false', '叉车作业时应集中精力，不与他人闲聊。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车作业时严禁接打电话、与他人闲聊、注意力分散。作业中应集中精力，保持对周围环境的持续观察。边打电话边操作是严重违章行为。（题库第106题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(102, 'true_false', '可以用叉车拖拽卡在地面的车辆。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁利用叉车惯性滑行取货卸货、用货叉顶推或拖拽其他车辆/设备、用叉车冲撞货物进行对位、用货叉撬动深埋固定重物。这些行为会损坏货叉、门架、液压系统，且极易引发事故。（题库第107题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(103, 'true_false', '叉车轮胎气压过高过低都存在安全隐患。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】轮胎检查标准：气压符合规定值（过高致胎面中部磨损易爆胎，过低致胎肩磨损转向沉重）、花纹深度不足1.6mm需更换、鼓包（帘布层断裂）立即更换、开裂（老化或外伤）及时更换。（题库第108题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(104, 'true_false', '冬季启动叉车可明火烘烤油箱油路。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】电瓶叉车充电要求：环境通风远离明火、充电温度5-30°C、充电时打开电池盖排气。充电时严禁靠近明火（氢气易爆）、严禁插拔带电接头（产生电弧引燃氢气）。（题库第109题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(105, 'true_false', '叉车门架前倾利于载货行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第110题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(106, 'true_false', '叉车载货越高，稳定性越差。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货物重心越高，叉车稳定性越差。载货越高，重心上移，转弯时离心力矩增大，侧翻风险急剧上升。因此载货行驶应保持货叉离地10-20cm、门架后倾。（题库第111题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(107, 'true_false', '发现他人违规操作叉车应及时制止。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第112题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(108, 'true_false', '叉车可以临时充当登高平台使用。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第113题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(109, 'true_false', '雨天仓库地面湿滑应加大安全距离。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第114题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(110, 'true_false', '叉车行驶中货物晃动应立即减速平稳行驶。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第117题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(111, 'true_false', '可以边打电话边开叉车作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车作业时严禁接打电话、与他人闲聊、注意力分散。作业中应集中精力，保持对周围环境的持续观察。边打电话边操作是严重违章行为。（题库第118题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(112, 'true_false', '两台叉车同时装卸同一堆货物要保持安全距离。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第119题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(113, 'true_false', '叉车刹车跑偏可以继续短途作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】制动偏刹表现为制动时车辆跑偏，原因是左右制动片磨损不均或制动分泵卡滞。排除方法：调整或更换制动片，确保左右制动力均衡。（题库第120题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(114, 'true_false', '货叉厚度磨损超过规定值应更换。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第121题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(115, 'true_false', '叉车可以在人员密集区域高速穿行。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第122题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(116, 'true_false', '作业结束应关闭总电源、拉紧手刹。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第123题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(117, 'true_false', '内燃叉车尾气可直接排放在密闭室内。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第124题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(118, 'true_false', '叉车驾驶员要熟悉车辆性能和操作规程。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第125题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(119, 'true_false', '可以超载10%以内正常作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第126题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(120, 'true_false', '叉车转弯不必减速，靠技术即可。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第127题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(121, 'true_false', '视线良好时载货可以适当抬高货叉行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第128题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(122, 'true_false', '叉车维修保养应由专业人员操作。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第129题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(123, 'true_false', '无证人员可学习操作，不上路即可。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第130题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(124, 'true_false', '叉车行驶遇拐弯应鸣笛减速。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第131题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(125, 'true_false', '货物超出货叉两端过多仍可转运。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货物超出货叉两端过多禁止转运。超长超宽超限货物需使用特殊装置（如加长货叉、侧移器）或分拆搬运。严禁私自加长货叉搬运货物（重心偏移极易翻车）。（题库第132题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(126, 'true_false', '电瓶叉车充电场地严禁烟火。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第133题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(127, 'true_false', '叉车方向盘间隙过大应及时检修。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第134题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(128, 'true_false', '可以用脚踢货叉调整位置。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第135题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(129, 'true_false', '叉车在泥泞路面应低速匀速行驶。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第136题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(130, 'true_false', '坡道下坡可以空挡滑行省油。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第137题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(131, 'true_false', '叉车作业现场要有安全警示标识。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第138题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(132, 'true_false', '驾驶员可以擅自拆除叉车安全防护装置。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车安全防护装置（护顶架、安全带、倒车报警器、警示灯等）严禁擅自拆除。安全带防止侧翻时驾驶员被甩出，倒车报警器提醒后方人员避让。（题库第139题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(133, 'true_false', '起升货物时禁止猛踩升降操纵杆。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第140题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(134, 'true_false', '叉车可以运载超长货物斜向行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货物超出货叉两端过多禁止转运。超长超宽超限货物需使用特殊装置（如加长货叉、侧移器）或分拆搬运。严禁私自加长货叉搬运货物（重心偏移极易翻车）。（题库第141题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(135, 'true_false', '仓库内严禁叉车高速超车。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第142题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(136, 'true_false', '叉车日常保养可由驾驶员自行完成。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第143题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(137, 'true_false', '发现叉车有异响应立即停机检查。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第144题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(138, 'true_false', '可以站在货物上指挥叉车对位。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第145题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(139, 'true_false', '叉车行驶中严禁急刹车。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第147题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(140, 'true_false', '偏载货物容易导致叉车侧翻。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第148题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(141, 'true_false', '夜间叉车作业可不开灯光凭经验行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第149题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(142, 'true_false', '叉车停车后货叉无需落地悬空即可。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第150题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(143, 'true_false', '易燃易爆物品必须用专用防爆叉车转运。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第151题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(144, 'true_false', '叉车可以在坡道上掉头。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第152题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(145, 'true_false', '驾驶员离岗必须熄火、拔钥匙、拉手刹。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第153题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(146, 'true_false', '液压系统缺油会导致起升无力。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第154题）', NULL, NULL, NULL, 2, 2, 'published', NULL, 'admin', now(), now()),
(147, 'true_false', '可以用货叉撞击门框、货架对位。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第155题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(148, 'true_false', '叉车会车时窄通道一方应停车避让。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第156题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(149, 'true_false', '长期不用的叉车应断开电瓶负极。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第157题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(150, 'true_false', '叉车轮胎损坏仍可继续重载作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第158题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(151, 'true_false', '作业前不必检查安全带、防护装置。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车安全防护装置（护顶架、安全带、倒车报警器、警示灯等）严禁擅自拆除。安全带防止侧翻时驾驶员被甩出，倒车报警器提醒后方人员避让。（题库第159题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(152, 'true_false', '叉车平稳起步可以减少货物晃动。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第160题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(153, 'true_false', '可以多人同时在一辆叉车上作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第161题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(154, 'true_false', '叉车涉水行驶后应检查制动性能。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车涉水行驶后应检查制动性能。制动片进水会打滑，导致制动距离变长。雨天刹车变迟钝就是因为刹车片进水打滑。涉水后应低速轻踩制动，利用摩擦热蒸干水分。（题库第162题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(155, 'true_false', '超载作业会缩短叉车使用寿命。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第163题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(156, 'true_false', '叉车操作手柄不用时可随意扳动。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车操作手柄回中位时，液压动作停止，同时多路换向阀中位使液压泵卸荷，减少能量消耗和系统发热。不使用操作手柄时不可随意扳动。（题库第164题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(157, 'true_false', '仓库通道应有明确的叉车行驶路线。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第165题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(158, 'true_false', '叉车制动液不足会造成制动失灵。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第166题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(159, 'true_false', '熟练司机可以不鸣笛直接起步转弯。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第167题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(160, 'true_false', '货物码放过高倒塌风险大，应限制高度。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】液压油正常工作温度30-55°C，最高不超过80°C。油温过高会导致密封件老化、油液变质、系统内泄增大。常见原因：油量不足、散热器堵塞、长时间超负荷作业。（题库第168题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(161, 'true_false', '叉车可以代替吊车进行起重作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第169题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(162, 'true_false', '雨雪天后地面结冰应停止叉车室外作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第170题）', NULL, NULL, NULL, 2, 6, 'published', NULL, 'admin', now(), now()),
(163, 'true_false', '叉车行驶中发现故障应开到空旷处停机。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第173题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(164, 'true_false', '货叉变形弯曲可以校正后继续使用。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第174题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(165, 'true_false', '叉车起步后可立即加速高速行驶。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第175题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(166, 'true_false', '狭窄通道倒车行驶更安全。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第176题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(167, 'true_false', '叉车可以在电梯内随意转弯掉头。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车在电梯内作业应低速驶入、禁止车内转弯掉头。电梯空间狭小，转弯会碰撞轿壁，且电梯承重分布不均可能卡阻。（题库第177题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(168, 'true_false', '定期保养能减少叉车故障发生率。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第178题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(169, 'true_false', '酒后操作叉车只要不出事就行。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第179题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(170, 'true_false', '叉车作业时地面人员不得靠近货叉下方。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第180题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(171, 'true_false', '可以利用叉车带动其他机械作业。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第181题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(172, 'true_false', '叉车门架导轨缺润滑应及时加注黄油。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】液压油加注四要求：使用原厂型号（L-HM46抗磨液压油）、油位在标准区间、过滤无杂质、停机冷却后加注。严禁混用不同型号液压油。（题库第182题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(173, 'true_false', '空载叉车转弯也需要减速慢行。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第183题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(174, 'true_false', '企业必须建立叉车安全作业管理制度。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第184题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(175, 'true_false', '叉车灯光、喇叭失效禁止出车作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第185题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(176, 'true_false', '货物重心居中是安全叉运的前提。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第186题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(177, 'true_false', '叉车下坡正向行驶比倒车更安全。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第187题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(178, 'true_false', '可以在叉车货叉上捆绑货物加长搬运。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第188题）', NULL, NULL, NULL, 2, 5, 'published', NULL, 'admin', now(), now()),
(179, 'true_false', '叉车停机后应关闭总电源开关。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第189题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(180, 'true_false', '视力不佳、色盲人员不得从事叉车作业。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第190题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(181, 'true_false', '叉车行驶遇路口应一慢二看三通过。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第191题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(182, 'true_false', '电瓶叉车长期闲置应定期补电保养。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第193题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(183, 'true_false', '叉车链条拉长应及时调整或更换。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第194题）', NULL, NULL, NULL, 2, 4, 'published', NULL, 'admin', now(), now()),
(184, 'true_false', '可以在叉车行驶中上下车辆。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第195题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(185, 'true_false', '叉车重载行驶转弯极易发生侧翻。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第196题）', NULL, NULL, NULL, 2, 3, 'published', NULL, 'admin', now(), now()),
(186, 'true_false', '作业完毕应清洁叉车外观及底盘杂物。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第197题）', NULL, NULL, NULL, 2, 1, 'published', NULL, 'admin', now(), now()),
(187, 'true_false', '叉车安全阀可私自调高增加起升重量。', '{"对":"正确","错":"错误"}'::jsonb, '错', '【答案：错误】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第198题）', NULL, NULL, NULL, 2, 2, 'published', NULL, 'admin', now(), now()),
(188, 'true_false', '遵守操作规程能有效避免安全事故。', '{"对":"正确","错":"错误"}'::jsonb, '对', '【答案：正确】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第199题）', NULL, NULL, NULL, 2, 6, 'published', NULL, 'admin', now(), now()),
(189, 'single_choice', '厂内仓库叉车常规限速多少？', '{"A":"3km/h","B":"5km/h","C":"10km/h","D":"15km/h"}'::jsonb, 'B', '【答案：B】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第2题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(190, 'single_choice', '叉车载货行驶货叉离地标准高度？', '{"A":"5-10cm","B":"10-20cm","C":"30cm","D":"40cm"}'::jsonb, 'B', '【答案：B】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第3题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(191, 'single_choice', '叉车坡道正确行驶方式？', '{"A":"上坡倒走下坡正走","B":"上坡正走下坡倒走","C":"都可快速冲坡","D":"空挡溜车"}'::jsonb, 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第4题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(192, 'single_choice', '叉车严格禁止的行为是？', '{"A":"减速鸣笛","B":"载人行驶","C":"平稳叉货","D":"停车落叉"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第5题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(193, 'single_choice', '易燃易爆库房必须使用哪种叉车？', '{"A":"柴油","B":"汽油","C":"防爆电瓶","D":"普通电瓶"}'::jsonb, 'C', '【答案：C】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第6题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(194, 'single_choice', '叉车制动失灵应立即？', '{"A":"开回库房","B":"靠边停车熄火报修","C":"强行打方向减速","D":"继续作业"}'::jsonb, 'B', '【答案：B】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第7题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(195, 'single_choice', '叉车日常出车检查重点是？', '{"A":"只看油量","B":"制动转向液压轮胎灯光","C":"只看外观","D":"不用检查"}'::jsonb, 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第8题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(196, 'single_choice', '叉车转弯正确操作？', '{"A":"高速急转","B":"提前减速鸣笛慢行","C":"边转边加速","D":"不用观察"}'::jsonb, 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第10题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(197, 'single_choice', '叉车作业人员最低学历要求？', '{"A":"小学","B":"初中","C":"高中","D":"大专"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第11题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(198, 'single_choice', '叉车满载行驶时稳定性？', '{"A":"变好","B":"变差","C":"不变","D":"无关"}'::jsonb, 'B', '【答案：B】货物重心越高，叉车稳定性越差。载货越高，重心上移，转弯时离心力矩增大，侧翻风险急剧上升。因此载货行驶应保持货叉离地10-20cm、门架后倾。（题库第12题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(199, 'single_choice', '货叉起升后下方应？', '{"A":"可站人","B":"严禁站人穿行","C":"临时停留","D":"随意走动"}'::jsonb, 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第13题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(200, 'single_choice', '叉车停车正确做法？', '{"A":"货叉悬空","B":"货叉落地拉手刹熄火","C":"空挡不拉手刹","D":"随意停放"}'::jsonb, 'B', '【答案：B】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第14题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(201, 'single_choice', '电瓶叉车充电应在什么环境？', '{"A":"密闭房间","B":"通风远离明火","C":"靠近热源","D":"楼道内"}'::jsonb, 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第15题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(202, 'single_choice', '叉车侧翻最主要原因是？', '{"A":"车速慢","B":"超载偏载高速转弯","C":"路面平整","D":"低速行驶"}'::jsonb, 'B', '【答案：B】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第16题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(203, 'single_choice', '叉车作业视线受阻应？', '{"A":"快速通过","B":"鸣笛低速专人指挥","C":"凭经验猛开","D":"加速绕行"}'::jsonb, 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第17题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(204, 'single_choice', '内燃叉车正常工作水温？', '{"A":"40-50℃","B":"80-90℃","C":"100℃以上","D":"30℃以下"}'::jsonb, 'B', '【答案：B】内燃叉车正常水温80-90°C。冷车启动应怠速预热3-5分钟，使机油充分润滑各部件。熄火前应怠速降温片刻，避免高温骤冷损坏发动机。怠速时间不宜过长，否则积碳增多、缸壁润滑不良。（题库第19题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(205, 'single_choice', '叉车行驶门架应保持？', '{"A":"前倾","B":"后倾","C":"垂直","D":"随意"}'::jsonb, 'B', '【答案：B】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第20题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(206, 'single_choice', '严禁叉车在坡道上？', '{"A":"低速行驶","B":"转弯掉头","C":"倒车慢行","D":"平稳通过"}'::jsonb, 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第21题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(207, 'single_choice', '叉车货物超高遮挡视线应？', '{"A":"抬高行驶","B":"倒车低速行驶","C":"加速冲过","D":"靠边停车"}'::jsonb, 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第22题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(208, 'single_choice', '叉车起步第一步应？', '{"A":"直接加油","B":"鸣笛观察四周","C":"挂挡就走","D":"打方向"}'::jsonb, 'B', '【答案：B】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第24题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(209, 'single_choice', '叉车轮胎花纹磨损严重应？', '{"A":"继续用","B":"更换","C":"充气就行","D":"放气"}'::jsonb, 'B', '【答案：B】轮胎检查标准：气压符合规定值（过高致胎面中部磨损易爆胎，过低致胎肩磨损转向沉重）、花纹深度不足1.6mm需更换、鼓包（帘布层断裂）立即更换、开裂（老化或外伤）及时更换。（题库第25题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(210, 'single_choice', '雨天叉车行驶应？', '{"A":"急刹减速","B":"减速慢行避免急转","C":"高速行驶","D":"靠边停车不开"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第26题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(211, 'single_choice', '叉车最大允许爬坡角度一般为？', '{"A":"5%","B":"10%-15%","C":"30%","D":"45%"}'::jsonb, 'B', '【答案：B】叉车满载爬坡应低速匀速上坡，严禁中途换挡变速。换挡瞬间动力中断可能导致溜坡。最大允许爬坡角度一般为10%-15%。（题库第27题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(212, 'single_choice', '可以用叉车运载？', '{"A":"人员","B":"合规货物托盘","C":"超长悬空物料","D":"易燃易爆无防护品"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第28题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(213, 'single_choice', '叉车操作中发现液压无力应？', '{"A":"坚持干完","B":"立即停机检查","C":"猛踩操纵杆","D":"继续重载"}'::jsonb, 'B', '【答案：B】货叉不起升故障排查顺序：①液压油不足（补充液压油）②液压泵故障（泵磨损、内泄）③溢流阀调定压力过低（调整或更换）④换向阀卡滞（清洗或更换）⑤起升油缸内泄（更换密封件）。（题库第29题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(214, 'single_choice', '叉车驾驶员离岗时应？', '{"A":"熄火不拔钥匙","B":"熄火拉手刹拔钥匙","C":"不熄火留人看管","D":"只拉手刹"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第30题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(215, 'single_choice', '普通叉车严禁进入什么区域？', '{"A":"普通仓库","B":"易燃易爆危险区","C":"厂房通道","D":"露天货场"}'::jsonb, 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第31题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(216, 'single_choice', '叉车会车狭窄通道应？', '{"A":"互不相让","B":"一方停车礼让","C":"加速抢行","D":"占道行驶"}'::jsonb, 'B', '【答案：B】会车避让原则：靠右减速、空载让重载、小车让大车、支线让干线、下坡车让上坡车。窄道一方停车礼让，禁止互不相让抢行。两台叉车同向行驶安全间距不得小于5米。（题库第32题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(217, 'single_choice', '货叉磨损厚度超过多少需更换？', '{"A":"5%","B":"10%","C":"20%","D":"30%"}'::jsonb, 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第33题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(218, 'single_choice', '叉车夜间作业必须开启？', '{"A":"雾灯","B":"照明灯","C":"双闪","D":"不用开灯"}'::jsonb, 'B', '【答案：B】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第34题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(219, 'single_choice', '叉车作业时货叉间距应？', '{"A":"随意调","B":"与托盘适配居中","C":"越宽越好","D":"越窄越好"}'::jsonb, 'B', '【答案：B】货叉间距应与托盘宽度适配、对称居中，禁止一宽一窄。货叉间距不当会导致货物受力不均、倾斜甚至滑落。调节后必须紧固锁定。（题库第36题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(220, 'single_choice', '内燃叉车怠速预热目的？', '{"A":"省油","B":"保护机件正常润滑","C":"加快作业","D":"排放尾气"}'::jsonb, 'B', '【答案：B】内燃叉车正常水温80-90°C。冷车启动应怠速预热3-5分钟，使机油充分润滑各部件。熄火前应怠速降温片刻，避免高温骤冷损坏发动机。怠速时间不宜过长，否则积碳增多、缸壁润滑不良。（题库第37题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(221, 'single_choice', '叉车行驶中禁止？', '{"A":"减速","B":"急刹车急转弯","C":"鸣笛","D":"观察路况"}'::jsonb, 'B', '【答案：B】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第38题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(222, 'single_choice', '企业叉车安全第一责任人是？', '{"A":"司机","B":"单位负责人","C":"维修员","D":"保安"}'::jsonb, 'B', '【答案：B】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第39题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(223, 'single_choice', '叉车起升货物应？', '{"A":"猛升猛降","B":"平稳缓慢升降","C":"快速升到最高","D":"快速落下"}'::jsonb, 'B', '【答案：B】起升货物应平稳缓慢升降，禁止猛升猛降、猛踩操纵杆。起升后稍作停顿检查平稳性，确认货物无倾斜、无松动后方可移动。起升速度过快会造成货物倾倒损坏。（题库第40题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(224, 'single_choice', '叉车行驶时，与前车安全距离至少保持？', '{"A":"1米","B":"2米","C":"3米以上","D":"无所谓"}'::jsonb, 'C', '【答案：C】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第41题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(225, 'single_choice', '叉车装卸货物时，车辆应处于什么状态？', '{"A":"行驶中对位","B":"熄火拉手刹挂空挡","C":"怠速滑行","D":"半离合状态"}'::jsonb, 'B', '【答案：B】装卸货物时车辆应处于熄火、拉手刹、挂空挡状态。靠近月台时减速，确认货叉与车厢对正。装卸完毕确认货叉清空后方可驶离。（题库第42题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(226, 'single_choice', '电瓶叉车蓄电池液面低于极板时应加？', '{"A":"自来水","B":"矿泉水","C":"蒸馏水","D":"电解液"}'::jsonb, 'C', '【答案：C】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第43题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(227, 'single_choice', '叉车发生侧翻瞬间驾驶员应？', '{"A":"跳车","B":"紧握方向盘身体向内倾","C":"往外跳","D":"松手下车"}'::jsonb, 'B', '【答案：B】侧翻瞬间正确处置：紧握方向盘、身体向内倾（向倾翻反方向），禁止跳车、禁止松手下车。跳车会被车辆压砸，留在车内受护顶架保护反而更安全。（题库第44题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(228, 'single_choice', '货叉起升、下降速度过快会造成？', '{"A":"省油","B":"货物倾倒损坏","C":"更平稳","D":"无影响"}'::jsonb, 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第45题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(229, 'single_choice', '叉车通过狭窄通道应？', '{"A":"加速快速通过","B":"低速慢行专人指挥","C":"借道占道","D":"随意通行"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第46题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(230, 'single_choice', '叉车载物超高遮挡视线，正确做法？', '{"A":"抬高货叉往前开","B":"倒车低速行驶","C":"加速冲过去","D":"找人前方引路"}'::jsonb, 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第48题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(231, 'single_choice', '禁止叉车在地面哪种路况高速行驶？', '{"A":"平整路面","B":"凹凸、湿滑、松软路面","C":"水泥路面","D":"沥青路面"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第49题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(232, 'single_choice', '叉车日常保养由谁负责？', '{"A":"专职维修","B":"当班驾驶员","C":"管理员","D":"保安"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第50题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(233, 'single_choice', '叉车转弯时，车身哪一侧容易侧翻？', '{"A":"内侧","B":"外侧","C":"前后","D":"中间"}'::jsonb, 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第51题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(234, 'single_choice', '叉车严禁叉运？', '{"A":"标准托盘货物","B":"埋地固定重物","C":"纸箱货物","D":"塑料货物"}'::jsonb, 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第52题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(235, 'single_choice', '叉车液压系统工作压力由什么控制？', '{"A":"油管","B":"安全阀","C":"手柄","D":"油泵"}'::jsonb, 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第53题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(236, 'single_choice', '叉车倒车时观察重点？', '{"A":"前方货物","B":"后方人员及障碍物","C":"车顶","D":"轮胎"}'::jsonb, 'B', '【答案：B】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第54题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(237, 'single_choice', '内燃叉车冒黑烟原因？', '{"A":"燃油充分","B":"燃烧不充分","C":"水温过高","D":"机油过多"}'::jsonb, 'B', '【答案：B】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第55题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(238, 'single_choice', '叉车制动踏板发软制动力不足，原因是？', '{"A":"刹车油充足","B":"油路进气","C":"轮胎过硬","D":"车速太慢"}'::jsonb, 'B', '【答案：B】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第56题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(239, 'single_choice', '货物重心越高，叉车稳定性？', '{"A":"越好","B":"越差","C":"不变","D":"无关联"}'::jsonb, 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第57题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(240, 'single_choice', '叉车空载行驶，货叉离地高度？', '{"A":"越低越好","B":"10-20cm","C":"50cm","D":"随意高度"}'::jsonb, 'B', '【答案：B】载货行驶时货叉应离地10-20cm，门架后倾。过低易刮碰地面减速带，过高影响稳定性且货物可能滑落伤人。此为叉车安全操作的基本规范。（题库第58题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(241, 'single_choice', '叉车坡道停车正确制动方式？', '{"A":"仅手刹","B":"脚刹+手刹+落叉","C":"空挡滑行","D":"直接熄火"}'::jsonb, 'B', '【答案：B】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第59题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(242, 'single_choice', '两台叉车同向行驶，安全间距不得小于？', '{"A":"1米","B":"3米","C":"5米","D":"10米"}'::jsonb, 'C', '【答案：C】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第60题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(243, 'single_choice', '叉车严禁哪种操作？', '{"A":"平稳装卸","B":"惯性滑行对位","C":"低速转弯","D":"鸣笛警示"}'::jsonb, 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第61题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(244, 'single_choice', '电瓶叉车充电温度最佳范围？', '{"A":"0℃以下","B":"5-30℃","C":"40℃以上","D":"高温暴晒"}'::jsonb, 'B', '【答案：B】电瓶叉车充电要求：环境通风远离明火、充电温度5-30°C、充电时打开电池盖排气。充电时严禁靠近明火（氢气易爆）、严禁插拔带电接头（产生电弧引燃氢气）。（题库第62题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(245, 'single_choice', '叉车链条松紧标准？', '{"A":"下垂1-2cm","B":"下垂5-10cm","C":"越紧越好","D":"越松越好"}'::jsonb, 'A', '【答案：A】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第63题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(246, 'single_choice', '叉车安全阀作用？', '{"A":"增加速度","B":"限制液压最大压力","C":"省油","D":"降温"}'::jsonb, 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第64题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(247, 'single_choice', '叉车倒车转弯，内轮差会导致？', '{"A":"内侧盲区","B":"外侧盲区","C":"无盲区","D":"车身变短"}'::jsonb, 'A', '【答案：A】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第65题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(248, 'single_choice', '仓库消防通道叉车？', '{"A":"可临时停放","B":"严禁占用停放","C":"卸货可短暂停留","D":"随意停靠"}'::jsonb, 'B', '【答案：B】叉车严禁停放在消防通道、路口拐角、坡道斜坡、人员密集通道。应停放在指定停车区域。随意停放会阻碍通行、影响应急疏散，引发次生事故。（题库第66题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(249, 'single_choice', '叉车仪表盘红色指示灯代表？', '{"A":"正常运行","B":"故障报警","C":"提示灯光","D":"怠速状态"}'::jsonb, 'B', '【答案：B】叉车仪表盘红色指示灯代表故障报警。常见报警灯：机油压力报警、水温报警、电量不足提示、故障报警。红灯亮起应立即停机检查，不可继续作业。（题库第67题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(250, 'single_choice', '叉车起升货物最大倾斜角度？', '{"A":"5度","B":"10度","C":"20度","D":"30度"}'::jsonb, 'B', '【答案：B】门架前倾角3°-6°（便于叉取卸货），后倾角6°-12°（防止货物滑落）。起升货物最大倾斜角度为10度，超过此角度有侧翻风险。（题库第68题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(251, 'single_choice', '遇到突发暴雨，室外叉车应？', '{"A":"继续作业","B":"就近安全位置停靠","C":"高速开回库房","D":"原地不动不熄火"}'::jsonb, 'B', '【答案：B】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第69题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(252, 'single_choice', '叉车货叉磨损超标标准？', '{"A":"磨损5%","B":"磨损10%","C":"磨损15%","D":"磨损20%"}'::jsonb, 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第70题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(253, 'single_choice', '内燃叉车机油作用不包括？', '{"A":"润滑","B":"冷却","C":"燃烧","D":"密封"}'::jsonb, 'C', '【答案：C】内燃叉车机油作用：润滑机件、冷却降温、密封防锈、缓冲减震。机油不含「燃烧」功能。机油不足会损坏发动机，应立即补充。（题库第71题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(254, 'single_choice', '叉车低速行驶时使用？', '{"A":"高速挡","B":"低速挡","C":"空挡","D":"倒挡"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第72题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(255, 'single_choice', '叉车在厂区主干道限速？', '{"A":"5km/h","B":"10km/h","C":"15km/h","D":"20km/h"}'::jsonb, 'B', '【答案：B】厂区主干道叉车最高行驶速度10km/h，仓库车间内限速5km/h。限速是预防碰撞事故的首要措施，载货时限速还应适当降低。（题库第73题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(256, 'single_choice', '货物偏载放置，叉车容易发生？', '{"A":"提速变快","B":"单侧侧翻","C":"刹车灵敏","D":"转向轻便"}'::jsonb, 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第74题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(257, 'single_choice', '维修叉车液压系统第一步？', '{"A":"加注液压油","B":"停机泄压","C":"直接拆卸","D":"启动车辆"}'::jsonb, 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第75题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(258, 'single_choice', '叉车灯光损坏、喇叭失灵？', '{"A":"白天可作业","B":"禁止出车作业","C":"慢速作业","D":"偏远区域作业"}'::jsonb, 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第76题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(259, 'single_choice', '叉车爬坡重载应？', '{"A":"高速冲坡","B":"低速匀速上坡","C":"中途换挡","D":"中途停车"}'::jsonb, 'B', '【答案：B】叉车满载爬坡应低速匀速上坡，严禁中途换挡变速。换挡瞬间动力中断可能导致溜坡。最大允许爬坡角度一般为10%-15%。（题库第77题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(260, 'single_choice', '电瓶叉车电池鼓包原因？', '{"A":"充电正常","B":"过充过热","C":"电压过低","D":"存放太久"}'::jsonb, 'B', '【答案：B】蓄电池鼓包原因是过充过热导致极板膨胀变形。电池损坏原因：过度充电、亏电长期停放（极板硫化）、高温暴晒、电解液缺失。循环寿命约1500次。（题库第78题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(261, 'single_choice', '叉车方向盘自由转动量过大说明？', '{"A":"转向正常","B":"转向间隙过大需检修","C":"转向更灵活","D":"无需处理"}'::jsonb, 'B', '【答案：B】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第79题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(262, 'single_choice', '叉车作业结束停放，错误做法？', '{"A":"货叉落地","B":"门架前倾","C":"拉手刹断电","D":"停指定区域"}'::jsonb, 'B', '【答案：B】叉车严禁停放在消防通道、路口拐角、坡道斜坡、人员密集通道。应停放在指定停车区域。随意停放会阻碍通行、影响应急疏散，引发次生事故。（题库第80题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(263, 'single_choice', '易燃易爆粉尘车间选用？', '{"A":"柴油叉车","B":"防爆叉车","C":"普通电瓶叉车","D":"汽油叉车"}'::jsonb, 'B', '【答案：B】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第81题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(264, 'single_choice', '叉车叉取托盘，货叉插入深度？', '{"A":"一半即可","B":"插入托盘2/3以上","C":"刚好触碰","D":"随意插入"}'::jsonb, 'B', '【答案：B】货叉叉取托盘时插入深度应达到托盘的2/3以上，确保货物稳定。插入过浅会导致货物倾斜或滑落。同时应调整货叉间距与托盘适配、对称居中。（题库第82题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(265, 'single_choice', '叉车行驶颠簸路面应？', '{"A":"加速通过","B":"低速匀速慢行","C":"猛踩刹车","D":"快速转弯"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第83题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(266, 'single_choice', '特种设备安全管理部门是？', '{"A":"交通局","B":"市场监督管理局","C":"公安局","D":"住建局"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第84题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(267, 'single_choice', '叉车驾驶员上岗必须佩戴？', '{"A":"拖鞋","B":"劳保鞋","C":"背心","D":"首饰"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第85题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(268, 'single_choice', '叉车液压油变黑浑浊应？', '{"A":"继续使用","B":"及时更换","C":"加水稀释","D":"过滤接着用"}'::jsonb, 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第86题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(269, 'single_choice', '空叉车下坡正确方式？', '{"A":"正向下坡","B":"倒车下坡","C":"空挡滑行","D":"加速下坡"}'::jsonb, 'B', '【答案：B】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第87题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(270, 'single_choice', '叉车发生轻微碰撞，应？', '{"A":"无视继续作业","B":"停车检查车况货物","C":"加速离开","D":"私下处理"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第88题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(271, 'single_choice', '冬季低温，电瓶叉车续航会？', '{"A":"变长","B":"变短","C":"不变","D":"无影响"}'::jsonb, 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第89题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(272, 'single_choice', '叉车操作手柄回中位作用？', '{"A":"加速行驶","B":"停止液压动作","C":"熄火停机","D":"解锁方向"}'::jsonb, 'B', '【答案：B】叉车操作手柄回中位时，液压动作停止，同时多路换向阀中位使液压泵卸荷，减少能量消耗和系统发热。不使用操作手柄时不可随意扳动。（题库第90题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(273, 'single_choice', '货架高位取货，叉车必须？', '{"A":"靠近货架、垂直对位","B":"斜向靠近","C":"快速撞击对位","D":"单侧靠近"}'::jsonb, 'A', '【答案：A】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第91题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(274, 'single_choice', '叉车轮胎气压过低会导致？', '{"A":"行驶轻快","B":"轮胎变形磨损严重","C":"刹车变灵","D":"转向变轻"}'::jsonb, 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第92题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(275, 'single_choice', '禁止叉车在哪种地面掉头？', '{"A":"平整空地","B":"坡道、狭窄通道","C":"水泥路面","D":"空旷场地"}'::jsonb, 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第93题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(276, 'single_choice', '发现地面油污，叉车应？', '{"A":"快速碾压通过","B":"减速绕行","C":"原地刹车","D":"加速打滑通过"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第95题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(277, 'single_choice', '叉车最大起升重量标注在？', '{"A":"仪表盘","B":"车身铭牌","C":"轮胎","D":"座椅下方"}'::jsonb, 'B', '【答案：B】叉车额定起重量标注在车身铭牌上，是指标准载荷中心距（500mm）和标准起升高度下的最大允许起升重量。实际作业时需根据载荷曲线图确定不同高度/载荷中心下的安全起重量。（题库第96题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(278, 'single_choice', '两台叉车并排作业安全距离？', '{"A":"1米","B":"2米以上","C":"0.5米","D":"紧贴行驶"}'::jsonb, 'B', '【答案：B】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第97题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(279, 'single_choice', '叉车涉水深度不得超过？', '{"A":"轮胎1/2高度","B":"轮胎全部淹没","C":"车门高度","D":"任意深度"}'::jsonb, 'A', '【答案：A】叉车涉水行驶水深不得超过轮胎1/2高度。涉水后必须检查制动性能（制动片进水会打滑），低速匀速通过，禁止深水通行。（题库第98题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(280, 'single_choice', '叉车熄火前应？', '{"A":"直接断电","B":"怠速降温片刻","C":"猛踩油门","D":"快速熄火"}'::jsonb, 'B', '【答案：B】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第99题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(281, 'single_choice', '货物捆绑松散，叉车应？', '{"A":"小心搬运","B":"禁止起升","C":"低速转运","D":"人工扶住"}'::jsonb, 'B', '【答案：B】货物捆绑不牢固禁止起升转运。捆绑松散存在的风险：运输途中散落、货物倾倒、砸伤人员、损坏设备。必须确认捆绑牢固后方可叉运。（题库第100题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(282, 'single_choice', '叉车倒车鸣笛目的？', '{"A":"提醒前方车辆","B":"提醒后方人员避让","C":"装饰作用","D":"警示管理人员"}'::jsonb, 'B', '【答案：B】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第101题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(283, 'single_choice', '内燃叉车排气颜色为蓝色，说明？', '{"A":"燃烧正常","B":"烧机油","C":"燃油过多","D":"水温过高"}'::jsonb, 'B', '【答案：B】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第102题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(284, 'single_choice', '叉车制动失灵首选处置？', '{"A":"跳车逃生","B":"松油门、缓慢摩擦障碍物减速","C":"急打方向","D":"加速避让"}'::jsonb, 'B', '【答案：B】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第103题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(285, 'single_choice', '厂区十字路口叉车行驶速度？', '{"A":"5km/h以下","B":"10km/h","C":"15km/h","D":"20km/h"}'::jsonb, 'A', '【答案：A】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第104题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(286, 'single_choice', '货叉弯曲变形处理方式？', '{"A":"敲打校正","B":"直接更换","C":"凑合使用","D":"加热掰直"}'::jsonb, 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第105题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(287, 'single_choice', '电瓶叉车严禁充电时？', '{"A":"开盖通风","B":"插拔充电器","C":"靠近明火","D":"静置充电"}'::jsonb, 'C', '【答案：C】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第106题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(288, 'single_choice', '叉车长期停放，货叉应？', '{"A":"悬空离地","B":"完全落地","C":"抬高最高","D":"倾斜放置"}'::jsonb, 'B', '【答案：B】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第107题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(289, 'single_choice', '叉车重载行驶，重心向哪偏移？', '{"A":"向前","B":"向后","C":"向左","D":"向右"}'::jsonb, 'A', '【答案：A】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第108题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(290, 'single_choice', '狭窄通道会车，原则？', '{"A":"大车让小车","B":"空载让重载","C":"快速抢行","D":"随意避让"}'::jsonb, 'B', '【答案：B】会车避让原则：靠右减速、空载让重载、小车让大车、支线让干线、下坡车让上坡车。窄道一方停车礼让，禁止互不相让抢行。两台叉车同向行驶安全间距不得小于5米。（题库第109题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(291, 'single_choice', '叉车液压油温过高故障原因？', '{"A":"油量充足","B":"油路堵塞散热差","C":"车速过慢","D":"空载作业"}'::jsonb, 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第110题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(292, 'single_choice', '驾驶员身体不适、头晕乏力应？', '{"A":"坚持干完","B":"停止作业报备","C":"慢速作业","D":"简单休息继续开"}'::jsonb, 'B', '【答案：B】酒后、疲劳、生病、身体不适、头晕乏力时严禁驾驶叉车。驾驶员身体不适会影响判断力和反应速度，增加事故风险。发现身体不适应立即停止作业报备。（题库第111题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(293, 'single_choice', '叉车严禁用货叉？', '{"A":"搬运货物","B":"撬动重物","C":"装卸托盘","D":"堆码货物"}'::jsonb, 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第112题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(294, 'single_choice', '雨天叉车刹车变迟钝，因为？', '{"A":"刹车片进水打滑","B":"刹车油过多","C":"轮胎变硬","D":"车速太慢"}'::jsonb, 'A', '【答案：A】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第113题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(295, 'single_choice', '叉车灯光不包含？', '{"A":"照明灯","B":"转向灯","C":"远光灯","D":"作业警示灯"}'::jsonb, 'C', '【答案：C】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第114题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(296, 'single_choice', '叉车堆货时，上层货物应？', '{"A":"外宽内窄","B":"上轻下重","C":"上重下轻","D":"随意堆放"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第116题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(297, 'single_choice', '叉车行驶中，禁止人员？', '{"A":"远离车辆","B":"靠近货叉两侧","C":"站在安全区域","D":"远处观望"}'::jsonb, 'B', '【答案：B】叉车行驶中严禁换挡滑行、急刹车、急转弯。这些操作会导致货物晃动倾倒、车辆失控侧翻。行驶中应保持平稳，遇突发情况先减速后制动。（题库第117题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(298, 'single_choice', '内燃叉车燃油不足警示灯亮，应？', '{"A":"继续作业","B":"及时加油","C":"高速行驶省油","D":"颠簸路面行驶"}'::jsonb, 'B', '【答案：B】制动液不足会造成制动失灵。制动液位应在MIN-MAX之间，低于MIN线说明系统有泄漏或制动片磨损严重，需立即检修。（题库第118题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(299, 'single_choice', '叉车门架不能升降故障大概率是？', '{"A":"轮胎故障","B":"液压系统故障","C":"灯光故障","D":"转向故障"}'::jsonb, 'B', '【答案：B】门架不能升降故障大概率是液压系统故障。门架导轨干涩卡顿应加注润滑黄油，不可强行操作或加水润滑。门架升降异响应立即停机检查。（题库第119题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(300, 'single_choice', '叉车空载转弯和重载转弯，哪个更容易侧翻？', '{"A":"空载","B":"重载","C":"一样","D":"平整路面无区别"}'::jsonb, 'B', '【答案：B】转弯作业规程：提前减速、鸣笛、慢行。转弯半径越小越易侧翻，重载转弯比空载转弯更危险。空载叉车转弯同样需要减速慢行。转弯时注意内轮差导致的内侧盲区。（题库第120题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(301, 'single_choice', '仓库湿滑地面，叉车制动距离？', '{"A":"变短","B":"变长","C":"不变","D":"立即刹停"}'::jsonb, 'B', '【答案：B】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第121题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(302, 'single_choice', '叉车驾驶员离岗多久必须熄火拔钥匙？', '{"A":"离开视线范围","B":"5分钟以上","C":"10分钟以上","D":"半小时以上"}'::jsonb, 'A', '【答案：A】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第122题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(303, 'single_choice', '货叉叉取货物，货物重心应在？', '{"A":"货叉前端","B":"货叉中间居中","C":"货叉尾部","D":"偏向一侧"}'::jsonb, 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第123题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(304, 'single_choice', '叉车禁止超高，一般限高？', '{"A":"3米","B":"4.5米","C":"5米","D":"6米"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第124题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(305, 'single_choice', '叉车倒车速度不得超过？', '{"A":"3km/h","B":"5km/h","C":"8km/h","D":"10km/h"}'::jsonb, 'A', '【答案：A】叉车倒车速度不得超过3km/h。倒车时驾驶员视野受限，后方盲区大，低速行驶是保证安全的关键。（题库第125题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(306, 'single_choice', '叉车链条干涩异响，应加注？', '{"A":"液压油","B":"黄油润滑油","C":"自来水","D":"机油"}'::jsonb, 'B', '【答案：B】液压油加注四要求：使用原厂型号（L-HM46抗磨液压油）、油位在标准区间、过滤无杂质、停机冷却后加注。严禁混用不同型号液压油。（题库第126题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(307, 'single_choice', '叉车维修警示标志颜色？', '{"A":"红白色","B":"黄黑色","C":"蓝白色","D":"绿白色"}'::jsonb, 'B', '【答案：B】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第127题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(308, 'single_choice', '普通叉车不能行驶在？', '{"A":"硬化路面","B":"泥泞松软深坑路面","C":"水泥地","D":"沥青地"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第128题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(309, 'single_choice', '叉车起步抖动原因？', '{"A":"平稳起步","B":"离合结合过快","C":"车速太慢","D":"轮胎过硬"}'::jsonb, 'B', '【答案：B】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第129题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(310, 'single_choice', '企业叉车安全培训频次至少？', '{"A":"半年一次","B":"一年一次","C":"两年一次","D":"无需培训"}'::jsonb, 'B', '【答案：B】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第130题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(311, 'single_choice', '叉车紧急情况避险原则？', '{"A":"优先保护货物","B":"优先保护人员安全","C":"优先保护车辆","D":"优先逃离现场"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第131题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(312, 'single_choice', '内燃叉车空气滤清器堵塞会导致？', '{"A":"动力不足","B":"油耗降低","C":"排放减少","D":"水温下降"}'::jsonb, 'A', '【答案：A】空气滤清器堵塞会导致发动机进气不足、燃油燃烧不充分，表现为动力不足、冒黑烟、油耗增加。应定期清洁或更换空气滤芯。（题库第132题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(313, 'single_choice', '叉车停车时，制动踏板应处于？', '{"A":"自由松开状态","B":"一直踩住","C":"半踩状态","D":"随意状态"}'::jsonb, 'A', '【答案：A】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第133题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(314, 'single_choice', '电瓶叉车控制器主要作用？', '{"A":"控制电机转速","B":"储存电量","C":"过滤杂质","D":"散热降温"}'::jsonb, 'A', '【答案：A】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第134题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(315, 'single_choice', '叉车堆码货物，堆叠高度一般不超过？', '{"A":"2层","B":"3-4层","C":"5层","D":"越多越好"}'::jsonb, 'B', '【答案：B】货物码放基本原则：上轻下重（重物在下、轻物在上）、居中码放、整齐稳固、不超过边界。堆叠高度一般不超过3-4层。码放过高会导致倒塌风险大，应限制高度。（题库第135题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(316, 'single_choice', '叉车作业时，严禁使用？', '{"A":"劳保手套","B":"化纤防滑手套","C":"安全帽","D":"劳保鞋"}'::jsonb, 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第136题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(317, 'single_choice', '液压油正常工作温度？', '{"A":"20℃以下","B":"30-55℃","C":"60-80℃","D":"90℃以上"}'::jsonb, 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第137题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(318, 'single_choice', '叉车倒车时，方向盘转动方向？', '{"A":"同向转动","B":"反向转动","C":"不用转动","D":"随意转动"}'::jsonb, 'A', '【答案：A】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第138题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(319, 'single_choice', '叉车安全阀严禁？', '{"A":"检查","B":"私自调节","C":"保养","D":"紧固"}'::jsonb, 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第139题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(320, 'single_choice', '叉车夜间行驶，照明不好应当？', '{"A":"加速通过","B":"停止作业","C":"凭感觉开","D":"关闭灯光"}'::jsonb, 'B', '【答案：B】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第140题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(321, 'single_choice', '叉车作业中发现有人突然横穿通道，应立即？', '{"A":"急打方向避让","B":"紧急制动停车","C":"鸣笛继续行驶","D":"加速通过"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第141题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(322, 'single_choice', '平衡重式叉车的平衡重作用是？', '{"A":"增加行驶速度","B":"平衡前部载荷、防止前倾翻","C":"便于转向","D":"增加爬坡能力"}'::jsonb, 'B', '【答案：B】平衡重式叉车的平衡重作用是平衡前部载荷、防止前倾翻。平衡重位于车体后部，通过杠杆原理抵消货物产生的向前倾覆力矩。（题库第142题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(323, 'single_choice', '叉车起升油缸漏油，最直接影响的是？', '{"A":"转向失灵","B":"起升无力或自动下沉","C":"刹车不灵","D":"灯光不亮"}'::jsonb, 'B', '【答案：B】起升油缸漏油最直接影响是起升无力或货叉自动下沉。漏油会导致液压系统压力不足，应立即停用检修，更换油缸密封件。（题库第143题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(324, 'single_choice', '内燃叉车启动后，怠速时间不宜过长，主要为了？', '{"A":"省油","B":"避免缸壁润滑不良、积碳增多","C":"提高水温","D":"便于观察"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第144题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(325, 'single_choice', '电瓶叉车行驶中突然断电，首先应检查？', '{"A":"液压油位","B":"蓄电池接线是否松动或熔断丝","C":"刹车油","D":"轮胎气压"}'::jsonb, 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第145题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(326, 'single_choice', '叉车在厂房内行驶，视线不良处应？', '{"A":"快速通过","B":"减速、鸣笛、必要时停车确认","C":"凭记忆通过","D":"开远光灯猛冲"}'::jsonb, 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第146题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(327, 'single_choice', '叉车“三包”不包括？', '{"A":"包修","B":"包换","C":"包退","D":"包保养"}'::jsonb, 'D', '【答案：D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第147题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(328, 'single_choice', '叉车门架前倾过多，行驶时容易造成？', '{"A":"转向变轻","B":"前轮载荷过大、制动跑偏、前翻风险增大","C":"更稳定","D":"油耗降低"}'::jsonb, 'B', '【答案：B】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第148题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(329, 'single_choice', '叉车倒车时，如后方视线完全被挡，应？', '{"A":"快速倒车","B":"低速并专人指挥","C":"不开灯倒车","D":"边倒边打电话"}'::jsonb, 'B', '【答案：B】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第149题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(330, 'single_choice', '叉车在交叉路口通行，应遵循？', '{"A":"抢行优先","B":"一慢、二看、三通过，礼让行人与直行车辆","C":"鸣笛就可以冲","D":"谁快谁先走"}'::jsonb, 'B', '【答案：B】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第151题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(331, 'single_choice', '内燃叉车排气管冒蓝烟，说明？', '{"A":"燃油过多","B":"烧机油","C":"水温过高","D":"喷油过早"}'::jsonb, 'B', '【答案：B】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第152题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(332, 'single_choice', '叉车液压系统油温过高，不宜？', '{"A":"停机冷却","B":"继续重载作业","C":"检查散热","D":"检查油量"}'::jsonb, 'B', '【答案：B】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第153题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(333, 'single_choice', '货叉裂纹超过规定应？', '{"A":"焊补后继续用","B":"报废更换","C":"打磨一下继续用","D":"降低载荷用"}'::jsonb, 'B', '【答案：B】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第154题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(334, 'single_choice', '叉车额定起重量是指？', '{"A":"任意高度的最大重量","B":"标准载荷中心距、标准起升高度下的最大允许重量","C":"驾驶员估算重量","D":"装满货的重量"}'::jsonb, 'B', '【答案：B】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第155题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(335, 'single_choice', '叉车夜间作业，必须开启？', '{"A":"警示灯即可","B":"照明灯、必要时示宽灯/警示灯","C":"只开转向灯","D":"不用开灯"}'::jsonb, 'B', '【答案：B】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第156题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(336, 'single_choice', '叉车在坡道上长时间停车，除拉手刹外，还应？', '{"A":"挂空挡即可","B":"落叉触地、必要时垫三角木","C":"只熄火就行","D":"挂高速挡"}'::jsonb, 'B', '【答案：B】停车规范五步：①货叉完全落地 ②拉紧手刹 ③挂空挡 ④断电熄火 ⑤拔下钥匙。驾驶员离车必须拔钥匙。禁止停放在消防通道、路口、坡道、人员密集通道。（题库第157题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(337, 'single_choice', '电瓶叉车蓄电池电解液不足时，只能添加？', '{"A":"自来水","B":"蒸馏水","C":"电解液原液","D":"矿泉水"}'::jsonb, 'B', '【答案：B】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第158题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(338, 'single_choice', '叉车制动踏板自由行程过大，会导致？', '{"A":"刹车太灵","B":"制动滞后、制动力不足","C":"转向沉重","D":"油耗升高"}'::jsonb, 'B', '【答案：B】制动踏板自由行程过大，会导致制动滞后、制动力不足，应及时调整。自由行程是指踩下踏板到实际产生制动效果前的空行程。（题库第159题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(339, 'single_choice', '两台叉车同时在同一通道相向而行，应？', '{"A":"互不相让抢行","B":"空载让重载、窄处一方停车避让","C":"同时靠边挤过去","D":"倒车撞开对方"}'::jsonb, 'B', '【答案：B】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第160题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(340, 'single_choice', '叉车转弯半径越小，说明？', '{"A":"车越长","B":"转向越灵活，但急转弯侧翻风险更高","C":"稳定性更好","D":"车速更快"}'::jsonb, 'B', '【答案：B】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第161题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(341, 'single_choice', '叉车停车熄火后，正确收尾动作是？', '{"A":"不用拔钥匙","B":"货叉悬空","C":"拉紧手刹、关闭总电源、拔下钥匙","D":"随意停放"}'::jsonb, 'C', '【答案：C】坡道行驶严禁熄火、空挡溜车。空挡溜车时发动机制动失效，仅靠脚刹减速，制动距离大幅增加，极易失控。（题库第162题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(342, 'single_choice', '叉车货物重心偏向左侧，行驶中最容易发生？', '{"A":"前轮磨损","B":"左侧侧翻","C":"转向失灵","D":"刹车失效"}'::jsonb, 'B', '【答案：B】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第163题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(343, 'single_choice', '下列哪一项不属于叉车日常保养？', '{"A":"检查轮胎","B":"拆解变速箱","C":"紧固螺丝","D":"清理链条油污"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第164题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(344, 'single_choice', '叉车在电梯内作业正确操作是？', '{"A":"低速驶入、禁止车内转弯","B":"高速冲进去","C":"车内原地掉头","D":"随意停靠"}'::jsonb, 'A', '【答案：A】叉车在电梯内作业应低速驶入、禁止车内转弯掉头。电梯空间狭小，转弯会碰撞轿壁，且电梯承重分布不均可能卡阻。（题库第165题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(345, 'single_choice', '防止叉车货物滑落，最关键的操作是？', '{"A":"提高车速","B":"货物居中、门架后倾、平稳行驶","C":"门架前倾","D":"颠簸行驶"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第166题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(346, 'single_choice', '叉车刹车油长期不更换会导致？', '{"A":"转向变轻","B":"刹车变质、制动失灵","C":"油耗降低","D":"水温下降"}'::jsonb, 'B', '【答案：B】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第167题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(347, 'single_choice', '叉车厂区行驶遇到施工路段应？', '{"A":"加速通过","B":"减速慢行、听从专人指挥","C":"直接绕行碾压障碍物","D":"不用观察"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第168题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(348, 'single_choice', '内燃叉车长期怠速不会造成哪种问题？', '{"A":"积碳增多","B":"油耗升高","C":"发动机过热","D":"动力提升"}'::jsonb, 'D', '【答案：D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第169题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(349, 'single_choice', '叉车防护顶棚主要作用是？', '{"A":"遮挡阳光","B":"防止高空坠物砸伤驾驶员","C":"防雨防水","D":"装饰美观"}'::jsonb, 'B', '【答案：B】叉车护顶架（防护顶棚）主要作用是防止高空坠物砸伤驾驶员，不是遮挡阳光或防雨。在高位堆垛、货架作业时尤其重要。（题库第170题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(350, 'single_choice', '叉车操作人员上岗前必备体检项目不包含？', '{"A":"视力","B":"辨色力","C":"身高","D":"肢体健康"}'::jsonb, 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第171题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(351, 'single_choice', '叉车液压油过少最直接故障是？', '{"A":"灯光不亮","B":"起升缓慢、举升无力","C":"轮胎漏气","D":"喇叭不响"}'::jsonb, 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第172题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(352, 'single_choice', '湿滑路面叉车制动距离相比正常路面？', '{"A":"缩短","B":"变长","C":"不变","D":"瞬间刹停"}'::jsonb, 'B', '【答案：B】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第173题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(353, 'single_choice', '叉车严禁在货叉上进行哪项操作？', '{"A":"搬运托盘","B":"载人登高作业","C":"堆放纸箱","D":"转运塑料筐"}'::jsonb, 'B', '【答案：B】严禁货叉上载人、站在托盘上作业、用叉车充当登高平台、搭载随行人员。货叉不是载人平台，无安全防护，人员极易坠落或被货物压伤。（题库第174题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(354, 'single_choice', '叉车维修拆卸液压油管第一步必须？', '{"A":"直接拆卸","B":"停机泄压、关闭电源","C":"启动车辆加压","D":"随意拆卸"}'::jsonb, 'B', '【答案：B】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第176题）', NULL, NULL, NULL, 3, 2, 'published', NULL, 'admin', now(), now()),
(355, 'single_choice', '空旷厂区主干道，叉车最高行驶速度为？', '{"A":"5km/h","B":"10km/h","C":"15km/h","D":"20km/h"}'::jsonb, 'B', '【答案：B】厂区主干道叉车最高行驶速度10km/h，仓库车间内限速5km/h。限速是预防碰撞事故的首要措施，载货时限速还应适当降低。（题库第177题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(356, 'single_choice', '叉车行驶中门架前倾的主要危害是？', '{"A":"重心前移、容易前翻","B":"转向变轻","C":"视线变好","D":"行驶平稳"}'::jsonb, 'A', '【答案：A】行驶时门架应后倾（后倾角6°-12°），防止货物滑落；前倾角3°-6°仅用于叉取和卸货操作。载货行驶时门架前倾会导致重心前移、前翻风险大增。（题库第178题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(357, 'single_choice', '雨雪天后路面结冰，室外叉车应当？', '{"A":"慢速行驶","B":"正常作业","C":"停止室外作业","D":"谨慎行驶"}'::jsonb, 'C', '【答案：C】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第179题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(358, 'single_choice', '叉车驾驶证严禁？', '{"A":"本人使用","B":"转借他人使用","C":"年审","D":"补办"}'::jsonb, 'B', '【答案：B】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第181题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(359, 'single_choice', '叉车行驶途中突发故障，正确做法？', '{"A":"原地维修","B":"开往空旷安全处停机检查","C":"继续作业","D":"低速开回库房不检查"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第182题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(360, 'single_choice', '货叉出现变形弯曲应当？', '{"A":"敲打校正","B":"加热矫正","C":"直接报废更换","D":"降低载荷使用"}'::jsonb, 'C', '【答案：C】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第183题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(361, 'single_choice', '叉车起步禁忌？', '{"A":"平稳起步","B":"起步立即高速加速","C":"鸣笛起步","D":"观察起步"}'::jsonb, 'B', '【答案：B】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第184题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(362, 'single_choice', '狭窄通道作业，哪种行驶方式更安全？', '{"A":"高速前进","B":"低速倒车行驶","C":"原地转弯","D":"快速超车"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第185题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(363, 'single_choice', '叉车严禁在电梯内进行什么操作？', '{"A":"低速驶入","B":"原地转弯掉头","C":"平稳驶出","D":"静止停留"}'::jsonb, 'B', '【答案：B】叉车在电梯内作业应低速驶入、禁止车内转弯掉头。电梯空间狭小，转弯会碰撞轿壁，且电梯承重分布不均可能卡阻。（题库第186题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(364, 'single_choice', '定期保养叉车最主要目的？', '{"A":"好看整洁","B":"应付检查","C":"降低故障、延长使用寿命","D":"方便停放"}'::jsonb, 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第187题）', NULL, NULL, NULL, 3, 4, 'published', NULL, 'admin', now(), now()),
(365, 'single_choice', '下列行为属于严重违章的是？', '{"A":"低速行驶","B":"佩戴劳保用品","C":"酒后操作叉车","D":"规范叉货"}'::jsonb, 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第188题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(366, 'single_choice', '货叉作业时，下方区域要求？', '{"A":"可短暂站人","B":"人员快速穿行","C":"严禁人员停留穿行","D":"随意通行"}'::jsonb, 'C', '【答案：C】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第189题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(367, 'single_choice', '禁止利用叉车进行哪种操作？', '{"A":"搬运托盘","B":"拖拽带动其他机械设备","C":"堆码货物","D":"转运纸箱"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第190题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(368, 'single_choice', '叉车门架导轨干涩卡顿应？', '{"A":"强行操作","B":"不用处理","C":"加注润滑黄油","D":"加水润滑"}'::jsonb, 'C', '【答案：C】门架不能升降故障大概率是液压系统故障。门架导轨干涩卡顿应加注润滑黄油，不可强行操作或加水润滑。门架升降异响应立即停机检查。（题库第191题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(369, 'single_choice', '关于空载叉车转弯，说法正确的是？', '{"A":"可以急转弯","B":"同样需要减速慢行","C":"车速越快越稳","D":"无需观察"}'::jsonb, 'B', '【答案：B】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第192题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(370, 'single_choice', '叉车安全管理制度建立主体是？', '{"A":"驾驶员","B":"用工企业单位","C":"维修人员","D":"管理人员"}'::jsonb, 'B', '【答案：B】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第193题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(371, 'single_choice', '叉车灯光喇叭失效，正确处置？', '{"A":"白天可作业","B":"禁止出车、报修后使用","C":"偏远区域作业","D":"低速凑合使用"}'::jsonb, 'B', '【答案：B】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第194题）', NULL, NULL, NULL, 3, 6, 'published', NULL, 'admin', now(), now()),
(372, 'single_choice', '195、安全叉运货物的核心前提是？', '{"A":"车速缓慢","B":"人员指挥","C":"货物重心居中","D":"货叉抬高"}'::jsonb, 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第1题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(373, 'single_choice', '196、叉车下坡哪种操作错误？', '{"A":"倒车下坡","B":"低速慢行","C":"正向高速冲坡","D":"禁止空挡滑行"}'::jsonb, 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第2题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(374, 'single_choice', '严禁私自加长货叉搬运货物，原因是？', '{"A":"重心偏移、极易翻车","B":"行驶太慢","C":"装卸麻烦","D":"容易磨损"}'::jsonb, 'A', '【答案：A】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第197题）', NULL, NULL, NULL, 3, 5, 'published', NULL, 'admin', now(), now()),
(375, 'single_choice', '叉车作业结束停机首要操作？', '{"A":"直接下车","B":"不用落叉","C":"关闭总电源、落叉拉手刹","D":"随意停放"}'::jsonb, 'C', '【答案：C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第198题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(376, 'single_choice', '下列人员不得报考叉车证的是？', '{"A":"年满20周岁","B":"色盲、视力障碍人员","C":"身体健康人员","D":"初中以上学历"}'::jsonb, 'B', '【答案：B】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库单选题第199题）', NULL, NULL, NULL, 3, 1, 'published', NULL, 'admin', now(), now()),
(377, 'single_choice', '叉车通过厂区路口最标准操作？', '{"A":"加速通过","B":"鸣笛直冲","C":"一慢、二看、三通过","D":"减速不观察"}'::jsonb, 'C', '【答案：C】交叉路口通行应遵循「一慢、二看、三通过」原则，礼让行人与直行车辆，减速鸣笛。厂区十字路口行驶速度5km/h以下。严禁抢行优先。（题库第200题）', NULL, NULL, NULL, 3, 3, 'published', NULL, 'admin', now(), now()),
(378, 'multi_choice', '叉车作业前驾驶员需要检查的项目有（）。', '{"A":"刹车制动","B":"转向系统","C":"液压管路","D":"灯光喇叭"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第2题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(379, 'multi_choice', '平衡重式叉车主要组成结构包含（）。', '{"A":"工作装置","B":"行走机构","C":"液压系统","D":"电气系统"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】平衡重式叉车的平衡重作用是平衡前部载荷、防止前倾翻。平衡重位于车体后部，通过杠杆原理抵消货物产生的向前倾覆力矩。（题库第3题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(380, 'multi_choice', '叉车严禁作业的行为有（）。', '{"A":"超载偏载","B":"载人行驶","C":"野蛮撞击货物","D":"高速急转弯"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车充电场地严禁烟火。充电时严禁靠近明火高温、严禁在密闭空间充电、严禁插拔充电器带电接头（产生电弧引燃氢气）、严禁混用劣质充电器。（题库第4题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(381, 'multi_choice', '电瓶叉车蓄电池日常维护正确操作有（）。', '{"A":"保持通风充电","B":"补充蒸馏水","C":"严禁明火靠近","D":"定期除尘接线柱"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第5题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(382, 'multi_choice', '叉车在湿滑路面行驶危害有（）。', '{"A":"轮胎打滑","B":"制动距离变长","C":"车辆甩尾","D":"容易侧翻"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第6题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(383, 'multi_choice', '叉车装卸货物正确操作包含（）。', '{"A":"垂直对位插取","B":"货物居中放置","C":"轻起轻落","D":"落叉平稳着地"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】装卸货物时车辆应处于熄火、拉手刹、挂空挡状态。靠近月台时减速，确认货叉与车厢对正。装卸完毕确认货叉清空后方可驶离。（题库第7题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(384, 'multi_choice', '内燃叉车日常保养十字作业法包含（）。', '{"A":"清洁","B":"润滑","C":"紧固","D":"调整、防腐"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车日常保养「十字作业法」：清洁、润滑、紧固、调整、防腐。每班次出车前必须执行，由当班驾驶员负责完成。日常保养能减少叉车故障发生率、延长使用寿命。（题库第8题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(385, 'multi_choice', '叉车发生侧翻常见原因有（）。', '{"A":"高速转弯","B":"超载偏载","C":"坡道掉头","D":"急刹车"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车侧翻常见原因：高速转弯、超载偏载、坡道掉头、急刹车。转弯半径越小越易侧翻，重载转弯尤其危险。侧翻瞬间驾驶员应紧握方向盘、身体向内倾，禁止跳车。（题库第9题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(386, 'multi_choice', '叉车驾驶员上岗必须具备的条件（）。', '{"A":"年满18周岁","B":"身体健康无色盲","C":"持有N1证书","D":"禁止酒后上岗"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第10题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(387, 'multi_choice', '下列哪些属于叉车危险作业场景（）。', '{"A":"人员密集通道","B":"狭窄密闭空间","C":"易燃易爆库房","D":"凹凸松软路面"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第11题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(388, 'multi_choice', '叉车液压系统常见故障有（）。', '{"A":"油管漏油","B":"起升无力","C":"门架自动下滑","D":"液压油温过高"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第12题）', NULL, NULL, NULL, 4, 2, 'published', NULL, 'admin', now(), now()),
(389, 'multi_choice', '两台叉车同场作业安全要求（）。', '{"A":"保持安全间距","B":"禁止并排高速行驶","C":"空载礼让重载","D":"窄道单向通行"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】两台叉车同场作业要求：保持安全间距（同向≥5米、并排≥2米）、禁止并排高速行驶、空载礼让重载、窄道单向通行。同时装卸同一堆货物要保持安全距离。（题库第13题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(390, 'multi_choice', '叉车停车规范要求包含（）。', '{"A":"停指定区域","B":"货叉完全落地","C":"拉紧手刹","D":"关闭总电源"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第14题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(391, 'multi_choice', '货物捆绑不牢固存在的风险（）。', '{"A":"运输途中散落","B":"货物倾倒","C":"砸伤人员","D":"损坏设备"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】货物捆绑不牢固禁止起升转运。捆绑松散存在的风险：运输途中散落、货物倾倒、砸伤人员、损坏设备。必须确认捆绑牢固后方可叉运。（题库第15题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(392, 'multi_choice', '叉车倒车安全操作要求（）。', '{"A":"提前鸣笛警示","B":"观察后方盲区","C":"低速行驶","D":"视线受阻专人指挥"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第16题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(393, 'multi_choice', '内燃叉车排气异常颜色包含（）。', '{"A":"黑烟","B":"蓝烟","C":"白烟"}'::jsonb, 'A,B,C', '【答案：A,B,C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第17题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(394, 'multi_choice', '叉车制动失灵正确处置方法（）。', '{"A":"松开油门","B":"低速摩擦障碍物减速","C":"禁止急打方向","D":"平稳靠边停机"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】制动失灵应急处置：①松油门降低速度 ②利用发动机牵阻减速 ③低速摩擦障碍物（护栏、路沿）减速 ④平稳靠边停机。严禁急打方向（防止侧翻），这是保命的关键。（题库第18题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(395, 'multi_choice', '高位堆垛作业安全要点（）。', '{"A":"垂直对位货架","B":"低速缓慢升降","C":"禁止人员下方停留","D":"货物居中平稳"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第19题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(396, 'multi_choice', '叉车安全防护装置包含（）。', '{"A":"防护顶棚","B":"护额架","C":"倒车报警器","D":"警示灯"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车安全防护装置（护顶架、安全带、倒车报警器、警示灯等）严禁擅自拆除。安全带防止侧翻时驾驶员被甩出，倒车报警器提醒后方人员避让。（题库第20题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(397, 'multi_choice', '下列哪些人员不得操作叉车（）。', '{"A":"无证人员","B":"酒后人员","C":"疲劳生病人员","D":"未成年人"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第21题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(398, 'multi_choice', '叉车链条损坏判定标准（）。', '{"A":"链条拉长超标","B":"链节变形","C":"严重锈蚀","D":"裂纹磨损"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第22题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(399, 'multi_choice', '厂区内叉车行驶禁止行为（）。', '{"A":"超速行驶","B":"随意超车","C":"占道停放","D":"穿插人流"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第23题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(400, 'multi_choice', '叉车坡道行驶正确做法（）。', '{"A":"上坡正向行驶","B":"下坡倒车行驶","C":"低速匀速通行","D":"坡道禁止掉头转弯"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第24题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(401, 'multi_choice', '货叉报废更换标准有（）。', '{"A":"出现裂纹","B":"弯曲变形","C":"磨损超10%","D":"严重锈蚀"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第25题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(402, 'multi_choice', '电瓶叉车充电禁止操作（）。', '{"A":"靠近明火高温","B":"密闭空间充电","C":"充电插拔带电接头","D":"混用劣质充电器"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第26题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(403, 'multi_choice', '叉车视线不良场景有（）。', '{"A":"货物超高遮挡","B":"拐角盲区","C":"昏暗库房","D":"雨雪大雾天气"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】货物超高遮挡视线时应倒车低速行驶，必要时有人指挥。拐角盲区应鸣笛低速通过。视线不良情况（雨雪大雾、昏暗库房）应停止作业或加强照明，严禁凭经验盲开。（题库第27题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(404, 'multi_choice', '叉车维修作业安全要求（）。', '{"A":"切断电源熄火","B":"垫牢车轮防滑","C":"货叉支撑固定","D":"禁止明火检修"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第28题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(405, 'multi_choice', '叉车轮胎异常情况包含（）。', '{"A":"气压不足","B":"花纹磨损","C":"鼓包开裂","D":"严重变形"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第30题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(406, 'multi_choice', '叉车堆码货物基本原则（）。', '{"A":"上轻下重","B":"居中码放","C":"整齐稳固","D":"不超边界"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】货物码放基本原则：上轻下重（重物在下、轻物在上）、居中码放、整齐稳固、不超过边界。堆叠高度一般不超过3-4层。码放过高会导致倒塌风险大，应限制高度。（题库第31题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(407, 'multi_choice', '内燃叉车冬季防冻措施（）。', '{"A":"加注防冻冷却液","B":"冷车怠速预热","C":"禁止明火烘烤油箱","D":"停放保温区域"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】冬季低温环境下电瓶叉车续航会变短，因为蓄电池化学反应速率随温度降低而减慢。应将叉车停放在保温区域，充电时确保环境温度在5°C以上。（题库第32题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(408, 'multi_choice', '叉车行驶时货物正确状态（）。', '{"A":"离地10-20cm","B":"门架轻微后倾","C":"货物居中不偏斜","D":"无松散晃动"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第33题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(409, 'multi_choice', '叉车会车避让原则（）。', '{"A":"空载让重载","B":"小车让大车","C":"支线让干线","D":"窄道一方停车"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】会车避让原则：靠右减速、空载让重载、小车让大车、支线让干线、下坡车让上坡车。窄道一方停车礼让，禁止互不相让抢行。两台叉车同向行驶安全间距不得小于5米。（题库第34题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(410, 'multi_choice', '液压油变质表现为（）。', '{"A":"颜色发黑","B":"浑浊杂质","C":"泡沫过多","D":"异味粘稠"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第35题）', NULL, NULL, NULL, 4, 2, 'published', NULL, 'admin', now(), now()),
(411, 'multi_choice', '叉车作业中人员安全禁忌（）。', '{"A":"禁止站货叉下方","B":"禁止倚靠车体","C":"禁止贴近行驶路线","D":"禁止攀爬货物"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第36题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(412, 'multi_choice', '叉车常见制动故障有（）。', '{"A":"制动偏刹","B":"刹车发软","C":"制动卡顿","D":"刹车异响"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第37题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(413, 'multi_choice', '防爆叉车适用作业环境（）。', '{"A":"粉尘车间","B":"易燃易爆仓库","C":"油气场所","D":"密闭化工车间"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】易燃易爆危险区域（粉尘车间、油气场所、密闭化工车间）必须使用专用防爆叉车。普通电瓶叉车不可代替防爆叉车使用，因普通叉车在运行中会产生电火花引燃可燃气体。（题库第38题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(414, 'multi_choice', '叉车空载行驶注意事项（）。', '{"A":"低速转弯","B":"禁止高速漂移","C":"门架保持后倾","D":"禁止载人"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第39题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(415, 'multi_choice', '叉车超载作业危害包含（）。', '{"A":"极易侧翻","B":"损坏液压系统","C":"轮胎过载变形","D":"门架永久变形"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】额定起重量是指标准载荷中心距（500mm）、标准起升高度下的最大允许起重量。当载荷中心距增大或起升高度增加时，实际允许起重量需按载荷曲线图相应降低。超载会缩短叉车寿命、引发侧翻。（题库第40题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(416, 'multi_choice', '雨天室外叉车作业限制（）。', '{"A":"降低行驶速度","B":"禁止急刹转弯","C":"积水深坑绕行","D":"结冰停止作业"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第41题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(417, 'multi_choice', '叉车起步前必要操作（）。', '{"A":"观察四周环境","B":"鸣笛警示","C":"松开手刹","D":"平稳缓慢起步"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第42题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(418, 'multi_choice', '下列属于特种设备的有（）。', '{"A":"平衡重式叉车","B":"前移式叉车","C":"托盘堆垛车"}'::jsonb, 'A,B,C', '【答案：A,B,C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第43题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(419, 'multi_choice', '叉车转向沉重原因有（）。', '{"A":"液压油不足","B":"转向节缺润滑","C":"轮胎气压低","D":"转向间隙过小"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】转向沉重原因：液压油不足、转向节缺润滑、轮胎气压低。方向盘自由转动量过大说明转向间隙过大，需检修转向系统。转向沉重应立即停机检查。（题库第44题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(420, 'multi_choice', '叉车长期停放保管要求（）。', '{"A":"货叉落地放平","B":"断电拉手刹","C":"断开电瓶负极","D":"停干燥通风处"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第45题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(421, 'multi_choice', '货物偏载容易造成的后果（）。', '{"A":"单侧倾斜","B":"车辆侧翻","C":"轮胎承压变形","D":"制动跑偏"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】货物重心居中是安全叉运的前提。偏载会导致叉车单侧侧翻——重心偏左则左侧易翻，重心偏后则后翻风险增大。码放货物应上轻下重、居中、整齐稳固。（题库第46题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(422, 'multi_choice', '叉车灯光系统包含哪些（）。', '{"A":"作业照明灯","B":"转向灯","C":"警示爆闪灯","D":"倒车灯"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第47题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(423, 'multi_choice', '叉车禁止叉运的货物有（）。', '{"A":"深埋固定重物","B":"松散无包装散料","C":"无防护危险品","D":"超长超宽超限货物"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第48题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(424, 'multi_choice', '事故应急处置基本原则（）。', '{"A":"先救人","B":"后保物","C":"保护现场","D":"及时上报"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第49题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(425, 'multi_choice', '叉车日常外观检查内容（）。', '{"A":"车身有无破损","B":"螺丝紧固情况","C":"管路有无渗漏","D":"链条磨损程度"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第50题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(426, 'multi_choice', '电瓶叉车电池损坏原因（）。', '{"A":"过度充电","B":"亏电长期停放","C":"高温暴晒","D":"电解液缺失"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】蓄电池鼓包原因是过充过热导致极板膨胀变形。电池损坏原因：过度充电、亏电长期停放（极板硫化）、高温暴晒、电解液缺失。循环寿命约1500次。（题库第51题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(427, 'multi_choice', '叉车通过铁路道口正确做法（）。', '{"A":"提前减速","B":"停车观察","C":"低速平稳通过","D":"禁止中途换挡"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第52题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(428, 'multi_choice', '叉车操作手柄包含哪些控制（）。', '{"A":"门架前倾后仰","B":"货叉升降","C":"侧移调节"}'::jsonb, 'A,B,C', '【答案：A,B,C】叉车操作手柄回中位时，液压动作停止，同时多路换向阀中位使液压泵卸荷，减少能量消耗和系统发热。不使用操作手柄时不可随意扳动。（题库第53题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(429, 'multi_choice', '仓库安全通道要求（）。', '{"A":"保持通畅无杂物","B":"禁止堆放货物","C":"禁止叉车长期停放","D":"张贴警示标识"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】厂区行驶限速规定：仓库车间内限速5km/h，主干道限速10km/h。此规定依据《工业企业厂内铁路道路运输安全规程》，目的是保障作业区域人员安全，避免高速碰撞。（题库第54题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(430, 'multi_choice', '叉车驾驶员劳保用品包含（）。', '{"A":"劳保鞋","B":"工作服","C":"防护手套"}'::jsonb, 'A,B,C', '【答案：A,B,C】叉车驾驶员必须穿戴劳保用品：劳保鞋（防砸防滑）、工作服、防护手套。严禁穿拖鞋、短裤、赤膊作业。严禁使用化纤防滑手套（易产生静电火花）。（题库第55题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(431, 'multi_choice', '内燃叉车机油作用包含（）。', '{"A":"润滑机件","B":"冷却降温","C":"密封防锈","D":"缓冲减震"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】内燃叉车机油作用：润滑机件、冷却降温、密封防锈、缓冲减震。机油不含「燃烧」功能。机油不足会损坏发动机，应立即补充。（题库第56题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(432, 'multi_choice', '叉车高位卸货注意事项（）。', '{"A":"对位准确","B":"缓慢落叉","C":"禁止人员靠近","D":"货物平稳放置"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第57题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(433, 'multi_choice', '造成液压油温过高原因（）。', '{"A":"长时间重载作业","B":"散热滤网堵塞","C":"液压油不足","D":"油路卡顿"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第58题）', NULL, NULL, NULL, 4, 2, 'published', NULL, 'admin', now(), now()),
(434, 'multi_choice', '叉车禁止停靠的位置（）。', '{"A":"消防通道","B":"路口拐角","C":"坡道斜坡","D":"人员密集通道"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第59题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(435, 'multi_choice', '叉车转弯盲区危险因素（）。', '{"A":"内轮差盲区","B":"人员突然闯入","C":"货物突出遮挡","D":"视线受阻"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】严禁在坡道上掉头或转弯。坡道转弯时叉车重心偏移，极易发生侧翻事故。必须驶至平坦区域后方可掉头。（题库第60题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(436, 'multi_choice', '特种设备报废条件包含（）。', '{"A":"达到使用年限","B":"无维修改造价值","C":"存在重大隐患","D":"检测不合格"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】货叉报废更换标准：出现裂纹、弯曲变形、磨损厚度超过原厚10%、严重锈蚀。货叉裂纹超过规定不可焊补后继续使用（焊接会改变金属结构强度），必须报废更换。（题库第61题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(437, 'multi_choice', '叉车空载、重载共同点禁止操作（）。', '{"A":"高速转弯","B":"急刹车","C":"坡道掉头","D":"野蛮操作"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第62题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(438, 'multi_choice', '叉车轮式制动优点有（）。', '{"A":"制动灵敏","B":"维护简单","C":"散热效果好"}'::jsonb, 'A,B,C', '【答案：A,B,C】制动发软原因是制动管路内有空气，空气可被压缩导致制动力传递效率下降。排除方法：排气作业（排出管路空气）。刹车油更换后也必须排空空气，否则制动效果大打折扣。（题库第63题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(439, 'multi_choice', '叉车作业环境恶劣的情况（）。', '{"A":"粉尘大","B":"高温闷热","C":"潮湿积水","D":"低温结冰"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】恶劣作业环境（粉尘大、高温闷热、潮湿积水、低温结冰）应做好防护措施。粉尘环境佩戴防尘口罩和防尘防护，潮湿积水注意防滑和电气绝缘，高温环境注意散热和液压油温。（题库第64题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(440, 'multi_choice', '货叉间距调节要求（）。', '{"A":"适配托盘宽度","B":"对称居中","C":"禁止一宽一窄","D":"紧固锁定"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】货叉间距应与托盘宽度适配、对称居中，禁止一宽一窄。货叉间距不当会导致货物受力不均、倾斜甚至滑落。调节后必须紧固锁定。（题库第65题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(441, 'multi_choice', '叉车倒车禁止行为（）。', '{"A":"高速倒车","B":"不观察盲区","C":"不鸣笛警示","D":"边倒车边转头闲聊"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】倒车作业要求：提前鸣笛警示（提醒后方人员避让）、观察后方盲区、低速行驶（不超过3km/h）。后方视线完全被挡时应有人指挥，严禁快速倒车。（题库第66题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(442, 'multi_choice', '企业叉车安全管理制度包含（）。', '{"A":"人员培训制度","B":"车辆维保制度","C":"作业操作规程","D":"隐患排查制度"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】企业（用工单位）是叉车安全第一责任人。企业必须建立叉车安全管理制度：人员培训制度、车辆维保制度、作业操作规程、隐患排查制度。安全培训频次至少一年一次。（题库第67题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(443, 'multi_choice', '叉车仪表盘报警指示灯包含（）。', '{"A":"机油压力","B":"水温报警","C":"电量提示","D":"故障报警"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车仪表盘红色指示灯代表故障报警。常见报警灯：机油压力报警、水温报警、电量不足提示、故障报警。红灯亮起应立即停机检查，不可继续作业。（题库第68题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(444, 'multi_choice', '叉车液压安全阀作用描述正确的是（）。', '{"A":"限制最高油压","B":"保护液压元件","C":"防止过载损坏","D":"禁止私自调节"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】液压系统工作压力由安全阀（溢流阀）控制，额定压力14-17.5MPa。安全阀限制系统最高油压，防止过载损坏液压元件。严禁私自调高溢流阀压力，否则会导致管路爆裂、油缸损坏。（题库第69题）', NULL, NULL, NULL, 4, 2, 'published', NULL, 'admin', now(), now()),
(445, 'multi_choice', '叉车涉水行驶注意事项（）。', '{"A":"水深不超轮胎1/2","B":"低速匀速通过","C":"涉水后检查制动","D":"禁止深水通行"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车涉水行驶水深不得超过轮胎1/2高度。涉水后必须检查制动性能（制动片进水会打滑），低速匀速通过，禁止深水通行。（题库第70题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(446, 'multi_choice', '货物倒塌常见原因（）。', '{"A":"码放过高","B":"重心偏移","C":"捆绑松散","D":"颠簸震动"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第71题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(447, 'multi_choice', '叉车维修禁止操作（）。', '{"A":"带电拆卸元件","B":"带压拆卸油管","C":"私自改装限速","D":"擅自拆除防护"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】维修液压系统前必须停机泄压、关闭电源。液压系统内残余高压油液可达14-17.5MPa，带压拆卸油管会导致高压油液喷溅，可致严重工伤。（题库第72题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(448, 'multi_choice', '叉车低速行驶适用场景（）。', '{"A":"拐角路口","B":"人员密集区","C":"狭窄通道","D":"装卸对位"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第73题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(449, 'multi_choice', '内燃叉车冒白烟原因（）。', '{"A":"环境温度低水汽大","B":"燃油含水","C":"发动机低温正常雾化"}'::jsonb, 'A,B,C', '【答案：A,B,C】内燃叉车排气颜色判断：黑烟=燃烧不充分（空气滤清器堵塞或喷油过多）；蓝烟=烧机油（活塞环磨损、气门油封失效）；白烟=含水或冷启动正常雾化（环境温度低水汽大）。（题库第74题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(450, 'multi_choice', '叉车链条保养方式（）。', '{"A":"定期加注润滑油","B":"调整松紧度","C":"清除表面灰尘油污","D":"锈蚀严重更换"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】出车前必检项目：轮胎（气压、花纹、鼓包）、制动（踏板行程、制动液位）、转向（灵活性、间隙）、液压（油位、管路渗漏）、灯光喇叭（照明灯、转向灯、警示灯、倒车灯）、链条（松紧度、变形、锈蚀）。（题库第75题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now());

INSERT INTO question (id, type, content, options, answer, explanation, image_url, reference_answer, scoring_criteria, score, knowledge_point_id, status, created_by, created_by_type, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(451, 'multi_choice', '叉车作业人员作业禁忌动作（）。', '{"A":"身体探出车体","B":"手脚伸出外侧","C":"行驶中上下车","D":"单手随意操作"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第77题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(452, 'multi_choice', '叉车坡道停车防护措施（）。', '{"A":"拉紧手刹","B":"货叉落地触地","C":"车轮垫三角木","D":"熄火断电"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】坡道长时间停车：拉紧手刹 + 货叉落地触地 + 车轮垫三角木 + 熄火断电。仅靠手刹不足以防止坡道溜车，必须多重制动。（题库第78题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(453, 'multi_choice', '造成叉车起步抖动原因（）。', '{"A":"离合结合过快","B":"轮胎气压不均","C":"路面凹凸不平","D":"传动部件松动"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车起步前应观察四周环境、鸣笛示意、松开手刹、平稳缓慢起步。起步抖动原因是离合结合过快，应均匀释放离合。起步后不可立即加速高速行驶。（题库第79题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(454, 'multi_choice', '叉车夜间作业必备条件（）。', '{"A":"照明设备完好","B":"警示灯正常","C":"降低行驶速度","D":"专人疏导人流"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】夜间作业必须开启照明灯，必要时开启示宽灯/警示灯。照明不良应停止作业，严禁凭经验或凭感觉行驶。灯光喇叭失效禁止出车作业。（题库第80题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(455, 'multi_choice', '托盘破损危害包含（）。', '{"A":"货物掉落","B":"叉取不稳","C":"倾斜翻车","D":"损坏货叉"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第81题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(456, 'multi_choice', '叉车安全行驶三大要素（）。', '{"A":"车速可控","B":"间距足够","C":"视野清晰"}'::jsonb, 'A,B,C', '【答案：A,B,C】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第82题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(457, 'multi_choice', '电瓶叉车省电保养技巧（）。', '{"A":"平稳起步减速","B":"禁止频繁启停","C":"避免重载爬坡","D":"定期补电保养"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】电瓶叉车行驶中突然断电，首先检查蓄电池接线是否松动或熔断丝是否熔断。这是电路断路的最常见原因，排除方法为紧固接线或更换熔断丝。（题库第83题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(458, 'multi_choice', '叉车碰撞事故主要原因（）。', '{"A":"观察不到位","B":"车速过快","C":"盲区判断失误","D":"违规抢行"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】事故应急处置基本原则：先救人后保物、保护现场、及时上报。发生伤人事故应立即停车保护现场、拨打120急救、上报管理部门、配合事故调查。（题库第84题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(459, 'multi_choice', '叉车液压油管破损危害（）。', '{"A":"液压油泄漏","B":"起升失灵","C":"路面打滑隐患","D":"设备损坏"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第85题）', NULL, NULL, NULL, 4, 2, 'published', NULL, 'admin', now(), now()),
(460, 'multi_choice', '叉车卸货落地正确流程（）。', '{"A":"对位停稳","B":"缓慢落叉","C":"确认平稳","D":"脱离货物"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第86题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(461, 'multi_choice', '叉车禁止使用的天气（）。', '{"A":"暴雨积水","B":"路面结冰","C":"大雾能见度低","D":"强风恶劣天气"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】突发天气应对：暴雨就近安全位置停靠（禁止高速冲回库房）；雨雪湿滑减速慢行、避免急刹急转、积水深坑绕行；结冰停止室外作业；大雾停止作业或加强照明。（题库第87题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(462, 'multi_choice', '叉车转向系统组成包含（）。', '{"A":"方向盘","B":"转向油缸","C":"转向拉杆","D":"转向桥"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第88题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(463, 'multi_choice', '叉车作业现场警示标识有（）。', '{"A":"限速标识","B":"禁止通行","C":"注意盲区","D":"安全通道"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第89题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(464, 'multi_choice', '叉车驾驶员职业要求（）。', '{"A":"持证上岗","B":"遵守规程","C":"服从管理","D":"杜绝违章"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第90题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(465, 'multi_choice', '叉车空载倒车下坡正确操作（）。', '{"A":"低速倒车","B":"轻踩制动","C":"禁止空挡滑行","D":"门架后倾"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】坡道作业规程：上坡正向行驶（货叉朝前），下坡倒车行驶（货叉朝后），防止货物前倾坠落。严禁坡道掉头、转弯、空挡溜车。坡道停车需脚刹+手刹+落叉+垫三角木。（题库第91题）', NULL, NULL, NULL, 4, 3, 'published', NULL, 'admin', now(), now()),
(466, 'multi_choice', '叉车液压油加注要求（）。', '{"A":"使用原厂型号","B":"油位标准区间","C":"过滤无杂质","D":"停机冷却加注"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】液压油变质判断四标准：颜色发黑（氧化变质）、浑浊杂质（含水）、泡沫过多（空气混入）、异味粘稠（高温分解）。发现任何一项都应及时更换液压油，否则会损坏液压泵和阀件。（题库第92题）', NULL, NULL, NULL, 4, 2, 'published', NULL, 'admin', now(), now()),
(467, 'multi_choice', '多人协同装卸货物要求（）。', '{"A":"专人指挥","B":"口令统一","C":"动作同步","D":"保持安全距离"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】装卸货物时车辆应处于熄火、拉手刹、挂空挡状态。靠近月台时减速，确认货叉与车厢对正。装卸完毕确认货叉清空后方可驶离。（题库第93题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(468, 'multi_choice', '叉车长期停用封存步骤（）。', '{"A":"清洁全车","B":"满电保养","C":"防锈涂抹","D":"遮挡防尘"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】叉车长期停放保管要求：货叉落地放平、断电拉手刹、断开电瓶负极、停放干燥通风处。长期不用应定期补电保养，防止电池亏电硫化。（题库第94题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(469, 'multi_choice', '叉车刹车油更换要求（）。', '{"A":"定期更换","B":"同型号油品","C":"排空油路空气","D":"更换后测试制动"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】刹车油更换要求：定期更换、同型号油品（DOT3/DOT4不可混用）、排空油路空气、更换后测试制动效果。不同型号刹车油沸点和粘度不同，混用会导致制动失效。（题库第95题）', NULL, NULL, NULL, 4, 4, 'published', NULL, 'admin', now(), now()),
(470, 'multi_choice', '下列属于叉车违章操作的是（）。', '{"A":"惯性滑行对位","B":"货叉顶推货物","C":"载人登高作业","D":"超载短途转运"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第96题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(471, 'multi_choice', '叉车高位堆垛禁止动作（）。', '{"A":"高空急转","B":"高空急落","C":"高空大幅度倾斜","D":"高空快速移动"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】高位作业安全要点：垂直对位货架、低速缓慢升降、禁止人员下方停留、货物居中平稳。禁止高空急转、急落、大幅度倾斜（最大倾斜10度）、快速移动。高位取货必须靠近货架、垂直对位。（题库第97题）', NULL, NULL, NULL, 4, 5, 'published', NULL, 'admin', now(), now()),
(472, 'multi_choice', '叉车防滑安全措施（）。', '{"A":"保持路面干燥","B":"清理油污积水","C":"降低行驶速度","D":"避免急刹转向"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第98题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(473, 'multi_choice', '特种设备安全监管内容包含（）。', '{"A":"定期年检","B":"人员持证","C":"维保记录","D":"隐患整改"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第99题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(474, 'multi_choice', '叉车车体防护部件作用（）。', '{"A":"顶棚防坠物","B":"护架防挤压","C":"保险杠防撞","D":"挡板防飞溅"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第100题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(475, 'multi_choice', '叉车启停规范操作（）。', '{"A":"空挡启动","B":"禁止带挡打火","C":"熄火前怠速降温","D":"禁止强制断电"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第101题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now()),
(476, 'multi_choice', '叉车作业中突发故障处理（）。', '{"A":"立即停机","B":"停靠安全区域","C":"禁止带病运行","D":"上报维修"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第102题）', NULL, NULL, NULL, 4, 6, 'published', NULL, 'admin', now(), now()),
(477, 'multi_choice', '叉车考试合格取证要求（）。', '{"A":"理论70分及格","B":"实操考试合格","C":"体检身体健康","D":"培训学时达标"}'::jsonb, 'A,B,C,D', '【答案：A,B,C,D】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库多选题第103题）', NULL, NULL, NULL, 4, 1, 'published', NULL, 'admin', now(), now());

-- 重置 IDENTITY 序列到 max(id)
SELECT setval(pg_get_serial_sequence('question', 'id'), COALESCE(MAX(id), 1), true) FROM question;
SELECT setval(pg_get_serial_sequence('course', 'course_id'), COALESCE(MAX(course_id), 1), true) FROM course;
SELECT setval(pg_get_serial_sequence('chapter', 'chapter_id'), COALESCE(MAX(chapter_id), 1), true) FROM chapter;
SELECT setval(pg_get_serial_sequence('knowledge_point', 'id'), COALESCE(MAX(id), 1), true) FROM knowledge_point;


-- ================================================================
-- 导入完成
-- 验证：
--   SELECT type, level, count(*) FROM question
--   WHERE knowledge_point_id IN (1,2,3,4,5,6) AND created_by_type='admin'
--   GROUP BY type, level ORDER BY type, level;
--   SELECT kp.name, q.level, count(*) FROM question q
--   JOIN knowledge_point kp ON q.knowledge_point_id = kp.id
--   WHERE q.created_by_type='admin' GROUP BY kp.name, q.level ORDER BY kp.name, q.level;
-- ================================================================
