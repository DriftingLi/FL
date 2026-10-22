# 本地预检：对「将要推的那个 commit」跑与 CI 同一份判据（#1770 S5）。
#
# 判据不在这里——这里只是 CLI 外壳，真正的判据在 .github/scripts/lib/gate-validator.mjs，
# 与 pr-evidence.yml / mobile-gates.yml 读的是同一个文件。
#
# 退出码：0 = 绿（可以开 PR）；1 = 红（打印缺哪几门）；2 = 用法 / 环境错误。
param(
    [string]$BodyFile = '',
    [int]$Pr = 0,
    [string]$Base = 'origin/master',
    [string]$Commit = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $cliArgs = @()
    if ($BodyFile) { $cliArgs += @('--body-file', (Resolve-Path $BodyFile).Path) }
    if ($Pr -gt 0) { $cliArgs += @('--pr', "$Pr") }
    if ($Base) { $cliArgs += @('--base', $Base) }
    if ($Commit) { $cliArgs += @('--commit', $Commit) }
    $cli = Join-Path $repoRoot '.github/scripts/preflight-pr.mjs'
    & node $cli @cliArgs
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}