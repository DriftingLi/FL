#!/usr/bin/env pwsh
<#
.SYNOPSIS
    worktree 初始化入口：共享主树 node_modules（junction）+ 机检「路径段首带点 ⇒ ③ 门假绿」。
.DESCRIPTION
    为什么存在（2026-09-28）：宿主「本地任务的 Worktree 配置」框里那段脚本依赖宿主注入的私有环境
    变量，命令行 / 闸门 / 别的编辑器都拿不到 ⇒ 同一件事每条链路各写一遍、只有宿主那条生效。
    已记过账的后果：worktree 里跑 npm test 报 `'jest' 不是内部或外部命令`。
    ⇒ 逻辑全在 scripts/lib/wt-bootstrap.ps1（被 ③ 门锁住），**框里只填一行调用本入口**；
      主树位置由 git 元数据反推，与用哪个工具无关。
    落 junction 这一步不在单测射程（Windows-only，CI 是 ubuntu）：它的验收判据是「真建一棵树跑通」，
    证据见 PR 正文。
.EXAMPLE
    pwsh scripts/wt-bootstrap.ps1            # 在某个 worktree 里跑（任意子目录均可）
    pwsh scripts/wt-bootstrap.ps1 -DryRun    # 只看计划，不落 junction
.NOTES
    退出码分工（Q5-C + I5，本枚举与 docs/agents/multi-agent-git.md、ADR-0029 的拆分步骤与约束段同一口径）：
      0 = 正常；**也含**「本树非 ③ 可信树但不是闸门建的 ⇒ 仅告知」（宿主树不可信是常态，不是故障）
      2 = 取不到本树根 / 主树，或显式注入的 -Root / -WtRoot 指向不存在的路径
      3 = **仅当 -ExpectEligible**（闸门建的树仍不可信 ⇒ 真故障，必须换树名）
      4 = 落 junction 时 New-Item 抛错（源缺失 / 目标父目录不可写等）——先打红字命名项目 / 目标 / 源再退（否则 $ErrorActionPreference=Stop 下错误记录会直接逸出，pwsh -File 退 1 ———— 一个文档从未列出的码）
#>
[CmdletBinding()]
param(
    [string]$Root,
    [string]$WtRoot,
    [switch]$ExpectEligible,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
# 自身输出也强制 UTF-8：入口打印的指引文案含 CJK 与符号，Windows 默认代码页会把它写坏
# （Task 2 的 W8 要 `toMatch(/静默假绿/)` —— 不预设编码则那条断言在 Windows 上必红）。
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }
. (Join-Path (Join-Path $PSScriptRoot 'lib') 'wt-bootstrap.ps1')

function Fail([string]$msg, [int]$code) {
    Write-Host "[wt-bootstrap] $msg" -ForegroundColor Red
    exit $code
}

$projRoot = Split-Path -Parent $PSScriptRoot
# M8：-Root / -WtRoot 可注入是为可测性，但拼错一个字母会得到一个空计划 + 绿字「③ 可信树」（无输出即过）
#     ⇒ 显式注入时必须先确认路径存在，不拿「不存在」当「空树」默认放行。
if ($WtRoot -and -not (Test-Path -LiteralPath $WtRoot)) { Fail "-WtRoot 指向不存在的路径：$WtRoot" 2 }
if ($Root -and -not (Test-Path -LiteralPath $Root)) { Fail "-Root 指向不存在的路径：$Root" 2 }
# ⚠️ -WtRoot / -Root 可注入是**为可测性**：退出码的分工（Q5-C，2026-09-28 定）必须能被守护锁住，
#    而默认值从 $PSScriptRoot 推 ⇒ 在临时目录里复现不出来。两个都显式给出时本入口不碰 git。
$treeRoot = if ($WtRoot) { $WtRoot } else { Get-WtTreeRoot -Base $projRoot }
if (-not $treeRoot) { Fail '取不到本树根（不在 git 仓库里？或用 -WtRoot 指定）' 2 }
$mainRoot = if ($Root) { $Root } else { Get-WtMainRoot -Base $projRoot }
if (-not $mainRoot -or -not (Test-Path -LiteralPath $mainRoot)) {
    Fail "取不到主树（本树 $treeRoot；可用 -Root 指定）" 2
}

foreach ($i in (Get-WtNodeModulesPlan -MainRoot $mainRoot -WtRoot $treeRoot)) {
    switch ($i.State) {
        'ok'   { Write-Host "ok   $($i.Project)" }
        'broken' {
            Write-Host "broken $($i.Project)（树里有 node_modules 但没有 jest 入口 ⇒ ③ 跑不起来）" -ForegroundColor Red
            Write-Host "[wt-bootstrap]        修法：先判它是不是 junction——若是，按仓规先 `cmd /c rmdir $((Split-Path -Parent $i.Target))` 摘掉联接再删（别 `Remove-Item -Recurse` 跟进主树），再删掉那份坏 node_modules 后重跑本脚本（工具不代删别人的目录）。" -ForegroundColor Red
        }
        'link' {
            if ($DryRun) { Write-Host "link $($i.Project)  (dry-run)" }
            else {
                # I5：New-Item Junction 无局部 try/catch 时，$ErrorActionPreference=Stop 会把终止错误直接抛出去
                #     ⇒ 调用方的 $LASTEXITCODE 分支不会跑、pwsh -File 退 1（无文档列出）、用户只看到裸错误记录。
                #     接住、按项目/目标/源打齐红字再 exit 4（一个枚举里列出的码）。
                try {
                    New-Item -ItemType Junction -Path $i.Target -Target $i.Source | Out-Null
                    Write-Host "link $($i.Project)"
                } catch {
                    Write-Host "link FAILED $($i.Project)（建 junction 抛错 —— 这不是测试坏了）" -ForegroundColor Red
                    Write-Host "[wt-bootstrap]        项目：$($i.Project)" -ForegroundColor Red
                    Write-Host "[wt-bootstrap]        目标：$($i.Target)" -ForegroundColor Red
                    Write-Host "[wt-bootstrap]        源（主树）：$($i.Source)" -ForegroundColor Red
                    Write-Host "[wt-bootstrap]        错误：$($_.Exception.Message)" -ForegroundColor Red
                    Write-Host '[wt-bootstrap]        正解：确认源目录存在、目标父目录可写；或直接在该子工程里 npm ci（本入口不重试、不代删）。' -ForegroundColor Red
                    exit 4
                }
            }
        }
        'refuse' {
            Write-Host "refuse $($i.Project)（本树不是 ③ 可信树 ⇒ 故意**不**共享这份依赖：链上 jest 会扫到 0 套件并报全绿）" -ForegroundColor Red
            Write-Host '[wt-bootstrap]        这不是「测试坏了」，也不是环境缺件 —— 是这条路径下 ③ 不可信。' -ForegroundColor Red
            Write-Host '[wt-bootstrap]        正解：用 new-worktree.ps1 建 wt-<票号>，在那棵树里跑。' -ForegroundColor Red
        }
        'skip' { Write-Host "skip $($i.Project)（主树未装：$($i.Source)）" -ForegroundColor Yellow }
    }
}

$gate3ok = Test-WtPathSafe -Path $treeRoot
if ($gate3ok) {
    Write-Host "[wt-bootstrap] 主树 $mainRoot；本树是 ③ 可信树。" -ForegroundColor Green
    exit 0
}
Write-Host '[wt-bootstrap] ⚠️ 本树路径含「段首为 . 或 glob 元字符」的目录段 ⇒ ③ 门 --listTests 会匹配 0 套件且 exit 0（静默假绿）' -ForegroundColor Red
Write-Host "[wt-bootstrap]              本树：$treeRoot" -ForegroundColor Red
Write-Host '[wt-bootstrap]              修法：git worktree move 到无点路径；移动端交付一律走 new-worktree.ps1 建 wt-<票号>。' -ForegroundColor Red
if ($ExpectEligible) {
    Write-Host '[wt-bootstrap] 本树由闸门创建 ⇒ 起名时就该可信 ⇒ 判失败（exit 3）。' -ForegroundColor Red
    exit 3
}
Write-Host '[wt-bootstrap] 以上为**告知**：非闸门树不可信是常态（宿主树恒如此）⇒ 不判失败，exit 0。' -ForegroundColor Yellow
exit 0
