<#
.SYNOPSIS
    截图门（`dev:finish` 步骤 7）的**纯判定**：把「有变化要不要红」这件事从 I/O 里拆出来，好被真跑。

.DESCRIPTION
    为什么要有这个文件：修 #1139 之前的步骤 7 在「有变化」时**只 `Write-Host` 一行黄字** ——
    没有 `Write-Result`、没有 `exit` ⇒ **永不 fail**。把它改「会 fail」时，最大的风险是
    改写成一个**仍然永不 fail** 的分支（那正是本仓三个「永远绿」先例的成因）。
    所以判定与文案被拆成这里的纯函数：`screenshotDiffBehavior.test.js` 用 pwsh **真跑**它，
    成对断言「无变化 ⇒ 绿 / 有变化 ⇒ 红」——文本守护做不到这件事。

    判定表（判据预登记，逐条都有对应用例）：

      | 输入                                            | 结论        | 动作          | 退出码 |
      |-------------------------------------------------|-------------|---------------|--------|
      | `NewBaseline = true` 且未 `-UpdateBaseline`      | ok          | 自动写基线    | 0      |
      | `ChangedCount = 0`                              | ok          | 无            | 0      |
      | `ChangedCount > 0` 且未 `-UpdateBaseline`        | **fail**    | 无（要人裁决）| **1**  |
      | `ChangedCount > 0` 且带 `-UpdateBaseline`        | ok          | 刷新基线      | 0      |

    两个刻意选择：
      1. **首次运行（基线为空）自动写入基线并 pass** —— 首次没有「有意改动 vs 噪声」可裁决，
         卡在这里只会逼人加 `-UpdateBaseline` 再跑一遍（多花一轮真机）。这一条把
         「基线永远是空的」那个死循环（ADR-0008:419）直接掐掉。
      2. **默认 fail、`-UpdateBaseline` 是唯一出路** —— 「确认是有意改动」这个判断只能由人看过
         diff 之后做，`-UpdateBaseline` 就是那个动作的载体；默认不阻塞会重演「配好了没人用」。
#>

function Get-PngDiffVerdict {
    <#
    .SYNOPSIS
        由 `Compare-ScreenshotBaseline` 的结果算出步骤 7 的结论（纯函数、不碰文件系统）。

    .OUTPUTS
        [pscustomobject]：
          Ok        该步骤是否算通过（$false ⇒ 调用方必须 `Write-Result $false` + 非零退出）
          Action    'write-baseline' | 'none' | 'request-decision' | 'refresh-baseline'
          ExitCode  0 / 1
          Message   给人看的结论句（**不**含 ✅/❌ 前缀，由调用方加）
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]$DiffResult,
        [switch]$UpdateBaseline
    )

    $changed = [int]$DiffResult.ChangedCount
    $isNew = [bool]$DiffResult.NewBaseline
    $hasError = -not [string]::IsNullOrEmpty([string]$DiffResult.Error)

    if ($hasError) {
        # 「无当前截图可对比」是**判不了**，不是「无变化」—— 判不了必须红（同 png-diff 的纪律）。
        return [pscustomobject]@{
            Ok = $false; Action = 'request-decision'; ExitCode = 1
            Message = "截图对比无法判定：$($DiffResult.Error)"
        }
    }

    if ($isNew) {
        if ($UpdateBaseline) {
            return [pscustomobject]@{
                Ok = $true; Action = 'write-baseline'; ExitCode = 0
                Message = "首次运行，已建立基线（$changed 页本轮截图已写入基线）"
            }
        }
        return [pscustomobject]@{
            Ok = $true; Action = 'write-baseline'; ExitCode = 0
            Message = "首次运行：基线为空 ⇒ 已自动以本轮截图建立基线（$changed 页）；此后每轮都会真比"
        }
    }

    if ($changed -eq 0) {
        return [pscustomobject]@{
            Ok = $true; Action = 'none'; ExitCode = 0
            Message = '所有页面无变化'
        }
    }

    if ($UpdateBaseline) {
        return [pscustomobject]@{
            Ok = $true; Action = 'refresh-baseline'; ExitCode = 0
            Message = "$changed 页有变化，已按 -UpdateBaseline 刷新基线（确认过是有意改动才可这么用）"
        }
    }

    return [pscustomobject]@{
        Ok = $false; Action = 'request-decision'; ExitCode = 1
        Message = "$changed 页有变化且未确认：确认是有意改动 ⇒ 重跑并加 -UpdateBaseline；否则修回"
    }
}
