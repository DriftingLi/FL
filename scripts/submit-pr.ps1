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

    # 第一遍只看 ci-summary（红也能读出 JSON）：③ 门的 run 链接要写进正文才能判绿，
    # 而链接只有 check-runs 里有——这就是 S4「不再要人手抄」的落点。
    $probe = (& node $cli --json --body-file $BodyFile --base "origin/$Base" 2>$null) | Out-String
    try { $probeJson = $probe | ConvertFrom-Json } catch { throw "预检没吐出 JSON：$probe" }
    $ciRun = $probeJson.ciRun
    if (-not $ciRun) { throw "commit $($head.Substring(0,7)) 上还没有 ci-summary（先把分支推到 origin，等 CI 跑完再来）" }
    if ($ciRun.conclusion -ne 'success') { throw "ci-summary 当前是 $($ciRun.conclusion)：③ 门没有可引用的绿 run，等它绿再来（别贴一个没绿的链接）" }

    # ③ run 链接：交给同一支 node（--fill-body）补，PowerShell 不再自己拼字符串 ——
    # 「占位不补 / 已有链接不补」两个判据也就只有一个实现，可被单测钉住。
    #
    # 但**预检已经绿就别补**：非运行时面 PR 按约定只写「免（未命中运行时面）」，正文里本来就没有 ③ 行，
    # 此时去补会以 no-gate-3-line 误报（#1770 自己的 PR 就是这一类，踩过一次）。
    if ($probeJson.ok) {
        Write-Host '预检已绿：正文无需补 ③ run 链接'
    } else {
        $fill = (& node $cli --fill-body --body-file $BodyFile) | Out-String
        try { $fillJson = $fill | ConvertFrom-Json } catch { throw "补 ③ 行没吐出 JSON：$fill" }
        if ($fillJson.filled) {
            Write-Host "③ 行已自动补上 ci-summary run 链接：$($fillJson.ciRun.url)"
        } elseif ($fillJson.reason -eq 'already-has-run') {
            Write-Host '③ 行已有 run 链接，未改动'
        } elseif ($fillJson.reason -in @('conclusion-placeholder', 'no-conclusion-field')) {
            throw "③ 行的结论还没写（$($fillJson.reason)）：先在正文写清结论（如「两腿全绿」），再重跑 —— 补链接不替你写结论"
        } elseif ($fillJson.reason -eq 'no-gate-3-line') {
            throw '正文里没有 ③ 行：按模板补一行（行首 ③、含「结论：」）'
        } else {
            throw "③ 行补链接失败：$($fillJson.reason)"
        }
    }

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