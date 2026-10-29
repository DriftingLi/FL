# 0050 - 积分商城页与 ai_pro 可兑登记

**状态**：已采纳（2026-10-29，来源：#1942 开工前调查 + 维护者三问裁定「货源 B / 验收 A / 入口 B」；落库：本 ADR 与同批实现）

## ① 目标与范围

把任务中心「积分商城」死按钮接成真页面：列表来自后端 `GET /points/shop/items`，兑换走既有单管线 `POST /points/shop/{sku}/redeem`。

开工前调查把票面一条隐含前提推翻了：该接口按 sku 对账表（根 ADR-0062 票2 D1）过滤，而全库唯一 enabled 种子行 `unreal_paper` 类行登记为「无读者」，`ai_pro` 在后端**零登记** ⇒ 生产商城恒 `[]`、任何 sku 兑换都被拒。**没有可兑物的页面验不出「兑换成功」**，所以本票必须同时回答「卖什么」。

范围：商城页（列表 / 已拥有 / 确认 / 余额提示 / 空错态）、两个入口（任务中心、积分明细 banner）、后端 `ai_pro` 一行种子 + 对账表登记。

不含：购物车与现金商品（移动端 0031 已清退）、真·推荐排序、评分/评价、SKU 分页（在售量级个位数）。

## ② 选型 + 一句理由

| 决策 | 一句话理由 |
| --- | --- |
| **ai_pro 登记为可兑 SKU**（迁移种子行 + `shopSKUDecls` 一条 `(ai_pro, ai_pro)`，读者 = 商城列表 `owned`） | 它已有权益读面（`useAiPro` 能力门）与价格常量，是唯一「登记即闭环」的候选；不登记就只能交一个空页面 |
| **验收第 3 条改口径：兑换成功 → 权益即时生效、落点可达**（不再写「课程可达」） | ai_pro 的落点是 AI 助手专业版；「课程可达」只有让商城卖课程才成立，而那与 `pages/mall` 课程商城重复（口径由维护者选 A） |
| **入口两处：任务中心「去兑换」+ 积分明细 banner「去积分商城」** | 设计稿两处都画了商城入口，只换一处会留下第二个死入口的观感 |
| **余额不足：确认弹窗里明说「会被服务端拒绝」并给「去赚积分」出路，但不客户端预扣、不擅自拦截** | 兑换不可退（管线无退单），说清代价是义务；扣不扣分仍只由服务端裁定（0031 决策 3 同口径） |
| **成功落点用 `AI_PRO_SKU` 常量判定，不写字面量** | 与能力门同一个 SKU 单点（`composables/useAiPro.uts`），两处字面量迟早漂移 |

## ③ 明确不做的事

- **不改** `unlock_real_paper` 的「只作卷价事实源、不可兑」身份（下它会退掉全部卷价，先例 `sku_registry_test.go`）。
- **不改** `useAiPro` 的数据源（仍按 `redeem_ai_pro` 流水推导）：它只差一个接口，但换源会动 #920 的整条门契约，不塞进本票。
- **不做** 本地预扣 / 本地写「已拥有」/ 本地缓存列表——已拥有与可兑性都只信服务端。
- **不做** SKU 富文本详情页、下单记录页（兑换记录已在积分流水 `积分支出` 里可见）。

## ④ 落库清单

| 面 | 文件 |
| --- | --- |
| 后端 | `backend/migrations/000049_shop_ai_pro_sku.{up,down}.sql`、`backend/internal/points/sku_registry.go`（`aiProSKU` + 登记）、`backend/internal/points/shop_ai_pro_contract_test.go` |
| 移动端 | `types/points.uts`（`ShopItem`/`ShopItemsResult`）、`api/points.uts`（`getShopItemsApi`）、`pages/points/points-shop.uvue`、`pages/points/task-center.uvue`（入口）、`pages/points/points-detail.uvue`（banner）、`pages.json`（新页登记） |
| 声明面 | `utils/modules.js`（points 新增页 + ai-assistant 消费者登记 points）、`utils/batch6UiContract.test.js`（页面枚举 60→61） |
| 守护 | `utils/pointsShopContract.test.js`（列表/可点性/确认/落点/两入口/对账表同源） |
