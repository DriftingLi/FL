# 截图 diff 真链路成对取证（#1542）

被测对象：`scripts/dev-finish.ps1` **步骤 7「截图对比」** 的像素判据
（`scripts/lib/png-diff.mjs` → `Compare-ScreenshotBaseline` → `Get-PngDiffVerdict`）。

设备 `b32d8398`（23049RAD8C，1080×2400，Android 15）· `HEAD=ebab6af0` ·
命令 `pwsh scripts/dev-finish.ps1 -Level standard -Device b32d8398` · 日期 2026-10-05。

## 为什么必须成对

只跑「通过」的那一次证明不了判据有牙 —— 一个恒绿的门同样会「通过」。
所以同一条链跑三轮，**全程不加 `-UpdateBaseline`**（加了就是人裁决过的绿，判不出牙）。

票面写「两轮」，这里跑三轮：比较需要既有基线，**A 轮是前置**（走 `NewBaseline=true ⇒ write-baseline` 那一支），
判据轮是 **B / C**。

## 三轮读数

| 轮 | 工作树状态 | 步骤 7 机检行 | 退出码 |
| --- | --- | --- | --- |
| A | 只有零视觉效应锚点注释 | `首次运行：基线为空 ⇒ 已自动以本轮截图建立基线（1 页）；此后每轮都会真比` | **0** |
| B | 与 A **逐字节相同** | `📄 settings.png — 无变化（像素差 0.056% / 阈值 0.5%）` → `所有页面无变化` | **0** |
| C | B + 一行样式（`.header` 底色 `#ffffff` → `#ff0000`） | `⚠️ settings.png — 有变化（像素差 6.006% / 阈值 0.5%）` → `1 页有变化且未确认` | **1** |

⇒ **同内容必不红 + 超阈值必红，两条都在真链路上成立。**

### 截图与哈希（本目录三个文件）

| 文件 | 字节 | sha256 |
| --- | --- | --- |
| `round-A-baseline.png` | 53,683 | `23f321989e0566d1943ce33cf0b061f34ed4a0710ede97898a460b1cd1819dd7` |
| `round-B-current.png` | 53,321 | `dfd0634d095c6c01c57979d6075127aae2a704f62177f7d8e082306aa292174c` |
| `round-C-current.png` | 52,728 | `7bd28bba9202efd1db376be7859211a3297cc3f14e3a2b706d39171effae8936` |

**B 轮不是「拿同一张图跟自己比」。** A 轮基线与 B 轮当前截图**字节不同、sha 不同**
（状态栏时钟 14:28 → 14:35、网速读数变了）——**旧的 MD5 判据在这里会直接判「有变化」**；
像素判据算出 0.056% ≤ 0.5%，放行。这正是 `png-diff.mjs` 头顶注释里 Q17 建像素层要买的东西，
现在它有真机数据撑着，不再只是注释里的一句话。

C 轮判红后基线**没有被刷新**：`round-A-baseline.png` 的 sha 在 C 轮前后一致
（`.ci-verify/baseline/settings.png` 仍是 `23f32198…`）⇒ 判红没有顺手把红盖成绿。

## 复算（不接设备）

```
node scripts/lib/png-diff.mjs --a round-A-baseline.png --b round-B-current.png --threshold 0.005
node scripts/lib/png-diff.mjs --a round-A-baseline.png --b round-C-current.png --threshold 0.005
```

实测输出：

```
B：{"ok":true,"mode":"pixel","width":1080,"height":2400,"total":2592000,
    "different":1456,"ratio":0.000562,"threshold":0.005,"changed":false}
C：{"ok":true,"mode":"pixel","width":1080,"height":2400,"total":2592000,
    "different":155683,"ratio":0.060063,"threshold":0.005,"changed":true}
```

⇒ `0.000562` 对上轮内机检行的「0.056%」，`0.060063` 对上「6.006%」——同一判据、同一份图，
   轮内路径与离线路径一致。C 轮的 `changed=true` 就是那个**非零退出码的可复算来源**。

**把状态栏整条排除后再判一次，C 轮仍然红：**

```
node scripts/lib/png-diff.mjs --a round-A-baseline.png --b round-C-current.png \
      --threshold 0.005 --ignore-top-rows 127
{"ok":true,"mode":"pixel","total":2454840,"different":130262,"ratio":0.053063,"changed":true,
 "ignoredRows":{"top":127,"bottom":0}}
```

⇒ 这条红是 `.header` 换色本身造成的，**不依赖状态栏读数**（判据不是靠噪声吃饭）。
⇒ 顺带说明 B 轮为什么不需要 `-IgnoreTopRows`：它连状态栏一起比也只有 0.056%。

## 夹具面积预算 vs 实测

夹具选 `.header` 换底色，是因为实测发现 **0.005 这个阈值比某些真实改动还松**：拿仓内已入库的
真机 before/after 对（`docs/verification/jobs/1327/`，一次真代码改动）跑同一判据，
`job-detail` 是 `ratio 0.000782`、`job-list` 是 `0.00103`，**两个都 `changed:false`**。
⇒ 「改一行样式」若挑的是小面积元素，测不出东西（分不清是夹具太弱还是判据没牙）。

按 `1rpx = 1080/750 = 1.44px`、`.header{padding:16rpx 32rpx}` + `.header-title{font-size:36rpx}`
+ `border-bottom:1rpx`，标题行高取 1.2–1.5 估 header 高 110–125px ⇒ 预算 `ratio 0.046–0.052`
（阈值像素数 = `0.005 × 2,592,000 = 12,960`）。实测 `different=155683` ⇒ `ratio 0.060063`，
**是阈值的 12.0 倍**，比预算偏大（标题文字、返回箭头、下边框都跟着变），方向和量级都对，预算偏保守。

## 与既有守护的边界（别把这份产物当成重复劳动）

「判据有没有牙」在**纯函数 + 合成 PNG** 那一层早就被钉住了 ——
`utils/screenshotDiffBehavior.test.js` 用 pwsh **真跑** `Get-PngDiffVerdict`：

| 用例 | 断言 | 对应本目录哪一轮 |
| --- | --- | --- |
| G1 | 基线为空 ⇒ `action=write-baseline`、`exit=0` | A |
| G2 | `pixelRan=True / mode=pixel / changed=0 / exit=0` | B |
| G3 | 恰 1 像素不同（0.012% < 0.5%）⇒ 不红 | 阈值下界 |
| G4 | 40×40 不同（19.5% > 0.5%）⇒ `ok=False / exit=1` | C |

**本票的增量是真链路那一层**：真机 PNG 过 `dev:finish` 步骤 6→7 的解码子集、时间戳新鲜度过滤
（`Select-ThisRunShots` 只判当前侧）、页身份（日志里的 `进入页面:"pages/profile/settings"`）、
非黑帧与亮屏前置、以及**状态栏噪声在真设备上到底会不会把「同内容」判红**。这些 G2/G4 都碰不到。
⇒ 反过来，这份产物**不**证明判据逻辑本身正确（那由 G1–G4 守着），它证明的是**这条链在真机上会真的红**。

## 已知边界与如实记的缺陷

1. **只覆盖 1 页**（`pages/profile/settings`）。步骤 6 的目标页集合从 `git diff` 推导，
   锚点只让这一页进入集合；`-MaxPages=5` 未触发裁剪。多页/其他页型（含远程图、定时器的页）未测。
2. **A 轮是首次建基线** ⇒ 它验的是 `write-baseline` 那一支，不是「比出无变化」。判据轮是 B/C。
3. **取证用的锚点注释与 C 轮样式行都不进改动集**（当场还原）。
   锚点若留着，改动集会含 `*.uvue` ⇒ 命中运行时面，与本票「未命中运行时面」的门归属冲突。
4. **归档脚本 `run-round.ps1` 有个缺陷**：它把 `screenshots/` 与 `baseline/` 的 PNG 拷进同一个
   `round-<X>/` 目录，两边文件名都叫 `settings.png` ⇒ **基线的拷贝覆盖了当前截图的拷贝**。
   表现：核对时 `round-B/settings.png` 的 sha 等于 A 轮基线，与轮内日志的 `DFD0634D…` 矛盾。
   已当场从 `.ci-verify/` 把各轮真图捞出另存，本目录三个文件的 sha 即捞回后的现测（与上表一致）。
5. **原始控制台日志未入库**：包装脚本没像 `auto-screenshot.ps1` 的导航包装那样钉住
   `[Console]::OutputEncoding = UTF8`，落盘的 `dev-finish.stdout.txt` 是 UTF-8 被按 GBK 解释的
   mojibake（`级别判定` → `绾у埆鍒ゅ畾`），`iconv -f UTF-8 -t GBK` 只能部分还原（不可映射字符已被
   替换成 `?`，不可逆）。⇒ 本文件里引用的机检行是**还原后人工核对**的文本，不是原始字节。
   这条是仓库里已记录的同一个坑（`auto-screenshot.ps1` 的 wrapper 注释）在我这次包装上的复发。
6. **步骤 3 曾被墙钟假红挡过一次**（同一命令第一次跑：`hxLaunchDetachBehavior` 的「<8 秒收口」和
   `concurrent401Refresh` 的「5 秒超时」两条红，`Time: 757.841 s`）。归因：一个父进程已死的
   `grep.exe` 孤儿在烧 93% 一个核、跑了约 14.7 小时，把空闲内存压到 6 MB。
   清掉后同一条 detach 测试从 7.822 秒（对 8 秒预算只剩 0.18 秒）回到 3.9 秒，三轮全绿。
   ⇒ 与本票被测对象无关，但**它足以让成对取证整轮白跑**，故记在此处。
