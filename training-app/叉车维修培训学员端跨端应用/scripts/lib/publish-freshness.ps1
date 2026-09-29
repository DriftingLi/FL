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
    是否逐字一致」。内容一致性由「publish 成立的证据 + 新鲜度」联合兜住；成立与否有两层证据（硬标记 /
    联合判据，见下面 `$PublishSuccessMarkers` 段），任一层都不成立、或新鲜度不成立，即判红。

    ⚠️ #1381 定性（本票标题写的「文案判据在当前 CLI 上失配」是**误诊**，别照着它改常量）：本机 2026-09-29
    用同一版 CLI（launcher 5.23 / 编译器 5.26）连跑三次，`导出 android 成功` **三次都命中**（主树两次、
    按闸门新建的 worktree 一次，原文已钉进守护测试的样本）。真实形态是**尾部记录随机丢一条**，
    丢哪条不固定 —— 所以「按当前输出重新校准某一条文案」修不好它（下一个丢的是另一条），
    能修的是**不再让单条记录成为必要条件**。
#>

# publish「成立」的判据 = 输出里的**正向标记**（不是退出码：HBuilderX CLI 退出码恒为 0）。
#
# 为什么不能只靠一条文案（#1381，2026-09-29 复现）：这条流是 CLI 从 HBuilderX 主程序**转发**过来的
# 合并日志，实测两件事（本机同版 CLI 连跑、原文见 utils/kotlinAllStaleExportBehavior.test.js 的样本钉）：
#   ① 记录之间**不保证有序**（同一次运行里时间戳 15:11:40.551 → .553 → .556 → .549）；
#   ② **尾部记录会丢**，且丢哪条不固定 —— 主树那次两条尾部记录都在，worktree 那次缺 `wgt文件由HBuilderX…`
#      提示，#1349 的 wt-1349 那次缺 `导出 android 成功`（而导出目录 mtime 照常刷新、cli exitcode 照常 0）。
# 把「唯一一条中文文案」当**充要**条件 ⇒ 采集面随机丢一条就让整扇门恒红，且每次跑 ④ 都要人重查一遍根因。
# （#1285 当年已把「导出已刷新但成功文案没读到」记成一种待分辨的处置方向，本票是它第一次真的发生。）
#
# 判据因此分两层，两层都是**正向**证据：
#   硬标记 `$PublishSuccessMarkers` 任一命中 ⇒ 直接成立（版本改了文案就往集合里加一条，不动逻辑）；
#   硬标记没读到 ⇒ 走联合判据 `Test-PublishCompoundEvidence` + 新鲜度实测量（见其说明）。
$PublishSuccessMarkers = @('导出 android 成功')

# 联合判据需要的两条**输出**证据；另外两条是**实测量**——「本次导出目录确实被重写」（新鲜度）与
# 「每个 .kt 都被重写」（完成度），都在 `Get-PublishStageVerdict` 里合。四条缺一仍判红。
# 为什么这四条合起来足以成立、且不会退回 #1272 的假绿、也不会放过截断导出（写实，别读成「已放宽」）：
#   - 「编译成功」+「正在导出」证明**这一次调用**真走完了编译并进入导出阶段（不是上一次留下的）；
#   - 「新鲜度 fresh」= 导出目录最新 `.kt` 的 mtime 晚于本次发第一条 CLI 命令**之前**取的时刻。
#     #1272 实测 publish 是**全量重写**（工作树一字未改再跑，119/119 个 `.kt` mtime 照样前进），
#     所以「磁盘上留着一份旧导出」这一形态必然判 stale ⇒ 被这条实测量拦住。
#   - 「完成度」= **最旧**的 `.kt` 也晚于基准（`Test-PublishExportCompleteness`）。这一条是评审补的：
#     硬标记本是「导出走到终点」的唯一信号，而联合分支只在它缺失时触发 ⇒ 只看「最新 mtime」会放过
#     「重写了一个文件就失败」的**截断导出**。全量重写 ⇒ 最旧的也前进；截断 ⇒ 有文件停在旧 mtime。
#   - 编译失败 ⇒ 无「编译成功」；没进导出 ⇒ 无「正在导出」；导出没写完 ⇒ 后两条实测量不成立。都判红。
#   - 命中 `$PublishFailurePattern` 时**不给它翻案**：失败签名优先（见 `Get-PublishStageVerdict`）。
$PublishCompileMarker = '编译成功'
$PublishExportStageMarker = '正在导出'
# publish「没成立」的判据：命中即判这一步没成立（**不**用来宣告「命令不存在」）。
$PublishFailurePattern = '与主程序的连接已中断|不存在或缺少参数|命令执行错误|启动超时'

<#
 .SYNOPSIS
     输出里是否出现**硬**成功标记（`$PublishSuccessMarkers` 任一）。
 .DESCRIPTION
     单独成函数（#1381）：门脚本在「要不要轮询导出目录」这个决策上也需要问一次正向证据，
     而 `Get-PublishVerdict` 的返回形状是裁决用的（Ok/Reason），不适合当布尔问句 —— 抄字面量又会漂。
#>
function Test-PublishSuccessMarker {
    param([string]$Output)
    foreach ($m in $PublishSuccessMarkers) {
        if ($Output -match [regex]::Escape($m)) { return $true }
    }
    return $false
}

<#
 .SYNOPSIS
     硬标记没读到时，输出里是否还齐着「这一步大概成立了」的两条**独立**证据（#1381）。
 .DESCRIPTION
     只管输出这一半；新鲜度那条实测量由调用方（`Get-PublishStageVerdict`）合上。
     拆成两半是为了让门脚本能拿它决定「要不要继续轮询导出目录」——轮询要的正是输出证据，
     而那一刻还不该拿新鲜度当准入条件（异步晚到的导出此时必然还没 fresh，拿它准入就等于永不轮询）。
#>
function Test-PublishCompoundEvidence {
    param([string]$Output)
    if ($Output -notmatch [regex]::Escape($PublishCompileMarker)) { return $false }
    if ($Output -notmatch [regex]::Escape($PublishExportStageMarker)) { return $false }
    return $true
}

<#
 .SYNOPSIS
     publish 步的输出 → 这一步是否成立。
 .OUTPUTS
     hashtable：`Ok`（bool）/ `Reason`（timeout | cli-ipc-blocked | cli-command-failed | no-success-marker | exported）
     —— 只认**硬标记**；联合判据在 `Get-PublishStageVerdict` 里合（那里才有新鲜度实测量）。
#>
function Get-PublishVerdict {
    param([string]$Output, [bool]$TimedOut)
    if ($TimedOut) { return @{ Ok = $false; Reason = 'timeout' } }
    if ($Output -match '与主程序的连接已中断') { return @{ Ok = $false; Reason = 'cli-ipc-blocked' } }
    if ($Output -match $PublishFailurePattern) { return @{ Ok = $false; Reason = 'cli-command-failed' } }
    if (Test-PublishSuccessMarker -Output $Output) { return @{ Ok = $true; Reason = 'exported' } }
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
     导出目录**每一个** `.kt` 的 mtime 是否都晚于基准（#1381 评审补的完成度代理）。
 .DESCRIPTION
     为什么联合判据还要这一条（评审发现，2026-09-29）：`Test-AppResourceFreshness` 只看**最新**那个
     `.kt`，于是「重写了一个文件就失败」的**截断导出**也算 fresh —— 而硬标记 `导出 android 成功` 原本
     正是「导出走到了终点」的**唯一**信号，联合分支偏偏只在它缺失时触发，所以这一格必须由别的东西补上，
     否则就是本次改动**新引入**的假绿面（不是 #1272 那条，那条被新鲜度拦住了）。
     判据的依据与新鲜度同源：#1272 实测 publish 是**全量重写**（119/119 个 `.kt` mtime 全部前进），
     故「最旧的也晚于基准」在全量重写成立、在截断不成立（截断 ⇒ 有文件停在旧 mtime）。
     **只加在联合分支**：硬标记命中时已经有终点信号，不需要代理 —— 拿它去收紧既有路径只会把
     「某文件本版确实没重写」这种未观测情形变成新的恒红。
#>
function Test-PublishExportCompleteness {
    param([string]$ExportDir, [datetime]$Since)
    $kt = @(Get-AppResourceKtFiles -ExportDir $ExportDir)
    if ($kt.Count -eq 0) { return $false }
    $oldest = ($kt | Sort-Object LastWriteTime | Select-Object -First 1).LastWriteTime
    return ($oldest -gt $Since)
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
       `Reason`（ok | ok-compound | stale-export | no-artifact | publish-timeout | publish-cli-ipc-blocked |
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
        # #1381：硬标记没读到、但四条正向证据齐 ⇒ 放行，并把结论**记成 ok-compound**（不是 ok）——
        # 「靠哪条证据过的」必须读得出来，否则下次排查又只能重新猜。四条：编译成功 + 已进入导出（两条输出证据）、
        # 本次导出目录确实被重写（新鲜度实测量）、且**每个** .kt 都被重写（完成度代理 —— 硬标记原本是「导出
        # 走到终点」的唯一信号，缺它时必须由完成度补上，否则截断导出会被放行；评审发现，见
        # `Test-PublishExportCompleteness`）。
        # 失败签名（timeout / ipc / cli-command-failed）到不了这一支：`Get-PublishVerdict` 先判它们，
        # reason 就不是 no-success-marker，联合判据**不给它翻案**。
        if ($v.Reason -eq 'no-success-marker' -and (Test-PublishCompoundEvidence -Output $PublishOutput) -and $f.Fresh -and (Test-PublishExportCompleteness -ExportDir $ExportDir -Since $Since)) {
            return @{ ExitCode = 0; Reason = 'ok-compound'; Freshness = 'fresh'; KtCount = $f.KtCount; Newest = $f.Newest }
        }
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
