<#
.SYNOPSIS
    Triage label 互斥机检：列出同时挂 ready-for-agent 与 ready-for-mobile-agent 的票。

.DESCRIPTION
    判据源 docs/agents/triage-labels.md「移动端变体」：两套字符串按票所属技术栈取用其一，
    **不要同时挂**。这是确定集合的机械判据（2026-11-25 retro 立），不靠 review 记忆；
    关闭票不必摘 label（triage-labels.md 已裁定「标签保持原样」），所以默认只扫 open。

.PARAMETER State
    open（默认）/ closed / all。默认只看 open：关闭票的 label 是惰性的，扫它只增噪。

.PARAMETER Json
    输出 JSON 数组（每项 number / title / labels），供其他脚本消费；人类可读模式只在冲突时退出 1。

.EXAMPLE
    pwsh scripts/check-triage-labels.ps1

.EXAMPLE
    pwsh scripts/check-triage-labels.ps1 -State all -Json

.NOTES
    退出码：0 = 无冲突；1 = 有冲突（票号已列出）；2 = 环境不可用（gh 缺失 / 未登录 / 查询失败）。
    依赖 gh CLI（本仓 issue tracker 走 GitHub Issues）。
#>
[CmdletBinding()]
param(
    [ValidateSet('open', 'closed', 'all')]
    [string]$State = 'open',

    [switch]$Json
)

$ErrorActionPreference = 'Stop'

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    Write-Host '环境中找不到 gh CLI：本检查依赖 GitHub Issues 查询，先安装并 gh auth login。' -ForegroundColor Red
    exit 2
}

$labels = @('ready-for-agent', 'ready-for-mobile-agent')

$raw = & gh issue list --state $State --limit 2000 --json number,title,labels 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "gh issue list 失败（exit=$LASTEXITCODE）：$raw" -ForegroundColor Red
    exit 2
}

$issues = @($raw | ConvertFrom-Json)

$hits = @(
    foreach ($issue in $issues) {
        $names = @($issue.labels | ForEach-Object { $_.name })
        if (($names -contains $labels[0]) -and ($names -contains $labels[1])) {
            [pscustomobject]@{
                number = [int]$issue.number
                title  = [string]$issue.title
                labels = $names
            }
        }
    }
)

if ($Json) {
    $hits | ConvertTo-Json -Depth 4
} elseif ($hits.Count -eq 0) {
    Write-Host "OK：无互斥 triage label（state=$State，扫描 $($issues.Count) 张票）。" -ForegroundColor Green
} else {
    Write-Host "发现 $($hits.Count) 张票同时挂 $($labels[0]) 与 $($labels[1])（判据源 docs/agents/triage-labels.md）：" -ForegroundColor Red
    foreach ($hit in ($hits | Sort-Object number)) {
        Write-Host ("  #{0} {1}" -f $hit.number, $hit.title)
    }
    Write-Host '处置：按票所属技术栈保留其一 —— 移动端留 ready-for-mobile-agent，其余留 ready-for-agent。'
}

if ($hits.Count -gt 0) { exit 1 }
exit 0
