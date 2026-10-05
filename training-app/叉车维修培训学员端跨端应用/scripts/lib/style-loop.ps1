<#
.SYNOPSIS
    样式内循环入口（`scripts/style-loop.ps1`，issue #1543）的**纯判定**：有没有活 / 阶段结论 /
    那一行机检判据 / 平台配置回写的还原。

.DESCRIPTION
    为什么单独放一个 lib（本仓既有纪律，见 `lib/hx-errors.ps1` 与 `lib/screenshot-gate.ps1` 头顶同一条理由）：
      入口是一个脚本 —— dot-source 它会整篇执行，没法单测其中某个分支。判定与文案上移到这里的纯函数后，
      `utils/styleLoopBehavior.test.js` 能用 pwsh **真跑**它们并断言**产物**（机检行的字节数与字段形状、
      退出码、还原后的文件字节），而不是断言入口脚本的源码文本（票面验收第 5 条，对齐 #1526 的口径）。

    四条硬口径：

      · **本入口不是门**：机检行恒带 `gate=none`。它不替代真机门 / 微信门 / 契约门 / 编译门，
        也不改任何门的触发面与归属 —— 门的结论只能由门脚本自己产出。这一条写进行内，
        防的是「把内循环的绿色读成门过了」（本仓反复出现的假绿形态换了个位置）。
      · **判不了 ⇒ 红**（沿用 `lib/screenshot-gate.ps1` 的纪律）：没有改动页面、截图没落盘、
        没有本轮截图可对比 —— 一律非零退出，绝不回退成「无变化」。
      · **机检行 ≤200 字节是承诺不是目标**（同 #1526 的「≤2KB 是承诺不是目标」）：装不下时按
        「页名清单收到 1 个 → 日志路径取末段 → 页名本身截断 → 整行硬裁剪」的阶梯收缩，
        每步重新量字节；结论字段排在尾部之前，所以硬裁剪先吃掉的是 pages/shots/reason/secs。
      · **日志全量落盘**：入口把子步骤的输出（含 Write-Host 走的 information stream）与 stderr
        一并接进 `.ci-verify/style-loop.log`，终端只留那一行 —— 会话侧默认只读那一行（票面第 3 条）。

    机检行的字段顺序是**刻意**的：`gate/verdict/phase/changed/shot/log` 排在 `pages/shots/reason/secs` 之前，
    所以任何极端裁剪（超长页面名、超长日志路径）先掉的是后一半，「结论 + 哪一页 + 产物在哪」活到最后。

    纯函数边界：`Get-StyleLoopWorkState` 只在**没给测试缝**时读 git；`Get-StyleLoopVerdict` /
    `Format-StyleLoopLine` 不碰文件系统；`Get-StyleLoopConfigFingerprint` / `Restore-StyleLoopConfig`
    只碰调用方给的那几个配置文件。⇒ 除配置那一对，全部可在任何机器上断言，不需要设备。
#>

# 「这一轮截哪些页」的判据**不在这里重写** —— 唯一真源仍是 `lib/auto-screenshot.ps1` 的
# `Invoke-AutoScreenshot`（它从 git diff 推导并按 pages.json 校验）。本模块只回答一个更便宜的问题：
# **要不要为一次 HBuilderX 启动付钱**（见 Get-StyleLoopWorkState）。

function Get-StyleLoopWorkState {
    <#
    .SYNOPSIS
        预检：改动集里到底有没有「样式内循环要看的活」。返回 @{ HasWork; PageFileCount; Source }。

    .DESCRIPTION
        判据是「`pages/` 目录下有没有改动文件」——**故意取得比 auto-screenshot 宽**（只看目录前缀，
        不看 `.uvue` 后缀、不查 pages.json）：它是那个权威集合的**超集**，所以
        「推导能找到页 ⇒ 这里必不为空」。取窄了就会在最坏的方向上出错：**谎称没活、把人挡在门外**
        （白跑一轮是成本，被一句 `no-work` 劝退是损失）。真出现「这里有数、那边推不出页」时，
        入口会走到截图那一步、由 auto-screenshot 明报「无改动页面」⇒ 结论仍然红，只是多付一次启动。

        git 失败（不在仓库里 / git 不可用）⇒ **fail-open**（HasWork=$true）：预检没有判红权，
        判红权在 auto-screenshot 的推导与像素层。

    .PARAMETER ChangedFiles
        测试缝：直接给「项目根相对」的改动文件清单时**不碰 git**（守护靠它成对喂数）。

    .PARAMETER PagesExplicit
        人已用 `-Pages` 显式指定页面 ⇒ 有活，不必看改动集（首轮建基线正是这种形态）。
    #>
    [CmdletBinding()]
    param(
        [string]$ProjectDir,
        [string[]]$ChangedFiles,
        [switch]$PagesExplicit
    )

    if ($PagesExplicit) {
        return [pscustomobject]@{ HasWork = $true; PageFileCount = -1; Source = 'explicit-pages' }
    }

    $files = @()
    if ($PSBoundParameters.ContainsKey('ChangedFiles')) {
        $files = @($ChangedFiles | Where-Object { $_ -and "$_".Trim() } | ForEach-Object { "$_".Trim() })
    }
    else {
        if (-not $ProjectDir) {
            $ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
        }
        # 与 `lib/auto-screenshot.ps1` 的两条调用**同形**：`--relative`（本项目挂在
        # `training-app/<中文目录>/` 下，仓库根相对路径前面那串非 ASCII 段会让 `^pages/` 锚定恒失配）
        # + 必须过解码器（core.quotepath 把非 ASCII 路径整条加引号 + \ooo 转义 ⇒ 末尾不是 `uvue`）。
        # 解码器的唯一真源在 level-detect.ps1；本模块被独立 dot-source 时补加载（同 auto-screenshot 的懒加载写法）。
        if (-not (Get-Command ConvertFrom-GitQuotedPath -ErrorAction SilentlyContinue)) {
            . (Join-Path $PSScriptRoot 'level-detect.ps1')
        }
        foreach ($gitArgs in @(
                @('diff', '--name-only', '--relative', 'HEAD'),
                @('diff', '--name-only', '--relative', 'origin/master...HEAD'))) {
            try {
                $raw = & git -C $ProjectDir @gitArgs 2>$null
                if ($raw) {
                    $files += @($raw | Where-Object { $_ -and "$_".Trim() } |
                        ForEach-Object { ConvertFrom-GitQuotedPath "$_".Trim() })
                }
            }
            catch {
                return [pscustomobject]@{ HasWork = $true; PageFileCount = -1; Source = 'git-failed' }
            }
        }
        $files = @($files | Select-Object -Unique)
    }

    $underPages = @($files | Where-Object { $_ -match '^pages/' })
    return [pscustomobject]@{
        HasWork       = ($underPages.Count -gt 0)
        PageFileCount = $underPages.Count
        Source        = 'git-diff'
    }
}

function Get-StyleLoopVerdict {
    <#
    .SYNOPSIS
        由各阶段的**结果对象**算出内循环的结论（纯函数）。优先级即阶段顺序，先失败者定相。

    .DESCRIPTION
        优先级：预检（有没有活）→ 环境 → 编译期诊断 → 部署 → 截图 → 像素层。
        为什么编译诊断在装机之前判：入口的动作顺序是「仅编译拿诊断（不碰设备）⇒ 真运行 ⇒ 逐页截图」。
        编译没过就没必要装机，更没必要截图 —— **编译失败的机器上截到的还是上一次构建的页面**，
        拿它做像素比会把「没生效」读成「没变化」（本票要消灭的那类空转）。

        退出码沿用本仓既有语义（`hx-run.ps1` / `dev-finish.ps1`）：
          0 = 绿（本轮确实比过且无变化）
          1 = 红（判据判出的问题：没活 / 有编译诊断 / 截图无效 / 像素差超阈值）
          2 = **环境不可用**（设备或 HBuilderX 取不到、没部署成功）—— 与「代码有问题」分开，
              因为它的出路是「换环境再看」，不是「改代码」。
    #>
    [CmdletBinding()]
    param(
        [bool]$HasWork = $true,
        [string]$EnvError = '',
        # 编译阶段的结论与退出码：$null = 没走到这一步（预检或环境就收了）。
        # 0 干净 / 1 有诊断行 / 2 环境不可用（取自 lib/test-compile.ps1 新增的 ExitCode 字段）
        [Nullable[bool]]$CompileOk = $null,
        [Nullable[int]]$CompileExit = $null,
        [bool]$Deployed = $false,
        [string]$DeployError = '',
        [string]$ShotError = '',
        [int]$PageCount = 0,
        [int]$ShotCount = 0,
        [int]$ChangedCount = 0,
        [string[]]$ChangedPages = @(),
        # 像素层结论 = lib/screenshot-gate.ps1 的 Get-PngDiffVerdict 返回对象；$null = 没走到像素层
        $DiffVerdict = $null
    )

    $verdict = 'green'; $phase = 'done'; $exit = 0; $reason = 'clean'

    if (-not $HasWork) {
        $verdict = 'red'; $phase = 'precheck'; $exit = 1; $reason = 'no-work'
    }
    elseif (-not [string]::IsNullOrWhiteSpace($EnvError)) {
        $verdict = 'env'; $phase = 'env'; $exit = 2; $reason = 'env-unavailable'
    }
    elseif ($null -ne $CompileOk -and -not [bool]$CompileOk) {
        # 判红权在 CompileOk（test-compile 的 fail-closed 结论），退出码只用来**分方向**：
        # 2 ⇒ 环境不可用（出路是换环境再看），其余 ⇒ 编译期有诊断（出路是回去改代码）。
        # 拿退出码当唯一判据会漏掉「Ok=false 但码是 0」这一形（陈旧 $LASTEXITCODE 的历史坑）。
        if ($null -ne $CompileExit -and [int]$CompileExit -eq 2) {
            $verdict = 'env'; $phase = 'compile'; $exit = 2; $reason = 'hx-env-unavailable'
        }
        else {
            $verdict = 'red'; $phase = 'compile'; $exit = 1; $reason = 'compile-diagnostic'
        }
    }
    elseif (-not $Deployed) {
        $verdict = 'env'; $phase = 'deploy'; $exit = 2; $reason = 'not-deployed'
    }
    elseif (-not [string]::IsNullOrWhiteSpace($ShotError)) {
        $verdict = 'red'; $phase = 'shot'; $exit = 1; $reason = 'screenshot-failed'
    }
    elseif ($null -eq $DiffVerdict) {
        # 走到了截图却没拿到像素层结论 ⇒ 判不了 ⇒ 红（不许把「判不了」当「无变化」）
        $verdict = 'red'; $phase = 'pixel'; $exit = 1; $reason = 'no-diff-verdict'
    }
    elseif (-not [bool]$DiffVerdict.Ok) {
        $verdict = 'red'; $phase = 'pixel'; $exit = [int]$DiffVerdict.ExitCode
        if ($exit -eq 0) { $exit = 1 }   # fail-closed：判定说不通过却给 0 ⇒ 按红处理
        $reason = 'pixel-changed'
    }
    else {
        switch ([string]$DiffVerdict.Action) {
            'write-baseline'   { $phase = 'baseline-written';   $reason = 'first-baseline' }
            'refresh-baseline' { $phase = 'baseline-refreshed'; $reason = 'baseline-refreshed' }
        }
    }

    return [pscustomobject]@{
        Verdict      = $verdict
        Phase        = $phase
        ExitCode     = $exit
        Reason       = $reason
        Pages        = $PageCount
        Shots        = $ShotCount
        Changed      = $ChangedCount
        ChangedPages = @($ChangedPages)
    }
}

function Format-StyleLoopLine {
    <#
    .SYNOPSIS
        把结论对象渲染成**那一行**机检判据。

    .OUTPUTS
        [pscustomobject]：
          Line       那一行（单行 ASCII）
          Bytes      UTF-8 字节数（判据用字节，不用字符数 —— 预算的意义就在于此）
          ShotNames  行内点名的页面（可能为空：仅整行硬裁剪那一步）
          ShotHidden 未列出的页数（`+N` 里那个 N）
          Ladder     用到阶梯的哪一档：full | single | leaf-log | clipped-name | hard-trim

    .DESCRIPTION
        三条形状承诺，全部可被守护真跑断言：

          ① **字节数 ≤ MaxBytes（默认 200）**：按 `页名清单收到 1 个 → 日志路径取末段 →
             页名本身截断（≥8 字符）→ 整行硬裁剪` 的阶梯，每步重新量字节。
          ② **恒为单行 ASCII**：非 ASCII 与非可打印字符替换成 `?`。理由见 #1542 缺陷 5 ——
             中文行在包装链上会按 GBK 解释成 mojibake；机检判据必须能在任何码页下原样 grep 到。
          ③ **判红时至少点一页**：`shot=` 至少含一个页面名（必要时截断它，也不换成 `-`）——
             票面第 2 条要的就是「机检行指出是哪一页变的」。绿色或未走到像素层时 `shot=-`。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]$Verdict,
        [string]$LogPath = '.ci-verify/style-loop.log',
        [int]$Seconds = 0,
        [ValidateRange(40, 4096)]
        [int]$MaxBytes = 200
    )

    $utf8 = [System.Text.Encoding]::UTF8

    # 局部脚本块而不是 `function`：不留第二个顶层名字，也不会被 AST 抽函数定义时漏掉（#1526 同款写法）
    $ascii = {
        param($Text)
        "$Text" -replace '[^\x20-\x7E]', '?'
    }

    $render = {
        param($V, $Secs, $ShotValue, $Log)
        @(
            'STYLE_LOOP'
            'gate=none'
            "verdict=$([string]$V.Verdict)"
            "phase=$([string]$V.Phase)"
            "changed=$([int]$V.Changed)"
            "shot=$ShotValue"
            "log=$Log"
            "pages=$([int]$V.Pages)"
            "shots=$([int]$V.Shots)"
            "reason=$([string]$V.Reason)"
            "secs=$([int]$Secs)"
        ) -join ' '
    }

    $v = [pscustomobject]@{
        Verdict  = [string]$Verdict.Verdict
        Phase    = [string]$Verdict.Phase
        Changed  = [int]$Verdict.Changed
        Pages    = [int]$Verdict.Pages
        Shots    = [int]$Verdict.Shots
        Reason   = [string]$Verdict.Reason
    }

    $names = @(@($Verdict.ChangedPages) | Where-Object { $_ } | ForEach-Object { & $ascii $_ })
    $fullLog = & $ascii $LogPath
    $leafLog = & $ascii (Split-Path -Leaf $LogPath)
    if (-not $leafLog) { $leafLog = $fullLog }

    $shotValue = {
        param($List, $Hidden)
        if (@($List).Count -eq 0) { return '-' }
        $joined = (@($List) -join ',')
        if ([int]$Hidden -gt 0) { return "${joined}+${Hidden}" }
        return $joined
    }

    # ---- 收缩阶梯 ----
    $chosen = $null
    foreach ($rung in @(@{ Names = 3; Leaf = $false; Tag = 'full' }, @{ Names = 1; Leaf = $false; Tag = 'single' }, @{ Names = 1; Leaf = $true; Tag = 'leaf-log' })) {
        $shown = @($names | Select-Object -First ([int]$rung.Names))
        $hidden = [Math]::Max(0, @($names).Count - @($shown).Count)
        $log = $fullLog
        if ($rung.Leaf) { $log = $leafLog }
        $cand = & $render $v $Seconds (& $shotValue $shown $hidden) $log
        if ($utf8.GetByteCount($cand) -le $MaxBytes) {
            $chosen = [pscustomobject]@{
                Line = $cand; Shown = $shown; Hidden = $hidden; Log = $log; Ladder = $rung.Tag
            }
            break
        }
    }

    if (-not $chosen -and @($names).Count -gt 0) {
        # 页名本身截断，但**不换成 `-`**：判红了说不出哪一页 = 票面第 2 条没做到
        $name0 = [string]$names[0]
        $hidden = [Math]::Max(0, @($names).Count - 1)
        for ($len = $name0.Length; $len -ge 8; $len--) {
            $val = "$($name0.Substring(0, $len))~$(if ($hidden -gt 0) { "+$hidden" } else { '' })"
            $cand = & $render $v $Seconds $val $leafLog
            if ($utf8.GetByteCount($cand) -le $MaxBytes) {
                $chosen = [pscustomobject]@{
                    Line = $cand; Shown = @($name0.Substring(0, $len)); Hidden = $hidden
                    Log = $leafLog; Ladder = 'clipped-name'
                }
                break
            }
        }
    }

    if (-not $chosen) {
        # 极端兜底（预算小到 40 字节这类）：整行按字节硬裁剪。结论字段在前 ⇒ 砍的是尾部，
        # 且裁剪必以 `~` 结尾，让人一眼看出这行不完整，而不是读到一个恰好短的结论。
        $cand = & $render $v $Seconds (& $shotValue $names 0) $leafLog
        $chars = "$cand".ToCharArray()
        $cut = $cand
        for ($n = $chars.Length; $n -gt 0; $n--) {
            $t = -join $chars[0..($n - 1)]
            if ($utf8.GetByteCount($t) -le ($MaxBytes - 1)) { $cut = "$t~"; break }
        }
        $chosen = [pscustomobject]@{
            Line = $cut; Shown = @(); Hidden = @($names).Count
            Log = $leafLog; Ladder = 'hard-trim'
        }
    }

    return [pscustomobject]@{
        Line       = $chosen.Line
        Bytes      = $utf8.GetByteCount($chosen.Line)
        ShotNames  = @($chosen.Shown)
        ShotHidden = [int]$chosen.Hidden
        LogField   = $chosen.Log
        Ladder     = [string]$chosen.Ladder
    }
}

function Resolve-StyleLoopChildOutput {
    <#
    .SYNOPSIS
        包装层（父进程）把子进程的**全部** stdout 换成「日志正文 + 那一行机检判据」。

    .OUTPUTS
        [pscustomobject]：
          Line       要打到终端的那一行（一定非空 —— 见下面的 fail-closed）
          FromChild  这行是不是子进程自己算出来的
          ExitCode   子进程的退出码；子进程静默却报 0 ⇒ 强制改 1
          Body       子进程的完整输出（进日志，不进终端）

    .DESCRIPTION
        为什么判据要写成纯函数而不是内联在包装层：包装层唯一的职责就是「这一行从哪来、
        拿什么码退出」，那正是本入口的承诺（红则非零、绿则零、永远一行）。写成纯函数
        `utils/styleLoopBehavior.test.js` 才能成对真跑（有行⇒原样 relay / 无行⇒合成且判红）。

        三条 fail-closed：
          · 子进程**没**打出 `STYLE_LOOP` 行（崩了、被 kill、在 `Wait-HxFree` 里 `exit 2` 走了）
            ⇒ 合成一行 `verdict=env phase=child reason=child-silent|child-exited-<码>`，
              绝不退化成「终端一个字都没有、退出码 0」——那正是本仓反复出现的假绿形态。
          · 子进程退出码 0 却没有行 ⇒ 退出码改判 **1**（自称成功却拿不出结论，按红处理）。
          · 只认**第一行** `STYLE_LOOP`：子进程若打了两行，第二行只会进日志 ⇒ 结论取第一条，
            不去猜哪条更新（第二真源的形态就是这么开始的）。
    #>
    [CmdletBinding()]
    param(
        [AllowEmptyString()][AllowNull()][string]$Captured,
        [Parameter(Mandatory = $true)][int]$ChildExitCode,
        [string]$LogPath = '.ci-verify/style-loop.log',
        [int]$Seconds = 0,
        [ValidateRange(40, 4096)]
        [int]$MaxBytes = 200
    )

    $body = "$Captured" -replace "`r`n", "`n"
    $lines = @($body -split "`n" | Where-Object { $_ -ne $null })
    $found = @($lines | Where-Object { $_ -match '^STYLE_LOOP\s' } | Select-Object -First 1)

    if (@($found).Count -gt 0) {
        return [pscustomobject]@{
            Line = [string]$found[0]; FromChild = $true; ExitCode = $ChildExitCode; Body = $body
        }
    }

    $verdict = 'env'
    $exit = $ChildExitCode
    $reason = "child-exited-$ChildExitCode"
    if ($ChildExitCode -eq 0) {
        # 自称成功却拿不出结论 ⇒ 判红（不是 env：env 的出路是换环境，这条的出路是查包装链）
        $verdict = 'red'; $exit = 1; $reason = 'child-silent'
    }
    $v = [pscustomobject]@{
        Verdict = $verdict; Phase = 'child'; Changed = 0; Pages = 0; Shots = 0
        Reason  = $reason;  ChangedPages = @()
    }
    $fmt = Format-StyleLoopLine -Verdict $v -LogPath $LogPath -Seconds $Seconds -MaxBytes $MaxBytes
    return [pscustomobject]@{
        Line = [string]$fmt.Line; FromChild = $false; ExitCode = $exit; Body = $body
    }
}

function Copy-StyleLoopBaselineShot {
    <#
    .SYNOPSIS
        首次运行（基线为空）时把**本轮**截图写进基线目录。返回 @{ Copied; Skipped; SourceExists; BaselineDir }。

    .DESCRIPTION
        为什么入口必须自己做这一步：`lib/screenshot-gate.ps1` 的判定表里 `write-baseline` 那一支只说
        「自动写基线」，动作却归调用方执行（`dev-finish.ps1` 步骤 7 就自己拷了一遍）。入口若只转述结论、
        不执行动作，第一轮的绿就是空的 —— 基线目录永远没有图，下一轮还是「首次运行」，判据永远不咬
        （ADR-0008:419 记的那个死循环）。所以这一步不是便利，是那条死循环的封口。

        与 `dev-finish` 那一份拷贝的一处**刻意**差别：这里按 `RunStartedAt` 过滤，只拷本轮截图。
        `.ci-verify/screenshots/` 从不清理 ⇒ 上一轮失败运行残留的同名图被拷进基线，就是把坏画面钉成参考图
        （#1158 记的那半岛，判据与 `Select-ThisRunShots` 同源，不另写一份时间戳规则）。
        不传 `RunStartedAt` ⇒ 全拷（向后兼容旧形态）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [Nullable[datetime]]$RunStartedAt = $null,
        [string]$SourceDir,
        [string]$BaselineDir
    )

    if (-not $SourceDir) { $SourceDir = Join-Path $ProjectDir '.ci-verify/screenshots' }
    if (-not $BaselineDir) { $BaselineDir = Join-Path $ProjectDir '.ci-verify/baseline' }

    $copied = @()
    $skipped = @()
    $sourceExists = Test-Path -LiteralPath $SourceDir -PathType Container
    if (-not $sourceExists) {
        return [pscustomobject]@{ Copied = $copied; Skipped = $skipped; SourceExists = $false; BaselineDir = $BaselineDir }
    }

    $null = New-Item -ItemType Directory -Force -Path $BaselineDir
    foreach ($f in @(Get-ChildItem -Path $SourceDir -Filter '*.png' -File)) {
        if ($null -ne $RunStartedAt -and $f.LastWriteTime -lt [datetime]$RunStartedAt) {
            $skipped += $f.Name
            continue
        }
        Copy-Item -LiteralPath $f.FullName -Destination (Join-Path $BaselineDir $f.Name) -Force
        $copied += $f.Name
    }

    return [pscustomobject]@{
        Copied       = @($copied)
        Skipped      = @($skipped)
        SourceExists = $true
        BaselineDir  = $BaselineDir
    }
}

function Get-StyleLoopConfigFingerprint {
    <#
    .SYNOPSIS
        给会被平台回写的配置文件拍**字节**快照（票面第 6 条：跑完不把工作树弄脏）。

    .DESCRIPTION
        为什么入口还要自己拍一遍：`lib/build-deploy.ps1` 已经还原 manifest.json / pages.json，
        但它比的是 `Get-Content -Raw` 的**文本**、还原用 `Set-Content -Encoding utf8` ⇒
        BOM 或行尾可能与原文件不同，于是「文本判据说没变化」而 git 仍看得到改动。
        本快照存**字节**、还原写**字节**，是那条还原的兜底；并把 `platformConfig.json` 纳入
        （它是第三份运行时面判据来源，误留在改动集里会让 PR 凭空命中打包面门）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$ProjectDir,
        [string[]]$Paths = @('manifest.json', 'pages.json', 'platformConfig.json')
    )

    $hash = @{}
    $bytesByPath = @{}
    foreach ($rel in @($Paths)) {
        $full = Join-Path $ProjectDir $rel
        if (Test-Path -LiteralPath $full -PathType Leaf) {
            $b = [System.IO.File]::ReadAllBytes($full)
            $sha = [System.Security.Cryptography.SHA256]::Create().ComputeHash($b)
            $hash[$rel] = (([BitConverter]::ToString($sha)) -replace '-', '')
            $bytesByPath[$rel] = $b
        }
        else {
            $hash[$rel] = $null
        }
    }
    return [pscustomobject]@{
        ProjectDir  = $ProjectDir
        Paths       = @($Paths)
        Hash        = $hash
        BytesByPath = $bytesByPath
    }
}

function Restore-StyleLoopConfig {
    <#
    .SYNOPSIS
        把快照之后被改动的配置文件写回原字节。返回 @{ Restored; Appeared; Unchanged }（相对路径数组）。

    .DESCRIPTION
        三个方向分开报，因为处置不同：
          · `Restored` —— 原本存在、现在字节不同 ⇒ 写回原字节（票面第 6 条要的动作）。
          · `Appeared` —— 原本不存在、现在多了这个文件 ⇒ **只报不删**：删别人的产物是不可逆动作，
            而入口无权判断它是平台回写还是人刚建的。
          · `Unchanged` —— 原本存在且字节未变 ⇒ 什么都不做。这条单独存在的意义是让「什么都没还原」
            能被守护**证实**，而不是靠「Restored 为空」反推（空数组也可能是快照本身没拍到）。
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]$Fingerprint,
        [switch]$DryRun
    )

    $restored = @()
    $appeared = @()
    $unchanged = @()
    $sha = [System.Security.Cryptography.SHA256]::Create()

    foreach ($rel in @($Fingerprint.Paths)) {
        $full = Join-Path $Fingerprint.ProjectDir $rel
        $before = $Fingerprint.Hash[$rel]
        $exists = Test-Path -LiteralPath $full -PathType Leaf

        if ($null -eq $before) {
            if ($exists) { $appeared += $rel }
            continue
        }
        if (-not $exists) {
            # 快照里有、现在文件没了 —— 不是本入口的动作（它从不删配置），照实报，不擅自重建
            $appeared += "$rel (deleted)"
            continue
        }

        $b = [System.IO.File]::ReadAllBytes($full)
        $now = (([BitConverter]::ToString($sha.ComputeHash($b))) -replace '-', '')
        if ($now -eq $before) { $unchanged += $rel; continue }

        if (-not $DryRun) {
            [System.IO.File]::WriteAllBytes($full, $Fingerprint.BytesByPath[$rel])
        }
        $restored += $rel
    }

    return [pscustomobject]@{
        Restored  = @($restored)
        Appeared  = @($appeared)
        Unchanged = @($unchanged)
    }
}
