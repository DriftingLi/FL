# #1421 ①a 真机取证 —— 积分兑换接入①（货架真价 · 详情兑换 · 购物车族清退）

设备：Redmi `marble` / `23049RAD8C`，ABI `arm64-v8a`（取证基座唯一支持档）。
运行时面命中（`*.uvue`/`*.uts`）；非能力面（无指纹/权限弹窗/上传/ROM 交互）⇒ ①b 免人工签（ADR-0016）。

## 两类证据，各自如实标注来源

### A. 真机实拍（本会话 #1421 build，生产数据）— 目录 `mall/1421/`

| 文件 | 内容 | 判据 |
| --- | --- | --- |
| `mall/1421/shelf-real-prices-top.png` | 商城货架首屏 | 价格行走单点渲染（真实「免费」）；🛒 头部按钮、数量选择器、「已有 N 人参与学习」行**全部消失** |
| `mall/1421/shelf-real-prices-scrolled.png` | 货架下滑 | 排序栏无 🛒；列表项仅剩「标题 + 单点价格」，无金额、无销量 |

> 生产库当前**没有任何付费课程**（`points_price` 迁移只建列、种子全为 NULL），实测该账号整个目录恒「免费」。
> 因此「未拥有付费 → N 积分兑换」「确认弹窗」「余额不足·还差 N 分」三格**无法在生产真机上取到真实数据**——
> 造付费课 + 真调 redeem 会扣不可逆积分资产（ADR-0031 明令禁止），故这三格走下面的 B 类 primary source。

### B. 状态机四格形态（票面指定的 primary source）— 目录 `courses/1421/`

来源：原型分支 `prototype/points-redeem-cta`（同一台 marble/arm64 真机、adb tap+screencap 出证；
原型页用假数据 + 状态控制台逐格切换，**绝不真调 redeem**）。票面与移动端 ADR-0031 均把该分支定为
「状态机与提示形态的 primary source」。本目录是从该分支原样搬运的 blob（字节一致）。

| 文件 | 对应 AC 格 | 内容 |
| --- | --- | --- |
| `cell-free.png` | 免费 | `points_price=null/0` → CTA「开始学习」，零兑换痕迹 |
| `cell-notowned-enough.png` | 未拥有·余额足 | `points_price>0 & entitled=false & balance≥price` → CTA「N 积分兑换」，无提示 |
| `cell-notowned-lack-hint.png` | 未拥有·余额不足（含「还差 N 分」数值） | 同 CTA + 装饰提示「余额不足 · 还差 N 分 · 去赚积分」（候选 b 形态，ADR-0031 决策 3） |
| `cell-owned.png` | 已拥有 | `entitled=true` → 兑换 UI 消失、回归「开始/继续学习」（决策 6：不常驻） |
| `confirm-modal.png` | 确认必过 | `uni.showModal`「该课程需 N 积分解锁，确认兑换？」（决策 2） |
| `chapter-404-unlock.png` | 章节未解锁出路 | 404 支「去解锁」跳回详情兑换面，不内联兑换（决策 5） |

## 承重行为锁（③ 门，机器可检，非截图）

- `utils/coursePointsCtaBehavior.test.js`：CTA 五组组合成对断言 + 4 条注入变异（判别力）。
- `utils/coursePointsRedeemBehavior.test.js`：`loadUts` 真跑 `api/course.uts`/`api/points.uts` ——
  详情 mapper 对真 DTO 迁出 `points_price`/`entitled`、兑换动词请求形状与失败原样透出 + 变异注入。
- `utils/coursesContract.test.js` #1421 组：DTO 迁出、CTA 单点被货架+详情两处消费、购物车族零出现、
  chapter-view 零兑换调用、详情兑换面接线。
- ③ 门全量：150 套件 / 2801 例绿。④c 本地编译门：errors=0。
