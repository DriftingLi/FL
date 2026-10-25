# #1911 图标基座 —— 视觉证据

无头浏览器（本机 Microsoft Edge 156，puppeteer-core，`--no-save` 安装）对**构建产物**截图：
`vite build` → `vite preview --port 4173` → 访问 `/login` 与 `/training`。
接口面用请求拦截返回成功信封（`{code:200,message:'ok',data:…}`），因此截图里没有后端错误提示；
顶栏那张需要一个能过 JWT 过期判定的假令牌（`exp` 在将来）与 `/auth/me` 的桩。

| 文件 | 内容 |
| --- | --- |
| `01-login-light.png` | 登录外壳（AuthPageShell）浅色，右上角是主题触发器 |
| `02-trigger-sun.png` | 触发器特写：浅色 = sun |
| `03-theme-menu-three-states.png` | 三态菜单：sun / moon / sun-moon（跟随系统）—— 同一套注册表数据 |
| `04-morph-mid.png` | **形变中间帧**（点击「深色」后约 70ms）：形状在太阳与月亮之间，不是直接跳变 |
| `05-trigger-moon.png` | 触发器特写：深色 = moon（结算态写回目标 d） |
| `06-login-dark.png` | 登录外壳深色 |
| `07-trigger-system.png` | 触发器特写：跟随系统 = sun-moon |
| `08-login-system.png` | 登录外壳「跟随系统」 |
| `09-reduce-40ms.png` / `10-reduce-180ms.png` | 系统 `prefers-reduced-motion: reduce` 下切换后 40ms 与 180ms —— **两张字节完全一致**（瞬时切换，无中间态） |
| `11-topbar-theme-menu.png` | 登录后的顶栏用户菜单（SidebarLayout 外壳）：外观三项 + 退出登录全部走 `UiIcon` |
| `12-topbar-light.png` | 顶栏整条（通知 / 身份 / 菜单触发器） |
| `13-assistant-shell.png` | AI 助手外壳（公开路由，第三个外壳） |

复现：`cd frontend && npm run build && npx vite preview --port 4173`，另开一个进程用
`puppeteer-core` 驱动本机 Edge 截图（脚本未入库 —— 它依赖本机浏览器路径与预览服务，
入库只会随环境漂移；截图本身是证据，方法即上面这几步）。
