# #1194 招聘者身份面 —— 真机门（①）执行状态

- **PR**：#1206
- **分支**：`feat/mobile-1194-recruiter-identity`
- **状态更新（2026-09-20 15:25）**：**①a 已补跑（部分覆盖）**，取证记录与产物见同目录
  [`README.md`](./README.md)（截图 `01-recruiter-login-after.jpg`、页身份日志 `02-…txt`、只读日志 `03-…txt`）。
  **①b 仍未签**（P1 命中能力面 ⇒ 必做、由人给出原文）。
- 本文件成档时的 HEAD：`0d87a70c`；①a 取证时 HEAD：`5157c844`（两者之间只有文档/测试与两处 Kotlin 类型修复）

## ①a 结果摘要（详细见 README.md）

| 目标页 | 结果 |
| --- | --- |
| `pages/recruiter/login` | ✅ 截图入仓；同帧实拍到身份互斥模态「登录招聘者账号将退出当前学员账号」 |
| `pages/login/login`（底部招聘者入口） | ❌ 未取截图 —— `--pagePath` 进去了，632 ms 后自跳到 `pages/dashboard/dashboard`（设备持有在册学员会话）⇒ 页身份判据 fail-closed |

机检行：`DEVICE_CAPTURE_RESULT=PASS`（前台 `io.dcloud.uniappx/io.dcloud.uniapp.UniAppActivity`，
logcat 窗口 317 行 / `FATAL EXCEPTION=0` / `ANR in 包=0`），全程只读、未注入 `input`。

## 未验证项

- **①a 的 `pages/login/login` 一页**：需一次**未登录态**（或人手动退出学员账号后）的批次补拍。
- **①b 能力面人工门**：指纹路径（`pages/login/login.uvue`）、真机上传路径（`api/request.uts`）——
  **必须由人**按手指 / 选文件后给出原文；**补齐前 ① 不算过**。
- **④b release 云打包**：发版前置条款，正式发版前由人执行（非本 PR 合并前置）。
