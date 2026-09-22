<#
.SYNOPSIS
    ④c（`scripts/kotlin-all-check.ps1`）publish 步与「导出新鲜度」的判据 —— 两个**纯函数**。

.DESCRIPTION
    为什么单独成库（#1272，2026-09-22）：这两条判据必须能被**真执行**地测到 ——
    `utils/kotlinAllStaleExportBehavior.test.js` dot-source 本文件后直接驱动它们
    （先例 `scripts/lib/capability-surface.ps1` + `utils/capabilitySurfaceBehavior.test.js`）。
    留在门脚本里就只能读源码文本断言，而接线守护**不构成 ③ 证据**（`docs/agents/guards.md`）。
    本文件零副作用（只有函数与常量定义），dot-source 它不会跑门。

    两条判据各自补的那条缝
      - `Get-PublishVerdict`：publish 步**是否成立**。旧判据只认 `与主程序的连接已中断|启动超时`
        两条文案，于是 `-1:cli:命令'publish app-android'不存在或缺少参数` 被放过 —— 而实测那串
        文案**不代表命令不存在**（`cli publish app-android --type appResource --project <p>` 在
        v5.24 存在且可用，同机成功导出 119 个 .kt；主程序忙 / 未就绪时 CLI 也回这句通用文案）。
        故判据改成「输出里有没有**正向成功标记**」，而不是猜文案。
      - `Test-AppResourceFreshness`：产物**是否覆盖当前树**。旧判据只看「有没有 .kt」⇒ 磁盘上留着
        一份旧导出就满足，于是 publish 失败也照常编译出 ✅（#1272 的假绿本体）。

    实测依据（2026-09-22，本机）：工作树**一字未改**时再跑一次 publish，导出目录里 **119/119** 个
    `.kt` 的 mtime 照样前进（16:55:13 → 16:57:37）⇒ 导出是**全量重写**，
    「没改动 ⇒ 不重写」这个反例在本工具链上不成立 ⇒ 「最新 .kt mtime 晚于 publish 基准」是有效判据。

    已知边界（写实，勿读成「已覆盖」）：本库只判**新鲜度**与**这一步是否成立**，不判「产物与源码内容
    是否逐字一致」。内容一致性由「publish 成功标记 + 新鲜度」联合兜住；两者任一不成立即判红。
#>

# publish「成立」的判据 = 输出里的**正向标记**（不是退出码：HBuilderX CLI 退出码恒为 0）。
# 文案随版本可能变 ⇒ 收敛成一处常量，改这里就够。
$PublishSuccessMarker = '导出 android 成功'
# publish「没成立」的判据：命中即判这一步没成立（**不**用来宣告「命令不存在」）。
$PublishFailurePattern = '与主程序的连接已中断|不存在或缺少参数|命令执行错误|启动超时'

<#
 .SYNOPSIS
     publish 步的输出 → 这一步是否成立。
 .OUTPUTS
     hashtable：`Ok`（bool）/ `Reason`（timeout | cli-ipc-blocked | cli-command-failed | no-success-marker | exported）
#>
function Get-PublishVerdict {
    param([string]$Output, [bool]$TimedOut)
    if ($TimedOut) { return @{ Ok = $false; Reason = 'timeout' } }
    if ($Output -match '与主程序的连接已中断') { return @{ Ok = $false; Reason = 'cli-ipc-blocked' } }
    if ($Output -match $PublishFailurePattern) { return @{ Ok = $false; Reason = 'cli-command-failed' } }
    if ($Output -match [regex]::Escape($PublishSuccessMarker)) { return @{ Ok = $true; Reason = 'exported' } }
    return @{ Ok = $false; Reason = 'no-success-marker' }
}

<#
 .SYNOPSIS
     导出目录是否**覆盖当前树**：.kt 里最新的 mtime 必须晚于基准时刻。
 .OUTPUTS
     hashtable：`Fresh`（bool）/ `KtCount`（int）/ `Newest`（datetime|null）/ `Reason`（fresh | stale | no-kt）
#>
function Test-AppResourceFreshness {
    param([string]$ExportDir, [datetime]$Since)
    $kt = @(Get-ChildItem -LiteralPath $ExportDir -Recurse -Filter *.kt -File -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -notmatch '\\www\\' })
    if ($kt.Count -eq 0) { return @{ Fresh = $false; KtCount = 0; Newest = $null; Reason = 'no-kt' } }
    $newest = ($kt | Sort-Object LastWriteTime -Descending | Select-Object -First 1).LastWriteTime
    if ($newest -gt $Since) { return @{ Fresh = $true; KtCount = $kt.Count; Newest = $newest; Reason = 'fresh' } }
    return @{ Fresh = $false; KtCount = $kt.Count; Newest = $newest; Reason = 'stale' }
}
