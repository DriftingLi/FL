# #1478 着陆页 ①a 真机取证（2026-10-03 / 10-04）

- **复测对象**：`27c174ad8f41f1de1797f543b70d79342cb0a7d0`（分支 `feat/1478` 当前 head）
- **设备**：Xiaomi `23049RAD8C` / `marble`，adb serial `b32d8398`
- **执行人**：agent 执行（ADR-0016：①a 允许 agent 出证；本票**未命中能力面** ⇒ ①b 免）
- **取证方式**：`adb shell uiautomator dump` 读 a11y 树 + `screencap` 成对取；**agent 核不了截图内容**，故凡「文本是否出现」一律以 a11y 读数为判据、截图只作留痕。

---

## ⚠️ 先读：本轮踩到的三个取证陷阱（不读懂会重跑一遍）

### 陷阱 1：`uni.showToast` 在 a11y 树与 logcat 里**都读不到**
第一次点主 CTA 后我判「点击失效」——**判错了**。真实情况：`input tap` 一直生效，只是
`uni.showToast` 的 toast 既**不进 uiautomator dump**（拍不到 toast 层窗口），**也不走 `console`
通道**（故 logcat 里没有 `[LOG]---BEGIN:CONSOLE---` 行）。我一直在等一个**永远不会出现的信号**。

**正解**：要判「回调是否被触达」，只能在函数入口挂 `console.log` 探针，从 logcat 读。
本轮实测铁证：

```
01:50:36.796 I/console (18565): [LOG]---BEGIN:CONSOLE---[{"type":"string","value":"[probe] onWechatLogin entered agreed="},
{"type":"boolean","value":"true"},{"type":"string","value":"ready="},{"type":"boolean","value":"false"},
{"type":"string","value":" at pages/index/composables/useLandingLogin.uts:85"}]---END:CONSOLE---
```

⇒ `onWechatLogin` **确实被调用**、`agreed=true`（协议门槛已过）、`ready=false`（App 端接通面为假）。

### 陷阱 2：启动页会被**设备侧钉住**
`cli launch --pagePath pages/login/login`（`dev:finish` 自动截图步骤发的）会把启动页钉在设备上，
之后 `am force-stop` + `monkey -p io.dcloud.uniappx -c android.intent.category.LAUNCHER 1`
冷启**仍然落 login 页**，且 `[welcome] restored=` 那行（`index.uvue:82`）根本不打印 —— 即
**`index.uvue` 从未执行**。杀掉残留 `cli.exe` 也无效（钉在设备侧，不是进程侧）。

**正解**：显式深链指定要测的页 ——
`cli.exe launch app-android --project <项目目录> --deviceId b32d8398 --pagePath pages/index/index`。

### 陷阱 3：`cli launch` 进程常驻 + 同一项目**重复导入**
`cli project list` 实测返回两条同名条目（`1 - 叉车维修培训学员端跨端应用(UniApp_VUE)` /
`2 - 同上`）⇒ `dev:finish` 的 hx-run 与 auto-screenshot 各起一次 `launch` **互相抢占**，
落定判据等不到「页面进入行」。实测该行其实**在 32 秒时就出现了**
（`进入页面:pages/index/index`，launch 起于 01:02:49）——远早于 420 秒上限。

---

## 判据逐条读数

### (1) 已登录冷启动直落 dashboard —— **本轮未复测（诚实声明）**

设备当前**未登录**，而 agent **不持凭据、也不得索取或代填**（ADR-0008 夹具条款：为取证造数据须
「由人登录一次」）⇒ 该条**不在本轮取证范围**。文本侧只由 `utils/landingLoginContract.test.js`
的 `coldStartFacts` 静态锁兜底（`reLaunch` 语句在位且未被条件化）。
**待维护者用自己账号点一次**即可闭合。

### (2) App 端允许展示该入口（新口径，取代旧文「App 端渲染文本不含『一键』」）—— ✅

冷启落定后 a11y `content-desc` 读数（12 项，逐字）：

```
欢迎页 | 微信一键登录 | 同意 | 《用户协议》 | 和 | 《用户隐私》 |
未注册的微信号将自动注册账号， | 立即注册 | 其他登录方式 | Hi，您好！ | 欢迎使用叉车维修培训系统
```

`微信一键登录` **在** ⇒ App 端（`uniPlatform="app"`）展示面为真、入口直出。
未登录**停留着陆页**（未误跳 dashboard），与 `pages/index/index.uvue:93` 注释一致。

### (3) 两端入口可见性 —— ✅（App 端侧）

- 主 CTA `微信一键登录` **在**（`bounds=[137,1833][944,1971]`，`clickable="true"`，`enabled="true"`）
- 次级入口 `其他登录方式` **在**（`bounds=[411,2215][669,2273]`）
- 点击「其他登录方式」实测跳转成功：`进入页面:/pages/login/login`

⚠️ **修正 PR 正文一处旧表述**：原写「App 端直出『其他登录方式』」是**错的**——
`<view class="cta-secondary" @click="goOtherLogin">` 上**没有任何端别门禁** ⇒ **两端都渲染**。

小程序端不在本机取证面（② 门本次免）。

### (4) 点主 CTA → 友好提示 + **零请求** —— ✅

链路（`agreed=true` 前提，探针读数见陷阱 1）：

| 环节 | 读数 |
| --- | --- |
| `onWechatLogin` 被调用 | ✅ `[probe] onWechatLogin entered` |
| 协议门槛 | ✅ 已过（`agreed=true`） |
| **接通面** | ✅ **`ready=false`**（App 端）⇒ 走友好提示分支、`return` |
| 发出的网络请求 | ✅ **零**（logcat 无任何 `/auth/` 请求行） |
| 异常栈 | ✅ 无 |

`useLandingLogin.uts:90-93` 的分支即当时命中路径：
`if (wechatLoginReady == false) { uni.showToast({ title: WECHAT_NOT_WIRED_NOTICE, icon: 'none' }); return }`
（`WECHAT_NOT_WIRED_NOTICE = '微信登录暂未开通'`，单点定义在 `composables/useLoginProviders.uts`）。

**未勾协议点 CTA** 的对照组同样成立：协议门槛在接通面门槛**之前**（`useLandingLogin.uts:86-89`），
故未勾选时只出「请先同意用户协议和隐私政策」、同样零请求。

### (5) 登录页三档「立即注册」在位 —— ✅

登 录页 a11y 读数：`叉车维修培训系统 | +86 | 获取验证码 | 6 位数字验证码，5 分钟内有效 | 同意 |
《用户协议》 | 和 | 《用户隐私》 | 若手机号未注册， | 立即注册 | 登 录 | 手机验证码 | | |
邮箱验证码 | 账号密码登录 | 招聘者登录`。

`若手机号未注册，`+`立即注册` 同时在位（手机档）。点「立即注册」实测跳转成功：
`进入页面:/pages/register/register`。

---

## `input tap` 的可靠性对照（本轮副产品，供后续取证参考）

同一台设备、同一轮注入，各元素实测：

| 元素 | 类型 | `input tap` 是否触发回调 |
| --- | --- | --- |
| 着陆页协议勾选行 | `<view @click>` | ✅ 生效（`✓` 出现） |
| 着陆页「其他登录方式」 | `<view @click>` | ✅ 生效（跳转 login） |
| 登录页「立即注册」 | `<text @click.stop>` | ✅ 生效（跳转 register） |
| 着陆页主 CTA「微信一键登录」 | `<button @click>` | ✅ 生效（探针证明；**详见陷阱 1**） |

⇒ 结论：**`input tap` 对各类元素都可靠**。早先判「`<button>` 点不动」是**被陷阱 1 误导的误判**。
凡「点击后无可见变化」的情形，先挂探针确认回调是否触达，别急着归因到元素类型。

---

## 截图产物

原始截图与 dump 落在本 worktree 的 `.ci-verify/`（**未随提交入库**，因 `dev:finish` 的自动截图
步骤在本次运行中失败，见下）：

| 文件 | 说明 |
| --- | --- |
| `.ci-verify/landing-01-index.png` | 冷启落定的着陆页 |
| `.ci-verify/landing-04-agreed.png` | 勾选协议后（`✓` 出现） |
| `.ci-verify/a11y-landing-before.xml` | 判据 2/3 的 a11y 原始树 |
| `.ci-verify/a11y-after-agree.xml` | 勾选后的 a11y 原始树 |
| `.ci-verify/a11y-tap-cta-after-agree.xml` | 点 CTA 后的 a11y 原始树 |

### ⚠️ `dev:finish` 自动截图步骤本轮失败（与产品代码无关）

`[6/9] 自动截图` 两张都报「导航未在 420 秒内落定」⇒ `❌ 没有任何页面截图成功` ⇒ 整条命令 exit 1。
根因见上文**陷阱 3**（`cli launch` 常驻 + 项目重复导入导致两次 launch 抢占）。
第 3–5 步（单测 / ④c 编译门 / 真机部署）**全部通过**，部署实测耗时 457s、页面真起来了。
