-- 000015_kc_modifiers_to_db.down.sql
-- 回滚：删除 4 个 Kc 修正项 key
-- 注意：回滚后 kcondition.go 会走 provider 失败兜底默认值（0.03/0.03/0.05/0.05），
--       算法退回旧加性公式语义，但 KcResult 字段结构无法回退，仅影响 PDF 展示

DELETE FROM coefficient_configs
WHERE key IN (
    'kc_paint_bonus',
    'kc_maintenance_bonus',
    'kc_no_license_penalty_pct',
    'kc_no_registration_penalty_pct'
);
