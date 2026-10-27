<#
.SYNOPSIS
    「无代码变更就别再上一遍真机」的纯判定（hx-run.ps1 取锁前的预检，#2027）。

.DESCRIPTION
    **起因（实测数据，不是推测）**：`.ci-verify` 里累积的 12 条 `HX_RUN` 机检行显示——

      正常增量轮次：compile 40–130 秒、total **45–287 秒**（中位 73 秒）；
      3/12 次 `exit=env`：**643 / 906 / 906 秒**，合计 2455 秒 = 总墙钟 4514 秒的 **54%**。

    其中两次 `deployed=false` 的日志里，App 仍然在设备上每 30 秒轮询
    `/api/notifications/unread-count` ⇒ **部署其实成立**，只是 `www` 目录 mtime 没前进
    ⇒ 判据判「未部署」⇒ 等满 `-TimeoutSeconds`（900 秒）。

    根因不是编译慢，是**代码没变又跑了一遍**（脚本自己的提示语里也列了这条：
    「距上次部署无代码变更 ⇒ 保守起见一律判未部署」）。本文件只回答一个问题：

      **上一轮成功部署之后，HEAD 与工作树是否都没变？**
        变了 ⇒ 该跑（Noop = $false）
        没变 ⇒ 别跑（Noop = $true）

    **判据（只用 git，不碰 cli、不碰 adb、不取锁）**：
      标记文件里的 `headSha` == 当前 `HEAD`，且 `git status --porcelain --untracked-files=no` 为空
      （只算**被跟踪**文件的改动）。

    **为什么必须放在取锁之前**：契约测试 C7 要求任何 HBuilderX 交互都在 `Wait-HxFree` 之后；
    本判定不发起任何 cli 调用，所以可以（也应该）排在取锁之前 —— 无活时连锁都不取。

    **已知边界（写实，不假装更聪明）**：
      · **只新增、未被 git 跟踪、且没碰 `pages.json`** 的文件算「无变更」⇒ 那种场景用 `-Force`。
        反之 `pages.json` 本身是跟踪文件，新增页面必然改它 ⇒ 能被捕获。
      · 标记文件只在 `exit=ok`（`deployed=true`）成功后写；失败轮次不写、也不覆盖旧标记。

    **fail-safe（绝不因为「猜」挡掉一次真需要的部署）**：
      标记缺失 / 读不出 / 缺 `headSha` / git 不可用 / HEAD 取不到
      ⇒ 一律 `Noop = $false`（照常跑），Reason 里写明为什么放行。

.EXAMPLE
    . scripts/lib/hx-noop.ps1
    $r = Test-HxNoopRerun -Project 'D:\FL\training-app\...' -MarkerPath 'D:\FL\...\.ci-verify\last-deploy.json'
    if ($r.Noop) { Write-Host "别跑了：$($r.Reason)" }
#>

function Test-HxNoopRerun {
    <#
      返回 @{ Noop = <bool>; Reason = <string>; HeadSha = <string> }
      Noop=$true 只可能出现在「标记存在 + HEAD 相同 + 跟踪文件无改动」三条同时成立时。
    #>
    param(
        [Parameter(Mandatory = $true)][string]$Project,
        [Parameter(Mandatory = $true)][string]$MarkerPath
    )

    $result = @{ Noop = $false; Reason = ''; HeadSha = '' }

    if (-not (Test-Path -LiteralPath $MarkerPath -PathType Leaf)) {
        $result.Reason = '无上一轮部署标记（首轮，或标记被清）⇒ 照常跑'
        return $result
    }

    $marker = $null
    try {
        $marker = Get-Content -LiteralPath $MarkerPath -Raw -ErrorAction Stop | ConvertFrom-Json
    } catch {
        $result.Reason = "标记读不出（$($_.Exception.Message)）⇒ 照常跑"
        return $result
    }
    if (-not $marker -or -not ($marker.PSObject.Properties.Name -contains 'headSha') -or -not $marker.headSha) {
        $result.Reason = '标记缺 headSha ⇒ 照常跑'
        return $result
    }

    $headRaw = @()
    try { $headRaw = @(& git -C $Project rev-parse HEAD 2>$null) } catch { $headRaw = @() }
    $headLines = @($headRaw | Where-Object { "$_".Trim() -ne '' })
    if ($headLines.Count -eq 0) {
        $result.Reason = 'git rev-parse 拿不到 HEAD（git 不可用或不是仓库）⇒ 照常跑'
        return $result
    }
    $headSha = "$($headLines[0])".Trim()
    $result.HeadSha = $headSha

    if ($headSha -ne "$($marker.headSha)".Trim()) {
        $result.Reason = "HEAD 已变（$($marker.headSha) → $headSha）⇒ 有活"
        return $result
    }

    # ⚠️ 这里**必须**看 git 的退出码，且与 hxRunContract C6「判成败只解析 stdout」不冲突：
    #    C6 的理由是「HBuilderX cli 失败时退出码恒 0」，而 **git 的退出码是可信的**。
    #    `git status` 失败（索引损坏 / 并发 agent 的 index.lock / dubious ownership）时输出为空；
    #    若把它当成「工作树干净」就会判「没活」并**跳过一次真需要的部署** —— 正是本文件开头
    #    承诺绝不发生的事（Spec 轴 (c)1，2026-10-27 复审）。⇒ 拿不准就放行：Noop 保持 $false。
    $dirty = @()
    $statusOk = $false
    try {
        $dirty = @(& git -C $Project status --porcelain --untracked-files=no 2>$null)
        $statusOk = ($LASTEXITCODE -eq 0)
    } catch {
        $statusOk = $false
    }
    if (-not $statusOk) {
        $result.Reason = 'git status 失败（索引损坏 / index.lock / dubious ownership）⇒ 拿不准 ⇒ 照常跑'
        return $result
    }
    $dirtyLines = @($dirty | Where-Object { "$_".Trim() -ne '' })
    if ($dirtyLines.Count -gt 0) {
        $result.Reason = "工作树有 $($dirtyLines.Count) 处跟踪文件改动 ⇒ 有活"
        return $result
    }

    $result.Noop = $true
    $result.Reason = "上一轮部署成功（headSha=$($marker.headSha)），且 HEAD 与工作树都没变"
    return $result
}

function Write-HxDeployMarker {
    <#
      成功部署后记下 HEAD（供下一轮预检）。失败轮不调用本函数 ⇒ 旧标记保留。
      写盘失败不抛：标记只是省时间用的，不能因为它把一次成功的部署判红。
    #>
    param(
        [Parameter(Mandatory = $true)][string]$Project,
        [Parameter(Mandatory = $true)][string]$MarkerPath
    )

    try {
        $headRaw = @(& git -C $Project rev-parse HEAD 2>$null)
        $headLines = @($headRaw | Where-Object { "$_".Trim() -ne '' })
        if ($headLines.Count -eq 0) { return $false }
        $payload = [ordered]@{
            headSha = "$($headLines[0])".Trim()
            ts      = (Get-Date -Format 'yyyy-MM-ddTHH:mm:sszzz')
        } | ConvertTo-Json -Compress
        $dir = Split-Path -Parent $MarkerPath
        if ($dir -and -not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
        Set-Content -LiteralPath $MarkerPath -Value $payload -Encoding utf8
        return $true
    } catch {
        return $false
    }
}
