-- #2499 管理端新增「章节审核」页（C9/C10），能力键 chapter.review 独立一键
-- （#1639「管理端一页一键」：只给题库审核、不给章节审核必须能表达出来）。
--
-- 不缩权：此前章节审核动作挂在 course.manage 下，持有它的角色本来就能审章节；
-- 现在把它展开成新键，持有者拿到的可达面不变。展开后 course.manage 仍保留
-- （课程/章节 CRUD 还用），只是不再覆盖审核动作。

INSERT INTO admin_role_capability (role_id, capability)
SELECT rac.role_id, 'chapter.review'
FROM admin_role_capability rac
WHERE rac.capability = 'course.manage'
ON CONFLICT DO NOTHING;
