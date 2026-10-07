# adb 单次调用有界化 —— 真链路成对取证（票 #1560）

被测对象：`scripts/lib/auto-screenshot.ps1` 新增的有界执行器 `Invoke-BoundedAdbShot`，以及它的两个调用点
（`Wait-NavSettled` 的导航采样、`Invoke-AutoScreenshot` 的页面截图）。
判据面（`-TimeoutSeconds` / `-NavigateMinSeconds` / `-StableMaxDiffPercent`）本票一字未动。

设备 `b32d8398`（Xiaomi 23049RAD8C，1080×2400，Android 15）· 取证时 `HEAD=a455cc98` ·
日期 2026-10-07 · 载体 `driver-ref-leg.ps1` / `driver-hang-leg.ps1`（本目录，两条腿的真实驱动，参数写在文件头注释里）。

**代码字节绑定**（取证绑的是这三枚 blob，不是 commit 号 —— `git rev-parse a455cc98:./<路径>` 现取）：

| 文件 | blob sha |
| --- | --- |
| `scripts/lib/auto-screenshot.ps1` | `293491026c24b6a1b50a0b36b4e594c754d8cb12` |
| `utils/autoScreenshotContract.test.js` | `e56991c5ba8930ea22057da049f442aa35684892` |
| `utils/autoScreenshotStabilityBehavior.test.js` | `6aefe5e45eedfabf3358e6c711952c3c672a3941` |

> 本目录按**票号** 1560 归档（先例：`docs/verification/device/1027`，1027 现测是 issue 而非 PR）。
> 取证在开 PR 之前已完成，取票号可免「目录名 ↔ PR 号」互相依赖；PR 号见本票与 PR 正文。

## 为什么必须成对

只跑「通过」的那一次证明不了判据有牙 —— 一个恒绿的执行器同样会「通过」。所以同一条链跑**两腿**：
一腿正常预算（必须落定并出图），一腿把单次预算压到 **0** 使每轮必挂（必须到点 `Skipped`、循环继续下一页、整条链自然收口）。
⚠️ **强制腿只能用预算 0**：热通道实测健康返回 646 ms / 949 ms / 1217 ms 都出现过 ⇒ 压到 1 秒**不保证**每轮必挂，
拿它当判据会造出一条时有时无的假腿。

## 两腿读数

### 对照腿（`driver-ref-leg.ps1`，日志 `real-chain-ref-leg.txt`）

| 段 | 参数 | 读数 |
| --- | --- | --- |
| LEG_A | 默认预算（`-AdbCallTimeoutSeconds` 不传 ⇒ 15）、`-NavigateTimeoutSeconds 900` | `NAV_SAMPLE settled=True seconds=87 samples=15 callTimeouts=0 callBudgetSeconds=15 budgetSeconds=900` ⇒ `LEG_A ok=True shots=dashboard skipped= wall_seconds=92` |
| LEG_B | `-AdbCallTimeoutSeconds 1 -NavigateMinSeconds 60 -NavigateTimeoutSeconds 8`（`Min > Timeout` ⇒ **Skipped 与运气无关，结构必达**） | `NAV_SAMPLE settled=False seconds=11 samples=2 callTimeouts=0 callBudgetSeconds=1 budgetSeconds=8` ⇒ `LEG_B ok=False shots= skipped=dashboard wall_seconds=12` |
| LEG_C | `-Pages dashboard,index -AdbCallTimeoutSeconds 1 -MaxPages 2` | 第一页出图并**经 `.part` 归位**：`✅ dashboard.png (sha256=B9132CFC09C2A980…)`；第二页 `pages/index/index` 是 `ADR-0008:370` 记的**自跳转页**（S17 页身份判据按设计会 fail-closed 等满 600 秒），取证价值在归位那一半 ⇒ **人工叫停，本腿没有打出收尾行** |

⇒ 在仓那枚 `dashboard.png` 就是 LEG_C 第一页的归位产物（731,517 字节，完整 sha 见下方清单）。
⚠️ LEG_A 的截图字节是另一枚（日志里 `bytes=731099 sha256=838BD7FEED9656CF…`）：三条腿写进**同一个输出目录**，
LEG_C 把 LEG_A 那枚**覆盖**了 ⇒ 838BD7FE 只在日志里可核，本目录的 png 只对 B9132CFC 负责。别把两者当同一枚。

### 挂死腿（`driver-hang-leg.ps1`，单次预算 0）

修复**后**（日志 `real-chain-hang-leg-postfix.txt`，即本 PR 发货的字节）：

```
RC2 | D1_budget0 wall_ms=181 TimedOut=True Exit=-1 Seconds=0 file_exists_after=False err=单次 adb 调用未在 0 秒内返回（已终止整棵进程树；残帧已作废）
RC2 | D2_budget1 wall_ms=646 TimedOut=False Exit=0 Seconds=0.6 bytes=729987
RC2 | D3_budget15 wall_ms=719 TimedOut=False Exit=0 bytes=729987 magic=89504E47 blank=False sha256=0E8B749F3999FDEF7B860220A915C6F87578C39D23BFE340B1DD5226521C9366
NAV_SAMPLE settled=False seconds=31 samples=0 callTimeouts=6 callBudgetSeconds=0 budgetSeconds=30        ← 第 1 页 dashboard
NAV_SAMPLE settled=False seconds=31 samples=0 callTimeouts=6 callBudgetSeconds=0 budgetSeconds=30        ← 第 2 页 index（循环照常继续）
RC2 | B2 ok=False shots= skipped=dashboard|index wall_seconds=62 nav_probe_left=False
```

⇒ 票面 AC1/AC2/AC4 的靶心：**上限真的可达** —— 6 次调用全部被终止，`samples=0`（被终止的不算采样），
到 30 秒 fail-closed 记 `Skipped`，然后**继续第二页**，整条链 62 秒自然收口，而不是原来那种「32 分钟零写入」。

修复**前**（同一支驱动、同一台设备，日志 `real-chain-hang-leg-prefix.txt`）—— 这一腿照出了第二道防线：

```
RC2 | D1_budget0 wall_ms=170 TimedOut=True Exit=-1 Seconds=0 file_exists_after=True err=…残帧未删净
RC2 | B2 … nav_probe_left=True
```

⇒ `Kill` 只是**请求**终止，子进程握着的重定向句柄未必已放，**删除与终止有竞争、会当场失败**。
第一版把防线做成「残帧当场删」，到这里就漏了。⚠️ **说清各半边是谁照的**：真机这次留下的残帧是
**0 字节**（在仓 `residual-after-kill-prefix.png`，sha `e3b0c442…` = 空文件的 sha）⇒ 它被 `Length -gt 0` 挡住、
**没有**被数成采样；「被数成一次成功采样」那一半是**夹具照的** —— 见 `utils/autoScreenshotStabilityBehavior.test.js`
的 B6/B7W 与变异 M4 读数 `B7W_SAMPLES=6`（残帧非空 + 拆掉逻辑闸门 ⇒ 逐字被数成 6 次）。
所以修法不依赖删除成功率：采样条件加**逻辑闸门** `-not $shot.TimedOut`，页面截图先写 `<name>.png.part`、
只在「调用返回 + 帧非空」时 `Move-Item` 归位（步骤 7 按 `-Filter '*.png'` 扫目录 ⇒「不产截图」靠命名兑现）。

### 第二调用点（页面截图）的射程怎么拼齐

它的**超时支**在真链路上取不到：两个调用点共用同一个预算参数，单参数下「采样能落定 + 截图必挂」不可能同时成立
（预算压到 0 ⇒ 采样先挂，走不到截图）。⇒ 按三段拼装，缺一格就别宣称整链覆盖：
一是 `.part`/`Move-Item` 归位在真链路上**走通**（LEG_C 第一页，产物即在仓 `dashboard.png`）；
二是该调用点在 `S23` 的结构判据里（调用点计数 `>= 3`、`SHOT_CALL_TIMEOUT` 出口在位）；
三是它调的就是同一个 `Invoke-BoundedAdbShot`，该函数由 `B6` 用真 cmd 启动器 + 真挂死桩直接驱动。

## ③ 门全量（本机那一次）

`npm run test:unit:gate`（= `pwsh scripts/dev-finish.ps1 -UnitGate`）在 `HEAD=a455cc98`、`dirty=clean` 上跑，
完整原文在本目录 `unit-gate-full.txt`，尾部机检行：

```
TEST_UNIT_RESULT exit=0 suites=159 suitesFailed=0 tests=3044 testsFailed=0 time=290.597s wall=294s log=.ci-verify/test-unit.log summary=none head=a455cc9827e7c84a09ff47056389be6346e0c7f0 dirty=clean
```

本目录（docs 产物）之后进来的提交**没有**重跑本机这一道门，判据是 `docs/agents/dev-loop.md`
「全量 ③ 门该不该跑」的 **C2b**：改动集与 `Get-ContractTestPattern` 的射程（`--testPathPattern` 只匹配测试文件路径）无交集。
最终 head 的全量 ③ 由 CI 的 `mobile-test` 在那个 head 上跑（本机不跑不改变合入判据）。

## 文件清单

| 文件 | 字节 | sha256 | 是什么 |
| --- | --- | --- | --- |
| `driver-ref-leg.ps1` | 8093 | `0d24c25482229a7ba706dbd7be6eb34233c24d0ee8a90b9c61ed2598015b2153` | 对照腿驱动（三条腿 A/B/C 的参数与取数） |
| `driver-hang-leg.ps1` | 5023 | `67db1544d1f2548e6d1692864fbdcc9d6798e1f292e4a4847757e5dafee3fd4d` | 挂死腿驱动（D1/D2/D3 直调 + B2 整链，预算 0） |
| `real-chain-ref-leg.txt` | 3564 | `f517977efca9e6babc6cb6d4a3aae58178828ab1f96529a68b06566bf8f66cd4` | 对照腿 transcript 原文（`.log` 改名 `.txt`：根 `.gitignore` 的 `*.log` 会静默挡在仓外） |
| `real-chain-hang-leg-postfix.txt` | 3171 | `72bf2858e265150a06da982a61825e4714ed3b8a7d561d36a570594fb6ac8639` | 挂死腿 transcript（修复后 = 本 PR 发货字节） |
| `real-chain-hang-leg-prefix.txt` | 3164 | `0f209b743ec5a6e294c857e1a74370a55c1f4ef979ecb323db2ecebb7da6c51f` | 挂死腿 transcript（修复前，照出「删除不能当判据」那一腿） |
| `dashboard.png` | 731517 | `b9132cfc09c2a980c6ce13ed76329152d0883c192f401a6a9c49f05d9b5a8ee7` | LEG_C 第一页经 `.part` 归位的真截图 |
| `screencap-direct-budget15.png` | 729987 | `0e8b749f3999fdef7b860220a915c6f87578c39d23bfe340b1dd5226521c9366` | D3（预算 15 秒直调 `Invoke-BoundedAdbShot`）产物，`magic=89504E47 blank=False` |
| `residual-after-kill-prefix.png` | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | 修复前 D1 被 `Kill` 后**没删掉**的残帧（0 字节 ⇒ 正是「存在」≠「可用」） |
| `unit-gate-full.txt` | 47794 | `89aa0fd7b6d5e5eda5ef86d41acd3ea58041fd8037ee7376e63d85118080a84c` | 全量 ③ 门原文（含尾部机检行） |
