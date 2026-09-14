<#
.SYNOPSIS
    部署观察窗的**纯判定**：有界轮询「该不该收手、为什么收手」。供 hx-run.ps1 dot-source。

.DESCRIPTION
    为什么单独放在 lib 里：这是 #974 现象二的**唯一**判定点，而它必须是**可被运行期断言**的
    （ADR-0011 ⑥「关键分支必须有运行期守护」）。`hx-run.ps1` 是脚本 —— dot-source 它会整篇执行，
    没法单测其中某个分支。故把纯判定上移到本模块，测试直接 dot-source 本文件、用 fake 采样驱动
    （见 utils/hxTimingBehavior.test.js）。这与 `scripts/lib/level-detect.ps1` 的做法一致。

    判定的**优先级即不变量**（顺序不可颠倒）：

      ① 设备侧事实**前进**（`-Deployed`）⇒ **立即收手**（`advanced`）——
         一旦前进就 PASS，不多等一秒。**绝不能因为「会话已经退出」把已经落盘的事实丢掉**
         （2026-09-14 实测的假阴性正是这个形态：先看到「会话退出」就收手，而资源 7 秒后才落盘）。
      ② 未前进时，**必须等满最短观察窗** `$DeployDeadlineSeconds` 才允许因为
         「会话提前退出 / 停滞 / 到顶」收手 ⇒ 窗口内一律继续轮询。
      ③ 到顶（`$TimeoutSeconds`）仍未前进 ⇒ 收手，调用方据此判未部署（`exit=env` + exit 2）。

    为什么 ② 不能省（2026-09-14 实测）：资源落盘发生在推送到设备之后 **7–21 秒**。
    旧实现只要 `launch` 进程一退出就立刻收手（实测 `deploy=3` 秒就下了结论），于是把
    「7 秒后才落盘」误判成 `deployed=false / exit=env` ⇒ 使用者以为部署失败，白重跑一整轮编译（约 7 分钟）。
    ⇒ 窗口**不得**短于实测落盘时间；`hx-run.ps1` 的默认值取 **60 秒**（> 21 秒上限，留 3 倍余量）。

.NOTES
    纯函数：不碰设备、不碰 HBuilderX、不读文件、不写文件 ⇒ 可在任何机器上断言，也不需要 adb。
#>

function Test-DeployObservationStop {
    <#
      一次采样之后：该收手了吗？
      参数**全部由调用方注入**（采样结果、已等秒数、会话是否退出、编译段是否结束），
      故可用 fake 采样在无设备环境下做运行期断言。
      返回 @{ Stop = <bool>; Outcome = 'advanced' | 'exited' | 'stalled' | 'timeout' | 'poll' }
    #>
    param(
        [bool]$Deployed,
        [int]$PolledSeconds,
        [bool]$Exited,
        [bool]$CompileFinished,
        [int]$DeployDeadlineSeconds = 60,
        [int]$StallSeconds = 300,
        [int]$TimeoutSeconds = 900
    )

    # ① 前进即 PASS（优先级最高）
    if ($Deployed) { return @{ Stop = $true; Outcome = 'advanced' } }

    # ② 最短观察窗：未满 ⇒ 无论会话是否已退出都继续看（落盘可能晚 7–21 秒）
    $windowElapsed = ($PolledSeconds -ge $DeployDeadlineSeconds)
    if ($windowElapsed -and $Exited) { return @{ Stop = $true; Outcome = 'exited' } }
    if ($windowElapsed -and $CompileFinished -and ($PolledSeconds -ge $StallSeconds)) {
        return @{ Stop = $true; Outcome = 'stalled' }
    }

    # ③ 到顶仍未前进 ⇒ 收手判未部署
    if ($PolledSeconds -ge $TimeoutSeconds) { return @{ Stop = $true; Outcome = 'timeout' } }

    return @{ Stop = $false; Outcome = 'poll' }
}
