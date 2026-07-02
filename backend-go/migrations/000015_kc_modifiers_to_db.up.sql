-- 000015_kc_modifiers_to_db.up.sql
-- 将 Kc（车况系数）的 4 个修正项从硬编码迁移到 coefficient_configs 表
-- 公式扩展：油漆/保养保持加性，车牌/登记证改为乘性（缺双证复合放大）
-- 默认值调整：油漆/保养 0.03→0.02（影响更小），证件 0.05→0.10（影响更大）

INSERT INTO coefficient_configs (key, value, description) VALUES
    ('kc_paint_bonus',                  0.020000,
     '原厂漆 Kc 加成（绝对值，叠加在车况评级 base 上；建议 0.01~0.05；保养类小幅加成）'),
    ('kc_maintenance_bonus',            0.020000,
     '维保记录 Kc 加成（绝对值，叠加在 base 上；建议 0.01~0.05；保养类小幅加成）'),
    ('kc_no_license_penalty_pct',       0.100000,
     '缺车牌 Kc 扣减比例（乘性因子，0.10 表示扣减 10%；证件类影响大，建议 0.08~0.15；缺双证时复合放大）'),
    ('kc_no_registration_penalty_pct',  0.100000,
     '缺登记证 Kc 扣减比例（乘性因子，0.10 表示扣减 10%；证件类影响大，建议 0.08~0.15；缺双证时复合放大）')
ON CONFLICT (key) DO UPDATE SET
    value       = EXCLUDED.value,
    description = EXCLUDED.description,
    updated_at  = NOW();
