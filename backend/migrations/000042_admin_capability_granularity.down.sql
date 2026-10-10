-- 回滚 000042：把细键收回粗键（up 的逆），使库形状与旧代码认得的能力表一致。
--
-- 逆变换同样不能缩权：持有任一片细键的角色在旧代码里都只能靠粗键进门，故先补粗键再删细键。
-- admin.access 不在删除之列（它从未被本迁移删除），四条新细键按 up 的反向清掉。

INSERT INTO admin_role_capability (role_id, capability)
SELECT rac.role_id, m.old_key
FROM admin_role_capability rac
JOIN (VALUES
    ('catalog.manage', 'course.manage'),
    ('catalog.manage', 'position.manage'),
    ('catalog.manage', 'credential.manage'),
    ('content.manage', 'content.generate'),
    ('content.manage', 'featured.manage')
) AS m(old_key, new_key) ON m.new_key = rac.capability
-- 同 up：ON CONFLICT 紧跟在 JOIN ... ON 之后会被解析器读成那个 JOIN 的 ON。
WHERE true
ON CONFLICT DO NOTHING;

DELETE FROM admin_role_capability
WHERE capability IN (
    'hrwai_user.manage', 'tutor.manage', 'statistics.read', 'ai_config.manage',
    'course.manage', 'position.manage', 'credential.manage',
    'content.generate', 'featured.manage'
);
