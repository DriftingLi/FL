-- 000008_seed_more_original_prices.up.sql
-- 充实 original_prices 数据：基于网上真实报价补充更多品牌/车型/吨位组合
-- 数据来源：林德官方自营店、丰田经销商、合力/杭叉官网、阿里巴巴、1688、Machineryline 等
-- 价格单位：人民币元（含税参考价），均为新车指导价/市场价

-- =====================================================
-- 1. 补充缺失的品牌（中力）与系列（若已存在则跳过）
-- =====================================================
INSERT INTO brands (name, brand_type, k_brand, is_active) VALUES
    ('中力', '国产其他', 0.86, TRUE)
    ON CONFLICT (name) DO NOTHING;

-- 中力品牌的 "无" 系列（保证级联过滤至少返回一个选项）
INSERT INTO series (brand, name) VALUES
    ('中力', '无') ON CONFLICT (brand, name) DO NOTHING;

-- =====================================================
-- 1.1 补充缺失的系列（若已存在则跳过）
-- =====================================================
INSERT INTO series (brand, name) VALUES
    ('林德', 'E系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('丰田', '8FBE系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('丰田', '8FD系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('永恒力', 'EFG系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('永恒力', 'ETV系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('合力', 'CPD系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('合力', 'CPCD系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('杭叉', 'XE系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('杭叉', 'XH系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('杭叉', 'A系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('比亚迪', 'CPD系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('龙工', 'LG系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('中力', 'EPT系列') ON CONFLICT (brand, name) DO NOTHING;
INSERT INTO series (brand, name) VALUES
    ('中力', 'ES系列') ON CONFLICT (brand, name) DO NOTHING;

-- =====================================================
-- 2. 林德 E系列 电动平衡重式（进口一线，价格较高）
--    来源：林德官方自营店 e.kion-cn.com
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('进口一线', '林德', '电动平衡重式', 'E系列', 1.6, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 266500, TRUE),
    ('进口一线', '林德', '电动平衡重式', 'E系列', 2.0, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 280000, TRUE),
    ('进口一线', '林德', '电动平衡重式', 'E系列', 2.5, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 295000, TRUE),
    ('进口一线', '林德', '电动平衡重式', 'E系列', 3.0, '标准配置', '三级门架', 4500, '磷酸铁锂(LFP)', 302200, TRUE),
    ('进口一线', '林德', '电动平衡重式', 'E系列', 4.0, '标准配置', '三级门架', 4500, '磷酸铁锂(LFP)', 420000, TRUE),
    ('进口一线', '林德', '电动平衡重式', 'E系列', 5.0, '高配置', '三级门架', 4500, '磷酸铁锂(LFP)', 480000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 3. 林德 H系列 内燃平衡重式（补充更多吨位）
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('进口一线', '林德', '内燃平衡重式', 'H系列', 2.0, '标准配置', '两级门架', 3000, NULL, 155000, TRUE),
    ('进口一线', '林德', '内燃平衡重式', 'H系列', 2.5, '标准配置', '两级门架', 3000, NULL, 168000, TRUE),
    ('进口一线', '林德', '内燃平衡重式', 'H系列', 3.5, '标准配置', '三级门架', 4000, NULL, 195000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 4. 丰田 8FBE 电动平衡重式（补充更多吨位）
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('进口一线', '丰田', '电动平衡重式', '8FBE系列', 2.0, '标准配置', '两级门架', 3000, '铅酸', 210000, TRUE),
    ('进口一线', '丰田', '电动平衡重式', '8FBE系列', 3.0, '标准配置', '三级门架', 4500, '铅酸', 265000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 5. 丰田 8FD 内燃平衡重式（补充更多吨位）
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('进口一线', '丰田', '内燃平衡重式', '8FD系列', 2.5, '标准配置', '两级门架', 3000, NULL, 168000, TRUE),
    ('进口一线', '丰田', '内燃平衡重式', '8FD系列', 3.5, '标准配置', '三级门架', 4000, NULL, 195000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 6. 永恒力 EFG 电动平衡重式（补充更多吨位）
--    来源：永恒力官方报价
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('进口一线', '永恒力', '电动平衡重式', 'EFG系列', 1.5, '标准配置', '两级门架', 3000, '铅酸', 158000, TRUE),
    ('进口一线', '永恒力', '电动平衡重式', 'EFG系列', 2.5, '标准配置', '两级门架', 3000, '铅酸', 185000, TRUE),
    ('进口一线', '永恒力', '电动平衡重式', 'EFG系列', 3.0, '高配置', '三级门架', 4500, '磷酸铁锂(LFP)', 245000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 7. 永恒力 ETV 电动前移式（补充更多吨位）
--    来源：永恒力官方报价 229,600 元
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('进口一线', '永恒力', '电动前移式', 'ETV系列', 2.0, '标准配置', '三级门架', 5000, '铅酸', 229600, TRUE),
    ('进口一线', '永恒力', '电动前移式', 'ETV系列', 2.5, '高配置', '三级门架', 6000, '磷酸铁锂(LFP)', 280000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 8. 合力 CPD 电动平衡重式（补充更多吨位）
--    来源：合力官网报价 CPD15 约 8-12 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '合力', '电动平衡重式', 'CPD系列', 1.5, '标准配置', '两级门架', 3000, '铅酸', 98000, TRUE),
    ('国产一线', '合力', '电动平衡重式', 'CPD系列', 2.0, '标准配置', '两级门架', 3000, '铅酸', 115000, TRUE),
    ('国产一线', '合力', '电动平衡重式', 'CPD系列', 2.5, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 128000, TRUE),
    ('国产一线', '合力', '电动平衡重式', 'CPD系列', 3.5, '高配置', '三级门架', 4500, '磷酸铁锂(LFP)', 165000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 9. 合力 CPCD 内燃平衡重式（补充更多吨位）
--    来源：合力官网 FD30 约 12-18 万，CPCD50 约 25-35 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '合力', '内燃平衡重式', 'CPCD系列', 2.0, '标准配置', '两级门架', 3000, NULL, 75000, TRUE),
    ('国产一线', '合力', '内燃平衡重式', 'CPCD系列', 2.5, '标准配置', '两级门架', 3000, NULL, 82000, TRUE),
    ('国产一线', '合力', '内燃平衡重式', 'CPCD系列', 3.5, '标准配置', '三级门架', 4000, NULL, 105000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 10. 合力 内燃重型叉车（大吨位）
--     来源：合力 H2000 系列 10吨 40万+
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '合力', '内燃重型叉车', 'CPCD系列', 10.0, '标准配置', '三级门架', 4500, NULL, 380000, TRUE),
    ('国产一线', '合力', '内燃重型叉车', 'CPCD系列', 15.0, '标准配置', '三级门架', 4500, NULL, 450000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 11. 杭叉 XE 电动平衡重式（补充更多吨位）
--     来源：杭叉爱搬商城 AE系列 2吨锂电约 10 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '杭叉', '电动平衡重式', 'XE系列', 1.5, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 78000, TRUE),
    ('国产一线', '杭叉', '电动平衡重式', 'XE系列', 2.5, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 95000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 12. 杭叉 XH 电动平衡重式（高配置）
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '杭叉', '电动平衡重式', 'XH系列', 2.0, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 92000, TRUE),
    ('国产一线', '杭叉', '电动平衡重式', 'XH系列', 2.5, '高配置', '三级门架', 4500, '磷酸铁锂(LFP)', 125000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 13. 杭叉 A系列 内燃平衡重式（补充更多吨位）
--     来源：杭叉 3吨内燃约 9-10 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 2.0, '标准配置', '两级门架', 3000, NULL, 68000, TRUE),
    ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 2.5, '标准配置', '两级门架', 3000, NULL, 75000, TRUE),
    ('国产一线', '杭叉', '内燃平衡重式', 'A系列', 3.5, '标准配置', '三级门架', 4000, NULL, 95000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 14. 比亚迪 CPD 电动平衡重式（补充更多吨位）
--     来源：比亚迪叉车 CPD20/CPD25 约 10-12 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 1.6, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 92000, TRUE),
    ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 2.0, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 105000, TRUE),
    ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 2.5, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 118000, TRUE),
    ('国产一线', '比亚迪', '电动平衡重式', 'CPD系列', 3.0, '高配置', '三级门架', 4500, '磷酸铁锂(LFP)', 138000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 15. 龙工 内燃平衡重式（补充更多吨位）
--     来源：龙工 LG30DT 约 7-8 万，FD50 约 9.7-10.1 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产二线', '龙工', '内燃平衡重式', 'LG系列', 2.0, '标准配置', '两级门架', 3000, NULL, 62000, TRUE),
    ('国产二线', '龙工', '内燃平衡重式', 'LG系列', 2.5, '标准配置', '两级门架', 3000, NULL, 68000, TRUE),
    ('国产二线', '龙工', '内燃平衡重式', 'LG系列', 5.0, '标准配置', '三级门架', 4500, NULL, 128000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 16. 龙工 内燃重型叉车（大吨位）
--     来源：龙工 LG120 12吨约 35 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产二线', '龙工', '内燃重型叉车', 'LG系列', 12.0, '标准配置', '三级门架', 4500, NULL, 350000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 17. 电动托盘搬运车（新增车型数据，价格较低）
--     来源：中力官网 EPT15 约 7999-9900 元，EPT20 约 15000 元
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 1.5, '标准配置', '无', 0, '铅酸', 9900, TRUE),
    ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 2.0, '标准配置', '无', 0, '铅酸', 15000, TRUE),
    ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 2.0, '标准配置', '无', 0, '磷酸铁锂(LFP)', 17000, TRUE),
    ('国产其他', '中力', '电动托盘搬运车', 'EPT系列', 3.0, '标准配置', '无', 0, '磷酸铁锂(LFP)', 22000, TRUE),
    ('国产一线', '杭叉', '电动托盘搬运车', 'A系列', 2.0, '标准配置', '无', 0, '磷酸铁锂(LFP)', 28000, TRUE),
    ('国产一线', '合力', '电动托盘搬运车', 'CPD系列', 2.0, '标准配置', '无', 0, '铅酸', 25000, TRUE),
    ('进口一线', '永恒力', '电动托盘搬运车', 'EFG系列', 1.5, '标准配置', '无', 0, '铅酸', 35000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 18. 电动堆高车（新增车型数据）
--     来源：中力 ES系列 1.5吨约 21800 元，2吨约 52000 元
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产其他', '中力', '电动堆高车', 'ES系列', 1.5, '标准配置', '两级门架', 3000, '铅酸', 21800, TRUE),
    ('国产其他', '中力', '电动堆高车', 'ES系列', 1.5, '标准配置', '两级门架', 3000, '磷酸铁锂(LFP)', 25000, TRUE),
    ('国产其他', '中力', '电动堆高车', 'ES系列', 2.0, '标准配置', '两级门架', 3500, '铅酸', 52000, TRUE),
    ('国产一线', '杭叉', '电动堆高车', 'A系列', 2.0, '标准配置', '两级门架', 3300, '磷酸铁锂(LFP)', 55000, TRUE),
    ('国产一线', '合力', '电动堆高车', 'CPD系列', 1.5, '标准配置', '两级门架', 3000, '铅酸', 32000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 19. 电动前移式（补充国产品牌）
--     来源：杭叉前移式 2吨约 15 万，比亚迪前移式约 12 万
-- =====================================================
INSERT INTO original_prices (brand_type, brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, battery_type, original_price, is_active) VALUES
    ('国产一线', '杭叉', '电动前移式', 'A系列', 2.0, '标准配置', '三级门架', 6000, '铅酸', 150000, TRUE),
    ('国产一线', '比亚迪', '电动前移式', 'CPD系列', 1.5, '标准配置', '三级门架', 5500, '磷酸铁锂(LFP)', 120000, TRUE),
    ('国产一线', '合力', '电动前移式', 'CPD系列', 2.0, '标准配置', '三级门架', 6000, '铅酸', 135000, TRUE)
    ON CONFLICT DO NOTHING;

-- =====================================================
-- 20. 为新增的 (brand, vehicle_type, series) 组合补充 "无" 配置记录
--     复用 000006 的策略：对每个尚无 "无" 配置的组合，补一条最低原价的 "无" 记录
--     保证级联过滤在 config_type / mast_type / mast_height 步骤能命中 "无" 选项
-- =====================================================
INSERT INTO original_prices (
    brand_type, brand, vehicle_type, series, tonnage,
    config_type, mast_type, mast_height_mm, battery_type, original_price, is_active
)
SELECT DISTINCT
    op.brand_type, op.brand, op.vehicle_type, op.series, MIN(op.tonnage),
    '无', '无', 0, NULL, MIN(op.original_price), TRUE
FROM original_prices op
WHERE NOT EXISTS (
    SELECT 1 FROM original_prices p2
    WHERE p2.brand_type = op.brand_type
      AND p2.brand = op.brand
      AND p2.vehicle_type = op.vehicle_type
      AND p2.series = op.series
      AND p2.config_type = '无'
)
GROUP BY op.brand_type, op.brand, op.vehicle_type, op.series
ON CONFLICT DO NOTHING;
