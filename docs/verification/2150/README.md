# #2150 切换过渡验证

验证日期：2026-10-31。目标：主题切换的圆形揭示、主侧栏 / 分组节点 / AI 助手会话侧栏的伸缩过渡。

## 环境

- 真实浏览器：Edge **156.0.4314.58**（非 headless，CDP 9223，1440×900，`Page.bringToFront` 后 `visibilityState=visible`）
- 本地链路：`frontend` Vite 5173；`backend` :8080（Vite 代理 18080）；学员账号登录
- 判据：过渡进行中采样 DOM 几何，并用 `document.documentElement.getAnimations()` 读 View Transition 的 clip-path 关键帧

## 读数（真实浏览器）

| 交互 | 中间帧 | 终态 |
| --- | --- | --- |
| 主题切换 | `::view-transition-new(root)` clip `circle(0px at 1300.4px 27.6px)` → `circle(1565.93px at 1300.4px 27.6px)`；采样时 `data-theme` 已由 light 翻到 dark | 整页新主题 |
| 主侧栏收起 | 点击后约 80ms：width **186.18px**（240 → 64），同期顶栏左格 `grid-template-columns` 也是中间值 | 64px |
| 侧栏分组折叠 | 点击分组头后约 80ms：`.nav-group-collapse` height **93.25px**（151 → 0） | 0 |
| AI 助手侧栏收起 | 点击后约 80ms：margin-left **-136.99px**（0 → -260），`.chat-main` 宽度 1317 → 1440 | -260px |

同口径无头 Edge（`headless=new`，CDP 9222）读数：主题 clip 0 → 1570.63；主侧栏 159.61 → 64；分组 130.72 → 0；AI 侧栏 -118.9 → -260。

## 截图

- `01-theme-reveal-mid-real.png`：主题圆形揭示中间帧（圆外仍是旧主题，圆内已是新主题）
- `02-sidebar-collapse-mid-real.png`：主侧栏宽度过渡中间帧（标签已退出布局、图标随侧栏收缩居中）
- `03-ai-sidebar-collapse-mid-real.png`：AI 助手会话侧栏滑出中间帧（主对话区同步变宽）

> 画面左上角的 “A” 是 Edge 浏览器自身浮层，不属于页面内容。

## 本地检查

- `cd frontend && npm run type-check` 通过
- `cd frontend && npm test` → 164 files / 1518 tests passed
- 前端守卫（el-controls / ui-intent / api-seam / async-section / a11y / font-scale / icons / ai-assistant-send）全量无违规
