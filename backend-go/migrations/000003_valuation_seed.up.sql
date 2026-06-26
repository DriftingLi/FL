-- 000003_valuation_seed.up.sql
-- 残值评估模块种子数据（合并源 000004_seed + 000005_seed_repair 的 INSERT 部分）
-- 所有 INSERT 已加 ON CONFLICT DO NOTHING，重复执行安全
-- 品牌型号 UPDATE 见 000004_brand_models_repair

-- =====================================================
-- 1. 品牌初始数据
-- =====================================================
INSERT INTO brands (name, tier, k_brand) VALUES
    ('林德',   'tier1_intl',     1.10),
    ('丰田',   'tier1_intl',     1.10),
    ('永恒力', 'tier1_intl',     1.10),
    ('斗山',   'tier2_intl',     1.00),
    ('凯斯',   'tier2_intl',     1.00),
    ('海斯特', 'tier2_intl',     1.00),
    ('合力',   'tier1_domestic', 0.95),
    ('杭叉',   'tier1_domestic', 0.95),
    ('龙工',   'tier1_domestic', 0.95),
    ('比亚迪', 'tier2_domestic', 0.85),
    ('柳工',   'tier2_domestic', 0.85),
    ('中联重科','tier2_domestic', 0.85),
    ('宝骊',   'tier2_domestic', 0.85) ON CONFLICT DO NOTHING;

-- =====================================================
-- 2. 系数配置初始数据
-- =====================================================
INSERT INTO coefficient_configs (key, value, description) VALUES
    ('lambda_electric',   0.120000, '电动叉车时间衰减率 λ'),
    ('lambda_combustion', 0.100000, '内燃叉车时间衰减率 λ'),
    ('w_work_condition',  0.200000, '工况权重 w₁'),
    ('w_brand',           0.200000, '品牌权重 w₂'),
    ('w_condition',       0.500000, '车况权重 w₃'),
    ('w_market',          0.100000, '市场权重 w₄'),
    ('k_market',          1.000000, '市场系数 Km（Demo 阶段固定 1.00）'),
    ('confidence_range',  0.050000, '95% 置信水平对应的置信区间范围 ±5%') ON CONFLICT DO NOTHING;

-- =====================================================
-- 3. 部件配置初始数据（电动 68 条 + 内燃 75 条）
-- =====================================================

-- ---------- 电动叉车 10 类 68 条 ----------

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'motor', '电机系统', 0.2000, 'left_drive_motor',  '左驱动电机', 0.25),
    ('electric', 'motor', '电机系统', 0.2000, 'right_drive_motor', '右驱动电机', 0.25),
    ('electric', 'motor', '电机系统', 0.2000, 'lift_motor',        '提升电机',   0.25),
    ('electric', 'motor', '电机系统', 0.2000, 'steer_motor',       '转向电机',   0.15),
    ('electric', 'motor', '电机系统', 0.2000, 'motor_other',       '其它部件',   0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'hydraulic', '液压系统', 0.1500, 'hydraulic_pump',    '液压泵',       0.30),
    ('electric', 'hydraulic', '液压系统', 0.1500, 'hydraulic_tank',    '液压油箱',     0.15),
    ('electric', 'hydraulic', '液压系统', 0.1500, 'multiway_valve',    '多路控制阀',   0.25),
    ('electric', 'hydraulic', '液压系统', 0.1500, 'lever_mechanism',   '操纵杆机构',   0.10),
    ('electric', 'hydraulic', '液压系统', 0.1500, 'hydraulic_hose',    '油管',         0.10),
    ('electric', 'hydraulic', '液压系统', 0.1500, 'hydraulic_other',   '其它部件',     0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'body', '车身车体', 0.0800, 'overhead_guard', '护顶架',     0.15),
    ('electric', 'body', '车身车体', 0.0800, 'seat',           '座椅',       0.10),
    ('electric', 'body', '车身车体', 0.0800, 'cover_shell',    '罩及外壳',   0.10),
    ('electric', 'body', '车身车体', 0.0800, 'counterweight',  '平衡重',     0.10),
    ('electric', 'body', '车身车体', 0.0800, 'nameplate',      '铭牌及标签', 0.05),
    ('electric', 'body', '车身车体', 0.0800, 'shock_mount',    '减震支座',   0.10),
    ('electric', 'body', '车身车体', 0.0800, 'body_frame',     '车身和车体', 0.15),
    ('electric', 'body', '车身车体', 0.0800, 'cab',            '驾驶舱',     0.15),
    ('electric', 'body', '车身车体', 0.0800, 'body_other',     '其它部件',   0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'mast', '门架', 0.1200, 'lift_cylinder', '提升油缸',   0.20),
    ('electric', 'mast', '门架', 0.1200, 'tilt_cylinder', '倾斜油缸',   0.15),
    ('electric', 'mast', '门架', 0.1200, 'mast',          '门架',       0.15),
    ('electric', 'mast', '门架', 0.1200, 'fork_carriage', '货叉架',     0.10),
    ('electric', 'mast', '门架', 0.1200, 'load_backrest', '挡货架',     0.05),
    ('electric', 'mast', '门架', 0.1200, 'roller',        '滚轮',       0.05),
    ('electric', 'mast', '门架', 0.1200, 'chain',         '链条',       0.10),
    ('electric', 'mast', '门架', 0.1200, 'mast_hose',     '油管',       0.05),
    ('electric', 'mast', '门架', 0.1200, 'attachment',    '属具',       0.10),
    ('electric', 'mast', '门架', 0.1200, 'mast_other',    '其它部件',   0.05) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'battery', '蓄电池', 0.1800, 'charge_cable',   '充电电缆', 0.15),
    ('electric', 'battery', '蓄电池', 0.1800, 'battery_cell',   '电池单体', 0.50),
    ('electric', 'battery', '蓄电池', 0.1800, 'battery_case',   '电池箱体', 0.20),
    ('electric', 'battery', '蓄电池', 0.1800, 'battery_other',  '其它部件', 0.15) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'transmission', '传动系统', 0.0800, 'left_gearbox',   '左齿轮箱', 0.25),
    ('electric', 'transmission', '传动系统', 0.0800, 'right_gearbox',  '右齿轮箱', 0.25),
    ('electric', 'transmission', '传动系统', 0.0800, 'drive_coupling', '驱动联接', 0.25),
    ('electric', 'transmission', '传动系统', 0.0800, 'rigid_flex_hose','硬管和软管',0.15),
    ('electric', 'transmission', '传动系统', 0.0800, 'transmission_other', '其它部件',0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'steer_wheel', '转向和车轮', 0.0600, 'steer_valve',    '转向阀',   0.20),
    ('electric', 'steer_wheel', '转向和车轮', 0.0600, 'steer_cylinder', '转向油缸', 0.15),
    ('electric', 'steer_wheel', '转向和车轮', 0.0600, 'steer_axle',     '转向桥',   0.15),
    ('electric', 'steer_wheel', '转向和车轮', 0.0600, 'steer_hose',     '油管',     0.10),
    ('electric', 'steer_wheel', '转向和车轮', 0.0600, 'tire_wheel',     '轮胎/车轮',0.20),
    ('electric', 'steer_wheel', '转向和车轮', 0.0600, 'rim',            '轮辋',     0.10),
    ('electric', 'steer_wheel', '转向和车轮', 0.0600, 'steer_wheel_other','其它部件',0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'electric_1', '电器部分一', 0.0500, 'combo_gauge',     '组合仪表',     0.10),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'drive_controller','行驶控制器',   0.20),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'hydraulic_controller','液压控制器',0.15),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'steer_controller','转向控制器',   0.15),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'drive_module',    '行驶模块',     0.10),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'hydraulic_module','液压模块',     0.10),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'steer_module',    '转向模块',     0.05),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'accel_sensor',    '加速传感器',   0.05),
    ('electric', 'electric_1', '电器部分一', 0.0500, 'electric_1_other','其它部件',     0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'electric_2', '电器部分二', 0.0500, 'speed_sensor',     '速度传感器',   0.10),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'angle_sensor',     '角度传感器',   0.05),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'other_sensor',     '其它传感器',   0.05),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'contactor',        '接触器',       0.15),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'cooling_fan',      '散热风扇',     0.10),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'light',            '灯光',         0.10),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'warning_light',    '警示灯及信号', 0.05),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'key_switch',       '钥匙和开关',   0.10),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'cable_wiring',     '电缆和线路',   0.20),
    ('electric', 'electric_2', '电器部分二', 0.0500, 'electric_2_other', '其它部件',     0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('electric', 'charger', '充电机', 0.0300, 'internal_element', '内部元件', 0.50),
    ('electric', 'charger', '充电机', 0.0300, 'charger_cable',    '充电电缆', 0.30),
    ('electric', 'charger', '充电机', 0.0300, 'charger_other',    '其它部件', 0.20) ON CONFLICT DO NOTHING;

-- ---------- 内燃叉车 12 类 75 条 ----------

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'engine', '发动机', 0.2200, 'engine_startup',   '发动机启动性能', 0.25),
    ('combustion', 'engine', '发动机', 0.2200, 'idle_stability',   '怠速稳定性',     0.20),
    ('combustion', 'engine', '发动机', 0.2200, 'accel_response',   '加速响应',       0.20),
    ('combustion', 'engine', '发动机', 0.2200, 'engine_exhaust_smoke','排气烟色',   0.10),
    ('combustion', 'engine', '发动机', 0.2200, 'oil_pressure',     '机油及油压',     0.15),
    ('combustion', 'engine', '发动机', 0.2200, 'engine_other',     '其它部件',       0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'fuel', '燃油系统', 0.0800, 'fuel_tank',      '燃油箱',         0.20),
    ('combustion', 'fuel', '燃油系统', 0.0800, 'fuel_pump',      '燃油泵',         0.25),
    ('combustion', 'fuel', '燃油系统', 0.0800, 'fuel_filter',    '燃油滤清器',     0.10),
    ('combustion', 'fuel', '燃油系统', 0.0800, 'injector_carb',  '喷油器/化油器',  0.20),
    ('combustion', 'fuel', '燃油系统', 0.0800, 'fuel_hose',      '油管及接头',     0.10),
    ('combustion', 'fuel', '燃油系统', 0.0800, 'lp_tank',        'LP气罐及管路',   0.05),
    ('combustion', 'fuel', '燃油系统', 0.0800, 'fuel_other',     '其它部件',       0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'hydraulic', '液压系统', 0.1200, 'hydraulic_pump',    '液压泵',       0.30),
    ('combustion', 'hydraulic', '液压系统', 0.1200, 'hydraulic_tank',    '液压油箱',     0.15),
    ('combustion', 'hydraulic', '液压系统', 0.1200, 'multiway_valve',    '多路控制阀',   0.25),
    ('combustion', 'hydraulic', '液压系统', 0.1200, 'lever_mechanism',   '操纵杆机构',   0.10),
    ('combustion', 'hydraulic', '液压系统', 0.1200, 'hydraulic_hose',    '油管',         0.10),
    ('combustion', 'hydraulic', '液压系统', 0.1200, 'hydraulic_other',   '其它部件',     0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'steer_wheel', '转向和车轮', 0.0600, 'steering_box',   '转向器',     0.25),
    ('combustion', 'steer_wheel', '转向和车轮', 0.0600, 'steer_cylinder', '转向油缸',   0.15),
    ('combustion', 'steer_wheel', '转向和车轮', 0.0600, 'steer_axle',     '转向桥',     0.15),
    ('combustion', 'steer_wheel', '转向和车轮', 0.0600, 'steer_hose',     '油管',       0.10),
    ('combustion', 'steer_wheel', '转向和车轮', 0.0600, 'tire_wheel',     '轮胎/车轮',  0.20),
    ('combustion', 'steer_wheel', '转向和车轮', 0.0600, 'rim',            '轮辋',       0.05),
    ('combustion', 'steer_wheel', '转向和车轮', 0.0600, 'steer_wheel_other','其它部件', 0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'body', '车身车体', 0.0600, 'overhead_guard', '护顶架',     0.15),
    ('combustion', 'body', '车身车体', 0.0600, 'seat',           '座椅',       0.10),
    ('combustion', 'body', '车身车体', 0.0600, 'cover_shell',    '罩及外壳',   0.10),
    ('combustion', 'body', '车身车体', 0.0600, 'counterweight',  '平衡重',     0.10),
    ('combustion', 'body', '车身车体', 0.0600, 'nameplate',      '铭牌及标签', 0.05),
    ('combustion', 'body', '车身车体', 0.0600, 'shock_mount',    '减震支座',   0.10),
    ('combustion', 'body', '车身车体', 0.0600, 'body_frame',     '车身和车体', 0.15),
    ('combustion', 'body', '车身车体', 0.0600, 'cab',            '驾驶舱',     0.15),
    ('combustion', 'body', '车身车体', 0.0600, 'body_other',     '其它部件',   0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'electric', '电器部分', 0.0800, 'combo_gauge',     '组合仪表',     0.08),
    ('combustion', 'electric', '电器部分', 0.0800, 'generator',       '发电机',       0.20),
    ('combustion', 'electric', '电器部分', 0.0800, 'starter_motor',   '启动马达',     0.20),
    ('combustion', 'electric', '电器部分', 0.0800, 'preheater',       '预热装置',     0.05),
    ('combustion', 'electric', '电器部分', 0.0800, 'sensor',          '传感器',       0.10),
    ('combustion', 'electric', '电器部分', 0.0800, 'light',           '灯光',         0.08),
    ('combustion', 'electric', '电器部分', 0.0800, 'warning_light',   '警示灯及信号', 0.05),
    ('combustion', 'electric', '电器部分', 0.0800, 'key_switch',      '钥匙和开关',   0.08),
    ('combustion', 'electric', '电器部分', 0.0800, 'cable_wiring',    '电缆和线路',   0.10),
    ('combustion', 'electric', '电器部分', 0.0800, 'electric_other',  '其它部件',     0.06) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'mast', '门架', 0.1000, 'lift_cylinder', '提升油缸',   0.20),
    ('combustion', 'mast', '门架', 0.1000, 'tilt_cylinder', '倾斜油缸',   0.15),
    ('combustion', 'mast', '门架', 0.1000, 'mast',          '门架',       0.15),
    ('combustion', 'mast', '门架', 0.1000, 'fork_carriage', '货叉架',     0.10),
    ('combustion', 'mast', '门架', 0.1000, 'load_backrest', '挡货架',     0.05),
    ('combustion', 'mast', '门架', 0.1000, 'roller',        '滚轮',       0.05),
    ('combustion', 'mast', '门架', 0.1000, 'chain',         '链条',       0.10),
    ('combustion', 'mast', '门架', 0.1000, 'mast_hose',     '油管',       0.05),
    ('combustion', 'mast', '门架', 0.1000, 'attachment',    '属具',       0.10),
    ('combustion', 'mast', '门架', 0.1000, 'mast_other',    '其它部件',   0.05) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'exhaust', '排气系统', 0.0400, 'muffler',       '消声器',       0.20),
    ('combustion', 'exhaust', '排气系统', 0.0400, 'exhaust_pipe',  '排气管',       0.20),
    ('combustion', 'exhaust', '排气系统', 0.0400, 'dpf_catalytic', 'DPF/催化装置', 0.25),
    ('combustion', 'exhaust', '排气系统', 0.0400, 'exhaust_smoke', '排气烟度',     0.15),
    ('combustion', 'exhaust', '排气系统', 0.0400, 'emission_compliance','排放合规性',0.10),
    ('combustion', 'exhaust', '排气系统', 0.0400, 'exhaust_other', '其它部件',     0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'transmission', '传动系统', 0.1000, 'clutch',         '离合器',         0.20),
    ('combustion', 'transmission', '传动系统', 0.1000, 'gearbox',        '变速箱',         0.30),
    ('combustion', 'transmission', '传动系统', 0.1000, 'drive_shaft',    '传动轴/驱动桥',  0.20),
    ('combustion', 'transmission', '传动系统', 0.1000, 'torque_converter','液力变矩器',   0.15),
    ('combustion', 'transmission', '传动系统', 0.1000, 'rigid_flex_hose','硬管和软管',    0.05),
    ('combustion', 'transmission', '传动系统', 0.1000, 'transmission_other','其它部件',   0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'cooling', '冷却系统', 0.0500, 'radiator',      '散热器/水箱',   0.30),
    ('combustion', 'cooling', '冷却系统', 0.0500, 'water_pump',    '水泵',          0.20),
    ('combustion', 'cooling', '冷却系统', 0.0500, 'fan_belt',      '风扇及皮带',    0.15),
    ('combustion', 'cooling', '冷却系统', 0.0500, 'thermostat',    '节温器',        0.10),
    ('combustion', 'cooling', '冷却系统', 0.0500, 'coolant',       '冷却液/防冻液', 0.10),
    ('combustion', 'cooling', '冷却系统', 0.0500, 'water_hose',    '水管及接头',    0.05),
    ('combustion', 'cooling', '冷却系统', 0.0500, 'cooling_other', '其它部件',      0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'intake', '进气系统', 0.0500, 'air_filter',    '空气滤清器',   0.25),
    ('combustion', 'intake', '进气系统', 0.0500, 'intake_pipe',   '进气管路',     0.15),
    ('combustion', 'intake', '进气系统', 0.0500, 'turbocharger',  '涡轮增压器',   0.35),
    ('combustion', 'intake', '进气系统', 0.0500, 'intercooler',   '中冷器',       0.15),
    ('combustion', 'intake', '进气系统', 0.0500, 'intake_other',  '其它部件',     0.10) ON CONFLICT DO NOTHING;

INSERT INTO part_configs (forklift_type, category_code, category_name, category_weight, item_code, item_name, item_weight) VALUES
    ('combustion', 'brake', '制动系统', 0.0400, 'service_brake',   '行车制动',       0.30),
    ('combustion', 'brake', '制动系统', 0.0400, 'parking_brake',   '驻车制动',       0.20),
    ('combustion', 'brake', '制动系统', 0.0400, 'brake_master',    '制动总泵',       0.15),
    ('combustion', 'brake', '制动系统', 0.0400, 'brake_wheel',     '制动分泵/制动器',0.15),
    ('combustion', 'brake', '制动系统', 0.0400, 'brake_line',      '制动管路',       0.10),
    ('combustion', 'brake', '制动系统', 0.0400, 'brake_other',     '其它部件',       0.10) ON CONFLICT DO NOTHING;
