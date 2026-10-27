<#
.SYNOPSIS
    ④ 本地编译门两处**错误行判据**的纯函数宿主：④a（`compile-check.ps1`，dev 全量编译）与 ④c（`kotlin-all-check.ps1`，整模块 kotlinc）。供两份门脚本 dot-source，也供 ③ 门题库**真执行**。

.DESCRIPTION
    为什么抽进 lib（#2045；来源 = 血账 #4 / #5，登记见根 `docs/agents/ledger.md`）
      「这行日志算不算编译错误」的判据原先**内联写在门脚本里**，于是它只能被**读源码文本**断言 ——
      而文本断言在「字面量还在、算法被改坏」时**不会红**（`scripts/classify-guards.mjs` 头注里那条
      2026-09-18 的实测就是这个形状：改坏 `format.uts` 的 `return '0s'`，③ 门照样全绿）。
      搬进本文件后，判据可以被**真执行**：喂历史错题、看它判没判红。
      题库 = `utils/gateErrorLinesBehavior.test.js`，接线 = `utils/gateErrorLinesWiringContract.test.js`。
      形状先例：`lib/hx-errors.ps1`（第三条判据，已在 lib，其头顶写的正是同一条理由）与
      `lib/publish-freshness.ps1`。

    搬家纪律：**措辞逐字保形**
      搬家那一笔 commit 里，两条判据的 `(?i)` / `(?m)` 前缀、token 清单、否证行排除**一字未改**；
      验证 = 对同一份合成日志，搬前（脚本内联）与搬后（本库）输出全等（题库 F0 钉这条等价）。
      之后**只有一种**情形允许动措辞：题库首跑抓到的漏网族（血账 #4 的 `Could not resolve`），
      且修复与新增族同票、同 PR、附血账行号。
      —— 首跑真的抓到了（#2045：题库 11 条用例里恰有 G0/G4 两条红，其余 9 条绿），
      修复见 `Get-CompileGateErrorLines` 体内那条注释；搬家那一笔本身一字未改（等价验证：
      18 份合成样本搬前/搬后输出全等，不等=0）。

    为什么两份判据不合成一棵（合一 = 独立候选票，不在本票射程）
      **输入面本就不同**：④a 吃 HBuilderX CLI 的编译日志（插件诊断 `[plugin:uni:…]`、中文诊断、
      `已停止运行...` 收尾行都可能进来）；④c 吃**纯 kotlinc** 输出（`e: ` 前缀与 `路径:行:列: error:` 两形）。
      合成一棵之前要先裁「哪条判据对哪一路输出负责」，那是另一张票的事。

    已知边界（写实，勿读成「已覆盖」）
      - ④c 这条判据对两类**插件形态**的行不命中（`[plugin:uni:app-uts] kotlin编译失败`、
        `error: 找不到名称“region”`），逐行现测见 `docs/verification/recruiter-workspace/1207/README.md`
        的「同一批日志行对两条 pattern 的现测对照」表。那两行是 CLI/插件输出、本就不在 ④c 的 kotlinc
        输入面上 ⇒ 本票按票面「拿不准的宁缺毋滥」不为它改措辞，只登记进合一候选。
      - 第三条判据 `Get-HxErrorLines`（本目录 `hx-errors.ps1`，`hx-run.ps1` 日常路径用）**不在本文件**，
        且它同样缺 `Could not resolve` 族 —— 本票射程只到 ④ 的两处，这条单独记进根 `docs/agents/ledger.md`。

    准入门槛（对齐 `lib/gate-common.ps1` 的 G1–G3，别绕过）
      搬进共享库的代码应当「各副本逐字节相同」**且**「未被守护测试作为字面量锚点 pin 住」。
      ④c 那份**曾被** `utils/kotlinAllGateContract.test.js` 的 C3 以源码文本 pin 住
      （`$ErrorLinePattern = '...'` 与 `[regex]::Matches($result.Output, $ErrorLinePattern)` 两条锚点）。
      本次的处理是**同批改写锚点、不是放宽锚点**：C3 从「脚本内声明判据字面量」改成
      「脚本 dot-source 本库 + 调用点在真路径上」，判据内容这一侧改由**题库真执行**举证；
      并加一条反向断言「脚本内不得再留判据字面量」（防第二真源）。裁定与实测见
      `docs/adr/0012-真运行路径错误行判据的作用域.md` 的「实施回记（#2045）」段。

.NOTES
    纯函数：不读文件、不碰 HBuilderX、不碰设备 ⇒ 任何机器上都能断言，不需要 adb、不需要 `hx-agent.lock`。
    ⚠️ 返回值一律 `@()` 包裹，调用点也**必须**再包一层 `@()`：StrictMode 下无命中的裸管道退化成 `$null`，
       而 `$null.Count` 会抛（同 `hx-errors.ps1` 头注记的那条坑，票面 User Story 6）。
#>

# ---------- ④a：dev 全量编译门的错误行判据 ----------
<#
 .SYNOPSIS
     ④a 的错误行判据（自 `scripts/compile-check.ps1` 内联管道逐字搬入）。
 .DESCRIPTION
     输入 = 门日志全文（`build.log` 的 raw 内容）；返回 = 被判为编译诊断的**整行**。
     第二道管道是**否证行排除**：`0 error` / `errors=0` / `no errors` / `error count: 0` 一类
     「门自己写的结论行」不得算错误，否则这道门恒红。
     ⚠️ 本判据**不排除**带时间戳的行：④a 的输入面是 CLI 编译日志，不是设备 console 日志
        （设备日志的排除族在 `hx-errors.ps1` 那条判据上，两者输入面不同，别互相抄）。
 #>
function Get-CompileGateErrorLines {
    param([string]$Output)
    # #2045 修复（搬家后的**唯一**一次措辞改动，题库首跑抓到的现行）：
    #   补 `Could not resolve` 族 —— 这一族行内不含 `error` 一词，`\berror\b` 结构上不可能命中，
    #   于是编译不过的树照样打 `COMPILE_RESULT errors=0 clean=True` 且自陈 ✅。
    #   血账行号：根 `docs/agents/ledger.md`:18（#4，实测 `Could not resolve` 命中 1、脚本判据命中 0，#1196）；
    #   成因原文：移动端 `docs/adr/0008-移动端验收门与证据.md`「2026-09-20 修订（④ 本地编译门判据盲区）」
    #   与其「后续（刻意本轮不做）」段；逐行对照：`docs/verification/recruiter-workspace/1207/README.md`
    #   「同一批日志行对两条 pattern 的现测对照」表第一行。
    #   销账证据：`utils/gateErrorLinesBehavior.test.js` G0/G4（真执行本函数，喂上面那两行原文）。
    #   ⚠️ 同族在第三条判据 `lib/hx-errors.ps1`（hx-run 日常路径）仍缺 —— 本票射程只到 ④ 的两处，
    #   那条单独记进根 ledger，合一票处理。
    return @("$Output" -split "`r?`n" |
        Where-Object { $_ -match '(?i)\berror\b|unresolved reference|cannot infer type|找不到名称|类型不匹配|编译失败|Could not resolve' } |
        Where-Object { $_ -notmatch '(?i)0\s*error|errors?\s*[:=]\s*0|no errors?|error count\s*[:=]\s*0' })
}

# ---------- ④c：整模块 kotlinc 门的错误行判据 ----------
# kotlinc 的两种错误行形：行首 `e: `（原生错误前缀）与 `路径:行:列: error:`。
$KotlincGateErrorLinePattern = '(?m)(^\s*e: )|(:\d+:\d+: *error:)'

<#
 .SYNOPSIS
     ④c 的**计数**判据（自 `scripts/kotlin-all-check.ps1` 逐字搬入）。
 .DESCRIPTION
     返回 = 命中的**片段**数组（不是整行）—— `KOTLIN_ALL_RESULT errors=<N>` 里的 N 就是片段数。
     搬动时若顺手把它改成整行，计数语义会变（一行里出现两处 `error:` ⇒ 2 与 1 之差），
     故保持片段形状（机检行一字不动，票面 Implementation Decisions 末条）。
 #>
function Get-KotlincGateErrorMatches {
    param([string]$Output)
    return @([regex]::Matches("$Output", $KotlincGateErrorLinePattern) | ForEach-Object { $_.Value })
}

<#
 .SYNOPSIS
     ④c 的**打印**判据（自 `scripts/kotlin-all-check.ps1` 失败日志段逐字搬入）。
 .DESCRIPTION
     返回 = 被判中的整行（调用方取前 20 行贴进失败日志，`.Trim()` 也在调用方，形状不变）。
     与 `Get-KotlincGateErrorMatches` 共用同一份 `$KotlincGateErrorLinePattern` ⇒
     「计数说有过、打印却说没有」这种自相矛盾在结构上不可能出现。
 #>
function Select-KotlincGateErrorLines {
    param([string]$Output)
    return @("$Output" -split "`n" | Where-Object { $_ -match $KotlincGateErrorLinePattern })
}
