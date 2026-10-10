-- #1618（段1/4）管理角色权限链：admin_role + admin_role_capability + admin.role_id。
--
-- 背景：管理端此前是**单一 admin 角色**——能力全绑死在 authz 静态表（backend/internal/authz/authz.go），
-- 超管无法给其它管理员分配权限。本迁移把「管理员 → 角色 → 能力」落成数据。
--
-- 为什么 seed 与建表在同一条迁移里：落地后 authz 静态表**不再回答 admin 的能力**（同一批改动里删掉
-- 那些行）。建表与挂接若分两次发布，中间那段时间所有管理员都会 fail closed —— 一条迁移内完成
-- 「建表 → seed 超管角色 → 存量管理员全部挂接」，则不存在这个中间态。
--
-- 为什么 protected 角色不落 admin_role_capability 行：它的能力恒为 authz 能力表的**全量**（由代码回答），
-- 于是能力词表仍只有 authz.go 一个事实源，SQL 里不必抄第二份能力清单（抄一份就会漂移）。
--
-- role_id 允许 NULL 且语义为「未授权」（fail closed）：新建管理员在挂上角色之前不该拿到任何能力。
-- 存量行的挂接判据正是 role_id IS NULL，故本迁移对已有库与空库都幂等。

CREATE TABLE admin_role (
    role_id    INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       VARCHAR(50)  NOT NULL UNIQUE,
    protected  BOOLEAN      NOT NULL DEFAULT FALSE,
    remark     VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
COMMENT ON TABLE admin_role IS '管理角色表';
COMMENT ON COLUMN admin_role.protected IS '受保护角色（超级管理员）：能力恒为 authz 能力表全量，不可改能力、不可删除';

CREATE TABLE admin_role_capability (
    role_id    INT         NOT NULL REFERENCES admin_role(role_id) ON DELETE CASCADE,
    capability VARCHAR(64) NOT NULL,
    PRIMARY KEY (role_id, capability)
);
COMMENT ON TABLE admin_role_capability IS '管理角色能力（protected 角色不落行：其能力取 authz 全量）';

ALTER TABLE admin ADD COLUMN role_id INT REFERENCES admin_role(role_id);
COMMENT ON COLUMN admin.role_id IS '所挂管理角色；NULL = 未授权（fail closed）';

INSERT INTO admin_role (name, protected, remark)
VALUES ('超级管理员', TRUE, '受保护角色：拥有全部管理能力，不可编辑/删除');

UPDATE admin
SET role_id = (SELECT role_id FROM admin_role WHERE protected)
WHERE role_id IS NULL;
