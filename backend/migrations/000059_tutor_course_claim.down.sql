-- 回滚 000059：删认领表。
-- 只增表 + 回填，不改既有表的形状 ⇒ down 之后库与 000058 逐字等价（回滚 = 退回旧代码，
-- 此时编辑权谓词已随旧代码一起消失）。
DROP TABLE IF EXISTS tutor_course_claim;
