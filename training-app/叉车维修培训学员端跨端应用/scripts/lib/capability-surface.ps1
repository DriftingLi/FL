<#
.SYNOPSIS
    「能力面」（ADR-0016 ①b）的唯一真源：路径白名单 + 原生能力调用点扫描（**只做提示、不判红**）。

.DESCRIPTION
    ADR-0016 把 ① 真机门的触发面收窄为「能力面」—— agent 机械上观测不到的四类：
    指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互。本文件提供三件事：

      ① `Get-CapabilitySurfaceRules` + `Get-CapabilitySurfaceVerdict`
         —— **路径白名单**，判「本批是否触及能力面」（`dev:finish` 收口提示行里的「是 / 否」）。
      ② `Get-CapabilityCallSites`
         —— 扫改动集里的**原生能力调用点**，只把命中打在提示行里（**不是判据**）。
      ③ `Get-CapabilityCategories`
         —— 四个类别的规范名，与 `.github/PULL_REQUEST_TEMPLATE.md` 的自报勾选框**逐字**对齐。

    三条硬约束（ADR-0016 ③，别在这里加戏）：
      · **不判红**：本文件不返回退出码、不被 `pr-evidence` 引用；判错「是」的代价是看一眼，不是门红。
      · **不拿文本当触发判据**（#1030 的教训）：判「文件里出现过某字符串」会把**注释/文档里的提及**
        也算命中 —— 本仓 2026-09-16 实测有 5 个文件只在注释里提过 API 名（api/auth.uts / api/forum.uts /
        api/aiAssistant.uts / utils/uploadPath.uts / pages/forum/components/forum-contribution-form.uvue）。
        故扫描用**调用语法**（uni + 点 + API 名 + 左括号），白名单更是**只按路径**判、完全不读内容。
      · **自报是声明**：白名单没命中 ≠ 没触及（新页面可能新调 chooseImage）⇒ 提示行把
        「白名单未命中但扫描有命中」显式打出来，由人决定要不要在 PR 模板自报「是」。

.NOTES
    纯函数 + 只读改动集里的文本文件：不碰设备、不碰 HBuilderX、不取锁 ⇒ 任何机器上都能跑，秒级。
    守护：`utils/capabilitySurfaceBehavior.test.js`（H1–H8，**运行期**断言，不是源码文本断言）。
    口径真源：`docs/adr/0016-真机门的人工性收缩与按批取证.md`（② 第 6 条 / ③ / ④ 第 1 项 / ⑥）。
#>

function Get-CapabilityCategories {
    <#
      四个类别的规范名 —— 与 `.github/PULL_REQUEST_TEMPLATE.md` 的自报框**逐字**对齐（守护 H6）。
      改这里就必须同步改模板，否则 H6 判红（故意的：两份产物漂移等于自报框失去解释力）。
    #>
    return @('指纹', '运行时权限弹窗', '真机上传', '厂商 ROM 交互')
}

function Get-CapabilitySurfaceRules {
    <#
      能力面**路径白名单**（宽口径，2026-09-16 裁定）：既含原生调用点，也含承接其流程的页面。
      为什么宽：login.uvue 的三次实质提交（#499 生物识别回填 / #550 安全记住密码 /
        #624 快捷登录 + 孤儿凭据自愈）都是能力变更 —— 「改了这道门控」本身只有人按手指才观测得到，
        而白名单若只列调用点，恰恰会把最常发生的能力变更漏在机检之外（该页历史仅 10 次提交，代价可控）。
      为什么按路径不按内容：见文件头 · #1030（注释里提 API 名会被内容判据误报）。
      维护：新增能力调用点时把路径补进来。**漏加不判红** —— 收口提示行会打「白名单未命中但扫描有命中」。
    #>
    return @(
        [pscustomobject]@{
            Category  = '指纹'
            Paths     = @('composables/useBiometric.uts', 'utils/secureStorage.uts', 'pages/login/login.uvue')
            Rationale = 'SOTER 封装（唯一 startSoterAuthentication 载体）+ 凭据密文与孤儿自愈终态（ADR-0004）+ 门控回填入口页'
        }
        [pscustomobject]@{
            Category  = '运行时权限弹窗'
            Paths     = @(
                'composables/useForumImagePicker.uts',
                'composables/useReplyComposer.uts',
                'pages/forum/forum-create.uvue',
                'pages/profile-setup/profile-setup.uvue',
                'pages/profile/personal-info.uvue',
                'pages/profile/profile.uvue'
            )
            Rationale = 'uni.chooseImage 在真机上会拉起相册/相机的系统权限弹窗；弹窗本身与拒绝后的行为只有人能观测。useForumImagePicker.uts 是 uni.chooseImage 全树唯一调用点（#1240 P3 把图片流水线从发帖/回复两宿主抽出为共享 picker），改它即改选图/上传能力面；另两条旧宿主路径承接其流程，profile 系页面各自直接调 chooseImage'
        }
        [pscustomobject]@{
            Category  = '真机上传'
            Paths     = @('api/request.uts', 'utils/uploadPath.uts')
            Rationale = 'uni.uploadFile + content:// 落盘中转 —— #602001 是 content provider 差异造成的真机独有失败模式'
        }
        [pscustomobject]@{
            Category  = '厂商 ROM 交互'
            Paths     = @()
            Rationale = '当前无代码载体（2026-09-16 grep 实测：无 MANUFACTURER / plus.android / Build.* 条件分支）⇒ 仅由 PR 模板自报兜；将来落了 ROM 分支代码，把路径补进这里'
        }
    )
}

function Get-CapabilitySurfaceVerdict {
    <#
      路径白名单判定（纯函数，便于运行期断言）。匹配用**后缀**：改动集是**仓库根相对**路径
      （`dev:finish` 走 `git diff --name-only`，本仓项目位于 `training-app/<中文目录名>/` 下 ⇒
       形如 `training-app/叉车维修培训学员端跨端应用/composables/useBiometric.uts`）。
      返回 { Touched; Hits[{Path,Rule,Category}]; Categories }。
    #>
    param([string[]]$ChangedFiles = @())

    $files = @($ChangedFiles | Where-Object { $_ -and $_.Trim() } | ForEach-Object { $_.Trim().Replace('\', '/') })
    $hits = @()
    foreach ($rule in (Get-CapabilitySurfaceRules)) {
        foreach ($p in @($rule.Paths)) {
            foreach ($f in $files) {
                if ($f -eq $p -or $f.EndsWith('/' + $p)) {
                    $hits += [pscustomobject]@{ Path = $f; Rule = $p; Category = $rule.Category }
                }
            }
        }
    }

    return [pscustomobject]@{
        Touched    = ($hits.Count -gt 0)
        Hits       = $hits
        Categories = @($hits | ForEach-Object { $_.Category } | Select-Object -Unique)
    }
}

function Get-CapabilityCallTokens {
    <#
      原生能力调用点的 API 名（扫描用）。只列**会拉起 agent 观测不到的设备交互**的 API；
      用调用语法匹配（见 `Get-CapabilityCallSites`），所以裸串出现的注释/文档不会命中。
      列表可加：新增一类真机独有能力时，把 API 名加进来（加完跑一次 `npm run test:unit`）。
    #>
    return @(
        # 指纹 / 生物识别
        'startSoterAuthentication', 'checkIsSupportSoterAuthentication', 'checkIsSoterEnrolledInDevice',
        # 运行时权限弹窗（相册 / 相机 / 定位 / 蓝牙）
        'chooseImage', 'chooseVideo', 'chooseMedia',
        'requestPermissions', 'openAppAuthorizeSetting', 'saveImageToPhotosAlbum',
        'getLocation', 'startBluetoothDevicesDiscovery',
        # 真机上传
        'uploadFile'
    )
}

function Resolve-CapabilityChangedPath {
    <#
      把改动集里的路径解析成**真实存在的文件路径**（解析不到 ⇒ 返回 $null，调用方跳过）。
      为什么不能只 `Join-Path $ProjectDir`：`dev:finish` 的改动集来自 `git diff --name-only`，
      而它在本仓给的是**仓库根相对**路径（本仓项目位于 `training-app/<中文目录名>/` 下）——
      `Join-Path <项目目录> <仓库根相对路径>` 会拼成一个不存在的路径 ⇒ 扫描**静默零命中**，
      而那正是「写了但走不到」的假绿形态（2026-09-16 实测：`useBiometric.uts` 明明有 SOTER 调用，
      提示行却打 `call_sites=0`；守护 H5b 钉的就是这一条）。

      三个候选依次试（命中即返回）：
        ① 已是**绝对路径**且存在（少数调用方会传绝对路径）；
        ② 项目目录相对（`composables/x.uts`）；
        ③ 仓库根相对（`training-app/<中文目录名>/composables/x.uts`）——用**项目目录名**做锚点剥前缀，
           不用 `git rev-parse --show-toplevel`：少一个外部依赖，也不吃 git 的 quotepath 转义。
    #>
    param(
        [string]$Path,
        [string]$ProjectDir = ''
    )
    if (-not $Path) { return $null }
    $norm = $Path.Replace('\', '/')

    if ([System.IO.Path]::IsPathRooted($norm)) {
        if (Test-Path -LiteralPath $norm -PathType Leaf) { return $norm }
    }

    if ($ProjectDir) {
        $candidate = Join-Path $ProjectDir $norm
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }

        $projName = Split-Path -Leaf $ProjectDir.TrimEnd('\', '/')
        if ($projName) {
            $anchor = '/' + $projName + '/'
            $i = $norm.LastIndexOf($anchor, [System.StringComparison]::Ordinal)
            if ($i -ge 0) {
                $candidate = Join-Path $ProjectDir $norm.Substring($i + $anchor.Length)
                if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
            }
        }
    }

    return $null
}

function Get-CapabilityCallSites {
    <#
      扫改动集里的原生能力调用点（**只读工作树文本，只做提示，不判红**）。
      匹配 `uni.<api> + 可选空白 + 左括号`：注释里**提到** API 名不算（#1030 的坑位：文本存在 ≠ 行为）。
      只扫代码扩展名，避免把二进制/大文件读进来；删除的文件解析不到 ⇒ 跳过（见 Resolve-CapabilityChangedPath）。
      返回 @( { Path; Line; Api } )，按改动集顺序。
    #>
    param(
        [string[]]$ChangedFiles = @(),
        [string]$ProjectDir = ''
    )

    $tokens = @(Get-CapabilityCallTokens)
    if ($tokens.Count -eq 0) { return @() }
    $pattern = 'uni\.(' + (($tokens | ForEach-Object { [regex]::Escape($_) }) -join '|') + ')\s*\('
    $codeExt = '\.(uts|uvue|vue|ts|js|mjs)$'

    $out = @()
    $files = @($ChangedFiles | Where-Object { $_ -and $_.Trim() } | ForEach-Object { $_.Trim() })
    foreach ($f in $files) {
        $norm = $f.Replace('\', '/')
        if ($norm -notmatch $codeExt) { continue }
        $full = Resolve-CapabilityChangedPath -Path $norm -ProjectDir $ProjectDir
        if (-not $full) { continue }
        $lines = @(Get-Content -LiteralPath $full -Encoding UTF8 -ErrorAction SilentlyContinue)
        for ($i = 0; $i -lt $lines.Count; $i++) {
            $m = [regex]::Match([string]$lines[$i], $pattern)
            if ($m.Success) {
                $out += [pscustomobject]@{ Path = $norm; Line = ($i + 1); Api = $m.Groups[1].Value }
            }
        }
    }
    return @($out)
}

function Format-CapabilityHint {
    <#
      `dev:finish` 收口时打的那几行（纯构造，便于运行期断言）。最后一行是**机检行**
      （`CAPABILITY_SURFACE touched=…`，ASCII，供守护/取证 grep；它**不是判据**，不影响退出码）。

      文案要守的两条验收标准：
        · 「是」⇒ 点明 ①b 必做 + 由**人**给出原文（与 PR 模板自报框语义一致，ADR-0016 ⑥）；
        · 「否」⇒ **不得**引入任何新的必做项（普通运行时面改动不能被这行字凭空加要求）。
    #>
    param(
        [string[]]$ChangedFiles = @(),
        [string]$ProjectDir = '',
        [object[]]$CallSites = $null
    )

    $verdict = Get-CapabilitySurfaceVerdict -ChangedFiles $ChangedFiles
    # ⚠️ 不能写成 `$sites = if (…) { @($CallSites) } else { @(…) }`：**if 作表达式时，分支输出空数组
    #    会在输出流里变成「什么都不输出」⇒ $sites = $null**，下一行的 `$sites.Count` 就在
    #    `Set-StrictMode -Latest` 下抛「在此对象上找不到属性 Count」（2026-09-16 实测踩到，
    #    同族先例：level-detect.ps1 的「@() 必须包住整条管道」注释）。
    $sites = @()
    if ($null -ne $CallSites) { $sites = @($CallSites) }
    else { $sites = @(Get-CapabilityCallSites -ChangedFiles $ChangedFiles -ProjectDir $ProjectDir) }

    $lines = New-Object System.Collections.Generic.List[string]
    $verdictWord = if ($verdict.Touched) { '是' } else { '否' }
    $lines.Add("🔎 本批触及能力面：$verdictWord（ADR-0016 ①b；白名单真源 scripts/lib/capability-surface.ps1）")

    if ($verdict.Touched) {
        $named = @($verdict.Hits | ForEach-Object { "$($_.Path)（$($_.Category)）" } | Select-Object -Unique)
        $lines.Add("   · 白名单命中 $($named.Count) 个文件：" + ($named -join '、'))
    }
    else {
        $lines.Add('   · 白名单未命中（' + ((Get-CapabilityCategories) -join ' / ') + '）。')
    }

    if ($sites.Count -gt 0) {
        $parts = @($sites | ForEach-Object { "$($_.Path):$($_.Line) $($_.Api)" })
        $lines.Add('   · 原生能力调用点扫描（只做提示、不判红）：' + ($parts -join '、'))
    }
    else {
        $lines.Add('   · 原生能力调用点扫描（只做提示、不判红）：未命中。')
    }

    if ($verdict.Touched) {
        $lines.Add('   · ⇒ ①b 必做：由**人**在真机上跑一次，把原文给到 PR 证据段「执行人」栏（agent 只可代录、不得自拟）；PR 模板『本次改动触及「agent 观测不到的能力面」』框须勾上。')
    }
    elseif ($sites.Count -gt 0) {
        $lines.Add('   · ⚠️ 白名单未命中但扫描出现调用点：若本次改动**改变了这些调用的行为**，请在 PR 模板自报勾「是」（自报是声明、不是判据；漏报属违规）。')
    }
    else {
        $lines.Add('   · 未触及 ⇒ 无需 ①b；PR 模板『本次改动触及「agent 观测不到的能力面」』框不勾（勾了才会要求人签）。')
    }

    $lines.Add("CAPABILITY_SURFACE touched=$(if ($verdict.Touched) { 'yes' } else { 'no' }) whitelist_files=$($verdict.Hits.Count) call_sites=$($sites.Count)")
    return $lines.ToArray()
}
