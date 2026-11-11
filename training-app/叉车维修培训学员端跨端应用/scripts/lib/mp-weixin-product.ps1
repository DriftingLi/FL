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
 5) **`app.json.pages` 路由必须唯一**（#1690 run7-10 实测教训）：重复路由下微信构建器 `checkMainPkgPages`
    拒启模拟器 —— 症状是 `Tool.getInfo` 应答而 `App.getPageStack` 永不应答（`page-stack-not-answering`），
    病根藏在产物文件里，不查这份判据就找不到。合法产物由构建器保证唯一；重复 ⇒ 产物被外部改写/污染。

 退出码语义（与门脚本一致）：本判据失败 = **产物坏了 ⇒ 门未过（exit 1）**，不是「环境不可用（exit 2）」。
 两者混在一起正是票面的病根：坏产物会以「看起来像环境问题」的形态（白屏 / `pageStack` 永不应答 /
 `routeTo appLaunch timeout`）把真正的病根藏起来。

 为什么单独成库而不是写成门脚本里的内联块：判据必须能被**真执行** —— 守护
 `utils/mpWeixinGateContract.test.js`（已随 ② 退役删除，#2326）的 C20 段用**好 / 坏产物夹具**直接调 `Get-MpWeixinProductReport`，
 现役的同款真执行覆盖在 `utils/mpWeixinBuildOnlyBehavior.test.js` 的 G3（非法产物 ⇒ exit 1）与
 `utils/mpWeixinPublishSettleBehavior.test.js`，
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

    # 路由唯一性（#1690 run7-10 实测教训）：重复路由 ⇒ 微信构建器 checkMainPkgPages 拒启模拟器，
    # 症状是 Tool.getInfo 应答而 App.getPageStack 永不应答（page-stack-not-answering）——病根在产物里。
    # 判序放在缺 .js 检查之前：坏夹具里重复项常同时缺配套文件，先报「重复」才指向真病根。
    $seen = @{}
    $dups = @()
    foreach ($p in $pages) {
        $rel0 = ([string]$p).TrimStart('/')
        if ($rel0.Length -eq 0) { continue }
        if ($seen.ContainsKey($rel0)) { if (-not ($dups -contains $rel0)) { $dups += $rel0 } } else { $seen[$rel0] = $true }
    }
    if ($dups.Count -gt 0) {
        return [ordered]@{
            ok = $false; stage = 'product-pages'; appid = $appid; pages = $pages.Count
            detail = ('app.json.pages 路由重复（' + (($dups | Select-Object -First 5) -join ' / ') + '）—— 微信构建器会拒启模拟器（症状：pageStack 永不应答），产物已被外部改写或污染，须重新 publish')
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

<#
  发行开关 + 包体积读数（#2417）。

  与上面的「产物合法性」**分开**，因为它们回答的是两个不同的问题，复用面也不同：
    · 合法性 = 「产物写完没有、能不能跑」—— lib/mp-weixin-publish-settle.ps1 的**收流判据**复用它，
      只该看写盘完成度；
    · 开关/体积 = 「这份产物是否带着发行必需的开关、主包多大」—— 只有构建面检查（⑤，可选不判红）读它。
  若把开关断言塞进 Get-MpWeixinProductReport，一个「产物完整但开关没开」的构建会被收流判据判成
  「没写完」，一路等到 publish 预算超时（900 s 白烧）—— 这是两个判据必须分开的实质理由，不是整洁洁癖。

  口径（判据取**产物**，不取仓根配置 —— 铁律）：
    仓根 project.config.json / manifest.json 改对了、产物却没带（构建没重来 / HBuilderX 没把仓根那份
    复制进产物）时，微信上传面板的「JS 压缩」「组件按需注入」照样红 ⇒ 断言源码只会给出**假绿**。
    故这里只读产物里的 app.json 与 project.config.json。

  主包字节数 = 产物目录全部文件字节 **减去** app.json 声明分包（subPackages / subpackages，取各自
  root 目录）下的文件。无分包时等于全量字节。指纹文件 .mp-weixin-build-fingerprint.json 不计
  （与 mp-weixin-build-fingerprint.ps1 / mp-weixin-publish-settle.ps1 的排除口径同源）。

  为什么本地字节能当「2 M 硬限」的判据：minified 是**上传时**压缩（#2415 同树 A/B 实证：开关 ON/OFF
  产物字节只差 42 B、.js 分毫不差）⇒ 上传包 ≤ 本地产物字节，本地不过硬限上传必不过；反过来本地产物
  过线也不等于上传面板的 1.5 M 建议线读数（那是面板口径，本函数只给本地读数，不冒充面板）。
#>

<#
  安全取 JSON 属性：ConvertFrom-Json 的对象在 Set-StrictMode -Version Latest 下访问不存在的属性会抛
  （不是回 $null），故按 PSObject 判存在；[ordered]@{} 字典的键不是属性，另走 IDictionary 分支
  （#1817：字典键按属性读会静默回 $null）。
#>
function Get-ProductJsonProperty {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $p = $Object.PSObject.Properties[$Name]
    if ($p) { return $p.Value }
    if ($Object -is [System.Collections.IDictionary] -and $Object.Contains($Name)) { return $Object[$Name] }
    return $null
}

<#
  把一个开关值渲染成结果行可用的**字面量**：true / false / 字符串原样 / 缺失写 missing。
  用字面量而不是空串：结果行要「直接读出真实状态」，空值会被读成「没这一项」。
#>
function Format-ProductSwitchValue {
    param($Value)
    if ($null -eq $Value) { return 'missing' }
    if ($Value -is [bool]) { if ($Value) { return 'true' } else { return 'false' } }
    return [string]$Value
}

<#
  布尔开关严格判定：值必须**是 JSON 布尔**且等于期望值。
  为什么不用 -eq 直接比：PowerShell 会把字符串 'true' 与 $true 按字符串比成相等 ⇒ 写成
  "minified": "true" 的产物会被放过，而微信按布尔读，照样不压缩。
#>
function Test-ProductSwitchBool {
    param($Value, [bool]$Expected)
    if ($null -eq $Value) { return $false }
    if ($Value -isnot [bool]) { return $false }
    return ($Value -eq $Expected)
}

<#
  读一份 mp-weixin 产物的发行开关与包体积。
  @param Dist 产物目录（unpackage/dist/build/mp-weixin）
  @returns 有序字典：ok / stage / detail / lazyCodeLoading / minified / es6 /
           totalBytes / mainBytes / files / subPackages
           stage ∈ product-ok / product-missing / product-json（与 #1210 同族，便于失败行统一认读）
#>
function Get-MpWeixinArtifactReadings {
    param([Parameter(Mandatory = $true)][string]$Dist)

    $fail = {
        param([string]$Stage, [string]$Detail)
        return [ordered]@{
            ok = $false; stage = $Stage; detail = $Detail
            lazyCodeLoading = $null; minified = $null; es6 = $null
            totalBytes = [long]0; mainBytes = [long]0; files = 0; subPackages = 0
        }
    }

    $appPath = Join-Path $Dist 'app.json'
    $cfgPath = Join-Path $Dist 'project.config.json'
    if (-not (Test-Path -LiteralPath $appPath)) { return & $fail 'product-missing' "产物缺 app.json：$appPath" }
    if (-not (Test-Path -LiteralPath $cfgPath)) { return & $fail 'product-missing' "产物缺 project.config.json：$cfgPath" }

    $app = Read-ProductJsonFile -Path $appPath
    if (-not $app.ok) { return & $fail 'product-json' "app.json 非法 JSON：$($app.error)" }
    $cfg = Read-ProductJsonFile -Path $cfgPath
    if (-not $cfg.ok) { return & $fail 'product-json' "project.config.json 非法 JSON：$($cfg.error)" }

    $setting = Get-ProductJsonProperty -Object $cfg.value -Name 'setting'
    $lazy = Get-ProductJsonProperty -Object $app.value -Name 'lazyCodeLoading'
    $minified = Get-ProductJsonProperty -Object $setting -Name 'minified'
    $es6 = Get-ProductJsonProperty -Object $setting -Name 'es6'

    # 分包根：主包 = 分包之外的一切。两条属性名都要认（微信产物写 subPackages，历史上也有小写形态）。
    $subs = Get-ProductJsonProperty -Object $app.value -Name 'subPackages'
    if ($null -eq $subs) { $subs = Get-ProductJsonProperty -Object $app.value -Name 'subpackages' }
    $roots = @()
    foreach ($s in @($subs)) {
        $root = ([string](Get-ProductJsonProperty -Object $s -Name 'root')).Trim().TrimStart('/').TrimEnd('/')
        if ($root.Length -gt 0) { $roots += ($root -replace '\\', '/') }
    }

    $files = 0
    $total = [long]0
    $main = [long]0
    if (Test-Path -LiteralPath $Dist -PathType Container) {
        foreach ($f in @(Get-ChildItem -LiteralPath $Dist -Recurse -File -Force -ErrorAction SilentlyContinue)) {
            if ($f.Name -eq '.mp-weixin-build-fingerprint.json') { continue }
            $files++
            $total += $f.Length
            $rel = ([System.IO.Path]::GetRelativePath($Dist, $f.FullName)) -replace '\\', '/'
            $inSub = $false
            foreach ($r in $roots) {
                if ($rel.StartsWith($r + '/', [System.StringComparison]::OrdinalIgnoreCase)) { $inSub = $true; break }
            }
            if (-not $inSub) { $main += $f.Length }
        }
    }

    return [ordered]@{
        ok = $true; stage = 'product-ok'
        detail = ("产物开关与体积读数：lazyCodeLoading=$(Format-ProductSwitchValue $lazy)；" +
            "minified=$(Format-ProductSwitchValue $minified)；es6=$(Format-ProductSwitchValue $es6)；" +
            "主包 $main B / 全量 $total B / $files 文件 / 分包 $(@($roots).Count) 个")
        lazyCodeLoading = $lazy; minified = $minified; es6 = $es6
        totalBytes = $total; mainBytes = $main; files = $files; subPackages = @($roots).Count
    }
}
