# 移动端帮助中心（#1081）—— ①a Android 真机截图取证

- **PR**：#1110（base `master`，head `feat/mobile-help-center-1081`）
- **复测对象**：`feat/mobile-help-center-1081` HEAD `3cbad7f3`（本目录随后的那次 push **只新增文档/截图**，不动运行时面）
- **日期**：2026-09-17
- **设备**：`f0bae674`（Xiaomi 2510DRK44C / `annibale`，USB）
- **包名**：`io.dcloud.uniappx`（uni-app-x 调试基座）
- **项目目录**：取证时用唯一名 `fl-mobile-wt1081`（HBuilderX 按**项目名**解析；同时段 `D:\wt-1083` / `D:\FL\wt-1082b` 的项目目录 basename 与本仓主树**同名**，不改名会误 publish 到别的树）
- **执行人**：agent 执行（本目录只承载 ①a；①b 的「能力面」未命中，故无人工签收项）

## 取证手法

本页是**无参页**，直接复用 `scripts/lib/auto-screenshot.ps1` 的 `Invoke-AutoScreenshot`（分离派发
`cli launch app-android --pagePath …` + `Wait-NavSettled`：**页身份 + 非全黑 + 相邻帧宽容一致 ≤0.5%**，
到上限不满足即 `Settled=false` 且**不落截图**，fail-closed）。

页身份取自设备侧日志（App 自己打的 `进入页面:` 行），不靠退出码：

```
16:22:23.087 进入页面:pages/profile/help-center 。[{"创建dom元素个数":"14个","耗时":"6ms"},{"排版":"1次","耗时":"4ms"},{"渲染":"1次","耗时":"11ms","跳转页面到onReady总耗时":"65ms"}]
```

导航落定 **189s**，相邻两次采样差异 **0%**。

> 首次（修 `white-space` 之前）同一手法也落定过（188s / 0%），但**中途被另两个会话的 launch 顶掉两次**
> （日志尾 `已停止运行...`）—— 这是 HBuilderX 单实例串行 + 同一台设备被多会话争用的实测现象，
> 不是本页的问题。补跑的第 3 次拿到干净窗口。

**交互怎么来的**：`adb shell input` 在本机**本次可用**（2026-09-15 起两次实测均可用，与 2026-09-13 的
`INJECT_EVENTS` 硬拒不一致 ⇒ 按 ADR-0008 的口径**每次现测**）。点击坐标一律取 `uiautomator dump` 的
`bounds` 中心，不按截图目测；判「点没点中」看 **`content-desc` 是否变化**，不看退出码。

## 逐状态结果

| # | 状态 | 截图 | `content-desc` 条数 | 关键判据（设备侧原文） |
| --- | --- | --- | --- | --- |
| 01 | 首屏（默认：折叠 + 全部分类） | `01-help-center-after.jpg` | 34 | `全部 (30)`、`账号与登录 (5)`、`证件与学习路径 (3)`、`课程学习 (4)`；分组标题 + 各题 + 折叠符 `▸` |
| 02 | 点开第一条 | `02-help-center-expanded-after.jpg` | 31 | 折叠符变 `▾`，答案正文上屏：`在登录页点「注册」，可用手机号或邮箱验证码注册。…` |
| 03 | 端上搜索 `AI` | `03-help-center-search-after.jpg` | 14 | `全部 (2)`、`账号与登录 (0)`、`证件与学习路径 (1)`、`课程学习 (0)`；**跨分类**命中：`证件与学习路径` + `题库与考试`；出现清空位 `✕` |
| 04 | 搜索无命中 `zzzz` | `04-help-center-nomatch-after.jpg` | 10 | `全部 (0)` 全 0 + 空态 `🔍` + `没有匹配的问题` |
| 05 | 列表下滑（覆盖余下分类） | `05-help-center-scroll-after.jpg` | 33 | `笔记与社区`、`就业与简历` 两个分组标题 + `积分与任务` 的题目（每日任务什么时候重置？…） |

- 5 张截图为 **JPEG q75 / 宽 720**，合计 **413,431 B**（限 1.5 MB），单张最大 **92,438 B**（限 150 KB）。
- 每张旁边配一份 `<同名>.content-desc.txt`：uvue 的文字在 a11y 树里走 **`content-desc`**、不走 `text`
  （按 `text` 取只会拿到输入框与原生 tabBar，看着像「页面没渲染」）。
- 数据源是**真实后端** `https://www.gccsmile.com/api`（已登录，`Token validated, isLoggedIn=true`）：
  `[request] >>> GET https://www.gccsmile.com/api/faq` → `[request] <<< 200 https://www.gccsmile.com/api/faq`。
  `全部 (30)` 与后端种子的 **30 条已发布条目**一致（`000036_faq.up.sql`）。

## ①a 抓到的一条真缺陷（本票内已修）

静态守护查不出、只有真机日志能查出的 uvue CSS 违规：

```
style property `white-space` is only supported on `<text>|<button>`.
there is an error on `<scroll-view class="category-scroll" scrollTop="0" scrollLeft="0" scrollX="true">`.
```

- **机制**：`.category-scroll`（`scroll-view`）上的 `white-space: nowrap` 是从既有的 `.filter-scroll`
  系列照抄的；uvue 原生端该属性**只支持 `<text>` / `<button>`**，写在 `scroll-view` 上被判错并**忽略**
  ⇒ 死声明。横滑本身靠同一 class 上的 `flex-direction: row` + 子项 `flex-shrink: 0`。
- **修法与复验**：删掉该声明（`3cbad7f3`）后重跑同一套取证 ——
  **该错行由 1 条降为 0 条**，且 chip 行仍是**单行横排**。这一条不是「看着像」：01 / 05 两次 dump 里
  4 个 chip 的 `bounds` **y 完全相同（478..527）**、x 依次 **73 → 320 → 651 → 1055**，
  第 4 个（`课程学习 (4)`）右缘落在 **1156 = 屏宽**处被裁 ⇒ 行内容确实溢出、横滑成立。
- **守护**：`utils/helpCenterContract.test.js` 加了「本页不得出现 `white-space`」，并把 CSS 判据改为
  **先剥注释再判**（否则「说明为什么禁它」的注释会把守护自己判红）。
- **坑位回写**：移动端 `AGENTS.md` 的 uvue CSS 兼容性表补了一行；存量 6 处同款写法（`favorites` /
  `records` / `practice-records` / `featured-list` / `ai-feature` / `mock-exam`）属**既有**缺陷，另立 #1113。

## 应用控制台错误行

按 ADR-0012 的口径在 launch 日志里找应用侧错误行（本次运行 289 行）：

| 判据 | 结果 |
| --- | --- |
| `style property` 错行 | **0**（修前为 1） |
| `[request] <<< FAIL` / `Uncaught` / `Exception` / `TypeError` | **0** |
| 页身份 | `进入页面:pages/profile/help-center` 命中 |
| `GET /api/faq` | **200** |

## 诚实声明（本目录**没有**验证到的面）

1. **失败态 + 重试位没有在真机上走到**：要触发得让 `/api/faq` 返错或断网，本次没造那个夹具。
   该态由 `utils/helpCenterContract.test.js` 的静态契约（`failed` 分支 + `@click="reload"` + 与空态互斥）
   覆盖，**不是**真机证据。
2. **答案的「原样换行」没有被真机演示**：后端 30 条种子答案**都是单行**（`000036_faq.up.sql` 无内嵌换行），
   所以「`\n` 在 `<text>` 里换行」这条口径本次**取不到证据**（不是失败，是没有样本）。管理端日后录入多行答案时
   才真正走到这条路径。
3. **只核到可见节点**：`uiautomator dump` 只报当前屏可见的节点，所以「7 个分类全部渲染」是由 01（账号/证件/课程）、
   03（题库）、05（笔记/就业 + 积分题目）**跨三张图**拼出来的，不是单次全量。7 类标题与 `全部 (30)` 都在证据里。
4. **agent 看不到截图内容**：本目录不把「截图已入仓」当成「内容已核对」——可核的是上面那份 `content-desc`
   文本判据与日志判据；截图的视觉判断留给签收人。

## 复现命令

```powershell
# 0) 项目目录先改成唯一名（HBuilderX 按项目名解析），本仓主树同名，不改会误命中别的工作树
Rename-Item "D:\wt-1081\training-app\叉车维修培训学员端跨端应用" "fl-mobile-wt1081"

# 1) 落定 + 截图（复用库里的 Wait-NavSettled：页身份 + 非全黑 + 相邻帧稳定）
. scripts\lib\auto-screenshot.ps1
Invoke-AutoScreenshot -Device f0bae674 -CliPath "<HBuilderX>\cli.exe" `
  -ProjectDir D:\wt-1081\training-app\fl-mobile-wt1081 `
  -OutputDir .ci-verify\wt1081-shots -Pages 'pages/profile/help-center'

# 2) 交互取证（input 先现测是否可用；坐标取 uiautomator dump 的 bounds 中心）
adb -s f0bae674 shell input tap <cx> <cy>              # 展开第一条
adb -s f0bae674 shell input text 'AI'                  # 端上搜索（adb input text 不支持中文）
adb -s f0bae674 exec-out uiautomator dump --compressed /dev/tty   # content-desc 文本判据
adb -s f0bae674 exec-out screencap -p > shot.png

# 3) 收口后改回原名（改名被项目自身的常驻 cli 箝住时，只结束命令行里引用本项目路径的那个 cli.exe）
cli.exe project close --path D:\wt-1081\training-app\fl-mobile-wt1081
Rename-Item "D:\wt-1081\training-app\fl-mobile-wt1081" "叉车维修培训学员端跨端应用"
```

> **会话式坑位披露（ADR-0008 补遗第 8 点路径）**：改名被拒后，本会话**只结束了一个 `cli.exe`** ——
> 逐个核对 `CommandLine` 确认它引用的正是本项目路径（`launch app-android --pagePath pages/profile/help-center
> --project …\fl-mobile-wt1081`），**没有**碰 `HBuilderX.exe`，也**没有**碰别的会话的 cli（同期的
> `D:\wt-1083` / `D:\FL\wt-1082b` 进程全程未动）。共发生 2 次（两次改名各一次）。
