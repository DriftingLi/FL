<#
.SYNOPSIS
    改动级别判定模块：根据 git diff 分析改动集，自动判定 🟢快速 / 🟡标准 / 🔴完整 三级。

.DESCRIPTION
    判定逻辑：
      - 改动集包含 manifest.json / pages.json / platformConfig.json → full（🔴）
      - 改动集包含新增的 .uvue 页面文件 → full（🔴）
      - 改动集包含 .uts 文件 → standard（🟡）
      - 其他（未命中运行时面）→ quick（🟢），且 `Reason` **分两种写清**：
          · 改动集里**只有**工具链/测试/文档类文件（`.ps1`/`.js`/`.mjs`/`.json`/`.md`）
            ⇒ 原因写「工具链/测试改动，未命中运行时面」——**不要**把它报成「纯样式/文案改动」
            （改脚本不是样式改动；这类改动按契约测试间接验证，不会自动跑真机）
          · 其余（含只改 `.uvue` 模板/样式）⇒ 原因写「纯样式/文案改动，未命中运行时面」

    支持 -ForceLevel 参数覆盖自动判定。

.EXAMPLE
    . scripts/lib/level-detect.ps1
    $result = Get-DetectLevel -ProjectDir "D:\FL\training-app\叉车维修培训学员端跨端应用"
    Write-Host "Level: $($result.Level) — $($result.Reason)"
#>

function ConvertFrom-GitQuotedPath {
    <#
      把 git 的**引号+八进制转义**路径还原成真实路径（纯函数，便于运行期断言）。
      为什么需要：`core.quotepath=true`（git 默认）下，**非 ASCII 路径**会被整条加引号并转义，
      例如 `"training-app/\345\217\211.../AGENTS.md"` —— 末尾是 `md"` 而**不是** `md`
      ⇒ 任何 `-match '\.md$'` / `'\.uvue$'` / `'\.uts$'` 一律**静默失配**。
      2026-09-15 实测踩到：改了一堆中文目录下的 `.ps1`/`.js`/`.md`，工具链分支从未命中。
      ⇒ 纯 ASCII 路径（如 `pages/**/*.uvue`）不受影响，但**中文名**的页面/脚本会漏判。

      ⚠️ 不能把 `\ooo` 直接转成 `[char]`：那会把 UTF-8 的**每个字节**当成一个 Latin-1 字符
      （`叉` = E5 8F 89 ⇒ 长度 3 而非 1），路径就废了。必须**先攒字节数组、再按 UTF-8 解码**。
    #>
    param([string]$Path)
    if ([string]::IsNullOrEmpty($Path)) { return $Path }
    if ($Path -notmatch '^"(.*)"$') { return $Path }
    $inner = $Matches[1]
    $bytes = New-Object System.Collections.Generic.List[byte]
    $i = 0
    while ($i -lt $inner.Length) {
        if ($inner[$i] -eq '\' -and ($i + 3) -lt $inner.Length -and $inner.Substring($i + 1, 3) -match '^[0-7]{3}$') {
            $bytes.Add([byte][Convert]::ToInt32($inner.Substring($i + 1, 3), 8))
            $i += 4
        }
        elseif ($inner[$i] -eq '\' -and ($i + 1) -lt $inner.Length) {
            $bytes.Add([byte][int][char]$inner[$i + 1])   # 转义的字面字符（ASCII）
            $i += 2
        }
        else {
            $bytes.Add([byte][int][char]$inner[$i])
            $i++
        }
    }
    return [System.Text.Encoding]::UTF8.GetString($bytes.ToArray())
}

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

    # 路径归一化见**文件作用域**的 ConvertFrom-GitQuotedPath（与本函数同级，便于被运行期断言直接测）。

    # 获取改动文件列表
    $changedFiles = @()
    $gitArgs = @('diff', '--name-only', 'HEAD')
    if ($ProjectDir) {
        $gitArgs = @('-C', $ProjectDir) + $gitArgs
    }
    try {
        $raw = & git @gitArgs 2>$null
        if ($raw) {
            $changedFiles = @($raw | Where-Object { $_ -and $_.Trim() } | ForEach-Object { ConvertFrom-GitQuotedPath $_.Trim() })
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
            $stagedFiles = @($stagedRaw | Where-Object { $_ -and $_.Trim() } | ForEach-Object { ConvertFrom-GitQuotedPath $_.Trim() })
        }
    } catch { }

    # 合并去重
    # ⚠️ `@()` 必须包住**整条管道**（含 Select-Object），不能只包前面的相加：
    #    空 diff 时管道**什么都不输出** ⇒ 变量本身为 $null ⇒ `Set-StrictMode -Latest` 下
    #    `$allFiles.Count` 直接抛「The property 'Count' cannot be found on this object」
    #    （2026-09-14 冷启动会话按交接文档 §8 跑第一条命令时踩到；同类先例见 hx-run.ps1「调用点必须包 @()」）
    $allFiles = @(@($changedFiles + $stagedFiles) | Select-Object -Unique)

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
            $newPages = @($newRaw | Where-Object { $_ -and $_.Trim() } | ForEach-Object { ConvertFrom-GitQuotedPath $_.Trim() } | Where-Object { $_ -match '\.uvue$' })
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

    # 🟢 快速：未命中运行时面（`.uvue` / `.uts` / 三份 json 都不是）。但**原因分两种写清** ——
    #    否则「改了工具链脚本」会被报成「纯样式/文案改动」。2026-09-14 两次实际会话都踩到这个误导：
    #    改 `.ps1` 被判 🟢、文案却说成样式改动，而真相是「工具链改动、按契约测试间接验证」。
    #    分档本身（🟢）不变：这些文件确实不命中运行时面；变的是**告诉用户为什么**。
    $uvueFiles = @($allFiles | Where-Object { $_ -match '\.uvue$' })
    $toolchainFiles = @($allFiles | Where-Object { $_ -match '\.(ps1|js|mjs|json|md)$' })

    if ($uvueFiles.Count -eq 0 -and $toolchainFiles.Count -gt 0) {
        return [pscustomobject]@{
            Level        = 'quick'
            Reason       = "工具链/测试改动，未命中运行时面（$($toolchainFiles.Count) 个文件）—— 按契约测试间接验证，不会自动跑真机"
            ChangedFiles = $allFiles
        }
    }

    return [pscustomobject]@{
        Level        = 'quick'
        Reason       = "纯样式/文案改动，未命中运行时面（$($allFiles.Count) 个文件）"
        ChangedFiles = $allFiles
    }
}
