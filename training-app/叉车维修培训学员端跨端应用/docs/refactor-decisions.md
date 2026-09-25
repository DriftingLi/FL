# 重构决策清单（spec #638 页面手术结论）
> 2026-09-08 · 来源：T02 mall（#640）、T03 profile（#641/#675–#678/#684）手术复盘；总纲 #638，手册 ADR-0007。

> **并列的另一份 spec（2026-09-18）**：验证层（三个「永远绿」的整改 + 判据「有没有牙齿」）另立一份 ——
> 见 `docs/spec-永绿整改.md`（来源：E2E 通道裁决会话，PR #1136 / issue #1139）。**两份互不改写**：
> 本文管**页面结构**，那份管**判据与守护**。

## 目标与范围
治理巨型文件（原 20 个 >600 行）、页面层重复无沉淀、api 契约不一致（类型错误到云打包才暴露）、守护不全四类问题；手段 = 页面层手术 + 域 api 契约收紧，按模块渐进绞杀。

## 选型（各附一句理由）
- 状态管理维持页面级 reactive、不上 Pinia——遵 ADR-0002，uni-app-x 兼容风险不值得。
- api 出口 = mapper-callback 家族（requestMapped/getMapped/postMapped）——与 ADR-0003 手动 JSON 映射同构。
- 顶层骨架不动、不做 features/ 大迁移——pages.json 53 路径 + easycom + UTS import 脆弱，负收益。
- 安全网 = 契约测试机检 + 静态守护 + 四门冒烟——uvue 无组件级测试生态。

## 明确不做
UI 重设计、性能优化、新增组件级测试、状态管理改造、拆子路由、iOS 验收、动非手术模块的页面结构；发现问题一律记 issue，不顺手改。

## 拆分步骤（* = 可与主线并行）
PR-0(#639)✅ → mall(#640)✅ → profile(#641)✅ → forum(T04，600 预算第三复评) → dashboard → practice → exam → courses → resume → ai-assistant → auth 三兄弟（login pin 在生物识别 PR 后）；*#652/#653 域 api 批量收紧；*#654 allowlist 退役收尾。

> **收尾现测（2026-09-25，#654 / T16 落地 ⇒ epic #638 结题）**：
> ① 上面点名的收口三件 **#652 / #653 / #654 全部合并**（T01–T16 逐票台账与代价见 ADR-0007 各复盘节）；
> ② **600 软预算的最终判定**（`node -e` 走 `utils/contractHarness.js` 现测，非回忆）：声明面 23 个模块里
> **19 个已翻成执法**（`budgetViolations()` 返回空），**4 个仍 `budget: 'pending'`** =
> `ai-assistant` / `points` / `recruiter` / `search`；全仓 231 份 `.uts` + `.uvue` 现测 **9 份 >600**
> （839 / 727 / 702 / 700 / 667 / 643 / 629 / 621 / 617），其中 8 份落在那 4 个 pending 模块内、
> 1 份是登记在基础设施面的共享件 `api/request.uts`（643，`INFRA.oversized` 且 drift 为空）；
> 本文件「目标与范围」记的开工基线是 **20 份 >600** ⇒ 收口到 9 份。
> ③ **pending 分两类，别混成一句「都达标了」**：`points` / `recruiter` / `search` 不在这串手术步骤里
> （本文「明确不做」= 不动非手术模块的页面结构），其 pending 属**设计内**；而 `ai-assistant` 是 T10（#648）
> 的正靶，该票按 **「进度：50%」关闭**（2026-09-10），其表内写 `ai-assistant.uvue` 486 行，**现测 839**
> ⇒ 关闭后无人执法、已回潮（连同 `ai-feature` 629 / `ai-settings` 621 / `api/aiAssistant.uts` 667）。
> 这笔登记给后续票，**不写进本 epic 的完成账**。
> ④ allowlist 机制（expand 侧）已删除：删除时名单里只剩规则 H 两条过期条目，摘除后 **F/G/H/I/J 五条**
> 机械坑位规则一律无豁免全量执法（票面写「四条」是 T01 立项时表里只有 F/G/H/I，J 是后来按遭遇加的）；
> 「不许把机制带回来」由 `utils/modulesDeclarationContract.test.js` C 组守；⑤ 「全项目 api 契约一致」至今仍不成立，
> 残表（`api/auth.uts` 登录族八条出口）由 **#1324 / T21** 承接。

## 约束
- 冻结：uni-secure-storage、main.uts、App.uvue、manifest.json、config/、残值域 valuation 模块。
- API 兼容：请求形态不得变（GET 走手动 query 序列化）；既有 request/get/post 原样保留（expand–contract）。
- uni-app-x：文件 ≤600 行、目录 ≤2 层；数据所有权单一（谁消费谁持有、扁平下发，禁 `ref<any>`/`defineExpose` 反向桥接）；守护 M–S 禁：裸 builder 引用、`as unknown as`、模板裸插值/直调 import 函数、`ref<any`、`async : void`、可选对象 prop 成员直读。

## 验收标准
手术 PR 四门（现行口径见 ADR-0008；2026-09-11 / 2026-09-12 修订见其状态行）：① 触及运行时面时 Android 真机逐页截图前后一致（**2026-09-16 修订**：① 拆成 ①a／①b —— ①a 由 agent 出证、按一次分支收口跑；**只有命中「能力面」**（指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互）**时 ①b 必过、由人给原文**。见 `docs/adr/0016-真机门的人工性收缩与按批取证.md`）（低风险运行时面——仅 .uts 逻辑改动——免 ①②；**2026-09-12 起 ① 是唯一的人工门**）、② 命中 MP-WEIXIN 面时微信开发者工具无报错（**2026-09-12 起为半自动门**：agent 可跑 `npm run build:mp-weixin-check` → `MP_WEIXIN_RESULT` + `.ci-verify/mp-weixin.log` + `.ci-verify/*.png`，结果可由 `-PostToPr` 贴的 sha 绑定评论承载；**执行人栏仍由人签收、合并仍由人**）、③ npm test 全绿、④ 本地编译门（默认 ④c `npm run build:kotlin-all` → `.ci-verify/kotlin-all.log`；dev 专属面追加 ④a `npm run build:compile` → `.ci-verify/build.log`；结果可由脚本 `-PostToPr` 贴的 sha 绑定评论承载）、④b release 云打包按**打包面**触发（正式发版前必跑一次）；缺证据不再只是声明——由 `pr-evidence` **可见检查**兜底（**2026-09-16 写实更正**：原文写「必检 + ruleset `approve=1`」，实测 ruleset `protect master` 的必检只有 `ci-summary`，`pr-evidence` 不进必检、`approve` 也未启用，理由见 ADR-0008「为何不装『必检 + approve』」；落地票 #841）。epic 完成 = 主力 + auth 手术全部达标且 #652/#653/#654 收口。
