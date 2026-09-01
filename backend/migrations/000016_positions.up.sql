-- 问题4：岗位与专业方向解绑（管理员配置岗位字典）。
-- 新增 positions 字典表；job_postings.specialty_id → position_id；job_cards.expected_specialty_id → expected_position_id。
-- ⚠️ 生产执行前需人工确认：job_postings/job_cards 的既有 specialty 数据如需保留，
--    先人工映射到岗位字典再迁移（DROP COLUMN 不可逆，见 down 注释）。

CREATE TABLE positions (
    position_id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        VARCHAR(30)  NOT NULL UNIQUE,
    name        VARCHAR(50)  NOT NULL,
    description TEXT,
    sort_order  INT          NOT NULL DEFAULT 0,
    status      SMALLINT     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE positions IS '岗位字典（管理员配置；职位发布与简历期望岗位共用，与专业方向解耦）';

-- 种子岗位（叉车行业常见岗位；管理员可增删改）
INSERT INTO positions (code, name, description, sort_order, status) VALUES
    ('forklift_driver', '叉车司机', '叉车驾驶与货物搬运', 1, 1),
    ('maintenance_tech', '叉车维修技师', '叉车日常维修与保养', 2, 1),
    ('warehouse_keeper', '仓库管理员', '仓库货物管理与盘点', 3, 1),
    ('logistics_operator', '物流操作员', '物流分拣与装卸', 4, 1),
    ('equipment_supervisor', '设备主管', '设备管理与维护团队', 5, 1);

ALTER TABLE job_postings
    ADD COLUMN position_id INTEGER REFERENCES positions(position_id) ON DELETE SET NULL;
CREATE INDEX idx_job_postings_position ON job_postings (position_id);
COMMENT ON COLUMN job_postings.position_id IS '岗位字典（业务层必填；库层可空，字典项删除置空不级联）';
-- 移除专业方向列（问题4：职位完全移除专业方向）
ALTER TABLE job_postings DROP COLUMN IF EXISTS specialty_id;

ALTER TABLE job_cards
    ADD COLUMN expected_position_id INTEGER REFERENCES positions(position_id) ON DELETE SET NULL,
    ADD COLUMN expected_position_extra VARCHAR(100) NOT NULL DEFAULT '';
CREATE INDEX idx_job_cards_expected_position ON job_cards (expected_position_id);
COMMENT ON COLUMN job_cards.expected_position_id IS '期望岗位（岗位字典；专业方向历史别名的替代）';
COMMENT ON COLUMN job_cards.expected_position_extra IS '期望岗位自由补充（历史字段 expected_specialty_extra 的替代）';
ALTER TABLE job_cards DROP COLUMN IF EXISTS expected_specialty_id;
ALTER TABLE job_cards DROP COLUMN IF EXISTS expected_specialty_extra;
