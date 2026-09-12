<#
    门脚本共享库（scripts/lib/）——**只放「零风险切片」**。

    为什么有准入门槛（2026-09-13 实测，勿放宽）：
      本仓的契约守护（utils/*Contract.test.js）断言的是门脚本的**源码字面量**，不是行为。例如
      mpWeixinGateContract 直接 pin 住 `Invoke-Process -FilePath $devTools -Arguments @('auto'`、
      `Invoke-Process -FilePath $devTools -Arguments @('open', '--project', $dist)`、
      `Write-Log "`n>>> devtools-open`、`$ProbeTimeoutMs = ` 等原文。
      把这类代码搬进共享库，守护会立刻失配，而唯一的「修法」是放宽守护 —— 那等于把防护拆掉。
      所以准入本文件的条件有两条，**缺一不可**：
        1) 各副本**逐字节相同**（不是「看起来一样」）；
        2) **没有被任何 utils/*Contract.test.js 作为字面量锚点 pin 住**。
      不满足就别搬：留在原处重复，比把守护一起搬走更便宜。

    已入库成员（连同它为什么合格）：
      · Get-HeadSha —— compile-check / kotlin-all-check / mp-weixin-check 三份**逐字节相同**
        （412 B / 14 行），且实测 `Get-HeadSha` 在 utils/*.test.js 中 **0 命中**（无守护引用）。
    明确**未**入库（不满足准入，勿擅自加）：
      · Test-PeHeader —— 4 份形态各异（305 / 433 / 438 / 504 B），得先定出哪一份是权威行为，
        属设计问题而非搬运问题；
      · Save-Jpeg / Get-WebpEncoder / Publish-GateComment —— 已被守护 pin 住（见上）。

    背景与实测数据见 PR #913；「守护从文本改行为」的收束计划另有设计票跟踪。
#>

function Get-HeadSha {
    param([string]$ProjectDir)
    $attempts = @()
    if ($ProjectDir) { $attempts += , @('-C', $ProjectDir, 'rev-parse', 'HEAD') }
    $attempts += , @('rev-parse', 'HEAD')
    foreach ($a in $attempts) {
        try {
            $out = & git @a 2>$null | Select-Object -First 1
            if ($out -and "$out".Trim()) { return "$out".Trim() }
        } catch { }
    }
    return ''
}
