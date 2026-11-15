-- 积分商城商品目录扩列（#2548 / #2512 G-01 / N1）
-- 展示元数据落库而不是写死前端：改价/上下架/换图/改文案都在管理端完成，不需发版。
-- category 四值是原型筛选档（图书/资料/课程/其他）的唯一事实源；sort_weight 只负责目录排序。
ALTER TABLE points_shop_item ADD COLUMN IF NOT EXISTS image_url TEXT;
ALTER TABLE points_shop_item ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE points_shop_item ADD COLUMN IF NOT EXISTS category VARCHAR(16) NOT NULL DEFAULT 'other';
ALTER TABLE points_shop_item ADD COLUMN IF NOT EXISTS sort_weight INT NOT NULL DEFAULT 0;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'chk_points_shop_item_category'
      AND conrelid = 'points_shop_item'::regclass
  ) THEN
    ALTER TABLE points_shop_item
      ADD CONSTRAINT chk_points_shop_item_category
      CHECK (category IN ('book', 'material', 'course', 'other'));
  END IF;
END $$;

COMMENT ON COLUMN points_shop_item.image_url IS '商品图 URL（NULL = 前端落空态图）';
COMMENT ON COLUMN points_shop_item.description IS '商品描述（原型「使用期限」「注意事项」等运营文案落这一列）';
COMMENT ON COLUMN points_shop_item.category IS '商品分类：book|material|course|other（原型筛选档唯一事实源）';
COMMENT ON COLUMN points_shop_item.sort_weight IS '目录排序权重，越大越靠前；同权重按 price ASC, sku ASC';

-- 存量两行的归类：AI 专业版归课程、真题解锁归资料（不改动可兑性，只补展示分类）。
UPDATE points_shop_item SET category = 'course' WHERE sku = 'ai_pro';
UPDATE points_shop_item SET category = 'material' WHERE sku = 'unlock_real_paper';
