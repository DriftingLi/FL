# #1194 招聘者身份面 —— 真机门（①）执行状态

- **PR**：#1206
- **分支**：`feat/mobile-1194-recruiter-identity`
- **状态更新（2026-09-20 15:25）**：**①a 已补跑（部分覆盖）**，取证记录与产物见同目录
  [`README.md`](./README.md)（截图 `01-recruiter-login-after.jpg`、页身份日志 `02-…txt`、只读日志 `03-…txt`）。
  **①b 仍未签**（P1 命中能力面 ⇒ 必做、由人给出原文）。
- 本文件成档时的 HEAD：`0d87a70c`；①a 取证时 HEAD：`5157c844`（两者之间只有文档/测试与两处 Kotlin 类型修复）
- **状态更新（2026-09-20 20:53，收口批）**：**①a 两帧补齐** —— 帧1 `03-recruiter-login-student-exit.jpg`（`pages/recruiter/login` 显示新出口）、
  帧2 `04-back-to-student-login.jpg`（点出口后落回 `pages/login/login`，死胡同已解）；本轮另按 ADR-0008 ①a 补遗第 3 条
  **现测后执行了唯一一次 `input tap`**（设备由维护者声明归本次取证专用），读数见 `05-exit-capture-readings.txt` / `06-machine-check-and-readings.txt`。
  ①a 取证时 HEAD：`3a1c1c50`。**①b 已由人签**（2026-09-20，原文由人给出、agent 代录，见 PR #1206 正文 ①b 行）。

## ①a 结果摘要（详细见 README.md）

| 目标页 | 结果 |
| --- | --- |
| `pages/recruiter/login` | ✅ 截图入仓；同帧实拍到身份互斥模态「登录招聘者账号将退出当前学员账号」 |
| `pages/login/login`（底部招聘者入口） | ❌ 未取截图 —— `--pagePath` 进去了，632 ms 后自跳到 `pages/dashboard/dashboard`（设备持有在册学员会话）⇒ 页身份判据 fail-closed |

机检行：`DEVICE_CAPTURE_RESULT=PASS`（前台 `io.dcloud.uniappx/io.dcloud.uniapp.UniAppActivity`，
logcat 窗口 317 行 / `FATAL EXCEPTION=0` / `ANR in 包=0`），全程只读、未注入 `input`。

## 未验证项

> 下面两条**已成档时的状态**保留原样（记录不改写）；当前状态见每条后的「已补 / 已签」。

- **①a 的 `pages/login/login` 一页**：需一次**未登录态**（或人手动退出学员账号后）的批次补拍。
  **已补（2026-09-20 20:52）**：以「点招聘者登录页出口后的落地页」形态取到 `04-back-to-student-login.jpg`（同帧可见底部两个入口）。
- **①b 能力面人工门**：指纹路径（`pages/login/login.uvue`）、真机上传路径（`api/request.uts`）——
  **必须由人**按手指 / 选文件后给出原文；**补齐前 ① 不算过**。
  **已签（2026-09-20）**：原文由人给出、agent 逐字代录（见 PR #1206 正文 ①b 行；本目录不复制该原文，避免两处漂移）。
- **④b release 云打包**：发版前置条款，正式发版前由人执行（非本 PR 合并前置）。**仍未跑**（维持不变）。
