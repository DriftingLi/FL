# #1194 招聘者身份面 —— ①a 真机只读取证记录（PR #1206）

> 归档时间：2026-09-20 15:24–15:25 ｜ 分支：`feat/mobile-1194-recruiter-identity` ｜ 取证时 HEAD：`5157c844`
> 设备：`b32d8398`（Redmi `23049RAD8C`，1080×2400）｜ 工具链：HBuilderX 5.23.2026080626（`E:\HBuilderX.5.23.2026080626\HBuilderX`）+ adb 1.0.41
> 口径：`training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`「①a 取证手法补遗」+ `0016-真机门的人工性收缩与按批取证.md`
> **①a 由 agent 出证；本文件不含也不替代 ①b**（P1 命中能力面：`pages/login/login.uvue` 指纹白名单 / `api/request.uts` 真机上传白名单 ⇒ **①b 必做、由人给出原文**）。

## 一、结论：①a **部分覆盖**（2 个目标页中 1 页取得截图，另 1 页因登录态不可达）

| # | 目标页 | 结果 | 依据 |
| --- | --- | --- | --- |
| 1 | `pages/recruiter/login`（独立登录页） | ✅ **截图入仓** `01-recruiter-login-after.jpg` | 导航落定 281s；日志 `进入页面:pages/recruiter/login` |
| 1b | 同上：**身份互斥**提示 | ✅ **实拍到**「退出当前账号 / 登录招聘者账号将退出当前学员账号」模态 | 见下第三节；对应 `pages/recruiter/login.uvue:61-74` |
| 2 | `pages/login/login`（学员登录页底部招聘者入口） | ❌ **未取得截图**（选页失败，见第四节） | 日志先 `进入页面:pages/login/login`，**632 ms 后** `进入页面:/pages/dashboard/dashboard` |

机检行（`scripts/device-capture.ps1`，**只读**，`DEVICE_CAPTURE_RESULT=` 是唯一判据）：

```
DEVICE_CAPTURE_RESULT=PASS
  页面 pages/recruiter/login=SKIP  前台=io.dcloud.uniappx/io.dcloud.uniapp.UniAppActivity
  logcat窗口行数=317  FATAL=0  ANRin包=0
```

只读保证：全程只用 `adb devices` / `exec-out screencap -p` / `shell dumpsys activity` / `shell logcat -d`；
**未** kill-server、未 install/uninstall、未 force-stop、未清 logcat、未 push/pull/reboot、未注入 `input`。

## 二、逐页取证产物

| 文件 | 内容 |
| --- | --- |
| `01-recruiter-login-after.jpg` | `pages/recruiter/login` 实拍（1080×2400 → 720×1600，JPEG q80，45 800 B）；原图 sha256 `d07f4e4d3842c1e489d58f7566bba404c86446dfd69666e3802f913c8935e3ae` |
| `02-page-identity-launch-log.txt` | **页身份**证据：`--pagePath` 请求页 vs 应用日志「进入页面」行（`dumpsys` 只能给到 Activity/包名，uniapp 全页面同一个 Activity ⇒ 这是唯一设备侧页身份证据） |
| `03-device-capture-readonly.txt` | 只读取证日志全文（含上面的机检行、前台 Activity 原始行、logcat 窗口统计） |
| `04-kotlin-all-4c.txt` | ④c 整模块编译日志（`KOTLIN_ALL_RESULT errors=0 classes=1407 files=109`）；④c 的门证据由脚本贴的 sha 绑定 PR 评论承载 |

复现：

```powershell
cd D:\wt-1194\training-app\叉车维修培训学员端跨端应用
npm run dev:finish -- -Level standard -Device b32d8398 -MaxScreenshotPages 8   # 步骤 4-6：④c → 部署 → 逐页 --pagePath 截图
pwsh -NoProfile -File scripts/device-capture.ps1 -Device b32d8398 -Pages "pages/recruiter/login" -Module recruiter-login
```

## 三、`pages/recruiter/login`：页面形态 + 身份互斥（本轮的主要观测）

实拍画面（`01-recruiter-login-after.jpg`）：标题「招聘者登录」；**只有**「招聘者账号」与「请输入密码」两个输入框
（密码框带「显示」切换）；一个「登 录」按钮；左上「返回」。**没有**多余的小标题 / 说明性 hint 文案 —— 与 ADR-0022 ①
「独立招聘者登录页（仅账号密码）」一致。

同帧还拍到模态：**「退出当前账号」/「登录招聘者账号将退出当前学员账号」/「取消」「确定」**。它与源码路径逐条对上：

```
pages/recruiter/login.uvue:60-74
    auth.restoreFromStorage()
    if (auth.isLoggedIn.value && getActiveRole() == ACTIVE_ROLE_STUDENT) {
        uni.showModal({ title: '退出当前账号', content: '登录招聘者账号将退出当前学员账号', ... })
```

⇒ 两条结论：① **身份互斥**（ADR-0021 ② / ADR-0022 ①）在真机上真的生效；② **取证时设备上存在一个在册学员会话**
（这也是第 2 页不可达的原因）。同时说明该页**不自跳转**（页身份判据通过），是可达页。

## 四、`pages/login/login` 为什么没有截图（**选页失败，不是工具链失败**）

`02-page-identity-launch-log.txt` 原样记录：

```
15:17:00.982 进入页面:pages/login/login 。
15:17:01.614 进入页面:/pages/dashboard/dashboard 。
```

即 `--pagePath` **确实进了目标页**，但该页在 632 ms 后就自己跳到了 `pages/dashboard/dashboard`（设备持有在册学员会话 ⇒
启动页逻辑重定向）。`Wait-NavSettled` 的页身份判据因此如实点名「请求页 pages/login/login，日志里最后进入的是
pages/dashboard/dashboard」并 **fail-closed 不产截图**（ADR-0008 第五节「页身份约束」的既有行为，不是本次引入的缺陷）。

要拍到这一页必须先退出当前账号，而退出需要**在 UI 上点按**（`确定`）；本机按会话约定**未使用 `adb shell input` 注入**
（移动端 AGENTS.md / `device-capture.ps1` 只读红线；ADR-0008 补遗第 3 条要求「每次现测」而不默认可用）。
⇒ 按 ADR-0008「①a 取证的选页规则：目标页必须是**当前登录态下可达**的页」，本轮判**选页失败**并如实留痕，
**不伪造、不用别的页冒充**。

**后果写实**：P1 第 3 条判据（学员登录页底部纯文字「招聘者登录」入口 + 可返回）在本轮**未取得真机证据**，
只有 `pages/recruiter/login` 的自跳转-可达性与身份互斥证据。该判据的静态契约由
`utils/concurrent401RefreshBehavior.test.js` 与源码（`.recruiter-entry` / `goRecruiterLogin()`）承担；
真机截图待**一个未登录态**（或人手动退出学员账号后）的批次补拍。

## 五、未做项 / 谁来做

| 项 | 状态 | 谁 / 何时 |
| --- | --- | --- |
| ①a `pages/recruiter/login` | ✅ 已取（本目录） | agent（2026-09-20） |
| ①a `pages/login/login` 底部入口 | ❌ 未取（登录态不可达） | 需一次**未登录态**批次的 agent；或人退出账号后补 |
| **①b 能力面**（指纹路径 / 真机上传路径） | ⛔ **未签** | **人**给出原文（agent 只可代录、不得自拟、不得写「已通过」） |
| ④c 整模块 Kotlin 编译 | ✅ `errors=0`（见 PR 评论） | agent |
| ④a dev 全量编译 | ✅ 见 PR 评论 | agent |
| ④b release 云打包 | 未跑 | 发布前置条款，发版前由人执行 |
