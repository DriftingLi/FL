<#
.SYNOPSIS
    一键完成：编译 → 部署 → 截图 → 对比 → 生成证据。

.DESCRIPTION
    按改动复杂度自动判定级别（🟢快速 / 🟡标准 / 🔴完整），执行对应流程：

      🟢 快速（默认 **Q-A 静态守护**）：契约测试已跑；**秒级**、不取锁、不占设备、**未做编译诊断**
      🟢 快速 `-Compile`（**Q-B 编译诊断**）：追加 `hx-run -CompileOnly`；**热缓存 257 秒**，
         **冷 / 上次被打断后 826–901+ 秒**（2026-09-14 两次实测，48 页工程）
         —— **勿再引用旧口径「约 3 分钟起 / 冷 >15 分钟」**，两者都已被实测推翻
      🟡 标准：④c 编译门 + 真运行到设备 + 只截改动页 + 简要证据
      🔴 完整：④c 编译门 + 真运行到设备 + 只截改动页 + 完整证据

    **收口提示（ADR-0016 ①b，2026-09-16）**：摘要前会打「本批触及能力面：是 / 否」+ 原生能力调用点扫描。
    口径真源是 `lib/capability-surface.ps1` 的**路径白名单**（指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互）；
    **只做提示、不判红** —— 不进 `pr-evidence` 的判红路径、不影响本脚本退出码。
    命中 ⇒ ①b 必做且由**人**给出原文；未命中 ⇒ 提示行不新增任何必做项（普通运行时面改动不会被这行字加要求）。

    **Q-2 修正（2026-09-14）**：原本让 🟢 默认跑 `hx-run -CompileOnly` 当「快速」，但**实测推翻了前提** ——
    冷 / 失效缓存下 compile-only **>901 秒**，**比真运行（4–5 分钟）还慢**。故 🟢 默认降级为 **Q-A 静态守护**
    （秒级、不取锁、不占设备），编译诊断改为**显式 `-Compile`**。要看真机效果请用 🟡，或用 HBuilderX GUI 热刷新（秒级）。
    **Q-A 必须明说「未做编译诊断」** —— 静默跳过正是最早那版「什么都没做却报成功」的假绿。

    **A-3 临界区**：HBuilderX 是**单实例串行资源**，而本脚本有**三个**步骤需要它（编译 / 部署 / 截图），
    且后两步**共享设备状态**。若各步各拿一次锁，别的会话可能在「部署到目标页」与「截图」之间插进来
    launch 到别的页 ⇒ **截到别人的页面**。故改为**一个临界区覆盖步骤 4–6**。
    **Q-A 根本不进临界区**（它不使用主程序）—— 否则别的会话持锁时，秒级路径会白等到超时。

    **锁交接**：`hx-busy.ps1` 的锁**按 PID 判定且不可重入** ⇒ 父进程持锁后再调子脚本（子进程 PID 不同）
    会**自死锁**。故持锁后调 `Set-HxLockOwnerEnv` 写 `$env:HX_LOCK_OWNER`；子脚本的**取锁函数**
    认到「同一持有者」就复用、不再加锁。失败/超时路径经 `finally` 释放锁并清理 env。

    **并发会话会排队**（不是冲突）：等待上限 `-HxWaitSeconds`（默认 1800 秒，因临界区较长），
    超时 ⇒ `exit 2`（环境不可用），**绝不抢占、绝不 kill 主程序**。真机步骤请**集中到一个会话**执行。

.EXAMPLE
    npm run dev:finish
    npm run dev:finish -- -Level standard -Device 192.168.10.51:39181
    npm run dev:finish -- -DryRun
    npm run test:unit:gate                 # 只跑全量单测门：日志 + （红跑）摘要 + 机检行
    pwsh scripts/dev-finish.ps1 -UnitGate -PostToPr 1546   # 同上，并把结果贴成 sha 绑定评论
#>
[CmdletBinding()]
param(
    [string]$Device,
    [ValidateSet('quick', 'standard', 'full', '')]
    [string]$Level = '',
    [switch]$DryRun,
    # ---- 全量单测门落盘模式（#1546，2026-10-05）----
    # 只跑「全量单测门」这一件事：完整输出恒定落盘、红跑另产一份 ≤2KB 人读摘要、打一行机检行，
    # 然后**按 jest 的退出码原样退出**。不进下面的九步管线、不取 HBuilderX 锁、不碰设备。
    # 存在的理由：#1526 只把「单元测试（契约子集）」那一步的失败详情留住了；全量那一条至今没有落盘
    # 脚本，跑完只剩 exit code ⇒ 「刚才哪个红了」只能重跑几分钟（票 #1546 打的就是 C1 那一档）。
    [switch]$UnitGate,
    # 与 ②④ 两条门脚本同名同义：把本次结果贴成 sha 绑定的 PR 评论（0 = 不贴）。
    [int]$PostToPr = 0,
    [switch]$Distribute,
    [switch]$UpdateBaseline,
    # ---- Q17（2026-09-18，#1139）：步骤 7 的像素判据参数 ----
    # 截图噪声（字体栅格化 / 抗锯齿 / 状态栏读数）是**必然**的 ⇒ 无阈值等于每页恒红。
    # 0..1 之外没有意义 ⇒ 参数级拒绝（不静默钳位）。
    [ValidateRange(0.0, 1.0)]
    [double]$PixelThreshold = 0.005,
    # 忽略顶部/底部行数：真机建议取状态栏高度（时钟 / 网速读数是实时变化的）。
    [ValidateRange(0, 10000)]
    [int]$IgnoreTopRows = 0,
    [ValidateRange(0, 10000)]
    [int]$IgnoreBottomRows = 0,
    # Q-2 修正（2026-09-14 用户裁定）：🟢 quick **默认只做静态守护（Q-A）**，秒级、不占设备、不取锁。
    # 需要编译期诊断时显式加本开关 ⇒ Q-B（hx-run -CompileOnly；冷/失效缓存下实测 >901 秒，成本写实）。
    [switch]$Compile,
    [int]$MaxScreenshotPages = 5,
    [int]$HxWaitSeconds = 1800,
    # 透传给 hx-run 的超时（编译段 / 部署轮询）。2026-09-14 实测本项目冷缓存下
    # 编译可超 900 秒 ⇒ 默认取 1800，避免把「慢」误判成「环境不可用」。
    [int]$HxRunTimeoutSeconds = 1800
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# 子进程输出解码用 UTF-8（本仓先例：frontier-run.ps1 / wt-bootstrap.ps1 / mp-weixin-check.ps1）。
# 本机 `[Console]::OutputEncoding` 默认是 **gb2312** ⇒ `& npx jest` 的 stdout 在**进入内存那一刻**
# 就被按 GBK 解码：中文用例名与 `●` 标记变成乱码（#1526 实测：同一行原始字节 E2 97 8F 被解成
# 「鈻犫暈」形态）。若不在这里修，失败摘要会把「看不见红」换成「看得见但读不懂」——比不写更坏。
# 非控制台环境（CI、被重定向）设置可能抛 ⇒ try/catch 吞掉，行为退回改前。
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

$ProjectDir = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$started = Get-Date

# ============================================================
# 辅助
# ============================================================
function Write-Step {
    param([int]$Num, [int]$Total, [string]$Label)
    Write-Host ""
    Write-Host "[$Num/$Total] $Label" -ForegroundColor Cyan
}

function Write-Result {
    param([bool]$Ok, [string]$Msg)
    if ($Ok) { Write-Host "  ✅ $Msg" -ForegroundColor Green }
    else { Write-Host "  ❌ $Msg" -ForegroundColor Red }
}

<#
.SYNOPSIS
    把已经跑完的 jest 失败输出裁剪成 ≤$MaxBytes 字节的**人读**摘要并落盘（#1526，2026-10-04）。

.DESCRIPTION
    存在理由：单元测试这一步失败时，脚本只打一行「单元测试失败（exit code 1）」就 `exit 1`，
    而那一份完整输出**只活在内存里、随进程蒸发** ⇒ 想知道「哪一条红、为什么红」只能**重跑**
    jest（#1472 先例：为看失败原因重跑六遍，合计 2,140.466 秒）。本函数把那一份已经拿到的输出
    留下四件东西：**失败套件**、**失败用例名 + 其后的判据行**（为什么红）、**套件级说明**
    （`● Console` 日志与 `● Test suite failed to run` —— 它们不是失败用例）、**`Tests:` / `Time:` 汇总行**。

    四条硬口径：
      · **绿跑不落盘** —— `$ExitCode -eq 0` 直接返回 `$null`，不碰文件系统。一份「看起来像证据」
        的绿跑镜像文件正是本仓防的形态（永绿 / 假绿）。
      · **人读出口，不作门禁输入** —— 调用方的判定仍然只看 exit code；本文件不进任何判红路径。
      · **≤2KB 是承诺不是目标** —— 超预算就按「套件级说明 → 失败用例 → 失败套件」整条丢弃并
        **明说各丢了几条**（汇总段不丢）；连表头都装不下时按字符整体硬裁剪。绝不静默截半条多字节序列。
      · **对重复块幂等** —— 同一条 `FAIL` 行 / 同一个用例块在一次单元测试的输出里可能出现两遍
        （本机实测：`.scratch/1526/full-raw.txt` 第 15 与 189 行是同一条 FAIL 行，`● ` 块各两次，
        而 `Tests:` 只有一遍）。不去重就会写「共 2 个失败套件」，与同份输出里的 `Tests: 2 failed`
        自相矛盾 —— 摘要自己打自己脸，比不落盘更坏。

    纯函数边界：唯一副作用是写 `$Path` 那一个文件（父目录按需创建）；不读仓内源码、不取锁、不碰设备
    ⇒ 可被 `utils/devFinishContract.test.js` 用 AST 抽函数定义后**真执行**断言产物（先例
    `scripts/lib/process-capture.ps1` + `utils/hxLaunchDetachBehavior.test.js`）。
#>
function New-TestFailureSummary {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][AllowNull()][AllowEmptyString()][string]$Output,
        [Parameter(Mandatory = $true)][string]$Path,
        [int]$ExitCode = 1,
        [int]$MaxBytes = 2048,
        [int]$MaxEntries = 30,
        [int]$DetailLines = 2
    )
    if ($ExitCode -eq 0) { return $null }

    $utf8 = [System.Text.Encoding]::UTF8
    $lines = @($Output -split "\r?\n")

    # jest 的四行汇总（`Test Suites:` / `Tests:` / `Snapshots:` / `Time:`）—— 总量面
    $summary = @($lines | Where-Object { $_ -match '^\s*(?:Test Suites?|Tests|Snapshots|Time):\s' })

    # 失败套件行：`FAIL utils/x.test.js (7.421 s)`。**必须整行去重** ——
    # 本机实测（`.scratch/1526/full-raw.txt`，sha256=825BFA0A…4940，244 行；该探针产物不入仓，
    # 全文随 PR #1526 附上）：多套件的一次「单元测试」里，
    # 同一个失败套件的**整块详情出现两遍**（同一条 `FAIL` 行在第 15 行和第 189 行，`● ` 用例块各两次），
    # 而 `Tests:` 汇总行只有一遍 ⇒ 不去重就会写「共 2 个失败套件 / 5 条失败用例」，与同份输出里的
    # `Tests: 2 failed` 自相矛盾 —— 本票要消灭的是「失败原因不可得」，不是把它换成「读得出但读错」。
    # 计数一律给**去重后**的唯一数（`suiteCount`），被并掉的行数另报（`dupSuites`），不静默吞掉。
    # ⚠️ 本函数的注释里**别写**「Step + 步骤号」那种字面量（要指代就写步骤名「单元测试」）：
    #    守护 F2 判的是「本文件里每个步骤串的首次出现位置单调递增」，函数文档里多写一次步骤三
    #    就会让 F2 报红（2026-10-04 实测踩过），而那红报的是「步骤顺序错了」——一个根本没发生的改动。
    $seenSuite = [System.Collections.Generic.HashSet[string]]::new()
    $suites = [System.Collections.Generic.List[string]]::new()
    $dupSuites = 0
    foreach ($l in @($lines | Where-Object { $_ -match '(?:^|\s)FAIL\s+\S+\.test\.' })) {
        $k = "$($l)".Trim()
        if ($seenSuite.Add($k)) { $null = $suites.Add($k) } else { $dupSuites++ }
    }

    # 失败用例条目：`● 套件 › 用例名`（默认 reporter）/ `✕ 用例名`（verbose）+ 其后判据行。
    # ⚠️ 字形一律用 \uXXXX 转义写：源码里放裸字形会被编辑工具/编码链吃掉（本仓实测踩过）。
    # ⚠️ `● Console` 与 `● Test suite failed to run` **不是失败用例**：前者是套件打印的日志
    #    （实测那份输出里它挂在 `PASS utils/concurrent401RefreshBehavior.test.js` 之后 —— 通过的套件也会有），
    #    后者是整个套件没跑起来。把它们计入「失败用例 N 条」就与 `Tests:` 行对不上 ⇒ 单列一段。
    $caseRe = '^\s*[\u25CF\u2715\u2716\u2717\u2718]\s*(\S.*)$'
    $cases = [System.Collections.Generic.List[object]]::new()
    $notes = [System.Collections.Generic.List[object]]::new()
    $seenEntry = [System.Collections.Generic.HashSet[string]]::new()
    $dupEntries = 0
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -notmatch $caseRe) { continue }
        $name = "$($matches[1])".Trim()
        if ([string]::IsNullOrWhiteSpace($name)) { continue }
        $isNote = ($name -eq 'Console') -or ($name -match '^Test suite failed to run')
        $entry = [System.Collections.Generic.List[string]]::new()
        $entry.Add("  - $name")
        # 判据行：紧随其后的非空行（`Expected` / `Received` / 断言消息 / `at … .test.js:行:列` 栈在这层）。
        # ⚠️ **空行要跳过而不是中止**：jest 的失败块内部就有空行分隔（`● 用例名` / 空行 / `Expected: 31`），
        #    拿空行当终止符会让每条只剩用例名 ⇒「为什么红」整层丢失（本机实测踩过这一形）。
        #    中止条件只有三个：下一条用例名、套件行（FAIL/PASS）、jest 汇总行 —— 后者是**最后一坨**，
        #    不吃掉它才会既进「汇总」段、又被末条用例当判据重复一遍。
        $taken = 0
        for ($j = $i + 1; $j -lt $lines.Count -and $taken -lt $DetailLines; $j++) {
            if ($lines[$j] -match $caseRe) { break }
            if ($lines[$j] -match '^\s*(?:Test Suites?|Tests|Snapshots|Time):\s') { break }
            if ($lines[$j] -match '(?:^|\s)(?:FAIL|PASS)\s+\S+\.test\.') { break }
            $d = "$($lines[$j])".Trim()
            if ($d.Length -eq 0) { continue }
            if ($d.Length -gt 180) { $d = $d.Substring(0, 180) + '...' }
            $entry.Add("      $d")
            $taken++
        }
        $rendered = ($entry -join "`r`n")
        # 去重键带上「是不是套件级」⇒ 同名条目不会互相顶掉
        if (-not $seenEntry.Add($(if ($isNote) { "n|$rendered" } else { "c|$rendered" }))) { $dupEntries++; continue }
        if ($isNote) { $null = $notes.Add($rendered) } else { $null = $cases.Add($rendered) }
    }

    # 缺字段要**说出来**，而且要**写进文件**：调用点的 `Write-Host` 一闪而过，而这份摘要正是给
    # 「事后打开看」用的 —— 只在返回对象里带 Missing、文件却是一份干净的表头，就是本票要消灭的
    # 「看不见红」换了个位置（输出形态一变、摘要静默变空 = 新的假绿出口）。
    $missing = [System.Collections.Generic.List[string]]::new()
    if (@($summary | Where-Object { $_ -match '^\s*Tests:' }).Count -eq 0) { $missing.Add('Tests 汇总行') }
    if (@($summary | Where-Object { $_ -match '^\s*Time:' }).Count -eq 0) { $missing.Add('Time 汇总行') }
    # 「抓到东西」包含**套件级说明**：实测有 `● Test suite failed to run`（整个套件没跑起来）这种
    # 只有说明、没有用例条目的形态 —— 那时原因拿到了，不该报「什么都没抓到」。
    if ($suites.Count -eq 0 -and $cases.Count -eq 0 -and $notes.Count -eq 0) { $missing.Add('失败套件 / 失败用例名') }

    $head = @(
        "dev-finish 单元测试失败摘要（jest exit=$ExitCode，生成于 $((Get-Date).ToString('yyyy-MM-dd HH:mm:ss'))）"
        '本文件是人读出口，不作门禁输入：判定仍以单元测试的 exit code 为准。'
        ''
    )
    if ($missing.Count -gt 0) {
        $head += "警告：本次输出里没抓到 $($missing -join '、') ⇒ jest 的输出形态可能变了，请看终端里的完整输出。"
        $head += ''
    }

    # 组装 = 表头 + 汇总行 + 失败套件 + 失败用例 + 套件级说明 (+ 丢弃说明)。
    # 顺序有两条后果，分开说，别混成一条：
    #   · **整条丢弃只丢「说明 → 用例 → 套件」** —— 汇总段不在可丢集合里，所以 97 个套件一起红
    #     （FAIL 行就 ≈4 KB）时 `Tests:` / `Time:` 仍然在文件里（守护 S6）。
    #   · **硬裁剪保头不保尾** —— 预算小到连表头都装不下时按字符砍尾部，排在前面的先保住，
    #     所以「这份摘要在说什么 + 没抓到什么」的表头与警告排在最前（守护 S5）。
    # 用**局部脚本块**而不是 `function script:` —— 后者会在脚本作用域留下第二个真源名字，
    # 且 AST 抽函数定义时不会被抽到。
    # ⚠️ 参数名不能叫 `$Note`：PowerShell **变量名大小写不敏感**，`$Note` 与闭包里的 `$notes`
    #    （套件级说明列表）是同一个名字 —— 写下去就是「表头段被丢弃说明覆盖」这种查半天的坑。
    $assemble = {
        param($SuiteList, $CaseList, $NoteList, $DropNote)
        $keptSuites = @($SuiteList)
        $keptCases = @($CaseList)
        $keptNotes = @($NoteList)
        $parts = [System.Collections.Generic.List[string]]::new()
        foreach ($h in $head) { $parts.Add($h) }
        if ($summary.Count -gt 0) {
            $parts.Add('汇总：')
            foreach ($s in $summary) { $parts.Add("  $($s.Trim())") }
            $parts.Add('')
        }
        if ($keptSuites.Count -gt 0) {
            $parts.Add("失败套件（列出 $($keptSuites.Count) / 共 $($suites.Count) 个）：")
            foreach ($s in $keptSuites) { $parts.Add("  $($s.Trim())") }
            $parts.Add('')
        }
        if ($keptCases.Count -gt 0) {
            $parts.Add("失败用例（列出 $($keptCases.Count) / 共 $($cases.Count) 条）：")
            foreach ($c in $keptCases) { $parts.Add($c) }
            $parts.Add('')
        }
        if ($keptNotes.Count -gt 0) {
            $parts.Add("套件级说明（列出 $($keptNotes.Count) / 共 $($notes.Count) 条；Console 日志与「整个套件没跑起来」，不是失败用例）：")
            foreach ($n in $keptNotes) { $parts.Add($n) }
        }
        if ($DropNote) { $parts.Add($DropNote) }
        return (($parts -join "`r`n") + "`r`n")
    }

    # 先按条数封顶（返回对象里的计数仍是**去重后的总数**，不被上限骗过），再按字节裁剪
    $keepSuites = @($suites | Select-Object -First $MaxEntries)
    $keepCases = @($cases | Select-Object -First $MaxEntries)
    $keepNotes = @($notes | Select-Object -First $MaxEntries)
    # 丢最后一条（丢到空就停在这一段），三段共用一个收缩器，别再抄三份
    $shrink = { param($arr) @(if (@($arr).Count -le 1) { @() } else { $arr[0..(@($arr).Count - 2)] }) }
    $text = & $assemble $keepSuites $keepCases $keepNotes $null
    # 超预算 ⇒ 按 **套件级说明 → 失败用例 → 失败套件** 的顺序整条丢（汇总段永远不丢：它给出
    # 「总共红了几条」这一层，丢了就只剩一份看不出规模的清单），每一步都**明说**各丢了几条 ——
    # 静默截断等于换一种「看不见」。三段都丢光仍超（表头 + 汇总本身太大）⇒ 交给下面的硬裁剪。
    while ($utf8.GetByteCount($text) -gt $MaxBytes) {
        if (@($keepNotes).Count -gt 0) { $keepNotes = & $shrink $keepNotes }
        elseif (@($keepCases).Count -gt 0) { $keepCases = & $shrink $keepCases }
        elseif (@($keepSuites).Count -gt 0) { $keepSuites = & $shrink $keepSuites }
        else { break }
        $hidden = @()
        if ($notes.Count - @($keepNotes).Count -gt 0) { $hidden += "$($notes.Count - @($keepNotes).Count) 条套件级说明" }
        if ($cases.Count - @($keepCases).Count -gt 0) { $hidden += "$($cases.Count - @($keepCases).Count) 条失败用例" }
        if ($suites.Count - @($keepSuites).Count -gt 0) { $hidden += "$($suites.Count - @($keepSuites).Count) 个失败套件" }
        $dropText = $null
        if (@($hidden).Count -gt 0) { $dropText = "...另有 $($hidden -join '、')未列出（完整输出见本次运行的终端）。" }
        $text = & $assemble $keepSuites $keepCases $keepNotes $dropText
    }
    $droppedCases = $cases.Count - @($keepCases).Count
    $droppedSuites = $suites.Count - @($keepSuites).Count
    $droppedNotes = $notes.Count - @($keepNotes).Count

    # 兜底：连表头 + 汇总都装不下（极端：套件没跑成、堆栈巨长）⇒ 按**字符**整体裁剪，保证承诺成立。
    $truncated = $false
    if ($utf8.GetByteCount($text) -gt $MaxBytes) {
        $truncated = $true
        $note = "...（超出 $MaxBytes 字节预算，已按字节裁剪）`r`n"
        $budget = $MaxBytes - $utf8.GetByteCount($note)
        if ($budget -lt 0) { $budget = 0 }
        $chars = $text.ToCharArray()
        for ($n = $chars.Length; $n -gt 0; $n--) {
            $cand = -join $chars[0..($n - 1)]
            if ($utf8.GetByteCount($cand) -le $budget) { $text = $cand + $note; break }
        }
    }

    $dir = Split-Path -Parent $Path
    if ($dir) { $null = New-Item -ItemType Directory -Force -Path $dir }
    [System.IO.File]::WriteAllText($Path, $text, [System.Text.UTF8Encoding]::new($false))

    # 计数面全部是**去重后**的唯一数；被并掉的重复行/块另报（`DupSuites` / `DupEntries`），
    # 让「输出里同一条出现两遍」这件事在返回对象里看得见，而不是悄悄被去重吞掉。
    return [pscustomobject]@{
        Path          = $Path
        Bytes         = $utf8.GetByteCount($text)
        FileBytes     = ([System.IO.File]::ReadAllBytes($Path)).Length
        Lines         = @($text -split "\r?\n").Count
        Truncated     = $truncated
        SuiteCount    = $suites.Count
        CaseCount     = $cases.Count
        NoteCount     = $notes.Count
        KeptSuites    = @($keepSuites).Count
        KeptCases     = @($keepCases).Count
        KeptNotes     = @($keepNotes).Count
        DroppedSuites = $droppedSuites
        DroppedCases  = $droppedCases
        DroppedNotes  = $droppedNotes
        DupSuites     = $dupSuites
        DupEntries    = $dupEntries
        Missing       = @($missing)
    }
}

<#
.SYNOPSIS
    把一次**全量单测门**的运行落盘成两件产物（完整日志 + 红跑的人读摘要），并组出一行可复算的机检行
    （#1546，2026-10-05）。

.DESCRIPTION
    存在理由：#1526 只把「单元测试（契约子集）」那一步的失败详情留住了；全量那一条（`npm run test:unit`）
    **至今没有落盘脚本** —— 终端滚过去就没了，调用方拿到的只有 exit code。于是「刚才哪个红了」只能
    **再跑几分钟**（票 #1546 打的正是 #1545 读数里的 C1 那一档：为重看失败原因而重跑）。

    五条硬口径，逐条都是可断言的：
      · **日志恒定落盘，且落的是原文** —— 绿跑也要有日志：它是「这道门真的跑过、结论确实是 0」的唯一物证
        （④ 的两条门脚本同形：先写 `.ci-verify/kotlin-all.log`，再打结论行）。
      · **摘要只随红跑产生** —— 复用 `New-TestFailureSummary`，它自己保证 `ExitCode=0` 直接返回、不碰文件系统。
        一份「长得像证据」的绿跑镜像文件正是本仓防的形态。
      · **陈旧摘要要指名** —— 摘要路径是固定的，所以**下一次绿跑不会覆盖上一跑留下的那份红摘要**；而本票回答的
        是「**刚才**哪个红了」，「刚才」完全可能是这一次绿跑（什么都没红）。既不删（`.ci-verify/` 里的东西在本仓
        是判据输入，删了不可复算）也不装看不见 ⇒ 绿跑遇到磁盘上还有摘要时，机检行写 `staleSummary=<那份的 mtime>`。
      · **日志本体不被按内容 hash 折叠** —— 机检行引用的是**路径**，不是内容的摘要值：结论行可以被 sha 绑定，
        但事后复核靠的是日志文件本体（#1522 那条「同 sha 只留一份日志」的候选已撤销并在册）。
      · **不判成败、不新增第二个绿面** —— 本函数把传入的 `$ExitCode` **原样**写进机检行，不产出任何
        「绿/红」的新结论；调用方的判定仍然是且只是那一个退出码。

    机检行的数字全部取自本次输出**自己的**汇总行（`Test Suites:` / `Tests:` / `Time:`），所以拿着日志
    就能重算出同一串 —— 这是票面「机检行可复算」的落点。抓不到的字段**点名**写进 `missing=`，
    不静默补 0（补 0 是把「读不出」伪装成「没红」）。

    纯函数边界：唯一**写**副作用是写 `$LogPath` 与（仅红跑）`$SummaryPath` 两个文件；除此之外只读
    `$SummaryPath` 自己的 mtime（只为指名陈旧摘要，不删不改）；不跑 jest、不读工作树源码、
    不取锁、不碰设备 ⇒ 可被 `utils/devFinishContract.test.js` 用 AST 抽函数定义后**真执行**并断言产物。
#>
function Write-UnitGateArtifacts {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][AllowNull()][AllowEmptyString()][string]$Output,
        [Parameter(Mandatory = $true)][string]$LogPath,
        [Parameter(Mandatory = $true)][string]$SummaryPath,
        [int]$ExitCode = 0,
        [int]$MaxBytes = 2048,
        [int]$DurationSeconds = 0,
        # 机检行里显示的引用（默认用绝对路径）。调用点传仓内相对形（`.ci-verify/test-unit.log`），
        # 于是**终端打的那一行**与**贴进 PR 评论的那一行**逐字相同 —— 评论里的绝对本地路径没有读者能打开。
        [string]$LogRef = '',
        [string]$SummaryRef = ''
    )
    $utf8 = [System.Text.Encoding]::UTF8
    $lines = @($Output -split "\r?\n")

    # ---- 1) 汇总行取数：取**最后**一条 ----
    # jest 的失败详情里可能把同名的行再引一遍（#1526 实测过「同一个套件的整块详情出现两遍」），
    # reporter 收尾打的那一份在最后 ⇒ 取最后一条才不会把「详情里的引用」当成总量。
    $lastMatch = {
        param([string]$Pattern)
        @($lines | Where-Object { $_ -match $Pattern } | Select-Object -Last 1)
    }
    $suiteLine = & $lastMatch '^\s*Test Suites?:\s'
    $testLine = & $lastMatch '^\s*Tests:\s'
    $timeLine = & $lastMatch '^\s*Time:\s'

    # 「有这一行但没有 `N failed`」= jest **省略了零值类别**（绿跑只打 `3000 passed, 3000 total`），
    # 读成 0；「整行都没有」才是输出形态变了 ⇒ 交给你看的那份日志，不许在这里替它编一个数。
    $grab = {
        param([string]$Line, [string]$Label)
        if (@($Line).Count -eq 0 -or [string]::IsNullOrWhiteSpace("$Line")) { return $null }
        $m = [regex]::Match("$Line", "(\d+)\s+$Label")
        if ($m.Success) { return [int]$m.Groups[1].Value }
        return 0
    }
    $suitesFailed = & $grab $suiteLine 'failed'
    $suitesTotal = & $grab $suiteLine 'total'
    $testsFailed = & $grab $testLine 'failed'
    $testsTotal = & $grab $testLine 'total'
    $timeText = ''
    if (@($timeLine).Count -gt 0 -and -not [string]::IsNullOrWhiteSpace("$timeLine")) {
        $timeRaw = ("$timeLine" -replace '^\s*Time:\s*', '').Trim()
        # `Time:` 行的本体是「本次测量值」；jest 会在它后面挂一条 `, estimated 814 s` —— 那是**对下一跑的预估**
        # （2026-10-05 实测：一次 287.673 s 的绿跑挂着 `estimated 814 s`，因为上一跑真的跑了 813.602 s）。
        # 预估不是本次耗时，也不该混进一个 `key=value` 的机检 token 里 ⇒ 只取 `数 + s`，且它仍是日志里那一行的**前缀**
        # ⇒ 「拿日志复算」这条不因为归一而失效。整行读不出来时退回原文（去掉空白），不静默丢弃。
        $tm = [regex]::Match($timeRaw, '^([\d.]+\s*s)')
        $timeText = $(if ($tm.Success) { $tm.Groups[1].Value -replace '\s+', '' } else { $timeRaw -replace '\s+', '' })
    }

    $missing = [System.Collections.Generic.List[string]]::new()
    if ($null -eq $suitesTotal) { $missing.Add('suites') }
    if ($null -eq $testsTotal) { $missing.Add('tests') }
    if ([string]::IsNullOrWhiteSpace($timeText)) { $missing.Add('time') }

    # ---- 2) 摘要：绿跑不落（由 `New-TestFailureSummary` 自身保证），红跑落 ----
    # 摘要落盘**失败不能改变门的结果**：判据仍是调用方手里的那个退出码，所以这里 catch 而不是让它外抛；
    # 但也不能静默 —— 外抛或 catch 都要在机检行里留名（`summaryError=`）。
    $summary = $null
    $summaryError = ''
    try {
        $summary = New-TestFailureSummary -Output $Output -Path $SummaryPath -ExitCode $ExitCode -MaxBytes $MaxBytes
    }
    catch {
        $summaryError = "$($_.Exception.Message)" -replace '\s+', ' '
        if ($summaryError.Length -gt 120) { $summaryError = $summaryError.Substring(0, 120) }
    }
    # **陈旧摘要要指名**：摘要是「那一次红跑」的产物，路径固定 ⇒ 下一次绿跑不会覆盖它，它会留在原处。
    # 本票要回答的是「**刚才**哪个红了」，而「刚才」可能是这一次绿跑（什么都没红）—— 拿上一跑的摘要回答
    # 这一跑，比不落盘更坏。所以绿跑遇到磁盘上还留着摘要时：**不删**（`.ci-verify/` 里的东西在本仓是判据输入，
    # 删了不可复算），而是在机检行里把它的时间戳点出来，让读的人知道那不是本次的产物。
    $staleSummary = ''
    if ($null -eq $summary -and -not $summaryError -and (Test-Path -LiteralPath $SummaryPath -PathType Leaf)) {
        # 只认**文件**：写失败时那一条路径可能是别的形态（守护 U6 就故意把它种成一个目录），
        # 报「陈旧摘要」会把「写不进去」说成「上次红过」—— 两件事必须分开有名。
        # 时间戳取**无空格**形态：机检行是 `key=value` 空格分隔的一行，值里不能夹空格
        $staleSummary = ([System.IO.File]::GetLastWriteTime($SummaryPath)).ToString('yyyyMMdd-HHmmss')
    }

    $logRef = $(if ($LogRef) { $LogRef } else { $LogPath })
    $summaryRef = $(if ($null -eq $summary) { 'none' } elseif ($SummaryRef) { $SummaryRef } else { $SummaryPath })

    # ---- 3) 机检行 ----
    $show = { param($v) $(if ($null -eq $v) { '?' } else { "$v" }) }
    $parts = @(
        'TEST_UNIT_RESULT'
        "exit=$ExitCode"
        "suites=$(& $show $suitesTotal)"
        "suitesFailed=$(& $show $suitesFailed)"
        "tests=$(& $show $testsTotal)"
        "testsFailed=$(& $show $testsFailed)"
        "time=$(if ($timeText) { $timeText } else { '?' })"
        "wall=${DurationSeconds}s"
        "log=$logRef"
        "summary=$summaryRef"
    )
    if ($staleSummary) { $parts += "staleSummary=$staleSummary" }
    if ($missing.Count -gt 0) { $parts += "missing=$(@($missing) -join '|')" }
    if ($summaryError) { $parts += "summaryError=$summaryError" }
    $resultLine = $parts -join ' '

    # ---- 4) 日志：原文 + 机检行 ----
    # 机检行**追加在日志尾部**（④c 同形：`Write-Log "\n$summary"`）⇒ 只拿这一份文件就能既读到
    # 完整输出、又读到结论行，复算不需要第三个东西。
    $dir = Split-Path -Parent $LogPath
    if ($dir) { $null = New-Item -ItemType Directory -Force -Path $dir }
    [System.IO.File]::WriteAllText($LogPath, ($Output + "`r`n" + $resultLine + "`r`n"), [System.Text.UTF8Encoding]::new($false))

    return [pscustomobject]@{
        ResultLine   = $resultLine
        LogPath      = $LogPath
        LogBytes     = ([System.IO.File]::ReadAllBytes($LogPath)).Length
        ExitCode     = $ExitCode
        SuitesTotal  = $suitesTotal
        SuitesFailed = $suitesFailed
        TestsTotal   = $testsTotal
        TestsFailed  = $testsFailed
        Time         = $timeText
        Missing      = @($missing)
        SummaryPath  = $(if ($null -ne $summary) { $summary.Path } else { '' })
        SummaryBytes = $(if ($null -ne $summary) { $summary.Bytes } else { 0 })
        SummaryError = $summaryError
        StaleSummary = $staleSummary
    }
}

<#
.SYNOPSIS
    把一次全量单测门的结果贴成 **sha 绑定**的 PR 评论（#1546 的「纳入 sha 绑定评论」出口）。

.DESCRIPTION
    与 ②④ 两条门脚本的 `Publish-GateComment` 同一格式（标记 + `commit: <sha>` + 结论行含产物 + 复现），
    所以 `pr-evidence.yml` 的祖先/等值绑定口径将来若要接住 ③，不需要改评论形状。
    **本函数不是判据**：贴得过与贴不过都不改门的结果，两条路径都 `Write-Host` 警告后返回。
    贴评论**只在红跑时附摘要正文** —— `.ci-verify/` 是 gitignored、且随工作树回收一起消失，
    而「刚才哪个红了」要能在树没了之后仍然读得到；评论是它唯一的持久出口。绿跑不附正文，
    否则每次收口都往 PR 上贴一份「长得像证据」的绿跑镜像。
#>
function Publish-UnitGateComment {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][int]$PrNumber,
        [Parameter(Mandatory = $true)][string]$ResultLine,
        [Parameter(Mandatory = $true)][string]$LogRelative,
        [string]$SummaryText = '',
        [string]$ProjectDir = '',
        [string]$ReproCommand = 'npm run test:unit:gate'
    )
    $sha = Get-HeadSha -ProjectDir $ProjectDir
    if (-not $sha) {
        Write-Host '[warn] 取不到 HEAD sha（git 不可用或不在仓库里）：跳过贴 PR 评论，门结论不受影响。' -ForegroundColor Yellow
        return
    }
    $short = $sha.Substring(0, [Math]::Min(7, $sha.Length))
    # 门号字形用 [char] 拼，不放裸字形进源码（本文件 `New-TestFailureSummary` 里的同一条纪律）。
    $marker = '<!-- gate-evidence:{0} -->' -f [char]0x2462
    $body = [System.Collections.Generic.List[string]]::new()
    $body.Add($marker)
    $body.Add('**全量单测门（`npm run test:unit`，agent 执行）**')
    $body.Add("- commit: $sha")
    $body.Add("- 结论（含产物）：``$ResultLine``；日志 ``$LogRelative``")
    $body.Add("- 复现：``$ReproCommand``")
    if ($SummaryText) {
        $body.Add('')
        $body.Add('失败摘要正文（人读出口，不作门禁输入）：')
        $body.Add('```')
        $body.Add("$SummaryText".TrimEnd())
        $body.Add('```')
    }
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
        Write-Host '[warn] 找不到 gh CLI：跳过贴 PR 评论，门结论不受影响（结果见上面日志）。' -ForegroundColor Yellow
        return
    }
    try {
        $out = (& gh pr comment $PrNumber --body ($body -join "`n") 2>&1 | Out-String)
        if ($out -match 'github\.com/') {
            Write-Host "✅ 已贴 PR #$PrNumber 的全量单测门评论（sha 绑定 $short）。" -ForegroundColor Green
        }
        else {
            Write-Host "[warn] 贴 PR #$PrNumber 评论疑似失败（门结论不受影响）：$($out.Trim())" -ForegroundColor Yellow
        }
    }
    catch {
        Write-Host "[warn] 贴 PR 评论失败（门结论不受影响）：$_" -ForegroundColor Yellow
    }
}


# ============================================================
# DryRun：只打印计划
# ============================================================
if ($DryRun) {
    Write-Host "=== dev:finish 计划（-DryRun：不执行任何操作）===" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "步骤："
    Write-Host "  1. 级别判定（根据 git diff）"
    Write-Host "  2. 环境检测（Q-A 只验 git+项目根；其余验设备 + HBuilderX cli）"
    Write-Host "  3. 单元测试（本工具链契约测试）"
    Write-Host "  ── 以下 4-6 仅在实际需要 HBuilderX 时才进入锁临界区（Q-A 直接跳过）──"
    Write-Host "  4. 编译：quick 默认 **不编译**（Q-A）；-Compile 或 standard/full 才编译"
    Write-Host "  5. 部署：quick → 不部署；standard/full → hx-run 真运行"
    Write-Host "  6. 截图：只截改动页面（🟡🔴；最多 $MaxScreenshotPages 页）"
    Write-Host "  ── 临界区结束，锁已释放 ──"
    Write-Host "  7. 截图对比基线（🟡🔴）"
    Write-Host "  8. 生成验收证据（🟡🔴）"
    Write-Host "  9. 还原 manifest.json / pages.json"
    Write-Host "  + 能力面提示（ADR-0016 ①b）：本批触及能力面 是/否 + 原生能力调用点扫描（只做提示、不判红）"
    Write-Host ""
    Write-Host "参数："
    Write-Host "  -Device:              ${Device:-<自动检测>}"
    Write-Host "  -Level:               ${Level:-<自动判定>}"
    Write-Host "  -Compile:             $Compile   # quick 下是否做编译期诊断（Q-B）"
    Write-Host "  -Distribute:          $Distribute"
    Write-Host "  -UpdateBaseline:      $UpdateBaseline"
    Write-Host "  -PixelThreshold:      $PixelThreshold   # 步骤 7 像素差阈值（0.005 = 0.5%）；像素层判不了时回退 MD5"
    Write-Host "  -IgnoreTopRows:       $IgnoreTopRows   # 步骤 7 忽略顶部行数（状态栏读数；真机建议取状态栏高度）"
    Write-Host "  -IgnoreBottomRows:    $IgnoreBottomRows"
    Write-Host "  -MaxScreenshotPages:  $MaxScreenshotPages"
    Write-Host "  -HxWaitSeconds:       $HxWaitSeconds"
    Write-Host "  -HxRunTimeoutSeconds: $HxRunTimeoutSeconds"
    Write-Host ""

    # DryRun 也把**真实的**能力面判定打出来：白名单判定 + 调用点扫描都是 git diff 的纯函数
    # （不取锁 / 不占设备 / 不跑测试），所以这里能给出「脚本真的接线了」的**行为级**证据
    # —— 守护 H7 就是跑这条路径，而不是断言源码文本（ADR-0008 的收束方向）。
    . (Join-Path $PSScriptRoot 'lib\level-detect.ps1')
    . (Join-Path $PSScriptRoot 'lib\capability-surface.ps1')
    $dryLevel = Get-DetectLevel -ProjectDir $ProjectDir -ForceLevel $Level
    Write-Host '--- 能力面提示（真实结果；正式收口时打在摘要之前）---' -ForegroundColor DarkCyan
    foreach ($line in @(Format-CapabilityHint -ChangedFiles @($dryLevel.ChangedFiles) -ProjectDir $ProjectDir)) {
        Write-Host $line
    }
    exit 0
}

# ============================================================
# 全量单测门落盘模式（#1546）：只跑这一件事，跑完按 jest 的退出码退出
#   位置在 -DryRun 之后 —— `-DryRun -UnitGate` 同时给时，「不执行任何操作」那条承诺优先。
#   不进九步管线、不取 HBuilderX 锁、不碰设备 ⇒ 别的会话持锁时它照样能跑（③ 本来就不占主程序）。
# ============================================================
if ($UnitGate) {
    $gateStarted = Get-Date
    $ciDir = Join-Path $ProjectDir '.ci-verify'
    $unitLogPath = Join-Path $ciDir 'test-unit.log'
    $unitSummaryPath = Join-Path $ciDir 'test-unit-failure.txt'
    Write-Host '=== 全量单测门：日志 +（仅红跑）失败摘要 + 机检行 ===' -ForegroundColor Cyan
    Write-Host "  日志（绿跑也落，落原文）：$unitLogPath"
    Write-Host "  摘要（只有红跑才落）：$unitSummaryPath"
    # 跑的是 package.json 里**那一条门命令本身**：本脚本不再抄一份 jest 参数 —— 参数一旦在这里再抄一份，
    # 「门」与「被落盘的那一次」就是两个东西了（与 #974 的 pattern 双份漂移同形）。
    try {
        $unitOutput = & npm run test:unit 2>&1 | Out-String
    }
    catch {
        # 与「单元测试」那一步同形：`$ErrorActionPreference='Stop'` 下原生命令写 stderr 可能抛 ⇒ 把异常
        # 文本当成输出继续走，判定仍取退出码（这里拿不到退出码时脚本会在 strict mode 下炸 = fail-closed）。
        $unitOutput = "$($_.Exception.Message)"
    }
    $unitExitCode = $LASTEXITCODE
    $art = Write-UnitGateArtifacts -Output $unitOutput -LogPath $unitLogPath -SummaryPath $unitSummaryPath `
        -ExitCode $unitExitCode -DurationSeconds ([int]((Get-Date) - $gateStarted).TotalSeconds) `
        -LogRef '.ci-verify/test-unit.log' -SummaryRef '.ci-verify/test-unit-failure.txt'

    Write-Host ''
    Write-Host $art.ResultLine -ForegroundColor $(if ($art.ExitCode -eq 0) { 'Green' } else { 'Red' })
    Write-Host "  日志：$unitLogPath（$($art.LogBytes) 字节）" -ForegroundColor Yellow
    if ($art.SummaryPath) {
        # 只报路径与体量，正文留在文件里 —— 终端这一行是给「之后要不要重跑」做决定的，不是给人读详情用的
        Write-Host "  失败摘要（$($art.SummaryBytes) 字节，人读，不作门禁输入）：$($art.SummaryPath)" -ForegroundColor Yellow
    }
    elseif ($art.StaleSummary) {
        # 磁盘上那份是**上一跑**的红摘要（绿跑不覆盖它）。不指出来，就会有人拿它回答「刚才哪个红了」，
        # 而刚才这一次什么都没红 —— 那是把「看不见」换成「看得见但看错」。不删：`.ci-verify/` 在本仓是判据输入。
        Write-Host "  ⚠️ 本次是绿跑，但 $unitSummaryPath 还留着 $($art.StaleSummary) 那一次红跑的摘要 ⇒ 它不是本次结论" -ForegroundColor DarkYellow
    }
    if ($art.SummaryError) {
        Write-Host "  ⚠️ 摘要未落盘：$($art.SummaryError)（判定不受影响，照旧只看退出码）" -ForegroundColor DarkYellow
    }
    if (@($art.Missing).Count -gt 0) {
        # 汇总行读不出来 ⇒ 明说读不出来。「缺字段」补 0 就是把「读不出」伪装成「没红」
        Write-Host "  ⚠️ 机检行缺字段：$(@($art.Missing) -join '、')（jest 的输出形态可能变了，请看日志原文）" -ForegroundColor DarkYellow
    }
    if ($art.ExitCode -eq 0) { Write-Result $true '全量单测门通过（结论由退出码给出，摘要只在红跑产出）' }
    # ⚠️ 属性引用必须走 `$($art.ExitCode)` 子表达式：双引号里的 `$art.ExitCode` **不展开属性** ——
    #    PowerShell 把 `$art` 整个 ToString 再接一串字面量 `.ExitCode`（2026-10-05 真链路实测：终端打出
    #    `exit @{ResultLine=…; ExitCode=1; …}.ExitCode`）。同文件的其它行本来就是子表达式形态，
    #    只有这一行漏了 —— 而守护钉的是 `exit $art.ExitCode` 那条**语句**，钉不到这句文案。
    else { Write-Result $false "全量单测门未过（exit $($art.ExitCode)）" }

    if ($PostToPr -gt 0) {
        # Get-HeadSha 取共享库那份（`lib/gate-common.ps1` 里唯一合格的成员）；本脚本不自定义。
        . (Join-Path $PSScriptRoot 'lib\gate-common.ps1')
        $summaryText = ''
        if ($art.SummaryPath -and (Test-Path -LiteralPath $art.SummaryPath)) {
            $summaryText = [System.IO.File]::ReadAllText($art.SummaryPath, [System.Text.Encoding]::UTF8)
        }
        Publish-UnitGateComment -PrNumber $PostToPr -ResultLine $art.ResultLine `
            -LogRelative '.ci-verify/test-unit.log' -SummaryText $summaryText -ProjectDir $ProjectDir
    }
    exit $art.ExitCode
}

# ============================================================
# Step 1: 级别判定
#   提前到环境检测之前 —— 「要不要 HBuilderX / 设备」取决于级别与 -Compile，
#   而 Q-A（quick 且未 -Compile）**既不需要主程序也不需要设备**，若仍走
#   Test-BuildEnv 就会去取 HBuilderX 互斥锁 ⇒ 别的会话持锁时「秒级」变「等 120 秒后 exit 2」。
# ============================================================
Write-Step 1 9 '级别判定'
. (Join-Path $PSScriptRoot 'lib\level-detect.ps1')
$levelResult = Get-DetectLevel -ProjectDir $ProjectDir -ForceLevel $Level
$detectedLevel = $levelResult.Level
$levelIcon = switch ($detectedLevel) { 'quick' { '🟢' } 'standard' { '🟡' } 'full' { '🔴' } }
Write-Result $true "$levelIcon $detectedLevel — $($levelResult.Reason)"

# 本次是否需要 HBuilderX / 设备（Q-A 不需要）
$needsHx = ($detectedLevel -ne 'quick') -or $Compile
$quickTier = ''
if ($detectedLevel -eq 'quick') {
    $quickTier = if ($Compile) { 'Q-B 编译诊断' } else { 'Q-A 静态守护' }
}

# ============================================================
# Step 2: 环境检测（按需求分级）
# ============================================================
Write-Step 2 9 '环境检测'
. (Join-Path $PSScriptRoot 'lib\env-check.ps1')
if ($needsHx) {
    $envResult = Test-BuildEnv -ProjectDir $ProjectDir -Device $Device -HxWaitSeconds 120
    if (-not $envResult.Ok) {
        Write-Result $false $envResult.Error
        exit 2
    }
    Write-Result $true "设备 $($envResult.Device) 在线，HBuilderX cli 可用"
    $Device = $envResult.Device
}
else {
    $envResult = Test-StaticEnv -ProjectDir $ProjectDir
    if (-not $envResult.Ok) {
        Write-Result $false $envResult.Error
        exit 2
    }
    Write-Result $true "$quickTier：只验 git + 项目根（不取锁、不要求设备）"
}

# ============================================================
# Step 3: 单元测试
# ============================================================
Write-Step 3 9 '单元测试'
# ⚠️ pattern 取自**唯一真源** `lib\contract-tests.ps1`，本文件**不得**再抄一份自己的字面量
#    （曾有第二份、且与 test-compile 的那份漂移：少了 hxRun/hxTimingBehavior/hxError，
#     而步骤 3 先跑且失败即 exit 1 ⇒ 实际门禁是**更窄的那份**）。守护：contractTestPatternBehavior.test.js
. (Join-Path $PSScriptRoot 'lib\contract-tests.ps1')
$testPattern = Get-ContractTestPattern
try {
    $testOutput = & npx jest --config jest.config.unit.js -i --testPathPattern $testPattern --forceExit 2>&1 | Out-String
}
catch {
    $testOutput = "$($_.Exception.Message)"
}
# 先固化成变量：下面要**调一个函数**（内部有 New-Item / [IO.File] 等托管调用），而 `$LASTEXITCODE`
# 只由原生命令更新 —— 现在没事，但「判据依赖一个可能被后续代码改写的自动变量」正是本仓防的形态。
$testExitCode = $LASTEXITCODE
# ---- 失败摘要落盘（#1526）----
# 为什么放在这里：本次 jest 的完整输出**只活在内存里**，`exit 1` 一执行就随进程蒸发 ⇒ 想知道
# 「哪一条红、为什么红」只能重跑（#1472 先例：六遍合计 2,140.466 秒）。
# **绿跑不落**由 `New-TestFailureSummary` 自身保证（`ExitCode=0` ⇒ 直接返回、不碰文件系统）——
# 把这条口径放进被守护执行的单元里，而不是只靠调用点「恰好写在 if 里」。
# 摘要是**人读出口，不作门禁输入**：下面判定仍然只看 `$testExitCode`。
$failSummaryPath = Join-Path $ProjectDir '.ci-verify\dev-finish-test-failure.txt'
$failSummary = $null
try {
    $failSummary = New-TestFailureSummary -Output $testOutput -Path $failSummaryPath -ExitCode $testExitCode
}
catch {
    # 摘要落盘失败**不能**改变门的结果：判据仍是单元测试的退出码（fail-closed 的方向是「照旧判红」）
    Write-Host "  ⚠️ 失败摘要未落盘：$($_.Exception.Message)" -ForegroundColor DarkYellow
}
if ($testExitCode -ne 0) {
    Write-Result $false "单元测试失败（exit code $testExitCode）"
    if ($failSummary) {
        Write-Host "  失败摘要（$($failSummary.Bytes) 字节，人读，不作门禁输入）：$($failSummary.Path)" -ForegroundColor Yellow
        if (@($failSummary.Missing).Count -gt 0) {
            # 没抓到东西就明说没抓到 —— 不许产出一份「看着像摘要、其实空的」文件还不吭声
            Write-Host "  ⚠️ 摘要缺字段：$(@($failSummary.Missing) -join '、')（输出形态可能变了，需人工看完整输出）" -ForegroundColor DarkYellow
        }
    }
    exit 1
}
Write-Result $true '单元测试通过'

# ============================================================
# 临界区：步骤 4-6 一次持锁（A-3）
#   HBuilderX 是单实例串行资源；且部署与截图共享设备状态，
#   中途被别的会话 launch 插进来会截到别人的页面。
# ============================================================
. (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
$hxLog = Join-Path $ProjectDir '.ci-verify\dev-finish.log'
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $hxLog) | Out-Null

Write-Host ''
if ($needsHx) {
    Write-Host '>>> 进入 HBuilderX 锁临界区（步骤 4-6）' -ForegroundColor DarkGray
    # 取锁：忙就等（上限 $HxWaitSeconds），超时 ⇒ exit 2 且**不抢占、不 kill**
    $null = Wait-HxFree -CliExe $envResult.CliPath -TimeoutSeconds $HxWaitSeconds -LogPath $hxLog
    # 交接给子脚本（子进程会继承该 env；父持锁时子脚本不再自己加锁 ⇒ 避免自死锁）
    Set-HxLockOwnerEnv
}
else {
    # Q-A 不碰 HBuilderX ⇒ **不取锁**（否则别的会话持锁时这里会白等，把秒级变分钟级）
    Write-Host ">>> $quickTier：不取 HBuilderX 锁（本路径不使用主程序）" -ForegroundColor DarkGray
}

$screenshotResult = $null
$deployResult = $null

try {
    # ===== Step 4: 编译 =====
    Write-Step 4 9 '编译'
    . (Join-Path $PSScriptRoot 'lib\test-compile.ps1')
    $compileResult = Invoke-TestAndCompile -Level $detectedLevel -ProjectDir $ProjectDir -CliPath $envResult.CliPath -SkipTests -QuickCompile:$Compile -HxRunTimeoutSeconds $HxRunTimeoutSeconds
    if (-not $compileResult.Ok) {
        Write-Result $false "$($compileResult.Step) 失败: $($compileResult.Error)"
        exit 1
    }
    if ($detectedLevel -eq 'quick' -and -not $Compile) {
        Write-Result $true "$quickTier：契约测试已过；**未做编译诊断**（需要编译请加 -Compile）"
    }
    elseif ($detectedLevel -eq 'quick') {
        Write-Result $true "编译期诊断为空（hx-run -CompileOnly，未占设备）"
    }
    else {
        Write-Result $true "编译门通过（$($compileResult.CompileResult)）"
    }

    # ===== Step 5: 部署 =====
    Write-Step 5 9 '部署'
    . (Join-Path $PSScriptRoot 'lib\build-deploy.ps1')
    $deployResult = Invoke-BuildAndDeploy -CliPath $envResult.CliPath -Device $Device -ProjectDir $ProjectDir -Level $detectedLevel -RunTimeoutSeconds $HxRunTimeoutSeconds
    if (-not $deployResult.Ok) {
        Write-Result $false "部署失败: $($deployResult.Error)"
        exit 1
    }
    if ($deployResult.Deployed) {
        Write-Result $true "已到设备（耗时 $($deployResult.Duration)s）"
    }
    else {
        Write-Host "  ⏭️ 未部署：$($deployResult.Reason)" -ForegroundColor Gray
    }
    if ($deployResult.ManifestRestored) { Write-Host '  📝 manifest.json 已还原' -ForegroundColor Yellow }
    if ($deployResult.PagesRestored) { Write-Host '  📝 pages.json 已还原' -ForegroundColor Yellow }

    # ===== Step 6: 截图 =====
    Write-Step 6 9 '自动截图'
    if ($detectedLevel -eq 'quick') {
        Write-Host '  ⏭️ 快速模式未部署到设备，截图无意义 ⇒ 跳过' -ForegroundColor Gray
    }
    else {
        . (Join-Path $PSScriptRoot 'lib\auto-screenshot.ps1')
        $screenshotResult = Invoke-AutoScreenshot -Device $Device -CliPath $envResult.CliPath `
            -ProjectDir $ProjectDir -MaxPages $MaxScreenshotPages `
            -RunStartedAt $started
        if ($screenshotResult.Ok) {
            Write-Result $true "$($screenshotResult.Screenshots.Count) 页截图完成（$($screenshotResult.Screenshots -join ', ')）"
        }
        else {
            Write-Result $false "截图未通过: $($screenshotResult.Error)"
            exit 1
        }
    }
} finally {
    # 收尾**无条件**执行：断言失败 / exit / 异常都走这里
    # 只有真取过锁的路径才需要释放（Q-A 从未取锁，Release-HxLock 本就是 no-op，但仍显式分流以免误读）
    if ($needsHx) {
        Clear-HxLockOwnerEnv
        Release-HxLock
        Write-Host '>>> 已退出锁临界区' -ForegroundColor DarkGray
    }
}

# ============================================================
# Step 7: 截图对比（🟡🔴）
# ============================================================
$diffResult = $null
$evidenceResult = $null

Write-Step 7 9 '截图对比'
if ($detectedLevel -eq 'quick') {
    Write-Host '  ⏭️ 快速模式，跳过对比' -ForegroundColor Gray
}
else {
    . (Join-Path $PSScriptRoot 'lib\screenshot-diff.ps1')
    # 「本轮产物」的起点 = **本脚本开头那一个** `$started`（与步骤 6 传下去的**同一个**值）。
    # 不在这里另取 `Get-Date`：目录从不清理、文件名按页名固定，两个判据各取一次会漂 ——
    # 早取的那次会把本轮刚落的截图误判成陈旧（issue #1158）。
    $diffResult = Compare-ScreenshotBaseline -ProjectDir $ProjectDir -UpdateBaseline:$UpdateBaseline `
        -PixelThreshold $PixelThreshold -IgnoreTopRows $IgnoreTopRows -IgnoreBottomRows $IgnoreBottomRows `
        -RunStartedAt $started
    # 判据与文案的**单点真源**是 lib/screenshot-gate.ps1 的 Get-PngDiffVerdict（纯函数、被
    # utils/screenshotDiffBehavior.test.js 真跑并成对断言「无变化⇒绿 / 有变化⇒红」）。
    # 这里只负责**执行**它给出的动作 —— 2026-09-18（#1139）之前这段是 `Write-Host` 一行黄字就完事，
    # 没有 Write-Result、没有 exit ⇒ **永不 fail**（本仓三个「永远绿」先例之一）。
    $verdict = Get-PngDiffVerdict -DiffResult $diffResult -UpdateBaseline:$UpdateBaseline

    $fallbackReasons = @($diffResult.Fallback | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) })
    if (-not $diffResult.PixelRan -and $fallbackReasons.Count -gt 0) {
        # 像素层一次都没跑成（node 不在 / 全是非 PNG）⇒ 必须**说出来**，不许静默当 MD5 用
        Write-Host "  ⚠️ 本轮未走像素层（MD5 回退）：$($fallbackReasons -join '; ')" -ForegroundColor Yellow
    }

    # 被跳过的陈旧残留（issue #1158）：目录从不清理，上一轮失败运行的图会被算进「页数 / 变化数」。
    # 跳过是**有意的**，但必须**可见** —— 静默丢弃 = 另一种假绿（照实打出来，且已进返回对象）。
    $staleSkipped = @($diffResult.SkippedStale)
    if ($staleSkipped.Count -gt 0) {
        $staleNames = @($staleSkipped | ForEach-Object { $_.Name })
        Write-Host "  ⏭️ 跳过 $($staleSkipped.Count) 张非本轮产物（早于本轮运行起点）：$($staleNames -join ', ')" -ForegroundColor DarkYellow
        foreach ($s in $staleSkipped) {
            Write-Host "     · $($s.Name)（$($s.Side) 侧陈旧）" -ForegroundColor DarkGray
        }
    }

    # 首次运行 ⇒ 以本轮截图建立基线：这一条把「基线目录永远是空的」那个死循环直接掐掉
    # （ADR-0008:419 记的坑：旧实现只在 -UpdateBaseline 下填基线，于是每轮都把全部页面判成「新增」）。
    if ($verdict.Action -eq 'write-baseline') {
        $srcDir = Join-Path $ProjectDir '.ci-verify\screenshots'
        $dstDir = Join-Path $ProjectDir '.ci-verify\baseline'
        New-Item -ItemType Directory -Force -Path $dstDir | Out-Null
        foreach ($png in @(Get-ChildItem -Path $srcDir -Filter '*.png' -File)) {
            Copy-Item -LiteralPath $png.FullName -Destination (Join-Path $dstDir $png.Name) -Force
        }
    }

    Write-Result $verdict.Ok $verdict.Message
    if (-not $verdict.Ok) {
        Write-Host '     出路①：确认是有意改动 ⇒ 重跑并加 -UpdateBaseline（刷新基线）' -ForegroundColor Yellow
        Write-Host '     出路②：若是噪声/误改 ⇒ 修回后重跑；勿用 -UpdateBaseline 把红盖成绿' -ForegroundColor Yellow
        exit $verdict.ExitCode
    }
}

# ============================================================
# Step 8: 生成证据（🟡🔴）
# ============================================================
Write-Step 8 9 '生成证据'
if ($detectedLevel -eq 'quick') {
    Write-Host '  ⏭️ 快速模式，无需写证据' -ForegroundColor Gray
}
else {
    . (Join-Path $PSScriptRoot 'lib\evidence-gen.ps1')
    $evidenceResult = New-Evidence -Level $detectedLevel `
        -CompileResult $compileResult.CompileResult `
        -ScreenshotDiff $diffResult.Diff `
        -ScreenshotChangedCount $diffResult.ChangedCount `
        -ProjectDir $ProjectDir
    if ($evidenceResult.Generated) {
        Write-Result $true "已写入 $($evidenceResult.Path)"
    }
    else {
        Write-Result $false '证据生成失败'
    }
}

# ============================================================
# Step 9: 还原配置
# ============================================================
Write-Step 9 9 '还原配置'
if ($deployResult -and ($deployResult.ManifestRestored -or $deployResult.PagesRestored)) {
    Write-Result $true '配置文件已还原'
}
else {
    Write-Result $true '配置文件未被改脏'
}

# ============================================================
# 能力面提示（ADR-0016 ①b）：**只做提示、不判红**
#   口径真源 scripts/lib/capability-surface.ps1（路径白名单 + 原生能力调用点扫描）。
#   不进 `pr-evidence` 的判红路径、不影响本脚本退出码 —— 判错的代价是「看一眼」，不是「门红」。
#   位置：步骤 9 之后、摘要之前 —— 收口的人先看到「要不要人签 ①b」，再读下面的证据段建议句。
# ============================================================
. (Join-Path $PSScriptRoot 'lib\capability-surface.ps1')
Write-Host ''
Write-Host '========================================' -ForegroundColor DarkCyan
Write-Host '  能力面（ADR-0016 ①b）' -ForegroundColor Cyan
Write-Host '========================================' -ForegroundColor DarkCyan
foreach ($line in @(Format-CapabilityHint -ChangedFiles @($levelResult.ChangedFiles) -ProjectDir $ProjectDir)) {
    Write-Host $line
}

# ============================================================
# 摘要
# ============================================================
$totalSeconds = [int]((Get-Date) - $started).TotalSeconds
$minutes = [int]($totalSeconds / 60)
$seconds = $totalSeconds % 60

Write-Host ''
Write-Host '========================================' -ForegroundColor Green
Write-Host "  dev:finish 完成！总耗时 $minutes 分 $seconds 秒" -ForegroundColor Green
Write-Host "  级别: $levelIcon $detectedLevel" -ForegroundColor Green
if ($deployResult) { Write-Host "  部署: $(if ($deployResult.Deployed) { '已到设备' } else { '未部署' })" -ForegroundColor Green }
Write-Host '========================================' -ForegroundColor Green

if ($evidenceResult -and $evidenceResult.Generated) {
    Write-Host ''
    Write-Host '📋 验收证据已生成，粘贴到 PR 正文的 ## 验收证据 段：' -ForegroundColor Cyan
    Write-Host $evidenceResult.Content
}
elseif ($detectedLevel -eq 'quick') {
    Write-Host ''
    # 2026-09-16（#1037）：旧文案「免（低风险运行时面：仅 .uvue 样式/文案改动）」把两个口径混成一个
    # （「低风险运行时面」在 ADR-0008 里恰是「**无 .uvue**」的 .uts 改动），对 `.uvue` 改动照它写正文
    # 会被 `pr-evidence` 判红。建议句的**单点真源**放在 level-detect.ps1 的 Get-QuickEvidenceHint
    # （纯函数、有运行期断言 G7），这里只负责打印。
    Write-Host "📝 快速模式（本地）：$(Get-QuickEvidenceHint -ChangedFiles @($levelResult.ChangedFiles))" -ForegroundColor Gray
}

exit 0
