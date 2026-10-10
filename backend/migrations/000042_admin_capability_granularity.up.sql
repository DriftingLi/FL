-- #1639 管理端能力按侧栏叶子切分：把两个被删除的粗键在存量角色上**展开**成细键。
--
-- 不缩权：精确持有 admin.access 的角色仍能进仪表盘（该键保留），并拿到它原来就盖着的
-- 用户管理/讲师管理/统计/AI 配置四片；catalog.manage 与 content.manage 的持有者按同一片
-- 可达面拆成细键。展开完成后删掉两条粗键 —— 只加细键不删粗键，等于把可达面**扩大**了
-- （粗键在新代码里已不在 authz 能力表内，界面看不到、也管不着，但守卫仍会放行）。
--
-- 为什么旧键必须删干净：保存角色时后端按 authz 能力表收口能力键（表外键/白名单），
-- 表里留着已消失的键会让整次保存 400 —— 那是一个改不动角色的死界面。

INSERT INTO admin_role_capability (role_id, capability)
SELECT rac.role_id, m.new_key
FROM admin_role_capability rac
JOIN (VALUES
    ('admin.access', 'hrwai_user.manage'),
    ('admin.access', 'tutor.manage'),
    ('admin.access', 'statistics.read'),
    ('admin.access', 'ai_config.manage'),
    ('catalog.manage', 'course.manage'),
    ('catalog.manage', 'position.manage'),
    ('catalog.manage', 'credential.manage'),
    ('content.manage', 'content.generate'),
    ('content.manage', 'featured.manage')
) AS m(old_key, new_key) ON m.old_key = rac.capability
-- WHERE true 不是废话：紧跟在 JOIN ... ON 之后的 ON CONFLICT 会被解析器当成那个 JOIN 的 ON
-- （PostgreSQL 文档给出的消歧写法就是先用一个 WHERE 收束 FROM 子句）。
WHERE true
ON CONFLICT DO NOTHING;

DELETE FROM admin_role_capability WHERE capability IN ('catalog.manage', 'content.manage');
