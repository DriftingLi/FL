#!/usr/bin/env pwsh
<#
.SYNOPSIS
    安装仓库级 git 钩子：把 core.hooksPath 指向 .githooks/。

.DESCRIPTION
    钩子本体在 .githooks/pre-commit（入库），本脚本只做本地接线。
    新克隆的仓库跑一次即可；CI 不跑钩子。

.EXAMPLE
    pwsh scripts/install-hooks.ps1
#>
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    git config core.hooksPath .githooks
    if ($LASTEXITCODE -ne 0) { throw 'git config core.hooksPath 失败' }

    # Linux/macOS 上钩子需要可执行位；Windows 忽略该位，失败不阻断。
    git update-index --chmod=+x .githooks/pre-commit 2>$null | Out-Null

    Write-Host "已安装：core.hooksPath = $(git config --get core.hooksPath)"
    Write-Host "钩子：.githooks/pre-commit（单文件 > 20000 字节拒绝提交）"
} finally {
    Pop-Location
}
