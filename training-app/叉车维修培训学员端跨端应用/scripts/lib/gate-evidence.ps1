<#
.SYNOPSIS
    验收证据**形状**的 PowerShell 侧读取器：同一份描述符，node 与 pwsh 读同一段字节。

.DESCRIPTION
    门的形状（五门、必填字段、产物类别、① 免行口径与跳过文案）只有一个真源：
    仓库根 `.github/scripts/lib/gate-validator.mjs` 里 `GATE_EVIDENCE_DESCRIPTOR BEGIN/END`
    marker 圈出的那段**严格 JSON**（#1920）。判据侧（校验器）与预检侧（preflight-pr.mjs）
    是 node，直接 import 那个常量；本机生成侧（evidence-gen.ps1 等）是 pwsh，用本模块抽
    marker 之间的**同一段字节** `ConvertFrom-Json`。

    ⚠️ 别把形状抄一份进 PowerShell：改了描述符却忘了改这边，就是 #1920 要收的单一真源漂移。
    守护：`utils/evidenceGenContract.test.js` 的往返用例（改描述符 ⇒ 生成结果跟着变）。
#>

function Get-GateEvidenceRepoRoot {
    <#
    .SYNOPSIS
        从脚本位置向上找到仓库根（含 `.github/scripts/lib/gate-validator.mjs` 的目录）。
    .DESCRIPTION
        不用固定层数的 `..\..\`：worktree / 子目录搬动会让层数变，找不到就抛错（fail-closed），
        别静默退回到某个常量路径 —— 那正是「本地产物读不到却判绿」的来源。
    #>
    [CmdletBinding()]
    param([string]$StartDir = $PSScriptRoot)

    $dir = Get-Item -LiteralPath $StartDir
    while ($dir) {
        if (Test-Path -LiteralPath (Join-Path $dir.FullName '.github\scripts\lib\gate-validator.mjs')) {
            return $dir.FullName
        }
        $dir = $dir.Parent
    }
    throw "找不到仓库根：从 $StartDir 向上没有 .github\scripts\lib\gate-validator.mjs（本模块只能在仓库内运行）"
}

function Get-GateEvidenceDescriptor {
    <#
    .SYNOPSIS
        读取 `GATE_EVIDENCE` 描述符（marker 之间的严格 JSON）。
    #>
    [CmdletBinding()]
    param([string]$RepoRoot = (Get-GateEvidenceRepoRoot))

    $validator = Join-Path $RepoRoot '.github\scripts\lib\gate-validator.mjs'
    if (-not (Test-Path -LiteralPath $validator)) {
        throw "找不到判据源：$validator"
    }
    $text = [System.IO.File]::ReadAllText($validator, [System.Text.Encoding]::UTF8)
    $begin = '// ===== GATE_EVIDENCE_DESCRIPTOR BEGIN ====='
    $end = '// ===== GATE_EVIDENCE_DESCRIPTOR END ====='
    $i = $text.IndexOf($begin, [System.StringComparison]::Ordinal)
    $j = $text.IndexOf($end, [System.StringComparison]::Ordinal)
    if ($i -lt 0 -or $j -lt $i) {
        throw "判据源里找不到描述符 marker（$begin / $end）：$validator"
    }
    $block = $text.Substring($i + $begin.Length, $j - $i - $begin.Length).Trim()
    # 去掉绑定声明与前后的 JS 语法糖，只留 JSON 本体（与 node 侧 extractDescriptorText 同一手法）
    $json = $block -replace '^export\s+const\s+GATE_EVIDENCE\s*=\s*', '' -replace ';\s*$', ''
    try {
        return $json | ConvertFrom-Json -Depth 12
    }
    catch {
        throw "描述符块不是严格 JSON（PowerShell 侧只能读 JSON）：$($_.Exception.Message)"
    }
}

function Get-GateEvidenceGate {
    <#
    .SYNOPSIS
        按门号取门定义（①/②/③/④/④b）；取不到就抛错。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$Key,
        $Descriptor = (Get-GateEvidenceDescriptor)
    )

    $gate = $Descriptor.gates | Where-Object { $_.key -eq $Key } | Select-Object -First 1
    if (-not $gate) {
        throw "描述符里没有门 $Key（现有：$(($Descriptor.gates | ForEach-Object key) -join ' / ')）"
    }
    return $gate
}

function Format-GateEvidenceRow {
    <#
    .SYNOPSIS
        按描述符把一门的字段值渲染成**单行**证据（字段以 ` · ` 分隔、结论带「（含产物）」）。
    .DESCRIPTION
        行形态（列表项前缀、字段名、字段显示名、分隔符）全部来自描述符；本函数只做拼装。
        校验器按「以门号开头的那一行」取门，所以**必须单行** —— 多行块会被判「缺该行」
        （2026-09-29 #1403 的回归形状，见 evidence-gen.ps1 头注释）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]$Gate,
        [Parameter(Mandatory)][hashtable]$Values,
        $Descriptor = (Get-GateEvidenceDescriptor)
    )

    $conclusionField = [string]$Descriptor.conclusionField
    $sep = [string]$Descriptor.fieldSeparator.display
    $rowPrefix = [string]$Descriptor.listItem.prefix
    $parts = foreach ($field in $Gate.fields) {
        $display = if ($field -eq $conclusionField) { [string]$Descriptor.conclusionLabel } else { [string]$field }
        $value = if ($Values.ContainsKey($field)) { [string]$Values[$field] } else { '' }
        "$display：$value"
    }
    return "$rowPrefix$($Gate.key) $($Gate.label) — $($parts -join $sep)"
}
