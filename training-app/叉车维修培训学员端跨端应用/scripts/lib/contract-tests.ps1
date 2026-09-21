<#
.SYNOPSIS
    Q-A 契约测试的**测试集范围**唯一真源（`--testPathPattern`）。供 dev-finish.ps1 与 lib/test-compile.ps1 共用。

.DESCRIPTION
    为什么必须只有一份（2026-09-14 实测）：
      `dev-finish.ps1` 步骤 3 与 `lib/test-compile.ps1` 各自抄了一份 `$testPattern`，**两份漂移了** ——
      步骤 3 那份少了 `hxRun` / `hxTimingBehavior` / `hxError` 三个 token。
      后果不是「少跑几个测试」：步骤 3 **先跑且失败即 `exit 1`**，所以**真正起门禁作用的是范围更窄的那份**，
      而步骤 4 用的是更宽的 —— 同一套 Q-A 出现两个范围，新增的运行期守护可能只在其中一条路径上生效。

    怎么防再次漂移：
      ① 本文件是唯一的字面量所在地（调用方只 dot-source 并调用函数）；
      ② 守护 `utils/contractTestPatternBehavior.test.js` 做**运行期**断言（dot-source 本文件、调函数、
         断言返回的 pattern 确实覆盖各组关键守护）—— 比 pin 源码文本强，且与 ADR-0008「守护从断言文本
         改为断言行为」的方向一致；
      ③ 守护同时断言两个调用方都**没有**再抄一份自己的 pattern 字面量。

    ⚠️ `--testPathPattern` 是**子串**匹配（已用 `jest --listTests` 实证）：
      `hxRun` **匹配不到** `hxErrorLinesBehavior`、`hxTimingBehavior` 也匹配不到它
      ⇒ 每个套件都要能找到它的 token，不能靠另一个 token 顺带命中。
      新增运行期守护时，**必须**把它的 token 加进本函数，否则该守护**永不执行**（本仓反复踩过的假绿形态）。

.NOTES
    纯函数：不碰设备、不碰 HBuilderX、不读文件 ⇒ 可在任何机器上断言，也不需要 adb。
#>

function Get-ContractTestPattern {
    <#
      Q-A 契约测试的 `--testPathPattern`。返回**单个字符串**（jest 参数），不作为数组。
      token 与它覆盖的守护套件一一对应（见下方注释），新增守护须同步加 token。
    #>
    # levelDetect      → levelDetectContract / levelDetectBehavior（含 §8 空 diff 崩溃 G1–G3）
    # envCheck         → envCheckContract
    # testCompile      → testCompileContract
    # buildDeploy      → buildDeployContract
    # autoScreenshot   → autoScreenshotContract
    # screenshotDiff   → screenshotDiffContract
    # evidenceGen      → evidenceGenContract
    # devFinish        → devFinishContract
    # hxBusyGate       → hxBusyGateContract（锁：H1–H12）
    # hxRun            → hxRunContract（C1–C14，文本层）
    # hxTimingBehavior → hxTimingBehavior 的运行期守护（#974）
    # hxError          → hxErrorLinesBehavior 的运行期守护（ADR-0012）
    # hxLaunchDetach   → hxLaunchDetachBehavior 的运行期守护（2026-09-15：派发形态不得泄漏句柄）
    # capabilitySurface → 能力面行为守护（ADR-0016 ①b：白名单判定 + 调用点扫描 + 与 PR 模板对齐）
    # concurrent401Refresh → concurrent401RefreshBehavior / concurrent401RefreshContract
    #                        （#1124：并发 401 只发一次刷新 + 空 storage 不弹回首页；行为与接线两层）
    # contractTestPattern → 本真源自身的运行期守护（**pattern 必须自指**，否则该守护永不执行）
    # contractReaderEol → contractReaderEolContract（ADR-0019 读取层归一的共享读者守护）
    # resumeAttachment → resumeAttachmentContract（#1198：附件删除端点两端同源；错路径不得回归）
    # jobApplyState    → jobApplyStateContract（#1199：投递态回流；本地布尔量不得复活按钮）
    # recruiterResume → recruiterResumeContract / recruiterResumeBehavior（#1196：脱敏字段清单与后端
    #                 resume_projection.go 逐项对账、未授权态不渲染敏感面、打码 PDF 出口、8 维筛选）
    return 'levelDetect|envCheck|testCompile|buildDeploy|autoScreenshot|screenshotDiff|evidenceGen|devFinish|hxBusyGate|hxRun|hxTimingBehavior|hxError|hxLaunchDetach|capabilitySurface|concurrent401Refresh|contractTestPattern|contractReaderEol|resumeAttachment|jobApplyState|recruiterResume'
}
