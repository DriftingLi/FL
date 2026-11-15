-- 回滚 #2548 的商品目录扩列。存量两行的 category 归属随列一并消失，无需单独还原。
ALTER TABLE points_shop_item DROP CONSTRAINT IF EXISTS chk_points_shop_item_category;
ALTER TABLE points_shop_item DROP COLUMN IF EXISTS sort_weight;
ALTER TABLE points_shop_item DROP COLUMN IF EXISTS category;
ALTER TABLE points_shop_item DROP COLUMN IF EXISTS description;
ALTER TABLE points_shop_item DROP COLUMN IF EXISTS image_url;
