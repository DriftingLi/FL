<#
.SYNOPSIS
    串行轮转外层的两块判据真源：取票判据 + agent 纪律清单（纯逻辑，无副作用）。

.DESCRIPTION
    为什么需要这个文件（#1435）：
      「挂机跑一张移动端票」的那条链**仓里已经有了** —— `scripts/dev-finish.ps1` 覆盖
      级别判定 → ④c → 真运行 → 只截改动页 → 对比 → 证据 → 还原 manifest/pages。
      外层只补三件事：取票、起一次 headless agent、在 agent 之后调 `dev-finish`。
      其中最贵的两个判断都是纯逻辑，且都**静默**：

      1) 取票判错。跑了一张 `blocked_by` 未清的票 ⇒ 改的是中间态（#1423 票面自己就写了
         「扫的是前序票完成后的终态形状，必须等全部功能票合并」）；或抢了别人已认领的票
         （`multi-agent-git.md` 记的互踩形态）。⇒ 判据必须 **fail-closed**：
         「读不出阻塞」判 `blocked_unknown` 而**不是**当「没阻塞」放行 —— 取不到票只是空转，
         取错票要赔一整批返工。

      2) 纪律清单写漏。本机可用的 agent CLI **没有任何权限档位**（`pi` 的选项面里没有
         permission 一类，见 #1435 ②）⇒ 「agent 会跑哪些命令」不能靠约束它，只能靠
         **不交给它**。所以这份清单是判据，不是提醒：允许面极窄（写码 + `npm run test:unit`
         + 本票文件的 add/commit），禁止面逐条点名真实存在的脚本。

    刻意不在这里做的事（射程）：不取票、不建树、不起 agent、不跑任何门。
    副作用那一步的验收判据是「真跑一张票跑通」，由带真机证据的那个 PR 给（#1435 ⑤）——
    在本文件里假称「已经能挂机」正是 release.md:25 记的那类「什么都没做却报成功」。

.NOTES
    守护：utils/frontierBehavior.test.js（FR1–FR5，token `frontier`）。
    只被 dot-source；不要直接 `pwsh -File` 跑它（它没有入口段）。
#>

function Get-FrontierVerdict {
    <#
      .SYNOPSIS
          一张候选票能不能被外层取走。返回 @{ Verdict; Reason; Number }。
      .DESCRIPTION
          Reason 是判据的枚举面，测试逐条钉住；新增分支必须同时加用例。
          fail-closed：任何「读不出来」都判不可取，不判放行。
          判据一律走 $Ticket.PSObject.Properties['<名>'] —— 不用嵌套函数、
          不用 `字段 -or 默认值`（`-or` 会把空串 / 0 / $false 一并吞掉，那是「缺字段即放行」的藏身处）。

          ⚠️ **本函数判不出「波级 issue（母票）」，这是已知边界，不是待补的洞**：
          本仓没启用 sub-issues ⇒ gh 的 `parent` 字段对 #1420 / #1422 / #1424 **全返 null**
          （2026-09-30 现测），父子关系只写在正文（`Part of #1420`）；而母票的两条形态
          （spec 形 / 索引形）**都不带** `Part of` ⇒ 「正文里有没有父」这种判据既挡不住母票、
          又会把独立票一起挡掉。所以确认归**取数面**：`frontier-run.ps1` 自动取票必须带 `-i`
          逐条打印候选票号 + 标题让人确认，显式 `-Ticket` 时不重复确认（人已指定=人已负责）。
          曾在本轮写过一条 `body` 标记判据，**已删** —— 它给的是「挡住了一件事」的错觉。
      .OUTPUTS
          [pscustomobject] @{ Verdict = 'eligible' | 'ineligible'; Reason = <枚举>; Number }
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)] $Ticket
    )

    function New-Verdict([string]$verdict, [string]$reason, $number) {
        return [pscustomobject]@{ Verdict = $verdict; Reason = $reason; Number = $number }
    }

    $props = @($Ticket.PSObject.Properties.Name)
    $number = if ($props -contains 'number') { $Ticket.number } else { $null }

    # 1) 状态必须读得出，且必须是 OPEN。
    if (-not ($props -contains 'state')) { return New-Verdict 'ineligible' 'state_missing' $number }
    $state = [string]$Ticket.state
    if ([string]::IsNullOrWhiteSpace($state)) { return New-Verdict 'ineligible' 'state_unknown' $number }
    if ($state -ne 'OPEN') { return New-Verdict 'ineligible' 'closed' $number }

    # 2) 必须是移动端那张 label。`ready-for-agent` 是后端/前端票（triage-labels.md 的两套字符串），
    #    用它取票会把外层的 agent 派到非移动端承载面上去 —— 那个面的判据本文件管不到。
    $labels = @()
    if ($props -contains 'labels') { $labels = @($Ticket.labels) }
    if ($labels -notcontains 'ready-for-mobile-agent') { return New-Verdict 'ineligible' 'not_labelled' $number }

    # 3) 已认领不取（一会话一票）。字段读不出来也判不可取。
    if (-not ($props -contains 'assignees')) { return New-Verdict 'ineligible' 'assignees_missing' $number }
    if (@($Ticket.assignees).Count -gt 0) { return New-Verdict 'ineligible' 'assigned' $number }

    # 4) 阻塞必须**确证**已清。读不出来 / 字段缺了 ⇒ 不取（不是「当作没阻塞」）。
    if (-not ($props -contains 'blockedBy')) { return New-Verdict 'ineligible' 'blocked_unknown' $number }
    $raw = $Ticket.blockedBy
    if ($null -eq $raw) { return New-Verdict 'ineligible' 'blocked_unknown' $number }
    # **空集合是「确证无阻塞」，不是「读不出来」**。这一条是本轮 TDD 抓出来的真 bug：
    # 上一版在这里用 `if ($b) {…} else {$null}` 组装读数，把 `blocked_by: []` 折成 $null，
    # 于是「没有任何阻塞依赖」的票反而一律判 blocked_unknown ⇒ 取票面永远选不出票。
    # fail-closed 的边界要划在「读不出来」，不能顺手把「读到了空」一起关掉。
    $blockers = @($raw)
    if ($blockers.Count -eq 0) { return New-Verdict 'eligible' '' $number }
    foreach ($b in $blockers) {
        if ($null -eq $b) { return New-Verdict 'ineligible' 'blocked_unknown' $number }
        # ⚠️ 属性探测**不能用** `$b.PSObject.Properties.Name -contains 'state'`：
        #    hashtable（`@{...}`）造出来的对象没有 PSObject 属性面，那个判据会把**合法的**阻塞读数
        #    误判成「读不出来」（本轮 FP7 实测红在此）。统一走值判据 —— dot 取值对
        #    PSCustomObject / hashtable / ConvertFrom-Json 三态都成立；缺属性的情况在
        #    Set-StrictMode 下取到 $null，落进下面这条保守分支。
        $state = [string]$b.state
        if ([string]::IsNullOrWhiteSpace($state)) { return New-Verdict 'ineligible' 'blocked_unknown' $number }
        # 只有明确已离开 open 的才算清；其余（含未知态）一律视为仍在阻塞 —— 保守一侧。
        if (@('CLOSED', 'MERGED') -notcontains $state) { return New-Verdict 'ineligible' 'blocked' $number }
    }

    return New-Verdict 'eligible' '' $number
}

function Get-AfkAgentDiscipline {
    <#
      .SYNOPSIS
          交给 headless agent 的那份纪律。返回 @{ Allow; Deny; Text }。
      .DESCRIPTION
          Allow / Deny 是判据面（测试逐条比对、并锁「两侧不得重叠」）；
          Text 是喂给 `--append-system-prompt` 的成品文本。
          被点名的脚本必须真实存在 —— 指到不存在的名字，那条禁令就是空的。
      .OUTPUTS
          [pscustomobject] @{ Allow = string[]; Deny = string[]; Text = string }
    #>
    [CmdletBinding()]
    param()

    $allow = @(
        '写代码与本票射程内的文本/样式/逻辑改动',
        'npm run test:unit',
        'git add <本票文件>',
        'git commit'
    )

    $deny = @(
        'hx-run.ps1',
        'dev-finish.ps1',
        'kotlin-all-check.ps1',
        'compile-check.ps1',
        'mp-weixin-check.ps1',
        'auto-screenshot.ps1',
        'device-capture.ps1',
        'emulator-smoke.ps1',
        'cli.exe / cli.bat',
        'Stop-Process / taskkill',
        'git push / gh issue / gh pr',
        'pages.json',
        'manifest.json',
        'platformConfig.json'
    )

    $textLines = @(
        '你正处在一个**无人值守**的移动端会话里。射程只有：写代码、`npm run test:unit`、本票文件的逐文件 `git add` 与 `git commit`。',
        '',
        '不得执行（无例外，也别试探）：',
        '  · 任何 HBuilderX 相关调用：`scripts/hx-run.ps1`、`scripts/dev-finish.ps1`、`scripts/kotlin-all-check.ps1`、',
        '    `scripts/compile-check.ps1`、`scripts/mp-weixin-check.ps1`、`scripts/lib/auto-screenshot.ps1`、',
        '    `scripts/lib/device-capture.ps1`、`scripts/emulator-smoke.ps1`、`cli.exe`、`cli.bat`。',
        '    门与设备步骤**由外层脚本在你结束后跑**，不属于你的射程。',
        '  · 结束任何进程：`Stop-Process` / `taskkill`。看到编译「正在编译中…」**不是**你可以清理的东西：',
        '    外层会等，等到上限后 `exit 2`（环境不可用）。强杀主程序是红线（ADR-0008），`hx-run.ps1` 自己也不含任何强杀调用。',
        '  · `git push`；`gh issue` / `gh pr` 等任何 `gh` 子命令（取票与签收都由外层做）。',
        '  · `git add -A` / `git add .`（只准逐文件 `git add`，工作区里常混着别的会话的在飞改动）。',
        '  · 改 `pages.json` / `manifest.json` / `platformConfig.json`（它们是运行时面判据源，且会被 HBuilderX 回写；',
        '    `dev-finish` 的「还原 manifest/pages」那一步不由你承担）。',
        '',
        '完成判定：`npm run test:unit` 全绿 + 一次 `git commit`。做完就停。',
        '拿不到结论时（需要真机效果、需要人签收、判据不清）：**停下来什么都不做**，不要为「推进」而去碰上面的东西。'
    )

    return [pscustomobject]@{
        Allow = $allow
        Deny  = $deny
        Text  = ($textLines -join [Environment]::NewLine)
    }
}

function Get-FrontierRunPlan {
    <#
      .SYNOPSIS
          一张票的轮转计划：**纯数据，不执行任何东西**。
      .DESCRIPTION
          四步固定顺序，每步是「可比较的数据」而不是拼好的字符串 —— 这样测试能直接
          断言「agent 那一步里没有 dev-finish」「建树那一步的 cwd 是主树」。
          把判据写成字符串拼接，测试就只能正则，正则判不住「顺序」与「归属」。

          步骤归属的分工是这份计划的核心（#1435 ②）：
            worktree   —— 由外层建（必须在主树 cwd 下调闸门，见 new-worktree.ps1 的嵌树血账）
            agent      —— 只写码 + 只跑 ③；**没有任何门**（本机 agent CLI 无权限档位 ⇒ 不交给它）
            self-test  —— 外层复跑 ③，不信 agent 的自述
            gate       —— 由外层调 dev-finish（占 HBuilderX 与设备；下一步 2 在 agent 之后）
      .OUTPUTS
          [pscustomobject[]] 每项 @{ Name; File; Cwd; Args; StopOn }
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string]$RepoRoot,
        [Parameter(Mandatory = $true)][int]$TicketNumber,
        [ValidateSet('quick', 'standard', 'full')][string]$Level = 'standard',
        [AllowNull()][AllowEmptyString()][string]$Device,
        # 'none' = 写码这一格交回人/会话（暂停步）；'pi' 等 = headless CLI 执行该步。
        # 本机现状：pi 的密钥失效（实调 401，而 auth check 报 ready ⇒ 状态不可信），
        # 所以 none 不是退路而是默认档位 —— 外层价值在取票/建树/门/收尾，不在起 agent。
        [string]$AgentExe = 'none',
        # 纪律文件路径：AgentExe -ne 'none' 时必填（无纪律=无约束的 AFK agent，宁可组不出计划）；
        # none 档下可为空 —— 那时纪律由入口段直接打在终端给人读。
        [string]$DisciplineFile
    )

    # ⚠️ 路径必须**逐段** Join-Path：`Join-Path $Root 'a\b\c'` 在 Windows 上能过，
    #    在 ubuntu CI 上直接抛（子路径含反斜杠 = 非法字符）—— 本仓 ③ 门跑在 ubuntu，
    #    这条是 CI 实测红出来的（frontier.ps1:199，本地全绿、CI 判红，本地测不到）。
    $mobileProject = 'training-app/叉车维修培训学员端跨端应用'   # 正斜杠：两平台都是合法相对段
    $gateRoot = Join-Path (Join-Path $RepoRoot $mobileProject) 'scripts'
    $tree = Join-Path $RepoRoot "wt-$TicketNumber"
    $treeProject = Join-Path $tree $mobileProject

    # 'none' 档没有可执行体，纪律由入口段打给人读 —— 不再为它要求纪律文件。
    if (($AgentExe -ne 'none') -and [string]::IsNullOrWhiteSpace($DisciplineFile)) {
        throw "Get-FrontierRunPlan: AgentExe='$AgentExe' 时 -DisciplineFile 必填。没有纪律的计划=一个无约束的无人值守 agent，宁可组不出计划。"
    }

    $prompt = "按 issue #$TicketNumber 在本树实现并自检。纪律见 --append-system-prompt 注入的那份清单：只写代码、只跑 npm run test:unit，不得调用任何 HBuilderX 相关脚本，做完提交到当前分支即停。"

    $steps = @()

    # 1) 建树 —— cwd 必须是主树：在别的树里调闸门会把新树建进那棵树（实测建出 wt-1185\wt-(wip)1185）。
    $steps += [pscustomobject]@{
        Name   = 'worktree'
        File   = (Join-Path $gateRoot 'new-worktree.ps1')
        Cwd    = $RepoRoot
        Args   = @('-Task', "$TicketNumber")
        StopOn = '2'
    }

    # 2) 写码 —— 'none' ⇒ 暂停步（File 为空，入口打印指令后等回车）；
    #    CLI 档 ⇒ 纪律随会话注入，这一步里没有门。
    #    ⚠️ CLI 档的 --append-system-prompt 接的是**纪律文件路径**，不是本仓真源 .ps1
    #    （塞源码路径，agent 读到的是函数定义不是禁令 —— DryRun 实测过，「参数非空」判不住）。
    if ($AgentExe -eq 'none') {
        $steps += [pscustomobject]@{
            Name   = 'agent'
            File   = ''
            Cwd    = $treeProject
            Args   = @('写码步：在本目录实现本票并 git commit，完成后按 Enter 继续（门与收尾由外层接着跑）')
            StopOn = ''
        }
    }
    else {
        $steps += [pscustomobject]@{
            Name   = 'agent'
            File   = $AgentExe
            Cwd    = $treeProject
            Args   = @('--print', '--append-system-prompt', $DisciplineFile, '--', $prompt)
            StopOn = ''
        }
    }

    # 3) 自检 —— 外层复跑 ③，不信 agent 的自述（不占设备、不取锁）。
    $steps += [pscustomobject]@{
        Name   = 'self-test'
        File   = 'npm'
        Cwd    = $treeProject
        Args   = @('run', 'test:unit')
        StopOn = ''
    }

    # 4) 门 —— 只有这一步能调 dev-finish；无设备时**不带** -Device（不凭空造设备串）。
    $gateArgs = @('-Level', $Level)
    if ($Device) { $gateArgs += @('-Device', $Device) }
    $steps += [pscustomobject]@{
        Name   = 'gate'
        File   = (Join-Path $gateRoot 'dev-finish.ps1')
        Cwd    = $treeProject
        Args   = $gateArgs
        StopOn = '2'
    }

    return $steps
}

function Select-FrontierTicket {
    <#
      .SYNOPSIS
          从候选票里取第一张**合格**的（升序），其余逐条记原因。
      .DESCRIPTION
          纯函数：候选由调用方从 tracker 取来（取数面属入口，不在这层 —— 否则本函数
          要联网，`-DryRun` 就无法在取票之前返回）。
          空候选 ⇒ Number 为 $null，**不猜票**：取不到票是「停」，不是「随便挑一张」。
          落选原因逐条留痕，是为了让「今天为什么没跑 1423」这件事可回答 ——
          沉默地跳票是轮转最容易被质疑却又最难自证的地方。
      .OUTPUTS
          [pscustomobject] @{ Number; Rejected = @(@{ Number; Reason }) }
    #>
    [CmdletBinding()]
    param(
        [AllowEmptyCollection()][object[]]$Candidates = @()
    )

    $sorted = @($Candidates | Sort-Object { [int]$_.number })
    $rejected = @()
    $picked = $null
    foreach ($t in $sorted) {
        $v = Get-FrontierVerdict -Ticket $t
        if ($v.Verdict -eq 'eligible') {
            if ($null -eq $picked) { $picked = $t.number }
            else { $rejected += [pscustomobject]@{ Number = $t.number; Reason = 'superseded' } }
        }
        else {
            $rejected += [pscustomobject]@{ Number = $t.number; Reason = $v.Reason }
        }
    }
    return [pscustomobject]@{ Number = $picked; Rejected = $rejected }
}
