<#
 ② 门构建段「产物已完整 ⇒ 可以收流」判据库 —— 单点真源（#2163）。

 病根（票面两形态 + `.scratch/2163/handoff.md`）：`cli publish mp-weixin` 的收敛信号只有
 「进程退出」一条（`Invoke-Process` 的 `WaitForExit(预算)`）。实测（2026-10-31 新树 5 s 采样）
 HBuilderX 是在构建**结束时一次性写出**整个产物目录（643 文件在同一 ≤5 s 窗口内出现），写完 cli
 正常退出 ⇒ 门过（245 s）；但 cli 也有「产物已写满而自己不退出」的形态（票面 B：产物 23:32:45 齐、
 cli 到 12 分钟后仍活着）。只要收敛只认进程退出，后一种形态必然烧到预算并以
 `errors=env-blocked reason=timeout` 收场 —— 整轮 ② 白烧，而默认预算 900 s 下的新树首编译
 到点被杀时产物**根本不存在**（本票实测：被杀的那轮 dist 缺席，不是半成品）。

 判据（三条同时成立才收流，缺一不收 —— fail-closed）：
   1) **产物合法**：复用 `scripts/lib/mp-weixin-product.ps1` 的 `Get-MpWeixinProductReport`
      （必需文件齐 + 两份 JSON 可解析 + `app.json.pages` 路由唯一且每页有 `.js`）。
      ⚠️ 本库**不另定义**「什么算完整产物」：那是 #1210 的单点真源，抄第二份必然漂移（I4）。
   2) **本次写盘**（新鲜度）：产物里最新的文件 mtime 晚于本轮 publish 发命令前取的时刻 `Since`。
      拦的是 #1272 那一族假绿 ——「盘上留着上一次的完整产物」在本判据下必判 stale 而不收流。
      （实测依据：被杀的构建**不留下** dist，所以「已存在且新」只可能出自本轮。）
   3) **已静默**：最新文件距今 ≥ `QuietSeconds`。产物是末尾一次性写盘，写入本身要几秒；
      没有这条就会拿「正在写的目录」当写完的目录（`app.js` 落了、页面 `.js` 还在写）。
      `Get-MpWeixinProductReport` 的「每页都有 .js」已经拦掉大部分半途形态，这条补的是
      「声明的页齐了但公共 chunk 还在写」那种它看不见的窗口。

 指纹文件 `.mp-weixin-build-fingerprint.json` **不计入新鲜度**：它由门在构建成功后写进产物目录，
 拿它当「本次写过」的证据会把「产物全旧、只有指纹新」读成成立（S5 钉的就是这一条）。
 排除口径与 `Get-MpWeixinDistContentHash`（#1860）同源 —— 同一个文件，两处必须一起排除。

 只报读数、不改门语义：返回体带 `Pages` / `AppId` / `Files` / `NewestAgeS`，`Settled` 为真时
 结果行才允许写 `publish=settled-by-artifact`；不成立时调用方照旧走既有分支（timeout ⇒ exit 2）。
 ② 的三分（通过 / exit 1 门未过 / exit 2 环境受阻）在这里不新增第四态。
#>

if (-not (Get-Command Get-MpWeixinProductReport -ErrorAction SilentlyContinue)) {
    . (Join-Path $PSScriptRoot 'mp-weixin-product.ps1')
}

<#
  产物目录里的文件（排除指纹文件自身）。返回 [IO.FileInfo[]]；目录不存在 ⇒ 空数组。
  排除口径与 `mp-weixin-build-fingerprint.ps1` 的 `Get-MpWeixinDistContentHash` 一致。
#>
function Get-MpWeixinDistFiles {
    param([Parameter(Mandatory = $true)][string]$Dist)
    if (-not (Test-Path -LiteralPath $Dist -PathType Container)) { return @() }
    return @(Get-ChildItem -LiteralPath $Dist -Recurse -File -Force -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ne '.mp-weixin-build-fingerprint.json' })
}

<#
  构建段收流判据：产物合法（复用单点真源）+ 本次写盘 + 已静默 ⇒ Settled。
  @param Dist    产物目录（`unpackage/dist/build/mp-weixin`）
  @param Since   本轮 publish **发命令之前**取的时刻（与调用方同源，别在收流时才取）
  @param QuietSeconds 静默窗口秒数；最新文件距今不足它 ⇒ 还在写，不收流
  @param Now     注入的「现在」（缺省 `Get-Date`；守护用它把时间钉成确定值）
  @returns 有序字典：Settled / Reason / Files / NewestAgeS / OldestAgeS / Pages / AppId / Detail
           Reason ∈ settled | product-missing | product-json | product-pages | no-dist |
                      stale | writing
#>
function Get-MpWeixinPublishSettle {
    param(
        [Parameter(Mandatory = $true)][string]$Dist,
        [Parameter(Mandatory = $true)][datetime]$Since,
        [int]$QuietSeconds = 15,
        [datetime]$Now = (Get-Date)
    )
    $fail = {
        param($Reason, $Detail)
        return [ordered]@{
            Settled = $false; Reason = "$Reason"; Files = 0; NewestAgeS = -1; OldestAgeS = -1
            Pages = 0; AppId = ''; Detail = "$Detail"
        }
    }

    if (-not (Test-Path -LiteralPath $Dist -PathType Container)) {
        return & $fail 'no-dist' "产物目录不存在：$Dist"
    }

    # 1) 产物合法 —— 直接问 #1210 的单点真源，不在这里复述它的判据
    $prod = Get-MpWeixinProductReport -Dist $Dist
    if (-not $prod.ok) {
        return & $fail $prod.stage $prod.detail
    }

    $files = @(Get-MpWeixinDistFiles -Dist $Dist)
    if ($files.Count -eq 0) {
        return & $fail 'no-dist' "产物目录里没有任何文件（指纹除外）：$Dist"
    }

    # 2) 新鲜度 —— 指纹文件已在上面排除：它新不代表产物新
    $newest = ($files | Sort-Object LastWriteTime -Descending | Select-Object -First 1).LastWriteTime
    $oldest = ($files | Sort-Object LastWriteTime | Select-Object -First 1).LastWriteTime
    $newestAgeS = [int]($Now - $newest).TotalSeconds
    $oldestAgeS = [int]($Now - $oldest).TotalSeconds

    if ($newest -le $Since) {
        return & $fail 'stale' ("产物里最新的文件也早于本轮 publish 基准（newest=$newest since=$Since）——" +
            '这是上一次留下的目录，本轮还没写盘 ⇒ 不收流（#1272 假绿族）')
    }

    # 3) 静默 —— 末尾一次性写盘，写入进行中最忌早收
    if ($newestAgeS -lt $QuietSeconds) {
        return [ordered]@{
            Settled = $false; Reason = 'writing'; Files = $files.Count
            NewestAgeS = $newestAgeS; OldestAgeS = $oldestAgeS
            Pages = $prod.pages; AppId = $prod.appid
            Detail = "最新文件落在 $newestAgeS s 前，不足静默窗口 $QuietSeconds s ⇒ 产物仍在写"
        }
    }

    return [ordered]@{
        Settled = $true; Reason = 'settled'; Files = $files.Count
        NewestAgeS = $newestAgeS; OldestAgeS = $oldestAgeS
        Pages = $prod.pages; AppId = $prod.appid
        Detail = "产物合法（$($prod.pages) 页 / $($files.Count) 文件）且为本次写盘（最新 $newestAgeS s 前、最旧 $oldestAgeS s 前）并已静默 $QuietSeconds s"
    }
}
