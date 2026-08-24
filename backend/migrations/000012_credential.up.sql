-- 目标证件（target credential）顶层分区：持证目标
CREATE TABLE IF NOT EXISTS credential (
    id          INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(50)  NOT NULL UNIQUE,
    name        VARCHAR(100) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    category    VARCHAR(30)  NOT NULL CHECK (category IN ('special_operation', 'skill_level')),
    level       INT          CHECK (level IS NULL OR (level BETWEEN 1 AND 5)),
    sort_order  INT          NOT NULL DEFAULT 0,
    status      SMALLINT     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_credential_status   ON credential (status);
CREATE INDEX IF NOT EXISTS idx_credential_category ON credential (category);
CREATE INDEX IF NOT EXISTS idx_credential_sort     ON credential (sort_order);
COMMENT ON TABLE credential IS '目标证件表（学员报考的外部持证目标，与证书模板区分）';
COMMENT ON COLUMN credential.code     IS '证件编码（唯一，如 forklift_n1）';
COMMENT ON COLUMN credential.category IS '类别：special_operation 特种作业上岗证 / skill_level 职业技能等级';
COMMENT ON COLUMN credential.level    IS '等级：仅 skill_level 类填 1-5（5 初级→1 高级），特种作业为 NULL';
COMMENT ON COLUMN credential.status   IS '状态：1-启用，0-停用';

-- 学员当前证件（可空，NULL 表示未预筛选）
ALTER TABLE hrwai_users ADD COLUMN IF NOT EXISTS current_credential_id INT REFERENCES credential(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_hrwai_users_current_credential ON hrwai_users (current_credential_id);
COMMENT ON COLUMN hrwai_users.current_credential_id IS '当前目标证件（单选上下文，NULL 表示未预筛选）';

-- 课程归属证件（V1 单归属，预留 M:N 扩展）
ALTER TABLE course ADD COLUMN IF NOT EXISTS credential_id INT REFERENCES credential(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_course_credential ON course (credential_id);
COMMENT ON COLUMN course.credential_id IS '所属目标证件（单归属分区）';

-- 题目归属证件
ALTER TABLE question ADD COLUMN IF NOT EXISTS credential_id INT REFERENCES credential(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_question_credential ON question (credential_id);
COMMENT ON COLUMN question.credential_id IS '所属目标证件（单归属分区）';

-- 种子数据：8 个目标证件（幂等）
INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('forklift_n1', '叉车司机N1证', '场内专用机动车辆作业-叉车司机N1', 'special_operation', NULL, 1, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('low_voltage_electrician', '低压电工证', '特种作业-低压电工（占位，内容建设中）', 'special_operation', NULL, 2, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('welder', '焊工证', '特种作业-熔化焊接与热切割作业（占位，内容建设中）', 'special_operation', NULL, 3, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('maintenance_L5', '工程机械维修工（叉车维修方向）五级', '职业技能等级 五级/初级工（占位，内容建设中）', 'skill_level', 5, 4, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('maintenance_L4', '工程机械维修工（叉车维修方向）四级', '职业技能等级 四级/中级工', 'skill_level', 4, 5, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('maintenance_L3', '工程机械维修工（叉车维修方向）三级', '职业技能等级 三级/高级工', 'skill_level', 3, 6, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('maintenance_L2', '工程机械维修工（叉车维修方向）二级', '职业技能等级 二级/技师', 'skill_level', 2, 7, 1)
ON CONFLICT (code) DO NOTHING;

INSERT INTO credential (code, name, description, category, level, sort_order, status) VALUES
    ('maintenance_L1', '工程机械维修工（叉车维修方向）一级', '职业技能等级 一级/高级技师', 'skill_level', 1, 8, 1)
ON CONFLICT (code) DO NOTHING;

-- 存量数据归一：历史课程/题目归为叉车N1（仅在列无值时）
UPDATE course SET credential_id = (SELECT id FROM credential WHERE code = 'forklift_n1') WHERE credential_id IS NULL;
UPDATE question SET credential_id = (SELECT id FROM credential WHERE code = 'forklift_n1') WHERE credential_id IS NULL;
