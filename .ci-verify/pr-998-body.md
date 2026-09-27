## 改了什么 / 为什么改

补完 **#998（AI 助手页内容区滚不动）的剩余部分**：把 5 宫格入口做成**悬浮可及**。

#998 的滚动根因（`.pro-scroll` 只写 `flex: 1`、缺 `height: 0` ⇒ 容器长到内容高度、手势划不动）已在 PR #1008 修好，宫格「**能**滑回去够到」；但「有对话后入口不在手边」这一半当时按维护者指示**暂缓**，本 PR 落地它。

- 新增 `components/ai-chat/ai-chat-function-sheet.uvue`：半屏 sheet 承载**同一组**宫格入口（范式照 `ai-chat-pro-sheet.uvue`）；
- `ai-assistant.uvue` 新增悬浮按钮（FAB）：`v-if="messages.length > 0"` —— **无对话时宫格本就在首屏**，常驻等于「同一功能两个入口」，既冗余又永久占屏；
- 悬浮入口与首屏宫格**共用** `onFunctionClick`，**刻意不新增第二条发送路径**（两条路径必然漂移：一条带功能键、一条不带，那正是 #999「每轮送空键、专项通道静默失效」的成因形态）。

Refs #998（该票已 CLOSED：滚动部分随 #1008 合入；本 PR 补其票面「处置方向 2」）

## 改动类型
- [x] feat（新功能）

## 影响范围 / 风险点

- 仅移动端 AI 助手页：`pages/ai-assistant/ai-assistant.uvue`（+56 行）、新增 1 个组件、新增 1 个契约测试；**无后端、无接口、无数据库、无 web 改动**。
- 相对 `origin/master` 只有这 3 个文件，无跨模块夹带。**分支已 `merge origin/master`**（含 `#1013` 的 launch 派生修复 `a44f5ba7`、`#1004` forum 移动端、`#1012`），故 ③④ 两门已在同步后的 head `8db49693` 上重跑重贴。
- **两处「写错不报错、只会静默坏」的约束**（都已在代码注释里写明理由）：
  1. `manifest.json` 的 `styleIsolationVersion: "2"` ⇒ **组件样式默认隔离**，sheet 组件拿不到页面的 `.function-grid` 等规则（同仓对照证据：`ai-chat-bubble.uvue` 自己也重复定义了 `.message-*`）⇒ 宫格样式在组件内**自带**，两侧逐值一致由契约测试守护；
  2. FAB 的 `bottom: 240rpx` 是算出来的：`.input-card` 实测 202rpx 高（padding 20 + input-top 72 + input-bottom 8+64 + padding 14 + 卡片上下 margin 24，逐条见页面样式注释）⇒ 留 38rpx 余量，**不遮挡输入区**。
- `z-index` 分层：FAB = 100（低于抽屉的 100/101 与 sheet 的 300/301）⇒ 开抽屉或 sheet 时不会浮在遮罩之上。
- 未新增静态资源、未改 `pages.json` / `platformConfig.json` / 打包面。

## 验收证据

- ① Android 真机逐页截图对比 — 执行人：待维护者给出原文（agent 不得代填） · 日期：待设备插线后补 · 复测对象：AI 助手页宫格悬浮入口（有对话时 FAB 出现 → 点开 sheet → 选中一格发出一轮） · 结论（含产物）：**本 PR 尚未取真机证据** —— 设备 `f0bae674` 已从 USB 掉线（`adb devices` 为空），需重新插线后才能取证，截图将落 `docs/verification/ai-assistant/<本PR号>/`。**故本 PR 保持 draft、不合并。**
- ③ `npm run test:unit` 全绿 — 结论（含产物）：CI run https://github.com/DriftingLi/FL/actions/runs/34941790533 （该 run 的 `mobile-test` job = `npm run test:unit`，**同步 master 后 68 suites / 1077 tests 全绿**；本机复跑同结果。同步前的基线为 67 suites / 1063 tests）
- ④ 本地编译门（默认 ④c `npm run build:kotlin-all`） — 执行人：@zhengcookie（agent 执行） · 日期：2026-09-15 · 复测对象：整模块 Kotlin 编译（`unpackage/resources/app-android`） · 结论（含产物）：见评论 https://github.com/DriftingLi/FL/pull/1010#issuecomment-5676431652 （**同步 master 后重跑**：`KOTLIN_ALL_RESULT errors=0 classes=1263 files=96`，sha 绑定 `8db4969`；日志 `.ci-verify/kotlin-all.log`）
- ④b release 云打包（触及打包面时必填） — 执行人：@zhengcookie · 日期：2026-09-15 · 复测对象：不适用（未触及打包面） · 结论（含产物）：免（未改动三份 json / `uni_modules/**/utssdk/app-android/**` / `*.aar` / `libs/*.jar`）

**第②门免**：改动未命中 MP-WEIXIN 面（无 `MP-WEIXIN` 条件编译段，未改 `manifest.json` / `platformConfig.json`），依移动端 `docs/adr/0008-移动端验收门与证据.md` 不填该行。

> **本 PR 的 `pr-evidence` 检查会红，且是预期的**：① 未取真机证据（设备掉线），我没有编造截图路径来「凑绿」。此红即「未就绪、不可合并」的信号；插线补齐 ① 后才会转绿。
>
> 另：本 PR 的 draft 状态与 ① 的缺失是同一条纪律的两面 —— 按 ADR-0008「签收在人」，① 的「执行人」栏须由人给出原文、agent 只代录。

## 自检清单
- [x] 提交信息遵循 Conventional Commits
- [x] 一个分支只做一件事，可独立 revert
- [x] 未把其他会话/模块的改动夹带进来
- [x] 已关联对应 Issue（Refs #998）
