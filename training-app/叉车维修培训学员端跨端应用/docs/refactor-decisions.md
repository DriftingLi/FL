# 重构决策清单（spec #638 页面手术结论）
> 2026-09-08 · 来源：T02 mall（#640）、T03 profile（#641/#675–#678/#684）手术复盘；总纲 #638，手册 ADR-0007。

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

## 约束
- 冻结：uni-secure-storage、main.uts、App.uvue、manifest.json、config/、残值域 valuation 模块。
- API 兼容：请求形态不得变（GET 走手动 query 序列化）；既有 request/get/post 原样保留（expand–contract）。
- uni-app-x：文件 ≤600 行、目录 ≤2 层；数据所有权单一（谁消费谁持有、扁平下发，禁 `ref<any>`/`defineExpose` 反向桥接）；守护 M–S 禁：裸 builder 引用、`as unknown as`、模板裸插值/直调 import 函数、`ref<any`、`async : void`、可选对象 prop 成员直读。

## 验收标准
手术 PR 四门（现行口径见 ADR-0008；2026-09-11 修订见其状态行）：① 触及运行时面时 Android 真机逐页截图前后一致（低风险运行时面——仅 .uts 逻辑改动——免 ①②）、② 命中 MP-WEIXIN 面时微信开发者工具无报错（两道人工门须在 PR 正文「## 验收证据」段留四字段与产物）、③ npm test 全绿、④ 本地编译门（默认 ④c `npm run build:kotlin-all` → `.ci-verify/kotlin-all.log`；dev 专属面追加 ④a `npm run build:compile` → `.ci-verify/build.log`；结果可由脚本 `-PostToPr` 贴的 sha 绑定评论承载）、④b release 云打包按**打包面**触发（正式发版前必跑一次）；缺证据不再只是声明——由 `pr-evidence` 必检 + ruleset `approve=1` 兜底（落地票 #841）。epic 完成 = 主力 + auth 手术全部达标且 #652/#653/#654 收口。
