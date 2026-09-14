<#
.SYNOPSIS
    真运行 / 仅编译两条路径共用的**错误行判定**纯函数。供 hx-run.ps1 dot-source。

.DESCRIPTION
    为什么单独放在 lib 里：这条判据必须能被**运行期断言**（ADR-0011 ⑥ / ADR-0012）。
    `hx-run.ps1` 是脚本 —— dot-source 它会整篇执行，没法单测其中某个分支。
    故把纯判定上移到本模块，测试直接 dot-source 本文件、喂真实日志行
    （见 utils/hxErrorLinesBehavior.test.js）。与 hx-deploy.ps1 / level-detect.ps1 的做法一致。

    ⚠️ **设备日志行必须排除**（ADR-0012，2026-09-14 实测）：
      HBuilderX 把 App 的运行 console 日志**混进同一条 stdout**，其中的 `[Error]` 是**应用业务错误**
      （实测：登录过期打出 4 行 `[dashboard] … failed: [Error] …`），不是编译期诊断。
      真运行路径若拿它判失败 ⇒ 把**成功部署**判成失败（假失败），且**吃掉 `HX_RUN_DEPLOY` 证据行**。
      判据取「行首 `HH:mm:ss.mmm`」：设备 console 日志必带该前缀，编译期诊断不带 ⇒ 能干净分开两者。

.NOTES
    纯函数：不碰设备、不碰 HBuilderX、不读文件、不写文件 ⇒ 可在任何机器上断言，也不需要 adb。
#>

function Get-HxErrorLines {
    <#
      判成败只解析 stdout（CLI 退出码恒 0）：扫 error / 编译失败 一类字样，再排除「0 error」这类否证行。
      返回**非设备日志**里的诊断行（数组；调用点必须包 @()，空数组退化成 $null，StrictMode 下 .Count 会抛）。
    #>
    param([string]$Output)
    $deviceLogLine = '(?m)^\s*\d{2}:\d{2}:\d{2}\.\d{3}'
    return @("$Output" -split "`r?`n" |
        Where-Object { $_ -notmatch $deviceLogLine } |
        Where-Object { $_ -match '(?i)\berror\b|unresolved reference|cannot infer type|找不到名称|类型不匹配|编译失败|运行失败' } |
        Where-Object { $_ -notmatch '(?i)0\s*error|errors?\s*[:=]\s*0|no errors?|error count\s*[:=]\s*0' })
}
