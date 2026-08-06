-- 000024_training_catalog.up.sql
-- 培训数据模型扩展（LH-11 / LH-27）：
--  1. 专业方向 specialty（操作 / 维修 / 安全 / 电池）
--  2. 课程等级 course_level（入门 / 进阶 / 专项 / 认证）
--  3. 证书模板 certificate_template（含有效期 validity_days）
--  4. 前置课程 course_prerequisite（课程多对多）
--  5. 题库标签 question_tag（法规/结构/液压/电气/制动/故障诊断/应急等考点模块）
--  6. 题目-标签关联 question_tag_relation
-- course 表新增：specialty_id / level_id / theory_hours / practice_hours / certificate_template_id
-- 全部使用 IF NOT EXISTS / ON CONFLICT 幂等写法，可重复执行、可回滚，不破坏既有数据。
-- 种子数据使用 ON CONFLICT DO NOTHING：不覆盖管理端对种子行（名称/排序/启停）的修改。

BEGIN;

-- ===== 1. 专业方向 =====

CREATE TABLE IF NOT EXISTS specialty (
    specialty_id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code         VARCHAR(30)  NOT NULL UNIQUE,
    name         VARCHAR(100) NOT NULL,
    description  TEXT,
    sort_order   INT          NOT NULL DEFAULT 0,
    status       SMALLINT     NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE  specialty IS '专业方向表（课程目录一级节点）';
COMMENT ON COLUMN specialty.status IS '状态：1-启用，0-停用';

-- ===== 2. 课程等级 =====

CREATE TABLE IF NOT EXISTS course_level (
    level_id    INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(30)  NOT NULL UNIQUE,
    name        VARCHAR(50)  NOT NULL,
    description TEXT,
    sort_order  INT          NOT NULL DEFAULT 0,
    status      SMALLINT     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE  course_level IS '课程等级表（入门/进阶/专项/认证）';
COMMENT ON COLUMN course_level.status IS '状态：1-启用，0-停用';

-- ===== 3. 证书模板 =====

CREATE TABLE IF NOT EXISTS certificate_template (
    id            INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code          VARCHAR(50)  NOT NULL UNIQUE,
    name          VARCHAR(100) NOT NULL,
    description   TEXT,
    validity_days INT          NOT NULL DEFAULT 365,
    template_url  VARCHAR(500) NOT NULL DEFAULT '',
    status        SMALLINT     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE  certificate_template IS '证书模板表（有效期单位：天）';
COMMENT ON COLUMN certificate_template.validity_days IS '证书有效期（天）';
COMMENT ON COLUMN certificate_template.status IS '状态：1-启用，0-停用';

-- ===== 4. 前置课程（多对多） =====

CREATE TABLE IF NOT EXISTS course_prerequisite (
    course_id              INT NOT NULL REFERENCES course(course_id) ON DELETE CASCADE,
    prerequisite_course_id INT NOT NULL REFERENCES course(course_id) ON DELETE CASCADE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (course_id, prerequisite_course_id),
    CONSTRAINT chk_course_prerequisite_not_self CHECK (course_id <> prerequisite_course_id)
);
COMMENT ON TABLE course_prerequisite IS '课程前置课程关联表（A 的 course_id 行表示 A 的前置课程为 prerequisite_course_id）';
CREATE INDEX IF NOT EXISTS idx_course_prerequisite_reverse
    ON course_prerequisite (prerequisite_course_id);

-- ===== 5. 题库标签 =====

CREATE TABLE IF NOT EXISTS question_tag (
    id          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(50)  NOT NULL UNIQUE,
    name        VARCHAR(50)  NOT NULL,
    category    VARCHAR(50)  NOT NULL DEFAULT '',
    description TEXT,
    sort_order  INT          NOT NULL DEFAULT 0,
    status      SMALLINT     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE  question_tag IS '题库标签表（法规/结构/液压/电气/制动/故障诊断/应急等考点模块）';
COMMENT ON COLUMN question_tag.category IS '考点模块分类';
COMMENT ON COLUMN question_tag.status IS '状态：1-启用，0-停用';

-- ===== 6. 题目-标签关联（多对多） =====

CREATE TABLE IF NOT EXISTS question_tag_relation (
    question_id INT NOT NULL REFERENCES question(id) ON DELETE CASCADE,
    tag_id      INT NOT NULL REFERENCES question_tag(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (question_id, tag_id)
);
COMMENT ON TABLE question_tag_relation IS '题目-题库标签关联表';
CREATE INDEX IF NOT EXISTS idx_question_tag_relation_tag
    ON question_tag_relation (tag_id);

-- ===== 7. course 表扩展 =====

ALTER TABLE course
    ADD COLUMN IF NOT EXISTS specialty_id           INT REFERENCES specialty(specialty_id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS level_id               INT REFERENCES course_level(level_id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS theory_hours           INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS practice_hours         INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS certificate_template_id INT REFERENCES certificate_template(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_course_specialty ON course (specialty_id);
CREATE INDEX IF NOT EXISTS idx_course_level     ON course (level_id);
CREATE INDEX IF NOT EXISTS idx_course_cert_template ON course (certificate_template_id);
-- 学员端/管理端课程列表高频过滤：状态 + 专业方向 + 课程等级
CREATE INDEX IF NOT EXISTS idx_course_status_specialty_level ON course (status, specialty_id, level_id);

COMMENT ON COLUMN course.specialty_id IS '专业方向（目录一级节点）';
COMMENT ON COLUMN course.level_id IS '课程等级（入门/进阶/专项/认证）';
COMMENT ON COLUMN course.theory_hours IS '理论学时';
COMMENT ON COLUMN course.practice_hours IS '实操学时';
COMMENT ON COLUMN course.certificate_template_id IS '关联证书模板（有效期取模板 validity_days）';

-- ===== 8. 种子数据（幂等） =====

-- 专业方向：操作 / 维修 / 安全 / 电池
INSERT INTO specialty (code, name, description, sort_order, status)
VALUES ('operation', '操作', '叉车驾驶与装卸作业方向', 1, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO specialty (code, name, description, sort_order, status)
VALUES ('maintenance', '维修', '叉车结构与维修技术方向', 2, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO specialty (code, name, description, sort_order, status)
VALUES ('safety', '安全', '叉车安全操作与应急方向', 3, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO specialty (code, name, description, sort_order, status)
VALUES ('battery', '电池', '叉车动力电池方向', 4, 1)
ON CONFLICT (code) DO NOTHING;

-- 课程等级：入门 / 进阶 / 专项 / 认证
INSERT INTO course_level (code, name, description, sort_order, status)
VALUES ('beginner', '入门', '基础理论与基础技能，面向零基础学员', 1, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO course_level (code, name, description, sort_order, status)
VALUES ('intermediate', '进阶', '深入原理与常见维护保养技能', 2, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO course_level (code, name, description, sort_order, status)
VALUES ('specialized', '专项', '面向特定作业场景的专项技能训练', 3, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO course_level (code, name, description, sort_order, status)
VALUES ('certification', '认证', '对标职业标准与考核规范的认证课程', 4, 1)
ON CONFLICT (code) DO NOTHING;

-- 证书模板示例（有效期单位：天；N1/N2 特种设备作业人员证复审周期通常为 4 年）
INSERT INTO certificate_template (code, name, description, validity_days, template_url, status)
VALUES ('FORKLIFT_OPERATION_CERT', '叉车操作培训合格证书', '叉车安全操作培训合格证书模板（对标 TSG 81-2022 培训要求）', 1460, '', 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO certificate_template (code, name, description, validity_days, template_url, status)
VALUES ('FORKLIFT_MAINTENANCE_CERT', '叉车维修技能培训合格证书', '叉车维修技能专项培训合格证书模板', 1460, '', 1)
ON CONFLICT (code) DO NOTHING;

-- 题库标签：法规 / 结构 / 液压 / 电气 / 制动 / 故障诊断 / 应急
INSERT INTO question_tag (code, name, category, description, sort_order, status) VALUES
('regulation', '法规', '法规', '法规、标准与作业规范相关考点（含 TSG 81-2022）', 1, 1),
('structure', '结构', '结构', '叉车整车与零部件结构相关考点', 2, 1),
('hydraulic', '液压', '液压', '液压系统原理、元件与维护相关考点', 3, 1),
('electrical', '电气', '电气', '电气系统与电控相关考点', 4, 1),
('brake', '制动', '制动', '制动系统结构、原理与维护相关考点', 5, 1),
('fault_diagnosis', '故障诊断', '故障诊断', '故障现象识别与诊断排除相关考点', 6, 1),
('emergency', '应急', '应急', '应急处置与突发情况应对相关考点', 7, 1)
ON CONFLICT (code) DO NOTHING;

-- ===== 9. 既有课程回填（按原 category 映射到专业方向与等级，保持目录树立即可用） =====
-- CATEGORY_01-基础理论 → 维修/入门；CATEGORY_02-安全规范 → 安全/入门
-- CATEGORY_03-实操技能 → 操作/进阶；CATEGORY_04-进阶提升 → 维修/进阶
-- 仅回填 000001 seed 的 6 门内置课程，后续课程由管理端配置。

UPDATE course SET specialty_id = (SELECT specialty_id FROM specialty WHERE code = 'maintenance'),
                  level_id     = (SELECT level_id FROM course_level WHERE code = 'beginner')
WHERE category = 'CATEGORY_01' AND specialty_id IS NULL;

UPDATE course SET specialty_id = (SELECT specialty_id FROM specialty WHERE code = 'safety'),
                  level_id     = (SELECT level_id FROM course_level WHERE code = 'beginner')
WHERE category = 'CATEGORY_02' AND specialty_id IS NULL;

UPDATE course SET specialty_id = (SELECT specialty_id FROM specialty WHERE code = 'operation'),
                  level_id     = (SELECT level_id FROM course_level WHERE code = 'intermediate')
WHERE category = 'CATEGORY_03' AND specialty_id IS NULL;

UPDATE course SET specialty_id = (SELECT specialty_id FROM specialty WHERE code = 'maintenance'),
                  level_id     = (SELECT level_id FROM course_level WHERE code = 'intermediate')
WHERE category = 'CATEGORY_04' AND specialty_id IS NULL;

COMMIT;
