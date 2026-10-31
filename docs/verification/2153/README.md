# #2153 侧栏分组头与主题闪烁修复验证

验证日期：2026-10-31。来源：#2150 合并后的线上反馈。

## 1. 侧栏分组节点展开态被遮挡

根因：`.sidebar-nav` 是定高 flex 列（`flex:1` + `overflow-y:auto`），子项默认
`flex-shrink:1`。内容高度超过容器时，分组头被从 34px 压到 **16px**，而它又有
`overflow:hidden`，12px/4px 内边距 + 18px 行高装不进去，文字被裁切/看起来被遮挡。

修复：`.sidebar-nav > * { flex-shrink: 0 }`，内容保持自身高度、溢出交给纵向滚动。

实测（真实 Edge 156，1440×900）：

| 指标 | 修复前 | 修复后 |
| --- | --- | --- |
| 分组头高度 | 16px（padding 12/4 + line-height 18 被压扁） | **34px** |
| 分组折叠中间帧 / 终态 | — | 151 → 46 → 0 |
| 导航列 scrollHeight / clientHeight | — | 915 / 648（可滚动，最后一项可达） |

## 2. 分组节点区分度

分组头不再复用叶子图标，只出文字（`学习中心` / `题库与考试` / `互动与工具` / `个人`）；
折叠态分组头退化为一条 1px 分隔线（`--color-border-light`），保留分组边界。
叶子行仍是「图标 + 文字」，两者不再同形。

## 3. 主题切换闪烁

定位到两处会在切换瞬间制造视觉中断的点：

1. **新快照先整页可见、圆扩散动画后到**：`::view-transition-new(root)` 在 WAAPI 挂上
   之前没有初始 clip，可能闪一帧新主题。修复：做法是先把圈心写进
   `--theme-reveal-x/y`，CSS 把新快照预先裁成 `circle(0px …)`，动画加 `fill: both`
   保持满圆到过渡层移除。
2. **主题菜单白底 popper 在圆里停留**：菜单选中后自身淡出约一百多毫秒，留在圆里像
   闪了一下白块。修复：`is-theme-revealing` 期间对该 popper 加 `display:none`
   （`popper-class="theme-toggle-popper"`）。

实测（真实 Edge 156，点击「深色」后约 60ms）：

- `html.is-theme-revealing = true`
- `.theme-toggle-popper` 存在但 `display: none`
- `::view-transition-new(root)` 动画关键帧：
  `circle(0px at 1300.4px 27.6px)` → `circle(1565.93px at 1300.4px 27.6px)`
- 约 700ms 后：`is-theme-revealing = false`，`--theme-reveal-x/y` 已清空，主题为 dark

## 截图

- `00-sidebar-group-before.png`：修复前（分组头 16px、文字被裁、仍带分组图标）
- `01-sidebar-group-expanded-real.png`：修复后展开态（分组头 34px、只出文字）
- `02-sidebar-group-collapsed-real.png`：修复后折叠态（分组头退化为分隔线）
- `03-theme-reveal-mid-real.png`：主题切换中间帧（菜单已隐藏，圆内无白块）

> 画面左上角的 “A” 是 Edge 浏览器自身浮层，不属于页面内容。

## 本地检查

- `cd frontend && npm run type-check` 通过
- `cd frontend && npm test`：165 files / 1517 tests passed
- 前端守卫：el-controls / ui-intent / api-seam / async-section / a11y / font-scale / icons / ai-assistant-send 全量无违规