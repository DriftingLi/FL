# uvue 文字类样式落在 `<view>` 上被静默忽略 —— 本票改动点修复 + 全仓机检落锁（#1269）· ①a Android 真机术前/术后对照取证

- **票**：#1269（同族先例 #1113 / `white-space`，PR #1149）
- **PR**：本目录**先按票号取证**；开 PR 后按仓内约定改名为 PR 号（改名只动 `docs/verification/**` ⇒ 非运行时面，不影响 ①a 证据的 sha 绑定）
- **分支 / 复测对象**：`fix/1269`，运行时面落在提交 **`e35ede17`**（本目录的截图/日志即对这一棵树取证）。取证时的两个 `.uvue` 内容 sha256 前 20 位：`forgot-password.uvue` `c7aa27c9e90af3f3f848`、`register.uvue` `5a05b273dd7aaf7ec580`（二者**工作区与 git blob 同 sha**，已核）；术前对照 = 二者 `HEAD` 版本 `f78f1c220b885343c076` / `15aff48366f96ecbddeb`
- **日期**：2026-09-24
- **设备**：`192.168.0.212:37611`（Xiaomi `23049RAD8C` / Android 15，**无线** adb）
- **屏 / 密度**：`1080x2400` · density 440 ⇒ uvue 的 750rpx 基准下 **1rpx = 1.44px**，`28rpx` 的设计值应为 **40.32px**（本目录用它反推字号是否真的生效）
- **包名**：`io.dcloud.uniappx`（uni-app-x 调试基座）
- **执行人**：agent 执行（本目录只承载 **①a**。**①b 未命中能力面** —— 本票改动是模板结构与 CSS 声明，不含指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互，故无人工签收项。**② 免** —— 未命中 MP-WEIXIN 面：改动集无 `manifest.json` / `platformConfig.json`，`.uvue` 的 diff 增删行里也没有 `#ifdef MP-WEIXIN` 系指令行，判据见根 `AGENTS.md` 与移动端 `docs/adr/0008` 的「② 触发判据的可机检口径」）

## 被验的改动（本票全部运行时面）

票面点名 `.mode-tab` / `.mode-tab-active`：uvue 原生端只把 `font-size` / `color` / `text-align` 认给 `<text>|<button>|<input>|<textarea>`（`font-weight` 另多一个 `<loading>`），落在 `<view>` 上会被渲染层**判错并忽略** ⇒ 设计稿的 `28rpx / #666666` 与选中态 `#2979ff + bold` 在 Android 端**从未生效过**，且**每进一次页打 3 条 error**。修法取「**容器与文字分成两位**」（票面「二选一」里的第二项）：`<view>` 只留 padding / background / border，文字挪进 `<text>` 子元素并让那四条声明跟到 `<text>` 的 class 上。

| # | 文件 | 改前 | 改后 |
| --- | --- | --- | --- |
| 1 | `pages/forgot-password/forgot-password.uvue` | `<view class="mode-tab …">手机号找回</view>`，`.mode-tab`{`font-size`,`color`}、`.mode-tab-active`{`color`,`font-weight`} 挂在 `<view>` 上 | 内嵌 `<text class="mode-tab-text" :class="{mode-tab-text-active:…}">`；新增 `.mode-tab-text`{`font-size:28rpx`,`color:#666666`} 与 `.mode-tab-text-active`{`color:#2979ff`,`font-weight:bold`} |
| 2 | `pages/register/register.uvue` | 同上（`手机号注册` / `邮箱注册` 两个 tab，绑 `reg.mode`） | 同上（同形，选择器前缀 `mode-tab-text`） |

两页各 **2 个 tab** ⇒ 本票修掉 **4 个违规位**（机检视角：4 个「class → 非法承载」映射位）。**声明的字面值一条未改**（`28rpx` / `#666666` / `#2979ff` / `bold` 原样搬走），改的只是**落在哪个元素上**。

全仓其余 **5 处同族违规**（ai-chat 两个 picker 组件 4 条、招募详情加载态 1 条）按票面「本票不顺手扩大范围」登记进守护的 `DEFERRED` 自净清单，见下节。

## 新增的全仓机检（票面「考虑」项 → 已落）

`utils/uvueFontCarrierContract.test.js` —— 30 用例，形态沿用本仓既有全仓守护（`uvueWhiteSpaceContract` / `gradientSyntaxContract`）：纯函数读源码文本 + **注入违规自检**（防空跑假绿）+ **合规样本**（防假红）+ 全仓断言 + 活性断言；helper 自带重复、不抽公共模块（与全部先例一致）。

- **判据**：一条 `<style>` 规则里的文字类样式，其选择器**最后一个 compound** 的每个 class，在**模板里**的所有承载标签都必须在白名单内。同一 class 既挂合法又挂非法 ⇒ 仍判违规（非法那一位的声明同样是死的）。
- **只锁有真机日志判据的四个属性**：`font-size` / `color` / `text-align` / `font-weight`。`line-height` / `font-family` **不锁** —— 仓内没有它们被判错的日志，写了就是替渲染器编规则（实测：把这两个加进判定，现扫违规**一条不增**）。
- **`DEFERRED` 不是豁免清单**：上界（不在清单里的新违规一律判红 ⇒ 新页面加不出第 6 处）+ 自净（条目对应违规一旦消失，「每条仍命中」的断言立刻判红，逼删条目）⇒ 清单只允许变短。
- **具名盲区**：`class` 在模板里找不到承载（全局样式 / `.uts` 常量动态挂的 class）**不判** —— 现测全仓 1577 个 occurrence 里 71 个属此类，判它们会 mass 假红；代价由 ③ 节的「已判定数 ≥ `JUDGED_MIN`=1200」兜住。`App.uvue`（应用根组件，按设计无 `<template>`）是另一条静默盲区，具名登记在 `NO_TEMPLATE_OK` 并由 ③ 节按**相等**断言 —— 新增第二处无模板文件必须先改那张清单。该文件的三条带文字类样式的 class 现测使用点全在合法承载上 ⇒ 这条盲区目前**无已知受害位**。
- **分类 = [接线] 守护**（`node scripts/classify-guards.mjs` 2026-09-24 实测：行为 **30** · 接线 **99** · 合计 129）。按 `docs/agents/guards.md`，接线守护**不构成 ③ 门证据** ⇒ 本票这条接线的**行为兜底是 ①a 真机门本身**（本目录 §「术前/术后对照」），不是某个单测。守护自己的判别力另有**两层**红控制：套件内注入样本必红（① 节）+ **术前真实树上必红**（见下文「红控制」）。

## 取证手法

- **部署**：`npm run hx:run`（真运行）→ 机检行；判据是**设备侧事实相对基线前进**，不是「前台是不是基座」。术后 `www=1790221428->1790227845`、术前 `1790228357->1790228910`，各配一次 `HX_RUN … exit=ok`。逐字见 `machine-lines.txt` §1。
  - 术前第一轮撞锁：`HX_BUSY wait=120 result=timeout`（HBuilderX 锁被另一会话占用）⇒ **未强杀任何进程**，改用 `-WaitSeconds 600` 重跑即 `result=free`。
- **进页**：`.scratch/1269-capture.ps1` 复用 `scripts/lib/auto-screenshot.ps1` 的 `Start-NavLaunchDetached` + `Wait-NavSettled`，**`--pagePath` 深链直达**（不经首页导航），4 次导航**全部落定**（`settled=True`，相邻帧差异 0% ≤ 0.5%），页身份逐次命中请求页。
- **截页**：`adb exec-out screencap -p`（经 `cmd.exe /c` 重定向；PowerShell 的 `>` 会破坏二进制）→ PNG 1080×2400，转 **JPEG q75 / 宽 720** 入库（每张 63,402–68,441 字节，≤ 150 KB 上限）；每张旁配一份 `<同名>.content-desc.txt` —— uvue 的文字在 a11y 树里走 **`content-desc`** 不走 `text`。
- **两个状态**：默认态（手机号 tab 选中）+ `input tap` 第二标签后的态（邮箱 tab 选中）。坐标取 dump 的 bounds 中心。
- **术前对照怎么来的**：把两个 `.uvue` **还原成 `HEAD` 版本**（`.scratch/1269-before/`）→ 重编译 → 重部署 → 同一脚本同一参数再跑一轮 → 把术后版本从 `.scratch/1269-fixed-backup/` 放回。**同一台设备、同一密度、同一会话序列**，唯一变量是那 4 个承载位。

## 术前 / 术后对照（本目录的核心判据）

### A. 渲染层 error 行：术前每页 **3 条** → 术后 **0 条**

逐字原文在 `machine-lines.txt` §3，原始四份日志已随目录入库（`console-logs/*.nav.txt`）。术后两页 `is only supported on` **零命中**；术前 3 条点名的 class 恰是改动位：

```
13:52:12.225 style property `font-size|color` is only supported on `<text>|<button>|<input>|<textarea>`. there is an error on `<view class="mode-tab mode-tab-active">`.
13:52:12.232 style property `font-weight` is only supported on `<text>|<button>|<input>|<textarea>|<loading>`. there is an error on `<view class="mode-tab mode-tab-active">`.
13:52:12.238 style property `font-size|color` is only supported on `<text>|<button>|<input>|<textarea>`. there is an error on `<view class="mode-tab">`.
```

| 日志 | 行数 | `style property` 错行 | FAIL | Uncaught | Exception | TypeError | 页身份 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `01-forgot-password-before` | 297 | **3** | 0 | 0 | 0 | 0 | `pages/forgot-password/forgot-password` ✓ |
| `03-forgot-password-after` | 324 | **0** | 0 | 0 | 0 | 0 | 同上 ✓ |
| `05-register-before` | 309 | **3** | 1 | 0 | 0 | 0 | `pages/register/register` ✓ |
| `07-register-after` | 312 | **0** | 0 | 0 | 0 | 0 | 同上 ✓ |

### B. 字号「从被忽略变为生效」是**可测的**，不靠观感

术前/术后共 8 份 a11y dump（两页 × 两态 × 两轮，`before-*/after-*.content-desc.txt`），两个 tab 文字节点的 bounds：

| 节点 | 术前 | 术后 | 宽 | 高 |
| --- | --- | --- | --- | --- |
| `手机号找回` / `手机号注册` | `[258,798][478,857]` | `[276,798][476,852]` | 220 → 200（−20px） | 59 → 54（−5px） |
| `邮箱找回` / `邮箱注册` | `[645,798][821,857]` | `[643,798][803,852]` | 176 → 160（−16px） | 59 → 54（−5px） |

单字宽：术前 `220/5 = 44px` ⇒ `44/1.44 ≈ 30.6rpx`（**声明被忽略、落到默认字号**）；术后 `200/5 = 40px` ⇒ `40/1.44 ≈ 27.8rpx`（**即声明的 28rpx**，`28×1.44=40.32px` 取整 40px）。两页四位的 bounds **逐字节同形** ⇒ 一次改动、两处模板同一组件族，行为一致。

**不变量（同一轮 dump 里的对照）**：DOM 元素个数术前术后**相同**（forgot-password 37 个 / register 48 个）；页眉副标题 bounds 完全未动（如 `手机号注册，验证码通过后自动登录` 两轮都是 `[268,649][812,695]`）⇒ 结构未变、其它文字层未受影响，变的只有这两个 tab 的文字与其下方 **5px 让位**。

### C. 视觉确认（留给签收人，agent 不据此宣称）

术后 `03-…-after-default.jpg` / `07-…-after-default.jpg` 里选中的那个 tab 呈 **蓝字 + 加粗**、未选中呈灰字；术前同页两态字色字重**看不出差别**（正是「`color`/`font-weight` 被忽略」的应有表现）。像素级 diff 结论见「诚实声明」第 3 条。

## 像素差归因（把「整屏差 6.4%」拆开算清，而不是含糊带过）

用仓内既有工具 `scripts/lib/png-diff.mjs` 的解码器逐带统计（脚本：`.scratch/1269-rowattr3.mjs`）。每格 = **该带内差异像素占该带总像素的 %**；`off=-5` 是「把术后帧整体上移 5px 再比」——若该带的差异只是刚性让位，这个数会塌下去：

| 带（术前 y 坐标） | forgot-password `off=0` | 同左 `off=-5` | register `off=0` | 同左 `off=-5` |
| --- | --- | --- | --- | --- |
| 头部 / 导航 0–780 | 0.91% | 4.84% ↑ | 0.90% | 4.77% ↑ |
| **TAB 带 780–870** | **16.47%** | 18.31% ↑ | **16.11%** | 17.77% ↑ |
| 表单 870–1130 | 9.51% | **0.31%** | 7.87% | **0.29%** |
| 验证码行 1133–1237 | 27.18% | **23.90%（未塌）** | 6.83% | **0.01%** |
| 获取验证码 1240–1440 | 11.19% | **0.11%** | 11.24% | **0.08%** |
| 下半屏 1440–2400 | 5.80% | **0.02%** | 10.18% | 2.66% |
| tab 行以下整体 870–2400 | 8.63% | **1.72%** | 9.66% | **1.73%** |

**读法（四条，每条都有上表的数支撑）**：

1. **5px 是测出来的，不是猜的**：偏移量扫描在 870–1280 带上给出 `off=-3 → 10.58%`、`-4 → 8.99%`、**`-5 → 6.34%`**、`-6 → 8.99%`、`0 → 14.19%` —— 极小值**正好落在 −5**。
2. **tab 行以下是刚性让位**：表单带、获取验证码带、下半屏带在 `off=-5` 下分别塌到 **0.31% / 0.11% / 0.02%** ⇒ 除了整体上移 5px，内容逐像素相同。头部带 `off=0` 只有 0.91% 且加偏移后**变差** ⇒ 让位确实从 tab 行下方才开始。
3. **唯一未塌的带是验证码行（23.90%），成因已定位为本票无关的随机图片**：把该带两侧各裁出来入库对照（`10-forgot-password-captcha-band-before.png` / `11-…-after.png`），术前图是 **`9 7 ?`**、术后是 **`8 6 ?`** —— 图形验证码每次请求随机重画；同一带里的 `图形验证码` 占位文字在两图里**逐字相同**（差异只落在右侧图片框内）。
4. **register 下半屏残留 2.66%** 对应术前独有的 `加载失败 点击重试` 节点（术前那次验证码接口 FAIL，见 §A 表），术后该节点不存在 ⇒ 属 §「产物清单」下方的差异声明，不属本票改动。

⇒ 本票**主张**的变化面只有一处：**TAB 带内那两个文字节点的字号/字色/字重真的生效了**（bounds 见 §B，视觉见 §C）；其余全部差异要么由测得的 5px 让位解释，要么由随机验证码解释。整屏原始比值 6.4%（forgot-password 默认态 166,295 / 2,592,000）与 7.1%（register 默认态 182,844 / 2,592,000）**不代表 UI 回归**。

## 红控制（成对取证：只跑通过的那一次不算验收）

术前树（把两个 `.uvue` 换回 `HEAD`）上跑三份相关套件 ⇒ **7 红**（`Tests: 7 failed, 120 passed, 127 total`；`Test Suites: 3 failed, 3 total`），逐字见 `machine-lines.txt` §7：

- **新守护 3 红**：③ 节的「改动点映射」「两页零违规」「除已知存量外全仓零违规」—— 后两条的失败消息**点名 4 个违规位的 `文件:行` + 选择器 + 具体死掉的声明**，不是泛泛「违规」；
- **两页各自的 `<style>` 逐字节冻结锁 4 红**（`forgotPasswordContract` / `registerContract` 的 sha256 锁 + 规则数守恒 + 判断力断言）⇒ 已按术后实况重定基线（`forgot-password` 规则数 43→45、判断力 42→44；`register` 37→39、36→38）。

⇒ 「术后全绿」不是恒真：同一套断言在**未修的树上必须红**，且红的文案可执行。

## 门结论（③ / ④）

- **③ `npm run test:unit`**：全量 **129 suites / 2474 tests 全绿**（`Time: 160s`）。三份直接相关套件单跑 **127 tests 全绿**（`uvueFontCarrierContract` 30 + 两页 contract 锁）。
- **④c `npm run build:kotlin-all`**：`KOTLIN_ALL_RESULT errors=0 classes=1501 files=120 input=unpackage\resources\app-android freshness=fresh log=D:\FL\wt-1269\training-app\叉车维修培训学员端跨端应用\.ci-verify\kotlin-all.log`。原文入库为本目录 `09-kotlin-all-4c.txt`。根 `.gitignore` 有 `*.log` ⇒ 按仓内先例改名入库，正文里仍保留 `.ci-verify/kotlin-all.log` 字面量供 ④ 判据匹配。
  - **该副本的行尾被 git 归一了，如实记**：源日志 11,507 字节（CRLF），入库 blob 11,394 字节（LF），差 113 个行尾；实测 `源.replace(CRLF,LF) == blob` 为真 ⇒ **内容逐行一致、只有行尾不同**。源日志 sha256 前 16 位 `fc7abda91051ac31`，**入库 blob** sha256 前 16 位 `e47a64b45bcc5cba`（先例 `docs/verification/recruiter-login/1206/04-kotlin-all-4c.txt` 同样是 LF 化的 blob，本目录不例外，但把两个 sha 都写出来，免得有人拿工作区 sha 去对 GitHub 上的）。
- **②**：本票**未命中触发面 ⇒ 免**（理由见页眉）。另跑了一次 `build:mp-weixin-check` 作为**补充**观察，其结论**不进**本票的门判据 —— 且**不要**把它当成本票改动的证据：那次运行的导航被降级，被改两页未被访问（详见「诚实声明」第 2 条）。

## 诚实声明（本目录**没有**验证到的面）

1. **H5 端未验**。本票判据的对象是 **uvue 原生渲染器**；术后声明落到了合法承载 ⇒ H5 端本就无视该限制、理论上等价，但本次**没在 H5 上实测**，不宣称。
2. **② 免的依据是「未命中触发面」，不是「跑过无报错」**。本次另跑了一次 `build:mp-weixin-check` 作补充，结果行 `MP_WEIXIN_RESULT … errorsTotal=0 exceptionsTotal=0 navigation=skip(unsupported) ide=2.02.2608070 sdk=3.16.3`、`assertions` 里 `allRoutesVisited` / `screenshotsProduced` 均为 `skip` ⇒ **本机自动化导航不可用，被改的两页在 ② 里根本没被访问到**（`skippedSteps` 点名 `page-1:pages/forgot-password/forgot-password`、`page-2:pages/register/register`，只落在入口页 `pages/index/index`）。所以那次运行**不构成**本票改动的任何证据，只是环境侧的一条附带观察（含一条有用的：端口 9420 被残留会话占用时脚本按 #1302 的处置自动换到 9421）。
3. **不主张「UI 像素级不变」**，且**逐带算清了差异的出处**（见下节「像素差归因」）。结论是：tab 行以下是**一次刚性的 5px 上移**（把 B 整体上移 5px 后差异塌到 ≤0.31%），真正变化的只有 tab 带本身与一处**随机验证码图片**。
4. **点击生效性不取 `TAP_RC`**。脚本里 `echo TAP_RC=$?` 的 `$?` 被 PowerShell 吃掉，回显的 `TAP_RC=True` 是**宿主**的状态量、不是设备退出码 ⇒ 判「点了有效」改取三条：点击后 content-desc 条数变化、两张 PNG sha 不同、选动态 bounds 位移。
5. **未跑 logcat 全缓冲崩溃断言**（#1149 跑过 `LOGCAT_RESULT fatal_exception=0 anr_in=0`）。本票改动不含逻辑与原生能力调用 ⇒ 未做；日志面的 `Uncaught`/`Exception`/`TypeError` 零命中只说明**这几个词**零命中，不等于「无任何运行时错误」。
6. **`5 处 DEFERRED 存量未上机**：本票没动那些文件，它们由守护的登记表 + 自净断言覆盖，真机表现（那些页现在也在打同族 error 行）留给各自收口的 PR 取证。
7. **`App.uvue` 的全局样式在本守护射程外**（无 `<template>` ⇒ 规则一条不判，具名登记）。跨文件判「同名 class 在 A 页挂 `<button>`、B 页挂 `<view>`」**故意没做** —— 那需要先把「全局样式参与渲染层承载校验」这个前提在真机上证成，而 #650/#1263 与本轮的全部错误行都由**页面局部样式**触发，没有一条支持该前提。
8. **截图内容 agent 不据此宣称已核对**：可核的是上面 `content-desc` 的 bounds 判据与日志判据；截图的视觉判断留给签收人。

## 产物清单

| 文件 | 内容 |
| --- | --- |
| `01…08-*.jpg` | 8 张术前/术后 × 两页 × 两态截图（JPEG q75 / 宽 720，63,402–68,441 字节） |
| `before-*/after-*.content-desc.txt` | 8 份 a11y 文本 + bounds 判据（文字在 `content-desc`，不在 `text`） |
| `console-logs/0{1,3,5,7}-*.nav.txt` | 4 份原始 launch 日志（297 / 324 / 309 / 312 行） |
| `console-logs/SUMMARY.txt` | 机检摘要表 + 主张/不主张清单 |
| `machine-lines.txt` | 全部机检行逐字汇编（§1 部署 / §2 页身份 / §3 错行 / §4 错误类计数 / §5 截图落盘 / §6 bounds / §7 红控制 / §8 ④c） |
| `09-kotlin-all-4c.txt` | ④c 编译日志原文（内容同 `.ci-verify/kotlin-all.log`，行尾被 git 归一为 LF ⇒ 见「门结论」的两个 sha） |
| `10-/11-forgot-password-captcha-band-{before,after}.png` | 验证码行 1133–1237 的放大裁切（各 1020×220，≈13.5 KB）—— 用来独立复核「未塌的那一带 = 随机验证码图」这条归因 |

> **编码照实记**：`.scratch/` 下的取证日志（`1269-hxrun-*.log`、`1269-capture-*.log`）由 PowerShell 以 **GBK 控制台编码**写出，含中文的字段（`reason=`、`settled=True reason=`）按 UTF-8 读是乱码、按 GBK 读通顺。`machine-lines.txt` 里的这些行是**按 GBK 解码后抄出**的；ASCII 判据字段（`deployed` / mtime / `compile=` / `exit=` / sha）与按字节读取完全一致。

## 复现命令

> **`.scratch/` 与 `.ci-verify/` 都被 gitignore**（根 `.gitignore:81` 与移动端 `.gitignore:10`）⇒ 下文点名的 `1269-capture.ps1` / `1269-rowattr3.mjs` 是**本机会话产物、不在仓库里**。本目录入库的 26 个文件（截图 / dump / 四份原始日志 / `machine-lines.txt`）才是可复核面；复现命令按「原样可重跑」写全，不依赖那两个脚本存在。

```powershell
# 0) 术前/术后各一轮：换树 → 部署 → 取证（同一台设备、同一脚本、同一参数）
#    术后 = 工作树现状；术前 = 把两个 .uvue 还原成 HEAD
git -C D:\FL\wt-1269 show "HEAD:training-app/叉车维修培训学员端跨端应用/pages/register/register.uvue" > .scratch\1269-before\register.uvue
npm run hx:run                 # 看 HX_RUN_DEPLOY deployed=true … mtime 相对基线前进 + HX_RUN … exit=ok
pwsh -NoProfile -ExecutionPolicy Bypass -File .scratch\1269-capture.ps1 -Tag after    # 或 -Tag before

# 1) 红控制：在术前树上跑三份相关套件，必红 7 条
npx jest --config jest.config.unit.js utils/uvueFontCarrierContract.test.js utils/forgotPasswordContract.test.js utils/registerContract.test.js

# 2) 收口：把术后版本放回，跑 ③ 全量与 ④c
npx jest --config jest.config.unit.js          # 期望 129 suites / 2474 tests 全绿
npm run build:kotlin-all                       # 读 .ci-verify/kotlin-all.log 的 KOTLIN_ALL_RESULT 行（勿用管道退出码）

# 3) 守护分类（写 PR 正文的 guards.md 三问要用）
node scripts/classify-guards.mjs               # 现测：行为 30 / 接线 99；本守护落在 [接线]
```

> **本机坑位（两条，照实记）**
> 1. **HBuilderX 是单实例串行资源**：撞锁时脚本给 `HX_BUSY wait=120 result=timeout` 并 `exit 2`。处置是**等锁重跑**（`-WaitSeconds 600` 实测 `result=free`），**不是**杀会话、**不是**删锁文件。
> 2. **`adb` 的两个二进制坑**：`exec-out screencap -p` 必须经 `cmd.exe /c` 重定向，PowerShell 的 `>` 会破坏 PNG 字节；`uiautomator dump` 前若 app 进程正持有相关资源可能 `Permission denied` —— 本次 bounds 判据全部取自**成功的那几轮**，未把失败轮算进结论。
