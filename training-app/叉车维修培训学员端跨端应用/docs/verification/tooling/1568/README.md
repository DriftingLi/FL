# #1568 无界 adb 调用收口取证：AC 第 1 条两个单点（16 + 12 个调用方）+ AC 第 2 条 emulator-smoke 的 13 处分档

本目录是 PR 的**可复算产物**目录（本仓纪律：预算数与「还剩几处」都必须指得到入库件，不能只写在注释里）。
§1–§4 的 3b 是 AC 第 1 条（两个收口点，`wireless-debug.ps1` 归零）；§3c 是 AC 第 2 条的第一片
（`emulator-smoke.ps1` 那 13 处按形态分档）。AC 第 2 条**还剩 11 处**：`device-capture.ps1` 6、
`hx-run.ps1` 4、`lib/env-check.ps1` 1 —— 按票面「建议切分」各自成 PR，不混进这两个 PR。

## 1. 复算尺（AC 最后那条防漂移条款的兑现件）

`adb-bounded-count.mjs` 就是票面「现测」段那句方法的**可执行版**：先删块注释、再删整行 `#`，
再按**大括号深度**所在函数归属数 `&\s*\$[aA]db[eE]xe`。三条实现要点各对应本族一次真实的数飘过：
掩码保行号（裸 grep 会把块注释里的反面教材数成调用点）、深度归属而非「最后一行 function」
（#1568 第一版就这样把 3 行记错到 `Publish-PrefilterComment` 名下）、特征不用 `Out-String`
（`emulator-smoke.ps1` 的直调里有 5 处是 `| Out-Null`）。

**先在同一把尺上复现票面的读数，再拿它量改动** —— 这是「开票与收票用同一把尺」的可执行含义：

```
node docs/verification/tooling/1568/adb-bounded-count.mjs
```

| 读数 | 命令输出（合计行） | 逐文件 |
| --- | --- | --- |
| 改前（master `0a8023e5`，与票面 26 处逐格一致） | `ADB_UNBOUNDED_TOTAL calls=26 files=5` | device-capture 6 / emulator-smoke 14 / hx-run 4 / wireless-debug 1 / env-check 1 |
| 改后（AC 第 1 条那个 PR） | `ADB_UNBOUNDED_TOTAL calls=24 files=4` | device-capture 6 / emulator-smoke 13 / hx-run 4 / env-check 1（**wireless-debug 归零**） |
| AC 第 2 条本片改后（本 PR） | `ADB_UNBOUNDED_TOTAL calls=11 files=3` | device-capture 6 / hx-run 4 / env-check 1（**emulator-smoke 归零**：13 → 0） |

改前那份逐函数分布与票面表**逐格相同**（含 `emulator-smoke.ps1` 的顶层脚本段 3 行 = `:665` / `:765` / `:766`），
所以「26 → 24」不是换一个数法得到的：两个收口点各是所在文件的那 1 行「本体」，收掉就各减 1。
「24 → 11」这一跳的**基线是复量过的**：把 `9931a223` 版 `emulator-smoke.ps1`（48,315 字节）临时放回原位
量一把（得 `calls=24 files=4`，emulator-smoke 那行 `calls=13 funcs=8`），再 `git checkout --` 还原，
副本与 `HEAD` blob 逐字节相同（`sha256` 前 16 位同为 `85b2b87bb0bc40de`）⇒ 量的确实是同一把尺、同一份被测物，
不是「改完之后再回头说改前是 24」。

## 2. 预算默认值的现测出处（`adb-latency-probe.ps1`）

```
pwsh -NoProfile -File docs/verification/tooling/1568/adb-latency-probe.ps1 -OutFile docs/verification/tooling/1568/latency-readings.txt
```

`latency-readings.txt` 的原文读数（2026-10-08 本机，Windows NT 10.0.26200 / pwsh 7.6.6 / adb 1.0.41）：

| 形态 | 现测 | 归到哪一档 |
| --- | --- | --- |
| `adb version` | 178 ms | 读数档 15 秒 |
| `adb devices` | 106 ms | 读数档 15 秒 |
| `adb mdns services`（空表） | 103 ms | 端点档 30 秒（发现类要等网络答话，不按这一格定档） |
| `adb connect 127.0.0.1:9`（端口拒绝） | 2,160 ms，rc=**0** | 端点档 30 秒 |
| `adb connect 10.255.255.1:5555`（不应答也不拒绝） | **10,152 ms** | ⇒ 这就是端点档不能套 15 秒的理由：那是「端口换了但还连得上的慢」，不是挂死 |
| 设备侧 `getprop` / `dumpsys` / `settings get` | **本轮未测** | 现测时没有 `state=device` 的 transport ⇒ 读数档引用 #1560 的真链路产物（同一条有界调用返回 646 / 719 / 949 / 1217 ms，且那还是一张约 730 KB 的 PNG，`docs/verification/tooling/1560/README.md:25`、`:48-49`） |

两条附带事实会影响实现，一并记下来：`adb connect` 失败时**退出码是 0** ⇒ 不能拿 `$LASTEXITCODE` 当判据
（本仓口径同 `emulatorSmokeContract` C2）；失败文案是**中文 socket 错误 + (10061)** ⇒ 判据取 ASCII 那截数字，
而 stderr/stdout 必须合并在一份文本里才拿得到（见下面 M2）。

## 3. 成对取证：改前必红 + 改后必不红

**必不红那一半**（改后全绿，`.ci-verify/` 的本地跑记录，非入库件）：

| 套件 | 用例 | 现测耗时 |
| --- | --- | --- |
| `utils/emulatorSmokeBoundedTextBehavior.test.js`（新，ETD1–ETD5） | 5 passed | 22.803 s |
| `utils/wirelessDebugBoundedTextBehavior.test.js`（新，WBD1–WBD5） | 5 passed | 25.098 s |
| `utils/emulatorSmokeContract.test.js`（+C10） / `utils/wirelessDebugContract.test.js`（+W9） | 全绿 | 6.02 s / 5.338 s |
| 同族既有套件（`autoScreenshot*` / `deviceCapture*` / `wirelessDebugBehavior` B1–B6 / `contractTestPattern`） | 118 passed（11 套件一批跑）+ 5 passed | `wirelessDebugBehavior` 81.214 s |

**必红那一半**（逐条把被测物弄坏，driver = `.ci-verify/mut-driver.mjs`（一次性跑批工具，不入库）；
读数全文已入库为本目录 `mut-readings.txt`——`.ci-verify/` 随工作树一起消失，故证据落在这里；
每条随后 `git checkout --` 还原并复验 `restored_clean=true`，末行 `OTHER_DIRTY_AFTER_ALL=0`）：

| 变异 | 弄坏的东西 | 跑的那条守护 | 红的用例（jest `--json` 读出的标题，非「随便一条红」） |
| --- | --- | --- | --- |
| M1 | `Invoke-Adb` 的委托换回 `& $AdbExe … \| Out-String`（无界形状回写） | `wirelessDebugContract`（全跑，36 条） | 2 红：`真文件零违规（九条判据同时成立）` + `W9：… 被拿掉或换形 ⇒ 必须判红`（后者的锚点是委托那行，锚点消失本身也算抓到） |
| M2 | 摘掉 `-MergeStdErr`（stderr 判据原料断供） | `wirelessDebugBoundedTextBehavior -t WBD3` | 1 红：`WBD3 pair 失败分支：adb 把 protocol fault 打在 stderr ⇒ 分类判据照样拿到它` |
| M3 | 端点档接线改成读数档（`-BudgetSeconds $AdbCallTimeoutSeconds`） | `wirelessDebugBoundedTextBehavior -t WBD1` | 1 红：`WBD1 挂死腿：mDNS 发现不返回 ⇒ 到预算给结论…点名哪一次调用与哪一档预算` |
| M4 | `Get-AdbOutput` 的委托换回无界直调 | `emulatorSmokeContract`（全跑，6 条） | 1 红：`真实文件：零违规`（C10 抓到） |
| M5 | 摘掉文本侧的 `-MergeStdErr` | `emulatorSmokeBoundedTextBehavior -t ETD2` | 1 红：`ETD2: 对照腿正常返回时文本照旧回给调用方，stdout 与 stderr 都在` |
| M6 | 调用方强行给 serial（等价于执行核恒拼 `-s <serial>` 那一格） | `wirelessDebugBoundedTextBehavior -t WBD2` | 1 红：`WBD2 对照腿：正常载体 ⇒ ensure 判 connected 且日志里没有一条超时` |

## 3b. 真 adb 链路腿（DoD：工具链改动的验收 = 那个行为在真实链路上成立）

ETD / WBD 用的是可编程假载体（要的是「真挂死」这一格 —— 真 adb 不可按需要挂死）。载体换成
**真 `adb.exe`** 的那一头由 `real-adb-legs.ps1` 补，读数原文在 `real-adb-readings.txt`
（2026-10-08 本机，全程只读、`-StateDir` 指临时目录，不碰维护者在用的状态目录）：

```
pwsh -NoProfile -File docs/verification/tooling/1568/real-adb-legs.ps1 -OutFile docs/verification/tooling/1568/real-adb-readings.txt
```

| 腿 | 现测 | 读到什么 |
| --- | --- | --- |
| R1 默认预算跑真链路 | `RL1_concluded=True`、`RL1_stdout_has_adb_version=True`、`ADB_CALL_BUDGET calls=3 timeouts=0 readBudgetSeconds=15 endpointBudgetSeconds=30 hungArgs=none`、wall 1,743 ms | 真 adb 的 stdout 经有界执行核**读回来了**，工具照常给结论，一条超时都没记 |
| R2 强制腿（预算 0，先例 #1560:25「强制腿只能用预算 0」） | `RL2_concluded=True`、`RL2_timeouts=2`、首条 `ADB_CALL_TIMEOUT args=version callBudgetSeconds=0 seconds=0.1`、wall 1,533 ms | 每次真调用都在 0.1 秒内被判到点并终止，工具**没有挂住**、仍然出结论 —— 「上限真的可达」在真 adb 通道上成立 |
| R3 stderr 合并在真载体上 | `RL3_nonempty=True`、`RL3_HAS_NOT_FOUND=True`、原文 `adb.exe: device 'emulator-5556' not found` | 真 adb 把这句话打在 **stderr** 上；`Get-AdbOutput` 把它原样回给调用方 ⇒ 收口没有换掉调用方拿到的东西 |


## 3c. 仿真机分档真链路腿（AC 第 2 条：`emulator-real-legs.ps1`，读数在 `emulator-real-legs-readings.txt`）

EMT 那九条用的是可编程假 adb（要的是「真挂死」这一格 —— 真 adb 不可按需要挂死）。这一份补载体那一头：
真 `adb.exe` + 真 AVD `Pixel_4a_API_30`，四遍各换一个形态（2026-10-08 本机，`-Pages pages/index/index`
单页、`-NoArchive`，读数全程只取 ASCII token）：

```
pwsh -NoProfile -File docs/verification/tooling/1568/emulator-real-legs.ps1
```

| 腿 | 现测 | 读到什么 |
| --- | --- | --- |
| L1 正常冒烟（`-SkipInstallBaseApk`） | `exit=0`、wall 121 s、`ADB_TIER_BUDGET text=15 logcat=60 boot_wait=120 install=180 push=120 teardown=60 shot=15`、`ADB_TEXT_CALL_BUDGET calls=30 timeouts=0`、`LOGCAT_INCONCLUSIVE=0` | 六档现值打进汇总；整趟 30 次文本调用**一条超时都没记** ⇒ 分档没有把正常等误判成挂死（「长等不得套 15 秒」那格的反面证据） |
| L2 `-AdbLogcatTimeoutSeconds 0` | `exit=1`、wall 129.1 s、`ADB_TEXT_TIMEOUT call=adb -s emulator-5554 logcat -d -v brief callBudgetSeconds=0 seconds=0.1 tier=logcat`（两条）、`LOGCAT_INCONCLUSIVE=1`、`hungCalls=logcat -d -v brief;logcat -d -v brief` | 档是**参数**（0 秒真的生效到点）；挂死/到点没有被子sequent 的计数读成「FATAL=0 ⇒ 无崩溃」，而是落 `INCONCLUSIVE=1` 且整趟判 FAIL |
| L3 `-AdbInstallTimeoutSeconds 0` | `exit=2`、wall 131.9 s、`install_tier=True`（点名 `call=adb -s … install -r -t <基座路径>`、`callBudgetSeconds=0`） | install 到点后整条链以 UNUSABLE 收口，**不永挂**（收尾那两格照旧跑到底） |
| L4 中文路径 APK 走新链路 | `exit=2`、wall 112.6 s、失败原文回到调用方。件里逐字（驱动把非 ASCII 脱敏成 `?`，真实路径是 `D:\软件\…`）：`adb.exe: failed to install D:\??\HBuilderX.5.23.2026080626\HBuilderX\plugins\uniappx-launcher\base\android_base.apk: Failure [INSTALL_FAILED_NO_MATCHING_ABIS: Failed to extract n…` |
| 收尾标记 | `LEGS_DONE=4/4 legs_produced_log=4` | 带分母（见下面第二条洞） |

⚠️ **一次真实撞上的 adb 通道不应答**（同日 12:22 那一轮，同一份代码、同一套参数）：L1 得
`exit=1`、wall 199.5 s、`timeout_count=3`、该页 `SKIP`、`EMULATOR_SMOKE_RESULT=FAIL`，三条到点的调用逐字是
`shell dumpsys activity activities` / `shell dumpsys window` / `shell pidof io.dcloud.simple`，
各 **15.2 / 15.3 / 15.2 秒**打在 15 秒文本档上；12:29 重跑同一腿 `exit=0`、30 次调用零超时。
两趟合起来才是这一格的全貌：**通道挂住是这台机器上会真实发生的事**，而有界化把它的下场从「整趟不返回」
换成「点名哪一次调用、多大预算、哪一档 + 该页记不入结论」——这正是 AC 第 4 条要的形状。
文本档 15 秒的定档依据（稳态空载仿真机 dumpsys 1.17 / 1.66 秒）没有因此改动：**没有证据说它常态不够**，
只有证据说它偶尔会到点，而到点这条路已经被上面这两趟各自覆盖。
⚠️ 两趟的**可复算程度不一样**，别写成一样。12:29 那轮整件入库；12:22 那轮当时**没入库**（驱动跑完 L1
就死在下面第 3 条洞的非法正则上），本轮把它唯一留下的汇总行以 `APPENDIX_prior_round` 附录并进同一件
（`exit=1 wall_seconds=199.5 timeout_count=3 result=FAIL inconclusive=0`）⇒ 这几格现在可复算；而**三条超时的
名字**（`dumpsys activity activities` / `dumpsys window` / `pidof`）当时只存在于子进程 stdout，且那一轮的
RAW 抓行有下面第 2 条「只抓到 token 为止」的缺陷、子日志随后被下一轮固定名覆盖 ⇒ **名字这一层是会话内
观测，仓内不可复算**。把这条说白，是为了不让下一个人拿这三个名字当判据去改文本档。

**判别力实测（N 系列，逐条弄坏被测物本体 `scripts/emulator-smoke.ps1`，还原后逐字节比对）**：
`PRE_PRODUCT_DIRTY=0`，九条全部 `exit=1` 且 `restored_clean=true bytes_match_commit=true`（驱动
`.ci-verify/mut-driver-1568b.mjs`，jest `--json` 取红腿名，绕开 pwsh 中文经码页变乱码那条血账）。

| 变异 | 弄坏什么 | 红在哪 |
| --- | --- | --- |
| N1 | install 点位摘掉自己的 `-BudgetSeconds`（档写了却没接 = 那处仍无界） | 契约 C11 两格 |
| N2 | install 档被塞成 15 秒（长等当挂死） | 契约 C11 两格 |
| N3 | `Get-LogcatSummary` 不带出 `TimedOut` | 行为 EMT5 |
| N4 | logcat 超时不再计入该页失败（改记 skip） | 契约 C11 两格 |
| N5 | version 的 `-NoSerial` 丢了（server 级命令被拼 `-s`） | 行为 EMT8 |
| N6 | 收尾 `wait-for-disconnect` 被摘出主 `finally` | 契约 C11 两格 |
| N7 | boot-wait 换成文本档 | 行为 EMT7 |
| N8 | version 点位绕开 `Get-AdbVersionBrief` 自己抄一遍 | 契约 C11 两格 |
| N9 | 把一处文本调用改回 `& $AdbExe … \| Out-String` | 契约 C11 清零锁 |

N8 第一版指到 `-t EMT8`，现测 **`exit=0 failed=0`** —— 函数**定义**还在，行为腿用 AST 抽定义直调，抽得到就照样绿。
⇒ 「点位与函数脱钩」这一格的判据只能在**接线层**（`SITE_THROUGH_FN`：名字出现次数 + 点位正则），
改指契约套件后才红。这条与本票 N5 的教训是同一件事的两面：行为腿证「函数对不对」，接线层证「点位走没走它」。

两条口径跟着这张表一起说清：

- ⚠️ **teardown 档没有挂死腿**（EMT1–EMT9 里没有一条真把 `emu kill` 挂住）：那两格在主 `finally` 段里，
  本族 AST 抽函数的手法打不到（要打得先把收尾抽成函数）⇒ 现由 C11③/⑤ + 变异 **N6** 在**接线层**锁。
  这是缺口，不是「已覆盖」，写在表旁边免得被读成成对九条各档都有。
- 定档现测里 push 那两个数（69.7 / 111.1 ms 每 MB）量的是**合成 blob**：探针 `emulator-tier-probe.ps1:148`
  用 `[System.IO.File]::Create` 造 48 MB 的 `blob.bin` 推到 `/data/local/tmp/tierprobe`，读数件自注
  `note=unpackage_resources_absent`、实测 `push_dir_proxy ms=3346` ⇒ 它量的是 **adb 流式传输的速率**，
  不是「真 www 资源目录」那一趟的耗时（120 秒这一档是按该速率外推的上界）。引用时别说成量过真资源。

### 取证驱动自己那三处洞（照实在这里记，别再有人信 `LEGS_DONE`）

1. `$Root` 上溯写成**三级**（`docs/verification/tooling/1568` 到工程根实为四级）⇒ `$Smoke` 指向
   `docs\scripts\emulator-smoke.ps1` 这个不存在的路径 ⇒ 四条腿全部 `exit=90 wall_seconds=0`、日志零行，
   而读数件照样打满四行并以 `LEGS_DONE=1` 收尾。**差一点就被当成本票的真链路证据引用**。
   现在：四级 + `Test-Path $Smoke` 先 throw + 末行带分母（`LEGS_DONE=4/4 legs_produced_log=4`）。
2. RAW 抓行正则写成 `(?m)^.*TOKEN` ⇒ 只抓到 token 为止，`ADB_TEXT_TIMEOUT` 行里的
   `call=` / `callBudgetSeconds=` / `tier=` 全丢 ⇒ 读数只剩 `timeout_count=3` 这一个数，答不出是哪三条。
   现在抓整行（`^.*TOKEN.*$`）。
3. `pats` 里 `'Failure ['` 是**非法正则**（Unterminated [] set）⇒ L1 跑完后进 RAW 循环当场炸掉，
   L2–L4 从未执行。现在转义成 `Failure \[`，并把六条 pattern 提到脚本级**开跑前逐条真编译**
   （非法就 `BAD_RAW_PATTERN` 当场 throw，打印 `RAW_PATTERNS_COMPILED=6`）—— 驱动自己的错要在浪费一轮
   仿真机之前就红，不能等产物。

读数件 `emulator-real-legs-readings.txt` = 本轮 12:29–12:33 那一趟（四腿 + 分母行俱全）。驱动此后又加了
两处（腿 stdout 按 `$RunStamp` 分组、RAW 开跑前自检），**不改变读数件的字段与数值**；重跑会得到同名 token、
不同分组文件名。

另有两处是本轮重跑时撞上的证据销毁：腿的 stdout 用固定名，下一轮同标签直接覆盖上一轮
（12:22 那轮的三条原文就是这么没的）⇒ 现在按 `$RunStamp` 分组；变异驱动 `git checkout --` 还原
被测物，所以**跑变异前必须先提交**（本轮 `BASE_HEAD=d5694bfd`、`PRE_PRODUCT_DIRTY=0`）。

## 3d. AC2 第二片：`device-capture.ps1` 的 6 处（2026-10-08）

同一把尺，改前改后各跑一次（`node docs/verification/tooling/1568/adb-bounded-count.mjs`）：

| 读数 | 命令输出（合计行） | 逐文件 |
| --- | --- | --- |
| 改前（master `704fd336`） | `ADB_UNBOUNDED_TOTAL calls=11 files=3` | device-capture **6** / hx-run 4 / env-check 1 |
| 改后（本 PR，改后 head 见下面「复现」段的现测命令） | `ADB_UNBOUNDED_TOTAL calls=5 files=2` | hx-run 4 / env-check 1（**device-capture 归零**） |

`device-capture.ps1` 那 6 处改前逐函数各 1 行（`Get-DeviceList` `:270` / `Get-ForegroundInfo` `:323` /
`Get-LogcatBaseline` `:343` / `Get-LogcatWindow` `:351` / `Resolve-LauncherComponent` `:459` /
`Start-AppPage` `:471`），归零后本文件不再出现在尺的输出里 ⇒ 「11 → 5」与「24 → 11」是同一把尺的两个读数，
不是换了数法。

### 六处的形态与「空」该给哪种结论

| 点位 | 命令 | 档（默认值） | 挂死时拿到的「空」 | 结论落点 | 只读性 |
| --- | --- | --- | --- | --- | --- |
| `Get-DeviceList` | `devices -l`（**server 级**，不带 `-s`） | text 15 s | 空列表 | **自己的 exit 2 出口**，排在「没有设备」之前 | 只读 |
| `Get-ForegroundInfo` | `shell dumpsys activity activities`（**每页都跑**） | text 15 s | Component 空 | 既有失败条目不动 + 补一条点名 `tier=read` | 只读 |
| `Get-LogcatBaseline` | `shell logcat -d -v epoch -t 1` | logcat 60 s | Epoch = 0 | 既有 `logcat=SKIP` **不升级**，只在提示里点名成因 | 只读 |
| `Get-LogcatWindow` | `shell logcat -d -v epoch`（全量） | logcat 60 s | 两个计数 0 | **必须落进 `$failures`** + `LOGCAT_INCONCLUSIVE=1` | 只读 |
| `Resolve-LauncherComponent` | `shell cmd package resolve-activity …` | text 15 s | 无匹配 | 走既有第二层回退（当前前台），点名行分开两种「没匹配」 | 仅 -AllowAppStart 可达 |
| `Start-AppPage` | `shell am start …` | text 15 s | 空原文 | 不抛不吞、原文行照打、流程继续 | 既有写路径，只加预算 |

档位默认值的现测出处（逐字指得到本目录的入库件，不抄进代码注释之外的地方）：

| 形态 | 现测 | 出处 |
| --- | --- | --- |
| `adb devices` | 106 ms | `latency-readings.txt` |
| `adb version` | 178 ms | `latency-readings.txt` |
| `shell dumpsys activity activities` | 1,168 / 1,662 ms | `emulator-tier-readings.txt` / `-run2.txt` |
| `shell cmd package resolve-activity` | 457 / 336 ms | 同上 |
| `logcat -d`（全量） | 5,896 / 6,186 ms | 同上 ⇒ 15 秒只剩 ≈2.4 倍 ⇒ 单列 60 秒档 |
| `logcat -d -v epoch -t 1` | **没有独立现测** | 与全量同档（同形态里更小的那一个）—— 这条是记账，不是量过 |
| `am start` | **没有独立现测** | 同族 AM 往返（resolve 两列）+ emulator-smoke 里同形态走文本档的先例 |

⚠️ 仿真机与真机不同机：上表除前两行外都来自 AVD `Pixel_4a_API_30` ⇒ **真机的 dumpsys / logcat 余量没量过**。
本片按仿真机现测给档（15 秒 ≈ 最慢现测的 9 倍、60 秒 ≈ 10 倍）；若真机现测更慢，**改档必须带新读数**。

### 成对取证（红相 / 绿相各一趟，同一套腿）

| 相 | 命令 | jest 自己的判定行 | 墙钟 |
| --- | --- | --- | --- |
| 红（接线前） | `npx jest --config jest.config.unit.js utils/deviceCaptureTierBehavior.test.js` | `Tests: 10 failed, 3 passed, 13 total` | **263.771 s** |
| 绿（接线后） | `npx jest --config jest.config.unit.js utils/deviceCapture`（三套件：Contract + BoundedShot + Tier） | `Tests: 30 passed, 30 total` / `Test Suites: 3 passed, 3 total` | **50.702 s** |

红相那 10 条**全部是断言「超时结论存在」的腿**（DCT1、DCT3–DCT11），过的 3 条是不依赖该结论的
DCT0（pwsh 可用见证）、DCT2（正常返回对照腿）、DCT12（多设备守卫 —— 改动前后都 exit 2）
⇒ 红不是因为腿写坏，是因为当时**真的没有单次超时**：那几条腿各等满假 adb 桩的 30 秒寿命
（`hangFor` 命中 ⇒ `setTimeout(…30000)`），再被 `execFileSync` 的上界收掉。
**同一族腿的墙钟从 263.8 秒降到 50.7 秒**（且后者多跑了两套守护）—— 这个下降本身就是「有界生效」的读数；
中间那一趟 `45.217 s`（1 failed / 29 passed）是 DCT1 的原因行正则写宽了（`(\S+)` 连 `）⇒` 一起吃掉），
与产品无关，改正后才是上面那行 30/30。

### 判别力：变异电池（逐条弄坏被测物，M1–M9，三趟）

原始读数件：`device-capture-mut-readings.txt`（工具输出原样拷贝，每条含 `landed=` / `JEST` / `RED` / `REVERT`）。
三趟的来历要写清：**pass1 跑完 M1–M6，M7 那条在它的 jest 执行中被本会话的一次 `git checkout` 打断 ⇒ 读数作废**；
pass2 重跑 M7–M9；pass3 是 M3 在契约锚改成**函数体 scoped** 之后的重跑（第一趟 M3 只有行为腿打红，理由见下表）。

| 变异 | 弄坏的那格 | 打红的守护（jest 自己的判定行数） | 读数 |
| --- | --- | --- | --- |
| M1 | server 点位丢 `-NoSerial` | 契约「真实文件：零违规」+ DCT1 + DCT2（3 failed / 30） | argv 判据真管用 |
| M2 | logcat 点位接成文本档 | 契约 + DCT5（2 failed） | 腿实参给 `text=3 / logcat=2` ⇒ 读到 3 即红 |
| M3 | 全量 logcat 不带出 `TimedOut` | pass1：只有 DCT6 + DCT7 + DCT11（3 failed，**契约不打红**）<br>pass3（锚 scoped 后）：契约 + 同三条（4 failed） | 全文级锚被另三处同形文字满足 ⇒ 当场改成 `Get-LogcatWindow` 函数体锚；险处不许只靠一层 |
| M4 | 窗口挂死不再落 `$failures` | **仅**契约（1 failed） | 如实记：行为腿用 AST 抽函数，main 不在射程 ⇒ 这一格契约独家 |
| M5 | 摘 `-MergeStdErr` | 契约 + DCT13（2 failed） | stderr 原文是判据原料 |
| M6 | 点名行 `tier=$Tier` 改成常量 | 契约 + DCT1/3/5/6/8/9（7 failed） | 每条挂死腿都依赖它 |
| M7 | devices 的挂死出口并进「没有设备」 | 契约 + DCT1（2 failed，pass2 重跑） | 原因行不再带 `tier=server` |
| M8 | 一处改回 `& $AdbExe … \| Out-String` | 契约的**整文件归零棘轮** + DCT8（2 failed） | DCT8 那一腿墙钟从 3.4 秒变 **31.2 秒** —— 无界形状的直接读数 |
| M9 | 从 pattern 里摘掉 `deviceCapture` token | `contractTestPatternBehavior` P1 + P2（2 failed / 5） | 「子串顺带命中」这件事本身有守护 |

### 在册性与真链路腿的边界

- 在册性现测：`npx jest --config jest.config.unit.js --listTests` 里子串 `deviceCapture` 已列出
  **三个**套件（`Contract` / `BoundedShotBehavior` / `TierBehavior`）⇒ **不新增格子**；M9 是这条裁定的反面见证。
- 真链路腿：`device-capture-emulator-legs.ps1`（真 adb + 真仿真机，三条腿：L1 默认预算对照 /
  L2 只把 logcat 档给 0 / L3 只把文本档给 0），读数在 `device-capture-emulator-readings.txt`。
  ⚠️ **①a 的真机取证本轮未跑**：无线调试离线、重开要人在手机上操作 ⇒ 本片的判据不依赖它，
  也不把它写成「已验」。仿真机替代的是「真 adb 的答复仍被原有解析器读懂 + 到点给可见结论」这两格。
- 驱动自己的四条纪律（四级根路径 + `Test-Path` 先 throw、每腿 `WaitForExit(ms)` 硬上界、子进程 stdout 按
  `$RunStamp` 分名、RAW 整行与开跑前逐条编译自检）都是从上面 §3c 那三处洞抄过来的 —— 见其 SYNOPSIS 末段。

## 4. 在册性（AC 第 5 条：先现测 `scripts/lib/contract-tests.ps1` 再决定）

- AC 第 2 条新增的行为套件 `emulatorSmokeTierBehavior` 同样落在子串 `emulatorSmoke` 的射程内
  （现测 `npx jest --config jest.config.unit.js --listTests` 该子串列出四个套件：`Contract` /
  `BoundedShot` / `BoundedText` / `TierBehavior`）⇒ **不新增格子**。
- `emulatorSmoke` token 是**子串**匹配 ⇒ 顺带命中新增的 `emulatorSmokeBoundedTextBehavior`，**不新增格子**
  （先例 #1560「追加进已在册的套件」）。
- `wirelessDebug*` 原本**两个都不在册** ⇒ 本票**补一个窄 token** `wirelessDebugBoundedText`。
  成本现测：窄的这条 25.1 s；宽 token `wirelessDebug` 会顺带命中 #1564 的 `wirelessDebugBehavior`
  （81.2 s，含真起后台 keep 再 stop 的腿）⇒ 取窄的。#1564 那两条维持不注册（那是它自己 PR 的裁定）。
- 由 `utils/contractTestPatternBehavior.test.js` P1–P3 兜：P1 逐字钉 pattern、P3 断言每个 token 都真能选中
  至少一个套件（防「加了 token 选不到守护」与「加了守护没加 token」两个方向）。

## 5. 复现这些数的最短路径

```
cd training-app/叉车维修培训学员端跨端应用
node docs/verification/tooling/1568/adb-bounded-count.mjs
pwsh -NoProfile -File docs/verification/tooling/1568/adb-latency-probe.ps1
pwsh -NoProfile -File docs/verification/tooling/1568/real-adb-legs.ps1
pwsh -NoProfile -File docs/verification/tooling/1568/emulator-tier-probe.ps1   # 定档现测（要真 AVD）
pwsh -NoProfile -File docs/verification/tooling/1568/emulator-real-legs.ps1    # AC 第 2 条四腿（要真 AVD，约 8 分钟）
npx jest --config jest.config.unit.js utils/emulatorSmokeBoundedTextBehavior.test.js utils/wirelessDebugBoundedTextBehavior.test.js
npx jest --config jest.config.unit.js utils/emulatorSmokeTierBehavior.test.js utils/emulatorSmokeContract.test.js
```

本目录不钉自己的 sha（文件不可能含自身 blob 值）；要核对读者拿到的是哪一份，按 PR 的 head sha 复算：

```
git cat-file blob <head-sha>:./docs/verification/tooling/1568/adb-bounded-count.mjs | sha256sum
```

⚠️ 口径提醒（本族记过的坑）：复算 blob 用 `git cat-file blob <sha>:./<路径>`，**不是** `git hash-object`
（后者给 SHA-1，与这里的 SHA-256 对不上）。两列是否相同取决于 `.gitattributes`：`*.ps1`（根 `.gitattributes:28`）
与 `*.mjs`（`:23`）**都钉了 `text eol=lf`** ⇒ 副本与 blob 逐字节相同、两列同值；
`.txt` **没有**该规则 ⇒ 在 `core.autocrlf=true` 的机器上检出成 CRLF ⇒ blob 是 LF、副本是 CRLF，两列必不同。
本机 2026-10-08 实测（blob 侧的 ref = `9be9a16e`，sha256 前 20 位）：

| 文件 | 副本 | blob | 判定 |
| --- | --- | --- | --- |
| `adb-bounded-count.mjs` | `86dbbbfd8e916e901f10` | `86dbbbfd8e916e901f10` | 同值（`.mjs` 被 `:23` 钉 LF） |
| `latency-readings.txt` | `9c5871e6b0953b2784c5` | `83b69095dc2203884b4e` | **不同**（`.txt` 未钉 ⇒ 副本 CRLF、blob LF） |

复算命令：

```
sha256sum docs/verification/tooling/1568/adb-bounded-count.mjs
git cat-file blob 9be9a16e:./docs/verification/tooling/1568/adb-bounded-count.mjs | sha256sum
```


