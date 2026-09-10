# 前端 UI 约定

> 改前端模板/样式前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

## UI 词汇

| 术语 | 含义 |
| --- | --- |
| **封装层** | `components/ui/` 下的自建组件集合。业务页面**不得直接使用 Element Plus 控件**，一律经此层（ADR-0035）。例外：`el-table`（197 处，走全局样式覆盖，ADR-0037）与 `el-radio` / `el-radio-button`（组内内容项，ADR-0038）。 |
| **分段控件** | `UiSegmentTabs`，带滑动指示条的选项卡。激活态必须用品牌语义色，**不可用 `bg-panel`** —— 与卡片同值会让滑块在卡片内彻底隐身（浅色 `#FFFFFF` on `#FFFFFF`、深色 `#1E293B` on `#1E293B`，两套主题都失效）。 |
| **空态两级** | 独立占据内容区的空态（整页 / 列表 / 面板）用 `UiEmptyState`；**卡片正文内嵌**的一行提示保留纯文案（统一 `text-ink-3`），不塞组件 —— 后者换成组件会多出图标与整块留白，比问题本身更重。 |
| **筛选栏** | `UiFilterBar`，只提供容器与 `#filters` / `#actions` 两个插槽，字段由各页自写；**不做 prop 化** —— 各页字段数与按钮语义不一致，prop 化会让组件持续膨胀。 |
| **表格** | `<el-table>` 走 `element-overrides.css` 的 `--el-table-*` 全局变量（ADR-0037），**不封装 UiTable**。两条硬边界：**不动行高与单元格内边距**；**禁用 `primary-*`/`accent-*` 做表格底色**（深色块未重定义这两个色阶，会出「暗底亮块」）。表格显式空态用 `UiEmptyState`，默认空态走全局文字色。 |
| **剩余控件** | 标签一律 `UiTag` 的 `tone`（**不写 `type`**；新值 `brand/primary/success/info/warning/danger/neutral`，映射表用导出的 `UiTagTone` 类型收窄）；开关/多选/单选组/上传/提示气泡对应 `UiSwitch` / `UiCheckbox`（+`UiCheckboxGroup`）/ `UiRadioGroup` / `UiUpload` / `UiTooltip`。全部薄封装（attrs/事件/slot 全透传、**不设默认值**，ADR-0038）；`el-radio` / `el-radio-button` 保持原生（组内内容项）。
| **管理端列表状态机** | admin **列表页**一律 `useAdminTable`（页面只声明 `fetch` adapter 与 `actions` adapter，内置三态 / 分页 / 搜索 / 行操作分发 / 删除确认；ADR-0015 + ADR-0039）。**非列表页不套**（详情/仪表盘/配置页用 `useAsyncPage` 的三态即可）。`useAsyncPage` 是服务全站 34 处的通用三态件，**不要为 admin 改它**。 |
| **确认框** | 一律 `useConfirm()`（`composables/useConfirm.ts`）：`confirm`（普通）/ `confirmDanger`（删除、清空、移除、驳回、撤销等不可逆操作 —— 红确认钮 + 焦点不落确认钮，连按回车不误执行）/ `prompt`（带输入）。**业务代码禁直接调 `ElMessageBox`**。 |

页面保持整洁：不要写冗余的小标题、装饰性提示与说明性 hint 文本，有的话就清理，仅保留必要的功能性提示。删除 hint 时同步删除对应的 CSS class 与 scoped style，避免残留死代码。

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
