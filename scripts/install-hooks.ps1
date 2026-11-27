#!/usr/bin/env pwsh
<#
.SYNOPSIS
    安装仓库级 git 钩子：把 core.hooksPath 指向 .githooks/，并把路径编码默认值落成 UTF-8 原样输出。

.DESCRIPTION
    钩子本体在 .githooks/pre-commit（入库），本脚本只做本地接线。
    新克隆的仓库跑一次即可；CI 不跑钩子。

    同时把 core.quotepath 写成 false（#2873）：本仓工作区路径含中文目录，而 git 默认
    quotepath=true 会把非 ASCII 路径输出成 C 风格转义（"training-app/\346\234…"），
    脚本里没带 `-c core.quotepath=false`、也没走解码器的调用会因此读错路径。
    这两个值都只写进本地 .git/config（不入库），新克隆与 CI 拿到的是 git 默认值——
    本脚本让新克隆与主树一致。注意 worktree 里跑会写进共享的 common config，效果等同主树。
    显式传 `-c core.quotepath=false` 的调用不受影响。

.EXAMPLE
    pwsh scripts/install-hooks.ps1
#>
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    git config core.hooksPath .githooks
    if ($LASTEXITCODE -ne 0) { throw 'git config core.hooksPath 失败' }

    # #2873：中文目录名下的路径编码。同样只写本地配置，靠本脚本在新克隆里重建。
    git config core.quotepath false
    if ($LASTEXITCODE -ne 0) { throw 'git config core.quotepath 失败' }

    # Linux/macOS 上钩子需要可执行位；Windows 忽略该位，失败不阻断。
    git update-index --chmod=+x .githooks/pre-commit 2>$null | Out-Null

    Write-Host "已安装：core.hooksPath = $(git config --get core.hooksPath)；core.quotepath = $(git config --get core.quotepath)"
    Write-Host "钩子：.githooks/pre-commit（单文件 > 20000 字节拒绝提交）"
} finally {
    Pop-Location
}
