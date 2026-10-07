# PR #1562 取证产物：有界单次调用在两条真链路上的读数

被测对象：`scripts/lib/auto-screenshot.ps1`（执行核 `Invoke-BoundedAdbCall` + 两个薄封装 + `Test-ScreenAwake`）、
`scripts/device-capture.ps1`（①a 取证截图）、`scripts/emulator-smoke.ps1`（仿真机冒烟截图）。
发货代码 = 分支 `feat/1562`（三个 commit：`30787339` 实现+守护 / `d3d50cf5` token 登记 / `21ed024b` ADR 回记；
本取证目录另起一个 docs commit，不在上面三个里）。

取证环境：本机 Windows + `D:\android-sdk`（adb 1.0.41 / emulator 37.1.11.0，WHPX 可用），
AVD `Pixel_4a_API_30`（x86 / API 30 / 1080×2340）。**基座 APK 装不上这台 x86 AVD**
（`INSTALL_FAILED_NO_MATCHING_ABIS: Failed to extract native libraries, res=-113`，arm 包）
⇒ 冒烟腿带 `-SkipInstallBaseApk` 跑到截图那一步（**取的是 adb 真实往返，不宣称门通过**）。

## 腿与读数

| 腿 | 命令要点 | 机检行原文（逐字摘自同目录日志） | 结论 |
| --- | --- | --- | --- |
| L1 冒烟对照腿 | 默认预算 15、两页 | `SHOT_CALL_BUDGET calls=2 timeouts=0 callBudgetSeconds=15 timedOutPages=none` | 两张图都出，无一次超时 |
| L2 冒烟强制腿 | `-AdbCallTimeoutSeconds 0`（每次调用必挂） | `SHOT_CALL_BUDGET calls=2 timeouts=2 callBudgetSeconds=0 timedOutPages=pages/index/index,pages/login/login`；`SHOT_CALL_TIMEOUT page=emulator-pages-index-index.png callBudgetSeconds=0 partBytes=0 finalWritten=False` | 到点给结论、**第二页照旧跑**、整腿 85 秒自然收口（旧写法在这里会一直等） |
| L3 ①a 对照腿 | `-Device emulator-5556`，默认预算 15 | `SHOT_CALL_BUDGET calls=1 timeouts=0 callBudgetSeconds=15 timedOutPages=none`；`DEVICE_CAPTURE_RESULT=PASS` | 真图 574.3 KB / `sha256=65E158FE6488…` |
| L4 ①a 强制腿 | `-AdbCallTimeoutSeconds 0` | `SHOT_CALL_BUDGET calls=1 timeouts=1 callBudgetSeconds=0 timedOutPages=pages-login-login-current.png`；`断言失败：截图调用未在 0 秒内返回（pages-login-login-current.png）—— 是 adb 通道不返回，不是设备没亮屏`；`DEVICE_CAPTURE_RESULT=FAIL` | 该页记 FAIL、3 秒收口；证据名上**没有**文件（`截图：pages-login-login-current.png=0B/n/a`） |
| R1 冒烟对照腿（重跑留图） | 同 L1，跑在强制腿之后 | `SHOT_CALL_BUDGET calls=2 timeouts=0`；`shotTimedOut=False` ×2 | `emulator-pages-index-index.png` 22,064 B / `magic=89504E47` / **可解码 1080×2340** / `sha256=1FD08757D3400292…` |
| R2 ①a 对照腿（重跑留图） | 同 L3，`-Device emulator-5558` | `SHOT_CALL_BUDGET calls=1 timeouts=0`；`DEVICE_CAPTURE_RESULT=PASS` | `pages-login-login-current.png` 588,756 B / `magic=89504E47` / **可解码 1080×2340** / `sha256=EC8BF3E15A7BA233…` |

⚠️ **两条腿的 `RESULT` 与截图无关，别把它们读成「门过了」**：R1 的 `EMULATOR_SMOKE_RESULT=FAIL` 来自
既有断言「前台 Activity 不是基座（`com.google.android.apps.nexuslauncher/…SecondaryDisplayLauncher`）」——
未装基座、也没给 `-ResourcesDir`，`am start` 到不了目标页（该页在本脚本里按 SKIP 处理）。
L1 与 R1 用同一组参数却给出 `PASS` / `FAIL` 两种 `RESULT`，差别就在前台 Activity 那一条，
**不在截图调用那一条**（两条腿的 `timeouts` 都是 0、两张图都出了）。
⇒ 本表只对「一次 adb 调用有没有界、残帧进不进证据名、超时之后循环继续不继续」这三件事负责。

⚠️ **强制腿在真链路上残帧恰好是 `partBytes=0`（删掉了）** —— 所以「不进证据名」这件事**不能**靠这一轮证明：
#1560 已实测 `Kill` 之后删除与子进程句柄有竞争、可能删不掉。那条防线由夹具与变异证明（见下表 M4/M5：
把逻辑闸门拆掉，同一形状当场被数成证据目录里的 `*.png`）。真链路这一轮只证明「到点给结论 + 循环继续 + 
`finalWritten=False`」。

## 判别力实测（逐条弄坏被测物，每次 `git checkout --` 还原并复验 `git status` 为空）

| 变异 | 弄坏的东西 | 红掉的守护 | 仍然绿（证明不是恒红夹具） |
| --- | --- | --- | --- |
| M1 | `Test-ScreenAwake` 收下预算但**不生效**（调用回到 `& adb … \| Out-String` 无界形状） | `S24` 红；`B10` 红（`B10_TIMEDOUT=False`）；`B11` 红（0 条 `AWAKE_PROBE`） | `B1–B9` 与 `S1–S23` 全绿；该套件墙钟 **149 s** —— 多出来的就是那次无界调用自己挂满 120 秒 |
| M2 | `device-capture` 截图调用回到无界 `cmd.exe /c` 直调 | `D12` 三条违规（未走执行器 / 回写 `cmd.exe /c` / 内层 `>`）；`BSD1–BSD5` 全红 | 其余 10 条契约用例绿 |
| M3 | `emulator-smoke` 同上 | `C9` 三条违规；ESD 系红（合计 6 红 / 5 绿） | 契约「自检：注入违规必须被检出」绿 |
| M4 | **拆掉 `device-capture` 的 `TimedOut` 逻辑闸门**（让「文件在不在」独判） | `D12`「超时判据缺失或顺序错」；`BSD1/BSD2/BSD4/BSD5` 红；`BSD5` 读数 **`STRAY_PNG=2`** —— 两枚半帧被 `Move-Item` 归位成 `*.png` 进了证据目录 | **`BSD3`（对照腿）仍绿** |
| M5 | 同 M4，落在 `emulator-smoke` | `C9`「超时判据缺失或顺序错」；`ESD1/ESD2/ESD4/ESD5` 红；`ESD5` 读数 **`STRAY_PNG=2`** | **`ESD3`（对照腿）仍绿**，契约其余用例绿（合计 5 红 / 6 绿） |

⇒ M4/M5 就是票面 AC3 要的「残帧规则有牙」：不删/不看 `TimedOut` 时**实测**留下非空残帧并被当成本次证据
（`STRAY_PNG=2`），删了/闸门在位时空转断言不成立。

## 文件清单

⚠️ **`.txt` 日志有两个 sha，表里都给**：根 `.gitattributes` 钉了 `*.ps1 / *.md / *.js` 等，**没有 `*.txt` 规则**，
而本机 `core.autocrlf=true` ⇒ git 在入库时把 CRLF 归一成 LF，**仓内 blob 与本机副本字节不同**（PNG 是二进制，两者一致）。
`副本 sha` = 从 Windows 工作树读到的；`blob sha` = `git cat-file blob HEAD:<路径> | sha256sum` 现测（Linux 检出与 CI 见到的那一面）。
对不上时先怀疑这一条，别怀疑日志被换过。

| 文件 | 字节 | 副本 sha256（前 20） | blob sha256（前 20） | 是什么 |
| --- | --- | --- | --- | --- |
| `1562-emu-ref.txt` | 3569 | `f36fd7415301ef76c027` | `ff96af14513e9d26baa5` | L1 冒烟对照腿完整输出 |
| `1562-emu-hang.txt` | 4561 | `6f9a6a5c79734a9e1e1f` | `00c06c8a3f7d04276bb8` | L2 冒烟强制腿完整输出（两条 `SHOT_CALL_TIMEOUT` 都在） |
| `1562-cap-ref.txt` | 2706 | `cfbf74892da03fcbc569` | `b6b73f0f274683be200c` | L3 ①a 对照腿完整输出 |
| `1562-cap-hang.txt` | 3112 | `2595e3d66170e4b9e72b` | `5a07777d79af7d4b287f` | L4 ①a 强制腿完整输出 |
| `1562-emu-ref2.txt` | 3787 | `fde2d5d97c7394e931bf` | `7a980eea0dc313729f67` | R1 重跑（留图那一轮；与 L1 参数相同，产物 sha 不同是因为两轮各截一次屏） |
| `1562-cap-ref2.txt` | 2706 | `f43ddc8634feefef8aa6` | `8ad79182648f1ff0f8b9` | R2 重跑（留图那一轮） |
| `emulator-pages-index-index.png` | 22064 | `1fd08757d3400292d873` | `1fd08757d3400292d873`（同） | 真链路出图（有界执行器产物，`magic=89504E47`，可解码 1080×2340） |
| `pages-login-login-current.png` | 588756 | `ec8bf3e15a7ba2336c3d` | `ec8bf3e15a7ba2336c3d`（同） | ①a 链真图（只读模式当前前台，同样可解码） |
| `real-chain-driver.ps1` | 4435 | `78ace4758c40cf12459e` | `78ace4758c40cf12459e`（同） | L1–L4 四腿取证驱动（绝对路径写死在本机 worktree，复现用） |
| `real-chain-ref-driver.ps1` | 3781 | `ad0180e83b6388edf506` | `ad0180e83b6388edf506`（同） | R1/R2 两腿驱动（跑在强制腿之后，把图留在盘上并当场验 magic/解码） |

两列怎么自己复现（本机现测命令，逐字）：

```powershell
# 副本
Get-FileHash -Algorithm SHA256 .\1562-emu-ref.txt | ForEach-Object { $_.Hash.ToLower() }
# 仓内 blob（LF 归一后的那一面；git show 走管道时不再经 autocrlf）
git cat-file blob HEAD:training-app/叉车维修培训学员端跨端应用/docs/verification/tooling/1562/1562-emu-ref.txt | sha256sum
```

`*.ps1` 两行两列相同不是巧合：`.gitattributes` 已把它们钉成 `text eol=lf`，写入时就是 LF ⇒ 无归一空间。

## 没做到的（写实）

- **①a 那一腿跑在仿真机上，不是维护者的真机**。本次真机不可达：`adb mdns services` 只出两条候选
  （`192.168.10.51:39527` / `:37611`），`scripts/wireless-debug.ps1` 逐个试探实测
  `WIRELESS_DEBUG action=ensure result=unconnected … extra=tried=2`（两条都回 10061）。
  手机侧重置之后没有任何 adb 通道可由脚本从零建立（见 `scripts/wireless-debug.ps1` 头部「防不住的部分」）。
  ⇒ 本产物证明的是**真实 adb 往返 + 真实 screencap 字节**，不声称替代 ① 真机门。
- 强制腿只用了**预算 0** 这一档：1 秒预算在本机不保证每次必挂（#1560 已把这条写进 ADR）。
- 未跑 ④（编译门）：改动集不含运行时面（`.ps1` / `.test.js` / `.md`），不参与应用编译产物。
