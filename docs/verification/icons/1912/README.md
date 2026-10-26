# #1912 图标全量迁移 —— 视觉证据

无头浏览器（本机 Edge 156 + puppeteer-core，`--no-save`）对 **`vite build` 产物**截图；
接口面用请求拦截返回成功信封（`{code:200,…}`），并按外壳注入对应角色的假令牌与 `/auth/me` 桩
（管理端另加 `/admin/me/capabilities` 的能力集桩，否则被守卫踢到 `/admin/no-access`）。

| 文件 | 外壳 | 说明 |
| --- | --- | --- |
| `login-light.png` / `login-dark.png` | 认证外壳 | 表单前缀图标已从「EP 名字字符串」换成 `UiIcon`（原先 `Lock`/`Message`/`Postcard`/`OfficeBuilding` 根本没被注册 ⇒ 那几格的图标一直是空的，本次顺带修好） |
| `student-light.png` / `student-dark.png` | 学员（training） | 侧栏分组图标、通知铃、用户菜单 |
| `tutor-light.png` / `tutor-dark.png` | 讲师（tutor） | 与学员端同一套侧栏/顶栏结构 |
| `admin-light.png` / `admin-dark.png` | 管理端（manage） | 仪表盘统计卡 + 快捷操作 + 侧栏收起按钮 |
| `recruit-light.png` | 招聘（recruit） | 只取到浅色：深色那张在无头渲染时页面失联（见下） |

对照口径：图标**尺寸与基线**（行内与文字对齐）、侧栏间距、统计卡图标块大小。迁移前后同源数据
（同一 `UiIcon`、同样的 `:size`），观感差异只来自描边/实心风格本身（ADR-0079）。

未取证：招聘外壳的**深色**（`/recruit` 在无头渲染时页面失联，`Page.captureScreenshot` 报
Not attached to an active page）；浅色已取到。与图标改动无关，人工过一遍即可。

复现：`cd frontend && npm run build && npx vite preview --port 4174`，另开进程用 puppeteer-core
驱动本机 Edge（脚本不入库，理由同 #1911 的 README：它依赖本机浏览器路径与预览服务）。
