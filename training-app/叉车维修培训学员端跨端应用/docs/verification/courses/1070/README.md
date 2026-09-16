# courses 模块手术（T08 / #646）—— ①a Android 真机逐页截图取证

- **PR**：#1070（base `master`，head `refactor/courses`）
- **复测对象**：`refactor/courses` HEAD `765464fe`（术前基线 = merge-base `750584c0`）
- **日期**：2026-09-16
- **设备**：`b32d8398`（Android 真机，USB）
- **包名**：`io.dcloud.uniappx`（uni-app-x 调试基座，appid `__UNI__1C1D180`）
- **执行人**：agent 执行（本目录仅承载 ①a 证据；①b 能力面未命中，故无人工门）

## 为什么是「会话式」取证

`scripts/device-capture.ps1` 只拼 `--pagePath`、**不支持 `--pageQuery`**（`grep pageQuery scripts/**.ps1` 零命中），
而本模块三页里的 `course-detail` / `chapter-view` 都是**带参页**（`?id=` / `?course_id=&chapter_id=`）。
按 `docs/adr/0008-移动端验收门与证据.md`「①a 取证手法补遗」第 1、5 条，带参页只能走
`cli.exe launch app-android --pagePath <页> --pageQuery <查询串>` 的**会话式**取证。

落定判据**不另造**：直接复用 `scripts/lib/auto-screenshot.ps1` 里已被真机验证的
`Start-NavLaunchDetached`（分离派发，不被常驻 launch 会话拖住）+ `Wait-NavSettled`
（三条同时满足才落定：应用日志出现**目标页**的 `进入页面:` 行 + 采样帧非全黑 + 相邻帧宽容一致 ≤0.5%）。
到上限不满足即 `Settled=false` 且**不落截图**（fail-closed）。

## 逐页结果

| 页 | 页身份（设备侧日志） | content-desc 条数 before/after | 文本判据 | 全帧像素对比 |
| --- | --- | --- | --- | --- |
| `pages/courses/courses` | `进入页面:pages/courses/courses` | 32 / 32 | **逐条完全一致** | 第 **96** 行以下哈希相同 |
| `pages/courses/course-detail?id=1` | `进入页面:pages/courses/course-detail?id=1` | 31 / 31 | **逐条完全一致** | 第 **96** 行以下哈希相同 |
| `pages/courses/chapter-view?course_id=1&chapter_id=1` | `进入页面:pages/courses/chapter-view` | 26 / 26 | **逐条完全一致** | 第 **96** 行以下哈希相同 |

- 三次术后取证的 `settled=true`、`diff=0`（相邻两帧差异 0%）、截图非全黑（均 168–272 KB）。
- **像素对比口径**：裁掉顶部 N 行后对两张图取 MD5，再对 N 做行带二分。三页都在 **N=96** 起哈希相等
  ⇒ 差异**只落在顶部 96 行**（设备状态栏的时钟/电量等实时读数）。**第 96 行以下逐像素相同**。
- 截图落点（宽 720 / JPEG q75，本机无 `cwebp`/`ffmpeg`/`magick` ⇒ 走 ADR-0008 记的 JPEG 兜底）：
  6 张合计 **540,654 B**（限 1.5 MB），单张最大 **110,770 B**（限 150 KB）。

## 数据源（重要，写实）

三页打的是**真实后端** `https://www.gccsmile.com/api`（应用已登录：`Token validated, isLoggedIn=true`），
不是 mock：

- `/course/1` 返回 200，`name = 叉车基础知识概述`、`theory_hours = 0`、`practice_hours = 0`、`progress = 0`、`is_enrolled = true`、3 章。
  ⇒ 详情页「理论学时 / 实操学时」显示 `-` 是**后端真的返回 0**，不是回归。
- **首取 `courses` 页时后端返回 `server system error`（500）**，请求落到 `catch-mock` 回退，
  于是那一张渲染的是 `getMockCourseList` 的六门课（叉车基本构造 / 液压传动系统 / …）——
  与术前那张（真实列表：内燃叉车动力装置、场（厂）内机动车辆基础…）**数据源不同**，像素自然不同。
  **故本目录的 `01-courses-after.jpg` 用的是重取的那一张**（重取后文本与术前逐条一致、像素第 96 行以下相同）。
- 这一点顺带**现场复现了 `catch-mock` 回退的掩盖面**：后端 500 被伪装成正常首屏，
  结构上与真实数据无从区分（口径见 ADR-0007「api 收紧的失败可见口径」；本票按删除禁区未退役该回退）。

## 应用控制台错误行

按 ADR-0012 的口径在 launch 日志里找应用侧错误行：

| launch 日志 | 进入页 | 应用控制台错误行 |
| --- | --- | --- |
| （首取 courses，未归档） | `pages/courses/courses` | **10 条**，全部源于后端 500：`[request] <<< FAIL errMsg= server system error` + 随之的 `[getCatalogStudent…]/[getCatalogTreeSpecialties] failed, using mock data` |
| 归档用 course-detail | `pages/courses/course-detail?id=1` | **0** |
| 归档用 chapter-view | `pages/courses/chapter-view` | **0** |
| 归档用 courses-retry | `pages/courses/courses` | **0** |

⇒ **本目录归档的 3 次术后取证运行，应用控制台错误行数为 0。**

## 诚实声明（agent 核不了截图内容）

本会话的**视觉通道配额用尽**（vision 通道返回「免费额度 10 次已用完」），agent **看不到截图内容**。
所以本目录不把「截图已入仓」当作「内容已核对」，而是配了**两条可核验的机器判据**：

1. **文本判据**：`<页名>.content-desc.txt` —— `uiautomator dump` 取到的 `content-desc`（uvue 的文字在 a11y 树里是
   `content-desc`，不是 `text`），逐条 before/after 比对，三页均**逐条完全一致**。
2. **像素判据**：裁掉顶部 96 行后 MD5 相同 ⇒ 正文区**逐像素一致**。

## 复现命令

```powershell
# 1) 部署（术后树 / 术前树各自一份，项目目录须用唯一名避开 HBuilderX 同名误命中）
pwsh -NoProfile -File scripts/hx-run.ps1 -Project <项目绝对路径> -Device b32d8398

# 2) 会话式逐页取证（带参页必须 --pageQuery；值要用引号包住，否则裸 & 会被 PowerShell 当运算符）
& cli.exe launch app-android --project <项目> --deviceId b32d8398 `
    --pagePath pages/courses/chapter-view --pageQuery "course_id=1&chapter_id=1"

# 3) 落定后 adb 取图 + a11y 文本
adb -s b32d8398 exec-out screencap -p > page.png
adb -s b32d8398 exec-out uiautomator dump --compressed /dev/tty > page.xml
```
