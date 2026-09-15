# #1027 收尾真机确认：`dev:finish -Level standard` 步骤 1–9 全绿

> 归档时间：2026-09-15 ｜ 分支：`fix/1027-stability-tolerant` ｜ 关联 issue：[#1027](https://github.com/DriftingLi/FL/issues/1027)
> 前序：PR [#1029](https://github.com/DriftingLi/FL/pull/1029)（步骤 6 的 adb 解析单一真源）｜ ADR：`docs/adr/0008-移动端验收门与证据.md`
> 设备：`b32d8398`（1080×2400）｜ 工具链：HBuilderX 5.23.2026060626 + adb 1.0.41｜ 命令：`npm run dev:finish -- -Level standard -Device b32d8398`

## 一、结论：九步跑完、exit 0（总耗时 11 分 33 秒）

| 步骤 | 结果 |
| --- | --- |
| 1 级别判定 | ✅ `🟡 standard`（强制指定） |
| 2 环境检测 | ✅ 设备在线、HBuilderX cli 可用（`HX_BUSY wait=0 result=free`） |
| 3 单元测试 | ✅ Q-A 契约测试通过 |
| 4 编译（④c 整模块） | ✅ `COMPILE_RESULT errors=0` |
| 5 部署（真运行到设备） | ✅ 已到设备（160s） |
| 6 自动截图 | ✅ `导航已落定（168s，已进入目标页且画面稳定（相邻两次采样差异 0% ≤ 0.5%））` → `dashboard.png` |
| 7 截图对比 | ✅ 走的是**首次运行**路径：基线目录为空，据实报告「未建立基线」 |
| 8 生成证据 | ✅ `.ci-verify/evidence.md` |
| 9 还原配置 | ✅ 配置文件未被改脏 |

产物：`01-dashboard-after.png`（743 KiB，`sha256=0C7ECD65B680222F8545D366F828AA1C7E18E2F1A8E8E82F7962B9CFBB265D9E`）＋ 四次跑的完整日志。

> 归档的日志用 `.txt` 扩展名：本仓 `.gitignore:62` 有 `*.log`（仓库里**没有任何**被跟踪的 `.log`），
> 这里**不**为了归档去改 ignore 规则，也不 `add -f` —— 换扩展名归档，理由写在这里，不做静默例外。

## 二、为什么跑了四次：每次失败点都不同（逐次钉死一个缺陷）

真机复跑跑一次照出一个 —— 三个都在步骤 6–7 这条路径上，**不是同一处**：

| 轮次 | 结果 | 失败点（原文） | 性质 |
| --- | --- | --- | --- |
| 1 | exit 1（步骤 6） | `请求页 pages/index/index，日志里最后进入的是 pages/dashboard/dashboard（页身份不符）` | **不是缺陷**：页身份判据（S17）的设计行为 —— `pages/index/index` 启动即自跳转 |
| 2 | exit 1（步骤 6） | `到上限 420 秒未同时满足「进入目标页 + 非全黑 + 画面稳定」` | **缺陷二**：全帧 hash 稳定性判据被系统状态栏实时读数钉死 |
| 3 | exit 1（步骤 7） | `在此对象上找不到属性"Count"` | **缺陷三**：`Get-ChildItem` 单文件退化成标量 × `Set-StrictMode -Version Latest` |
| 4 | **exit 0** | — | 修完二、三后九步全绿 |

## 三、缺陷二：不是「画面不稳」，是**只有状态栏在变**

原判据是**全帧 SHA256 相等**。系统状态栏里有**应用控制不了**的实时读数（MIUI「显示实时网速」的 KB/s）。
间隔 5 秒连拍三帧（复刻 `-PollSeconds` 默认值），实测：

| 帧对 | 整帧 sha256 | 逐带（每 100px）比较 |
| --- | --- | --- |
| frame-1 vs -2 | 不同 | **只有 0–99px** 有 >8 的像素差（MaxDiff 188），其余整幅逐字节相同 |
| frame-2 vs -3 | 不同 | 同上 |
| 同帧自比（对照） | 相同 | 全幅 0 |

⇒ 全帧 hash 在这台设备上**永远不可能相等** ⇒ 判据永远不满足 ⇒ 步骤 6 必然 420s 超时、**步骤 7–9 永远跑不到**，而 app 画面其实早就落定了。

**修法与阈值标定（实测，不是拍的）**：改「逐字节相同」为「采样网格上的差异像素占比 ≤ 阈值」（`Compare-ScreenFrames`，与 `Test-ScreenBlank` 同一套 24 步网格；参数 `-StableMaxDiffPercent`，默认 0.5%）：

| 场景 | 24 网格 | 48 网格 | 96 网格 | 192 网格 |
| --- | --- | --- | --- | --- |
| 只有状态栏在变（真机两帧） | 0% | 0% | 0% | **0.011%**（4/36000，全在顶带） |
| 真切页（对照：真帧 vs 全黑合成帧） | 100% | 100% | 99.98% | 99.98% |

⇒ 0.5% 卡在噪声上方 ~45×、真变化下方 ~200×。**写实边界**：面积小于阈值的动态元素（如某个 100×100 的动画角标）会被判「稳定」——有意取舍（判据目标是别把切页过渡帧/白屏当证据，不是抓微动画）。

## 四、缺陷三：`Get-ChildItem` 单文件 × `Set-StrictMode`

`screenshot-diff.ps1` 的 `$currentFiles = Get-ChildItem …` 只命中**一个** png 时退化成**标量**，而 `dev-finish.ps1` 以 `Set-StrictMode -Version Latest` 跑 ⇒ `$currentFiles.Count` 抛「在此对象上找不到属性 Count」⇒ **步骤 7 直接崩、步骤 8–9 跑不到**。
触发面正是最常见的形态：**基线目录不存在（首次运行）+ 恰好一页改动**（本轮就是 1 页）。0 个与 ≥2 个文件都**不会**触发 —— 所以文本/计数类守护天然漏掉它，必须用**运行期**用例（在 StrictMode 下真跑单文件场景）才抓得住。

处置：两处 `Get-ChildItem` 都用 `@(...)` 包住；并订正步骤 7 的文案（原写「首次运行，已建立基线」，但该函数只在 `-UpdateBaseline` 时才把截图拷进基线目录，否则只建了个空目录 ⇒ 现在据实分流）。

## 五、页身份约束（不是缺陷，但跑步骤 6 必须知道）

改动的页面若**自跳转**，`Wait-NavSettled` 的页身份判据会如实点名「请求页 X，实际进入 Y」并 fail-closed 超时（轮次 1 就是这样）。**要跑通步骤 6，改动页要选一个不自跳转的页**（本例改用 `pages/dashboard/dashboard` 后通过）。

## 六、复现方式（可重跑）

```powershell
# 1) 造一个「改动页」：给目标页的 <script> 块加一行注释（零语义风险），确认推导恰好一页
#    （不自跳转的页：pages/dashboard/dashboard；pages/index/index 会自跳到 dashboard）
# 2) 跑
cd <项目根>; npm run dev:finish -- -Level standard -Device b32d8398
# 3) 看 .ci-verify/screenshots/*.png（时间戳必须晚于本次运行起点）、evidence.md、本目录 run-4 日志
```

## 七、落锁与平台边界

- `autoScreenshotContract` **S22** + `autoScreenshotStabilityBehavior` **B1–B5**：宽容比较语义（相同 ⇒ 一致；命中 1/600 采样点的微变 ⇒ 仍一致且占比 >0；半幅变化/缺上一帧 ⇒ 不一致）。
- `screenshotDiffContract` **D7** + `screenshotDiffSingleFileBehavior` **P0/C1–C4**：StrictMode 下的单文件/双文件/零文件/同内容基线四场景。
- **判别力实测**：旧写法塞回去 ⇒ S22+B1–B5（6 条）、D7+P0+C1（连带 C2–C4）分别同时红。
- **平台边界写实**：`System.Drawing` 是 Windows-only ⇒ Linux CI 上 B 套件验的是「取色不可用 ⇒ 不判稳定」的 fail-closed 语义；D 套件（只算 MD5 与文件枚举）在两端都真跑。
