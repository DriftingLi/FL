-- 000061: 讲师授权链底座（#2599 / 规格 E1、E6）
--
-- 背景：到今天为止所有讲师都一模一样 —— 同一个 role 字符串、同一套静态能力。
-- 本迁移把「讲师 → 角色 → 能力」落成数据，让「只给某位讲师题库、不给课程」成为可能。
--
-- 为什么 seed 与建表在同一条迁移里（规格 E1）：建表与挂接若分两次发布，中间那段时间
-- 所有存量讲师都会 fail closed（role_id 为 NULL = 未授权），而讲师面是他们日常工作的入口。
-- 一条迁移内完成「建表 → seed 四角色 → 存量讲师全挂全权角色」，则不存在这个中间态。
--
-- 为什么 protected 角色（全权讲师）不落 tutor_role_capability 行：它的能力恒为
-- authz.ProtectedTutorCapabilities() 全量（由代码回答），于是能力词表仍只有 authz.go
-- 一个事实源，SQL 里不必抄第二份能力清单（抄一份就会漂移）。与 000041 的 admin_role 同姿势。
--
-- role_id 允许 NULL 且语义为「未授权」（fail closed）：新建讲师在挂上角色之前不该拿到任何
-- 能力（规格 E2）。存量行的挂接判据正是 role_id IS NULL，故本迁移对已有库与空库都幂等。

CREATE TABLE IF NOT EXISTS tutor_role (
    role_id    INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       VARCHAR(50)  NOT NULL UNIQUE,
    protected  BOOLEAN      NOT NULL DEFAULT FALSE,
    remark     VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
COMMENT ON TABLE tutor_role IS '讲师角色表（#2599）：不同讲师挂不同角色即拿到不同能力面';
COMMENT ON COLUMN tutor_role.protected IS '受保护角色（全权讲师）：能力恒为 authz 讲师侧全量，不可改能力、不可删除';

CREATE TABLE IF NOT EXISTS tutor_role_capability (
    role_id    INT         NOT NULL REFERENCES tutor_role(role_id) ON DELETE CASCADE,
    capability VARCHAR(64) NOT NULL,
    PRIMARY KEY (role_id, capability)
);
COMMENT ON TABLE tutor_role_capability IS '讲师角色能力（protected 角色不落行：其能力取 authz 全量）';

ALTER TABLE tutor ADD COLUMN IF NOT EXISTS role_id INT REFERENCES tutor_role(role_id);
COMMENT ON COLUMN tutor.role_id IS '所挂讲师角色；NULL = 未授权（fail closed）';

-- 四个预置角色（规格 E6）。角色名字面量与 authz.TutorRoleName* 逐字相同，
-- 由 PG 契约测试对账（backend/internal/api/tutor_role_contract_test.go）。
INSERT INTO tutor_role (name, protected, remark) VALUES
    ('全权讲师', TRUE,  '受保护角色：拥有全部讲师能力，不可编辑/删除'),
    ('题库讲师', FALSE, '只能维护题库与目录标签，不能改课程'),
    ('课程讲师', FALSE, '只能维护课程章节与个人文件库，不能改题库'),
    ('只读讲师', FALSE, '只能查看课程与章节，不能编辑')
ON CONFLICT (name) DO NOTHING;

-- 三个子角色的能力集（与 authz.TutorTemplateRoles() 逐键相同）。
-- protected 角色（全权讲师）刻意不插行：它的能力由代码回答。
INSERT INTO tutor_role_capability (role_id, capability)
SELECT r.role_id, c.capability
FROM tutor_role r
JOIN (VALUES
    ('题库讲师', 'tutor.access'),
    ('题库讲师', 'question.author'),
    ('题库讲师', 'catalog.author'),
    ('题库讲师', 'notification.use'),
    ('课程讲师', 'tutor.access'),
    ('课程讲师', 'course.author'),
    ('课程讲师', 'tutorfile.manage'),
    ('课程讲师', 'notification.use'),
    ('只读讲师', 'tutor.access'),
    ('只读讲师', 'notification.use')
) AS c(role_name, capability) ON c.role_name = r.name
ON CONFLICT (role_id, capability) DO NOTHING;

-- 存量讲师全部挂「全权讲师」—— 这是刻意的：上线当天没有一个讲师被降权。
-- 判据是 role_id IS NULL，故本句对已有库（首次执行）与空库都幂等。
UPDATE tutor
SET role_id = (SELECT role_id FROM tutor_role WHERE protected)
WHERE role_id IS NULL;
