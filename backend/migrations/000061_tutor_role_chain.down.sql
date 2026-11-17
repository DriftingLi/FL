-- 回滚 000061：退回「tutor 表无角色列」的旧形状。
-- 顺序：先摘 tutor.role_id（否则 tutor_role 被外键引用），再删两张表。
-- 本迁移只增列增表，不搬迁既有业务数据 ⇒ down 之后库形状与 000060 逐字等价（回滚 = 退回旧代码）。

ALTER TABLE tutor DROP COLUMN IF EXISTS role_id;

DROP TABLE IF EXISTS tutor_role_capability;
DROP TABLE IF EXISTS tutor_role;
