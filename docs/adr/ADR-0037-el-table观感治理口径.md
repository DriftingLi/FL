# ADR-0037: el-table 观感治理口径（全局变量层，不封装）

- 状态：已接受（2026-09-09）
- 领域：前端 / 表格观感（承接 ADR-0035 中「el-table 走全局样式覆盖、不做封装」的既定路线）

## 背景

全站 `<el-table>` 197 处 / 22 文件，是裸用量最大的 Element Plus 控件。`element-overrides.css` 覆盖了 button / card / dialog / drawer / dropdown / input / message-box / pagination / popper，唯独没有 table —— 表格的表头底色、边框、行 hover、文字色全是 EP 出厂值，与已收敛到 token 体系的卡片、弹窗、分页、筛选栏色温不一致，深色模式下更明显。

另外，表格的**空态**此前完全未治理：仅 1 处显式 `#empty`（且在上轮空态归一 #717 中已顺带换成 `UiEmptyState`）与 6 处 `empty-text` 属性（全部集中在 `ValuationConfigManage`），其余表格用 EP 默认空态文案。

## 决策

1. **不封装 `UiTable`，走 `--el-table-*` 全局变量层**。在 `element-overrides.css` 的 `:root` 定义表格变量，一次性覆盖全部 197 处，业务代码零改动。

2. **表格变量只引用会随主题翻转的项目 token**：`--color-bg-card` / `--color-bg-page` / `--color-border-light` / `--color-text-*`。**禁止**在表格上使用 `--color-primary-*` / `--color-accent-*` 做底色 —— 深色块只重定义了 `bg-*` 与 `text-*` 系列，`primary-50`（#F0FDFA）等在深色下仍是亮色，做 hover / 选中行底色会变成「暗底亮块」。

3. **不动行高与单元格内边距 —— 硬边界**。197 处表格的列表密度、分页位置、滚动条必须与改动前一致。表格观感治理永远只到「颜色与描边」层，不碰布局尺寸。

4. 结构层只加三条：表头字重（`--font-semibold`）、表头下边线（用 `--color-border` 而非 light，让表头与表体分界更实）、空态文字色（`--color-text-tertiary`）。

5. 深色模式唯一需要专属值的是**固定列阴影**：浅色的淡阴影在深色下不可见，横向滚动时看不出列被固定。在深色块补 `--el-table-fixed-box-shadow` 与左右列内阴影的深色值。

6. **空态口径**：显式空态（`#empty` 插槽或 `empty-text`）一律用 `UiEmptyState`（本轮迁移 `ValuationConfigManage` 6 处）；其余表格的默认空态走全局文字色跟随 token，不逐个改业务代码。

## 备选

- **封装 `UiTable`**：拒绝 —— 197 处逐处替换成本失控，且表格差异在列定义而非容器，封装收益低（ADR-0035 已论证）。
- **顺带统一行高与内边距**：拒绝 —— 197 处表格的垂直空间会全部变动，需要逐域目测，收益（密度统一）远小于回归风险；如未来确需统一，单独立项并逐域验收。
- **只补深色模式**：拒绝 —— 浅色下表格与卡片、弹窗同样不是一套语言，只做深色治标不治本。
- **22 个文件全部显式加 `#empty`**：拒绝 —— 没有自定义空态的表格写插槽是无意义 diff，与「默认就对」的目标相悖。

## 后果

- 197 处表格观感一次到位，新表格**自动继承**，无需任何额外动作。
- 本轮唯一无法自动验证的是观感本身 —— 需逐域人工走查（浅深两主题），这是全局样式类改动的固有代价。
- 表格配色今后**只在 `element-overrides.css` 单点调整**；业务页面出现针对 `.el-table` 的局部覆盖即视为违规（评审口径）。
- 行高边界是给未来的自己留的护栏：若某天要统一表格密度，必须单独立项、逐域验收，不允许搭车。

## 相关

- ADR：`ADR-0035-UI控件收敛到ui封装层`（el-table 边界的原始定义）、`ADR-0036-冻结区窄口径一次性解冻机制`（本轮 `ValuationConfigManage` 空态迁移的准入依据）
- 约定：`AGENTS.md`「UI 词汇」表格词条
- Issues：`#733`（本轮 spec 与实施）；`#717`（空态归一，表格空态的前序）
- 代码：`frontend/src/assets/styles/element-overrides.css`、`frontend/src/pages/admin/ValuationConfigManage.vue`
