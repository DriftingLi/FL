-- 缺少版本的历史记录按创建时旧公式解释，新记录由应用写入显式版本。
ALTER TABLE evaluations ADD COLUMN formula_version varchar(32);
