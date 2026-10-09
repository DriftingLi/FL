# 叉车维修培训学员端 — 移动端 UI 规范

> 适用范围：`training-app/叉车维修培训学员端跨端应用` 全部 `.uvue` 页面（**口径 133 份**：排除 `node_modules/` / `uni_modules/` / `unpackage/`，#1597 §7 实测；引用本数时必带口径）
> 输入来源（2026-10-10 升级，map #1591）：**无外部设计稿**——此前「基于 Figma 设计稿 2026-09-01 版本」经 #1596 证伪：全仓无 `.fig`/`.sketch`/`.xd`，`2026-09-01` 是本文档首次提交（0c2ff1ce）的入库日而非设计稿导出日；现存「设计输入」仅两页粘贴截图的文字化残影（见 §12 遗留）。本规范的形态真源 = 本文 + `uni.scss` + #1595 六十页台账。
> 机检总开关：`<style>` 必须 `lang="scss"`（§1 校正块）；五硬指标见 §9。

---

## 1. 色彩体系

> ⚠️ **本表的变量名以 `uni.scss` 实际定义为准**（2026-09-15 校正，#979 实测）：
> 此前表里写的 `$text-secondary` / `$text-placeholder` / `$text-inverse` / `$bg-page` **在 `uni.scss` 里都不存在**
> （真实名是 `$text-color-secondary` / `$text-color-placeholder` / `$text-color-inverse` / `$bg-color`）；
> 照旧表写会**静默失效**（`<style>` 未声明 `lang="scss"` 时变量不预处理、声明被直接丢弃，页面无报错、只是颜色没生效）。
> 机检：`utils/searchContract.test.js`「引用的 `$变量` 在 `uni.scss` 里都存在」。
>
> ⚠️ **但「变量已定义」不等于「页面能用」**：`.uvue` 的 `<style>` **不写 `lang="scss"` 时整段声明被丢弃**，
> 此时连字面色值也不生效。2026-10-10 实测：60 个页面里**只有 13 个**声明了 `lang="scss"`，其余 47 页根本没资格用 token。
> ⇒ **本规范的总开关是 ADR-0035 的 T2 票（补全 `lang="scss"` 声明），不补齐则后面所有 token 收敛都是空转。**

### 1.1 主色（2026-10-10 裁定：#2979ff → #239EDD）

| Token | 色值 | 用途 |
|---|---|---|
| `$primary-color` | **`#239EDD`** | 按钮、选中态、Tab 高亮、链接、**主色 header / 状态栏**（N01 蓝底套） |
| `$primary-color-light` | `#5b9aff` | 渐变底部、hover 态（随主色轮换一并复评） |
| `$primary-color-dark` | `#1c9eff` | 按钮 pressed 态（同上） |
| `$primary-tint` | `#ebf5ff` | 主色浅底（分区标题带、选中标签底、历史 chip 底） |

> **换色执行口径**：`uni.scss` 只改两处——`:18` `$uni-color-primary` 与 `:81` `$primary-color`，单点生效，**禁止逐页改字面色值**；
> 页面上残留的 `#2979ff` 字面值（#1595 实测 213 处/44 页）由逐页替换执行票收敛。主色字面值**不进**机检白名单（§9）。

### 1.2 页面背景与渐变

> ⚠️ **App 平台（uvue 原生端）的 `linear-gradient` 只接受「恰好 2 个颜色值 + 不带百分比停靠位 + `to` 关键字方向」**
> （2026-09-13 真机实测，见 **#937** / ADR-0010；守护 `utils/gradientSyntaxContract.test.js`）。
> 三色写法与 `deg` 角度都会被原生端**整条丢弃**（`deg` 的失效形态是**整幅退化成两端色的中点**，不是空白）。
> SCSS 变量在 `lang="scss"` 声明下**可以**用于渐变色值（编译期展开为字面值，非运行时 CSS 变量）——
> ⚠️ **2026-10-10 纠错（#1597 C5）**：旧版此处及 §6.1 注释里「`.uvue` 不支持 CSS 变量 ⇒ 渐变只能逐处写字面色值」是**错误结论**
>（把 SCSS `$变量` 误判成了 CSS `var(--x)`），正是它造成 12 个 token 零命中。渐变一律写 token。

| Token | 色值 | 用途 |
|---|---|---|
| `$gradient-start` | `#CFE9FB` | 渐变顶部（tabBar 页面专用） |
| `$gradient-end` | `#F5F5F5` | 渐变底部（**2026-10-10 裁定 #1597 C3**：tabBar 渐变方向改 `to bottom`，终点采 `$gradient-end`；`#D0EBFD` 退役） |
| `$bg-color` | `#F8F8F8` | 通用页面背景 |
| `$bg-tint` | `#F1F6FF` | 淡蓝画布（列表/结果页：白卡浮于其上，避免整页纯白） |
| `$bg-header` | `#ffffff` | **白底页头套底色（2026-10-10 新增，N02/N03 套）**——登记进 `uni.scss` 与本节 |

### 1.3 语义色与状态浅底

| Token | 色值 | 用途 |
|---|---|---|
| `$success-color` | `#4cd964` | 成功状态 |
| `$warning-color` | `#ff9900` | 警告状态 |
| `$danger-color` | `#ff3b30` | 危险操作（退出登录、删除） |
| `$success-tint` | `#e8f5e9` | **新增**：成功浅底（存量 17 处） |
| `$warning-tint` | `#fff3e0` | **新增**：警告浅底（存量 8 处） |
| `$danger-tint` | `#ffebee` | **新增**：危险浅底（存量 7 处，`#fff0f0` 并入） |

> `$error-color`（`uni.scss:86`）与 `$danger-color`（`:137`）同语义位，**删 `$error-color` 定义留注释占位**（#1597 Z12）。

### 1.4 文字色

| Token | 色值 | 用途 |
|---|---|---|
| `$text-color` | `#333333` | 主文字（标题、正文） |
| `$text-color-secondary` | `#666666` | 次要文字（描述、副标题、结果片段） |
| `$text-color-placeholder` | `#999999` | 辅助文字（日期、placeholder） |
| `$text-disabled` | `#cccccc` | 禁用 / 弱化文字 |
| `$text-color-inverse` | `#ffffff` | 反色文字（深色背景上） |

### 1.5 表面与边框

| Token | 色值 | 用途 |
|---|---|---|
| `$card-bg` | `#ffffff` | 卡片背景 |
| `$border-color` | `#e5e5e5` | 分隔线、卡片边框（**内容区分隔线的唯一值**，§7 共同格） |
| `$border-color-light` | `#f0f0f0` | 更浅的分隔线（列表行内分隔；**#1596 残留裁定：`#EEEEEE` 不采，非 token**） |
| `$input-border` | `#e5e5e5` | 输入框边框 |

### 1.6 长尾色值白名单（显式内容色）

存量基线 2142 处声明位/171 种色值（#1593 口径）：裁定 1–6 收敛约 1440 处，余约 700 处属**低频内容色**（130 种、每种 ≤4 次）——
**逐值一行理由入册**（写在机检白名单文件里，注明「哪个文件为什么留」），**不按次数自动豁免**。Material 近似族（如 `#f44336`→`$danger-color`，约 135 处）收敛进既有 token，不进白名单。

---

## 2. 间距系统（8rpx 倍数）

| Token | 值 | 用途 |
|---|---|---|
| `$spacing-xs` | `8rpx` | 极小间距（图标与文字间隙） |
| `$spacing-sm` | `16rpx` | 小间距（标签间距、列表项间距） |
| `$spacing-md` | `24rpx` | 标准间距（页面边距、卡片内边距、卡片间距） |
| `$spacing-lg` | `32rpx` | 大间距（区块间距、列表水平内边距） |
| `$spacing-xl` | `48rpx` | 超大间距（底部留白） |

---

## 3. 圆角系统

| Token | 值 | 用途 |
|---|---|---|
| `$radius-sm` | `8rpx` | Tag 标签、小 badge |
| `$radius-md` | `16rpx` | 卡片、图标容器 |
| `$radius-lg` | `24rpx` | 次按钮描边、大卡片（**#1597 C2：24rpx 为准**，旧文「32rpx」系写错 token——次按钮统一 `$radius-lg`） |
| `$radius-xl` | `32rpx` | 大面板、对话框 |
| `$radius-pill` | `999rpx` | 胶囊按钮（主按钮、筛选按钮；**#1597 C2：999rpx 胶囊是对的**） |

---

## 4. 字号系统

| Token | 值 | 用途 |
|---|---|---|
| `$font-size-xs` | `20rpx` | 极小标注（角标、极小提示） |
| `$font-size-sm` | `24rpx` | 辅助文字（标签、日期、meta 信息） |
| `$font-size-md` | `28rpx` | 正文（列表项文字、输入框文字） |
| `$font-size-lg` | `32rpx` | 卡片标题、小标题 |
| `$font-size-xl` | `36rpx` | 页面标题（居中大标题） |

> `$font-size-xxl`（`uni.scss:128`，44rpx）**全站零消费** ⇒ **删定义留注释占位**（#1597 Z7/C4，执行票承接）。

---

## 5. 组件规范

### 5.1 导航栏（appNavBar）

> **2026-10-10（ADR-0035）口径**：全仓**只有一份**页头件，**禁止 fork**；差异一律用 prop 表达。
> 现状的 `components/ai-chat/ai-chat-nav.uvue`（归一为 app-nav-bar 配置 + 可选 `subtitle` prop）与页面各自手写的 nav-bar
> 都要在 ADR-0035 的 T3/T4 收口到本件。`app-nav-bar.uvue:37` 缺 `lang="scss"`（`:43/:61/:63/:69` 引用 token 全部静默失效）
> ⇒ **补声明是骨架层首票第一步**（#1597 K2，批 0 承接）。
> 本件负责**状态栏占位与胶囊让位**（`navigationStyle: "custom"` 下避让责任 100% 在项目代码，见 ADR-0035 现状锚点表）。

- 高度：`88rpx`（**不含状态栏**）
- 状态栏占位：由本件内部按 `useSafeArea()` 取值渲染，**页面不再自带 `<view class="status-bar">`**
- 右侧按钮区：右边界**不得越过微信胶囊左边界**（`getMenuButtonBoundingClientRect().left` − 安全间距），也不得写死魔数
- 标题：居中，`$font-size-xl`（36rpx），font-weight bold
- 左侧返回按钮：`‹` 字符，`44rpx`
- 右侧图标：`44rpx`
- 背景：随页头套走（§7：N01 `$primary-color` / N02–N03 `$bg-header` / N05 `transparent` 页面自垫 / N06 白名单无页头）

**胶囊让位算法**（#1592）：测量层 `utils/system.uts` 扩 `useSafeArea()` 返回 `{ statusBarHeight, capsuleLeft, rightSlotWidth }` Ref；
小程序 `uni.getMenuButtonBoundingClientRect()` + px→rpx = `v_px × 750 / windowWidth`；App/H5 兜底 `useStatusBarHeight(44)`、H5 状态栏=0、右槽不设让位约束。
右槽单行定宽、右缘=胶囊左边线；放不下 = 页面把次要动作移出页头（**不隐藏、不换行、不压缩、不写死 80rpx 魔数**）。
页头 `paddingTop = statusBarHeight + 88rpx`。

### 5.2 卡片（appCard）

- 背景：`$card-bg`（#ffffff）
- 圆角：`$radius-md`（16rpx）
- 内边距：`$spacing-md`（24rpx）
- 外边距：`$spacing-md`（24rpx）
- 无阴影（卡片无明显阴影）

### 5.3 主按钮（appButton type=primary）

- 背景：`linear-gradient(to bottom, $primary-color, $primary-color-light)` 或纯色 `$primary-color`（**方向只能写 `to` 关键字，禁 `deg`** —— ADR-0010）
- 文字：`$text-color-inverse`，`$font-size-lg`，font-weight bold
- 圆角：`$radius-pill`（999rpx 胶囊形）
- 高度：`80rpx`
- 禁用态：opacity 0.5

### 5.4 次按钮（appButton type=secondary）

- 背景：透明
- 边框：2rpx solid `$primary-color`
- 文字：`$primary-color`，`$font-size-md`
- 圆角：`$radius-lg`（**24rpx**，§3 C2 裁定）

### 5.5 筛选标签（appChip）

- 圆角：`$radius-pill`（胶囊）
- 未选中：`$bg-color` 浅灰底 + `$text-color-secondary` 文字
- 选中：`$primary-color` **实底** + `$text-color-inverse` 反白文字 + font-weight 600
- 内边距：`16rpx 32rpx`
- 字号：`$font-size-sm`（24rpx）

> 2026-09-15（#979）改版：原「白底描边 + 浅蓝底描边」两态在真机上就是一片白，选中态几乎看不出；
> 改为「浅灰底 / 主色实底」后，色彩只落在**当前档位**上。

### 5.6 Tab 栏（appTabs）

- underline 风格：文字下方 4rpx 蓝色指示条
- pill 风格：胶囊标签（同 appChip 选中态）
- 字号：`$font-size-md`（28rpx）
- 未选中：`$text-color-secondary`
- 选中：`$primary-color` + font-weight 600

### 5.7 列表项（appListItem）

- 高度：`~96rpx`
- 左侧：标签文字 `$font-size-md` `$text-color`
- 右侧：值文字 `$font-size-sm` `$text-color-placeholder` + 箭头 `›`（**2026-10-10 改**：原 `$text-placeholder` 在 `uni.scss` 不存在，静默失效）
- 分隔线：底部 1rpx `$border-color`
- 危险项：文字 `$danger-color`

### 5.8 空状态（appEmptyState）

- 居中显示
- 图标：emoji `80rpx`
- 文字：`$font-size-md` `$text-color-placeholder`（**同 5.7 改**）
- 操作按钮：可选，样式同 appButton

### 5.9 角标（appBadge）

- 背景：`$danger-color`
- 文字：`$text-color-inverse`，`$font-size-xs`（**同 5.7 改**：原 `$text-inverse` 不存在）
- 最小宽度：`32rpx`，高度 `32rpx`
- 圆角：`$radius-pill`

### 5.10 伪类禁令

`.uvue` 原生端**不支持 CSS 伪类**（§9 机检口径）。唯一存量 `pages/courses/course-detail.uvue:469` `.action-btn:active`
改 `@touchstart`/`@touchend` 切 `pressed` class（#1593 裁定，执行票承接）。

---

## 6. 页面结构模板

### 6.1 tabBar 页面（带渐变背景）

```html
<template>
  <view class="container" :style="{ height: windowHeight + 'px' }">
    <appNavBar title="页面标题" />
    <scroll-view class="content-scroll" scroll-y>
      <!-- 内容 -->
    </scroll-view>
  </view>
</template>

<style lang="scss">
.container {
  flex: 1;
  /* App 平台只支持 2 个颜色值、无百分比停靠、方向只写 to 关键字（见 §1.2 注）；
     渐变色值走 token（SCSS 编译期展开），deg 与 CSS var(--x) 均不可用 */
  background: linear-gradient(to bottom, $gradient-start, $gradient-end);
  background-color: $gradient-end;
  flex-direction: column;
}
.content-scroll { flex: 1; }
</style>
```

> ⚠️ `<style>` **必须**写 `lang="scss"`：不写时变量不预处理、整段声明被直接丢弃且**无报错**（§1 顶部校正块）。
> 状态栏占位已内化进 `appNavBar`（ADR-0035），**页面不再自带 `.status-bar`**；`.status-bar` 只存在于页头件内部且**底色随套走**（§7）。

### 6.2 普通页面（白色/灰色背景）

```html
<template>
  <view class="container" :style="{ height: windowHeight + 'px' }">
    <appNavBar title="页面标题" />
    <scroll-view class="content-scroll" scroll-y>
      <!-- 内容 -->
    </scroll-view>
  </view>
</template>

<style lang="scss">
.container {
  flex: 1;
  background-color: $bg-color;
  flex-direction: column;
}
.content-scroll { flex: 1; }
</style>
```

---

## 7. 页头层（N 套）与骨架层（G 套）

> **双层套**（#1592 决议）：页头层 N 套扛**强机检**（唯一页头件 `app-nav-bar`，差异用 prop 表达，fork 即违规 ADR-0035）；
> 骨架层 G 套只锁**成员清单 + 内容区 token 弱约束**，不锁布局。
> N 轴归一化自 24 个页头签名簇（S01–S24，#1595），现实目标 = 20 簇归一 4 套 + 36 页 props 差异。

### 7.1 页头层 N 套

| 套 | 定义 | 底色 | 成员数（60 页口径，my-forum 删除后 59） |
|---|---|---|---|
| N01 | 蓝底页头 | `$primary-color`（状态栏同色） | 19（S01 全体，my-forum 移出后） |
| N02 | 白底页头 | `$bg-header`（#ffffff） | 10 |
| N03 | 白底带分隔线 | `$bg-header` + border-bottom 1rpx `$border-color-light` | 17 |
| N04 | —— | **已并入 N03**（编号留洞不重排） | — |
| N05 | 透明页头 | `transparent`，页面自垫背景 | 7 |
| N06 | 无页头白名单 | 页面自绘（含自垫状态栏、走 token） | 6：`index` / `login` / `register` / `forgot-password` / `profile-setup` / `search` |

`.status-bar` 底色三派随套走：N01=`$primary-color`，N02/N03=`$bg-header`，N05=`transparent`，N06 页面自垫走 token。
`register` / `forgot-password` 维持系统导航不动作（页面内自设 `navigationStyle` 无 API 支撑，`pages.json` 禁改——ADR-0035 问 9）。

### 7.2 骨架层 G 套（内容区成员清单 + token 弱约束）

| 套 | 成员数 | 代表页 |
|---|---|---|
| G1 | 4 | ai-feature、mock-exam、forum-detail、practice-do |
| G2 | 5 | tabBar 五页 |
| G3 | 21 | 通用列表页（my-forum 移出后） |
| G4 | 8 | 表单页族 |
| G5 | 11 | 设置/详情族 |
| G6 | 5 | —— |
| G7 | 3 | —— |
| G8 | 1 | 日历页 `check-in`（§6.8 形态：日历网格 + 打卡态，成员唯一） |
| G9 | 1 | 组合壳 `practice`（§6.9 形态：多区组合壳，成员唯一） |

**共同格**（所有 N×G 组合）：页头件 88rpx + padding 统一 `0 $spacing-md`；标题 `$font-size-xl` bold；
左 `‹` / 右图标 44rpx；内容区卡片 `$radius-md` + `$spacing-md`、无阴影；分隔线 `$border-color`。

### 7.3 N×G 全表真源与逐页验收

**全表（59 页 × N×G 归属）真源 = #1595 六十页台账**（GitHub issue #1595 长评论，含 Δ1–Δ13 差条目代号），本文不复制（防两处漂移）。
逐页验收判据 = 「页头归 N 套样式 + 骨架成员清单命中 G 套 + §9 机检五硬指标全绿」。
**验收方式 = 抽样取证 + 机检**（2026-10-10 裁定，非逐页人工）：每 PR 抽样 10 页真机截图（名单见 §11 批注）+ 全量机检行。
**14 页已判不可批量**（#1595）：check-in(G8)、practice(G9)、mall 双列、help-center 手风琴、task-center 双分区、notebook 行内编辑、chapter-view 二级导航、personal-activity 分段 feed、personal-info（11 input + 3 弹层）、custom-models CRUD、ai-settings switch 混排、resume-attach 卡片选择、choose-cert 三段、profile-setup 选证+表单 —— 这些页只归套、不做「三件套 −14 行」类批量替换。

---

## 8. 功能空洞生死裁定（#1594，执行随页 PR 同票收）

| # | 对象 | 裁定 | 承接 |
|---|---|---|---|
| ① | `my-forum` | **判死删页**（孤儿页，被 personal-activity 四主 tab 覆盖） | **独立票**（动 pages.json + 4 套契约测试 + `utils/modules.js:316`；60→59 页） |
| ② | `exam-info` | **判生留**（dashboard-menu-grid.uvue:31 四宫格 + materialsPageContract.test.js:39 双入口证伪「零入口」） | 本轮按 N02+G3 收口；mock 改真**独立内容票** |
| ③ | `check-in` 去 mock | **判死 mock**（api/checkin.uts 真实在产；loadMockCalendar:212-240 整条清除、失败显式报错、静默失败 :285-287 补提示） | 随该页 PR |
| ④ | `settings` 清缓存 | **判生解注释**（settings.uvue:43；onClearCache :154-187 完整含 refresh_token 保全 :165-179） | 随该页 PR（N03+G5） |
| ⑤ | `dashboard` 筛选 | **判生改接后端 filter**（单轴「全部/热门/精品」→ getCourseListApi filter（api/course.uts:308）；loadHotCourses 加 filter 参数；直播/录播轴、免费/付费轴删——无 content_type 维度） | 随该页 PR（N05+G2） |
| ⑥ | `practice` past_paper | **维持占位**（诚实占位 :49-52） | **独立产品票**；题库池源标记口径 api/practice.uts:198-199 |

---

## 9. 机检（五硬指标）

扫描对象：`pages/**` + `components/**` + `App.uvue`（排除 `node_modules/` / `uni_modules/` / `unpackage/` / `.ci-verify/`）。
hex 匹配口径：`<style>` 块**声明位** hex（3 位归一 6 位）；不匹配模板/注释/`$变量`；违规 = 声明位 hex ∉（token 值集合 ∪ §1.6 白名单）；**主色字面值不豁免**。

| # | 硬指标 | 口径 |
|---|---|---|
| 1 | **裸 hex = 0** | 声明位 hex 全部走 token 或白名单；无豁免 |
| 2 | **`lang="scss"` 100%** | 全部 `<style>` 声明；无豁免 |
| 3 | **手写 `.status-bar` = 0 且页头件引用 100%** | 页头唯一件 `app-nav-bar`；豁免名单 = N06 六页 |
| 4 | **`app-*` 引用 100%** | 对应交互一律走八件共享件（§10）；渐进转绿（先断言页头件自身，新页必须走共享件，旧页逐票转绿） |
| 5 | **主色背景走 token** | 背景=主色处禁字面值 |

反向机检 + 渐进：机检先断言共享件自身合规，存量页逐票转绿；N 套即白名单结构基础。

---

## 10. 文件约定

- UI 规范文档：`docs/ui-spec.md`（本文件）
- Token 定义：`uni.scss`（**唯一真源**；收编清单见 §1 各裁定注：改 :18/:81、删 :86/:128、增 `$bg-header` + 3 tint）
- 全局样式：`App.uvue` 中的 `<style>` 块
- 共享组件：`components/app-*/` 目录 —— **八件扶正**（ADR-0035：`app-*` 八件全部保留并推向兑现；#0022/#0028 的禁令**只限分段类组件**，`app-nav-bar` 不受限）
- 实施计划：`docs/ui-spec-implementation-plan.md`
- 台账/对比类文档：`docs/choose-cert-diff.md`、`docs/ui-design-diff-analysis.md` 为**历史快照**（各自文首取代块），不作为形态依据

**当前兑现度（2026-10-10 实测）**：8 件 `app-*` 中 **6 件 0 引用**（`app-badge` / `app-button` / `app-card` / `app-tabs` / `app-nav-bar` / `app-list-item`），活着的只有 `app-chip` 与 `app-empty-state`。本节不是现状清单而是**验收基线**——ADR-0035 的 T4/T5 就是把它推向兑现。死件里 `app-button` / `app-badge` / `app-tabs` / `app-list-item` 还引用了 `uni.scss` 里**不存在**的 `$text-inverse` / `$text-secondary` / `$text-placeholder`，且**自身没声明 `lang="scss"`** ⇒ 直接引用只会静默失效（扶正时一并修）。

`components/common` 两死件（`platform-nav-bar.uvue` 165 行、`page-container.uvue` 89 行 tabBarHeight 恒假）**直接删除**，非新决策（ADR-0035:69）；连带 `CONTEXT.md:143` 词条改过去时 —— **随批 0 骨架层首票**。

---

## 11. 推进批次（#1594 排序）

**批 0 骨架层基建**：`app-nav-bar.uvue:37` 补 `lang="scss"` + `components/common` 两死件删除（连带 CONTEXT.md:143）+ `$bg-header` token 登记 + N04 并入 N03 + 行数登记漂移订正（`utils/modules.js:132` INFRA.oversized 611→712；`docs/refactor-decisions.md:33-36` recruit.uts 727→778、request.uts 643→712，口径见 §12）
→ **批 1** 招聘端 9 页样板 → **批 2** N02 余 5 → **批 3** N03 余 14 → **批 4** N01 蓝底 19 → **批 5** N05 透明 7 → **批 6** N06 无页头 6。
批内按入度降序（Top：login 10、recruiter/home 9、dashboard 9、course-detail 8、forum-create 8、forum-detail 7）。
③④⑤（§8）各随其页 PR 同票收。**每批真机抽样 10 页**：search/search、practice/practice、forum/check-in、forum/forum、courses/course-detail、ai-assistant/ai-assistant、recruiter/home、profile/personal-activity、mall/mall、practice/practice-do。

**独立票清单**：my-forum 删除票 · exam-info mock 改真内容票 · past_paper 产品票 · uiDocDocMapContract 守护票（`utils/uiDocDocMapContract.test.js`：断言文档地图 6 份 + 状态枚举 + 文档 token 表↔uni.scss 一致性；动 `utils/` 触发 ④ 门 + guards.md 三问，开票与否 #1597 S3 待拍板）。

---

## 12. 行数预算与遗留决策

**600 行预算（#1598 口径）**：
- 执法口径 = `utils/contractHarness.js` 的 `fileLines`（`readText(f).split('\n').length` 总行数；CRLF 由 `utils/utsHarness.js:165` normalizeEol 归一）。
- **单向门**：不得新增 >600 行文件。**存量超限是观测项不作门**：触及哪份就在该 PR 贴 `fileLines` 前后对比。双向门与「拆文件另开票」冲突且必假红，不采。
- 存量超 600（全树 9 份，2026-10-10 实测）：`.uvue` 6 份（ai-assistant 839、recruiter/resume-detail 731、search 702、ai-feature 629、ai-settings 621、points/task-center 617）+ `.uts` 3 份（api/recruit.uts 778、api/request.uts 712、api/aiAssistant.uts 667）。骨架层对其中 5/6 份是减行（净 −35..−40），唯一增：ai-assistant.uvue 净 0..+3（839→842）。

**App 端影响面**（2026-10-10 裁定）：本规范只管**小程序端**；`.uvue` 两端共用 ⇒ **显式承认连带**（App 端样式随改随走），不设 App 端验证门，验证留给 App 轮。

**遗留决策清单**（不阻塞本规范生效）：
1. #1597 §8 三问待维护者：C4 `$font-size-xxl` 删定义（推荐删，本文按删登记）/ C3 模板顺手改 SCSS token（推荐改）/ S3 守护开票与否。
2. #1596 仓外 Figma 存在性三选一（无 / 有但未入库 / 有且将入库）；现存两份残影文档已降级历史快照。
3. `docs/adr/0035-*.md` 本身未入库（git `??`），**单独提票入库**。
4. uiDocDocMapContract 守护票是否开（同上 S3）。
