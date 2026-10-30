-- 积分商城：AI 专业版上架（#1942 口径 B / 2026-10-29 维护者裁定）
-- 只有这一行还不够：可兑放行判据是对账表 shopSKUDecls（ADR-0062 票2 D1），
-- 本行在 sku_registry.go 登记权益读者后才会进在售列表、才允许兑换。
INSERT INTO points_shop_item (sku, title, price, stock, enabled) VALUES
  ('ai_pro','AI 专业版',1000,NULL,true)
ON CONFLICT (sku) DO NOTHING;
