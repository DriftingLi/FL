-- 000004_brand_models_repair.down.sql
UPDATE brands SET models = '[]'::jsonb WHERE models != '[]'::jsonb;
