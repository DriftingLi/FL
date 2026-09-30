# 移动端课程按积分兑换接入（#1420）· 四门收口核对清单

> 来源票：#1424（收口登记）。核对基准：移动端 `ADR-0031`「课程兑换面状态机与购物车族清退」+ #1420 spec「验收边界」段。
> 本清单**只登记证据锚点、不改代码**；锚点均可第三方点开复核（入仓截图路径 / sha 绑定评论 / CI run / 机检结果行）。
> 核对日期：2026-09-30。

## 逐票 × 逐门

### #1421 ① 兑换闭环（货架真价 · 详情兑换 · 购物车族清退） — PR #1431（merged `10bca19f`）

| 门 | 判定 | 证据锚点 |
| --- | --- | --- |
| ①a 真机逐页截图 | ✅ 签 | `docs/verification/mall/1421/shelf-real-prices-top.png`、`shelf-real-prices-scrolled.png`；状态机四格 `docs/verification/courses/1421/cell-free.png`、`cell-notowned-enough.png`、`cell-notowned-lack-hint.png`、`cell-owned.png`、`confirm-modal.png`、`chapter-404-unlock.png`（逐格来源见同目录 `README.md`） |
| ② 微信门 | ✅ 免（合规） | 未命中 MP-WEIXIN 面：无 manifest.json/platformConfig.json 改动、diff 无条件编译 MP-WEIXIN 段（pr-evidence 判据） |
| ③ 契约门 | ✅ 签 | CI run https://github.com/DriftingLi/FL/actions/runs/36671064167 （mobile-test=pass，150 套件 / 2802 例） |
| ④ 编译门 | ✅ 签 | `KOTLIN_ALL_RESULT errors=0 classes=1525 files=122`（产物 `.ci-verify/kotlin-all.log`） |
| ①b 能力面 | ✅ 免 | 未命中能力面（无指纹/权限弹窗/上传/ROM 交互，ADR-0016） |

### #1422 ② 首页课程区接真实数据 — PR #1439（merged `4d325d2f`）

| 门 | 判定 | 证据锚点 |
| --- | --- | --- |
| ①a 真机逐页截图 | ✅ 签（含覆盖度说明） | `docs/verification/dashboard/1422/dashboard-real-data.png` + `dashboard-logcat-request.txt`（实发 `GET /courses?...&credential_id=7&filter=hot`→200、mapper 消费）；覆盖度如实声明见同目录 `README.md` |
| ② 微信门 | ✅ 签 | sha 绑定评论 https://github.com/DriftingLi/FL/pull/1439#issuecomment-5906747184 （`MP_WEIXIN_RESULT errorsTotal=0 exceptionsTotal=0 pageStack=1`；逐页导航 SKIP 为 automator 0.12.1 已知工具限制） |
| ③ 契约门 | ✅ 签 | CI run https://github.com/DriftingLi/FL/actions/runs/36682284562 （mobile-test=pass，150 套件 / 2810 例） |
| ④ 编译门 | ✅ 签 | `KOTLIN_ALL_RESULT errors=0 classes=1527 files=122`（④c 曾抓获 `.then` 返 Promise 的 Kotlin 错并修复） |
| ①b 能力面 | ✅ 免 | 未命中能力面（ADR-0016） |

> **覆盖度如实标注（非缺项）**：#1422 ①a 的「课程卡正向渲染像素」未入画——登录证件（credential_id=7）生产无热门课⇒区正确为空，且卡片在折叠线以下、只读 adb 无法滚动。卡片的「标题/价格/热标与后端一致」形状由 ③ `coursesContract` #1422 组钉死；运行时面以「请求实发形状 + 200 + mock 绝迹 + 无崩溃」出证。第三方若要补一张有卡的真机图，需把设备切到 credential_id=1（生产热门课挂在该证件）并手动滚到课程区。

### #1423 ③ 电商话术文案清扫 — 已关闭（零代码改动）

| 门 | 判定 | 依据 |
| --- | --- | --- |
| ①②③④ | — 不适用 | 无代码改动 ⇒ 无运行时面门。经维护者对照设计稿定稿：现状文案即终态（货架 `商品/必买清单/销量/可换商品` 为积分商城设计意图，价格已由 #1421 做成「N 积分」；首页 `热销/报名/购物车` 定稿不改）。关票评论 https://github.com/DriftingLi/FL/issues/1423#issuecomment-5907833743 |

## 原型分支留档（primary source）

- 状态机与提示形态的 primary source = 本地分支 `prototype/points-redeem-cta`（/prototype 绕行产物，out of main）。
- **确认无原型代码流入 master**：`git ls-tree -r origin/master | grep -i prototype` 为空；该分支**仅存本地、未推远端**。其产物以真机四格截图形态固化进 `docs/verification/courses/1421/`（即 #1421 ①a 锚点），代码本体不进任何交付 PR。

## 收口结论

- #1421 / #1422 四门证据**逐门签齐**，锚点可复核；#1423 判为零改动（设计稿为准）。
- **无缺项，无需开补救票。**
- tracer bullet「看到价 → 兑换 → 能学」在移动端跑通：三处价格面单点渲染、详情兑换 CTA 四态、章节撞墙回详情、购物车族死 UI 清退、首页接真数据。
- 父票 #1420 按 #1424 设计**不关闭**，仅在正文补「已交付范围」指针。
