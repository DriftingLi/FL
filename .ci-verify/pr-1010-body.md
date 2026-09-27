## 改了什么 / 为什么改

补完 **#998（AI 助手页内容区滚不动）的剩余部分**：让「5 宫格功能入口」在**滑动时也不丢失**，并顺手修掉一条与它耦合的能力口径问题。

#998 的滚动根因（`.pro-scroll` 缺 `height: 0`）已在 PR #1008 修好，宫格「**能**滑回去够到」；本 PR 落地该票票面「处置方向 2」，形态按维护者 2026-09-15 现场口径定稿：

- **入口上移常驻**：把 5 宫格**移出滚动容器**，放到导航栏下方做固定头部 ⇒「滑动不丢失」是**结构性保证**（实测：宫格锚点 y=517，滚完 y 仍=517；而滚动容器边界是 `[0,595][1156,1998]`）。
  **刻意不用 `position: sticky`**：该属性在整个移动端项目 **0 处先例**、uvue 支持亦无文档，写错只会静默失效。
- **`^` 收起 / `v` 展开**（`functionHeaderCollapsed`），**默认展开、不跨会话记忆**。
- **专项通道不吃「自定义模型」**：带 `featureKey` 的一轮由后端按管理端单绑定解析模型，并**忽略**选择子中的来源字段（`ai_config_service.go:494`「防绕过」）⇒
  ① `buildBody` 带键时早退，不再发送 `custom_api_key / custom_base_url / custom_model`；
  ② `onInputSend` 的「请先配置自定义模型」前置校验**只在不带键时生效**（此前它会拦住点宫格的人）。
  「取走 → 清空」仍早于全部早退守卫 —— **#999 不变量未动**（契约测试 ④ 组专门锁这条）。

**代价如实记**：固定头部位于档位横幅（解锁专业版）**之上**，横幅不再是首屏第一个元素。

Refs #998（该票已 CLOSED：滚动部分随 #1008 合入，本 PR 补其票面剩余部分）

## 改动类型
- [x] feat（新功能）

## 影响范围 / 风险点

- 仅移动端 AI 助手页：`pages/ai-assistant/ai-assistant.uvue` + 1 个契约测试；**无后端、无接口、无数据库、无 web 改动**；**未新增组件**（上一版的 `ai-chat-function-sheet.uvue` 已删除，不留「同一功能两个入口」）。
- 相对 `origin/master` 仅这 3 个改动面（页 / 测试 / 4 张取证截图），无跨模块夹带。分支已 `merge origin/master`（含 `#1013` 的 launch 派生修复 `a44f5ba7`）。
- 两条「写错不报错、只会静默坏」的事已由契约测试守护：**宫格必须在滚动容器之外**（含「塞回去必判红」的变形样本）、**不得依赖 position 定位**；另锁 `^` 三要素（默认值/切换/`v-if` 绑定）与「专项通道不吃自定义模型」。
- `scroll-y="true"`、`.pro-scroll` 的 `flex:1 + height:0`、根容器 `pageHeight` 三处**未动**，#998 既有滚动契约（`aiAssistantScrollContract`）与 #921 接线契约（`aiFeatureEntryWiringContract`）**均未修改且保持全绿**。

## 验收证据

- ① Android 真机逐页截图对比 — 执行人：@zhengcookie（代录；来源：维护者对本仓 ① 门「填 @zhengcookie，由我代填」的既有指示） · 日期：2026-09-15 · 复测对象：AI 助手页功能入口常驻与 `^` 收起/展开（`pages/ai-assistant/ai-assistant`） · 结论（含产物）：真机 `f0bae674` 自动取证（`uiautomator dump` 文本寻址 + **几何判据**，非肉眼）：① 宫格在首屏可见且可点；② **上滑后非宫格节点发生位移（内容确实滚了），而宫格锚点 y=517 逐值不变** ⇒ 滑动不丢失（滚动容器边界实测 `[0,595][1156,1998]`，宫格在其外）；③ 点 `^` 收起后宫格消失、把手仍在（箭头变 `v`）；④ 点 `v` 展开后宫格回到 y=517。产物：`docs/verification/ai-assistant/1010/01-ai-assistant-function-header-expanded.jpg`、`docs/verification/ai-assistant/1010/02-ai-assistant-scrolled-grid-pinned.jpg`、`docs/verification/ai-assistant/1010/03-ai-assistant-function-header-collapsed.jpg`、`docs/verification/ai-assistant/1010/04-ai-assistant-function-header-expanded-again.jpg`
- ③ `npm run test:unit` 全绿 — 结论（含产物）：CI run https://github.com/DriftingLi/FL/actions/runs/34944751182 （该 run 的 `mobile-test` job = `npm run test:unit`，**68 suites / 1063 tests 全绿**；本机复跑同结果）
- ④ 本地编译门（默认 ④c `npm run build:kotlin-all`） — 执行人：@zhengcookie（agent 执行） · 日期：2026-09-15 · 复测对象：整模块 Kotlin 编译（`unpackage/resources/app-android`） · 结论（含产物）：见评论 https://github.com/DriftingLi/FL/pull/1010#issuecomment-5676865517 （**同步 master 后重跑**：`KOTLIN_ALL_RESULT errors=0 classes=1258 files=95`，sha 绑定 `27c7fcc`；日志 `.ci-verify/kotlin-all.log`）
- ④b release 云打包（触及打包面时必填） — 执行人：@zhengcookie · 日期：2026-09-15 · 复测对象：不适用（未触及打包面） · 结论（含产物）：免（未改动三份 json / `uni_modules/**/utssdk/app-android/**` / `*.aar` / `libs/*.jar`）

**第②门免**：改动未命中 MP-WEIXIN 面（无 `MP-WEIXIN` 条件编译段，未改 `manifest.json` / `platformConfig.json`），依移动端 `docs/adr/0008-移动端验收门与证据.md` 不填该行。

> **取证方式如实声明**：① 的截图与判据由本机 `adb` 自动化产出（`input swipe` / `input tap` + `uiautomator dump`），**非人工逐页点击**；脚本 `D:\FL\.tmp-921-dev\verify-1010b.ps1`（未跟踪）。「执行人」栏按维护者既有指示填 `@zhengcookie`。
>
> **未在真机验证的一项**：④「专项通道不吃自定义模型」的**端到端**行为未上真机 —— 它需要把模型源切到「自定义模型」并留空模型名（或配置一个真实自定义模型），前者 UI 不允许、后者需要密钥（agent 不得索取/代填）。该项以**源码契约测试 + 代码评审**为证据，不计入 ① 的结论。

## 自检清单
- [x] 提交信息遵循 Conventional Commits
- [x] 一个分支只做一件事，可独立 revert
- [x] 未把其他会话/模块的改动夹带进来
- [x] 已关联对应 Issue（Refs #998）
