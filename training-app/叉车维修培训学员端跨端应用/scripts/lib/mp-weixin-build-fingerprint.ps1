<#
 ② 门「产物内容指纹 ⇒ 自动 -SkipBuild」判据库 —— 单点真源（#1860）。
 范式依据：docs/design/20261024-验收门成本fix与证据复用实践调研.md §1.2 条 2 / §3(b)
 （Turborepo/Nx/Bazel 共同原理：**cache key = 相关输入的内容哈希，不是 commit sha** ——
 「同输入不同 sha 必须命中、不同输入同 sha 必须不命中」；键里必须含 HBuilderX 版本号 ——
 姊妹篇坑位 6b 证明同代码不同工具版本结论会翻）。

 病根：#1494 §0.1 失败轮重跑时 publish 重建一模一样的产物（≈2–3 min/次，#1443 实测 -SkipBuild 的省值）。
 「产物未变」不该靠人记得带 `-SkipBuild`，该机判 —— 指纹命中即自动跳 publish。

 指纹键（三组件，**全等才命中**；落盘为产物目录内的 `.mp-weixin-build-fingerprint.json`）：
   inputs   = 构建输入集内容哈希（app 项目根递归，排除清单见 Get-MpWeixinBuildInputFiles）——
              源码变了必不命中：防「改了源码却拿旧产物出绿」的假证据（调研 §1.2 条 2 的核心风险）；
   product  = 产物目录内容哈希（排除指纹文件自身）—— 产物被换/被改/被直达换页残留污染必不命中
              （票面「产物或版本变化 ⇒ 必然重建」）；
   hbuilderx = HBuilderX 版本令牌（安装目录版本号 + cli.exe 内容哈希；cli.exe 无 PE 版本资源，
              2026-10-24 本机实测 FileVersion/ProductVersion 为空 ⇒ 目录名解析 + 二进制哈希双保险）。
 命中前置条件（**第四道**，#2048 G1；它是「记录 + 比对」，**不进上面三组件的哈希**）：
   command = 本轮要用的 publish 参数规范化串（Get-MpWeixinBuildCommandKey）—— 与指纹记录里的
             command 不等、或记录里没有该字段 ⇒ 不命中 ⇒ 重建。
             病根（#2048 判据①实测）：门脚本与一切 .ps1 都被输入集排除（见 Get-MpWeixinBuildInputHash
             的清单），而 publish 参数**决定产物**——改它不动任何哈希组件、旧产物原样在盘 ⇒ 三组件照样
             全等 ⇒ 命中 ⇒ 跳过 publish ⇒ 「新命令从未执行、门在旧命令的产物上出绿」= fail-open。
             命令属**非文件型来源**、进不了文件哈希，故用第四道比对收口，而不是把 .ps1 塞进输入集
             （那会让每次动工具脚本都白重建一次）。票面边界：不改 ② 门语义与触发判据，只收口
             「复用判定的可信度」；HBuilderX plugins/ 内的编译器本体（G2）**本票不覆盖**（维护者 2026-10-28
             裁定取比对方案），残余风险记在票面。
 失配方向一律 **fail 向重建**（调研 §1.2 条 2 / dorny 范式）：任一组件算不出、指纹文件缺失或
 损坏、命令未记录或不一致 ⇒ 不命中 ⇒ 重建。**判不出 = 重建，绝不猜命中。**

 作用域 = 当前树（票面）：指纹文件在产物目录里 —— 跨 worktree 不共享（白烧场景即同树重跑），
 产物被干净重建（Remove-Item dist / publish 清目录）时指纹随目录一起消失，自然失效。

 为什么单独成库：同 mp-weixin-env.ps1（#1655）/ mp-weixin-pages.ps1（#1853）/ mp-weixin-preprobe.ps1
 （#1860）的理由 —— 判据必须能被**真执行**：行为守护（utils/mpWeixinBuildFingerprintBehavior.test.js）
 用夹具项目 + 产物真改真比对，钉住「输入变 ⇒ 不命中 / 产物变 ⇒ 不命中 / 版本变 ⇒ 不命中 /
 命令变 ⇒ 不命中 / 命令未记录 ⇒ 不命中 / 本轮命令缺失 ⇒ 不命中 / 什么都不变 ⇒ 命中 /
 指纹损坏 ⇒ 不命中」，文本断言在判据被摘掉时照样绿（C20 教训）。
#>

<#
 SHA256 → 小写 hex。
#>
function Get-MpWeixinSha256Hex {
    param([Parameter(Mandatory = $true)][byte[]]$Bytes)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        return ([System.BitConverter]::ToString($sha.ComputeHash($Bytes)) -replace '-', '').ToLowerInvariant()
    } finally { $sha.Dispose() }
}

function Get-MpWeixinFileSha256Hex {
    param([Parameter(Mandatory = $true)][string]$Path)
    return Get-MpWeixinSha256Hex -Bytes ([System.IO.File]::ReadAllBytes($Path))
}

<#
 目录树内容哈希（输入集与产物两个组件的共享遍历：rel`t$size`t$sha 逐行，行**按相对路径排序**后
 再哈希 —— 与遍历顺序解耦，同一棵树任意两次计算必同值）。任一文件不可读 ⇒ $null（调用方 fail 向重建）。
 排除参数全部 fail-safe：多算 = 白重建一次（方向安全），漏算 = 拿旧产物出绿（方向危险）——
 所以调用方只排除**可证非输入**的东西。
#>
function Get-MpWeixinTreeContentHash {
    param(
        [Parameter(Mandatory = $true)][string]$Root,
        [string[]]$ExcludeDirs = @(),
        [string[]]$ExcludeExts = @(),
        [string[]]$ExcludeNamePatterns = @()
    )
    if (-not (Test-Path -LiteralPath $Root -PathType Container)) { return $null }
    $lines = [System.Collections.Generic.List[string]]::new()
    $stack = [System.Collections.Generic.Stack[string]]::new()
    $stack.Push($Root)
    while ($stack.Count -gt 0) {
        $dir = $stack.Pop()
        foreach ($item in @(Get-ChildItem -LiteralPath $dir -Force -ErrorAction SilentlyContinue)) {
            if ($item.PSIsContainer) {
                if ($ExcludeDirs -contains $item.Name) { continue }
                $stack.Push($item.FullName)
                continue
            }
            if ($ExcludeExts -contains $item.Extension.ToLowerInvariant()) { continue }
            $leafLower = $item.Name.ToLowerInvariant()
            $excluded = $false
            foreach ($pat in $ExcludeNamePatterns) { if ($leafLower -like $pat) { $excluded = $true; break } }
            if ($excluded) { continue }
            $rel = $item.FullName.Substring($Root.TrimEnd('\').Length + 1) -replace '\\', '/'
            try { $lines.Add("$rel`t$($item.Length)t$(Get-MpWeixinFileSha256Hex -Path $item.FullName)") } catch { return $null }
        }
    }
    return Get-MpWeixinSha256Hex -Bytes ([System.Text.Encoding]::UTF8.GetBytes(($lines | Sort-Object) -join "`n"))
}

<#
 构建输入集哈希：app 项目根递归，排除清单（fail-safe，理由见上）——
   目录：unpackage（产物）/ node_modules / .git / .ci-verify（证据与日志）/ docs / scripts
        （门与工具脚本）/ .hbuilderx（IDE 配置）；
   扩展名：.md / .ps1 / .mjs / .log（文档、门脚本、探针、日志——均不被 uni-app x 编译）；
   文件名：*.test.js / *.spec.js（jest 守护，运行于 node、不进产物）。
#>
function Get-MpWeixinBuildInputHash {
    param([Parameter(Mandatory = $true)][string]$Project)
    return Get-MpWeixinTreeContentHash -Root $Project `
        -ExcludeDirs @('unpackage', 'node_modules', '.git', '.ci-verify', 'docs', 'scripts', '.hbuilderx') `
        -ExcludeExts @('.md', '.ps1', '.mjs', '.log') `
        -ExcludeNamePatterns @('*.test.js', '*.spec.js')
}

<#
 publish 参数规范化串：每个参数写成「长度:参数体」再用空格连接，例如「6:publish 8:mp-weixin」。
 为什么带长度而不直接 -join ：参数值可含空格（项目路径），裸连接会让 [a, "b c"] 与 ["a b", c]
 得出同一串 ⇒ 「不同命令同键」，正是要防的那个方向。长度前缀免转义、免引号歧义，且不依赖
 ConvertTo-Json 的跨版本格式差异。
 空参数集 ⇒ $null（调用方 fail 向重建）。
#>
function Get-MpWeixinBuildCommandKey {
    param([Parameter(Mandatory = $true)][AllowEmptyCollection()][string[]]$Arguments)
    if ($Arguments.Count -eq 0) { return $null }
    return (($Arguments | ForEach-Object { "$($_.Length):$_" }) -join ' ')
}

<#
 产物目录内容哈希（排除指纹文件自身）。目录不存在 ⇒ $null（调用方 fail 向重建）。
#>
function Get-MpWeixinDistContentHash {
    param([Parameter(Mandatory = $true)][string]$Dist)
    return Get-MpWeixinTreeContentHash -Root $Dist -ExcludeNamePatterns @('.mp-weixin-build-fingerprint.json')
}

<#
 HBuilderX 版本令牌：安装目录名里的版本号（HBuilderX.5.23.2026080626 → 5.23.2026080626）
 + cli.exe 内容哈希（同一目录名可被手工换芯；二进制变 = 版本变，坑位 6b 的机判形态）。
 cli.exe 不可读 ⇒ $null（调用方 fail 向重建：版本组件算不出就不写/不比指纹）。
#>
function Get-MpWeixinHBuilderXVersionToken {
    param([Parameter(Mandatory = $true)][string]$HbxRoot, [AllowEmptyString()][string]$CliExe)
    if (-not "$HbxRoot") { return $null }
    $leaf = Split-Path -Path $HbxRoot -Leaf
    $parentLeaf = ''
    try { $parentLeaf = Split-Path -Path (Split-Path -Path $HbxRoot -Parent) -Leaf } catch { $parentLeaf = '' }
    $parts = @()
    # 版本号优先从安装目录名取：本机形态 D:\软件\HBuilderX.5.23.2026080626\HBuilderX 的版本在**父目录段**，
    # 叶段只是 'HBuilderX'（2026-10-24 全量跑实测 token 落成 dir=HBuilderX）⇒ 叶取不到再看父一段；
    # 都取不到才落 dir= 兜底。cli.exe 内容哈希始终在键里 —— 它才是「同目录名换芯」的机判组件（坑位 6b）。
    if ("$leaf" -match '(\d+(?:\.\d+)+)') { $parts += "ver=$($Matches[1])" }
    elseif ("$parentLeaf" -match '(\d+(?:\.\d+)+)') { $parts += "ver=$($Matches[1])" }
    else { $parts += "dir=$leaf" }
    if (-not "$CliExe" -or -not (Test-Path -LiteralPath $CliExe -PathType Leaf)) { return $null }
    try { $parts += "cli=$(Get-MpWeixinFileSha256Hex -Path $CliExe)" } catch { return $null }
    return ($parts -join '|')
}

function Get-MpWeixinBuildFingerprintPath {
    param([Parameter(Mandatory = $true)][string]$Dist)
    return (Join-Path $Dist '.mp-weixin-build-fingerprint.json')
}

# ConvertFrom-Json 产物的安全取值：StrictMode 下访问不存在的属性会抛（门脚本 Get-Prop 同款理由，
# 库自持一份 —— 判据库不得依赖宿主脚本的内部函数）。
function Get-MpWeixinFingerprintField {
    param($Record, [Parameter(Mandatory = $true)][string]$Name)
    if ($null -eq $Record) { return $null }
    $p = $Record.PSObject.Properties[$Name]
    if ($p) { return $p.Value }
    return $null
}

<#
 指纹比对：三组件哈希全等 **且构建命令一致** ⇒ 命中；任一缺失/失配/损坏 ⇒ 不命中
 （Reason = kebab slug，进门日志）。$BuildCommand 必填（Get-MpWeixinBuildCommandKey 的产物）：
 调用方给不出命令 = 判不出 = 重建。
 @returns 有序字典：Hit（bool）/ Reason（slug）/ Detail（人读串）
#>
function Test-MpWeixinBuildFingerprintHit {
    param(
        [Parameter(Mandatory = $true)][string]$Project,
        [Parameter(Mandatory = $true)][string]$Dist,
        [AllowEmptyString()][string]$HBuilderXVersion,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$BuildCommand
    )
    $miss = { param($Reason, $Detail) return [ordered]@{ Hit = $false; Reason = $Reason; Detail = "$Detail" } }
    if (-not "$HBuilderXVersion") { return & $miss 'hbuilderx-version-unavailable' '版本令牌算不出 ⇒ 重建' }
    if (-not "$BuildCommand") { return & $miss 'build-command-unavailable' '本轮 publish 命令规范化串为空（调用方没给）⇒ 判不出 = 重建' }
    $inputs = Get-MpWeixinBuildInputHash -Project $Project
    if (-not "$inputs") { return & $miss 'input-hash-unavailable' '输入集哈希算不出（项目根不可读/文件不可读）⇒ 重建' }
    $product = Get-MpWeixinDistContentHash -Dist $Dist
    if (-not "$product") { return & $miss 'dist-unreadable' '产物目录不可读或不存在 ⇒ 重建' }
    $fpPath = Get-MpWeixinBuildFingerprintPath -Dist $Dist
    if (-not (Test-Path -LiteralPath $fpPath -PathType Leaf)) { return & $miss 'no-fingerprint-file' '指纹文件不存在（首次构建 / 产物被清）⇒ 重建' }
    $rec = $null
    try { $rec = (Get-Content -LiteralPath $fpPath -Raw) | ConvertFrom-Json } catch { return & $miss 'fingerprint-file-corrupt' "指纹文件不是合法 JSON ⇒ 重建（$($_.Exception.Message.Trim())）" }
    $recInputs = Get-MpWeixinFingerprintField -Record $rec -Name 'inputs'
    $recProduct = Get-MpWeixinFingerprintField -Record $rec -Name 'product'
    $recVersion = Get-MpWeixinFingerprintField -Record $rec -Name 'hbuilderx'
    $recCommand = Get-MpWeixinFingerprintField -Record $rec -Name 'command'
    if ("$recVersion" -ne "$HBuilderXVersion") { return & $miss 'hbuilderx-version-changed' 'HBuilderX 版本变了（坑位 6b：工具版本会翻结论）⇒ 重建' }
    # 第四道（#2048 G1）：命令变了而三个哈希没变 = 新命令从未执行过 ⇒ 盘上产物属旧命令，不得复用。
    # 记录里没有 command 字段（schema /1 的旧指纹）同样不命中 ⇒ 迁移期多重建一次，方向安全。
    if (-not "$recCommand") { return & $miss 'build-command-unrecorded' '指纹记录里没有 command 字段（/1 旧指纹或写入方漏给）⇒ 重建' }
    if ("$recCommand" -ne "$BuildCommand") { return & $miss 'build-command-changed' "publish 命令与记录不一致（记录=$recCommand 本轮=$BuildCommand）⇒ 重建" }
    if ("$recInputs" -ne "$inputs") { return & $miss 'inputs-changed' '构建输入集内容变了 ⇒ 重建（不拿旧产物出绿）' }
    if ("$recProduct" -ne "$product") { return & $miss 'product-changed' '产物目录内容变了（被换/被改/换页残留）⇒ 重建' }
    return [ordered]@{ Hit = $true; Reason = 'fingerprint-hit'; Detail = "inputs/product/HBuilderX 三组件全等 + publish 命令一致 ⇒ 自动 -SkipBuild（$fpPath）" }
}

<#
 指纹落盘（构建成功且产物合法性/appid 断言都过了之后由门脚本调用）。
 任一组件算不出、或本轮 publish 命令没给 ⇒ 不写（Saved=$false + Reason）：下次重跑必然重建，
 fail 向重建。**记的就是本轮真用过的那条命令**（比对侧同源）。
 @returns 有序字典：Saved（bool）/ Reason（slug）/ Detail（人读串）
#>
function Save-MpWeixinBuildFingerprint {
    param(
        [Parameter(Mandatory = $true)][string]$Project,
        [Parameter(Mandatory = $true)][string]$Dist,
        [AllowEmptyString()][string]$HBuilderXVersion,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$BuildCommand
    )
    if (-not "$HBuilderXVersion") { return [ordered]@{ Saved = $false; Reason = 'hbuilderx-version-unavailable'; Detail = '版本令牌算不出 ⇒ 不写指纹（下次重建）' } }
    if (-not "$BuildCommand") { return [ordered]@{ Saved = $false; Reason = 'build-command-unavailable'; Detail = '本轮 publish 命令规范化串为空 ⇒ 不写指纹（下次重建）' } }
    $inputs = Get-MpWeixinBuildInputHash -Project $Project
    if (-not "$inputs") { return [ordered]@{ Saved = $false; Reason = 'input-hash-unavailable'; Detail = '输入集哈希算不出 ⇒ 不写指纹（下次重建）' } }
    $product = Get-MpWeixinDistContentHash -Dist $Dist
    if (-not "$product") { return [ordered]@{ Saved = $false; Reason = 'dist-unreadable'; Detail = '产物目录不可读 ⇒ 不写指纹（下次重建）' } }
    $fpPath = Get-MpWeixinBuildFingerprintPath -Dist $Dist
    try {
        $rec = [ordered]@{
            schema = 'mp-weixin-build-fingerprint/2'
            hbuilderx = "$HBuilderXVersion"
            inputs = "$inputs"
            product = "$product"
            command = "$BuildCommand"
            savedAt = (Get-Date -Format 'yyyy-MM-ddTHH:mm:ss')
        }
        Set-Content -LiteralPath $fpPath -Value (($rec | ConvertTo-Json -Depth 4) + "`n") -Encoding utf8NoBOM
    } catch {
        return [ordered]@{ Saved = $false; Reason = 'save-failed'; Detail = "指纹写盘失败（$($_.Exception.Message.Trim())）⇒ 下次重建" }
    }
    return [ordered]@{ Saved = $true; Reason = 'saved'; Detail = "指纹已落盘 $fpPath（同树重跑命中即自动 -SkipBuild）" }
}
