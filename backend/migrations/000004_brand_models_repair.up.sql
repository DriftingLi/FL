-- 000004_brand_models_repair.up.sql
-- 补填品牌型号数据（合并源 000005 末尾 + 000006，仅更新 models 仍为空数组的记录）

UPDATE brands SET models = '["L14", "L16", "L20"]'::jsonb                       WHERE name = '林德'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["8FBE15U", "8FBE20U", "8FBE30U"]'::jsonb            WHERE name = '丰田'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["ETV 216", "EFG 425", "EFG 535"]'::jsonb            WHERE name = '永恒力'   AND models = '[]'::jsonb;
UPDATE brands SET models = '["D25S", "D30S", "D35C"]'::jsonb                     WHERE name = '斗山'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["580H", "588H", "590H"]'::jsonb                     WHERE name = '凯斯'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["H50FT", "H70FT", "H80FT"]'::jsonb                  WHERE name = '海斯特'   AND models = '[]'::jsonb;
UPDATE brands SET models = '["CPD30", "CPD35", "CPD50"]'::jsonb                  WHERE name = '合力'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["CPC30", "CPCD30", "CPCD50"]'::jsonb                WHERE name = '杭叉'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["LG30GLT", "LG35GLT", "LG50GLT"]'::jsonb            WHERE name = '龙工'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["ECB16", "ECB20", "ECB30"]'::jsonb                  WHERE name = '比亚迪'   AND models = '[]'::jsonb;
UPDATE brands SET models = '["CLG2030H", "CLG2050H", "CLG2080H"]'::jsonb         WHERE name = '柳工'     AND models = '[]'::jsonb;
UPDATE brands SET models = '["FD30", "FD50", "FD70"]'::jsonb                     WHERE name = '中联重科' AND models = '[]'::jsonb;
UPDATE brands SET models = '["KBE15", "KBE20", "KBE30"]'::jsonb                  WHERE name = '宝骊'     AND models = '[]'::jsonb;
