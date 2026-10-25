<#
.SYNOPSIS
    验收证据生成模块：根据级别和结果生成 PR 验收证据文本。

.DESCRIPTION
    输出**逐门单行**、字段以 ` · ` 分隔、结论行内带产物 —— 严格对齐
    校验器（`validatePrEvidence`，单一来源 `.github/scripts/lib/gate-validator.mjs`；
    `pr-evidence.yml` 用 `repos.getContent` 取它执行，不再内联）的解析语法：
    它按「以门号开头的那一行」取门（`matchesGate`），再在**同一行**里正则抠
    `执行人 / 日期 / 复测对象 / 结论`。多行块（门号单独一行、字段拆成缩进 bullet）会被判
    「缺该行」—— 2026-09-29 实测（issue #1403 取证）旧版正是这个形状，故 PR 一开即红、
    逼人手工把结构重排一遍才转绿。守护见 `utils/evidenceGenContract.test.js`（真跑生成器
    喂进校验器，成对断言「补真产物⇒绿 / 退回多行⇒红」）。

    分级：
      - quick → 不生成文件，返回提示（由 dev:finish 打印 Get-QuickEvidenceHint）
      - standard → ① + ③ + ④（④ 为默认载体 ④c）
      - full → ① + ③ + ④（同门集；② 由 sha 绑定评论承载、④b 按需追加，本模块不产）

    ⚠️ 产物何时可得（#1770 起口径变了，别再照旧假设「开 PR 之前拿不到」）：
    证据在 **push 后、开 PR 前**就该齐 —— ① 的截图目录键是**票号**（不等 PR 号，push 后即可取证），
    ③ 的 `ci-summary` run 在 push 后即产出（`submit-pr.ps1` 会从 check-runs 自动发现链接并补进 ③ 行）。
    但 dev:finish 仍可能**赶在 push 之前**跑，那时这两项确实还不存在 ⇒ 结论行按「待补：<占位说明>」
    输出；它**不在**校验器的占位词表里（`待人工/待补/…` 精确匹配才算占位），会**如实判红**直到人贴上
    真产物 —— 符合「证据未到位即红」的门设计。本模块的职责是**给对形状**，让收口的人只填值、不重排。
    若产物已知（standalone 调用、push 后重跑、或 submit-pr 预检回填），用 `-CiRunUrl / -ScreenshotRelPath`
    传入即得逐门真产物。

.EXAMPLE
    . scripts/lib/evidence-gen.ps1
    $result = New-Evidence -Level 'standard' -CompileResult '...' -ScreenshotDiff @(...)
#>

function New-Evidence {
    [CmdletBinding()]
    param(
        [ValidateSet('quick', 'standard', 'full')]
        [string]$Level = 'quick',
        [string]$CompileResult = '',
        [array]$ScreenshotDiff = @(),
        [int]$ScreenshotChangedCount = 0,
        [string]$OutputPath = '',
        [string]$ProjectDir,
        # —— 证据齐了再开 PR（#1770）：产物在 push 之后就能取到；不传则该门结论落「待补：<说明>」（如实判红，非假绿）——
        # CI run 链接（③）。形如 https://github.com/<owner>/<repo>/actions/runs/<id>
        [string]$CiRunUrl = '',
        # 仓库内截图相对路径（①）。形如 docs/verification/<模块>/<票号>/<页名>.jpg（目录键须数字）
        [string]$ScreenshotRelPath = '',
        # ① 行执行人栏：①a 由 agent 出证（ADR-0016）默认「agent 执行」；命中能力面 ①b 时由人给原文后代录
        [string]$Executor = 'agent 执行'
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputPath) {
        $OutputPath = Join-Path $ProjectDir '.ci-verify\evidence.md'
    }

    $date = Get-Date -Format 'yyyy-MM-dd'

    # 形状的唯一真源（#1920）：五门 / 字段 / 行形态 / ① 免行口径都从判据源读，本模块只负责填值。
    . (Join-Path $PSScriptRoot 'gate-evidence.ps1')
    $descriptor = Get-GateEvidenceDescriptor

    # 🟢 快速模式：不生成文件
    if ($Level -eq 'quick') {
        # 建议句与 ① 免行同源（#1920/#1859）：旧版这里还写着「低风险运行时面」的退役口径，
        # 照它写正文会被 pr-evidence 判红 —— 字面量在这里就等于第二个真源。
        $exemptText = [string]$descriptor.exemptions.'①'[0].text
        return [pscustomobject]@{
            Generated = $false
            Path      = ''
            Content   = ''
            Message   = $exemptText
        }
    }

    # ③ 结论：有 CI run 链接即真产物，否则「待补」（含 `actions/runs/<id>` 字面会假绿，故占位句里不写这个词）
    if ($CiRunUrl) {
        $ciText = $CiRunUrl
    }
    else {
        $ciText = '待补：CI 跑完后贴 mobile-test 的 run 链接（本仓 npm run test:unit 由 CI 的 mobile-test job 断言）'
    }

    # ① 结论：有仓库内截图路径即真产物，否则「待补」（占位句刻意不含 docs/verification/<数字>/ 结构，避免假绿）
    if ($ScreenshotRelPath) {
        $shotText = $ScreenshotRelPath
    }
    else {
        $shotText = '待补：①a 逐页截图入库到 docs/verification 后的仓库内相对路径，或 -PostToPr 贴 sha 绑定评论后写「见评论」'
    }
    if ($ScreenshotDiff.Count -gt 0) {
        $shotSubject = "增量构建后真机逐页截图（$($ScreenshotDiff.Count) 页，$ScreenshotChangedCount 页有变化）"
    }
    else {
        $shotSubject = '真机逐页截图（本轮无截图产出，待人工签收）'
    }

    # ④ 结论：有编译结果串即真产物；否则「待补」——不再硬编码 errors=0（那是本仓自认的「永远绿」之一）
    if ($CompileResult) {
        $compileText = $CompileResult
    }
    else {
        $compileText = '待补：跑 npm run build:kotlin-all（或 pwsh scripts/kotlin-all-check.ps1 -PostToPr <PR号>）后引用编译日志或写「见评论」'
    }

    $lines = @()
    $lines += [string]$descriptor.heading
    $lines += ''
    $lines += Format-GateEvidenceRow -Descriptor $descriptor -Gate (Get-GateEvidenceGate -Key '①' -Descriptor $descriptor) -Values @{
        '执行人' = $Executor; '日期' = $date; '复测对象' = $shotSubject; '结论' = $shotText
    }
    $lines += Format-GateEvidenceRow -Descriptor $descriptor -Gate (Get-GateEvidenceGate -Key '③' -Descriptor $descriptor) -Values @{
        '结论' = $ciText
    }
    $lines += Format-GateEvidenceRow -Descriptor $descriptor -Gate (Get-GateEvidenceGate -Key '④' -Descriptor $descriptor) -Values @{
        '执行人' = $Executor; '日期' = $date; '复测对象' = 'npm run build:kotlin-all'; '结论' = $compileText
    }

    $contentText = $lines -join "`n"

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
