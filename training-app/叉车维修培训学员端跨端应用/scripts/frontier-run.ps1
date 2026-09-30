<#
.SYNOPSIS
    串行轮转外层：取一张移动端票 → 建可信树 → headless agent 写码 → 复跑 ③ → 调门。

.DESCRIPTION
    #1435。**一次一张票、串行**，不是并行 —— HBuilderX 是单实例资源（本侧 AGENTS.md
    「并发会话纪律」：「这是架构约束，不是可以调优的缺陷」）。

    分工是这份脚本唯一的设计要点（判据在 lib/frontier.ps1 的 Get-FrontierRunPlan）：

      · worktree   由**外层**建，且 cwd 钉在主树 —— 在别的树里调闸门会把新树建进那棵树。
      · agent      **只写码 + 只跑 ③**，计划里不含任何门脚本。本机可用的 agent CLI 没有
                   权限档位 ⇒ 「agent 会跑哪些命令」不能靠约束它，只能靠**不交给它**（#1435 ②）。
      · self-test  外层**复跑** ③，不信 agent 的自述。
      · gate       由**外层**调 `dev-finish.ps1`（④c → 真运行 → 只截改动页 → 证据 → 还原 json）。

    退出码分工（#1435 ⑤ 第三条）：`StopOn = 2` 的步骤返 2 ⇒ **整个轮转停**。
    `dev-finish` 的 2 是「环境不可用」（忙 / 超时 / 无设备），不是「这张票做坏了」；
    带着同一个坏环境继续取下一张，只会连续撞同一堵墙，还会把上一棵树的常驻会话留在原地。

    -DryRun 在**取票之前**就返回：它不联网、不调 gh、不执行任何一步。
    「看一眼今天会跑什么」这件事必须是零副作用的，否则它不会被人用。

.EXAMPLE
    # 零副作用：看计划（不需要票号，票号只是计划里的占位）
    pwsh scripts/frontier-run.ps1 -DryRun -Ticket 1499

.EXAMPLE
    # 真跑一张（需要设备与 HBuilderX 空闲；第一张必须人守着）
    pwsh scripts/frontier-run.ps1 -Ticket 1422 -Device 192.168.0.212:37611

.EXAMPLE
    # 自动取票（取不到会停并说明每张为什么落选，不猜票）
    pwsh scripts/frontier-run.ps1 -DryRun
#>
[CmdletBinding()]
param(
    # 不传即走「自动取票」。注意：取票需要联网调 gh；-DryRun + 不传 -Ticket 时只打印
    # 取票这一步、不执行它，这样「看计划」仍然零副作用。
    [int]$Ticket,
    [ValidateSet('quick', 'standard', 'full')][string]$Level = 'standard',
    [string]$Device,
    [string]$RepoRoot,
    [string]$AgentExe = 'pi',
    [switch]$DryRun,
    # 自动取票的**人工确认门**。判据层拿不到「这是不是波级母票」（本仓未启用 sub-issues，
    # gh 的 parent 恒 null，父子关系只在正文；母票本身不带 `Part of`）⇒ 自动选出的票
    # 必须先让人看一眼票面再跑。带 `-Ticket` 时不重复确认：人已指定＝人已负责。
    [switch]$i
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

. (Join-Path $PSScriptRoot 'lib\frontier.ps1')
. (Join-Path $PSScriptRoot 'lib\wt-bootstrap.ps1')

if (-not $RepoRoot) {
    # 仓库根一律从 git 的公共目录反推（与闸门同源）；不从脚本位置上溯，否则在树里调它会算出那个树。
    $RepoRoot = Get-WtMainRoot
}

function Show-Plan($plan, $ticketNumber) {
    Write-Host ""
    Write-Host "=== 轮转计划（票 #$ticketNumber，一次一张、串行）===" -ForegroundColor Cyan
    $i = 0
    foreach ($s in $plan) {
        $i++
        $stop = if ($s.StopOn) { "  返 $($s.StopOn) 即整轮停" } else { '' }
        Write-Host ("  {0}. {1,-10} cwd={2}" -f $i, $s.Name, $s.Cwd)
        Write-Host ("      → {0} {1}{2}" -f $s.File, ($s.Args -join ' '), $stop)
    }
    Write-Host ""
    Write-Host "  刻意不含：git push / gh pr / gh issue / ①b 签收 —— AFK 终点是本票分支上的本地提交。" -ForegroundColor DarkGray
}

# ---------- -DryRun：在取票之前返回，零副作用 ----------
if ($DryRun) {
    if ($Ticket) {
        # 纪律路径在这里**只打印、不落盘** —— 真跑时它由下面的落盘步生成。
        # 用一个可见的占位段而不是假造真实路径：DryRun 自称零副作用，就不该偷偷写一个 TEMP 文件。
        $placeholder = Join-Path ([System.IO.Path]::GetTempPath()) "fl-frontier-$Ticket-discipline.txt"
        $plan = Get-FrontierRunPlan -RepoRoot $RepoRoot -TicketNumber $Ticket -Level $Level -Device $Device -AgentExe $AgentExe -DisciplineFile $placeholder
        Show-Plan $plan $Ticket
        Write-Host "  （纪律文件在 DryRun 里只算路径不写盘；真跑时由落盘步生成，内容＝Get-AfkAgentDiscipline）" -ForegroundColor DarkGray
    }
    else {
        Write-Host ""
        Write-Host "=== -DryRun 且未指定 -Ticket：只打印取票这一步，不联网执行 ===" -ForegroundColor Cyan
        Write-Host "  → gh issue list --state open --label ready-for-mobile-agent --json number,state,labelList,assignees"
        Write-Host "  → 逐张过 Get-FrontierVerdict（blocked_by 走 API 读数，读不出来判 blocked_unknown 而非放行）"
        Write-Host "  → 取升序里第一张 eligible；其余逐条记原因"
        Write-Host "  想看具体计划：再加 -Ticket <票号>（那仍是零副作用，只是不联网）"
    }
    Write-Host ""
    Write-Host "DRY_RUN executed=0 side_effects=none" -ForegroundColor Green
    exit 0
}

# ---------- 取票 ----------
$ticketNumber = $Ticket
$rejected = @()
if (-not $ticketNumber) {
    Write-Host "[1/5] 取票（gh 读数）..." -ForegroundColor Cyan
    # 形状现测（2026-09-30，本机 gh）：`gh issue list --json blockedBy` 给的是 GraphQL 形
    # `{"blockedBy":{"nodes":[{number,state,title,url}],"totalCount":N}}` —— **不是** REST
    # dependencies 端点那个扁平的 `{"blocked_by":[...]}`。判据层（Get-FrontierVerdict）要的是
    # 扁平的 `@(@{number;state})`，所以**只在这一层剥 nodes**，别让形状泄漏进判据。
    # ⚠️ 空 nodes ⇒ 必须落成**空数组**（= 确证无阻塞），不能落成 $null（= 读不出来）：
    #    后者会让所有干净票一律判 blocked_unknown、取票面永远选不出票（FR6 钉的就是这一条）。
    $raw = gh issue list --state open --label ready-for-mobile-agent --json number,title,state,labels,assignees,blockedBy |
        ConvertFrom-Json
    $titles = @{}
    $candidates = @($raw | ForEach-Object {
        $nodes = @()
        if ($_.blockedBy -and @($_.blockedBy.nodes).Count -gt 0) {
            $nodes = @($_.blockedBy.nodes | ForEach-Object { [pscustomobject]@{ number = $_.number; state = $_.state } })
        }
        $titles[[string]$_.number] = $_.title
        [pscustomobject]@{
            number    = $_.number
            state     = $_.state
            labels    = @($_.labels | ForEach-Object { $_.name })
            assignees = @($_.assignees)
            blockedBy = $nodes
        }
    })
    $sel = Select-FrontierTicket -Candidates $candidates
    $ticketNumber = $sel.Number
    $rejected = @($sel.Rejected)
    if ($rejected.Count -gt 0) {
        foreach ($r in $rejected) { Write-Host ("  落选 #{0} → {1}" -f $r.Number, $r.Reason) -ForegroundColor DarkGray }
    }
    if ($null -eq $ticketNumber) {
        Write-Host "取不到可跑的票（候选里没有 eligible）。**不猜票、不降格判据** —— 停。" -ForegroundColor Yellow
        exit 0
    }
    # 自动取票 ⇒ 必须人确认。第一次真实取票就把母票 #1420（「移动端课程按积分兑换接入」那张
    # spec/索引形波级票）选成了目标：波级票**没有可实现的终态**，交给 AFK 会白烧一轮真机取证。
    if (-not $i) {
        Write-Host ""
        Write-Host ("AUTO_PICK_CONFIRM required=yes picked=#{0} title='{1}'" -f $ticketNumber, $titles[[string]$ticketNumber]) -ForegroundColor Yellow
        Write-Host "  自动取票只走到这里就停了 —— 判据层无法确证这是叶子票还是波级母票。" -ForegroundColor Yellow
        Write-Host "  确认无误：加 -i 重跑；或直接 -Ticket <票号>（人已指定＝不再重复确认）。" -ForegroundColor Yellow
        exit 0
    }
}

Write-Host ""
Write-Host "=== 本轮目标：票 #$ticketNumber ===" -ForegroundColor Cyan

# 纪律落盘到**仓外**（TEMP）：落进树里就有被 `git add` 进去的风险，而它不该是交付物。
# 这份文件是真源 Get-AfkAgentDiscipline 的产物，不是手写的第二份 —— 判据改一处即生效。
$disciplineFile = Join-Path $env:TEMP "fl-frontier-$ticketNumber-discipline.txt"
Set-Content -LiteralPath $disciplineFile -Value (Get-AfkAgentDiscipline).Text -Encoding utf8
Write-Host ("纪律文件（仓外）：{0}" -f $disciplineFile) -ForegroundColor DarkGray

$plan = Get-FrontierRunPlan -RepoRoot $RepoRoot -TicketNumber $ticketNumber -Level $Level -Device $Device -AgentExe $AgentExe -DisciplineFile $disciplineFile
Show-Plan $plan $ticketNumber

# ---------- 执行 ----------
$total = $plan.Count
$idx = 0
foreach ($s in $plan) {
    $idx++
    Write-Host ""
    Write-Host ("[{0}/{1}] {2}" -f $idx, $total, $s.Name) -ForegroundColor Cyan
    Push-Location $s.Cwd
    try {
        & $s.File @($s.Args)
        $code = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
    Write-Host ("FRONTIER_STEP step={0} exit={1}" -f $s.Name, $code)

    if ($s.StopOn -and "$code" -eq "$($s.StopOn)") {
        Write-Host ""
        Write-Host ("环境不可用（step={0} 返 {1}）⇒ **整轮停**，不取下一张。" -f $s.Name, $code) -ForegroundColor Red
        Write-Host "  这一步的语义是「环境不可用」而不是「这张票做坏了」：带着同一个坏环境继续，只会连续撞同一堵墙，" -ForegroundColor Red
        Write-Host "  并把上一棵树的常驻真运行会话留在原地（项目目录既删不掉也改不了名）。" -ForegroundColor Red
        Write-Host "FRONTIER_STOP reason=env_unavailable step=$($s.Name) exit=$code ticket=$ticketNumber" -ForegroundColor Red
        exit 2
    }
    if ($code -ne 0) {
        Write-Host ("步骤 {0} 返 {1} —— 不继续下一步，交由人判（本轮不自动重试、不跳过门）。" -f $s.Name, $code) -ForegroundColor Yellow
        Write-Host "FRONTIER_STOP reason=step_failed step=$($s.Name) exit=$code ticket=$ticketNumber" -ForegroundColor Yellow
        exit 1
    }
}

Write-Host ""
Write-Host "轮转四步都过了。剩下的是**人的事**，本脚本刻意不做：" -ForegroundColor Green
Write-Host "  · push / 开 PR / 合并 —— 未做（AFK 终点 = 本地提交）"
Write-Host "  · ①b 能力面签收（命中时）—— 执行人栏须人给原文"
Write-Host "  · 逐页截图的「对不对」判定 —— 机检行只判到没到设备 / 页面可达 / 有无 crash"
Write-Host "FRONTIER_DONE ticket=$ticketNumber steps=$total rejected=$($rejected.Count)" -ForegroundColor Green
exit 0
