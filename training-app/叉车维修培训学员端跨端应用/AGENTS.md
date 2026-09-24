# 叉车维修培训与残值评估系统

面向叉车维修培训与叉车残值评估的全栈系统。架构、领域词汇与评审记录见下方文件：

- **领域词汇表**：`CONTEXT.md`（repo 根）
- **架构决策记录（ADRs）**：`docs/adr/`
- **AI/agent 工作约定**：`docs/agents/`

## Agent skills

### Issue tracker

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.

### 派活与执行契约（移动端）

接到**移动端** issue 时的唯一事实源：形态判定 / 读现状（认领、**查承载面**、状态现测）/ 执行 / **阶段闸门** / 收尾（进度契约、重复票处置）/ 正文格式。派活只需一行：`按 training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md 执行 <issue URL>`。See `docs/agents/issue-dispatch.md`.
（**仅移动端**：这套「阶段闸门 / 进度契约」是这条线的工作方式，不是全仓约定 —— 故它落在移动端 `docs/agents/`，前后端会话不读、不受约束。）

### Triage labels

五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.

### 技能供给与管线归属（仅移动端）

技能的**生效面**（harness 扫描根止于 git 根 ⇒ 子树里的技能副本不生效）与四条约定：生成物不入库 / 第三方技能内容不入库 / 技能改动独立成 PR / 引用技能文档引小节名不引行号。See `docs/agents/skills.md`；决策与逐项实测见 `docs/adr/0024-技能供给与管线归属.md`.
（**仅移动端**：前后端会话不读、不受约束；根 `D:\FL\.dsh\skills` 落点与根 `docs/agents/skills.md` 两项已裁定**不做**。）

### Domain docs

Single-context：root `CONTEXT.md` + `docs/adr/`。See `docs/agents/domain.md`.

### 决策文档约定（所有会话必须遵守）

- **ADR 文件名一律中文命名**：`docs/adr/NNNN-中文标题.md`（先例 `0007-渐进式重构手册.md`）；新建或重命名 ADR 禁止英文文件名；无法翻译的技术专名（如 SSE、JSON）可保留，但凡有中文对应的词（如 playbook→手册）必须用中文。
- **决策/重构清单固定六段结构、≤25 行**，顺序为：① 目标与范围（这次到底解决什么）→ ② 选型 + 一句理由（每项决策附一句话理由）→ ③ 明确不做的事（防止后续加戏）→ ④ 拆分步骤（按模块拆，标出可并行项）→ ⑤ 约束（不能动的模块、API 兼容、uni-app-x 兼容性）→ ⑥ 验收标准（怎么算完成）。先例 `docs/refactor-decisions.md`。
- **新决策回写纪律**：手术/实现会话收口时，若产生了 playbook 未覆盖的新决策或新坑位，须先回写对应 ADR（含守护规则落锁情况）再关票——issue 评论不是冷启动会话的必读面，ADR 才是。
- **写票时交付物必须列「承载面 UI」**：验收标准若要求某个界面形态（如「某页只列某区」「某页显示某状态」），**交付物清单必须包含承载它的 UI 改动**。先例 #1041：交付物只写「专项页带键（list + create）」，而 ⑥ 的验收与 ADR 的放宽理由都要求「列该区」的界面**存在**，实施时才发现 #1040 交付的专项页根本没有该界面（须补「最近对话」抽屉）—— 按票面字面做完会**复现要防的回归**，且守护的不变式在代码里为假。缺口与判定见 `docs/adr/0013-专项页会话分区与列该区前置.md` 的实施记录。

### 会话切分约定（所有技能会话遵守）

- **一条思考链一个会话**：grilling → to-spec → to-tickets 在同一会话内连续完成，中途不 clear/compact——spec 与票不只依赖结论，还依赖被否决的分支和否决理由。
- **一件票一个会话**：implement/execute-task 每票冷启动新会话，只读票 + 对应 ADR + `docs/refactor-decisions.md`；票间 `/clear`，上一票的讨论残留不进下一票。
- **换气只在阶段边界**：接近 smart zone（约 150k）用 `/compact`；跨目录、跨工具或会话中途分叉才用 `/handoff`。
- **收束与执行分座，重构与 UI 分时**：回填结论/补复盘另开短会话，不占手术会话；UI 原型对齐走独立 grilling 链，且同一模块内 UI 对齐 PR 一律排在该模块手术合并之后。

### Security scan

AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.

## 前端 UI 约定

页面保持整洁：不要写冗余的小标题、装饰性提示与说明性 hint 文本，有的话就清理，仅保留必要的功能性提示。删除 hint 时同步删除对应的 CSS class 与 scoped style，避免残留死代码。

**明确例外（不要清）**：

1. 论坛**发帖 / 回复输入区的属地披露提示**（「发布内容会显示 IP 属地」，文案单点 `utils/forumDisplay.uts` 的 `FORUM_REGION_NOTICE`）属「必要的功能性提示」而非装饰——属地在点发布那一刻才产生，事前告知比事后解释便宜（ADR-0045）。
2. Markdown 档的**能力与边界提示行**（文案单点 `utils/forumDisplay.uts` 的 `FORUM_MARKDOWN_BOUNDARY_NOTICE`）—— 它**只讲边界、不讲能力清单**（移动端 ADR-0025 ⑥-5），依据是根 ADR-0046 的「判据放在作者看得见的地方（编辑器提示 + 预览里的越界说明）」；同一条例外在根 `docs/agents/ui-conventions.md` 里对应 Web 侧的 `FORUM_MARKDOWN_HINT`。

两条都由共享输入区组件 `pages/forum/components/forum-markdown-input.uvue` 渲染（发帖与回复**同一份形态**，ADR-0025 P3），样式类是该组件的 `.region-notice` / `.boundary-notice`；页面侧原来的 `.reply-region-notice` / `.form-region-notice` 已随形态迁移删除。按本条约定清理 hint 时**跳过这两条**。

### Tailwind 增量共存四条边界规则

项目已引入 Tailwind CSS v4，与既有 `<style scoped>` 长期共存（详细背景见 `.workbuddy/plans/student-ui-redesign.md`）。共存期间遵守：

| 规则 | 内容 |
|---|---|
| **R1 原子类区** | `src/components/ui/**` 与已纳入改造的页面：模板只用原子类；`<style scoped>` 仅保留伪元素、`:deep()` 改 Element Plus、keyframes、媒体查询 |
| **R2 冻结区** | 未列入当期改造的页面与组件，scoped 样式**一行不动**。改共用件时新特性一律走 prop + 默认值等于现状，让未传值的调用方零 diff |
| **R3 禁双写** | 同一元素同一属性不允许既有 scoped 类又有原子类。需覆盖 Element Plus 外观时二选一：① scoped 内 `:deep()`；② 原子类加 `!` 后缀（`!px-4`） |
| **R4 迁移动作** | 页面改用原子类后，删除 scoped 块中已被替代的规则（沿用上一段「删 hint 同步删 CSS」的约定） |

**变体一律追加覆盖，不改写现有规则**：新增外观分支写成 `.xx.is-dark { … }` 这类多一个类的选择器追加在样式块末尾，特异性天然高于原单类规则，无需 `!important`。

**样式入口只有一个**：`src/assets/styles/tailwind.css`，新增全局样式写进它或它 `@import` 的文件，不要在 `main.ts` 里再加 import。

### 不得触碰的边界

- `--color-brand-*` 属**残值域**专用（`assets/styles/valuation-tokens.css` 在 `.valuation-root` 内定义），**禁止提升为全局变量** —— 会击穿 `layouts/ValuationLayout.vue` 与 `pages/ai-assistant/*` 两处依赖「变量未定义 → 走 fallback」的写法。培训域品牌色用 `--color-primary-*`。
- 残值模块（`pages/student/valuation/**`、`components/valuation/**`）本轮冻结，批量替换色值等机械操作时记得排除。

### uni-app-x (uvue) CSS 兼容性规则

本项目使用 uni-app-x (uvue 模式) 编译到 Android/iOS 原生端。uvue 的 CSS 引擎是**原生渲染器**，仅支持 CSS 属性的子集，与 Web CSS 有显著差异。编写 `.uvue` 文件的 `<style>` 时必须遵守：

#### 必须使用 `<style lang="scss">`

`.uvue` 文件的 `<style>` 块**必须**声明 `lang="scss"`，否则 SCSS 变量不会被预处理，直接传递给 uvue CSS 引擎会报错。

#### 选择器限制（严格）

uvue 原生端**只支持 class 选择器**，以下选择器均不可用：

| 选择器类型 | 示例 | 是否支持 |
|---|---|---|
| class 选择器 | `.className {}` | ✅ 支持 |
| group 选择器 | `.a, .b {}` | ✅ 支持 |
| descendant（后代） | `.parent .child {}` | ⚠️ 支持但有运行时性能损耗，Vapor 不支持 |
| child combinator | `.parent > .child {}` | ⚠️ 支持但有运行时性能损耗 |
| adjacent sibling | `.a + .b {}` | ⚠️ 支持但有运行时性能损耗，Vapor 不支持 |
| **tag 选择器** | `view {}`, `text {}` | ❌ **不支持** |
| **tag + combinator** | `> view + view` | ❌ **不支持** |
| ID 选择器 | `#id {}` | ❌ 不支持 |
| 伪类/伪元素 | `:first-child`, `::before` | ❌ 不支持 |
| 属性选择器 | `[attr] {}` | ❌ 不支持 |

**编写 CSS 时只能使用 `.class-name` 形式的选择器。** 模板中的 `view`、`text` 等标签名不能出现在选择器中。

#### 不支持的 CSS 属性与值

| 不支持的语法 | 替代方案 | 说明 |
|---|---|---|
| `gap` / `row-gap` / `column-gap` | 子元素加 margin class | 在 template 的子元素上添加 `ml-N`、`mr-N mb-N` 等 class |
| `text-decoration` | `border-bottom` 模拟 | 用具体颜色值（不支持 `currentColor`） |
| `calc() + env()` | 固定 rpx 值 | 安全区域需通过 `uni.getSystemInfoSync()` 动态获取 |
| `vh` / `vw` 单位 | 固定 rpx 值 | 仅支持 `number` 和 `pixel`（含 `rpx`） |
| `align-items: baseline` | `flex-start` 或 `center` | 仅支持 `center`/`flex-start`/`flex-end`/`stretch` |
| `max-height: 百分比` | 固定 rpx 值 | 仅支持 `number` 和 `pixel` |
| CSS 自定义属性 `var(--xxx)` | 直接写色值或用 SCSS 变量 | uvue 原生端不支持 CSS 变量 |
| `currentColor` | 具体颜色值 | 如 `#2979ff`、`#999999` |
| `display: grid` / `grid-*` | flex 布局 | grid 布局不支持 |
| `transition` / `animation` | uni-app API 动画 | 原生端不支持 CSS 动画 |
| `font-size` / `color` / `text-align` / `font-weight` 写在文字类元素**之外**（合法承载：`<text>` / `<button>` / `<input>` / `<textarea>`，`font-weight` 另多一个 `<loading>`） | 把文字挪进 `<text>` 子元素，让这些声明跟到 `<text>` 的 class 上（容器只留 padding / background / border） | 原生端只把**文字类样式**认给**文字类元素**，落在 `<view>` 上会被渲染层判错并**忽略** ⇒ 设计稿的字号 / 色值 / 字重从未生效过，且**每个非法位各打一条** error（本票两页术前每页 3 条、术后 0 条，#1269 真机实测；污染 ①a 的错误行判据）。真机日志原文：``style property `font-size\|color` is only supported on `<text>\|<button>\|<input>\|<textarea>`. there is an error on `<view class="mode-tab">` ``（#650 T12 的 ①a 实测，术前术后两轮同形）。全仓机检 = `utils/uvueFontCarrierContract.test.js`（#1269 立；只锁同族日志点名的这四个属性，`line-height` / `font-family` 无判据故不锁）；另有一批**已知存量**按「文件 + class + occurrence 数」登记在该文件的 `DEFERRED`（2026-09-24 立锁时 5 位 / 7 条），那张清单**只减不增** —— 同一登记位再加一条也判红，改到那一页时须在同一 PR 里收口并删条目。**具体是哪几处以那份清单为准，本表不抄**：抄过来就成了第二份真相，而棘轮的用途正是让它变短 |
| `white-space` 写在 `<text>` / `<button>` **之外**的元素上 | 横滑行改用 `flex-direction: row` + 子项 `flex-shrink: 0` 撑出溢出 | 该属性**只在 `<text>` / `<button>` 上有效**；写在 `<scroll-view>` 等元素上会被渲染层判错并**忽略**：真机日志原文 `style property white-space is only supported on <text>\|<button>. there is an error on <scroll-view …>`（2026-09-17 #1081 ①a 实测）。**承载是 `<text>` / `<button>` 的用法合法**，但必须在 `utils/uvueWhiteSpaceContract.test.js` 的 `LEGAL_CARRIER_SITES` 里**登记**（附理由）—— 正例：`pages/profile/personal-info.uvue` 的 `.code-btn-text`（承载是 `<text>`，「获取验证码」倒计时不换行，**不要删**）。存量 6 处非法写法（`favorites` / `records` / `practice-records` 的 `.filter-scroll`、`featured-list` 的 `.filter-scroll`、`ai-feature` 的 `.diag-chips`、`mock-exam` 的 `.palette-scroll`）已由 #1113 删除，同一守护已落锁 |

#### `gap` 替换模式

由于 uvue 不支持 tag 选择器，`gap` 替换必须在 **template 的子元素上直接添加 margin class**：

```vue
<template>
  <view class="flex-row">
    <view class="item">A</view>
    <view class="item ml-16">B</view>
    <view class="item ml-16">C</view>
  </view>
</template>

<style lang="scss">
  .flex-row { flex-direction: row; }
  .ml-16 { margin-left: 16rpx; }
</style>
```

- **非换行水平排列**：首个子元素不加 class，后续子元素加 `ml-N`
- **换行排列**：所有子元素加 `mr-N mb-N`
- **v-for 循环**：用 `:class="{ 'base': true, 'ml-N': idx > 0 }"` 条件添加

#### UTS 类型系统限制

UTS（uni-app-x 的 TypeScript 变体）不支持以下 TypeScript 语法：
- **交叉类型 + 内联对象字面量**：`type C = A & { field: type }` → 必须展平为独立类型定义
- **联合字面量类型用于运行时强转**：`'student' | 'tutor'` 编译到 Kotlin 后无法用 `as` 强转 → 统一用 `string`

##### Kotlin 编译期常见错误（云打包 / 发行模式全量编译会暴露）

| 错误写法 | 报错 | 正确写法 |
|---|---|---|
| `undefined` | 找不到名称 undefined | 空值统一 `null` |
| `String(x)` | None of the following candidates is applicable | `x.toString()` |
| `let x : any = null` | Null cannot be a value of non-null 'Any' | `let x : number \| null = null` 等可空类型 |
| `.catch((e : any) => …)` | None of the following candidates is applicable | `.catch((e) => …)` 或 `.catch((e : any \| null) => …)` |
| `Record<string,string> = {a:1}` | Cannot create an instance of an abstract class | `new Map<string,string>()` + `.set()` |
| 事件回调 `(e : any)` 访问 `e.detail` | Unresolved reference 'detail' | `e as UTSJSONObject` + `e['detail']` 索引 |
| 函数定义引用后声明的变量 | Unresolved reference（无 hoisting） | 变量声明前置到函数之前 |
| 模板对可空 `?:` 字段做 `>`/`<` 比较 | Operator call is prohibited on nullable receiver | `(x ?? 0) > 0` 兜底 |
| `getStorageSync` 返回值传给 `setStorageSync` | Argument type mismatch: Any? vs Any | `if (x != null) setStorageSync(k, x as any)` |

> `any` 在 UTS 编译成 Kotlin 的**非空** `Any`；需要可空时用 `any | null` 或具体可空类型。模板属性访问走宽松路径不报错，但 `<script>` 是严格 Kotlin 检查——报错全在 script，别被模板的"安静"误导。

#### 编译验证

修改 `.uvue` 文件后，在 HBuilderX 中重新编译，检查控制台：
- **ERROR** = 阻断编译，必须修复
- **WARNING** = 不阻断但原生端可能不生效（如 `gap` 被静默忽略，布局会坏）

## 测试与检查流程

改动后**必须**跑完对应栈的检查，全绿才能提交：

- **后端（`backend/`）**：Go 工具链在 `~/go/bin`（`export PATH=/home/root86155/go/bin:$PATH`）
  - `gofmt -l .`（应无输出）
  - `go vet ./...`
  - `golangci-lint run ./...`（errcheck 等静态检查）
  - `go test ./...`
  - 已知例外：`internal/api` 的 `TestStaticOtherResource` 在 WSL 下因 `static/favicon.ico` 权限问题失败，与改动无关，可忽略
- **前端（`frontend/`）**：`cd frontend`
  - `npm run type-check`（vue-tsc）
  - `npm test`（vitest）
- **部署配置**：改 `docker-compose*.yml` / `deploy.sh` 后可用 `docker compose -f docker-compose.prod.yml config -q` 做语法校验
- **安全检测**：改动触及认证/授权/密钥/DB 连接/AI 生成代码时，跑 `python -m deepsec shield scan backend frontend/src`，确认无新增 critical/high（已知误报见 `docs/agents/security-scan.md`）。

## 开发内循环（移动端 UI 迭代）

> 口径：**门是全量的，日常是分层的**。`scripts/compile-check.ps1`（④a，带干净缓存重建，实测 4.8–8 分钟）是**门**，默认值不动；
> `scripts/hx-run.ps1` 是**日常载体**，它有两条路：`npm run hx:compile-only`（**只编译拿诊断**，不碰设备）与
> `npm run hx:run`（**真运行到真机**，编译 + 推送 + 启动）。两者**都不是门、不进 `## 验收证据`**。
>
> **`--compile` 的语义坑位（2026-09-13，血账）**：HBuilderX 官方帮助里 `--compile` 是「**仅编译代码**」（默认 false），
> 不是「先编译再运行」。所以 **`--compile true` 只允许出现在两处**：全量编译门 `compile-check.ps1`，以及
> `hx-run.ps1` 的 `-CompileOnly` 分支；**真运行路径一个都不许有** —— 从前它照抄了门的调用，于是**从未真正请求过运行**，
> `编译成功 → 已停止运行...` 被误读成「运行失败」并追了整整一票（#917）。**反过来说，「仅编译」正是拿编译期诊断的捷径**（见下表第一行）。
> 判据同样有坑：**判「有没有到设备」必须相对基线比**（运行前后各取设备侧事实，如资源目录 mtime 前进），
> 拿「当前前台是不是基座」当判据在**重复运行到同一台机器**时恒为真 ⇒ 假绿。

### 三层节奏

| 层 | 触发 | 动作 | 成本 |
| --- | --- | --- | --- |
| **内循环（只编译拿诊断）** | 改完 `.uvue` / `.uts`，**先确认没有编译期诊断** | `npm run hx:compile-only`（= `hx-run.ps1 -CompileOnly`：官方语义的「仅编译」，**不推送 / 不启动 / 不轮询 / 不需要设备**，机检行 `mode=compile-only`） | **热缓存 257 秒**（2026-09-14 实测，落在旧口径 205–261 秒内 ✅）；**冷 / 上次被打断后 826–901+ 秒**（同日两次实测）—— **勿再引用「约 3–4 分钟」当常态** |
| **内循环（热刷新）** | 只改模板 / 样式，想看真机效果 | HBuilderX「运行到手机」+ 热刷新；**不勾干净缓存重建、不重装基座** | 秒级～1 分钟 |
| **内循环（真运行到真机）** | 要在真机上看行为 / 收口取证前的部署 | `npm run hx:run`（**真运行**：无干净缓存重建、不重装基座、不做 `adb install`；判定靠**设备侧事实相对基线前进**） | 约 4–5 分钟（实测编译 205–261 秒 + 推送 15–21 秒） |
| **中循环** | **每个「视觉满意点」一次**（不是每次微调） | `npm run test:unit` + ④ 本地编译门（`npm run build:kotlin-all` 够用；必要时 `npm run build:compile` 全量）→ **一次提交** | 分钟级 |
| **外循环** | **主题收口一次** | ①a 真机自动取证（`adb` 只读逐页截图 + logcat，**不抢焦点**）→ 开 PR；证据 sha 对齐 | 一次 |

**提交粒度**：**一个视觉主题一个 commit**。色值微调与结构删除**不要混在一个提交里**——反例 `56d0fe3`（本地分支 `feat/ai-basic-ui-prototype`，+21 / −71）：「背景实色兜底」与「去掉小程序 chrome 胶囊」压在一起，回滚无从下手。

**时间花在哪（2026-09-13 实测，多次取数）**：基座 APK（95.7 MB）**只装一次**；**编译 205–261 秒**（冷启 HBuilderX 那次 610 秒）、
**推送 + 启动 15–21 秒**、一轮真运行合计 **236–287 秒**；全量编译 **4.8–8 分钟**。
旧口径的「增量约 1–2 分钟」**只覆盖编译段**，「约 10–11 分钟」是拿一次冷启 + 9 分钟推送的极值当常态 —— **两个都别再引用**，以本段为准。
**慢的从来不是编译，而是这三样（同日一笔账）**：**返工**（编译期诊断拖到真机才发现，两次各约 5 分钟）、
**排队**（等另一会话的 `hx-agent.lock`，一次约 10 分钟）、**卡死**（launch 挂住，白等约 20 分钟才到上限）。
后两样现在都有对策：**忙等待默认 600→120 秒**、**部署停滞 300 秒即提前判环境不可用**（`-DeployStallSeconds`）；
返工的对策是**诊断走 `hx:compile-only` 拦在本地**。
`hx-run.ps1` 会打印分段表与机检行 `HX_RUN mode=… compile=… deploy=… total=… exit=…`，另加部署判据行 `HX_RUN_DEPLOY deployed=… www=… pid_after=…`。

**真运行会话常驻，但下一次 launch 会顶掉旧的（2026-09-13 实测）**：不传 `--compile` 的真运行 `cli.exe` **不会自己收口**
—— 实测一个会话活了 **51 分钟**，期间每约 10 分钟重试一次 `wakeUpDevice`（被设备以 `INJECT_EVENTS` 拒绝，**非致命**）；
它最终是在**下一次 `launch` 发出后约 10 秒**打出 `已停止运行...` 才收口 ⇒ **不需要手工清理**。据此 `hx-run.ps1`
对 launch 步**不等待**：发出去 → 有界轮询设备侧事实 → 判完即返回（要提前停就在 HBuilderX 里点「停止」；脚本绝不 kill）。

### 并发会话纪律（多会话并行时必读，2026-09-14）

**HBuilderX 是单实例串行资源** —— 这把「真机自动化」钉成**不可并发**：多个会话同时要跑 `hx-run` /
编译门 / 截图时，`scripts/lib/hx-busy.ps1` 的互斥锁只会让它们**排队**（日志打 `HX_BUSY wait=<秒> result=free`），
**不会**并行；超时即 `exit 2`（环境不可用）。这是**架构约束，不是可以调优的缺陷**。

**故职责边界这样切**：

- **真机步骤**（`dev:finish` 的步骤 4–6：编译 / 部署 / 截图）**集中到一个会话执行**；
- **其余会话只做到「编译门」为止** —— 即 `npm run dev:finish` 的 🟢 **Q-A 默认路径**（级别判定 + 契约测试 + 静态守护，**秒级、不取锁、不占设备**）。

这样每个会话边界清晰，不会因争抢 HBuilderX 互相阻塞。

**锁交接（会话内连跑多个 HBuilderX 步骤，2026-09-14 加）**：锁**按 PID 判定且不可重入** ⇒
一个会话内父进程持锁后调子脚本会**自死锁**。`dev:finish` 通过 `$env:HX_LOCK_OWNER` + PID 比对把锁
**交接**给子脚本（认到同一持有者就复用、不再加锁）；**fail-safe**：PID 对不上 ⇒ 照常加锁，绝不放开互斥。
守护：`utils/hxBusyGateContract.test.js` H10/H11、`utils/devFinishContract.test.js` F6–F10。
判据与理由见 `docs/adr/0011-一键完成的并发与失败语义.md`。

#### 会话时间预算与「先写代码、后集中上机」（2026-09-15 加）

**并行会话各自只写代码、都不进真机阶段；真机阶段按批集中到一个会话。** 理由：锁只保证**串行**，
不保证**不阻塞** —— 别的会话持锁时你会在 `HX_BUSY wait=<秒>` 上安静排队，最坏等到自己的上限才 `exit 2`。

- **没写完就别进真机阶段**：先把改动在本会话全部写完（含自测用的 `npm run test:unit`），
  再进 `dev:finish` 的步骤 4–6。中途反复进出会让每次进出都在锁上重新排队。
- **同一批改动攒齐再上机一次**（中循环口径：每个「视觉满意点」一次，见上「三层节奏」）。
  ⚠️ 这与「**一个视觉主题一个 commit**」是两件事：**验证可以合批，提交仍按主题拆** ——
  合批降低机器等待，拆提交保住回滚粒度，不要用其中一个去否定另一个。
- **等待上限是分步的、且相差很大**（改之前先看清你调的是哪条路径）：

  | 入口 | 取锁等待上限 | 备注 |
  | --- | --- | --- |
  | `hx-run.ps1`（`hx:run` / `hx:compile-only`） | **120 秒** | 故意短：忙就快速 `exit 2`，不无声等 |
  | `dev:finish`（步骤 4–6 临界区） | **1800 秒** | 临界区长，故上限宽 |
  | `build:kotlin-all` / `build:compile` / `build:mp-weixin-check` | **600 秒** | 中等 |

- **超过上限不是失败，是「环境不可用」**（`exit 2`）：此时改跑不需要 HBuilderX 的检查
  （`npm run test:unit` / `build:kotlin-all -SkipPublish`），**绝不 kill 主程序、绝不抢占**。
- **不要拿「另一个会话空着」当假设**：判据现测，不看记忆 ——
  `Get-Process HBuilderX,cli`、`$env:TEMP\hx-agent.lock`、以及 `.ci-verify\` 里是否有文件**正在被写**。

### 反模式（逐条禁）

- **在真机上验「本该编译期就红」的东西**——类型名义不一致（`ClassCastException`）、uvue 样式规则违反、模板编译错误**都是编译期诊断**：先 `npm run hx:compile-only`（**不碰设备**；成本见下表），别让它变成一次 5 分钟的真机返工。2026-09-13 实测就这样白烧了两次。
- 每次微调都**全量重编译**——全量是门的活（`compile-check.ps1`），日常走 `hx-run.ps1`。
- 每次微调都**重装基座**——基座只装一次；`hx-run.ps1` 里没有任何 `adb install`，装机由 HBuilderX 自己处理。
- 每次微调都**跑全量门 + 提交**——门按中循环跑（每个视觉满意点一次），提交按「一个视觉主题一个 commit」。
- **kill `cli` 或 HBuilderX 主程序**——违反 ADR-0008 的「HBuilderX 是单实例串行资源」坑位：**忙就等**（`scripts/lib/hx-busy.ps1` 的「锁 + 忙探测 + 等待上限」），超时 `exit 2` 并改跑不需要 HBuilderX 的检查（`npm run test:unit` / `build:kotlin-all -SkipPublish` / `smoke:emulator`）；`hx-run.ps1` 内不含任何强杀调用。
- 拿**仿真机当热刷新用**——仿真机是**前置冒烟**（`npm run smoke:emulator`，非门、不替代 ①），装一次 SDK 3–4 GB / 20–40 分钟，不适合秒级迭代。

**HBuilderX 回写坑位同样适用**：跑完任何 HBuilderX 步骤（含 `hx-run.ps1`）先 `git status` 看 `manifest.json` **与 `pages.json`** 是否被改脏，脏了就还原再继续 —— 后者会被写入一段 `condition`（GUI 选的启动页，注释自述「仅开发期间生效」），**属本地开发配置、禁止提交**；而 `pages.json` 同时是「运行时面 / 打包面」判据来源，误提交会让 PR 凭空命中 ④b 云打包门（详见 `docs/adr/0008-移动端验收门与证据.md`）。

**另一条收尾坑位（2026-09-13 一天踩到两次）**：`hx:run` 的**真运行会话会常驻**，并把它自己写的 `.ci-verify\launch-*.out/.err` 攥在手里 ⇒ **只要会话还活着，那个项目目录既删不掉也改不了名**（报 `being used by another process`）。**删 worktree / 给项目目录改名之前**，先 `cli project close --path <项目>` 结束会话（或发起下一次 launch 把它顶掉）。`hx:compile-only` 不留常驻会话，因此**没有这个副作用**。

## 验收门与合并纪律（ADR-0008）

改动触及运行时面（改动集含 `*.uvue` / `*.uts`，或 training-app 下的 `manifest.json` / `pages.json` / `platformConfig.json`）时，适用 `docs/adr/0008-移动端验收门与证据.md` 的四门与证据要求。

- **签收在人、合并不限人**：把 PR 正文的 `## 验收证据` 段填齐（每门四字段 + 产物），其中人工门（**①b**）的「执行人」栏由**人**给出原文（agent 可代录、不得自拟）；填齐后 **agent 可直接合并**，不必停在「待人工签收」。
- 非运行时面的 PR（纯文档 / 测试 / CI 配置）不受此限，agent 可自行合并。
- **人工门只剩 ①b Android 真机「能力面」冒烟**（2026-09-12 修订：② 已移出人工门清单，见下条；**2026-09-16 修订**：① 拆成 ①a／①b，只有 ①b 由人签）；①b 的「执行人」栏由**人**给出原文（agent 代录，不得自拟、不得写「已通过」）。
- **① 的触发面与取证节奏（2026-09-16 修订，见 `docs/adr/0016-真机门的人工性收缩与按批取证.md`）**：①a（agent 出证：逐页截图 + logcat + 机检行 + 入仓截图）**按「一次分支收口」跑一次** —— 不是 PR 内的每次微调，也**不跨 PR 共用**（一次真机运行只反映一棵树；跨 PR 共用必须放宽证据绑定判据，方向与 2026-09-12 那次收紧相反）。①b（人签）**只在命中「能力面」时必过**：指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互 —— **不是**「任何运行时面」。① 行的「执行人」栏在该行只覆盖 ①a 时允许写「agent 执行」。能力面的可机检口径 = **路径白名单 + PR 模板自报勾选**（**自报是声明、不是判据**；漏报靠 ①a 的 logcat、评审与闭环条款兜 —— 合并前发现即补做，合并后发现走事后验证回执）。**明确不做**：基于 diff 文本 / API 字符串的触发判据（#1030 的教训）、跨 PR 共用真机、「冷态降级 ④」。
- **①a 真机取证的具体手法**（`--pagePath` + `--pageQuery` 深链、点击坐标取 `uiautomator dump` 的 bounds、`input` 可注入性**每次现测**、造取证夹具的「人登录一次 + CDP 驱动 + 页面内 fetch」做法）见 `docs/adr/0008-移动端验收门与证据.md` 的「①a 取证手法补遗（2026-09-15 实测）」。**多会话并发时如何不动别人的工作树就提交/同步 master** 见 `docs/agents/multi-agent-git.md` 的「游离提交：完整配方」。
- **② 微信开发者工具门 = 半自动门（2026-09-12，#883 收口）**：spike #883 实测全链路**完全无人值守**（HBuilderX `publish mp-weixin` 构建并自己拉起开发者工具 → `cli.bat close` 清残留 → `cli.bat auto --auto-port` 开自动化端口 → `miniprogram-automator` 读 pageStack / console / exceptions + 截图；≈4.5 分钟/次）。agent 可执行 `npm run build:mp-weixin-check`（= `scripts/mp-weixin-check.ps1`）产出 `MP_WEIXIN_RESULT` + `.ci-verify/mp-weixin.log` + `.ci-verify/*.png`，**结果可由脚本 `-PostToPr <PR号>` 贴成 sha 绑定的 PR 评论承载**（正文该行写「见评论 <链接>」即可）。**「执行人」栏仍由人给出原文（agent 代录，不得自拟、不得写「已通过」）。**
  - **运行前提（硬前提）**：CLI 靠与 HBuilderX 主程序的本地 IPC ⇒ **须以全访问权限执行**（同 ④a）；微信开发者工具**需处于已登录状态**（`cli.bat islogin` → `{"login":true}`），登录态过期时由人补扫一次码（agent 不得索取/代填任何凭证）；项目目录须用**唯一名**（HBuilderX 按项目名解析，同名目录会被误命中）。
  - **HBuilderX 是单实例串行资源（2026-09-12 追加）**：`cli.exe` 只驱动同一个主程序，重活排进主程序的编译队列 ⇒ **维护者用 GUI 编译/运行时，agent 的门脚本并发发起会既拖慢维护者、又因排队产生假失败**（实测 publish 停在「正在编译中...」不返回）。四个门脚本（② / ④a / ④c 的 publish 段）统一 dot-source **`scripts/lib/hx-busy.ps1`**：agent 互斥锁（`$env:TEMP\hx-agent.lock`，>30 分钟视为陈旧可抢占）+ 主程序忙探测（`cli project list` 带 5 秒硬超时）+ 等待上限（`-HxWaitSeconds`，默认 600；`-HxNoWait` 立即判忙）；**忙/超时 ⇒ `exit 2`（环境不可用），绝不 kill 主程序、绝不抢占项目**。**限制写实**：机械上无法可靠探测「维护者 GUI 是否正在编译」——本机制是「锁 + 探测 + 上限」的 fail-safe，**GUI 优先**。日志/评论里会打 `HX_BUSY wait=<秒> result=free|timeout`。守护：`utils/hxBusyGateContract.test.js`（H1–H9）。
  - **已知坑位（2026-09-12 实测，属所有 HBuilderX 门的共性）**：**HBuilderX 在首次导入 / 编译项目时会回写工作树的 `manifest.json`，把 `mp-weixin.appid` 置为 `null`**，产物随之落成 `touristappid` ⇒ ② 脚本已把「源 manifest appid」与「产物 appid」都做成 fail-closed 前置断言（不匹配即判门不过，不拿游客 appid 出个绿）。**更广的影响：任何 HBuilderX 门（④a / ④c / ②）在共享工作树里跑都可能静默改脏 `manifest.json`——而它是「运行时面」文件、正是 `pr-evidence` 的判据来源** ⇒ **跑完 HBuilderX 门后先 `git status` 看 `manifest.json` 是否被改脏**，脏了就 `git checkout -- manifest.json` 再继续。
  - **证据边界（非等价声明）**：② ≠ ① 真机门，也 ≠ ④b 云打包门；`content://` 上传、生物识别、第三方 SDK 回调、真机性能/ANR 仍只能靠 ① 与发版前全量逐页冒烟兜底。
  - **端口风险**：`cli.bat auto --auto-port` 实测绑通配地址（`::`）⇒ 自动化端口在局域网内可访问；共享网络上跑本门前先确认防火墙（脚本头部已声明该风险，未做加固）。
- **低风险运行时面（仅 `.uts` 逻辑改动，无 `.uvue` / 三份 json / `uni_modules`）免 ① ②**：正文里这两行可以整行不写（写了写「免（低风险运行时面：仅 .uts 逻辑改动）」也行）；代价是真机/渲染类问题推迟到发版前的 ① 全量冒烟兜底。③ 与 ④ 仍必过。
- **③ `npm run test:unit` 门 = 有判据的门（2026-09-18 立，#1156）**：③ 此前只有一句「全绿」，**没有判据** —— 而已实测证伪：「全绿」**可以恒真**（把 `utils/format.uts` 的 `return '0s'` 改坏 ⇒ `format.test.js` 照样 8 passed；把 `utils/checkinCalendar.uts` 改坏 ⇒ 照样绿；两者都是手抄的镜像实现）。**③ 的判据是三条**：① **「我故意弄坏被测物，它会不会红？」**（判别力 —— 答不上就不算验证层）② 「它测的是该测的那一支吗？」（条件编译 / 镜像 / 分支选错 ⇒ 跑起来了但测的是另一支）③ 「**只跑通过的那一次，不算验收**」（成对取证：必不红 / 必红各一条）。判据与守护分类（**行为守护**承重 / **接线守护**不构成 ③ 证据）见 [`docs/agents/guards.md`](docs/agents/guards.md)，真源是分类器 `node scripts/classify-guards.mjs`（机检形状由 `utils/guardClassification.test.js` 的 H1–H5 钉住）。**新增守护**时在 PR 正文回答 `guards.md` 末节那三问。本条只加判据，**不加人工门、不改 ①②④ 的触发面**。
- **④ 本地编译门（2026-09-11 修订：④a 与 ④c 合并，取代原「④a HBuilderX 全量编译」条款，裁定见 #870 / #859）**：**默认载体是 ④c 整模块编译**（`npm run build:kotlin-all` → `.ci-verify/kotlin-all.log`）；**dev 专属面追加 ④a**（`npm run build:compile` → `.ci-verify/build.log`）——命中 `pages.json` / `manifest.json` / `platformConfig.json` 改动、新增页面、模块手术收口 PR 时。agent 可执行这两条，**「执行人」栏填执行会话所用账号并注明「agent 执行」**；**结果可由脚本 `-PostToPr <PR号>` 贴成 sha 绑定的 PR 评论承载**（正文该行写「见评论 <链接>」即可，校验器只认结构与 sha 绑定，不校真伪）。**运行前提**：CLI 靠与 HBuilderX 主程序的本地 IPC，受限沙箱下报「与主程序的连接已中断」⇒ 跑 ④a **必须以全访问权限执行**。
- **④b release 云打包按「打包面」触发（2026-09-11 修订）**：改动 `manifest.json` / `pages.json` / `platformConfig.json`、`uni_modules/**/utssdk/app-android/**`、新增 `*.aar` / `libs/*.jar` 时必填；**正式发版前必须跑一次云打包并装机自测**（发布前置条款）。旧口径「新增 async / 新增 composable 即触发」已废止（那类问题本地编译门就能抓，实测见 ADR-0008）。
- 例外通道：正文写明「已接受未验证风险 + 理由 + 事后验证计划」，检查会打警告放行，但**仍必须由人执行合并**。禁止静默例外。
- `pr-evidence` 检查只校证据结构、不校真伪；它是**可见检查**而非 ruleset 必检——本仓**有可用的 admin 通道**（维护者持有仓库所有者账号），但**裁定不装**（逐 PR 审批成本高于约束收益，见 ADR-0008「为何不装『必检 + approve』」）。
## 发布流程（push / PR / merge）

master 有仓库 ruleset「protect master」保护（直接 push 会被拒，`push declined due to repository rule violations`），且限定 squash 合并。发布必须走分支 + PR：

1. **本地提交**（只 add 本次改动的文件，勿 `git add -A`）。
2. **建分支推送**：若提交已在本地 master 上，`git branch feat/xxx` 后 `git reset --hard origin/master` 还原本地 master；然后 `git push -u origin feat/xxx`。
3. **分支 push = CI + testing 冒烟**：push 触发 `ci.yml` 全量 CI（不再同时跑 `pull_request`，一次改动只跑一遍）。CI 全绿**且该分支已有开启的 PR** 时，`ci-summary` 用 `workflow_dispatch --ref <分支>` 派发 `cd.yml` 部署 testing（起栈 → 健康检查 → 立即 down）。孤儿分支/无 PR 分支不占 testing。
4. **创建 PR**：`gh pr create --base master --head feat/xxx --title "..." --body "..."`。**PR 事件不触发任何 CI/CD**；PR 页上的 `ci-summary` 等检查来自第 3 步分支 push 的同 commit run。若 CI 在开 PR 之前就已经跑完（纯文档改动约 20 秒），该 commit 会缺冒烟记录，`testing-smoke.yml` 会在 PR 开启时补发一次（仅此一种条件下动作，不重跑 CI）。
5. **等门禁**：`gh run watch <id> --exit-status` 等 CI 全绿（纯前端改动时 backend-* / migration-check 跳过属正常，`ci-summary` 仍会 success）。ruleset 把 `ci-summary` 设为必检并要求分支 up-to-date —— 未跑绿、或 master 已前进时不能合并（后者要 `git merge origin/master` 同步后重推、重跑 CI）。
6. **Squash merge → 直发 production**：`gh pr merge <n> --squash --delete-branch`。master 的 push **不跑 CI**，直接触发 `cd.yml` 的 `gate` job：从 commit 主题解析 `(#N)` → 校验该 PR head 的 `ci-summary=success`、该 commit 的 testing 冒烟 `success`（冒烟可能晚于合并，gate 最多轮询 15 分钟）→ 通过后才构建镜像并部署 production。若报 "requirements have not been met"，用 `gh pr view <n> --json statusCheckRollup` 排查。
7. **收尾**：`git fetch --prune` → `git checkout master && git pull --ff-only` → 删除本地 feat 分支（若 gh 已自动删）。

**应急通道**：gate 阻断但确认可以上生产时，手动放行 `gh workflow run cd.yml -f environment=production -f ref=<master sha>`（dispatch 不经门禁）。若是 testing 冒烟失败，先到 Actions 重跑该 CD run，再重新合并或走应急通道。

> ⚠️ 本次流水线变更**合并前**切出的分支：其 `ci-summary` 受旧条件约束（仅 master 上报），在 PR 上显示 skipped，既不满足必检也会被 gate 判为 `other`。先 `git merge origin/master` 重推、等 CI 重跑，再走合并流程。

> ⚠️ **不要用 `timeout N` 包裹 git/gh 的写操作**（merge / push / rebase / checkout）。被 SIGTERM 杀掉的是**执行到一半**的操作，比失败更糟：曾因 `timeout 180 gh pr merge --squash --delete-branch` 被中断，残留 `.git/index.lock` 且分支清理删了一半，`frontend/src` 下 265 个文件被删。这类操作一律用后台任务跑并等其自然结束。

## 注意事项

每次改动完成后，都必须创建一个对应的git commit，以便后续追踪和回滚。每次改动后，都必须编写或更新相关测试，并在交互给用户前，确保所有测试和验证全部通过

---

# 叉车维修培训学员端 - 移动端开发约定

> 本文件为移动端（uni-app-x 跨端应用）的 AI agent 工作约定，覆盖开发、测试、构建、发布全流程。
> 根目录 `D:\FL\AGENTS.md` 提供系统级全局约定，本文件补充移动端特有规范。（主树为 `D:\FL`；位置与搬迁记录见 `docs/agents/multi-agent-git.md` 开头的「主树位置」段。原备胎盘 `E:` 已废弃不存在 —— **别去找它、别照抄任何 `E:\FL` 路径**。）

## 项目概述

- **技术栈**：uni-app-x + Vue 3 + TypeScript
- **目标平台**：Android、iOS、微信小程序、支付宝小程序
- **代码风格**：Composition API + `<script setup>` 语法

## 开发约定

### 文件结构
```
src/
├── pages/           # 页面组件（按功能模块分组）
├── components/      # 公共组件
├── composables/     # 组合式函数
├── stores/          # 状态管理（Pinia）
├── api/             # API 接口封装
├── utils/           # 工具函数
├── types/           # TypeScript 类型定义
├── constants/       # 常量定义
├── static/          # 静态资源
└── assets/          # 样式资源
```

### 代码规范
- **命名**：文件名使用 kebab-case，组件名使用 PascalCase
- **导入顺序**：@vue/* → uni-app API → 第三方库 → 本地模块
- **类型安全**：所有 props、emit、状态必须有 TypeScript 类型
- **组件通信**：优先使用 props/emit，跨层用 provide/inject 或 Pinia

### 平台兼容性
- **条件编译**：使用 `#ifdef` / `#ifndef` 处理平台差异
- **API 选择**：优先使用 uni-app API，原生 API 用条件编译包装
- **样式适配**：使用 rpx 单位，避免固定像素值
- **触摸交互**：确保触摸区域不小于 44px × 44px

## 测试约定

### 单元测试与守护（本仓唯一在跑的验证层）
- 入口：`npm run test:unit`（= `jest.config.unit.js`，`testEnvironment: node`，只扫 `utils/**/*.test.[jt]s?(x)`）；`npm test` 是它的等价别名
- 它就是 **③ 门**（CI 的 `mobile-test` job，被 `ci-summary` 断言必过）。判据三条 —— ①「我故意弄坏被测物，它会不会红？」②「它测的是该测的那一支吗？」③「只跑通过的那一次，不算验收」—— 见 [`docs/agents/guards.md`](docs/agents/guards.md)，守护分类真源是 `node scripts/classify-guards.mjs`
- **新增守护**时在 PR 正文回答 `guards.md` 末节那三问；**接线守护不构成 ③ 证据**

### 端到端（E2E）自动化测试：本仓**不做**
- **已评估并否决，不要再按 uni-app 项目模板去接**：微信小程序通道只能做 page 级断言、不能做「点击 → 跳转 → 断言」的流程验证（PR #1136 裁决）；H5 通道在 `require` 阶段即炸；元素级与导航级 API 在本机组合下均不可用（ADR-0008 的 2026-09-18 实测）
- 裁决、实测与**将来要重启的配方**见 [`docs/adr/0020-端到端测试通道裁决落锁与装配清退.md`](docs/adr/0020-端到端测试通道裁决落锁与装配清退.md)；通道裁决的原始出处是 [`docs/spec-永绿整改.md`](docs/spec-永绿整改.md) §②
- 因此**没有** `jest.config.js` / `env.js` / `pages/**/*.test.js`，也**没有** `test:h5` / `test:android` / `test:ios` / `test:mp-weixin` 这些入口；`@dcloudio/uni-automator` 已移出依赖（ADR-0020）
- 想补强验证层的正确方向是**把镜像实现迁到 `utils/utsHarness.js` 真执行**（`spec-永绿整改.md` §④ 的 S3 / S6a–S6c），不是引入 E2E

## 构建与发布

### 开发环境（使用 HBuilderX）
```bash
# 启动 H5 开发服务器
# 在 HBuilderX 中运行：运行 → 运行到浏览器 → Chrome

# 启动微信小程序开发
# 在 HBuilderX 中运行：运行 → 运行到小程序模拟器 → 微信开发者工具

# 启动 Android 开发
# 在 HBuilderX 中运行：运行 → 运行到手机或模拟器 → Android
```

### 生产构建（使用 HBuilderX）
```bash
# 构建 H5
# 在 HBuilderX 中发行：发行 → H5-手机版

# 构建微信小程序
# 在 HBuilderX 中发行：发行 → 小程序-微信

# 构建 Android APK
# 在 HBuilderX 中发行：发行 → 原生App-云打包
```

### 发布流程
1. **代码审查**：所有改动必须通过 PR 审查
2. **测试验证**：`npm run test:unit` 全绿（E2E 通道本仓不做，见 ADR-0020）
3. **构建验证**：生产构建无错误
4. **平台审核**：微信小程序提交审核，Android/iOS 打包测试
5. **灰度发布**：先小范围验证，再全量发布

## 常见问题

### 跨端兼容性问题
- **问题**：某些 API 在特定平台不可用
- **解决**：使用条件编译 + 平台检测 + 降级方案

### 性能优化
- **列表渲染**：使用 `v-for` 加 `key`，避免 `index` 作为 key
- **图片优化**：使用懒加载，压缩图片大小
- **内存管理**：及时销毁定时器、事件监听

### 调试技巧
- **H5**：使用浏览器开发者工具
- **小程序**：使用微信开发者工具
- **Android**：使用 Chrome DevTools 远程调试

## 注意事项

1. **每次改动完成后，都必须创建一个对应的 git commit，以便后续追踪和回滚。**
2. **每次改动后，都必须编写或更新相关测试，并在交互给用户前，确保所有测试和验证全部通过。**
3. **提交前必须运行**：`npm run test:unit`（单元测试 + 守护）。注意本仓**没有** `type-check` 脚本——那是 Web 前端栈（`frontend/`）的入口，别照搬
4. **避免使用**：`setTimeout`/`setInterval` 等可能造成内存泄漏的 API，优先使用 uni-app 生命周期管理

## 相关文档

- **ADRs（移动端独立编号，现至 `0025`）**：`docs/adr/` —— 关键几条：`0008` 验收门与证据（四门判据）、`0016` 真机门的触发面与取证节奏、`0019` 契约测试读取层归一、`0020` 端到端通道裁决与装配清退、`0024` 技能供给与管线归属（死副本清退 + 四条约定）、`0025` 论坛正文格式与输入区形态（声明子集第三档 `SUBSET_FORUM` + 渲染接入 + 输入区形态）；与根仓库 `docs/adr/`（`ADR-0001`+ 编号）互不相关，引用时注意区分
- **Git 工作流**：`docs/GIT_WORKFLOW.md`
- **UI 规范**：`docs/ui-spec.md`
- **技术规范**：`docs/technical-spec.md`
- **产品设计**：`docs/product-design.md`
