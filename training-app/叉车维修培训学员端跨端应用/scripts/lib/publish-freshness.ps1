<#
.SYNOPSIS
    ④c（`scripts/kotlin-all-check.ps1`）publish 步与「导出新鲜度」的判据 —— 两个**纯函数**。

.DESCRIPTION
    为什么单独成库（#1272，2026-09-22）：这两条判据必须能被**真执行**地测到 ——
    `utils/kotlinAllStaleExportBehavior.test.js` dot-source 本文件后直接驱动它们
    （先例 `scripts/lib/capability-surface.ps1` + `utils/capabilitySurfaceBehavior.test.js`）。
    留在门脚本里就只能读源码文本断言，而接线守护**不构成 ③ 证据**（`docs/agents/guards.md`）。
    本文件零副作用（只有函数与常量定义），dot-source 它不会跑门。

    两条判据各自补的那条缝
      - `Get-PublishVerdict`：publish 步**是否成立**。旧判据只认 `与主程序的连接已中断|启动超时`
        两条文案，于是 `-1:cli:命令'publish app-android'不存在或缺少参数` 被放过 —— 而实测那串
        文案**不代表命令不存在**（`cli publish app-android --type appResource --project <p>` 在
        v5.24 存在且可用，同机成功导出 119 个 .kt；主程序忙 / 未就绪时 CLI 也回这句通用文案）。
        故判据改成「输出里有没有**正向成功标记**」，而不是猜文案。
      - `Test-AppResourceFreshness`：产物**是否覆盖当前树**。旧判据只看「有没有 .kt」⇒ 磁盘上留着
        一份旧导出就满足，于是 publish 失败也照常编译出 ✅（#1272 的假绿本体）。

    实测依据（2026-09-22，本机）：工作树**一字未改**时再跑一次 publish，导出目录里 **119/119** 个
    `.kt` 的 mtime 照样前进（16:55:13 → 16:57:37）⇒ 导出是**全量重写**，
    「没改动 ⇒ 不重写」这个反例在本工具链上不成立 ⇒ 「最新 .kt mtime 晚于 publish 基准」是有效判据。

    已知边界（写实，勿读成「已覆盖」）：本库只判**新鲜度**与**这一步是否成立**，不判「产物与源码内容
    是否逐字一致」。内容一致性由「publish 成功标记 + 新鲜度」联合兜住；两者任一不成立即判红。
#>

# publish「成立」的判据 = 输出里的**正向标记**（不是退出码：HBuilderX CLI 退出码恒为 0）。
# 文案随版本可能变 ⇒ 收敛成一处常量，改这里就够。
$PublishSuccessMarker = '导出 android 成功'
# publish「没成立」的判据：命中即判这一步没成立（**不**用来宣告「命令不存在」）。
$PublishFailurePattern = '与主程序的连接已中断|不存在或缺少参数|命令执行错误|启动超时'

<#
 .SYNOPSIS
     publish 步的输出 → 这一步是否成立。
 .OUTPUTS
     hashtable：`Ok`（bool）/ `Reason`（timeout | cli-ipc-blocked | cli-command-failed | no-success-marker | exported）
#>
function Get-PublishVerdict {
    param([string]$Output, [bool]$TimedOut)
    if ($TimedOut) { return @{ Ok = $false; Reason = 'timeout' } }
    if ($Output -match '与主程序的连接已中断') { return @{ Ok = $false; Reason = 'cli-ipc-blocked' } }
    if ($Output -match $PublishFailurePattern) { return @{ Ok = $false; Reason = 'cli-command-failed' } }
    if ($Output -match [regex]::Escape($PublishSuccessMarker)) { return @{ Ok = $true; Reason = 'exported' } }
    return @{ Ok = $false; Reason = 'no-success-marker' }
}

<#
 .SYNOPSIS
     「什么算 appResource 产物」的**唯一定义**：导出目录下所有 `.kt`（排除 `www` 段下的）。

 .DESCRIPTION
     为什么单独成函数（评审发现，2026-09-22）：门脚本与 `Test-AppResourceFreshness` 原先各自抄了一遍
     「递归找 `.kt` + 排除 `\www\`」—— **只有库这一份**进了守护，门那份是没证据的复制品。
     收成一处后，「产物口径」变了不可能只改一方。门脚本收集 `.kt` 喂 kotlinc 也走它。

     ⚠️ 排除用的正则**与路径分隔符无关**（`[\\/]www[\\/]`），这不是洁癖：原先是 Windows-only 的
     `\www\`，而本库的守护在 **ubuntu CI** 上跑 —— 那里 `/tmp/.../www/Y.kt` 不命中 Windows 写法，
     于是「`www` 下的 `.kt` 不算产物」这条口径当场判红（CI run 35709221297，新守护刚落地就抓到的
     潜伏跨平台缺陷；此前没有任何用例在 Linux 上执行过这条过滤）。
#>
function Get-AppResourceKtFiles {
    param([string]$ExportDir)
    return @(Get-ChildItem -LiteralPath $ExportDir -Recurse -Filter *.kt -File -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -notmatch '[\\/]www[\\/]' })
}

<#
 .SYNOPSIS
     导出目录是否**覆盖当前树**：.kt 里最新的 mtime 必须晚于基准时刻。
 .OUTPUTS
     hashtable：`Fresh`（bool）/ `KtCount`（int）/ `Newest`（datetime|null）/ `Reason`（fresh | stale | no-kt）
#>
function Test-AppResourceFreshness {
    param([string]$ExportDir, [datetime]$Since)
    $kt = @(Get-AppResourceKtFiles -ExportDir $ExportDir)
    if ($kt.Count -eq 0) { return @{ Fresh = $false; KtCount = 0; Newest = $null; Reason = 'no-kt' } }
    $newest = ($kt | Sort-Object LastWriteTime -Descending | Select-Object -First 1).LastWriteTime
    if ($newest -gt $Since) { return @{ Fresh = $true; KtCount = $kt.Count; Newest = $newest; Reason = 'fresh' } }
    return @{ Fresh = $false; KtCount = $kt.Count; Newest = $newest; Reason = 'stale' }
}

<#
 .SYNOPSIS
     publish 段的**整段裁决**：给定三步的输出/超时与导出目录 → 该以什么码退出、reason 是什么、日志三件事的取值。

 .DESCRIPTION
     为什么把「裁决」也抽出来（评审发现，2026-09-22）：只抽两个叶子判据，门的**决策序列**（尤其
     `stale-export` ⇒ exit 1 那一支）仍然没有任何用例真跑过 —— 那等于把新加的失败分支写成**没有证据的代码**。
     收成纯函数后，门与守护**共用同一段裁决**：守护用合成的步骤输出驱动它，就能把四个退出路径全跑一遍。

     次序是**先量后判**：即使 publish 没成立，也要量一次导出目录 —— 失败路径**更需要**日志里有 mtime。

 .OUTPUTS
     hashtable：
       `ExitCode`（0 通过 / 1 判红：导出陈旧或无产物 / 2 环境：publish 未成立）
       `Reason`（ok | stale-export | no-artifact | publish-timeout | publish-cli-ipc-blocked |
                 publish-cli-command-failed | publish-no-success-marker）
       `Freshness`（fresh | stale | no-kt）——**一律是实测量**；publish 未成立时也照报实测值（#1285：`fresh` = 导出已刷新但成功文案没读到（采集层方向），`stale`/`no-kt` = 根本没导出（环境方向）——旧的 `not-measured` 短路把处置完全不同的两者糊成同一条红；退出码与 reason 的 fail-closed 不变）
       `KtCount` / `Newest`（导出目录的诊断读数；未成立时也给出，供失败日志使用）
#>
function Get-PublishStageVerdict {
    param([string]$PublishOutput, [bool]$PublishTimedOut, [string]$ExportDir, [datetime]$Since)
    # 先量（失败路径也要能打出 mtime），后判
    $f = Test-AppResourceFreshness -ExportDir $ExportDir -Since $Since
    $v = Get-PublishVerdict -Output $PublishOutput -TimedOut $PublishTimedOut
    if (-not $v.Ok) {
        # #1285：publish 没成立只改 ExitCode / Reason（仍 fail-closed：2 / publish-*），freshness **照实报实测量** ——
        # 旧写法在这里短路成 'not-measured'，把「导出已刷新但成功文案没读到」（门的采集层 bug ⇒ 查采集层 / 文案判据）
        # 与「根本没导出」（环境未就绪 ⇒ 先解决导出）糊成同一条红，而两者处置完全不同。
        # 量在本函数开头已做过（先量后判）；fresh = 目录其实已刷新、只是标记没读到 —— 正是要区分出来的那一支。
        return @{
            ExitCode = 2; Reason = "publish-$($v.Reason)"; Freshness = $f.Reason
            KtCount = $f.KtCount; Newest = $f.Newest
        }
    }
    if ($f.Fresh) {
        return @{ ExitCode = 0; Reason = 'ok'; Freshness = 'fresh'; KtCount = $f.KtCount; Newest = $f.Newest }
    }
    if ($f.Reason -eq 'no-kt') {
        return @{ ExitCode = 1; Reason = 'no-artifact'; Freshness = 'no-kt'; KtCount = 0; Newest = $null }
    }
    return @{ ExitCode = 1; Reason = 'stale-export'; Freshness = 'stale'; KtCount = $f.KtCount; Newest = $f.Newest }
}
