<#
 ② 门「publish 前秒级会话可达性预探」判据库 —— **否定性闸门**（#1860）。
 范式依据：docs/design/20261024-验收门成本fix与证据复用实践调研.md §1.2 条 1
 （Playwright webServer「贵步骤前先探就绪、端口被占即 fail-fast」+ 熔断器「likely to fail 就立即失败」
 + 微信 HTTP 文档官方检测法）。

 病根（#1494 §0.1）：② 的失败轮 9–26 min/次白烧在「开发者工具会话不就绪」族 —— 环境被楔死时，
 门要把 publish（≈2–3 min）+ close→open→auto 整段重试全烧完，才在就绪闸门/端口轮询暴露。
 预探把楔死态的暴露点提到 publish **之前**（秒级），白烧的构建与重试预算不再花在死环境上。

 机制（官方检测法原文：「可通过检查用户目录、检查用户目录下是否有端口文件及尝试连接来判断工具
 是否安装/启动」）：HTTP 服务随工具启动自动开启，端口号记录在用户目录的端口文件里（macOS 形态：
 ~/Library/Application Support/微信开发者工具/<MD5>/Default/.ide；官方只给了 macOS +「Windows 同理」。
 **Windows 真实路径已实测**（2026-10-24，IDE 运行中抓取，见 Get-MpWeixinIdePortFiles 头注释）：
 %LOCALAPPDATA%\微信开发者工具\User Data\<MD5>\Default\.ide，同根多 profile 各一份 —— 枚举器按
 候选根有界递归，不写死单一路径）。

 楔死态签名（**最窄**，勿放宽）：端口文件存在 + 其端口可解析 + TCP 试连失败。
 为什么是否定性闸门（调研 §1.2 条 1 的风险条款，勿改成肯定性放行）：预探判错会把「可跑」误报成
 env-blocked。故只拦上面这一种组合，其余一律放行并如实记录观察：
   - 端口文件不存在           ⇒ IDE 未运行（冷起点，门自己经 publish/open 拉起）⇒ 放行；
   - 端口文件在 + 试连可达     ⇒ IDE 服务在听 ⇒ 放行 —— **不冒充「会话已就绪」**：就绪判据
                                 仍是协议级的 SDKVersion + pageStack，归就绪闸门（坑位 5/6）；
   - 端口文件在但内容不可解析  ⇒ 未知 ⇒ 放行（宁可漏拦交给后续闸门，不误杀可跑环境）。
 处置（楔死命中时由门脚本打印，人工执行）：退出并重启微信开发者工具 / 清点残留 IDE 进程后重开；
 ADR-0008 红线不变 —— 门**绝不 kill** IDE 主程序。

 为什么单独成库：同 mp-weixin-env.ps1（#1655）/ mp-weixin-pages.ps1（#1853）的理由 —— 判据必须能被
 **真执行**：行为守护（utils/mpWeixinGatePreprobeBehavior.test.js）用夹具用户目录 + 真 TCP 监听/
 真死端口注入三态钉住「楔死 ⟺ 文件在但连不上」，文本断言在判据被摘掉时照样绿（C20 教训）。
#>

<#
 枚举候选 .ide 端口文件。**Windows 真实路径（2026-10-24 本机实测，IDE 运行中抓取）**：
 %LOCALAPPDATA%\微信开发者工具\User Data\<MD5>\Default\.ide —— 官方只给了 macOS 形态
 （~/Library/Application Support/微信开发者工具/<MD5>/Default/.ide）+「Windows 同理」，且本机同根下
 **多个 profile 各有一份**（每个安装/Profile 一个 MD5 段）。故不做单一路径假设，对每个候选根做
 **有界递归**（-Depth 4 恰好覆盖 User Data\<MD5>\Default\.ide 与历史扁平形态；-Filter 在文件系统层
 收窄，秒级完成）。候选根缺省 = 本机标准位置；测试/楔死验收用 -UserDataRoot 注入密封。
 @param UserDataRoots 候选用户目录根；空清单 ⇒ 本机标准位置（%APPDATA%/%LOCALAPPDATA% 两种工具目录名）
 @returns 存在的 .ide 文件绝对路径清单（可能为空）
#>
function Get-MpWeixinIdePortFiles {
    param([AllowEmptyCollection()][string[]]$UserDataRoots)
    $roots = @($UserDataRoots)
    if ($roots.Count -eq 0) {
        foreach ($envName in @('APPDATA', 'LOCALAPPDATA')) {
            $base = [Environment]::GetEnvironmentVariable($envName)
            if (-not "$base") { continue }
            foreach ($leaf in @('微信开发者工具', '微信web开发者工具')) { $roots += (Join-Path $base $leaf) }
        }
    }
    $found = @()
    foreach ($root in $roots) {
        if (-not "$root" -or -not (Test-Path -LiteralPath $root -PathType Container)) { continue }
        $found += @(Get-ChildItem -LiteralPath $root -Recurse -Force -Depth 4 -Filter '.ide' -File -ErrorAction SilentlyContinue |
            ForEach-Object { $_.FullName })
    }
    return @($found | Select-Object -Unique)
}

<#
 TCP 试连（预算内连不上 = $false）。只做连接，不发协议请求 —— 深层判据归就绪闸门。
#>
function Test-MpWeixinTcpConnect {
    param([Parameter(Mandatory = $true)][int]$Port, [int]$TimeoutMs = 2000)
    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $task = $client.ConnectAsync('127.0.0.1', $Port)
        if (-not $task.Wait($TimeoutMs)) { return $false }
        return $client.Connected
    } catch {
        return $false
    } finally {
        $client.Dispose()
    }
}

<#
 否定性预探：「.ide 端口文件在但其服务端口试连不上」⇒ Wedged=$true（楔死），其余一律放行。
 @param UserDataRoots    候选用户目录根（空 ⇒ 本机标准位置）
 @param ConnectTimeoutMs 单次 TCP 试连预算（毫秒；门从 GATE_PLAN timeouts.preprobeMs 取）
 @returns 有序字典：Wedged（bool）/ Observed（人读观察串，进日志）/ DeadPorts（楔死时「文件:端口」清单）
#>
function Test-MpWeixinDevToolsSessionReachable {
    param(
        [AllowEmptyCollection()][string[]]$UserDataRoots = @(),
        [int]$ConnectTimeoutMs = 2000
    )
    $files = Get-MpWeixinIdePortFiles -UserDataRoots $UserDataRoots
    if (@($files).Count -eq 0) {
        return [ordered]@{
            Wedged = $false
            Observed = 'no-port-file：.ide 端口文件不存在 ⇒ IDE 未运行（冷起点，门自己拉起）⇒ 放行'
            DeadPorts = @()
        }
    }
    $alive = @()
    $dead = @()
    $unparsed = @()
    foreach ($f in @($files)) {
        $raw = ''
        try { $raw = ("$(Get-Content -LiteralPath $f -Raw)").Trim() } catch { $raw = '' }
        $port = 0
        if (-not [int]::TryParse($raw, [ref]$port) -or $port -lt 1 -or $port -gt 65535) {
            $unparsed += $f
            continue
        }
        if (Test-MpWeixinTcpConnect -Port $port -TimeoutMs $ConnectTimeoutMs) { $alive += "${f}:${port}" }
        else { $dead += "${f}:${port}" }
    }
    if (@($dead).Count -gt 0 -and @($alive).Count -eq 0) {
        return [ordered]@{
            Wedged = $true
            Observed = "port-file-dead：.ide 端口文件在但其服务端口试连不上（$($dead -join ', ')）⇒ IDE 残留/崩溃痕迹（楔死）"
            DeadPorts = @($dead)
        }
    }
    $notes = @()
    if (@($alive).Count -gt 0) { $notes += "试连可达（$($alive -join ', ')）⇒ 放行（不冒充会话就绪，就绪判据归就绪闸门）" }
    if (@($unparsed).Count -gt 0) { $notes += "端口文件内容不可解析（$($unparsed -join ', ')）⇒ 最窄签名不拦" }
    if ($notes.Count -eq 0) { $notes += '端口文件在但无可解析端口 ⇒ 放行' }
    return [ordered]@{ Wedged = $false; Observed = ($notes -join '；'); DeadPorts = @() }
}
