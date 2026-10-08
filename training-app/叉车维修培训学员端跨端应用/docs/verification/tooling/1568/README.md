# #1568 收口点取证：`Invoke-Adb`（16 个调用方）与 `Get-AdbOutput`（12 个调用点）接上唯一执行核

本目录是 PR 的**可复算产物**目录（本仓纪律：预算数与「还剩几处」都必须指得到入库件，不能只写在注释里）。
射程 = 票面 AC 第 1 条的两个单点；AC 第 2 条那 24 处直调不在本 PR。

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
| 改后（本 PR） | `ADB_UNBOUNDED_TOTAL calls=24 files=4` | device-capture 6 / emulator-smoke 13 / hx-run 4 / env-check 1（**wireless-debug 归零**） |

改前那份逐函数分布与票面表**逐格相同**（含 `emulator-smoke.ps1` 的顶层脚本段 3 行 = `:665` / `:765` / `:766`），
所以「26 → 24」不是换一个数法得到的：两个收口点各是所在文件的那 1 行「本体」，收掉就各减 1。

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


## 4. 在册性（AC 第 5 条：先现测 `scripts/lib/contract-tests.ps1` 再决定）

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
npx jest --config jest.config.unit.js utils/emulatorSmokeBoundedTextBehavior.test.js utils/wirelessDebugBoundedTextBehavior.test.js
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


