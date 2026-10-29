# 本地预检：对「将要推的那个 commit」跑与 CI 同一份判据（#1770 S5）。
#
# 判据不在这里——这里只是 CLI 外壳，真正的判据在 .github/scripts/lib/gate-validator.mjs，
# 与 pr-evidence.yml 读的是同一个文件（A1，#2100 起只读 PR 正文，不再读任何评论）。
#
# 退出码：0 = 绿（可以开 PR）；1 = 红（打印缺哪几门）；2 = 用法 / 环境错误。
param(
    [string]$BodyFile = '',
    [int]$Pr = 0,
    [string]$Base = 'origin/master',
    [string]$Commit = ''
)

$ErrorActionPreference = 'Stop'
# 同 submit-pr.ps1（#1853 范围追加）：node 输出按控制台代码页解码会吞 UTF-8 多字节字符的尾节，
# 入口先钉 UTF-8 与控制台代码页解耦（判据在 .github/scripts/lib/gate-validator.mjs，不在这里）。
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
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