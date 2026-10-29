<#
.SYNOPSIS
    验收证据生成模块：按 A1 形态产出 PR 正文 `## 验收证据` 段（2026-10-29，#2100）。

.DESCRIPTION
    **A1 形态**：每道触发的门一行，行内写 `跑于 <sha7>` + 门自带的机读读数行；不再有
    `执行人 / 日期 / 复测对象 / 结论` 四字段，也不再产 ① 行（①a 已降为发版前一次、①b 已退役）
    与 ③ 行（两腿在 ci-summary 里）。

    行的形状（列表项前缀、门号、`跑于` 锚点）来自**唯一真源**
    `.github/scripts/lib/gate-validator.mjs` 里 `GATE_EVIDENCE_DESCRIPTOR BEGIN/END` 圈出的严格 JSON
    （`scripts/lib/gate-evidence.ps1` 抽同一段字节 `ConvertFrom-Json`）。本模块只负责填值 ——
    形状改了它跟着变，不需要两处同时改。

    为什么保留这台机器：判据只认「读数行原样 + sha7」。手抄读数行最容易抄错一字（`errorsTotal=0`
    写成 `errors=0` 就判红），而这正是「门绿了但正文写错 → 假红」的老坑（#1403）。

    分级（沿用 dev:finish 的语义）：
      - quick     → 不生成文件，返回提示（由 dev:finish 打印）
      - standard  → ④ 行（④c 整模块）+ 有读数时才写 ② 行
      - full      → 同上；② / ④ 的读数各自独立判断

    产物何时可得：读数是**门跑完**才有的（`MP_WEIXIN_RESULT` / `KOTLIN_ALL_RESULT` / `COMPILE_RESULT`）。
    拿不到就写「待补：…」——它不在校验器的占位词表里（判据只看读数行 token 与 sha7），所以会
    **如实判红**直到贴上真读数，符合「证据未到位即红」的门设计。本模块的职责是**给对形状**。

.EXAMPLE
    . scripts/lib/evidence-gen.ps1
    $r = New-Evidence -Level 'standard' -CompileResult $kotlinLine -MpResult $mpLine -ScreenshotUrl $url
#>

function New-Evidence {
    [CmdletBinding()]
    param(
        [ValidateSet('quick', 'standard', 'full')]
        [string]$Level = 'quick',
        # ④ 门的机读读数行（`KOTLIN_ALL_RESULT ...` 或 `COMPILE_RESULT ...`），**原样**传，不要改写
        [string]$CompileResult = '',
        # ② 门的机读读数行（`MP_WEIXIN_RESULT ...`）；不传就不写 ② 行（未命中 MP-WEIXIN 面时本就该免）
        [string]$MpResult = '',
        # ② 行尾的截图（GitHub 附件直链）。A1：图是**附证**不是判据（#2108），可空
        [string]$ScreenshotUrl = '',
        # `跑于` 要写的 sha7；默认取当前 HEAD。门若是在更早的提交上跑的，传那个提交的前 7 位。
        [string]$Sha7 = '',
        [string]$OutputPath = '',
        [string]$ProjectDir,
        # —— 以下参数 A1 起**不再使用**，保留仅为调用方（dev:finish）签名兼容 ——
        # ① 行已整行删除（ADR-0043 / #2103）：ScreenshotDiff / ScreenshotChangedCount / Executor 无去处
        [array]$ScreenshotDiff = @(),
        [int]$ScreenshotChangedCount = 0,
        [string]$Executor = 'agent 执行',
        # ③ 行已整行删除（#2099 判定纯税）：CI run 链接不再写进正文
        [string]$CiRunUrl = ''
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputPath) {
        $OutputPath = Join-Path $ProjectDir '.ci-verify\evidence.md'
    }
    $ProjectDir = (Resolve-Path -LiteralPath $ProjectDir).Path

    . (Join-Path $PSScriptRoot 'gate-evidence.ps1')
    $descriptor = Get-GateEvidenceDescriptor

    # 🟢 快速模式：不生成文件
    if ($Level -eq 'quick') {
        return [pscustomobject]@{
            Generated = $false
            Path      = ''
            Content   = ''
            Message   = 'A1：命中门就往 PR 正文写一行「- ②/<门号> 跑于 `<sha7>` · <门读数行>」；未命中运行时面写「免（未命中运行时面）」。'
        }
    }

    if (-not $Sha7) {
        try {
            $Sha7 = (& git -C $ProjectDir rev-parse --short=7 HEAD 2>$null | Out-String).Trim()
        }
        catch {
            $Sha7 = ''
        }
    }
    if (-not $Sha7) {
        # 「跑于」是硬判据：没有 sha7 就生成不出可过门的行。这里不编一个假 sha（那正是假绿）。
        throw 'New-Evidence：取不到 HEAD 的短 sha，请用 -Sha7 显式传入（门实际跑的那个提交的前 7 位）'
    }

    $lines = @([string]$descriptor.heading, '')
    $gate2 = Get-GateEvidenceGate -Key '②' -Descriptor $descriptor
    $gate4 = Get-GateEvidenceGate -Key '④' -Descriptor $descriptor

    if ($MpResult) {
        $lines += Format-GateEvidenceLine -Descriptor $descriptor -Gate $gate2 -Sha7 $Sha7 -Readings $MpResult -Artifact $ScreenshotUrl
    }
    if ($CompileResult) {
        $lines += Format-GateEvidenceLine -Descriptor $descriptor -Gate $gate4 -Sha7 $Sha7 -Readings $CompileResult
    }
    if (-not $MpResult -and -not $CompileResult) {
        # 两门读数都没有：仍然给出形状（行首 + 跑于），把读数位置留成显式缺口，
        # 让「缺读数」在预检时以「该行没有读数」报出来，而不是让整个证据段消失。
        # ⚠️ 占位句里**不许出现读数行的名字**（`KOTLIN_ALL_RESULT` 之类）：判据只看 token 在不在这行，
        #    写了它就会被当成「读数已贴」⇒ 结构假绿（本文件老版本踩过同族的坑：占位句里写 `actions/runs/<id>`
        #    会满足 ③ 的产物判据）。
        $lines += Format-GateEvidenceLine -Descriptor $descriptor -Gate $gate4 -Sha7 $Sha7 -Readings '待补：④ 门读数尚未产出（跑一次编译门后把它的机读行原样贴进来）'
    }

    $contentText = ($lines -join "`n")

    $outputDir = Split-Path -Parent $OutputPath
    if ($outputDir -and -not (Test-Path -LiteralPath $outputDir)) {
        New-Item -ItemType Directory -Force -Path $outputDir | Out-Null
    }
    Set-Content -LiteralPath $OutputPath -Value $contentText -Encoding utf8

    return [pscustomobject]@{
        Generated = $true
        Path      = $OutputPath
        Content   = $contentText
        Message   = "验收证据已生成: $OutputPath"
    }
}
