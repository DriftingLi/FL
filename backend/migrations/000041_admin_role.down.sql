-- 回滚 000041：退回「admin 表无角色列」的旧形状。
-- 顺序：先摘 admin.role_id（否则 admin_role 被外键引用），再删两张表。
-- 本迁移只增列增表，不搬迁既有业务数据 ⇒ down 之后库形状与 000040 逐字等价（回滚 = 退回旧代码）。

ALTER TABLE admin DROP COLUMN IF EXISTS role_id;

DROP TABLE IF EXISTS admin_role_capability;
DROP TABLE IF EXISTS admin_role;
