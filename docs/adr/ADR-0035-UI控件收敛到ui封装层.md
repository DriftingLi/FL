# ADR-0035: UI 控件收敛到 `components/ui/` 封装层

- 状态：已接受（2026-09-08）
- 领域：前端 / 设计系统与 UI 治理（承接 AGENTS.md「前端 UI 约定」与 #554 裸色值治理）

## 背景

项目已建 `frontend/src/components/ui/` 封装层（20 个组件：UiButton / UiCard / UiDialog / UiEmptyState / UiSegmentTabs / UiSkeleton / UiStatCard …），但业务页面仍大量直接使用 Element Plus 原生控件。2026-09-08 全量实测：

| 原生控件 | 裸用量 | 已有封装 | 封装使用量 |
| --- | --- | --- | --- |
| `el-table` | 197 处 | 无 | — |
| `el-dialog` | 46 处 / 34 文件 | `UiDialog` | 5 个文件 |
| `el-pagination` | 33 处 | 无 | — |
| `el-empty` | 18 处 | `UiEmptyState` | 24 处 |

后果有三：① 同一控件在不同页面观感不同（弹窗头尾间距、分页器是否带背景、空态图标大小）；② 视觉调整无法单点生效，改一处要搜改几十个文件；③ 深色模式靠翻 CSS 变量实现，**只对走 `var()` 的声明生效**，裸用 EP 控件的部分天然游离在主题体系之外（这正是 #554 花了整轮去清裸 hex 的根因）。

## 决策

1. **业务页面不得直接使用 Element Plus 控件**，一律经 `components/ui/` 封装层。新代码禁止再写 `el-dialog` / `el-pagination` / `el-empty` / `el-button`（表单域等尚未封装的控件见第 3 条例外）。

2. **封装层的准入按「重复度 × 视觉影响」排序**，分批推进，本轮先收四类：弹窗（#716）、空态（#717）、分页与筛选栏（#718）、分段控件（#714）。

3. **`el-table`（197 处）是明确的适用边界 —— 走全局样式覆盖，不做封装**。理由：数量级比其余三类之和还大，逐处替换的评审与回归成本不可控；且表格的差异主要在列定义而非容器，封装收益低。其外观统一改由 `element-overrides.css` 承担（该文件目前只覆盖变量，未覆盖结构样式，需要时另行补）。

4. **封装必须纯增量（AGENTS.md R2）**：新增 prop 的默认值等于改造前裸用行为，让未传值的调用方零视觉 diff；变体一律追加覆盖，不改写现有规则。

5. **封装层内部约束不变（R1 / R3 / R4）**：模板只用 Tailwind 原子类；改 EP 内部结构走 scoped `:deep()`；颜色一律走 `design-tokens.css` 变量或经 `tailwind.css` 的 `@theme` 桥接的语义原子类，**禁止裸 hex**（CI `scripts/check-bare-hex.sh --diff` 拦截）。

6. **封装只收视图，不抢状态**：已有 `useAsyncPage`（39 个文件在用）/ `useAdminTable` / `useCrudTable` 等 composable 持有列表与分页状态，封装层不重复造状态管理，只暴露可直接对接的 props 契约。

## 备选

- **全量封装所有 EP 控件**：拒绝 —— 表单域（input / select / date-picker / switch / upload）与 table 合计数百处，一次性封装成本失控，且本轮的实际痛点是观感不统一而非「没有组件」。
- **只做全局样式覆盖、不封装**：拒绝 —— 能统一观感，但治不了「8 种分页模板变体」「三套筛选栏 class」这类写法重复，下次调整仍然要搜改。
- **维持现状、允许业务页面直接用 EP**：拒绝 —— 这就是 #554 那轮不得不全站清裸 hex 的成因；只要裸用存在，主题体系就有游离面。
- **给封装层加 lint 强制规则**：暂缓 —— 当前 CI 只检查裸 hex；等本轮四类收敛完成、封装层覆盖面足够后再评估加 lint，否则会一次性报出上百处告警。

## 后果

- `components/ui/` 成为 UI 的唯一入口，新代码不再直接写 `el-*`；评审新增一条口径：**业务页面出现裸 EP 控件即打回**。
- 迁移按 spec 分批落地（#716 弹窗 / #717 空态 / #718 分页与筛选栏 / #714 分段控件），每份独立验收，不合并成一个大改造。
- `el-table` 长期保持裸用 + 全局样式覆盖的双轨状态，需在 AGENTS.md 与后续 spec 中持续标注，避免被误当成「漏封装」。
- 封装层会随迁移扩张（新增 `UiPagination`、`UiFilterBar`，`UiDialog` 补 prop），需同步补 `ui-components.spec.ts` 用例。

## 相关

- ADR：`ADR-0036-冻结区窄口径一次性解冻机制`（本轮 admin 域迁移的准入依据）
- 约定：`AGENTS.md`（Tailwind 增量共存 R1~R4、禁止硬编码色值、冻结区）
- Issues：`#714`（分段控件）、`#716`（弹窗收敛）、`#717`（空态归一）、`#718`（分页与筛选栏）；`#554`（裸 hex 治理，本 ADR 的直接前因）
- 代码：`frontend/src/components/ui/`、`frontend/src/assets/styles/element-overrides.css`、`frontend/scripts/check-bare-hex.sh`
