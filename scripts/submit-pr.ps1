# 建 PR 的唯一入口：预检绿 + ci-summary 绿才建（#1770 S6）。
#
# 目的不是多一道手续，而是把「先开 PR 再补证据」这个导致首推必红的顺序堵在工具上——
# 靠人自律守不住（#1770 的根因就是顺序，不是人不认真）。判据在
# .github/scripts/lib/gate-validator.mjs，本脚本只做：预检 → 补 ③ run 链接 → gh pr create。
#
# 用法：
#   pwsh -NoProfile -File scripts/submit-pr.ps1 -Title "fix(ci): ..." -BodyFile .scratch/pr-body.md
#   pwsh -NoProfile -File scripts/submit-pr.ps1 -Title "..." -BodyFile ... -Draft -DryRun   # 只自检不建
param(
    [Parameter(Mandatory = $true)][string]$Title,
    [string]$BodyFile = '.scratch/pr-body.md',
    [string]$Base = 'master',
    [switch]$Draft,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
# 本机控制台是 GBK（#1853 范围追加）：node 的 stdout 是 UTF-8 JSON，`& node` 直捕按控制台代码页
# 解码会把「①」等字符的 UTF-8 尾字节与后随 `"` 拼成双字节对**吞掉引号** ⇒ 键名截断 ⇒
# ConvertFrom-Json 必炸「预检没吐出 JSON」（2026-10-24 两次复现）。入口先钉 UTF-8，
# 让下面三处 `& node` 的捕获与控制台代码页解耦；与判据解耦的本机 git 配置不改。
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $cli = Join-Path $repoRoot '.github/scripts/preflight-pr.mjs'
    if (-not (Test-Path -LiteralPath $BodyFile)) { throw "PR 正文文件不存在：$BodyFile" }

    $branch = (& git rev-parse --abbrev-ref HEAD).Trim()
    if ($branch -in @('master', 'main', 'HEAD')) { throw "当前分支是 $branch —— PR 必须从功能分支开" }
    $head = (& git rev-parse HEAD).Trim()
    $pushed = (& git rev-parse "origin/$branch" 2>$null)
    if ($LASTEXITCODE -ne 0 -or -not $pushed) { throw "origin/$branch 不存在：先 git push -u origin $branch" }
    if ($pushed.Trim() -ne $head) { throw "本地 HEAD 与 origin/$branch 不一致：先 push（预检与 CI 必须看同一个 commit）" }
    if ((& git status --porcelain | Measure-Object).Count -gt 0) {
        Write-Warning '工作区不干净：PR 只包含已提交内容，未提交改动不会被评审看到（确认这是有意的再继续）。'
    }

    # 第一遍先看 ci-summary（红也能读出 JSON）。A1（#2100）后正文**不再写 ③ 的 run 链接**
    # （③ 判据已判纯税：两腿在 ci-summary 里，正文抄一遍零信息），但「测试绿了没」仍是开 PR 的前置事实 ——
    # 不许在一个有红腿的 commit 上开 PR，那会让评审看到一份「证据齐、测试红」的自相矛盾。
    $probe = (& node $cli --json --body-file $BodyFile --base "origin/$Base" 2>$null) | Out-String
    try { $probeJson = $probe | ConvertFrom-Json } catch { throw "预检没吐出 JSON：$probe" }
    $ciRun = $probeJson.ciRun
    if (-not $ciRun) { throw "commit $($head.Substring(0,7)) 上还没有 ci-summary（先把分支推到 origin，等 CI 跑完再来）" }
    if ($ciRun.conclusion -ne 'success') { throw "ci-summary 当前是 $($ciRun.conclusion)：先让两腿跑绿再来开 PR" }

    # 第二遍：正文补完后再判一次，绿了才建 PR（这一步与 CI 同一份判据）。
    & node $cli --body-file $BodyFile --base "origin/$Base"
    if ($LASTEXITCODE -ne 0) { throw '预检红：按上面的缺项补齐后重跑本脚本（未建 PR）' }

    if ($DryRun) {
        Write-Host "DryRun：预检绿、ci-summary 绿、正文 $BodyFile —— 未执行 gh pr create"
        exit 0
    }

    $ghArgs = @('pr', 'create', '--base', $Base, '--head', $branch, '--title', $Title, '--body-file', $BodyFile)
    if ($Draft) { $ghArgs += '--draft' }
    $url = & gh @ghArgs
    if ($LASTEXITCODE -ne 0) { throw 'gh pr create 失败（看上面的报错）' }
    Write-Host "已建 PR：$url"
}
catch {
    Write-Host ""
    Write-Host "未建 PR：$($_.Exception.Message)"
    exit 1
}
finally {
    Pop-Location
}