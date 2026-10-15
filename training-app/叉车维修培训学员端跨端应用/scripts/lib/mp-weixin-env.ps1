<#
 ② 门「环境性失败三分」判据面 —— **单点真源**（#1655，ADR-0036 处置二）。

 三分（同一前缀 MP_WEIXIN_RESULT 下三选一，互斥）：
   通过      errorsTotal=0 的 metric 行（无 errors=/failures= token）—— exit 0；
   门未过    errors=<N> failures=…（N≥1，可验证子集不过且无环境性失败）—— exit 1；
   环境受阻  errors=env-blocked reason=<slug>[ failures=…][ <metricLine>] —— exit 2。

 env-blocked ≠ 门未过：处置 = 转人工 / 改跑**不需要 DevTools** 的检查，绝不 kill 主程序（ADR-0036）。
 环境受阻是**一等出口**，不是要修的 bug —— 不伪造登录态或自动化桥（ADR-0036 ③ 不做）。

 判据锚 = 探针失败条目的固定前缀「环境不可用：」（scripts/mp-weixin-probe.mjs 只在该前缀下写环境性
 失败并 finish(true)；`环境不可用` 字样的三处：缺模块 / 连接失败 / pageStack 空）。锚定用 **StartsWith**
 而不是裸 Contains：失败详情里**引用**这句话的普通断言失败（例如「处置 = 先看『环境不可用：』说明」）
 不得被误判成环境受阻。探针的 `pageStack 读取失败：`（不带前缀）⇒ 归门未过 —— 会话存在但响应异常，
 重跑无益，判 2 等于告诉调用方「重跑即可」。

 reason 派生（本文件是唯一真源，Get-MpWeixinEnvReasonCatalogue）：
   口径 connectError.kind=not-ready   → automation-not-ready（与就绪闸门 :1400 的 reason 同名，语义收敛）
   口径 connectError.kind=unreachable → automation-unreachable
   口径 connectError.kind=timeout     → automation-timeout
   口径 connectError.kind=unknown     → automation-unknown
   找不到 miniprogram-automator       → automator-missing
   pageStack 为空                     → page-stack-empty
   直达冷启动未成（reason=X）          → X 原样派生（#1690 run5：port-held-after-quit 等，不落 probe-env 兜底）
   其余带前缀但识别不出               → probe-env（兜底，让新增环境性失败不至于裸奔）

 为什么单独成库而不是写成门脚本里的内联块：判据必须能被**真执行** —— 守护
 utils/mpWeixinGateEnvBehavior.test.js（分类矩阵 + 前缀锚必红夹具）与
 utils/mpWeixinGateResultLineBehavior.test.js（三分端到端真跑）用夹具直调这些函数，
 而不是断言源码文本（C20 先例：文本断言在「判据被摘掉、只留一句字面量」时照样绿）。
 门计划（New-GatePlan）在 dot-source 本文件**之前**装配，且 -DryRun 在 dot-source 前退出
 ⇒ 计划的 branches 只声明形状与出口码，不引本文件的函数。
#>

<#
 把探针失败条目分成三类（纯函数，无 I/O）。
 @param Failures 探针失败条目（MP_WEIXIN_PROBE JSON 的 failures 数组，可为空）
 @returns 有序字典：kind（pass / gate-failed / env-blocked）/ isEnv / envReasons（派生 slug，去重保序）/
          failureCount（非空条目计数）
#>
function Get-MpWeixinEnvClassification {
    param([AllowEmptyCollection()][string[]]$Failures)

    $items = @()
    foreach ($f in @($Failures)) {
        if ($null -ne $f) { $items += "$f" }
    }

    $marker = '环境不可用：'
    $envReasons = @()
    foreach ($f in $items) {
        if (-not $f.StartsWith($marker)) { continue }
        $detail = $f.Substring($marker.Length)
        if ($detail -match '口径 connectError\.kind=([a-z-]+)') {
            $slug = switch ($Matches[1]) {
                'not-ready'   { 'automation-not-ready' }
                'unreachable' { 'automation-unreachable' }
                'timeout'     { 'automation-timeout' }
                default       { 'automation-unknown' }
            }
        } elseif ($detail.StartsWith('找不到 miniprogram-automator')) {
            $slug = 'automator-missing'
        } elseif ($detail.StartsWith('pageStack 为空')) {
            $slug = 'page-stack-empty'
        } elseif ($detail -match '直达冷启动未成（page=[^）]*reason=([a-z0-9-]+)）') {
            # #1690 run5：直达冷启动的环境性失败自带具体 reason（port-held-after-quit / devtools-open-timeout /
            # auto-timeout / no-ready-result / automation-not-ready / port-not-listening）⇒ 原样派生，不落 probe-env 兜底。
            $slug = $Matches[1]
        } else {
            $slug = 'probe-env'
        }
        if ($envReasons -notcontains $slug) { $envReasons += $slug }
    }

    $isEnv = @($envReasons).Count -gt 0
    $kind = if ($items.Count -eq 0) { 'pass' } elseif ($isEnv) { 'env-blocked' } else { 'gate-failed' }
    return [ordered]@{
        kind = $kind
        isEnv = $isEnv
        envReasons = @($envReasons)
        failureCount = $items.Count
    }
}

<#
 环境性 reason 派生枚举（唯一真源）：守护对着它校验（slug 形状 + 门计划引用的分类器路径）。
#>
function Get-MpWeixinEnvReasonCatalogue {
    return @(
        'automation-not-ready',
        'automation-unreachable',
        'automation-timeout',
        'automation-unknown',
        'automator-missing',
        'page-stack-empty',
        'port-held-after-quit',
        'probe-env'
    )
}

<#
 门未过结果行（exit 1）：errors=<N> failures=… [metricLine]。构造器保证**永不**写 env-blocked
 —— 三分互斥由构造器保证，不靠调用方自觉。
#>
function New-MpWeixinGateResultLine {
    param(
        [Parameter(Mandatory = $true)][string]$Prefix,
        [Parameter(Mandatory = $true)][int]$Count,
        [AllowEmptyString()][string]$FailuresJoined,
        [AllowEmptyString()][string]$MetricLine
    )
    $line = "$Prefix errors=$Count"
    if ("$FailuresJoined" -ne '') { $line += " failures=$FailuresJoined" }
    if ("$MetricLine" -ne '') { $line += " $MetricLine" }
    return $line
}

<#
 环境受阻结果行（exit 2）：errors=env-blocked reason=<slug>[ failures=…][ metricLine]。
 reason 只允许小写连字符 slug（非法即抛 —— 让「reason 里混进空格/中文」的坏实现当场炸，而不是蒙混出结果行）。
#>
function New-MpWeixinEnvResultLine {
    param(
        [Parameter(Mandatory = $true)][string]$Prefix,
        [Parameter(Mandatory = $true)][string]$Reason,
        [AllowEmptyCollection()][string[]]$Failures,
        [AllowEmptyString()][string]$MetricLine
    )
    if ($Reason -notmatch '^[a-z0-9-]+$') {
        throw "env reason 只允许小写连字符 slug（得到 '$Reason'）"
    }
    $line = "$Prefix errors=env-blocked reason=$Reason"
    if (@($Failures).Count -gt 0) { $line += " failures=" + (@($Failures) -join ' / ') }
    if ("$MetricLine" -ne '') { $line += " $MetricLine" }
    return $line
}
