# 前端 UI 约定

> 改前端模板/样式前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

## UI 词汇

| 术语 | 含义 |
| --- | --- |
| **封装层** | `components/ui/` 下的自建组件集合。业务页面**不得直接使用 Element Plus 控件**，一律经此层（ADR-0035）。例外：`el-table`（197 处，走全局样式覆盖，ADR-0037）与 `el-radio` / `el-radio-button`（组内内容项，ADR-0038）。 |
| **分段控件** | `UiSegmentTabs`，带滑动指示条的选项卡。激活态必须用品牌语义色，**不可用 `bg-panel`** —— 与卡片同值会让滑块在卡片内彻底隐身（浅色 `#FFFFFF` on `#FFFFFF`、深色 `#1E293B` on `#1E293B`，两套主题都失效）。 |
| **下划线 tab** | `UiUnderlineTabs`，**视图档位**用（临时切换看什么，如 编写/预览）。**数据档位**（会被保存的选择，如 纯文本/Markdown）仍用 `UiSegmentTabs`（胶囊 + 实心品牌色滑块）。两者**不可互换**：同一卡片里并排两组同款胶囊，用户分不清哪组会永久保存。激活下划线必须用品牌语义色 `--color-ui-*`，**不可 `bg-panel`**（与卡片同值会隐身，同分段控件的硬约束）；调用方给顶栏 1px 底边，组件用 `-mb-px` 压住它。 |
| **图标工具栏** | `MarkdownToolbar`，一排图标按钮：**每个按钮都要有 `UiTooltip` 中文提示 + `aria-label`**（文案与图标名的唯一来源是命令表 `utils/markdownToolbar.ts`，不许在组件里另抄）。置灰用 `aria-disabled` + 视觉降透明度，**不用原生 `disabled`** —— 原生 disabled 的按钮不派发鼠标事件，提示气泡会整排消失。 |
| **空态两级** | 独立占据内容区的空态（整页 / 列表 / 面板）用 `UiEmptyState`；**卡片正文内嵌**的一行提示保留纯文案（统一 `text-ink-3`），不塞组件 —— 后者换成组件会多出图标与整块留白，比问题本身更重。 |
| **列表四段式** | `UiAsyncSection`：**空态 → 错误态（+ retry）→ 骨架 → 内容**的唯一编排实现（props `loading` / `error` / `empty` / `retrying`，slots `default` / `empty` / `error` / `skeleton`，`@retry`）。页面只提供 loader 与 slot，**不得再手写四分支链**。空态判据与「筛选变化回第一页」在 `useAsyncPage` 内，不再各页手写（ADR-0053）。**判据只认 `isEmpty`**（第十一波 #1101）：新增 `:empty=` 只允许 `isEmpty`（composable 返回的，或页面里 `const isEmpty = …` 具名派生的）或登记例外，不得再写 `x.length === 0` / `!data` / 写死 `false`——守卫 `check-async-section` 机械拦截。**404 = 空态、其余 = 错误态**：`useAsyncPage` 提供 `loadErrorKind`（复用 `api/client.ts` 的 `ApiErrorKind`），资源不存在时 `isEmpty` 同样为真，**详情页不再自建「未找到」布尔**；判据形状特殊（一个 loader 写两个列表、以 total 为准）的页面复用 `utils/listState.ts` 的 `isEmptyValue` / `isEmptyList`，不要再抄一份表达式。**追加式分页（「加载更多」）只有 `useAsyncPage({ mode: 'append' })` 一个入口**（`itemsRef` + `loadMore` / `hasMore` / `loadingMore` / `reset`），页面不得再手算页码与累积。**表格页是它的一档**（`skeleton="none"`）：`el-table` 保留自带 loading 遮罩，空态仍按本表「表格」行（ADR-0037）——不为统一观感把 10+ 个管理页的加载态换掉。展示型第 5 态（如章节不存在）用 `UiEmptyState` + action 表达，**不自造状态**。 |
| **筛选栏** | `UiFilterBar`，只提供容器与 `#filters` / `#actions` 两个插槽，字段由各页自写；**不做 prop 化** —— 各页字段数与按钮语义不一致，prop 化会让组件持续膨胀。 |
| **表格** | `<el-table>` 走 `element-overrides.css` 的 `--el-table-*` 全局变量（ADR-0037），**不封装 UiTable**。两条硬边界：**不动行高与单元格内边距**；**禁用 `primary-*`/`accent-*` 做表格底色**（深色块未重定义这两个色阶，会出「暗底亮块」）。表格显式空态用 `UiEmptyState`，默认空态走全局文字色。 |
| **剩余控件** | 标签一律 `UiTag` 的 `tone`（**不写 `type`**；新值 `brand/primary/success/info/warning/danger/neutral`，映射表用导出的 `UiTagTone` 类型收窄）；开关/多选/单选组/上传/提示气泡对应 `UiSwitch` / `UiCheckbox`（+`UiCheckboxGroup`）/ `UiRadioGroup` / `UiUpload` / `UiTooltip`。全部薄封装（attrs/事件/slot 全透传、**不设默认值**，ADR-0038）；`el-radio` / `el-radio-button` 保持原生（组内内容项）。 |
| **溢出菜单** | `UiMoreMenu`，卡片/列表项右上角「⋯」触发的**治理动作收纳**（举报 / 删除等低频、破坏性、非互动类操作）。**互动动作不进菜单**——回复 / 点赞这类高频社交动作留在卡片底部主操作行，两者不可混放（ADR-0042 的回复区形态）。菜单项由调用方提供，**可见性判定也在调用方**（如自己的回复不出现「举报」）。 |
| **管理端列表状态机** | admin **列表页**一律 `useAdminTable`（页面只声明 `fetch` adapter 与 `actions` adapter，内置三态 / 分页 / 搜索 / 行操作分发 / 删除确认；ADR-0015 + ADR-0039）。**非列表页不套**（详情/仪表盘/配置页用 `useAsyncPage` 的三态即可）。`useAsyncPage` 是服务全站 34 处的通用三态件，**不要为 admin 改它**。**同一页面里的第二档要写明归属**（第十一波）：分页列表 → `useAdminTable`；只读/计数 section（巡检计数、汇总卡）→ `useAsyncPage` + `UiAsyncSection`。两档都不得 `catch {}` 静默吞错，档位在文件顶部注释里登记。 |
| **状态词表** | 同一业务状态（联络授权 / 投递 / 题目状态…）的 **label 与 tone 各只有一个 descriptor**：输入 status，输出 `{ label, tone }`，列表、抽屉、角标、admin 留痕只消费它；status 收成 union（取值集合与后端常量表对齐），**模板里不得内联状态文案裸串**（第十一波）。 |
| **确认框** | 一律 `useConfirm()`（`composables/useConfirm.ts`）：`confirm`（普通）/ `confirmDanger`（删除、清空、移除、驳回、撤销等不可逆操作 —— 红确认钮 + 焦点不落确认钮，连按回车不误执行）/ `prompt`（带输入）。**业务代码禁直接调 `ElMessageBox`**。 |

### 已收敛控件的机械守卫（ADR-0035 备选转正 / spec #940）

「业务页面不得直接使用 Element Plus 控件」不再只靠评审人眼守，CI 里有一条可核验的守卫：

```
node scripts/check-el-controls.mjs --all                  # 全量（CI 用这个），有违规则退出 1
node scripts/check-el-controls.mjs --diff origin/master   # 只看新增行（本地/渐进期）
```

- **守卫集 = 已收敛的 10 类**：dialog / empty / pagination / button / tag / switch / checkbox-group / radio-group / upload / tooltip。命中即报红并给出「→ UiXxx」的对应关系。
- **判定面只认根 `<template>` 块**并跳过 HTML 注释 —— `<script>` 里的字符串与 EP 类名字面量不算违规。
- **放行面**（有意不进守卫集，别当成漏封装）：`components/ui/**`（封装层内部本来就是 EP）、`el-radio` / `el-radio-button` / `el-checkbox`（组的「内容项」而非容器）、`el-table` / `el-table-column`（表格边界）、表单域与布局类 EP（未封装例外）。
- **守卫自己有表驱动自检**（`scripts/check-el-controls.test.mjs`，CI job `el-controls-selftest`）：守卫坏了必须报红，否则规则退化为假绿。

页面保持整洁：不要写冗余的小标题、装饰性提示与说明性 hint 文本，有的话就清理，仅保留必要的功能性提示。删除 hint 时同步删除对应的 CSS class 与 scoped style，避免残留死代码。

**明确例外（不要清）**：

1. 发帖 / 回复输入区的**属地披露提示**（「发布内容会显示 IP 属地」，文案单点 `frontend/src/utils/forumDisplay.ts` 的 `FORUM_REGION_NOTICE`）属于「必要的功能性提示」而非装饰——属地在点发布那一刻才产生，事前告知比事后解释便宜（ADR-0045）。
2. Markdown 档的**能力与边界提示**（文案单点 `forumDisplay.FORUM_MARKDOWN_HINT`，由 `ForumMarkdownInput` 渲染在输入框底部）——它告知两条硬边界：**表格不渲染**、**图片要走粘贴区**（正文里的 `![]()` 会被展开成文字）。依据是 ADR-0046 自己的判据「判据放在作者看得见的地方（编辑器提示 + 预览里的越界说明）」，内容精选编辑器已有同款（ADR-0052）。

按本条约定清理 hint 时**跳过这两条**。

## Tailwind 增量共存四条边界规则

项目已引入 Tailwind CSS v4，与既有 `<style scoped>` 长期共存（详细背景见 `.workbuddy/plans/student-ui-redesign.md`）。共存期间遵守：

| 规则          | 内容                                                                                                       |
| ----------- | -------------------------------------------------------------------------------------------------------- |
| **R1 原子类区** | `src/components/ui/**` 与已纳入改造的页面：模板只用原子类；`<style scoped>` 仅保留伪元素、`:deep()` 改 Element Plus、keyframes、媒体查询 |
| **R2 冻结区**  | 未列入当期改造的页面与组件，scoped 样式**一行不动**。改共用件时新特性一律走 prop + 默认值等于现状，让未传值的调用方零 diff                                |
| **R3 禁双写**  | 同一元素同一属性不允许既有 scoped 类又有原子类。需覆盖 Element Plus 外观时二选一：① scoped 内 `:deep()`；② 原子类加 `!` 后缀（`!px-4`）          |
| **R4 迁移动作** | 页面改用原子类后，删除 scoped 块中已被替代的规则（沿用上一段「删 hint 同步删 CSS」的约定）                                                   |

**变体一律追加覆盖，不改写现有规则**：新增外观分支写成 `.xx.is-dark { … }` 这类多一个类的选择器追加在样式块末尾，特异性天然高于原单类规则，无需 `!important`。

**样式入口只有一个**：`src/assets/styles/tailwind.css`，新增全局样式写进它或它 `@import` 的文件，不要在 `main.ts` 里再加 import。

## 不得触碰的边界

- `--color-brand-*` 属**残值域**专用（`assets/styles/valuation-tokens.css` 在 `.valuation-root` 内定义），**禁止提升为全局变量** —— 会击穿 `layouts/ValuationLayout.vue` 与 `pages/ai-assistant/*` 两处依赖「变量未定义 → 走 fallback」的写法。培训域品牌色用 `--color-primary-*`。

- 残值模块（`pages/student/valuation/**`、`components/valuation/**`）本轮冻结，批量替换色值等机械操作时记得排除。
  **2026-09-04 解除冻结**（见 #554）：为清理深色模式下不跟随主题的硬编码色值，残值域与 `components/catalog/**` 已解冻。解冻后两条约束不变：
  ① `--color-brand-*` 依然禁止提升为全局（见上一条）；
  ② `components/catalog/**`（FacetCard/FacetItem/CourseCard）师生管三端共用，只换色值、不新增 prop。

- **新增样式禁止硬编码色值**：深色模式是靠翻 CSS 变量实现的，只对走 `var()` 的声明生效；裸 hex 不参与变量链，暗色下保持亮色 → 「暗底亮块」崩坏。一律用 `design-tokens.css` 的变量或 Tailwind 原子类。
  CI 会对 PR 的新增行做检查（`scripts/check-bare-hex.sh --diff`），豁免 `var(--token, #fallback)` 防御写法、`#fff`/`#000`、注释行与 `<script>` 块（canvas 色板属合理存在）。存量进度自查：`bash scripts/check-bare-hex.sh --all frontend/src`。

- **主题切换按钮要覆盖所有布局**：现装在 `SidebarLayout`（学员/导师/管理/招聘四端继承它）与 `ValuationLayout`。
  认证布局 `AuthPageShell` 曾漏装，导致系统深色偏好的用户在登录页既看到崩坏画面、又无法切回浅色（#554）。**新增任何独立布局时，必须一并评估主题入口**——这条已踩两次（#432 补了 ValuationLayout，#554 补 AuthPageShell）。
