# 移动端课程页滚动回归（#1134）—— ①a Android 真机截图取证

- **PR**：待补（本票按维护者裁定**先验后开**：本次取证先于 PR，目录名在开 PR 后即改为 PR 号）
- **复测对象**：分支 `fix/mobile-1134-courses-scroll` HEAD **`031b7b08`**（本目录与它同树；取证期间未再改运行时面）
- **日期**：2026-09-18
- **设备**：`b32d8398`（Xiaomi `23049RAD8C`，Android 15，USB）
- **包名**：`io.dcloud.uniappx`（uni-app-x 调试基座）
- **项目目录**：取证时用唯一名 `fl-mobile-wt1134`（HBuilderX 按**项目名**解析；`cli project list` 现测有 8 个同名项目，不改名会误 publish 到别的工作树）
- **执行人**：agent 执行（本目录只承载 ①a；**①b 未命中能力面**，故无人工签收项 —— `pages/courses/courses.uvue` 不在 `scripts/lib/capability-surface.ps1` 白名单内）

## 被验的改动（本票唯一运行时面）

`f1ab2648`（#361 模块重构）删掉了 `courses.uvue` 的 `<scroll-view class="course-scroll" scroll-y="true" @scrolltolower="onLoadMore">`，而根容器仍绑 `height: windowHeight + 'px'` ⇒ 内容被裁、整页无可滚区域。本票把它包回来（并补 `.course-scroll { flex: 1; height: 0 }`）。

## 取证手法

- **部署**：`npm run hx:run`（真运行）→ 机检行
  `HX_RUN mode=incremental compile=120 deploy=0 total=138 exit=ok`；
  `HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1789695982->1789698555 pid_before=17490 pid_after=17554 foreground=io.dcloud.uniappx reason=资源已落到设备…`（判据是**设备侧事实相对基线前进**，不是「前台是不是基座」）。
- **进页**：点原生 tabBar 的「课程」格 —— 坐标取 `uiautomator dump` 的 `bounds`（`[297,2351][351,2388]` → 中心 `324,2370`），**不按截图目测**（ADR-0008 补遗第 2 条）。
- **`input` 可注入性每次现测**：本次 `input tap` / `input swipe` 均可用（`shell "input tap …; echo TAP_RC=$?"` → `TAP_RC=True`，logcat 无 `INJECT_EVENTS`）；继 ADR-0008 补遗第 3 条在本设备上的第三个数据点。
- **文本判据取 `content-desc`**（uvue 文字在 a11y 树里不走 `text`），逐张截图旁配 `<同名>.content-desc.txt`。
- **页身份**取设备侧日志原文（不靠退出码）：
  `10:31:13.503 I console : [LOG]---BEGIN:CONSOLE---{"type":"string","value":"进入页面:​/pages/courses/courses​ 。[{\"创建dom元素个数\":\"30个\",\"耗时\":\"24ms\"},{\"排版\":\"1次\",\"耗时\":\"63ms\"},{\"渲染\":\"1次\",\"耗时\":\"33ms\",\"跳转页面到onReady总耗时\":\"398ms\"}]"}---END:CONSOLE---`

## 逐状态结果

| # | 状态 | 截图 | `content-desc` 条数 | 关键判据（设备侧原文） |
| --- | --- | --- | --- | --- |
| 01 | 课程页首屏（列表顶） | `01-courses-first.jpg` | 32 | 列表首条 = `内燃叉车动力装置（内燃机）/ 共7个课时`；首屏**可见 7 条**；`更多课程` 与 `没有更多了` **均不在**首屏 |
| 02 | 上滑一次（内容位移） | `02-swipe-middle.jpg` | 28 | 顶项变为 `货叉操作技能训练 / 共3个课时` ⇒ **内容确实位移**（原 `内燃叉车动力装置（内燃机）` 已出屏） |
| 03 | 滚到底（末条 + 到底态） | `03-courses-bottom.jpg` | 28 | 出现第 8 条 `叉车基础知识概述 / 共3个课时`、`没有更多了`、`更多课程` —— **三者首屏都没有** |
| 04 | 反向滑回顶部（双向） | `04-swipe-back.jpg` | 32 | 顶项回到 `内燃叉车动力装置（内燃机）` ⇒ 上下双向可滚 |

**「首屏没有 → 底部有」逐字对照**（判据字符串直接搜两份 dump）：

| 字符串 | 01 首屏 | 03 底部 |
| --- | --- | --- |
| `叉车基础知识概述`（第 8 条课程） | ✗ 无 | ✓ 有 |
| `没有更多了`（到底态） | ✗ 无 | ✓ 有 |
| `更多课程`（按钮） | ✗ 无 | ✓ 有 |
| `内燃叉车动力装置（内燃机）`（首条） | ✓ 有 | ✗ 无 |
| `货叉操作技能训练`（中段） | ✓ 有 | ✓ 有 |

**再上滑一次画面不再变化**（`04-bottom-again` dump 与 `03-bottom` 逐字相等）⇒ 已到列表底部，不是「没滚到位」。

## 应用控制台错误行

按 ADR-0012 的口径在本次 launch 日志（`launch-a9c6bfb0cc2a4ea394c6ddad6481b127.out`，212 行）里找应用侧错误行：

| 判据 | 结果 |
| --- | --- |
| `style property` 错行 | **0** |
| `[request] <<< FAIL` | **0** |
| `Uncaught` / `Exception` / `TypeError` | **0** |
| 页身份 | `进入页面:​/pages/courses/courses` 命中 |
| 取数 | `[request] >>> GET https://www.gccsmile.com/api/student/courses` → `<<< 200` |

## 诚实声明（本目录**没有**验证到的面）

1. **`@scrolltolower` 的「自动加载下一页」没能在真机上区分验证** —— 本页真机上**只加载到 8 条**（首屏 7 条 + 滚到底出现第 8 条）随后即 `没有更多了` ⇒ **不存在第 2 页**；且 `loadCourses(reset=false)` 在 `noMore && !reset` 处会**提前返回**（`courses.uvue:161`），即便拖到底也观察不到可区分的状态变化。该条**只有代码级证据**（diff 里恢复了 `@scrolltolower="onLoadMore"` 绑定）+ 既有端到端先例，**不是真机证据**。
   （旁证：公开只读 `GET /api/courses?page=1&page_size=12` 现测 `total=8 / pages=1`，与设备侧 8 条一致；移动端真实路由 `/student/courses` 未鉴权返回 401，未取到 total。）
2. **可滚动区 = 课程列表区**（首条课程 → 「更多课程」按钮）；头部标题 / 「资料课·课程」分类卡 / 子分类 tabs / 筛选行**按设计钉住**（与 `f1ab2648` 之前的形态一致，也是 `docs/ui-spec.md` 的「钉 header、滚内容」口径）。故**在钉住区域拖动不会有位移**；若期望「整页都能跟着拖」，需改走备选 (a)（把分类卡以下整块包进 `scroll-view`）——一行改动，本目录未验。
3. **agent 看不到截图内容**：可核的是上面那份 `content-desc` 文本判据与日志判据；截图的视觉判断留给签收人 —— 本目录不把「截图已入仓」当成「内容已核对」。
4. **未做对照实验**：本次没有在「未修复的树上」再跑一遍同一手法（红/绿对照只到进程级 —— 守护 `utils/uvuePageScrollContract.test.js` 在修复前对 `courses.uvue` 判红、修复后 9/9 绿）。设备侧的红对照依赖把 `f1ab2648` 的删除态重新部署，本次未做。

## 复现命令

```powershell
# 0) 项目目录先改成唯一名（HBuilderX 按项目名解析；本仓有 8 个同名项目）
& "D:\软件\HBuilderX.5.23.2026080626\HBuilderX\cli.exe" project close --path D:\FL\wt-1134\training-app\叉车维修培训学员端跨端应用
Rename-Item "D:\FL\wt-1134\training-app\叉车维修培训学员端跨端应用" "fl-mobile-wt1134"

# 1) 部署本分支到真机（真运行；设备侧事实相对基线前进才算到）
npm run hx:run        # 见上方 HX_RUN / HX_RUN_DEPLOY 机检行

# 2) 进课程 tab：坐标取 uiautomator dump 的 bounds 中心（原生 tabBar 走 text=）
$adb = "D:\android-sdk\platform-tools\adb.exe"      # 本仓唯一真源：scripts/lib/env-check.ps1 的 Resolve-AdbExeLocal
& $adb -s b32d8398 shell "input tap 324 2370; echo TAP_RC=$?"      # 课程 tab 中心（现测 bounds [297,2351][351,2388]）
& $adb -s b32d8398 exec-out screencap -p > 01-courses-first.png
& $adb -s b32d8398 shell uiautomator dump --compressed /sdcard/a.xml; & $adb -s b32d8398 exec-out cat /sdcard/a.xml > a.xml

# 3) 滚动取证（列表区内拖动：y 1800 → 900 落在列表里，不碰 tabBar）
& $adb -s b32d8398 shell "input swipe 540 1800 540 900 300"
& $adb -s b32d8398 shell "input swipe 540 800 540 1900 300"        # 反向滑回
```

> **部署残留**：`hx:run` 的真运行会话**常驻且不自己收口**（要停就在 HBuilderX 里点「停止」，脚本绝不 kill）；改回项目目录名前先 `cli project close --path …`，仍被拒时**只结束命令行里引用本项目路径的 `cli.exe`**，**绝不**碰 `HBuilderX.exe`、也**不碰别的会话的 cli**（ADR-0008 补遗第 8 条）。
>
> **本次实际发生（如实披露）**：`cli project close --path …` 已返回「项目关闭完成」，`Rename-Item` **仍**被拒（`Access to the path '…fl-mobile-wt1134' is denied`）—— 与 ADR-0008 补遗第 8 条的实测一致。逐个核对 `CommandLine` 后，**只结束了 1 个引用本项目路径的 `cli.exe`（PID 7268**，即本次 `launch app-android --project …fl-mobile-wt1134 --pagePath pages/courses/courses` 那个常驻会话**）**，随即改名成功。全程**未碰** `HBuilderX.exe`（PID 22100 自始至终存活）、**未碰**任何别的会话的 cli。
