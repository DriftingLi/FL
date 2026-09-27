# register 页验证码取值迁移 ①a 补做取证 —— #1270（#650 收口批遗留缺口）

- **日期**：2026-09-26 00:20–00:40（设备本地时）
- **执行**：agent 执行（按 `docs/adr/0016-真机门的人工性收缩与按批取证.md`，①a 允许 agent 出证）
- **设备**：`192.168.10.54:40999`（Xiaomi marble / 23049RAD8C，Android 15，1080×2400），屏幕常亮前置
  （`screen_off_timeout=1800000` + `stay_on_while_plugged_in=3`），**登出态**（register 页有
  `auth.isLoggedIn → reLaunch(dashboard)` 登录门控，由维护者手动登出）
- **树**：worktree `wt-1270`，分支 `docs/1270`，HEAD `7c7537b3`（origin/master，含 #1328）
- **取证驱动**：`.scratch/t1270-capture.ps1 -Label <before|after>`（判据脚本 `.scratch/{a11y-triples,slot-check,png-rows}.mjs`
  + `scripts/lib/png-diff.mjs`；留存副本见文末）

## 两轮树形态（本票的实验设计）

| 轮 | 形态 | `api/auth.uts` captcha 出口 | `useRegisterForm.uts` 取值 |
| --- | --- | --- | --- |
| before | **术前**（b102ab3c^ 形态） | `get('/captcha'): Promise<UTSJSONObject>` | `data['id'] as string` / `data['image'] as string` |
| after | **术后**（HEAD = 迁移后） | `getMapped<CaptchaResult>(…, buildCaptcha)` | `data.id` / `data.image` |

**树身份（机检行 `T1270_*_SHA256`）**：
- before：`auth.uts=0B5960C3…`、`useRegisterForm.uts=97CCFE51…`
- after：`auth.uts=D6865DA5…`、`useRegisterForm.uts=0F954FD2…`（= HEAD blob，`git status` 术前脏/术后净）

**与票面配方的两处订正（编译门实测逼出，非自由发挥）**：

1. **术前不能整文件回填 `api/auth.uts`**：`b102ab3c..HEAD` 之间 `c3f20232`（#1320，批 B 收紧）
   也改过该文件 ⇒ `git checkout b102ab3c^ -- api/auth.uts` 会连带回退 #1320 的改动。改为**手术式回填
   captcha exit**（4 行），其余（`CaptchaResult`/`buildCaptcha`/其它 mapped 出口）保持 HEAD。
2. **回填 api 出口必须连带回填全部三个消费方**：第一次术前部署编译红，4 个
   `Assignment type mismatch: actual type is 'Any?', but 'String' was expected`（`useLoginForm.uts:103-104`、
   `useForgotPasswordForm.uts:89-90`；`useRegisterForm.uts` 两行已按票面回填）⇒ `data.id` 在 `UTSJSONObject`
   上是 `Any?`，**诊断回执里「useRegisterForm.uts 路径仍有效、api 保持不动」的写法编译不过**。
   三消费方一起回填后编译通过（术前 180s / 术后 130s，`HX_RUN … exit=ok`）。login/forgot 两文件仅
   为编译自洽回填，**不在本页取证射程内**（login 已由 #1288 闭合）。

部署各一轮（`npm run hx:run -- -Device 192.168.10.54:40999`，均 `deployed=true`：
术前 www mtime `1790333451→1790353296`、术后 `1790353907→1790354120`，pid 均前进）。

## 判据与结果

| 判据 | 结果 |
| --- | --- |
| 页身份（两轮 3 次进入） | `进入页面:pages/register/register`，`创建dom元素个数:48个` —— 两轮同形 |
| **a11y 文本集合 + 节点三元组** | `nodes=27/27`，`rawSha=5e68d977a0984d7f` **两轮逐字节相同**，`tripleSha=a374dc25694682bc` 同，`onlyA=0 onlyB=0 identical=true`，texts `23/23` |
| 跨轮逐行像素（`png-diff --threshold 0.005 --ignore-top-rows 90`） | `different=27090` `ratio=0.010859` `changed=true` |
| **差异归因（`slot-check`/`png-rows`）** | 图位 `[741,1469][999,1607]` 内 `inSlot=27090`、**`outSlot=0`**，bbox `741,1485,998,1589`；行带仅 `y1400-1499:3870` + `y1500-1599:23220` |
| 验证码图确实渲染 | 图位非白 `35260/35604 (99.0%)` —— 两轮同（对 #1269 before 轮「加载失败」形态的盲区补位） |
| 对照 A：无交互连拍（稳定性） | 两轮均 `different=0 changed=false` |
| 对照 B：点图刷新（注入现测） | 坐标锚点 `captcha-input-anchor[851,1538]`（现测 a11y bounds），`input tap` 注入被接受（空输出）；两轮均 `different=27090` 全落图位，**tap 后 ~20s logcat 出现第 2 次 `GET /api/captcha → 200`** |
| 对照 C：同树重新进页（不依赖注入） | 两轮均 `different=27090` 全落图位 |
| logcat（窗口起点 = 轮首设备 epoch） | 两轮均 `FATAL=0 ANR=0`；`captchaLines=6 captcha200=3`（进页 / 点图刷新 / 重进各一次，全部 200） |

**结论**：迁移前后文本集合与三元组逐字节相同；唯一像素差是随机验证码内容区
（`258×105=27090` = bbox 满密度，两次随机抽样必然全不同），与同树点图刷新 / 重进对照**完全同形**
⇒ **register 页验证码取值迁移在真机上行为无差异，取值链路（GET /captcha → 200 → 图渲染 → 可刷新）两轮均成立**
⇒ #650 的 ①a 缺口闭合。

## 产物

| 文件 | 说明 |
| --- | --- |
| `01-register-before-base.jpg` / `02-register-after-base.jpg` | 术前 / 术后首屏基线（720w JPEG q75，≈68 KB） |
| `03-before-refresh-tap.jpg` / `04-after-refresh-tap.jpg` | 对照 B：点图刷新后 |
| `05-before-reentry.jpg` / `06-after-reentry.jpg` | 对照 C：同树重新进页 |
| `a11y-before.xml` / `a11y-after.xml` | 两轮 a11y 原始 dump（**判据输入**，sha 见机检行） |
| `machine-lines.txt` | 两轮机检行原样 + 全部判据原文（复算即重跑该文件内命令） |

原始 1080×2400 PNG 与 logcat 全文（单轮 45 MB）按入库纪律不入仓，留存于本机
`D:\FL\.scratch\archived\issue-1270\t1270\{before,after}\`（含 `register-*.png`、`logcat-*.txt`、
`machine-*.txt`、launch 日志）；取证驱动与判据脚本留存于 `…\issue-1270\t1270\scripts\`。
⚠️ `.scratch/` 属 gitignore ⇒ 该留存仅本机可复算（写实），入仓的 XML + `machine-lines.txt`
已足以复算**文本集合/三元组与全部 sha**；像素复算需要上述本机 PNG。

## 设备侧写操作声明

- 只读：`screencap` / `uiautomator dump` / `dumpsys` / `logcat -d`、`cli launch --pagePath` 导航。
- **唯一的 `input` 注入**：对照 B 的 `input tap 851 1538`（验证码图位，现测坐标；手机号为空，
  即使误触「获取验证码」也只会触发本地校验提示 —— 本车位落在图位、未触达按钮）。
- 未安装/卸载、未 `logcat -c`、未 `adb kill-server`、未 kill HBuilderX/cli、未代填任何凭证。
- 每轮跑完均 `git status` 复核 HBuilderX 回写：`manifest.json` / `pages.json` **未被改脏**。
