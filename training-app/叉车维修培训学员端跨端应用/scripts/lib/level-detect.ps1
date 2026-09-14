<#
.SYNOPSIS
    改动级别判定模块：根据 git diff 分析改动集，自动判定 🟢快速 / 🟡标准 / 🔴完整 三级。

.DESCRIPTION
    判定逻辑：
      - 改动集包含 manifest.json / pages.json / platformConfig.json → full（🔴）
      - 改动集包含新增的 .uvue 页面文件 → full（🔴）
      - 改动集包含 .uts 文件 → standard（🟡）
      - 其他（只改 .uvue 模板/样式）→ quick（🟢）

    支持 -ForceLevel 参数覆盖自动判定。

.EXAMPLE
    . scripts/lib/level-detect.ps1
    $result = Get-DetectLevel -ProjectDir "D:\FL\training-app\叉车维修培训学员端跨端应用"
    Write-Host "Level: $($result.Level) — $($result.Reason)"
#>

function Get-DetectLevel {
    [CmdletBinding()]
    param(
        [string]$ProjectDir,
        [ValidateSet('quick', 'standard', 'full', '')]
        [string]$ForceLevel = ''
    )

    # 强制覆盖
    if ($ForceLevel) {
        return [pscustomobject]@{
            Level        = $ForceLevel
            Reason       = "强制指定 -Level $ForceLevel"
            ChangedFiles = @()
        }
    }

    # 获取改动文件列表
    $changedFiles = @()
    $gitArgs = @('diff', '--name-only', 'HEAD')
    if ($ProjectDir) {
        $gitArgs = @('-C', $ProjectDir) + $gitArgs
    }
    try {
        $raw = & git @gitArgs 2>$null
        if ($raw) {
            $changedFiles = @($raw | Where-Object { $_ -and $_.Trim() } | ForEach-Object { $_.Trim() })
        }
    } catch {
        # git 不可用，降级为 quick
        return [pscustomobject]@{
            Level        = 'quick'
            Reason       = 'git 不可用，无法分析改动集，默认 quick'
            ChangedFiles = @()
        }
    }

    # 也检查暂存区（staged changes）
    $stagedFiles = @()
    $stagedArgs = @('diff', '--cached', '--name-only')
    if ($ProjectDir) {
        $stagedArgs = @('-C', $ProjectDir) + $stagedArgs
    }
    try {
        $stagedRaw = & git @stagedArgs 2>$null
        if ($stagedRaw) {
            $stagedFiles = @($stagedRaw | Where-Object { $_ -and $_.Trim() } | ForEach-Object { $_.Trim() })
        }
    } catch { }

    # 合并去重
    $allFiles = @($changedFiles + $stagedFiles) | Select-Object -Unique

    if ($allFiles.Count -eq 0) {
        return [pscustomobject]@{
            Level        = 'quick'
            Reason       = '无改动文件（空 diff），默认 quick'
            ChangedFiles = @()
        }
    }

    # 判定逻辑：🔴 完整 → 🟡 标准 → 🟢 快速

    # 🔴 完整：配置文件变更
    $configPatterns = @('manifest\.json$', 'pages\.json$', 'platformConfig\.json$')
    foreach ($f in $allFiles) {
        foreach ($pat in $configPatterns) {
            if ($f -match $pat) {
                return [pscustomobject]@{
                    Level        = 'full'
                    Reason       = "配置文件变更: $f"
                    ChangedFiles = $allFiles
                }
            }
        }
    }

    # 🔴 完整：新增 .uvue 页面文件（检查暂存区是否有新增）
    $newPages = @()
    $statusArgs = @('diff', '--cached', '--name-only', '--diff-filter=A')
    if ($ProjectDir) {
        $statusArgs = @('-C', $ProjectDir) + $statusArgs
    }
    try {
        $newRaw = & git @statusArgs 2>$null
        if ($newRaw) {
            $newPages = @($newRaw | Where-Object { $_ -match '\.uvue$' })
        }
    } catch { }

    if ($newPages.Count -gt 0) {
        return [pscustomobject]@{
            Level        = 'full'
            Reason       = "新增页面文件: $($newPages -join ', ')"
            ChangedFiles = $allFiles
        }
    }

    # 🟡 标准：有 .uts 逻辑变更
    $utsFiles = @($allFiles | Where-Object { $_ -match '\.uts$' })
    if ($utsFiles.Count -gt 0) {
        return [pscustomobject]@{
            Level        = 'standard'
            Reason       = "逻辑变更（.uts）: $($utsFiles -join ', ')"
            ChangedFiles = $allFiles
        }
    }

    # 🟢 快速：只改 .uvue 模板/样式或其他文件
    return [pscustomobject]@{
        Level        = 'quick'
        Reason       = "纯样式/文案改动（$($allFiles.Count) 个文件）"
        ChangedFiles = $allFiles
    }
}
