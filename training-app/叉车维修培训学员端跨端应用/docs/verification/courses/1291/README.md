# #1268 chapter-view 分「需解锁 / 加载失败」—— ①a Android 真机术前↔术后对照

- **PR**：1291（对应 Issue #1268；本目录按 #1141 先例「先验后开」，取证先于 PR，落库时目录名即 PR 号）
- **复测对象**：分支 `feat/1268`
  - **术前** = `origin/master` 的 `api/course.uts` + `pages/courses/chapter-view.uvue`（`git checkout origin/master -- <这两个文件>`，其余文件保持本分支）
  - **术后** = 本分支 HEAD `f6bb233b`（已合并 `origin/master`）。本票运行时面最后一次改动是 `be5ad5b4`；`dd9cb2ad` 只动 `@note` 注释段与 ADR，`f6bb233b` 是合并提交。
    **两个术后对照面都在真机跑过**：02 支的入库截图取自 `f6bb233b`（最终 head），01 支在 `dd9cb2ad` 与 `f6bb233b` 各取一帧（`03-…-after-merged.jpg` 即后一轮）。
    `git diff dd9cb2ad f6bb233b -- api/course.uts pages/courses/chapter-view.uvue` **为空** ⇒ 本票那两个文件在两 sha 上逐字节相同。
- **日期**：2026-09-23
- **设备**：`192.168.0.212:38345`（Xiaomi `23049RAD8C` / model `marble`，无线调试）
- **包名**：`io.dcloud.uniappx`（uni-app-x 调试基座）
- **项目目录**：`D:\FL\wt-1268\training-app\叉车维修培训学员端跨端应用`（本 worktree；构建归属由「`编译成功` 时间戳落在本次运行窗口内 + 产物 58 页 + `该章节需先解锁` 在 `unpackage/dist/build/mp-weixin/pages/courses/chapter-view.wxml` 内」三点共同钉住）
- **执行人**：agent 执行（本目录只承载 ①a；**①b 未命中能力面** —— 无指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互）

## 被验的改动

`getChapterDetailApi` 原来带一个 `.catch`，把**任何**错误折成 `chapter_id: 0` 的空详情；页面据此只能落到「章节内容加载失败 + 重试」。后端对未兑换的付费章节按鉴权收回、返回 **404**（移动端 ADR-0062 决策 3 刻意不泄漏「章节是否存在」，所以未解锁与不存在共用同一支 404），于是「需要解锁」被报成「加载失败」，而那个重试按钮对着一个必然再次 404 的请求。

本票：① 摘掉该 `.catch`（错误原样 reject）；② 页面用 `api/request.uts` 的消息前缀判定出口 `isChapterNotFound()` 分出一支「**该章节需先解锁**」，该支内不给重试；真失败仍走「章节内容加载失败 + 重试」。**按 issue.patch.md 契约，后端与 Web 前端零改动。**

## 取证手法

- **部署**：`scripts/hx-run.ps1`（真运行，非仅编译）。术前机检行
  `HX_RUN mode=incremental compile=170 deploy=0 total=226 exit=ok` +
  `HX_RUN_DEPLOY deployed=true www=…/www=1790130335->1790130806 pid_before=29115 pid_after=32116`
  （判据是**设备侧资源 mtime 与进程号相对基线前进**，不是「前台是不是基座」）；术后 `dd9cb2ad` 为 `compile=110 deploy=0 total=139 exit=ok`；术后合并后 `f6bb233b` 为 `compile=150 deploy=0 total=244 exit=ok`。
- **进页**：`cli.exe launch app-android --pagePath pages/courses/chapter-view --pageQuery "…" --deviceId …`。**页身份只取应用自己的 `进入页面:` 日志行**（含 query 原文），不按截图目测、不看退出码。
- **落定判据**：`scripts/lib/auto-screenshot.ps1` 的 `Wait-NavSettled`（三条件：目标页日志行 + 画面非全黑 + 相邻采样差异 ≤0.5%）。**未落定即不落截图**（fail-closed）。
- **文本判据取 `content-desc`**（uvue 文字在 a11y 树里不走 `text`），逐张截图旁配 `<同名>.content-desc.txt`。
- 设备侧全程只读：`exec-out screencap -p` / `uiautomator dump` / `logcat -d`；**未 `adb kill-server`**。

## 逐状态结果

| # | 状态 | 术前渲染（`content-desc`） | 术后渲染（`content-desc`） |
| --- | --- | --- | --- |
| 01 | 免门禁章节 `course_id=1&chapter_id=1` | `01-chapter-view-free-before.jpg` — 27 条，标题「第一章 叉车分类与型号」+ 正文 | `01-chapter-view-free-after.jpg`（`dd9cb2ad`）— **同 27 条** |
| 02 | 后端 404 `course_id=1&chapter_id=999` | `02-chapter-view-locked-before.jpg` — `["‹","♡","章节内容加载失败","重试"]` | `02-chapter-view-locked-after.jpg`（**最终 head `f6bb233b`**）— `["‹","章节学习","♡","该章节需先解锁"]` |
| 03 | 免门禁章节，合并后复取 | —（同 01 术前） | `03-chapter-view-free-after-merged.jpg`（`f6bb233b`）— 27 条，与 01 术后只差第 5 条徽标，见下 |

**票面两条判据的落点**：

- 「未兑换付费章节：显需解锁、无死重试」→ 02 行：术前的 `重试` 节点（bounds `[500,1320][580,1374]`）在术后**整节点消失**，文案换成「该章节需先解锁」。
- 「免门禁 / 已兑换章节：行为逐字不变」→ 01 行给出三层「逐字」证据：
  - 两份 a11y dump **SHA-256 相同**（`8c4acc49212c257a…`，各 20,634 B；61 个 `content-desc` 属性、其中 27 个非空，含顺序逐条一致）；
  - 逐行像素（`LockBits`，1080×2400）**第 96 行以下 `different=0 ratio=0.000000`**，`Top=0` 时唯一差异行带是 `y28-67` = 状态栏时钟；
  - 同 build 相隔 6s 的两帧同样 `different=0` ⇒ 上面那个 0 不是「两张图根本没比」。

**03 行：合并 `origin/master` 后复取成功支**（唯一差异已归因，不由本票引起）。01 术后（`dd9cb2ad`）与 03（`f6bb233b`）两份 dump 各 27 条非空 `content-desc`、顺序一致，逐位比对只差第 5 条：`"学习中"` → `"已完成"`；像素侧也只有一行带 `y399-457`（59 行，`Top=96` 以下 `different=59 ratio=0.025608`）。该文案由 `pages/courses/chapter-view.uvue:40` 的 `detail!.study_status == 'completed'` 直接渲染**后端字段**，本票的 4 个 hunk 没有一处触及该行（见 `git diff origin/master...HEAD`）。同一份运行时面在 10:18 读出「学习中」、11:31 读出「已完成」⇒ 变量是取证期间被推进的服务端学习进度，不是代码。404 一支的两轮（`dd9cb2ad` / `f6bb233b`）`content-desc` 数组**逐字相同**、像素 `different=0`。

**404 一支的逐层对照**（术前 / 术后各自 launch 日志原文，见 `machine-lines.txt`）：两端打到的是**同一个请求、同一个应答**——`GET /api/course/1/chapter/999` → `<<< 404`，`message: "404:章节不存在"`。差别只在处理它的那一层：

| 时刻 | 术前（`origin/master`） | 术后（最终 head `f6bb233b`，`dd9cb2ad` 同文案） |
| --- | --- | --- |
| 404 到达 | `[request] <<< 404 …` | `[request] <<< 404 …` |
| 谁接住 | `[getChapterDetailApi] network failed: … at api/course.uts:465`（`.catch` 吞掉 ⇒ 返回 `chapter_id:0` 空详情） | `[chapter-view] load failed: … at pages/courses/chapter-view.uvue:237`（错误原样上抛 ⇒ 页面判 404） |
| 页面渲染 | 章节内容加载失败 + 重试 | 该章节需先解锁（无重试） |
| 顶栏标题 | **空**（`detail != null ? detail!.title : '章节学习'` 取了那个假空详情的 `''`） | 「章节学习」 |

> 顶栏标题这一行是**病根本身的可见证据**：术前拿到的是一个「非 null 但内容为空」的假详情，所以连页面标题都被折成空串。术后 `detail` 保持 `null`，标题才落到兜底文案。像素行带 `y158-201` 就是这里。

## 应用控制台错误行

| 判据 | 术前 | 术后 |
| --- | --- | --- |
| `FATAL EXCEPTION` / `ANR in io.dcloud.uniappx` | 0 | 0 |
| 404 一支的 `[request] <<< 404` | 有（预期，被验对象） | 有（预期，被验对象） |
| 成功一支 `<<< 200 …/chapter/1` | 有 | 有 |

## 诚实声明（本目录**没有**验证到的面）

1. **没有用真实「付费 + 未兑换」数据走端到端**。生产当前 0 门付费课（#1268 前置确认已实测：8 门课无一条带 `points_price`），本目录以**同一条 404 通道**（`chapter_id=999` 不存在）触发「需解锁」支。依据是移动端 ADR-0062 决策 3：后端刻意让「未解锁」与「不存在」返回同一个 404、不泄漏存在性 ⇒ 两者在客户端不可分辨、走同一支。**真实付费课的 403/404 差异若与「不存在」不同，本取证不覆盖**——这属票面第 3 条（兑换入口 + 去解锁跳转）排期时要一并验的面。
2. 01 行术前一支进页时遇后端瞬断（`<<< FAIL errMsg= server system error`，四连），页面如实落到「加载失败 + 重试」；随后**点该重试按钮**（`input tap 540 1347`，坐标取自 a11y 实测 bounds 中心）取到 200，截图取的是这一帧。副作用是一条白捡的证据：**真失败那支的「重试」在术后仍保留、且对瞬断确实可恢复**——本票只在「未解锁」一支摘它。
3. ②（微信开发者工具）**未跑**：本票不命中 MP-WEIXIN 面（`hitsMpWeixinFace` 对本分支 diff 实测 `needs2=false`）。会话内曾顺手跑两次，均判 `exit 2 = 环境不可用`（`no-ready-result` / `port-not-listening`），根因未追；细节写进 PR 正文「影响范围 / 风险点」。
4. **本次设备侧写操作逐条**：`settings put global stay_on_while_plugged_in 3`（取证后已 `settings delete`，回读 `global stay_on_while_plugged_in = null`，与取证前 `settings get` 读到的 `null` 一致）；`input tap 540 1347` 一次（仅用于上条所述的 200 复取）；`uiautomator dump` 写的设备侧临时 xml（本票用的 `/sdcard/ui1268.xml`、`/sdcard/now.xml` 已 `rm`；该目录内另有既往会话遗留的 dump，本次未触碰）。**`screen_off_timeout` 本次未写**——照实记其现值：`settings get system screen_off_timeout = 600000`（MIUI 默认档），`settings get global screen_off_timeout = null`；早先一次「前测 null」的读数取自 `global` 这一**错命名空间**，不作数，此处按实测更正。**未 kill 任何进程、未 `adb kill-server`、未改后端数据。**

5. HBuilderX 侧一次非预期停顿：叠加派发第三次 `cli launch` 时该进程活着但 stdout 恒 0 字节（420s 无 `进入页面` 行 ⇒ `Wait-NavSettled` 判未落定、fail-closed 不出图）。按 ADR-0008 的「绝不 kill 主程序 / 不终止 cli 子进程」处置，等其自行松开后重派即恢复。**这条属取证工具链经验，不是本票的运行时结论。**
