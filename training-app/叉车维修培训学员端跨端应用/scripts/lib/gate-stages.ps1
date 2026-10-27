#Requires -Version 5.1
# 门脚本阶段时间戳录制器（#2001）—— 零副作用纯追加日志行，只增不改。
#
# 供 ④a compile-check.ps1 / ④c kotlin-all-check.ps1 / ② mp-weixin-check.ps1
# 在阶段边界录制 "[STAGE] <name> start|end" 行，Stop 时收尾一行 STAGE_SUMMARY：
#   STAGE_SUMMARY stages=N total=<s> wall=<s> delta=<pct>% check=pass|fail (threshold 5%)
# 验收判据（票面）：一次真跑后日志尾能读出 4–5 段起止与秒数、段和与门墙钟差 ≤5%。
# 早退路径（脚本内部 exit）不补 end 行：STAGE_SUMMARY 停在最后完整闭段 = 断在哪段由日志揭示。
# 不改门判据、退出码、既有字段语义（只增）。

# 记一行到目标日志（外部函数注入的 $script:GateStageLogWriter 优先，测试可重定向；否则 Add-Content）。
function Write-GateStageLine {
    param([string]$Text)
    $writerVar = Get-Variable -Name GateStageLogWriter -Scope Script -ErrorAction SilentlyContinue
    if ($null -ne $writerVar -and $null -ne $writerVar.Value) {
        & $writerVar.Value $Text
        return
    }
    if (-not $script:GateStageLogPath) {
        throw "gate-stages: 未初始化（Initialize-GateStageRecorder 未调用或缺 -LogPath）"
    }
    Add-Content -LiteralPath $script:GateStageLogPath -Value $Text -Encoding utf8
}

function Initialize-GateStageRecorder {
    param(
        [Parameter(Mandatory = $true)][string]$LogPath,
        # 测试注入用：把墙钟起点拨早，验证 delta 超阈分支（B7）
        [datetime]$WallStart = $(Get-Date)
    )
    $script:GateStageLogPath = $LogPath
    $script:GateStageWallStart = $WallStart
    $script:GateStages = @()
    $script:GateStageOpen = $null
    Write-GateStageLine ("STAGE_RECORDER log=" + $LogPath)
}

# 开始一段；若已有未闭段，先自动补其 end+elapsed 行（防漏段）。
function Start-GateStage {
    param([Parameter(Mandatory = $true)][string]$Name)
    if ($null -ne $script:GateStageOpen) {
        Invoke-GateStageClose
    }
    $script:GateStageOpen = @{ Name = $Name; Start = Get-Date }
    Write-GateStageLine ("[STAGE] " + $Name + " start " + $script:GateStageOpen.Start.ToString("yyyy-MM-dd HH:mm:ss"))
}

# 闭当前段并写 STAGE_SUMMARY；无开段 fail-closed 抛错。
function Stop-GateStage {
    if ($null -eq $script:GateStageOpen) {
        throw "gate-stages: Stop-GateStage 无未闭段（早退揭示：断在已 Start 未 Stop 的段，或从未 Start）"
    }
    Invoke-GateStageClose
    $total = 0
    foreach ($seg in $script:GateStages) { $total += $seg.ElapsedSeconds }
    $wall = [int]((Get-Date) - $script:GateStageWallStart).TotalSeconds
    if ($wall -lt 0) { $wall = 0 }
    $delta = if ($wall -gt 0) { [math]::Round((($wall - $total) / $wall) * 100, 1) } else { 0 }
    if ($delta -lt 0) { $delta = 0 }
    $check = if ($delta -le 5.0) { "pass" } else { "fail" }
    Write-GateStageLine ("STAGE_SUMMARY stages=" + $script:GateStages.Count +
        " total=" + $total + "s wall=" + $wall + "s delta=" + $delta + "% check=" + $check +
        " (threshold 5%)")
}

# 内部：闭当前段，写 end 行并累计。
function Invoke-GateStageClose {
    $now = Get-Date
    $elapsed = [int]($now - $script:GateStageOpen.Start).TotalSeconds
    if ($elapsed -lt 0) { $elapsed = 0 }
    $script:GateStages += ,@{ Name = $script:GateStageOpen.Name; ElapsedSeconds = $elapsed }
    Write-GateStageLine ("[STAGE] " + $script:GateStageOpen.Name + " end " + $now.ToString("yyyy-MM-dd HH:mm:ss") +
        " elapsed=" + $elapsed + "s")
    $script:GateStageOpen = $null
}
