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
    foreach ($b in @($raw)) {
        if ($null -eq $b) { return New-Verdict 'ineligible' 'blocked_unknown' $number }
        if (-not ($b.PSObject.Properties.Name -contains 'state')) { return New-Verdict 'ineligible' 'blocked_unknown' $number }
        # 只有明确已离开 open 的才算清；其余（含未知态）一律视为仍在阻塞 —— 保守一侧。
        if (@('CLOSED', 'MERGED') -notcontains [string]$b.state) { return New-Verdict 'ineligible' 'blocked' $number }
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
