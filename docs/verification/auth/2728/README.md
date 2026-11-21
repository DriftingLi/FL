# #2728 真机 A/B 证据（移动端鉴权）

设备：`f0bae674`（2510DRK44C，Android 16 / API 36），基座 `io.dcloud.uniappx`，后端 `https://api.gdyidao.cn/api`。
账号：招聘者 `HRtester`（人工登录一次，保持登录态）。

## 红：修复前代码

把本次 5 个改动文件暂时收回后部署（设备侧 bundle `apps/__UNI__1C1D180/www` 落盘 2026-11-21 09:54）。
3 轮 `am force-stop` → `monkey` 冷启动（10:00:54–10:03:31）：

| 轮次 | 落地页 | `/api/auth/me` | HTTP 403 |
| --- | --- | --- | --- |
| 1 | `/pages/index/index` | 200 | 2 |
| 2 | `/pages/index/index` | 200 | 2 |
| 3 | `/pages/index/index` | 200 | 2 |

403 端点为 `/api/student/courses`、`/api/notifications/unread-count`；第 3 轮同时有
`[dashboard] loadMyCourses failed`。即：招聘者 token 有效，却被送进学员首页并触发 403，
用户可见症状为「权限不足」。

## 绿：修复后代码

恢复改动后重新部署（bundle 落盘 2026-11-21 10:24）。3 轮冷启动（10:26:58–10:28:14）：

| 轮次 | 落地页 | HTTP 403 | 「权限不足」文案 |
| --- | --- | --- | --- |
| 1 | `/pages/recruiter/home` | 0 | 0 |
| 2 | `/pages/recruiter/home` | 0 | 0 |
| 3 | `/pages/recruiter/home` | 0 | 0 |

界面实测文本含 `工作台`、`企业招聘工作台 · 账号 HRtester`。

## 边界

- 该 A/B 只覆盖「已登录冷启动按身份落地」。
- 「退出竞态」「退出即重登」需要延迟注入 401 的受控通道；调试基座指向 HTTPS，
  全局代理只能 CONNECT、无法改写响应，未做 MITM，留待联调 HTTP 后端可用后补测。
- 原始截图、UI 树与按 pid 过滤的 logcat 在临时目录 .ci-verify/auth-red，未入库。
- 设备侧部署由 `scripts/hx-run.ps1` 发起，但其 verdict 在本机误报 `deployed=false`
  （`pid_before=none`），证据以 bundle mtime 与冷启动行为为准；门判据本身另票跟踪。
