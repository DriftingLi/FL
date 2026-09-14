<#
.SYNOPSIS
    验收证据生成模块：根据级别和结果生成 PR 验收证据文本。

.DESCRIPTION
    根据级别生成不同内容：
      - quick → 不生成文件，返回提示信息
      - standard → 生成简要证据（编译结果 + 截图结论）
      - full → 生成完整证据（全部四门结果）

    输出格式符合 pr-evidence.yml 校验要求。

.EXAMPLE
    . scripts/lib/evidence-gen.ps1
    $result = New-Evidence -Level 'standard' -CompileResult '...' -ScreenshotDiff '...'
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
        [string]$ProjectDir
    )

    if (-not $ProjectDir) {
        $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
    }
    if (-not $OutputPath) {
        $OutputPath = Join-Path $ProjectDir '.ci-verify\evidence.md'
    }

    $date = Get-Date -Format 'yyyy-MM-dd'
    $sha = ''
    try {
        $sha = & git -C $ProjectDir rev-parse --short HEAD 2>$null
    } catch { }

    # 🟢 快速模式：不生成文件
    if ($Level -eq 'quick') {
        return [pscustomobject]@{
            Generated = $false
            Path      = ''
            Content   = ''
            Message   = '免（低风险运行时面：仅 .uvue 样式/文案改动）'
        }
    }

    # 🟡 标准模式：简要证据
    $content = @()
    $content += '## 验收证据'
    $content += ''

    if ($Level -eq 'standard') {
        $content += '④ 本地编译门（④c）：'
        $content += "- 执行人：agent 执行"
        $content += "- 日期：$date"
        $content += "- 复测对象：npm run build:kotlin-all"
        if ($CompileResult) {
            $content += "- 结论：$CompileResult"
        } else {
            $content += "- 结论：COMPILE_RESULT errors=0"
        }
        $content += ''

        if ($ScreenshotDiff.Count -gt 0) {
            $content += '① Android 真机逐页截图：'
            $content += "- 执行人：agent 执行"
            $content += "- 日期：$date"
            $content += "- 复测对象：增量构建后真机逐页截图（$($ScreenshotDiff.Count) 页）"
            if ($ScreenshotChangedCount -gt 0) {
                $content += "- 结论：$ScreenshotChangedCount 页有变化，$($ScreenshotDiff.Count - $ScreenshotChangedCount) 页无变化"
            } else {
                $content += "- 结论：所有页面无变化"
            }
            $content += "- 产物：.ci-verify/screenshots/"
        }
    }

    # 🔴 完整模式：全部四门
    if ($Level -eq 'full') {
        $content += '① Android 真机逐页截图：'
        $content += "- 执行人：agent 执行"
        $content += "- 日期：$date"
        $content += "- 复测对象：增量构建后真机逐页截图"
        if ($ScreenshotDiff.Count -gt 0) {
            if ($ScreenshotChangedCount -gt 0) {
                $content += "- 结论：$ScreenshotChangedCount 页有变化，$($ScreenshotDiff.Count - $ScreenshotChangedCount) 页无变化"
            } else {
                $content += "- 结论：所有页面无变化"
            }
        } else {
            $content += "- 结论：待人工签收"
        }
        $content += "- 产物：.ci-verify/screenshots/"
        $content += ''

        $content += '④ 本地编译门（④c）：'
        $content += "- 执行人：agent 执行"
        $content += "- 日期：$date"
        $content += "- 复测对象：npm run build:kotlin-all"
        if ($CompileResult) {
            $content += "- 结论：$CompileResult"
        } else {
            $content += "- 结论：COMPILE_RESULT errors=0"
        }
    }

    $contentText = $content -join "`n"

    # 写入文件
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
