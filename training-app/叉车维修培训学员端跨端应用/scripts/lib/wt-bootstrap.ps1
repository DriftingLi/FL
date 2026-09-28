<#
.SYNOPSIS
    worktree 初始化的**纯逻辑真源**：反推主树、枚举需要依赖的子工程、判路径是否会让 ③ 门假绿。
.DESCRIPTION
    本文件不落任何 junction、不写任何文件 —— 副作用全在入口 scripts/wt-bootstrap.ps1。
    这么分的理由是 ③ 门跑在 ubuntu-latest（CI mobile-test job）：守护要能 dot-source 本文件并跨平台
    断言判据，而 New-Item -ItemType Junction 是 Windows-only。
    形态与 scripts/lib/contract-tests.ps1 一致（真源可 dot-source，被运行期守护锁住）。
    token: wtBootstrap → utils/wtBootstrapBehavior.test.js
#>

function Get-WtMainRoot {
    <#
      主树根。判据 = git rev-parse --git-common-dir（对所有 worktree 指向同一处）。
      ⚠️ 不能从 $PSScriptRoot 上溯：那会得到**调用方所在的那棵树**（new-worktree.ps1「仓库根与主工作树」小节的血账）。
      ⚠️ new-worktree.ps1 的仓库根解析已 dot-source 本函数（不再各留一份 --git-common-dir 块）⇒ 本函数是「反推主树」的唯一实现。
      ⚠️ 不读宿主注入的私有环境变量：命令行 / 闸门 / 别的编辑器都没有它 —— 本文件存在的理由，
        就是把「主树在哪」收成一条与工具无关的判据。
    #>
    [OutputType([string])]
    param([string]$Base = (Get-Location).Path)
    $common = (& git -C $Base rev-parse --git-common-dir 2>$null | Select-Object -First 1)
    if (-not $common) { return $null }
    $common = $common.Trim()
    if (-not [System.IO.Path]::IsPathRooted($common)) { $common = Join-Path $Base $common }
    return (Split-Path -Parent (Resolve-Path -LiteralPath $common).Path)
}

function Get-WtTreeRoot {
    <# 当前**这一棵树**的根（主树或 worktree 都成立）：git rev-parse --show-toplevel。 #>
    [OutputType([string])]
    param([string]$Base = (Get-Location).Path)
    $top = (& git -C $Base rev-parse --show-toplevel 2>$null | Select-Object -First 1)
    if (-not $top) { return $null }
    return (Resolve-Path -LiteralPath $top.Trim()).Path
}

function Get-WtNodeProjects {
    <#
      需要 node_modules 的子工程，返回**相对仓库根**、正斜杠分隔的路径段。
      判据 = 「该目录下有 jest.config.unit.js」，**不写死项目名**：本文件会被宿主的启动配置框调用，
      框里的文本经宿主编码落盘时中文段有被写坏的风险；从文件系统读回来的 Name 不受影响。
      射程：只枚举移动端 —— Web 前端与后端不由本文件管（不枚举 `frontend`）。
    #>
    [OutputType([string[]])]
    param([string]$Base = (Get-Location).Path)
    $list = New-Object System.Collections.Generic.List[string]
    $mobile = Join-Path $Base 'training-app'
    if (Test-Path -LiteralPath $mobile) {
        foreach ($d in Get-ChildItem -LiteralPath $mobile -Directory) {
            if (Test-Path -LiteralPath (Join-Path $d.FullName 'jest.config.unit.js')) {
                $list.Add('training-app/' + $d.Name)
            }
        }
    }
    return @($list | Select-Object -Unique)
}

function Test-WtPathSafe {
    <#
      路径中是否**不含**「以 . 或 glob 元字符开头的目录段」。含 ⇒ $false（③ 门会静默匹配 0 套件）。
      机制（#1144，Windows-only）：jest-util 的 replacePathSepForGlob =
        path.replace(/\\(?![{}()+?.^$])/g, '/')
      它**故意不转换**后跟 {}()+?.^$ 的反斜杠 ⇒ 该段前的分隔符留成 '\' ⇒ picomatch 把 '\.' 读成
      转义的 '.'、分隔符随之消失 ⇒ testMatch 的绝对 glob 永不匹配真实路径。
      ⚠️ 判据不能拿 exit code：实测点父段下 --listTests 空输出且 exit 0（**静默假绿**）。
      段首才是判据，段中出现 . ( ) 等不触发（实测 wt-(wip)1185 → 104 套件正常）。
    #>
    [OutputType([bool])]
    param([string]$Path)
    return -not ($Path -match '[\\/][{}()+?.^$]')
}

function Get-WtJestEntryPath {
    <#
      一份 node_modules 里 ③ 用的 jest 入口文件**路径**（不判在不在，只拼路径）。
      `jest/bin/jest.js`（平台无关，不带 .cmd）——与守护自己拉 jest 的方式同源。
      收成函数而非散落的 Join-Path 字面量：入口、闸门（new-worktree.ps1 跑 --listTests 用的 jest 二进制）
      与下面的 Test-WtJestEntry 都取同一个真源，避免「jest 入口在哪」再次分叉成两份。
    #>
    [OutputType([string])]
    param([string]$NodeModules)
    return Join-Path (Join-Path $NodeModules 'jest') (Join-Path 'bin' 'jest.js')
}

function Test-WtJestEntry {
    <#
      一份 node_modules 能不能跑 ③ —— 判据就是 ③ 自己用的那个入口文件在不在（路径取 Get-WtJestEntryPath）。
      为什么需要它：只看目录存在，会把「一次失败的 npm ci 留下的空壳」或「指向已删主树的
      失效 junction」判成已装好 ⇒ 静默复发本计划要消灭的那个症状。
    #>
    [OutputType([bool])]
    param([string]$NodeModules)
    return Test-Path -LiteralPath (Get-WtJestEntryPath -NodeModules $NodeModules)
}

function Get-WtNodeModulesPlan {
    <#
      初始化计划（只读，不落盘）。每条：Project / State / Target / Source
        ok     = 树里已有 node_modules **且 jest 入口在位**（真装或已 link）→ 入口不动它
        broken = 树里有 node_modules 但**没有 jest 入口**（一次失败的安装残留空壳 / 指向已删主树的失效 junction）
                 → 入口打红字告知「删掉它再重跑」，**不自行删别人的东西**；链上就等于静默复发要消灭的那个症状
        refuse = 本树不是 ③ 可信树 → 故意**不链**。
                 链上就等于把「jest 起得来、扫到 0 套件、报全绿」的假绿装配好；
                 不链则 npx jest 直接命令找不到 = fail-loud（Q2 于 2026-09-28 定）。
        link   = 主树有、树里没有，且本树 ③ 可信 → 入口去建 junction（Source 是主树那一份）
        skip   = 主树也没有 → 入口打印「先在主树 npm ci」（Source 仅用于把缺的东西说清楚）
      主树缺依赖**不抛错**：入口要继续跑完点段判定并给出指引。
    #>
    [OutputType([object[]])]
    param(
        [Parameter(Mandatory = $true)][string]$MainRoot,
        [string]$WtRoot = (Get-Location).Path
    )
    $sep = [System.IO.Path]::DirectorySeparatorChar
    $gate3ok = Test-WtPathSafe -Path $WtRoot
    $plan = foreach ($p in (Get-WtNodeProjects -Base $WtRoot)) {
        $rel = ($p -replace '/', [string]$sep) + [string]$sep + 'node_modules'
        $src = Join-Path $MainRoot $rel
        $dst = Join-Path $WtRoot $rel
        # 真源只枚举移动端 ⇒ 每一项都受 ③ 判据约束，**不需要再设 $isMobile 这种恒真判据**
        # （按 #1185 的口径：默认命名下恒不触发的分支是死代码，宁可不写）
        $dstThere = Test-Path -LiteralPath $dst
        # ⚠️ 命令调用作 -and 右操作数必须加括号（实测 PowerShell 7.5：裸写法直接 ParserError，
        #    「必须在-and运算符后提供值表达式」）—— 语义与 brief 一致：优先级 ok > broken > refuse > link > skip
        $state = if ($dstThere -and (Test-WtJestEntry -NodeModules $dst)) { 'ok' }
                 elseif ($dstThere) { 'broken' }
                 elseif (-not $gate3ok) { 'refuse' }
                 elseif (Test-Path -LiteralPath $src) { 'link' }
                 else { 'skip' }
        [pscustomobject]@{ Project = $p; State = $state; Target = $dst; Source = $src }
    }
    return @($plan)
}
