# #1194 招聘者身份面 —— ①a 真机只读取证记录（PR #1206）

> 归档时间：2026-09-20 15:24–15:25 ｜ 分支：`feat/mobile-1194-recruiter-identity` ｜ 取证时 HEAD：`5157c844`
> **第二归档时间（本轮收口批：回学员端出口的两帧）：2026-09-20 20:43–20:53 ｜ 取证时 HEAD：`3a1c1c50`**
> 设备：`b32d8398`（Redmi `23049RAD8C`，1080×2400）｜ 工具链：HBuilderX 5.23.2026080626（`E:\HBuilderX.5.23.2026080626\HBuilderX`）+ adb 1.0.41
> （本轮复读：`hx-run.ps1` 实际探测到的主程序安装位置是 `D:\软件\HBuilderX.5.23.2026080626\HBuilderX`，编译器 5.24。上行 `E:\…` 是第一批当时的记录，**不改写历史**。）
> 口径：`training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`「①a 取证手法补遗」+ `0016-真机门的人工性收缩与按批取证.md`
> **①a 由 agent 出证；本文件不含也不替代 ①b**（P1 命中能力面：`pages/login/login.uvue` 指纹白名单 / `api/request.uts` 真机上传白名单 ⇒ **①b 必做、由人给出原文**）。

## 一、结论：①a **两批合计覆盖**（第一批 1/2 页；本收口批补齐「出口」两帧）

| # | 目标页 | 结果 | 依据 |
| --- | --- | --- | --- |
| 1 | `pages/recruiter/login`（独立登录页） | ✅ **截图入仓** `01-recruiter-login-after.jpg` | 导航落定 281s；日志 `进入页面:pages/recruiter/login` |
| 1b | 同上：**身份互斥**提示 | ✅ **实拍到**「退出当前账号 / 登录招聘者账号将退出当前学员账号」模态 | 见下第三节；对应 `pages/recruiter/login.uvue:61-74` |
| 2 | `pages/login/login` 底部纯文字入口（「招聘者登录」/「账号密码登录」） | ✅ **本轮补齐** `04-back-to-student-login.jpg` | 第一批因在册学员会话自跳拿不到（见第四节）；本轮以「点出口后的落地页」形态取到，a11y 树含 `账号密码登录` / `招聘者登录` |
| 3 | `pages/recruiter/login` 的**回学员端出口**（本轮新增，纯文字「返回学员登录」） | ✅ **截图入仓** `03-recruiter-login-student-exit.jpg` | a11y 树 = `返回 \| 招聘者登录 \| 显示 \| 登 录 \| 返回学员登录`；页身份 `20:52:16.649 进入页面:pages/recruiter/login` |
| 4 | **点该出口之后的落地页**（死胡同是否已解） | ✅ **截图入仓** `04-back-to-student-login.jpg` | 点后 a11y 树 = 学员登录页（出口文案消失）、页身份 `20:52:26.822 进入页面:/pages/login/login`，且**其间没有 index 重新入页** ⇒ 是**页内 reLaunch** 而非 App 重启 |

机检行（`scripts/device-capture.ps1`，**只读**，`DEVICE_CAPTURE_RESULT=` 是唯一判据）：

```
DEVICE_CAPTURE_RESULT=PASS
  页面 pages/recruiter/login=SKIP  前台=io.dcloud.uniappx/io.dcloud.uniapp.appframe.activity.UniPortraitPageActivity
  logcat窗口行数=442  FATAL=0  ANRin包=0
```

本轮全窗口（`09-20 20:47` 起，83302 行）另读数：`FATAL EXCEPTION=0` / `E AndroidRuntime=0` / `ANR in 包=0` / `INJECT_EVENTS=0`
（原文与逐条读数见 `06-machine-check-and-readings.txt`）。

只读保证：全程只用 `adb devices` / `exec-out screencap -p` / `shell dumpsys activity` / `shell logcat -d`；
**未** kill-server、未 install/uninstall、未 force-stop、未清 logcat、未 push/pull/reboot、未注入 `input`。

## 二、逐页取证产物

| 文件 | 内容 |
| --- | --- |
| `01-recruiter-login-after.jpg` | `pages/recruiter/login` 实拍（1080×2400 → 720×1600，JPEG q80，45 800 B）；原图 sha256 `d07f4e4d3842c1e489d58f7566bba404c86446dfd69666e3802f913c8935e3ae` |
| `02-page-identity-launch-log.txt` | **页身份**证据：`--pagePath` 请求页 vs 应用日志「进入页面」行（`dumpsys` 只能给到 Activity/包名，uniapp 全页面同一个 Activity ⇒ 这是唯一设备侧页身份证据） |
| `03-device-capture-readonly.txt` | 只读取证日志全文（含上面的机检行、前台 Activity 原始行、logcat 窗口统计） |
| `04-kotlin-all-4c.txt` | ④c 整模块编译日志（`KOTLIN_ALL_RESULT errors=0 classes=1407 files=109`）；④c 的门证据由脚本贴的 sha 绑定 PR 评论承载 |
| `03-recruiter-login-student-exit.jpg` | **本轮**：`pages/recruiter/login` 显示新出口「返回学员登录」实拍（1080×2400 → 720×1600，JPEG q80，38 568 B）；原图 sha256 `d951a5e85f368080af7284f3aa01ed9474a17d9b16567ccb9ac83fdd814ab5b2` |
| `04-back-to-student-login.jpg` | **本轮**：点该出口后的落地页 = `pages/login/login`（学员登录页；底部可见「账号密码登录」/「招聘者登录」）（720×1600，JPEG q80，69 238 B）；原图 sha256 `fbd26d20a8fde3460cffabe2a5090216a73f8ef2985fbad11f4ed63b80544429` |
| `05-exit-capture-readings.txt` | **本轮**：会话式取证全过程读数（轮询 descs、帧1/帧2 的 a11y 树、出口节点 `bounds`、**唯一一次 `input tap`** 的命令与 `TAP_RC`、注入可用性判据、页身份时间线） |
| `06-machine-check-and-readings.txt` | **本轮**：机检行原文 + 全窗口崩溃/ANR/注入判据 + 「pre-tap 身份槽**未取到**」的只读探测读数与结论边界 |

复现：

```powershell
cd D:\wt-1194\training-app\叉车维修培训学员端跨端应用
npm run dev:finish -- -Level standard -Device b32d8398 -MaxScreenshotPages 8   # 步骤 4-6：④c → 部署 → 逐页 --pagePath 截图
pwsh -NoProfile -File scripts/device-capture.ps1 -Device b32d8398 -Pages "pages/recruiter/login" -Module recruiter-login
# 本轮「出口两帧」的会话式取证（脚本在 .ci-verify/，含唯一一次 input tap）：
pwsh -NoProfile -ExecutionPolicy Bypass -File .ci-verify\exit-1206\capture.ps1
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
| ①a `pages/recruiter/login` | ✅ 已取（`01-…`；本轮另取出口帧 `03-…`） | agent（2026-09-20） |
| ①a `pages/login/login` 底部入口 | ✅ **本轮补齐**（`04-back-to-student-login.jpg`，以「点出口后落地页」形态取得，同帧可见底部两个入口） | agent（2026-09-20 收口批） |
| ①a 出口两帧 + 唯一一次 `input tap` | ✅ 已取（`03-…` / `04-…` / `05-…` / `06-…`） | agent（2026-09-20 收口批，依据见第七节） |
| **pre-tap `auth_active_role`（判「清槽支是否运行」）** | ⛔ **未取到**（只读口径判不出，见 `06-…` 第 3 节） | 不再补：该判据由 R8/R9/R5/R7 承担，真机读数不是它的唯一证据面 |
| **①b 能力面**（指纹路径 / 真机上传路径） | ✅ **已由人签**（2026-09-20，原文由人给出、agent 代录，见 PR 正文 ①b 行） | 人（zhengcookie） |
| ④c 整模块 Kotlin 编译 | ✅ 见 PR 评论（sha 绑定最终 HEAD） | agent |
| ④a dev 全量编译 | ✅ 见 PR 评论（sha 绑定最终 HEAD；**三条件**：`errors=0` + 手工 grep 零命中 + 正向「编译成功」） | agent |
| ④b release 云打包 | 未跑 | 发布前置条款，发版前由人执行 |

## 六、`pages/login/login` 的补拍是怎么来的（对第四节结论的**更新**，不改写第四节的当时事实）

第四节记的是**第一批（15:24）**的事实：持有在册学员会话时该页会自跳 dashboard ⇒ 选页失败。**本轮（20:52）**设备上**没有**在册会话
（`pages/login/login` 未自跳），于是同一页以「**点掉招聘者登录页出口之后的落地页**」形态取到，且同帧可见底部两个入口。
⇒ 第四节那段「未取得真机证据」的结论**只对第一批成立**，已由本节与第一节表格更新；原文保留（记录不改写）。

## 七、本轮为什么动用了 `input tap`（依据 + 现测读数 + 两帧路径）

**依据（原文）**：`docs/adr/0008-移动端验收门与证据.md` ①a 取证手法补遗第 3 条 ——
「`input` 能不能注入要现测 …… **2026-09-17 补第三个数据点（换到另一台设备）**：`b32d8398`（Xiaomi `23049RAD8C` / Android 15）上
`input tap` 与 `input swipe` 同样可用 …… **会话式 ①a 取证应把 `input` 视为可用工具、每次现测即可**」，
并明确「`device-capture.ps1` 内**仍然禁 `input`** …… 本节说的是**会话式人工取证**在『设备归本次取证专用』时可以动用 `input`，**不是放宽脚本红线**」。
本轮由维护者声明**设备归本次取证专用**并授权**一次**点按。

**现测读数（一次点按，两用）**：

```
uiautomator：<node … content-desc="返回学员登录" … bounds="[429,2259][651,2313]" />   ⇒ 中心 (540,2286)
$ adb -s b32d8398 shell "input tap 540 2286; echo TAP_RC=$?"      → TAP_RC=0
logcat（09-20 20:47 起，83302 行）：INJECT_EVENTS=0    注入被拒相关行=0
```

**两帧路径（机检 + 文本成对）**：

| 帧 | 路径 | 页身份（设备日志） | a11y 文本判据 |
| --- | --- | --- | --- |
| 帧1 `03-recruiter-login-student-exit.jpg` | `hx:run` 部署 → `cli launch --pagePath pages/recruiter/login`（`Start-NavLaunchDetached`）→ 轮询到 a11y 出现「返回学员登录」后截图 | `20:52:16.649 进入页面:pages/recruiter/login` | `返回 \| 招聘者登录 \| 显示 \| 登 录 \| 返回学员登录` |
| 帧2 `04-back-to-student-login.jpg` | 上帧同一次运行内：`input tap 540 2286` → 5s 后截图 | `20:52:26.822 进入页面:/pages/login/login` | `… 获取验证码 \| 账号密码登录 \| 招聘者登录`（出口文案**消失**） |

**为什么这算「死胡同已解」而不是「碰巧跳走」**：① 帧1 与帧2 之间**没有** `pages/index/index` 入页行 ⇒ 不是 App 重启，而是**页内跳转**；
② 目标页正是 `leaveRecruiterLogin()` 栈空分支的 `uni.reLaunch({ url: '/pages/login/login' })`；
③ 与当初的复现形态同形（守卫/401 用 `reLaunch` 进本页 ⇒ 栈空 ⇒ `navigateBack` 无效）。

**本轮没取到的**：点按那一刻 `auth_active_role` 的真值（因而**不能**由真机读数断言「清槽支执行了」）——
只读探测及其边界见 `06-machine-check-and-readings.txt` 第 3 节，**如实记为未取到，不给推测**。
