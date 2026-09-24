# T13（#651）①a 真机逐页取证 · login 模块页面手术

- 票：https://github.com/DriftingLi/FL/issues/651 · PR：https://github.com/DriftingLi/FL/pull/1288
- 日期：2026-09-23 · 设备：marble / 23049RAD8C（Android，1080×2400）· adb `192.168.0.212:38345`
- 术前树：`origin/master` 的 `pages/login/login.uvue`（977 行口径 / sha256 `FA1FFCA5…61F25`）
  —— 只回填这一个文件；分支新增的两个 composable 在 master 页里不被引用 ⇒ 部署行为等价于 master。
- 术后树：`refactor/login` @ `11bcdbf7`（`login.uvue` 581 行 / sha256 `806C0E0A…C3FD5`）
- 取证驱动：`.scratch/t13-capture.ps1`（**全只读**：不 install / 不 force-stop / 不 `input` / 不 `logcat -c`；
  术前术后走**同一条取证代码路径**，两轮之间只有树上的 `login.uvue` 不同）
- 部署：`npm run hx:run -- -Device 192.168.0.212:38345`（增量，`compile=150s total=184s`，
  `HX_RUN_DEPLOY deployed=true`，设备侧 `www` mtime `1790125218→1790125948`，pid `15906→17473`）

## 为什么是四张图（两态 × 术前术后）

登录页默认 `const mode = ref<string>('phone')`，而**生物识别快捷登录入口只在 password 模式内**
（`showedStored && biometric.isSupported`）。深链 `cli launch --pagePath pages/login/login` 每次都会把
`mode` 重置回 `'phone'` ⇒ 单靠深链**永远采不到**票面要求的「生物识别入口态」。处置：

- **A 态** = 深链进入的默认态（手机验证码 + 随机图形验证码），由 `Invoke-AutoScreenshot` 导航 + 「导航已落定」判据出证；
- **B 态** = 账号密码态（含 `🔒 指纹快捷登录` 入口），由**维护者本人在真机上点一次**「账号密码登录」，
  agent 用 `-NoNavigate` **只截不导航**。B 态目录里没有本轮 launch 日志（本轮没有 launch），
  页身份由 `a11y-*.xml` 的文本/bounds + `mCurrentFocus` / `topResumedActivity` 承载 —— 已在
  `page-entry-*.txt` 内如实标注，不留指向错误轮次的伪证据。

另：本页**有登录门控**（`onLoad` 里 `auth.isLoggedIn.value ⇒ reLaunch(dashboard)`），取证前须由维护者本人
在真机退出登录 —— agent 不索取、不代填任何凭证。

## 结论：正文零漂移

| 对照 | `different`（忽略顶部 90 行状态栏） | 差区 bbox | a11y 原始 XML |
| --- | --- | --- | --- |
| A 术前 ↔ 术后进页 1 | 27090 | `741,1083,998,1187` | **逐字节相同**（`68EC5123…406173`） |
| A 术前 ↔ 术后进页 2 | 27090 | 同上 | 同上 |
| **A 噪声对照**：术后进页 1 ↔ 进页 2（同一棵树两次进页） | **27090** | 同上 | 同上 |
| B 术前 ↔ 术后 | **0** | — | **逐字节相同**（`0B956EEB…F42F4D`） |

那 27,090 个像素**恰好等于图形验证码图片矩形的全部面积**：bbox 宽 `998-741+1 = 258`、高 `1187-1083+1 = 105`，
`258 × 105 = 27090`，且行带计数逐一对上（`y1000-1099: 4386 = 17×258`、`y1100-1199: 22704 = 88×258`）。
三张验证码裁剪件 sha 互不相同（术前 `d6b6a4d6…`、术后进页 1 `7c554a68…`、进页 2 `d1dd1d25…`；内容分别是
`3−3=?` / `8×4=?` / 又一张）⇒ 该矩形每次进页整幅重绘。**同一棵树自己两次进页也差 27090、bbox 与行带逐字相同**
⇒ 这个数与重构无关；矩形之外 2,467,710 个像素零差异。

B 态没有验证码字段，所以 `different=0` 是**逐像素完全相同**（把状态栏计入也只有 `y0-99: 6802`，
即时钟 `9:04→9:30` / 网速读数 / 电量三个系统侧元素；A 态同口径为 `y0-99: 7405`）。

a11y 面（比像素更强的结构面）：A 态三份、B 态两份原始 XML **各自逐字节相同**；归一化三元组
（值 / class / bounds）差 **0** —— A 态 21 节点 / 19 文本，B 态 17 节点 / 15 文本，术前术后一一对应。

崩溃面：五轮 logcat 窗口内 `FATAL EXCEPTION=0` / `ANR in=0`；页身份由应用自己打出的
`进入页面:pages/login/login`（附「创建 dom 元素个数 39 个 / 排版 1 次 / 渲染 1 次 / 跳转页面到 onReady 总耗时 429ms」）
+ `topResumedActivity=ActivityRecord{…} io.dcloud.uniappx/io.dcloud.uniapp.UniAppActivity` 钉住。

## 文件

- `01-login-A-before.jpg` / `02-login-A-after.jpg` — A 态（手机验证码）术前 / 术后，720w JPEG（q82，≈70 KB）
- `03-login-B-before.jpg` / `04-login-B-after.jpg` — B 态（账号密码 + 指纹快捷登录入口）术前 / 术后
- `machine-lines.txt` — 五轮机检行（树身份 sha / PNG sha / a11y sha / logcat 计数 / focus）+ 全部对比判据原文

原始 1080×2400 PNG、a11y XML、logcat 全文**取证时**落在取证 worktree 的 `.ci-verify/t13/<轮次>/`（未入仓：
logcat 单轮 40 MB，且 `.ci-verify/` 是 gitignore 的本地产物面）。该 worktree 与 `refactor/login` 分支已于收尾时删除
⇒ **上面那个路径现在是死链**；删树前已按 `docs/agents/multi-agent-git.md` 的留存纪律把整套判据输入拷到主树
`.scratch/archived/issue-651-t13/t13/`（五轮齐全，含判据脚本 `png-rows.mjs` / `a11y-triples.mjs` 与取证驱动
`t13-capture.ps1`，24 MB，2026-09-24 现测）。原件身份已机检对上：5 份 a11y XML 的 sha256 前缀与
`machine-lines.txt` 的 `rawSha` 逐条一致（`68ec5123c471a1b2` ×3 / `0b956eeb0120d10b` ×2）⇒ 本 README 的
`27090` 等像素读数**可复算**，复算脚本就在上述归档的 `scratch/` 下。
⚠️ 但 `.scratch/` 同属 gitignore ⇒ 留存只在**本机**，换机器即不可复算；a11y XML 每份约 6.5 KB，
远在截图入库纪律射程内，宜随下一次收口入仓（跟踪于 #1270）。

## 设备侧写操作声明

本轮对设备**只有只读操作**（`screencap` / `uiautomator dump` / `dumpsys` / `logcat -d`）与 `cli launch` 导航。
模式切换（点「账号密码登录」）由维护者本人在真机上完成，agent 未注入任何 `input`（本机 `INJECT_EVENTS`
亦被系统硬拒）。未索取、未代填任何凭证；未 `adb kill-server`（adb server 与 HBuilderX 共用）。
