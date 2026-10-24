<#
 ② 门 `-Pages auto` 判据库 —— 「本 PR 改动页」清单的单点真源（#1853）。

 判据语义：对 `origin/master...HEAD` 跑 `git diff --name-only`，命中 `pages/**/*.uvue` 的路径
 换算成页路由（`pages/checkin/checkin.uvue` → `pages/checkin/checkin`），去重保序。清单非空
 ⟺ diff 含 pages/**.uvue 改动（这条性质由守护 utils/mpWeixinGatePagesBehavior.test.js
 真执行钉住，不是文本断言）。

 编码层铁律（#1853 根因，两层缺一不可；根因叙事以本头注释为唯一全文，其它处只指路）：
   ① 显式 `-c core.quotepath=false` —— 与本地 `.git/config` 解耦。路径走原始 UTF-8 字节，
      不做八进制转义（quotepath=true 的转义输出会把整条路径包进双引号，行尾多一个 `"`，
      `\.uvue$` 锚就永远对不上——2026-10-24 本机实测 match=0）；
   ② ProcessStartInfo + StandardOutputEncoding=UTF8 —— 与控制台代码页解耦。本机控制台是
      gb2312 时，pwsh `& git` 直捕按 DBCS 解码：仓内中文目录「叉车维修培训学员端跨端应用」
      的 UTF-8 共 39 字节（奇数），末字节 `A8` 与后随 `/`（0x2F）被并成一个双字节对 ⇒ 斜杠
      被吞 ⇒ 行内不存在 `/pages/` ⇒ `(^|/)pages/` 恒假 ⇒ 改动页清单恒空，② 的直达取证面
      **静默**降级成只剩入口页（#1845 会话实测复现，MP_WEIXIN_RESULT shots=1 看起来还是绿的）。
   反过来，事故机写法（`& git` 直捕）在同一反条件下找不到改动页 —— 守护的必红对照当场翻红。

 为什么单独成库而不是门脚本里的内联块：同 mp-weixin-env.ps1（#1655）的理由 —— 判据必须能被
 **真执行**（真实 git 仓 + 真实 gb2312 控制台反条件），文本断言在「判据被摘掉、只留一句字面量」
 时照样绿（C20 教训）。门脚本 main 流程 dot-source 本文件并在 `-Pages auto` 分支取用。

 降级语义（诚实三分，不冒充）：
   Ok=$true  + Pages 非空  —— diff 命中改动页，直达取证面照单核验；
   Ok=$true  + Pages 空    —— diff 可用但没有 pages/**.uvue 改动（本来就没有，不是降级）；
   Ok=$false + Reason=slug —— git 判据不可用（git-unavailable / git-timeout / git-exit-<N>），
                              清单为空 = 取证面降级，调用方必须 [warn] 显式说破，不得静默。
#>

<#
 取本 PR（相对合并基）改动的 .uvue 页路由清单。
 @param Project 仓库工作树根（git -C 的落点）
 @param Range  diff 区间（缺省 origin/master...HEAD；无 origin 参照的调用方需自行兜底）
 @param TimeoutSeconds git 进程超时兜底（秒）
 @returns 有序字典：Pages（路由清单）/ Ok（git 判据是否可用）/ Reason（不可用时的 kebab-case slug）
#>
function Get-MpWeixinChangedUvuePages {
    param(
        [Parameter(Mandatory = $true)][string]$Project,
        [string]$Range = 'origin/master...HEAD',
        [int]$TimeoutSeconds = 30
    )

    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = 'git'
    # ① quotepath 显式置 false：输出形态只由这里的 -c 决定，与调用机的 .git/config 无关
    foreach ($a in @('-c', 'core.quotepath=false', '-C', $Project, 'diff', '--name-only', $Range)) {
        [void]$psi.ArgumentList.Add($a)
    }
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    # ② UTF-8 显式解码：git 的路径字节本就是 UTF-8，解码面只认这里，不认控制台代码页
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8

    try {
        $p = [System.Diagnostics.Process]::Start($psi)
    } catch {
        return [ordered]@{ Pages = @(); Ok = $false; Reason = 'git-unavailable' }
    }
    $stdout = $p.StandardOutput.ReadToEndAsync()
    $p.StandardError.ReadToEndAsync() | Out-Null
    if (-not $p.WaitForExit($TimeoutSeconds * 1000)) {
        try { $p.Kill($true) } catch { }
        return [ordered]@{ Pages = @(); Ok = $false; Reason = 'git-timeout' }
    }
    if ($p.ExitCode -ne 0) {
        # 无 origin/master 参照（detached 树）/ 非仓库等都在这里落地，Reason 保留退出码可追溯
        return [ordered]@{ Pages = @(); Ok = $false; Reason = "git-exit-$($p.ExitCode)" }
    }

    $lines = @("$($stdout.Result)" -split "\r?\n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    $pages = @(
        $lines | Where-Object { $_ -match '(^|/)pages/.+\.uvue$' } |
            ForEach-Object { ($_ -replace '^.*?/pages/', 'pages/') -replace '\.uvue$', '' } |
            Select-Object -Unique
    )
    return [ordered]@{ Pages = $pages; Ok = $true; Reason = '' }
}
