# ① 真机验证 · PR #1292（#1273 论坛列表预览接格式轴：源串不再出现在卡片上）

设备：Xiaomi `23049RAD8C`（`marble`），Android 15，HBuilderX 调试基座（包 `io.dcloud.uniappx`）——
按移动端 `docs/adr/0008-移动端验收门与证据.md` 的 ① 门（**①a：agent 出证**）留档。

**证据绑定的树**：取证那趟真机跑在运行时面 commit `8eeea173`（base = `origin/master` `ed39d854`）；本目录的取证提交
只往 `docs/verification/forum/1292/**` 加文件，运行时面一字未动。

**rebase 谱系（两次，2026-09-23）** —— 为过 ruleset 的「分支须与 master 同步」，分支两次 rebase：

| 趟 | master base | 该趟从 master 带入 | 运行时面 commit | 取证 commit |
| --- | --- | --- | --- | --- |
| 取证原树 | `ed39d854` | — | `8eeea173` | `48f48040` |
| rebase ① | `2872f8c7` | #1291（courses 域，18 files） | `02b24345` | `6f09fbe8` |
| rebase ② | `bec91850` | #1293（`backend/**` + 根 `ADR-0064`，18 files） | `3dd3d3c7` | `7219ca00` |

⇒ **表里除 `3dd3d3c7` / `7219ca00` 外的 sha 都是 force-push 前的历史出处**，squash 合并后在 `master` 上不可解析。
这批证据的**耐久身份不是 commit sha，是那 7 个 git blob**（`machine-lines.txt` 的 blob 表）—— 三棵树上逐字相同
（`8eeea173` == `02b24345` == `3dd3d3c7`，本次现测逐条打印 `SAME`），故 squash 后那个唯一 commit 里的字节
与设备上跑过的字节仍是同一批。

**①a 的图与 logcat 产出不重拍**，理由可核验而非自述：
① 上述 blob 恒等 ⇒ 「图里的字节变了」这条重拍判据不成立；
② `git diff --name-only 2872f8c7 bec91850` = #1293 那 18 个文件，**其中 `training-app` 文件数为 0**
（`grep -cE '\.(uvue|uts)$|(manifest|pages|platformConfig)\.json$'` ⇒ `0`）⇒ 第二次 rebase 连移动端的非运行时面都没碰；
③ 第一次 rebase 带入的 #1291 那 18 个文件全在 courses 域与其取证目录，与本票承载面无交集。

**④c 与 ③ 都在新 head 上重跑**：④c 三次跑（取证原树 12:05:39 / rebase ① 后 12:28:31 / rebase ② 后 12:48:53）
读数逐字一致 = `KOTLIN_ALL_RESULT errors=0 classes=1498 files=120 freshness=fresh`（本目录 `kotlin-all.txt` 存的是
**最新那次** 12:48:53，与原树那次 `diff` 仅时间戳行不同）；③ 由 CI `mobile-test` 在 head `33d70583` 上跑
（**125 suites / 2436 tests 全绿**，链接见 PR 正文 ③ 行），本机在同一棵树上另跑一次同读数，见 `machine-lines.txt` 末行。

**改前基线** = 取证时设备上**既有**的构建（不含本票改动；`www` mtime `1790134791`，由并发会话于 11:39 部署）。
「它不含本票改动」**不靠自述**，由判据 1 的「源串仍在」这一条实证。

**接入方式**：无线调试 `adb connect 192.168.0.212:<端口>`（端口每次轮换）；设备已登录，本轮**未触碰任何凭证**，
**未对生产论坛做任何写操作**（发帖 / 回复 / 上传都不在本票射程，也不在本轮授权内）。

**导航方式**：**不用**深链（`cli launch --pagePath` 在 #1261 实测会把 app 停在空白页、中途 kill 还会推坏设备上的 `www`），
走真实 UI 路径：首页 → `交流` →（`广场` / 右上 `📇` → `个人动态`）。坐标一律取**未压缩** `uiautomator dump` 的 `bounds` 中心。

## 夹具（生产存量，本轮未新建任何内容）

四条 `content` 与 `content_format` 的取值**不靠猜**：app 自己把列表接口载荷打进了 logcat
（`logcat.txt` 末尾「列表接口载荷里的四个夹具」段，逐字原文）。

| 帖 | `content_format` | 正文源串（设备侧载荷） | 用途 |
| --- | --- | --- | --- |
| `测试markdown渲染`（张三十，2026-09-12） | `markdown` | `## 测试 Markdown 渲染⏎⏎- 无序列表⏎⏎---⏎⏎1. 有序列表⏎2. 你好⏎⏎---⏎⏎```python⏎print("hello world")⏎```⏎⏎---`（91 字符，`images: []`） | 判据 1 / 2 / 4 / 5 的主夹具：多块 + 多行 |
| `测试摄影拍照功能`（张三，2026-09-22） | `markdown` | `### 拍照摄影` | 短 markdown ⇒ 卡片 a11y 文本**可读**（长正文那条读不到，见坑位 2） |
| `1`（张三，2026-09-17） | `text` | `1` | 判据 3 的零回归对照 |
| `测试帖子`（张三十，2026-08-28） | `text` | `这是一个测试帖子` | 判据 3 的第二面（带 scope 标签的卡） |

## 判据（不靠肉眼看「像不像」，都可机检）

| # | 判据 | 机检原文 |
| --- | --- | --- |
| 1 | **改前：列表预览直出源串** | `01-before-plaza-content-desc.txt` 里 markdown 帖的预览节点 = **`### 拍照摄影`**（源记号在卡片上）；`03-before-plaza-scrolled.jpg` 的长帖预览 = `## 测试 Markdown 渲染` / `- 无序列表` / `---` / ```python``` 的**原文**，且**换行没被去掉** ⇒ 预览节点 `bounds=[72,934][1009,1606]` = **672px 高**（一屏只剩两张卡） |
| 2 | **改后：源串不再出现，只剩正文文字** | `04-after-plaza-content-desc.txt`：同一节点 = **`拍照摄影`**（`### 拍照摄影` 在该文件命中数 **0**）；`05-after-plaza-scrolled.jpg`：长帖预览 = `测试 Markdown 渲染 无序列表 有序列表 你好 print("hello world")` —— `##` / `- ` / `---` / ```python``` 四条源记号**全部 0 命中** |
| 3 | **零回归：`text` 档逐字不变** | `01` 与 `04` 两份 dump 里 `1` 的预览都是 `1`；`03` 与 `05` 里 `这是一个测试帖子` 原样；`02-before-activity` / `06-after-activity` 同理 —— 且这几条的 `content_format` 由**设备侧载荷**实证为 `text`（不是「看着像纯文本」：`logcat.txt` 里 `title:"1"…content_format:"text"`、`title:"测试帖子"…content_format:"text"` 逐字在案） |
| 4 | **顺手修（全局去换行）落在真机上** | 同一张卡（`测试markdown渲染`）、同一列表位：预览节点高度 **672px ⇒ 112px**（`03` vs `05`）⇒ `replace('\n',' ')` → `/\n/g` 的效果不止是 ③ 里的一条断言 |
| 5 | **预览与正文同源**（票面判据 2） | 三段链条逐段可对：① 设备侧载荷给出源串与 `content_format=markdown`（`logcat.txt`）；② 本地**真执行**三层（`utils/forumChainHarness.js` 跑 `markdown.uts → forumBody.uts → forumDisplay.uts`）对该载荷返回 `测试 Markdown 渲染 无序列表 有序列表 你好 print("hello world")`（48 字符）；③ 设备像素面（`05`）与 ② **逐字符一致** ⇒ 设备上跑的就是那条格式轴，不是第二份实现 |
| 6 | **第三处承载面（个人动态）同口径** | `02-before-activity-content-desc.txt` = `### 拍照摄影` ⇒ `06-after-activity-content-desc.txt` = `拍照摄影`（同一节点 `bounds=[71,789][1010,845]`），同页 `text` 档 `1` 不变 |
| 7 | **logcat 写实数，不写「E = 0」** | `logcat.txt` 头注：窗口（设备 epoch `1790136012→1790136205`，约 193 秒）内解析 **74099** 行 / E 级 **428** 行 / **app 进程（pid 18178）248** 行，其中 E 级 **35** 行；这 35 行的形态**全部列出**（`No package ID XX found for resource ID 0x…` × 30、perf-hal × 3、Recents `baseIntent` × 2 —— 都是基座与 ROM 既存噪声）；命中本次改动关键词（forum / markdown / content_format / plain / Topic）的**报错**行数 = **0**；`FATAL` / `AndroidRuntime` 崩溃 / `NoSuchMethod` = 0 条 |
| 8 | **部署事实（不是门，是 ①a 的部署判据）** | `machine-lines.txt`：`HX_RUN_DEPLOY deployed=true www=…/__UNI__1C1D180/www=1790134791->1790135971 pid_before=…14090 pid_after=…18178 foreground=io.dcloud.uniappx reason=资源已落到设备…mtime 相对基线前进`；`HX_RUN mode=incremental compile=250 deploy=0 total=358 exit=ok` |

## 图片（同一夹具、同一列表位的前后对照）

| 文件 | 构建 | 说明 |
| --- | --- | --- |
| `01-before-plaza.jpg` | 设备上既有构建（不含本票改动） | 广场顶部：markdown 帖预览 = `### 拍照摄影`（源串），`text` 帖「1」= `1` |
| `02-before-activity.jpg` | 同上 | 个人动态「我的帖子」：同一张卡预览 = `### 拍照摄影` |
| `03-before-plaza-scrolled.jpg` | 同上 | 广场滚动位：长 markdown 帖预览**整段源串 + 保留换行**，卡片被撑到 672px（既有 bug 的实证） |
| `04-after-plaza.jpg` | 本分支 `8eeea173` | 同一位置：预览 = `拍照摄影`；`text` 档逐字不变 |
| `05-after-plaza-scrolled.jpg` | 本分支 | 同一张长帖：预览 = 一行纯文本（`测试 Markdown 渲染 无序列表 有序列表 你好 print("hello world")`），源记号 0 命中，卡片高度回落 |
| `06-after-activity.jpg` | 本分支 | 个人动态：预览 = `拍照摄影`，`text` 档不变 |
| `07-after-qa-tab.jpg` | 本分支 | `知识问答` tab 全量两条帖（`1` / `这是一个问题`）预览与源串无差 —— 该 tab **没有** markdown 帖（见未覆盖面 2） |

> 压缩口径：本机无 WebP 编码器，退 **JPEG q75、宽 720**（7 张 / 合计约 440 KB，最大 81 KB）—— 同 PR #1261。
> 每张图片都配同名 `-ui-dump.xml`（未压缩 `uiautomator dump` 原件）与 `-content-desc.txt`（从中抽出的 a11y 文本，一行一条）。
> `machine-lines.txt` 收本轮全部机检行；`kotlin-all.txt` 是 ④c 整模块编译的原日志（含 `KOTLIN_ALL_RESULT` 末行）——
> 后缀用 `.txt` 而非 `.log`，因为仓库 `.gitignore:62` 屏蔽 `*.log`（先例 `docs/verification/recruiter-login/1206/04-kotlin-all-4c.txt`）。

## 手法（可复现）

```powershell
# 1) 改前基线（设备上既有构建）：真实 UI 导航到目标位，逐位取「未压缩 dump + screencap」
adb -s <ip:port> shell uiautomator dump /sdcard/n.xml ; adb pull /sdcard/n.xml page.xml
adb -s <ip:port> shell screencap -p /sdcard/c.png    ; adb pull /sdcard/c.png page.png
adb -s <ip:port> shell "input tap <cx> <cy>; echo TAP_RC=$?"   # 坐标取自上一步 dump 的 bounds 中心，先现测可注入
# 2) 部署本分支（真运行）：机检行 HX_BUSY / HX_RUN / HX_RUN_DEPLOY（www mtime 相对基线前进）
npm run hx:run                       # 本轮 compile=250s、设备侧到 240s 才前进 ⇒ 耐心等，绝不 kill（坑位 1）
# 3) 改后：同一 UI 路径、同一列表位重复第 1 步
# 4) logcat：**不清缓冲**（与 #1261 不同，见坑位 4），一次全量 dump 后按设备自己的 epoch 收窄窗口
adb shell logcat -d -v epoch > logcat-window.txt
# 5) 判据 5 的「同源」不是手算：本地真执行三层，拿设备侧载荷当输入
node -e "const{forumChain}=require('./utils/forumChainHarness');const d=forumChain().display;
console.log(d.getContentPreview('## 测试 Markdown 渲染\n\n- 无序列表\n\n---\n\n1. 有序列表\n2. 你好\n\n---\n\n```python\nprint(\"hello world\")\n```\n\n---','markdown'))"
# -> 测试 Markdown 渲染 无序列表 有序列表 你好 print("hello world")
```

**坑位（本轮实测）**：

1. **落地慢不等于卡住**：`cli launch` 的编译段本轮 250 秒，之后设备侧 `www` 又过了约 240 秒才前进
   （`hx-run.ps1` 每 60 秒打一行「已等 N 秒（上限 900 秒）」）。**绝不中途 kill** —— #1261 的血账是
   kill 会把设备上的 `www` 推成半截、整个 app 白屏，只能重跑一次 `hx:run` 恢复。
2. **卡片预览的 a11y 文本只在**短**正文时可读**：长正文（本例 91 字符）那一条节点在**未压缩** dump 里
   连 `content-desc` 属性都不存在（打出来是 `desc=<none>`），而同一个源串在**详情页**的 a11y 里读得到（#1257 留档）。
   ⇒ 长夹具的改前/改后判据走**像素面 + 设备侧载荷 + 本地真执行**三面（判据 1/4/5），短夹具走**文本面**（判据 2/6）。
   这不是「哪面能读就用哪面」的凑数：判据 1/2 是**同一夹具、同一列表位**的前后对照，文本面与像素面各证一次。
3. **设备与 HBuilderX 被并发会话共用**：本轮 `HX_BUSY wait=0 result=free`（锁是空的），但改前基线那个构建
   是**并发会话** 11:39 推上去的 ⇒ 「改前 = 不含本票改动」不能靠 sha 自述，只能像判据 1 那样**用现象证**。
   同理，取证期间页面可能被别人切走：本轮每张图落盘前先 dump 校验页身份（`学员论坛` / `个人动态` 在 desc 里）。
4. **不清 logcat 缓冲**：`logcat -c` 会抹掉并发会话正在看的崩溃输出（`scripts/device-capture.ps1` 把这条列进只读红线）。
   本轮改用「一次 `logcat -d -v epoch` 全量 + 按设备自己的时间戳收窄窗口」，窗口起点取部署完成之后的设备时钟，
   故宿主与设备时钟不同步不影响结果。

## 未覆盖面（如实登记，勿读成「已覆盖」）

1. **`pages/forum/my-forum.uvue`（「我的论坛」页）本轮无设备侧证据**：全树 grep 只有 `pages.json` 注册与页面自身
   （`grep -rn "my-forum" --include=*.uvue --include=*.uts --include=*.json .` ⇒ 除自身外仅 `pages.json:261`）
   ⇒ **应用内没有任何入口**，真实 UI 路径走不到；深链在 #1261 实测不可靠且有风险（坑位 1）。
   该承载面由 ③ 承重：`forumContract.test.js` 的接线守护（薄包装 `topicPreview(item)` + 模板消费 + 无第二实现）
   与 `forumBodyBehavior.test.js` 的行为守护（同一个 `getContentPreview`）。⇒ 要这一页的图，需人给一次入口或另立票补深链可靠性。
2. **`知识问答` tab 未证到「源串消失」**：该 tab 与广场**共用同一个** `ForumTopicCard`（`variant='qa'` 只换版面），
   但存量两条帖的 `content_format` 都是 `text`（`07`）⇒ 这里只证到零回归那一半，判据 1/2 由广场承重。
3. **表格 / 图文混排的降级形态未在真机夹逼**：论坛档不声明 `table` / `image` ⇒ 预览里表格退回逐行原文（`|` 仍在）、
   `![](url)` 展开成 alt。这两条由 ③ 行为守护承重；生产存量里没有带表格的 markdown 帖，造一条就等于对生产论坛写内容 ⇒ 不在本轮授权内。
4. **80 字截断边界未在真机夹逼**：存量最长的 markdown 预览是 48 字符（判据 5 那条），够不到 80 字线
   ⇒ 边界（80 / 83）由 ③ 的 `forumBodyBehavior.test.js` 承重。
5. **`赞过` / `互动` / `游览记录` 三个 tab 未取证**：本轮账号（张三）这三格为空（`个人动态` 只有「我的帖子」两条），
   接线由 `personalActivityContract.test.js` 的「六个 tab 全部下发格式轴」守护承重（`<ActivityTopicCard ` 与
   `:content-format="item.content_format"` 各 **6** 处，机检计数）。
