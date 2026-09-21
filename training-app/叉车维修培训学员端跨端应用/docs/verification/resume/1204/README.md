# #1204 简历 notfound 语义 —— ①a Android 真机逐页截图取证

- **Issue**：#1204
- **被验对象**：分支 `feat/1204` HEAD `79e2bdd3`（`hx:run` 部署的就是这棵树）
- **日期**：2026-09-21
- **设备**：`192.168.0.212:43057`（Android 真机，Xiaomi `23049RAD8C` / Android 15，**无线调试**）
- **包名**：`io.dcloud.uniappx`（uni-app-x 调试基座，appid `__UNI__1C1D180`，打真实后端 `https://www.gccsmile.com/api`）
- **执行人**：agent 执行（本目录只承载 ①a 证据；**①b 能力面未命中** —— 本改动不碰指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互）
- **取证夹具**：设备上登录的是一个**没有简历**的账号（人在设备上完成登录，agent 不接触任何凭证）。判据是
  **设备侧请求事实**：`GET /resume → 404`（见 `03-logcat-request-404.txt`），总览页完善度 `0%（0/8 项已填写）`。
  取证前置的同一台设备上曾出现 `GET /resume → 200`（另一登录态）—— 那一轮**不构成**本票证据，故未采。

## 判据（本票验收的两条，逐条落到机器可核的事实）

| 验收判据（票面） | 机器判据 | 结果 |
| --- | --- | --- |
| 新用户（无简历）进编辑页**无**「简历加载失败」toast | ① 设备侧 `GET /resume` 返回 **404**（logcat，3 次）；② 编辑页 a11y 文本集合里 **0 次**「简历加载失败」 | ✅ 见 `02-resume-edit.a11y.txt` / `03-logcat-request-404.txt` |
| 总览页**展示**完善度引导卡 | a11y 文本含「**去填写简历**」与「**完善度 0%（0/8 项已填写）**」（`resumeIncomplete = 已加载 && 未填满` 的渲染结果） | ✅ 见 `01-resume-overview.a11y.txt` |

补一条**反面对照**（防「反正都看不见 toast」的假绿）：同一棵树上把 404 之外的失败留在原语义 ——
`500 / 业务码错误 / 网络失败 / 403 / 非 Error` 一律**不判** notfound、编辑页**照常**弹「简历加载失败」，
由行为守护 `utils/resumeNotFoundBehavior.test.js` 的 B2 与 A3–A7 钉住（成对断言：B1 必不红 / B2 必红）。

## 手法（会话式，非自动模块）

```powershell
# 0) 设备侧前提：插电常亮（编译期间灭屏会让 uiautomator 返空树、screencap 成全黑）
adb -s 192.168.0.212:43057 shell settings put global stay_on_while_plugged_in 3
# 1) 部署被验树（唯一一次编译；项目目录先改唯一名，避免 HBuilderX 同名误命中）
npm run hx:run
# 2) 深链落到简历总览（--pagePath；这条也会触发一次增量编译，故只发一次）
cli.exe launch app-android --project <项目> --deviceId 192.168.0.212:43057 --pagePath pages/resume/resume
# 3) 逐页取证：uiautomator dump（uvue 文字在 content-desc）+ screencap
pwsh .ci-verify/capture-1204.ps1 -Name 01-resume-overview -OutDir <本目录>
# 4) 点击导航进编辑页：坐标取 uiautomator 的 bounds（在线简历 [203,1538][363,1592] ⇒ 中心 283,1565）
adb shell "input tap 283 1565; echo TAP_RC=$?"     # 现测：TAP_RC=True（inject 可用）
pwsh .ci-verify/capture-1204.ps1 -Name 02-resume-edit -OutDir <本目录>
```

机检行（采自本轮）：

```
CAPTURE name=01-resume-overview device=192.168.0.212:43057 nodes=24 png_bytes=122581 a11y_lines=22
CAPTURE name=02-resume-edit     device=192.168.0.212:43057 nodes=37 png_bytes=162790 a11y_lines=35
```

## 产物清单

| 文件 | 内容 |
| --- | --- |
| `01-resume-overview.a11y.xml` / `.a11y.txt` / `.png` | 简历总览页：a11y 原始树、抽取文本、截图（引导卡「去填写简历」+「完善度 0%（0/8 项已填写）」） |
| `02-resume-edit.a11y.xml` / `.a11y.txt` / `.png` | 在线简历编辑页：a11y 原始树、抽取文本、截图（标题「在线简历」+ 表单字段，**无**错误 toast 文本） |
| `03-logcat-request-404.txt` | 11:35–11:39 的 logcat 切片：`[request] <<< 404 https://www.gccsmile.com/api/resume`，以及 `进入页面:/pages/resume/resume-edit` |

## 诚实声明（agent 无视觉通道）

本会话**模型不接受图片输入**（`read_image` 明确报错）⇒ 不把「截图已入仓」当作「内容已核对」。
截图**照常入仓**（满足 ①a 的「入仓截图」形状要求），但本轮**结论只由可机核的文本 / 日志判据支撑**：
a11y 文本集合 + logcat 的 HTTP 状态码。**改前对照（术前真机截图）本轮未取** —— 它要再花两次增量编译（部署旧树 ~5 min +
深链 ~5 min），而术前行为在本票票面已有原样记录、且行为守护的变异用例 C1–C3 已把「术前形态必红」钉死；
此处如实记为**未验证面**，不冒充已对照。
