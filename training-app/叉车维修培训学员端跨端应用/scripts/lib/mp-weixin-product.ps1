<#
 ② 门「产物合法性」判据面 —— **单点真源**（#1210）。

 判据（维护者 2026-09-21 裁定；决策回写在移动端 `docs/adr/0008-移动端验收门与证据.md` 的 ② 段）：

   1) **必需文件清单**：`project.config.json` / `app.json` / `app.js` / `app.wxss`。`sitemap.json` 与
      `project.private.config.json` **不作必需** —— 本机实测 `publish mp-weixin` 不产出它们
      （2026-09-21 实测：产物 580 文件 / 58 页 / 117 个页 `.js`，上述两份均不存在）。
   2) **JSON 必须可解析**：非法即 fail-closed，并把解析错误的**位置原文**（`line N, position M`）带出来 ——
      这正是票面「打印非法位置，让人一眼看到病根」那条。
   3) **appid 由解析后取值**：不再用正则从（可能非法的）文件里抠一个 appid 假装通过。
   4) **`app.json.pages` 每页必须有对应 `<page>.js`**。

 退出码语义（与门脚本一致）：本判据失败 = **产物坏了 ⇒ 门未过（exit 1）**，不是「环境不可用（exit 2）」。
 两者混在一起正是票面的病根：坏产物会以「看起来像环境问题」的形态（白屏 / `pageStack` 永不应答 /
 `routeTo appLaunch timeout`）把真正的病根藏起来。

 为什么单独成库而不是写成门脚本里的内联块：判据必须能被**真执行** —— 守护
 `utils/mpWeixinGateContract.test.js` 的 C20 段用**好 / 坏产物夹具**直接调 `Get-MpWeixinProductReport`，
 而不是断言源码文本（文本断言在「判据被摘掉、只留一句字面量」时照样绿）。
#>

<#
 读一个 JSON 文件并**容错解析**。
 @returns 有序字典：ok（是否可解析）/ value（解析结果，失败为 $null）/ error（失败原因原文，含位置）
#>
function Read-ProductJsonFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    $raw = Get-Content -LiteralPath $Path -Raw
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return [ordered]@{ ok = $false; value = $null; error = '文件为空' }
    }
    try {
        $value = $raw | ConvertFrom-Json
    } catch {
        # PowerShell（System.Text.Json）的报错原文里带 `line N, position M`：原样带出，不再自己数行
        return [ordered]@{ ok = $false; value = $null; error = $_.Exception.Message }
    }
    if ($null -eq $value) {
        return [ordered]@{ ok = $false; value = $null; error = '解析结果为 null（空 JSON 文档）' }
    }
    return [ordered]@{ ok = $true; value = $value; error = '' }
}

<#
 校验一份 mp-weixin 构建产物。
 @param Dist 产物目录（`unpackage/dist/build/mp-weixin`）
 @returns 有序字典：ok / stage / appid / pages / detail（stage 取值：product-ok / product-missing / product-json / product-pages）
#>
function Get-MpWeixinProductReport {
    param([Parameter(Mandatory = $true)][string]$Dist)

    $required = @('project.config.json', 'app.json', 'app.js', 'app.wxss')
    $missing = @()
    foreach ($f in $required) {
        if (-not (Test-Path -LiteralPath (Join-Path $Dist $f))) { $missing += $f }
    }
    if ($missing.Count -gt 0) {
        return [ordered]@{
            ok = $false; stage = 'product-missing'; appid = ''; pages = 0
            detail = "产物缺必需文件：" + ($missing -join ' / ') + "（目录 $Dist）"
        }
    }

    $cfg = Read-ProductJsonFile -Path (Join-Path $Dist 'project.config.json')
    if (-not $cfg.ok) {
        return [ordered]@{
            ok = $false; stage = 'product-json'; appid = ''; pages = 0
            detail = "project.config.json 非法 JSON：$($cfg.error)"
        }
    }
    $appJson = Read-ProductJsonFile -Path (Join-Path $Dist 'app.json')
    if (-not $appJson.ok) {
        return [ordered]@{
            ok = $false; stage = 'product-json'; appid = ''; pages = 0
            detail = "app.json 非法 JSON：$($appJson.error)"
        }
    }

    # appid：**解析后取值**（旧实现用正则 `"appid"\s*:\s*"([^"]*)"`，坏文件里也能抠出一个值来）
    $appid = ''
    if ($cfg.value.PSObject.Properties.Name -contains 'appid') { $appid = [string]$cfg.value.appid }

    $pages = @()
    if ($appJson.value.PSObject.Properties.Name -contains 'pages') { $pages = @($appJson.value.pages) }
    if ($pages.Count -eq 0) {
        return [ordered]@{
            ok = $false; stage = 'product-pages'; appid = $appid; pages = 0
            detail = 'app.json 的 pages 为空或缺失（没有页面可验 ⇒ 产物不可信）'
        }
    }

    $missJs = @()
    foreach ($p in $pages) {
        $rel = ([string]$p).TrimStart('/')
        if ($rel.Length -eq 0) { continue }
        if (-not (Test-Path -LiteralPath (Join-Path $Dist ($rel + '.js')))) { $missJs += ($rel + '.js') }
    }
    if ($missJs.Count -gt 0) {
        $head = (@($missJs | Select-Object -First 5) -join ' / ')
        $more = if ($missJs.Count -gt 5) { " 等 $($missJs.Count) 个" } else { '' }
        return [ordered]@{
            ok = $false; stage = 'product-pages'; appid = $appid; pages = $pages.Count
            detail = "app.json.pages 声明的页面缺 .js（$head$more）—— 产物写盘不完整"
        }
    }

    return [ordered]@{
        ok = $true; stage = 'product-ok'; appid = $appid; pages = $pages.Count
        detail = "产物合法：appid='$appid'；pages=$($pages.Count) 页各自有 .js；必需文件齐（project.config.json / app.json / app.js / app.wxss）"
    }
}
