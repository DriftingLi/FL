<#
.SYNOPSIS
    移动端验收门 ② 的载体：微信开发者工具无报错（**半自动门**，agent 可执行并附证据；合并仍由人）。

.DESCRIPTION
    口径见 docs/adr/0008-移动端验收门与证据.md；来源 spike issue #883（全链路实测，≈4.5 分钟/次）。
    2026-09-12 修订：② 从**人工门**降为**半自动门** —— 与 ④a 同款口径（#870 裁定 B）：
    agent 会话可执行本脚本并把结果贴成 sha 绑定评论；**「执行人」栏由人签收、合并仍由人执行**。

    全链路（每一步都无人工介入，#883 实测）：
      cli open → cli project open --path <项目>（HBuilderX 只认「已导入项目」，这一步是无人值守导入入口）
      → cli publish mp-weixin --project <项目>（构建**并自己拉起**开发者工具打开项目）
      → cli.bat close（清残留自动化会话）
      → **cli.bat open --project <构建产物>（重新打开项目窗口 —— 2026-09-12 复测新增的必需步骤）**
      → cli.bat auto --auto-port <Port> --trust-project（开自动化端口）
      → node scripts/mp-weixin-ready.mjs（**就绪闸门**：等端口接受连接 + Tool.getInfo 带 SDKVersion
        + App.getPageStack 开始应答，再把端点交给 automator —— 2026-09-13 实测新增，缺它 ② 时好时坏）
      → node scripts/mp-weixin-probe.mjs（miniprogram-automator 连上：读 pageStack / console / exceptions + 截图）

    **三条实测坑位（勿删，ADR-0008 已录）**：
      1. `pageStack` 为空会让**所有 page 级 API** 报同一个误导弹错
         `Cannot destructure property 'rawPath' of 't.getPageMetaByWebviewId(...)' as it is null`
         —— 根因是自动化会话里没有页面被打开，不是版本不兼容。处置 = 先 `cli.bat close` 再 `cli.bat auto`
         （本脚本把这一步写死），并把「pageStack 非空」作为**前置断言**（探针里判 exit 2 环境不可用）。
      2. `mp.screenshot()` 挂在 **MiniProgram** 上，不在 Page 上（`page.screenshot is not a function`），
         且不返回内容 ⇒ 产物只能靠落盘 PNG 取。
      3. `page.$()` 元素级断言 2026-09-18 原测判不可用（就绪闸门早跑窗口的产物）→ 2026-10-25 #1917 裁决
         证伪（`page.$('.app-title')` 8ms 可用）→ #1919 落锁收窄：`page.waitFor(选择器)` 与导航 API 仍禁；
         `page.$` + `element.text()` 锚点断言放行（仅直达模式，8s 步超时）。

    **时序坑位 4（2026-09-12 复测钉死，直接决定这门红还是绿）**：`cli.bat close` 会把项目窗口**一起关掉**，
    而 `cli.bat auto` **不会重开** ⇒ 少了重开这一步，`pageStack` **恒空**（所有 page 级 API 报误导弹错）。
    故顺序写死为 `close → open --project <dist> → auto`（守护测试 C12），且 `open` 是**显式步骤并写进日志**；
    `-SkipBuild` 下同样定位构建产物目录（`$dist` 在两个分支里都已做过存在性校验）。
    补证：插入 `open` 后 `pageStack=["pages/index/index"]`、`errorsTotal=0`。
    ⚠️ 2026-09-13 补正：这一步「必需」的结论**未与就绪窗口隔离**（见下面坑位 5）——当时那次的绿也可能
    只是会话恰好已经就绪。本步保留（close 确实会把项目窗口一起关掉），但**不得**再据它宣称
    「缺 open 必然 pageStack 恒空」；判据以坑位 5 的就绪闸门为准。

    **时序坑位 5（2026-09-13 实测定位，② 时好时坏的全部原因，直接决定这门红还是绿）**：
    `cli.bat auto` 返回**不等于**自动化会话可用。实测三个里程碑分得很开
    （本机 Stable v2.01.2510290 × miniprogram-automator@0.12.1）：
      · 端口开始接受连接          t ≈ 1s        （auto 返回后仍有一小段会 ECONNREFUSED）
      · `Tool.getInfo` 带 SDKVersion  t ≈ 1–3s    —— 早于此，result 里**只有 version、没有 SDKVersion**
      · `App.getPageStack` 开始应答   t ≈ 24–34s  （两次实测 24s / 34s，波动大）
    两条门红都由此产生，且**都被误报成了「端口连不上」**：
      a) `automator.connect()` 内部的 `checkVersion()` 紧跟 ws 打开之后执行，落在第二个里程碑之前 ⇒
         `licia/cmpVersion` 对 `undefined` 调 `.split('.')` ⇒ 抛
         `Cannot read properties of undefined (reading 'split')`，再被 `connectTool` 的 catch 换成
         “Failed connecting to …” ⇒ 看上去完全是端口问题；
      b) 探针的 `mp.pageStack()` 超时原本取 30s，**正落在 24–34s 这段就绪窗口中间** ⇒ 同一份代码
         有时绿有时红。这不是竞态，是**就绪等待不足**。
    处置：`close → open → auto` 之后**先跑就绪闸门**（第 4.9 步），等齐两个里程碑再交给 automator；
    探针的 `--timeout-ms` 同步抬到 `-ProbeTimeoutMs`（默认 60s）。探针侧另用 `connectError.kind`
    （not-ready / unreachable / timeout / unknown）把两类失败分开报，**不得**再合并成一句
    「自动化端口连不上」——那正是本轮误诊把排查引向端口、多花了一轮实测的原因。

    **时序坑位 6（2026-09-23 实测定位；② 的「第一枪常空、重跑即绿」就是这一条，已按整段重试处置）**：
    `cli.bat auto` 回显 `√ auto` **不等于**自动化端口起来了。冷起点 / IDE 重启窗口下实测两种形态：
    a) 端口在 `-PortWaitSeconds`（180s）内**始终不监听**；b) `auto` **直接挂住 >180s**。而**同一条命令**
    等 IDE 稳定后重跑，**1–8s** 就监听（对照实测：`auto#1` 连跑 60s 不起 → 等 90s → `auto#2` t=1s 起）。
    故 `auto → 端口` 段做成**整段重试**：`close → open → auto` 再来一遍，最多 `-AutoAttempts` 次，
    两次之间等 `-AutoRetryDelaySeconds` 秒；全部用尽才判 `exit 2`（环境不可用）。
    ⚠️ **不得只重试 `auto`**：实测只重跑 auto 会绑到一个**没加载项目的新 IDE 实例**上 —— 端口通了，
    但 `Tool.getInfo` **没有 `SDKVersion`**、`App.getPageStack` **永不应答**（复现结果行
    `reason=sdk-version-missing`），门会以为「会话已就绪」而实际什么都没有。判据始终是**协议级**
    （SDKVersion + pageStack），**不是**「端口出现在 `Get-NetTCPConnection` 里」。
    另照实记（**硬前提的补充**）：**受限沙箱下开发者工具 CLI 连自己的端口文件都写不了** ——
    `× #initialize-error: Error: EPERM: operation not permitted, open '<User Data>\Default\.cli'`，
    于是 close / open / auto **三步全废**、自动化根本起不来（表象与「工具没装」几乎一样：
    `[error] Please ensure that the IDE has been properly installed`）。开发者工具 CLI 与 HBuilderX CLI
    一样**须以全访问权限执行**。

    **坑位 6 续（2026-09-23 深夜实测，两条，均已落锁）**：
      a) **残留自动化服务会被静默复用**（本脚本已改：**换端口**，不再只 warning）。请求端口被占时
         `cli.bat auto` 回显 `√ auto`、端口也在监听、ws 也接得上，但 `Tool.getInfo` **全程无应答**
         （就绪闸门 `reason=sdk-version-missing`）—— 与 (1) 里「复用没加载项目的会话」同族。
         实测处置：换一个空闲端口重跑，同一条命令立刻恢复（换成 9431 后 ② 一次通过）。守护：
         `utils/mpWeixinGatePortFallbackBehavior.test.js`（必不红：被占 ⇒ 换；必红：空闲 ⇒ 沿用）。
      b) **同一个工具版本可能是坏的**：本机 `2.01.2510290`（二进制 2026-03）上 `cli.bat auto` 会把 IDE
         **一起带走**（`WeappLog\launch.log`：`监视到子进程(39428)退出, 共运行了79秒 / Dump Files: nothing`），
         端口不起、连 `SDKVersion` 都没有，而 `close` / `open` / `auto` **全部回显成功** —— 命令行侧完全看不出坏。
         换装 `2.02.2608070` 后，同一棵树 / 同一份产物 / 同一条命令 **② 一次通过**。
         ⇒ 判据：`auto` 之后端口不起**且** `Tool.getInfo` 无 `SDKVersion` 时，**先核对工具版本**，别在产物/代码上找。
      c) **GPU 加速是 Tool 通道假死的真根因（2026-09-24 用户实测定位）**：现象与 a) 全同（端口 t=0 监听、
          ws 接得上、`Tool.getInfo` 120s 全程无应答、`reason=sdk-version-missing`），杀净 IDE / 冷启动 /
          手动 open→auto 均无效，且当时工具已是 `2.02`（b 的版本判据解释不了）。用户在开发者工具里
          **关闭 GPU 加速**后通道即恢复。⇒ 判据修正：端口起而 Tool 通道假死时，**先关 GPU 加速**，
          再核残留会话（a），最后才核对工具版本（b）。
       d) 顺带修掉一处真缺陷：无截图（失败路径）时 `Publish-ScreenshotArchive` 在 StrictMode 下抛
         「在此对象上找不到属性"Sum"」（`Measure-Object` 对空管道不产出对象）⇒ 收成空安全的 `Get-MadeTotalBytes`；
         它只 warning、不影响门结论，但会把整段入库打成异常。

    **导航不可用时诚实降级（2026-09-12 复测，**不许假绿**）**：本机 `miniprogram-automator@0.12.1` 下
    `mp.reLaunch` / `mp.navigateTo` **恒报 `Uncaught [object Object]`**，而 `mp.connect` / `mp.pageStack` /
    `mp.screenshot` 正常 ⇒ 「逐页导航 + 每页截图」这组断言在此环境**不可能成立**。处置是探针把这组记成
    **SKIP（reason=navigation-api-unsupported）**：既不算通过、也不算失败，并在结果行显式写出
    `navigation=skip(unsupported)`，PR 评论同步写明。门只对**可验证子集**作结论，即探针 assertions 里的：
    `connected` + `pageStackNonEmpty` + `entryPageInStack` + `noConsoleErrors` + `noExceptions`
    + `currentPageScreenshot`（**至少 1 张当前页截图**）；**降级时** `failures=` 只由这几条产生
    （`allRoutesVisited` / `screenshotsProduced` 在降级时值为字符串 `skip`，**不得**写成 `true`，见守护测试 C13；
    导航在可用机器上时这两条按「每步导航后当前页 == 目标页」判，**不拿导航前读到的 pageStack 去比所有路由**——
    那样恒假、会让健康机器上的门永远过不去）。
    结果行的 `navigation=` 取值域：`ok`（逐页导航可用，或直达探针全部达成）/ `partial(ok=<N>,skip=<M>)`（直达探针部分达成：
    N 页达成、M 页未达成——未达成页的读数不得连累已达成页，见 #1816）/ `skip(reason)`（降级）/ `n/a`（环境早退，没走到导航）。
    **未证实**：是否为版本组合（automator 0.12.1 × 开发者工具 Stable v2.01.2510290）问题，**留给后续排查**，
    不得据此宣称「已定位根因」；**首跑校准**要求人工核对结果行的 `navigation=` 字段（ADR-0008 ② 段）。

    **任意页直达探针（#1682 · ADR-0039 拆分步骤 2）**：上面的 SKIP 只证明入口页 ⇒ 取证面用 `-Pages` 扩到被改页。
    机制（已裁定，勿另选）= ADR-0038 决策 1 的「构建产物 `app.json` 首项临时换页 + plain 冷启动」——**不调用任何
    导航 API**，页身份判据 = 冷启动落地页 `pageStack[0]` 等于目标路由（真机侧同手法先例 #1678）。`-Pages auto`（缺省）
    = 入口页 + 本 PR 改动页（git diff 命中 `pages/**/*.uvue`）；**清单为空**（改 `components/` / `utils/*.uts` /
    `App.uvue` / `pages.json`）时退**固定冒烟集两页**、结果行给 `coverage=smoke:2`（#2101：否则只跑入口页一张 =
    覆盖为零的绿），git 判据不可用 ⇒ `coverage=none` 且**不**退化冒烟集；`-Pages none` 关闭直达走旧导航组；`-Pages <csv>` 指定清单。
    每页取证 = 截图（落日志目录；只有显式 `-Archive` 才另存一份到 `docs/verification/<模块>/<号码>/`）+ 该页 console error/exception
    读数（`errorsTotal` 语义不变：逐页累加，重试不重复计数）。未达成页**逐页**记入 `navigation.pages`（#1816），组级形态：
    全部达成 ⇒ `ok`、部分达成 ⇒ `partial(ok=<N>,skip=<M>)`（已达成页读数照常计入，**不得**被未达成页整组塌缩）、
    全部未达成 ⇒ `navigation=skip(reason=…)`（reason ∈ direct-page-unsupported / page-not-in-build / app-json-invalid），
    **三种都既不当通过也不当失败**；
    直达冷启动/会话失败 ⇒ env-blocked（exit 2）。逐页读数行（`direct-page …` / `direct-total …`）进日志与
    sha 绑定评论，供 0039 窗口主门取证成本校准。换页只动 dist 构建产物，**源头 `pages.json` 一个字不改**（ADR-0038 决策 2）。

    **Pre-flight quit 清场（#1634）**：用脚本启动（IPC 链）时，残留/堆叠的 IDE 实例会让 `open` 把项目窗口
    叠进一个未初始化的实例 ⇒ 项目窗口白屏（连菜单/Console 都没有；直接双击 exe 启动不复现）。故跑门前先
    `cli.bat quit` 退出整个 IDE 并轮询 wechatdevtools 进程归零（60s 超时只 [warn]），再走 close → open → auto。
    quit/轮询失败不判 exit 2；**红线：不得用 Stop-Process 替代 quit**（运行中 kill IDE 是 ADR-0008 红线）。
    步骤与预算声明在门计划的 `devtools-quit` 步（唯一真源）；-Doctor 跳过（体检不操控 IDE）。
    #1860 收紧：quit **之前先清点 IDE 进程** —— 0 个 ⇒ 不执行 quit。`cli quit` 在 IDE 未运行时会**自启**
    一个无项目实例并占住服务端口（官方 `--port` 全局选项「没启动则会启动」的同族机制；2026-10-23 本机实测：
    自启实例占 43005 ⇒ open 120s 超时 ⇒ 一轮 9–26 min 白烧）。quit 只在「确实有 IDE 在跑」时发起。

    **② 白烧收敛三件套（#1860）**：把「环境被楔死」的暴露点从 publish+open 之后（9–26 min/轮）提到秒级
    （范式依据 docs/design/20261024-验收门成本fix与证据复用实践调研.md §1.2：Playwright webServer
    贵步骤前先探就绪 / 熔断器「likely to fail 就立即失败」/ 微信 HTTP 文档官方检测法）。
      ① pre-flight quit 防自启（上段）。
      ② **publish 前秒级预探**（门计划 `env-preprobe` 步，判据库 `scripts/lib/mp-weixin-preprobe.ps1`）：
         读 `.ide` 端口文件 + TCP 试连，**否定性闸门** —— 只拦「端口文件在但试连不上」的楔死态
         （exit 2 env-blocked，reason=ide-port-unreachable，在 publish 之前退出、不进重试循环），
         其余一律放行并**不冒充**「会话已就绪」（就绪判据仍是协议级的 SDKVersion + pageStack，归就绪闸门）。
      ③ **产物内容指纹自动 -SkipBuild**（判据库 `scripts/lib/mp-weixin-build-fingerprint.ps1`）：
         指纹键 = 构建输入集内容哈希 + 产物目录内容哈希 + HBuilderX 版本令牌，三组件全等 ⇒ 自动跳
         publish（同树重跑省 ≈2–3 min，#1443 实测）；任一组件失配 / 算不出 / 指纹损坏 ⇒ 重建
         （fail 向重建，不拿旧产物出绿）。指纹落产物目录内（`.mp-weixin-build-fingerprint.json`）
         ＝ 作用域当前树，跨 worktree 不共享，产物被清时自然失效。

    **判成败一律解析输出，绝不看退出码**：HBuilderX CLI 失败时退出码恒为 0（如实测「项目不存在，请先导入」仍返回 0），
    开发者工具 `cli.bat` 与探针的退出码同样不作为门结论。

    **端口监听面的风险（照实记，未做加固）**：`cli.bat auto --auto-port <Port>` 的监听地址由开发者工具决定，
    实测 `Get-NetTCPConnection` 显示绑定在通配地址（`::`）而非仅 `127.0.0.1` ⇒ **自动化端口在局域网内可被访问**。
    本机为开发/CI 前置机时风险可接受；在**共享网络**上跑本门应先确认防火墙。本脚本无法从 CLI 侧收窄监听面
    （`cli auto -h` 无绑定地址参数），故只在此声明，不假装已处理。

    **门计划 `GATE_PLAN` 与 `-DryRun`（#914 收束后守护的判据面）**：本脚本「步骤序列 / 每步参数与判据 /
    超时预算与 exit 2 归属 / 结果行字段清单」四类不变量**集中在 `New-GatePlan` 一处声明**，执行路径按 id
    取用（`Get-GateStep`）——调用点不再自带参数原文。`-DryRun` 只输出**一行** `GATE_PLAN {json}` 后 exit 0：
    **不执行任何外部命令、不写工作树、不接 HBuilderX**（可在没有 HBuilderX / 没有开发者工具的机器上跑，
    包括 Linux CI）。守护 `utils/mpWeixinGateContract.test.js` 的 C2 / C12 / C15 / C18 改为**运行 `-DryRun`
    并断言这份 JSON**，不再断言调用点源码文本 —— 这样把调用点原文抽进共享库时，守护不会失配（原口径下
    唯一的「修法」是放宽守护，等于把防护拆掉）。决策与代价见 `docs/adr/0008-移动端验收门与证据.md`。

    **build-only 模式 `-BuildOnly`（#1861 第一段）**：同一条 ② 链路的**构建面**载体 —— 跑到「publish 成功行 +
    产物合法性（#1210）+ 指纹落盘」为止，**会话段整段跳过**（pre-flight quit / close→open→auto 重试单元 /
    就绪闸门 / 探针 / 截图入库 / 贴门评论），不打开自动化会话、不等页面就绪。入口
    `npm run build:mp-weixin-buildonly`；形态沿 ④a 的 `hx-run.ps1 -CompileOnly`（官方语义「仅编译代码」，
    同族范式：开关 + 真跑 + 真跳过，不抽库、不建薄壳）。
    补的是 #1670 盲格：纯 `.uvue` / `.uts` 改动（没有 MP-WEIXIN 行、没有三份 json）时 ② 的判据认不出编译面，
    ADR-0043 决策 3 只写了「靠纪律」。**它不是 ② 的替代** —— console error / exception / pageStack / 截图
    一概未取证 ⇒ 建成它**不构成**降频 ② 的理由（#1861「边界（已裁定）」段；C1 裁定不删门）。
    三条与 ② 不同的处置（#1861 2026-10-28 裁定）：
      ① **不跑 appid 断言**（源 manifest 那条 fail-closed 与产物 appid 那条都跳过）。理由：不上传 ⇒ 产物落
         `touristappid` 无害（`Get-MpWeixinProductReport` 只解析取值、不比对期望），而 HBuilderX 首次导入/编译
         会回写工作树 `manifest.json` 把 `mp-weixin.appid` 置 null（ADR-0008 实测，mtime 与 publish 启动同秒），
         保留断言 ⇒ **第二次起必假红**。实读到的 appid 仍写进结果行，不藏。
      ② **跑完打印脏改提示**（`git status` 核对 `manifest.json` / `pages.json`；脏了 `git checkout --` 还原，
         `pages.json` 的 `condition` 段属本地开发配置、禁止提交）；脚本自身**不自动改工作树**。
      ③ 结果行 token 换成 **`MP_WEIXIN_BUILDONLY_RESULT`**（与 ② 的 `MP_WEIXIN_RESULT` 可判读分流，同
         `TEST_UNIT_LOCAL_RESULT` 先例），三条产出路径（通过 / 门未过 / 异常）都用它 —— 前缀只有一个取值点
         （门计划的 `resultLine.prefix`），不靠调用方自觉。
    退出码与 ② 同形（0 通过 / 1 产物不合法 / 2 环境受阻）；**不产出 ② 的证据块**（`-PostToIssue` 只提示忽略，
    ⑤ 行的证据形态由 #2049 落地；A1 #2100 起全部门脚本都只打印块、不贴评论）。

    **非等价声明（勿删）**：② ≠ ① 真机门，也 ≠ ④b 云打包门。
      ② 只证明「小程序端在开发者工具里打开入口页 + 被改页时 console 无 error、无 exception」；
      **挡不住**的类别（#883 逐条实测）：`content://` 上传、生物识别门控（开发者工具明确回
      `checkIsSupportSoterAuthentication:fail … 请使用真机进行开发`）、第三方 SDK 回调、
      真机性能/内存/ANR/厂商 ROM 差异 ⇒ 这些仍只能靠 **① 真机门** 与发版前的全量逐页冒烟兜底。

    运行前提（硬前提）：HBuilderX CLI 靠与主程序的**本地 IPC**；受限沙箱下会报「与主程序的连接已中断」，
    须以全访问权限执行（同 ④a）。开发者工具需已登录（`cli.bat islogin` → `{"login":true}`），
    登录态过期时需人补扫一次码。

    结果行三分（#1655 / ADR-0036 处置二；同一前缀 MP_WEIXIN_RESULT 下三选一，互斥）：
       通过       metric 行本身（**无** errors= token；`errorsTotal=0` 承载通过语义）—— exit 0；
       门未过     `errors=<N> failures=…`（可验证子集不过，N≥1）—— exit 1；
       环境受阻   `errors=env-blocked reason=<slug> [failures=…][ <metric>]` —— exit 2。
       env-blocked ≠ 门未过：环境性失败是一等结果行（分类判据 = scripts/lib/mp-weixin-env.ps1），
       处置 = 转人工 / 改跑**不需要 DevTools** 的检查，绝不 kill 主程序。

    退出码：0 = ② 通过；1 = 门未过（**产物不合法**（#1210）/ console error / exception / 页面未打开 / 截图缺失）；
    2 = 环境受阻（env-blocked：缺 CLI / 连不上 / 超时，reason= 指明是哪一类）。**产物坏了判 1、不判 2** —— 判 2 等于告诉调用方「重跑即可」，
    而重跑会得到同一份坏产物（判据见 `scripts/lib/mp-weixin-product.ps1` 与 ADR-0008 的「② 产物合法性判据」段）。

.PARAMETER Project
    项目根目录。缺省为本脚本上一级目录（scripts/ 位于项目根下）。
    注意：HBuilderX 按**项目名（目录名）**解析，同名目录会被误命中 ⇒ 用唯一目录名跑本门（#883 坑位）。

.PARAMETER HBuilderX
    HBuilderX 安装根（需含 cli.exe 与 plugins/）。缺省按「正在运行的 HBuilderX → $env:HBuilderX_HOME → 常见安装位置」探测。

.PARAMETER Cli
    cli.exe 路径。**显式给出即权威**：不可用直接 exit 2，不再回退探测。

.PARAMETER DevToolsCli
    微信开发者工具 cli.bat 路径。缺省按常见安装位置探测。

.PARAMETER AppId
    期望的微信小程序 appid。**默认真实 appid**（与 manifest.json 的 mp-weixin.appid 同值）。
    用途有二：① 作为**门的前置断言**——校验构建产物 project.config.json 落盘的 appid 与之一致；
    ② 传给 `cli publish mp-weixin --appid <AppId>`。实测（#883 + 本次复测）：该参数对**构建产物无影响**，
    产物 appid 一律取自 manifest.json 的 `mp-weixin.appid`；manifest 缺该项时落成 `touristappid`（游客 appid）。
    传空串（-AppId ''）可跳过该断言。

.PARAMETER Port
    自动化端口（默认 9420）。传给 `cli.bat auto --auto-port <Port>`。

.PARAMETER Routes
    待验页面路由（逗号分隔）。**第一个 = 入口页**（用 reLaunch 打开），其余按 navigateTo 打开。
    缺省 `pages/index/index,pages/login/login`（与 #883 实测口径一致：入口页 + 一个被改页）。

.PARAMETER SkipBuild
    跳过 `cli publish mp-weixin`（构建）。适用于已有新鲜产物、只想重跑自动化判定的场合。
    注意：跳过构建后开发者工具里打开的仍是上一次的产物，门结论只对那份产物成立。

.PARAMETER BuildOnly
    构建面模式（#1861 第一段）：只跑「publish + 产物合法性 + 指纹落盘」，会话段整段跳过（不等页面就绪、
    不开自动化会话、不截图、不贴评论）。退出码与 ② 同形（0 通过 / 1 产物不合法 / 2 环境受阻），
    结果行 token 为 `MP_WEIXIN_BUILDONLY_RESULT`。
    可与 `-SkipBuild` 同时给出（= 只查现有产物的合法性，结果行写 `build=skipped(explicit)`，**不假装构建过**）；
    与 `-Doctor` 互斥（体检不构建、本模式要构建 ⇒ 同给按用法错误 exit 2）。
    `-BuildOnly` 下**不跑** appid 断言（理由见脚本头），实读 appid 仍写进结果行。

.PARAMETER AutoAttempts
    `close → open → auto → 端口` 这一段的**整段**重试次数（默认 3）。实测冷起点 / IDE 重启窗口下
    第一次 `auto` 会「回显 √ 但端口不起」或「直接挂住」⇒ 单发必红（见脚本头坑位 6）。
    `-AutoAttempts 1` = 恢复单发语义（排障用）。**重试单元是整段**，不是只重试 `auto`（理由见坑位 6）。
    **本参数只管 auto 段**：逐页冷启动（直达探针）的重试固定 2 次，取自门计划 `directPages.attempts`，
    不读本值（#2101 落库时按实测语义把两者拆开，理由见计划里那处注释）。

.PARAMETER AutoRetryDelaySeconds
    两次整段尝试之间的等待秒数（默认 20）：让开发者工具把上一次的 IDE 实例/端口收干净再重来。

.PARAMETER PostToIssue
    > 0 时，**仅在门通过（exit 0）分支**把证据块**打印到终端** —— A1（#2100）起**不再贴任何评论**。
    打印出来的块就是该粘进 PR `## 验收证据` 段的那一段：结论行（含 MP_WEIXIN_RESULT）与产物路径 + 复现命令。
    校验器只读 PR 正文（一行 `跑于 <sha7>` + 门读数行），「贴在哪个 sha 上」不再参与判定，
    因此不再校验 sha 可达性、不发任何 gh 写请求；号码只作**批次标识**（以及 `-Archive` 时的归档目录键）。
    投递退役不是失败：不打印警告，**不影响门的结论**（退出码 0/1/2 语义不变）。**本参数无写副作用**。

.PARAMETER PostToPr
    `-PostToIssue` 的**别名**（同值，一个版本后退役）。

.PARAMETER Archive
    **显式开启截图入库**（默认关闭）。开启后门通过时把截图压缩后 `git add` + `commit` + push 到
    `docs/verification/<模块>/<号码>/`，即**门自己会给分支造一个 commit 并挪走 head sha**。
    A1（#2100）的裁定是「门证据不入库、图按需贴」，所以这件事必须是显式动作，不能由传号码顺带发生
    —— 否则正文里的 `跑于 <sha7>` 会从 head 变成祖先。只是要证据块的话**不要**加这个开关。
    入库任何失败都只警告，不影响门的结论（维护者 2026-09-12 要求）。

.PARAMETER NoArchive
    `-Archive` 的**反向逃生开关**（优先级更高）：同时给 `-Archive -NoArchive` 时以 `-NoArchive` 为准。

.PARAMETER IdeUserDataRoot
    预探（#1860）的 `.ide` 端口文件用户目录根**注入点**（行为守护与楔死态验收用）。
    缺省空串 = 探测本机标准用户目录（%APPDATA% / %LOCALAPPDATA% 下两种工具目录名）。
    显式给出即**只**探这个根 —— 不碰真实用户目录，测试夹具因此可密封。

.PARAMETER DryRun
    只打印门计划（单行 `GATE_PLAN {json}`）后 exit 0：**不执行任何外部命令、不写工作树、不接 HBuilderX**。
    用途是让守护断言「行为输出」而不是源码文本（#914）：JSON 覆盖步骤序列 / 每步参数与判据 / 超时预算与
    exit 2 归属 / 结果行字段清单。与 `-Doctor` 同时给出时 `-DryRun` 优先（先打印计划即退出）。

.EXAMPLE
    npm run build:mp-weixin-check
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -SkipBuild
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -Routes 'pages/index/index,pages/login/login'
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -PostToPr 884          # 只打印证据块，不写盘
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -PostToPr 884 -Archive  # 显式开截图入库（会造 commit）
    pwsh -NoProfile -File scripts/mp-weixin-check.ps1 -DryRun
    npm run build:mp-weixin-buildonly          # = -BuildOnly：只跑构建面（#1861）
#>
[CmdletBinding()]
param(
    [string]$Project,
    [string]$HBuilderX,
    [string]$Cli,
    [string]$DevToolsCli,
    [string]$AppId = 'wx38c3e31b16a7ced0',
    [int]$Port = 9420,
    [string]$Routes = 'pages/index/index,pages/login/login',
    [string]$Pages = 'auto',
    # #2097：截前滚动位置（px）。目标区在首屏之下时必给 —— 否则截图拍的是标题、改动在屏幕外，
    # 那种「证据」不成立（#1939 已踩一次）。只对直达冷启动分支生效，整屏滚动（wx.pageScrollTo）。
    [int]$PageScrollTop = 0,
    # #2097：逐页截图模式。'full'（默认）= 逐屏滚到页尾各截一张（-p1/-p2/…），
    # 让「改动落在第几屏」不再决定证据有没有用；'single' = 旧行为（只拍首屏）。
    [ValidateSet('full', 'single')][string]$Shots = 'full',
    # 会话注入（#1690）：auto = 直达循环前与每页冷启动后注入登录态（幂等，已登录即 existing）；off = 不注入。
    [string]$Auth = 'auto',
    [switch]$SkipBuild,
    # 构建面模式（#1861 第一段）：跑到「publish + 产物合法性 + 指纹落盘」为止，会话段整段跳过。
    [switch]$BuildOnly,
    [int]$PublishTimeoutSeconds = 900,
    # 构建段「按产物收流」（#2163）两旋钮，只在 publish 一步生效：
    #   Quiet  = 产物最新文件距今至少这么多秒才算「写完了」（实测产物是末尾一次性落盘，643 文件
    #            出现在同一个 ≤5 s 窗口里 ⇒ 15 s 足够，且能盖住「页 .js 齐了但公共 chunk 还在写」）。
    #   Grace  = 收流判据成立后**再**等 cli 自然退出这么多秒。实测健康跑的空洞 ≈90 s，落在窗口内
    #            ⇒ 健康跑行为与改动前一致（仍读得到成功文案）；只有 B 形态（空洞无界）才改走产物依据。
    [int]$PublishSettleQuietSeconds = 15,
    [int]$PublishSettleGraceSeconds = 120,
    [int]$AutoTimeoutSeconds = 180,
    [int]$AutoAttempts = 3,
    [int]$AutoRetryDelaySeconds = 20,
    [int]$OpenTimeoutSeconds = 120,
    [int]$PortWaitSeconds = 180,
    [string]$Module = 'mp-weixin',
    [switch]$NoArchive,
    # A1（#2100）之后：截图入库从「传号码就发生」改成「显式 -Archive 才发生」。
    # 旧默认下为了拿证据块**必须**传 -PostToPr，于是「正常拿证据」会顺手 commit + push 挪走 head sha。
    [switch]$Archive,
    [int]$HxWaitSeconds = 600,
    [switch]$HxNoWait,
    [int]$ProbeTimeoutSeconds = 180,
    [int]$ProbeAttempts = 6,
    [int]$ProbeRetryDelaySeconds = 10,
    [int]$ProbeTimeoutMs = 60000,
    [int]$ReadyWaitSeconds = 120,
    [switch]$Doctor,
    # #1770：号码（票号/PR 号）的三处用途：①归档目录键 ②展示副本落点 ③无 PR 时的批次标识。
    # `-PostToIssue` 是正式名；`-PostToPr` 保留为**别名**（同值，一个版本后退役），旧调用点一字不改。
    [int]$PostToIssue = 0,
    [int]$PostToPr = 0,
    # 预探（#1860）的 .ide 用户目录根注入点：空 = 标准位置；显式 = 只探该根（测试密封用）
    [string]$IdeUserDataRoot = '',
    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# #1770：验收证据主键从 PR 号改成 commit sha。`-PostToIssue` 是正式名（票号），`-PostToPr` 是**别名**
# （同值，一个版本后退役）—— 归一成一枚后，下文一律沿用旧字形 `$PostToPr`，不制造第二份来源。
if ($PostToIssue -gt 0) { $PostToPr = $PostToIssue }

$DistRelative = 'unpackage\dist\build\mp-weixin'
$ProbeRelative = 'scripts\mp-weixin-probe.mjs'
$ReadyRelative = 'scripts\mp-weixin-ready.mjs'
$AuthRelative = 'scripts\mp-weixin-auth.mjs'
$WxDevToolsPatterns = @(
    'D:\微信web开发者工具\cli.bat',
    'D:\软件\微信web开发者工具\cli.bat',
    'C:\Program Files (x86)\Tencent\微信web开发者工具\cli.bat',
    'C:\Program Files\Tencent\微信web开发者工具\cli.bat',
    "$env:LOCALAPPDATA\微信web开发者工具\cli.bat"
)
$script:closed = $false
# pre-flight quit 的幂等哨兵（#1634）：与 $script:closed 同款，必须先初始化 —— 严格模式下
# 未设置的 $script:quitDone 会在 if 判定时抛「无法检索变量」（m00099 夹具实测：整段重试一次都没跑就 exit 2）
$script:quitDone = $false
# 楔死早退路径的收尾哨兵（#1860）：预探判楔死 exit 2 时，finally 的 close 会读到死端口 ⇒ cli **自启**
# 一个 IDE 再去关它（2026-10-24 楔死验收实测：close 输出「IDE server started successfully…43005」）。
# 本路径从未创建自动化会话、无残留可收，楔死处置本就指向人工 ⇒ 置位跳过 close。
$script:skipFinalClose = $false
# 任意页直达探针（#1682）的 app.json 备份哨兵：与 $script:closed 同款先初始化（严格模式下
# 未设置会在 finally 判定抛「无法检索变量」）；Restore-DistAppJson 以「备份为 null」当幂等哨兵。
$script:AppJsonBackup = $null
# 门计划引用（#1861）：结果行的前缀、步骤、预算、exit 2 归属都从它取 ⇒ 未建计划前保持 $null，
# 好让 `Get-GateResultPrefix` 在早期 exit 路径上安全回落到 ② 的默认前缀（严格模式下未定义变量会抛）。
$script:GatePlan = $null
# 指纹命中而自动跳过 publish 的哨兵（#1861）：-BuildOnly 的结果行要如实区分
# build=publish / build=skipped(fingerprint) / build=skipped(explicit) —— 不能把「没构建」写成「构建过」。
$script:buildSkippedByFingerprint = $false
# -PortWaitSeconds 由 90 抬到 180（2026-09-13）：-Doctor 首跑实测到一次「cli.bat auto 回显 √ auto、
# 端口 90s 内始终没监听」，紧接着同一命令 8s 内就起来了 ⇒ **这一条只抬预算，不是修复**：病根未定位，
# 已如实记进 ADR-0008 ② 段的待排查项；遇到它时结果行是 reason=port-not-listening，-Doctor 的 L6 会标 FAIL。
# -Doctor：只做环境体检（不接 HBuilderX、不构建、不跑探针、不产证据块、**不产出门的通过结论**），
# 逐层给 OK/WARN/FAIL。它复用主流程的 close → open → auto → 就绪闸门，所以体检走的就是门真正的路径。
if ($Doctor) { $SkipBuild = $true }

# -BuildOnly 与 -Doctor 互斥（#1861）：-Doctor **不构建**（它自己把 $SkipBuild 置位），而 build-only 的全部意义
# 就是真构建 ⇒ 两者同给没有可解释的语义。按**用法错误** exit 2（不产结果行）：与「不像 uni-app-x 项目根」
# 那条早期 exit 2 同形 —— 门结论只能由构建面判据产出，用法错误不该冒充门未过或环境受阻。
if ($BuildOnly -and $Doctor) {
    Write-Host '[error] -BuildOnly 与 -Doctor 互斥：-Doctor 只体检环境（不构建），-BuildOnly 要真构建。二选一。' -ForegroundColor Red
    exit 2
}

# 临时根：**不能只读 $env:TEMP** —— Linux 上（CI 跑 -DryRun）只有 TMPDIR，$env:TEMP 为 $null，
# 而 Join-Path 拿到 $null 会直接抛（Set-StrictMode 下同理）⇒ 显式兜底到 .NET 的临时目录。
$tempRoot = if ($env:TEMP) { $env:TEMP } elseif ($env:TMPDIR) { $env:TMPDIR } else { [System.IO.Path]::GetTempPath() }

function Test-PeHeader {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    try {
        $fs = [System.IO.File]::OpenRead($Path)
        try { $buf = New-Object byte[] 2; $read = $fs.Read($buf, 0, 2) } finally { $fs.Dispose() }
    } catch { return $false }
    # 'MZ'：E:\HBuilderX 那份残留副本前 8 字节全为 00（文件损坏），必须排除
    return ($read -eq 2 -and $buf[0] -eq 0x4D -and $buf[1] -eq 0x5A)
}

function Resolve-HBuilderXRoot {
    param([string]$Explicit, [string]$ExplicitCli)
    if ($ExplicitCli) {
        if (-not (Test-PeHeader -Path $ExplicitCli)) { return $null }
        return (Split-Path (Resolve-Path -LiteralPath $ExplicitCli).Path -Parent)
    }
    if ($Explicit) {
        $probe = Join-Path $Explicit 'cli.exe'
        if (-not (Test-PeHeader -Path $probe)) { return $null }
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    try {
        $proc = Get-Process HBuilderX -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($proc) {
            $candidate = Split-Path $proc.Path -Parent
            if (Test-PeHeader -Path (Join-Path $candidate 'cli.exe')) { return $candidate }
        }
    } catch { }
    $candidates = @()
    if ($env:HBuilderX_HOME) { $candidates += $env:HBuilderX_HOME }
    foreach ($pattern in @(
            'D:\软件\HBuilderX*\HBuilderX', 'D:\HBuilderX*\HBuilderX',
            'C:\Program Files\HBuilderX*\HBuilderX', 'C:\Program Files\HBuilderX', "$env:LOCALAPPDATA\HBuilderX")) {
        $candidates += (Get-ChildItem -Path $pattern -Directory -ErrorAction SilentlyContinue |
            Sort-Object FullName -Descending | ForEach-Object { $_.FullName })
    }
    foreach ($c in $candidates) {
        if (Test-PeHeader -Path (Join-Path $c 'cli.exe')) { return (Resolve-Path -LiteralPath $c).Path }
    }
    return $null
}

function Resolve-DevToolsCli {
    param([string]$Explicit)
    if ($Explicit) {
        if (Test-Path -LiteralPath $Explicit -PathType Leaf) { return (Resolve-Path -LiteralPath $Explicit).Path }
        return $null
    }
    foreach ($p in $WxDevToolsPatterns) {
        if (Test-Path -LiteralPath $p -PathType Leaf) { return (Resolve-Path -LiteralPath $p).Path }
    }
    return $null
}

# 统一进程入口：超时兜底 + UTF-8 解码。**不返回退出码作为判据**（只在日志里留痕）。
#
# 可选的「按产物收流」（#2163）：只给 -SettleDist / -SettleSince 时启用，服务构建段。
# 为什么要它（实测 2026-10-31，新树 5 s 采样）：产物写满（643 文件，14:53:54）与 cli 退出
# （14:55:29）之间**连健康跑都有 ≈90 s 空洞**；票面 B 形态是同一个空洞失去上界（产物 170 s 齐、
# cli 12 分钟不退）。旧实现只认 `WaitForExit(预算)` 一个信号 ⇒ 空洞一无界就必然烧到预算，
# 以 `errors=env-blocked reason=timeout` 白烧整轮 ②。
# 次序刻意是「判据成立后**再**等 cli 自然退出 GraceSeconds」，不是判据一成立就返回：
#   健康跑（实测 ≈90 s）落在 grace 窗口内 ⇒ 仍由 cli 自己退出、仍拿得到「导出微信小程序成功」
#   文案 ⇒ 行为与改动前一致；只有 cli 赖着不退（B 形态）才改走产物依据。
# 红线（票面第 5 条 / 移动端 ADR-0008）：收流**不 kill 任何进程** —— 与 timeout 分支的
# `Kill($true)` 相反，这里留下活着的 cli 并把它的 pid 记进门日志。
function Invoke-Process {
    param(
        [string]$FilePath, [string[]]$Arguments, [int]$TimeoutSeconds, [string]$Tag, [string]$WorkingDirectory,
        [string]$SettleDist = '', [datetime]$SettleSince = (Get-Date),
        [int]$SettleQuietSeconds = 15, [int]$SettleGraceSeconds = 120, [int]$SettlePollSeconds = 2
    )
    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $FilePath
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.WorkingDirectory = if ($WorkingDirectory) { $WorkingDirectory } else { $script:workDir }
    $psi.StandardOutputEncoding = [System.Text.Encoding]::UTF8
    $psi.StandardErrorEncoding = [System.Text.Encoding]::UTF8
    foreach ($a in $Arguments) { [void]$psi.ArgumentList.Add($a) }
    Write-Host ">>> [$Tag] $FilePath $($Arguments -join ' ')"
    $p = [System.Diagnostics.Process]::Start($psi)
    $stdout = $p.StandardOutput.ReadToEndAsync()
    $stderr = $p.StandardError.ReadToEndAsync()

    # ---------- 有界等待 +（可选）按产物收流 ----------
    # 旧写法一次 `WaitForExit(整个预算)`：预算期内除「进程退出」没有别的信号，产物早写满也得干等。
    # 这里切成小片轮询，片间问一次收流判据；不给 SettleDist 时 sliceMs = 整个剩余预算，与旧行为同形。
    $settled = $false
    $settleInfo = $null
    $settledAt = $null
    $exited = $false
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ($true) {
        $remainMs = [int](($deadline - (Get-Date)).TotalMilliseconds)
        if ($remainMs -le 0) { break }
        $sliceMs = if ($SettleDist) { [Math]::Min($SettlePollSeconds * 1000, $remainMs) } else { $remainMs }
        if ($p.WaitForExit($sliceMs)) { $exited = $true; break }
        if (-not $SettleDist) { continue }
        $settleInfo = Get-MpWeixinPublishSettle -Dist $SettleDist -Since $SettleSince -QuietSeconds $SettleQuietSeconds
        if (-not $settleInfo.Settled) { $settledAt = $null; continue }
        if ($null -eq $settledAt) {
            $settledAt = Get-Date
            Write-Host "[settle] [$Tag] 产物已完整（$($settleInfo.Files) 文件 / $($settleInfo.Pages) 页），再等 cli 自然退出至多 $SettleGraceSeconds s" -ForegroundColor DarkGray
        }
        if (((Get-Date) - $settledAt).TotalSeconds -ge $SettleGraceSeconds) { $settled = $true; break }
    }

    if (-not $exited) {
        if ($settled) {
            # 不 kill：留下活着的 cli（其进程树里还有 HBuilderX 拉起的开发者工具），只放弃输出流。
            Write-Host "[warn] [$Tag] cli 未在 grace 窗口（$SettleGraceSeconds s）内退出（pid=$($p.Id)）⇒ 按产物收流，**未终止任何进程**；输出流放弃读取" -ForegroundColor Yellow
            return @{
                Output = ''; ExitCode = -1; TimedOut = $false; DrainAbandoned = $true
                Settled = $true; ChildPid = $p.Id; SettleReason = $settleInfo.Reason
                SettleDetail = $settleInfo.Detail; SettleFiles = $settleInfo.Files; SettlePages = $settleInfo.Pages
            }
        }
        try { $p.Kill($true) } catch { }
        # 到点被杀时把「为什么没收流」的实测量留在输出里：票面 A 形态要求日志能分清
        # 「没产物」与「有产物没退出」—— 旧实现两种都只打 reason=timeout，读不出差别。
        $why = 'settle=off'
        $s = @{ Reason = ''; Files = 0; NewestAgeS = -1 }
        if ($SettleDist) {
            $s = Get-MpWeixinPublishSettle -Dist $SettleDist -Since $SettleSince -QuietSeconds $SettleQuietSeconds
            $why = "settleReason=$($s.Reason) files=$($s.Files) newestAgeS=$($s.NewestAgeS) dist=$SettleDist"
        }
        return @{ Output = "[timeout] $Tag 超过 $TimeoutSeconds 秒未返回，已终止（$why）"; ExitCode = -1; TimedOut = $true; DrainAbandoned = $false; Settled = $false; ChildPid = 0; SettleReason = $s.Reason; SettleDetail = $why; SettleFiles = $s.Files; SettlePages = 0 }
    }
    # ---------- F4（2026-10-29）：主进程已退出 ≠ 输出流 EOF ----------
    # 病根（#2107 量尺实测，1050 s 纯挂死的一段）：`cli.bat close` 回显 `✔` 后 cmd.exe 正常退场，
    # 但**它拉起的开发者工具进程还活着，并继承了 stdout 句柄** ⇒ `ReadToEndAsync` 永远等不到 EOF，
    # 下方 `.Result` 会一直挂着。而超时判据**刚刚已经被 WaitForExit 走过了**（它返回 true），
    # 于是没有任何兜底：门其实早就出了结论，命令却不返回，现场读起来像「还在跑」。
    # 处置：主进程退出后给输出流一个**有界收尾窗口**（5 s，正常命令的数据早已写完）；
    # 收不到就明确放弃剩余输出并打 [warn]，绝不让一条挂住的读取抹掉「进程已退出」这个事实。
    # 判据因此只认**进程真退出**（WaitForExit 的返回值），不认 `✔` 之类的回显文案。
    $drainMs = 5000
    $drained = $false
    try { $drained = [System.Threading.Tasks.Task]::WaitAll(@($stdout, $stderr), $drainMs) } catch { $drained = $false }
    if (-not $drained) {
        Write-Host "[warn] [$Tag] 主进程已退出（exit=$($p.ExitCode)）但输出流仍被占用（子进程继承了 stdout 句柄）⇒ 放弃剩余输出，不影响退出码与门结论" -ForegroundColor Yellow
    }
    # 只在真的完成时取值：未完成的 Task 上取 .Result 就是上面那条挂死的复现方式
    $outText = ''; $errText = ''
    try { if ($stdout.IsCompleted) { $outText = $stdout.Result } } catch { $outText = '' }
    try { if ($stderr.IsCompleted) { $errText = $stderr.Result } } catch { $errText = '' }
    return @{ Output = ($outText + $errText); ExitCode = $p.ExitCode; TimedOut = $false; DrainAbandoned = (-not $drained); Settled = $false; ChildPid = 0; SettleReason = ''; SettleDetail = ''; SettleFiles = 0; SettlePages = 0 }
}

# 探针 JSON 的安全取值：Set-StrictMode 下访问不存在的属性会抛异常，
# 而「navigation 字段是否存在」正是降级判据，不能因为字段缺失把脚本炸成 env 失败。
function Get-Prop {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $p = $Object.PSObject.Properties[$Name]
    if ($p) { return $p.Value }
    # 字典（`[ordered]@{}` / `@{}`）的键**不是** PSObject 属性：#1817 实测 `Get-Prop $plan 'routes'`
    # 静默回 $null，计划里声明的游客页族被读成空清单（游客页照旧被注 auth 会话），
    # 而源码文本面的守护照样全绿 —— 读不到配置比读错配置更难发现。字典按 key 取。
    if ($Object -is [System.Collections.IDictionary] -and $Object.Contains($Name)) { return $Object[$Name] }
    return $null
}

# ---------- 门计划（GATE_PLAN）：步骤序列 / 每步参数与判据 / 预算与 exit 2 归属 / 结果行字段 ----------
# 为什么有这一块（#914，勿删）：守护以前断言的是**调用点源码原文**（`-Arguments @('open', '--project', $dist)`
# 之类）。后果是任何把这些原文抽进共享库的重构都会让守护失配，而唯一的「修法」是放宽守护 —— 等于把防护
# 拆掉。收束办法：把「门要做什么」收成**一份数据**，执行路径按 id 取用（Get-GateStep），`-DryRun` 把同一
# 份数据打成单行 JSON 交给守护断言 ⇒ 顺序/参数类不变量变成「行为的直接观测」，搬运不再碰守护。
# 判据（守护 C18）：每个步骤都必须被执行路径按 id 取用一次，且调用点原文**至多**在脚本里出现一次。
function New-GatePlan {
    param(
        [string]$Project,
        [string]$Dist,
        [string]$ProbePath,
        [string]$ReadyPath,
        [string]$AuthPath,
        [string]$AutoRoot,
        [string]$LogDir,
        [string]$LogRelative,
        [string[]]$RouteList,
        [int]$Port,
        [int]$OpenTimeoutSeconds,
        [int]$AutoTimeoutSeconds,
        [int]$AutoAttempts,
        [int]$AutoRetryDelaySeconds,
        [int]$PortWaitSeconds,
        [int]$ReadyWaitSeconds,
        [int]$ProbeTimeoutMs,
        [int]$ProbeTimeoutSeconds,
        [int]$ProbeAttempts,
        [int]$ProbeRetryDelaySeconds,
        [int]$PublishTimeoutSeconds,
        [bool]$SkipBuild,
        [bool]$Doctor,
        [bool]$BuildOnly,
        [int]$PostToPr,
        [string]$PagesMode,
        [int]$PageScrollTop = 0,
        [string]$Shots = 'full'
    )
    $ws = "ws://127.0.0.1:$Port"
    $entryUrl = if (@($RouteList).Count -gt 0) { $RouteList[0] } else { '' }
    $steps = @()

    # -1) 环境预探（#1860）：publish 之前的**秒级否定性预探** —— 把「环境被楔死」的暴露点从
    #     publish+open 之后（9–26 min/轮）提到秒级。判据库 scripts/lib/mp-weixin-preprobe.ps1
    #     （官方检测法：用户目录端口文件 + 尝试连接）。⚠️ 只做**否定性闸门**：只拦「.ide 端口文件
    #     在但其服务端口试连不上 **且 IDE 进程数 ≥ 1**」的楔死态；其余一律放行且不冒充「会话已就绪」
    #     （就绪判据归就绪闸门）。#2147：进程数 = 0 时只剩陈旧文件（stale-port-file），按冷起点放行。
    #     本步**只读**（不操控 IDE、不发命令），故排在 quit 之前（quit 是第一个会话步骤）；
    #     -Doctor 跳过（体检自带逐层报告）。试连预算写死在 timeouts.preprobeMs（计划是预算唯一真源）。
    $steps += [ordered]@{
        id = 'env-preprobe'
        tag = 'env-preprobe'
        exec = 'none'
        argv = @()
        criteria = @(
            'publish 之前的秒级否定性预探（#1860）：读 .ide 端口文件 + TCP 试连，楔死态提前到 publish 之前判 exit 2（不烧构建与重试预算）',
            '楔死态签名（最窄，勿放宽）= 端口文件存在 + 端口可解析 + 试连失败 ⇒ exit 2 reason=ide-port-unreachable，处置提示指向人工重启开发者工具（门绝不 kill IDE，ADR-0008 红线）',
            '端口文件不存在（IDE 未运行，冷起点）或试连可达 ⇒ 放行 —— 不做肯定性放行、不冒充「会话已就绪」（预探判错会把可跑环境误报成 env-blocked，调研 §1.2 条 1 风险条款）',
            '端口文件内容不可解析 ⇒ 放行（宁可漏拦交给就绪闸门，不误杀可跑环境）',
            '端口文件在 + 试连失败 + IDE 进程数 = 0 ⇒ stale-port-file（只剩陈旧文件）按冷起点放行（#2147）：0 个进程时无从 kill，不违反 ADR-0008 红线，也不是把否定性闸门放宽成肯定性放行',
            '-Doctor 跳过本步（体检自带逐层报告）；行为守护 utils/mpWeixinGatePreprobeBehavior.test.js 真执行判据库'
        )
        exit2 = @([ordered]@{ on = 'unreachable'; reason = 'ide-port-unreachable' })
    }

    # 0) quit：跑门前**清场** —— 退出整个开发者工具 IDE（#1634）。病根：用脚本（IPC 链）启动时，残留/堆叠的
    #    未初始化 IDE 实例会让后面的 open 把项目窗口叠进白屏（连菜单/Console 都没有；双击 exe 启动不复现）。
    #    close 只关项目窗口、对未初始化实例无能为力 ⇒ 必须 quit（等价于 ADR-0008 的 pre-flight 处置：
    #    跑前先 `cli.bat quit` 清场，实测有效）。门本就独占 IDE，quit 关整个工具没有额外代价。
    #    ⚠️ 红线（ADR-0008）：运行中**不许 kill IDE** —— 收拾残留只能走 quit，不得用 Stop-Process 替代。
    #    进程归零的轮询预算（waitSeconds）与本步超时同源，都写死在这里（计划是预算的唯一真源）。
    #    #1860 收紧：quit **之前先清点 IDE 进程** —— 0 个 ⇒ 不执行 quit（quit 在 IDE 未运行时会自启
    #    无项目实例占住服务端口，官方 --port「没启动则会启动」的同族机制；2026-10-23 实测占 43005）。
    $steps += [ordered]@{
        id = 'devtools-quit'
        tag = 'devtools-quit'
        exec = 'devToolsCli'
        argv = @('quit')
        timeoutSeconds = 60
        waitSeconds = 60
        awaitCompletion = $true
        logged = $true
        criteria = @(
            '跑门前先 cli.bat quit 退出整个 IDE：脚本启动（IPC 链）下残留/堆叠的未初始化实例会让 open 把项目窗口叠进白屏（#1634）',
            '#1860：quit 之前先清点 IDE 进程（wechatdevtools/微信开发者工具 双名匹配）—— 0 个 ⇒ 不执行 quit（防 quit 自启无项目实例占服务端口）',
            'quit 之后轮询 wechatdevtools 进程归零（最多 60 秒）；归零不了只 [warn] 继续 —— IDE 收不掉时 quit 重试无益，白屏风险记日志、由后续 open/auto/就绪闸门的判据兜',
            '本步失败只警告，不判 exit 2（不得用 Stop-Process 替代 quit：运行中 kill IDE 是 ADR-0008 红线）',
            '-Doctor 跳过本步（体检不操控 IDE）'
        )
        exit2 = @()
    }

    # 1) close：清残留自动化会话（不 close 会撞 pageStack 空，坑位 1）。失败只警告，不判 exit 2。
    $steps += [ordered]@{
        id = 'devtools-close'
        tag = 'devtools-close'
        exec = 'devToolsCli'
        argv = @('close', '--project', $Dist)
        timeoutSeconds = 120
        awaitCompletion = $true
        criteria = @('清残留自动化会话与自动化端口（不 close 会撞 pageStack 空）', '失败只警告，本步不判 exit 2')
        exit2 = @()
    }

    # 2) open：close 会把项目窗口一起关掉，而 auto 不会重开（时序坑位 4）⇒ 必须是显式步骤且写进日志。
    $steps += [ordered]@{
        id = 'devtools-open'
        tag = 'devtools-open'
        exec = 'devToolsCli'
        argv = @('open', '--project', $Dist)
        timeoutSeconds = $OpenTimeoutSeconds
        awaitCompletion = $true
        logged = $true
        criteria = @('close 之后、auto 之前必须重开项目窗口（缺这步 pageStack 恒空）', '项目目录须是构建产物目录（-SkipBuild 下同样定位得到）', '仅超时判 exit 2（窗口没重开就没法继续）')
        exit2 = @([ordered]@{ on = 'timeout'; reason = 'devtools-open-timeout' })
    }

    # 3) auto：必须跑完 —— 中途 kill 会让端口永不监听；端口是延迟出现的，故后面还要轮询。
    #    2026-09-23（坑位 6）：冷起点/IDE 重启窗口下第一次 auto 会「回显 √ 但端口不起」或「直接挂住」⇒
    #    执行路径把 `close → open → auto → 端口` 当**一个可重试的单元**跑 attempts 次（不是只重试 auto）。
    $steps += [ordered]@{
        id = 'devtools-auto'
        tag = 'devtools-auto'
        exec = 'devToolsCli'
        argv = @('auto', '--project', $Dist, '--auto-port', "$Port", '--trust-project')
        timeoutSeconds = $AutoTimeoutSeconds
        attempts = $AutoAttempts
        retryDelaySeconds = $AutoRetryDelaySeconds
        awaitCompletion = $true
        criteria = @('auto 必须等它跑完（中途 kill ⇒ 端口永不监听）', "自动化端口由 --auto-port $Port 指定（监听面由开发者工具决定，实测绑通配地址）", '仅超时判 exit 2', "端口没起来时**整段**重试（close → open → auto，最多 $AutoAttempts 次；**不得**只重试 auto —— 实测只重试 auto 会绑到一个没加载项目的新 IDE 实例上：Tool.getInfo 缺 SDKVersion、pageStack 永不应答，见脚本头坑位 6）")
        exit2 = @([ordered]@{ on = 'timeout'; reason = 'auto-timeout' })
    }

    # 4) 端口就绪：auto 返回 ≠ 端口已监听（实测延迟出现）。每次整段尝试各有一份本预算。
    $steps += [ordered]@{
        id = 'port-listening'
        tag = 'port-listening'
        exec = 'none'
        argv = @()
        waitSeconds = $PortWaitSeconds
        attempts = $AutoAttempts
        criteria = @("端口 $Port 须在 $PortWaitSeconds 秒内进入 listening（auto 返回后端口是延迟出现的）", '未监听 ⇒ 先重试整段（最多 attempts 次），全部用尽才 exit 2（环境不可用，不是门未过）', "两次尝试之间等 $AutoRetryDelaySeconds 秒（实测冷起点第 1 次 180s 不起、等 IDE 稳定后重跑 1–8s 即起）")
        exit2 = @([ordered]@{ on = 'not-listening'; reason = 'port-not-listening' })
    }

    # 5) 就绪闸门：端口 listening ≠ 会话可用（实测 App.getPageStack 要 24–34s 才应答，坑位 5）。
    $steps += [ordered]@{
        id = 'ready-gate'
        tag = 'ready-gate'
        exec = 'node'
        argv = @($ReadyPath, '--ws', $ws, '--wait-seconds', "$ReadyWaitSeconds", '--require-stack', '--require-root', $AutoRoot)
        timeoutSeconds = $ReadyWaitSeconds + 60
        awaitCompletion = $true
        criteria = @('Tool.getInfo 须带 SDKVersion', 'App.getPageStack 须开始应答（只等 SDKVersion 不够）', '须解析出 MP_WEIXIN_READY 结果行')
        exit2 = @(
            [ordered]@{ on = 'timeout'; reason = 'no-ready-result' },
            [ordered]@{ on = 'not-ready'; reason = 'automation-not-ready' }
        )
    }

    # 5.5) 会话注入（#1690）：直达探针冷启动**登录态页**时 DevTools 里没有会话 ⇒ 首屏 401 ⇒
    #      handleUnauthorized reLaunch 到登录页 ⇒ reached=False。本步在门运行环境里走真实登录链路
    #      （wx.login → POST /auth/wx-login → setStorageSync 四键），**不改业务代码**。
    #      失败**不判环境受阻**：status=none(reason) 记录后照跑直达循环，登录态页如实 miss ——
    #      「注入不可用」不得升级成「环境坏了」，也不得冒充取证成立。
    $steps += [ordered]@{
        id = 'auth-inject'
        tag = 'auth-inject'
        exec = 'node'
        argv = @($AuthPath, '--ws', $ws, '--require-root', $AutoRoot, '--timeout-ms', '150000')
        timeoutSeconds = 300
        awaitCompletion = $true
        criteria = @('判成败只看输出 MP_WEIXIN_AUTH 结果行（退出码不作判据）', 'status ∈ existing|injected|cleared|none：auth 相 none ⇒ 该页立即记 SKIP(reason=auth-session-unavailable) 且不换页不冷启动不探针（#1863：路由判据不得独自放行，#1845 实证路由可通过而内容是 401 态）；guest 相 none 只告警不改判（游客页无会话才是合法落地态，落地判据兜）', 'auth 相失败 detail（reason + HTTP 状态/网络错误/超时）透传进结果行 auth= 字段（#1863：wx-login-api-failed 必须可归因）', '结果行不得含令牌：只带 user 标识与耗时', '幂等：已有会话 ⇒ existing 即返回；guest 相写盖帽 ⇒ cleared', '落盘确认（#1690 run13）：写/盖帽后必须等标记出现在 WeappStorage 盘上才放行冷启动——IDE 异步刷盘且 quit 不冲刷')
        exit2 = @()
    }

    # 6) 探针：判成败只看输出 MP_WEIXIN_PROBE（cli / 探针退出码一律不作判据）。
    $steps += [ordered]@{
        id = 'automator-probe'
        tag = 'automator-probe'
        exec = 'node'
        argv = @($ProbePath, '--ws', $ws, '--routes', ($RouteList -join ','), '--entry-url', $entryUrl, '--shot-dir', $LogDir, '--require-root', $AutoRoot, '--timeout-ms', "$ProbeTimeoutMs")
        timeoutSeconds = $ProbeTimeoutSeconds
        attempts = $ProbeAttempts
        retryDelaySeconds = $ProbeRetryDelaySeconds
        awaitCompletion = $true
        criteria = @('判成败只看输出 MP_WEIXIN_PROBE（HBuilderX CLI / cli.bat / 探针的退出码都不作判据）', 'pageStack 非空是前置断言，为空判环境不可用', '`page.waitFor(选择器)` 与导航 API 仍禁（#1917 实测挂起 8s / 恒败）、`page.$`+`element.text()` 锚点断言放行（#1919）')
        exit2 = @([ordered]@{ on = 'timeout'; reason = 'probe-timeout' })
    }

    # -BuildOnly（#1861）：**会话段步骤不进计划** —— 计划是「这轮真会跑什么」的唯一真源，列着不跑的步骤
    # 就是两份真相。保留的只有构建面仍要用的两步：env-preprobe（publish 之前的秒级否定性预探）与
    # devtools-close（finally 里收掉 publish 自启的开发者工具实例 —— 这条正是「不建薄壳」的主要理由之一）。
    # 会话段步骤（devtools-quit / devtools-open / auto / 就绪闸门 / probe）整段不执行，其 exit 2 归属
    # 随 $exit2Summary 的摊平自动消失（归属只声明给计划里真有的步骤）。
    if ($BuildOnly) {
        $steps = @($steps | Where-Object { @('env-preprobe', 'devtools-close') -contains "$($_['id'])" })
    }

    # exit 2 归属的**汇总视图**：把每步的 exit2 摊平 + 标出是哪一步决定的（守护断言两者一致）。
    $exit2Summary = @()
    foreach ($st in $steps) {
        foreach ($e in @($st['exit2'])) {
            $exit2Summary += [ordered]@{ step = $st['id']; on = $e['on']; reason = $e['reason'] }
        }
    }

    # 结果行字段清单：执行路径**按这份清单**拼结果行（Format-GateMetricLine），所以它与实际输出同一真源。
    # -BuildOnly 是**另一套字段**（#1861）：会话段那些读数（pageStack / entry / shots / sdk / ide …）本轮
    # 根本不存在，列着就是逼执行路径填假值；build-only 只声明它真能读到的六项。
    $resultFields = @()
    if ($BuildOnly) {
        # build = publish | skipped(fingerprint) | skipped(explicit)：**不把「没构建」写成「构建过」**。
        # elapsed = 本轮 publish 段墙钟（秒）；跳过构建时写 n/a。appid = 产物实读值（本模式不断言，只如实上报）。
        foreach ($n in @('appid', 'pages', 'product', 'build', 'elapsed', 'log')) {
            $resultFields += [ordered]@{ name = $n }
        }
    } else {
        foreach ($n in @('appid', 'pageStack', 'entry', 'errorsTotal', 'exceptionsTotal', 'logsTotal', 'shots', 'navigation', 'ready', 'sdk', 'ide')) {
            $resultFields += [ordered]@{ name = $n }
        }
        # 覆盖判据（#2101）：`coverage=changed:N|smoke:N|none`，**只在 `-Pages auto` 下产出**。
        # 为什么是 optional 而不是必填：`-Pages <csv>`（定向单验）与 `-Pages none`（旧导航组）都是合法跑法，
        # 它们没有「改动页覆盖」这个语义 —— 硬塞一个值等于让读数说谎。缺席 ≠ 降级，读的人按模式读。
        $resultFields += [ordered]@{ name = 'coverage'; optional = $true }
        $resultFields += [ordered]@{ name = 'navigationSkipReason'; optional = $true }
        # 会话注入（#1690）读数：auth=existing|injected|none(reason[:detail-slug])——detail-slug 是
        # HTTP 状态/网络错误/超时的归因类别（#1863）；只在直达循环启用时出现（可选字段）
        $resultFields += [ordered]@{ name = 'auth'; optional = $true }
        # 锚点断言执行数（#1919）：只在有锚点执行时出现——无锚点跑次 Format-GateMetricLine 跳过它，结果行逐字节不变
        $resultFields += [ordered]@{ name = 'asserts'; optional = $true }
        # 平台噪音读数（2026-10-29 F3）：被白名单放行的 console error 条数。**只在真的过滤过东西时出现**
        # （noise>0），没有噪音的跑次结果行逐字节不变——零回归；有噪音时它一定出现在行上，过滤不可静默。
        $resultFields += [ordered]@{ name = 'noise'; optional = $true }
        $resultFields += [ordered]@{ name = 'log' }
    }

    return [ordered]@{
        schema = 'gate-plan/1'
        gate = '②'
        mode = [ordered]@{ dryRun = $true; skipBuild = $SkipBuild; doctor = $Doctor; buildOnly = $BuildOnly; postToPr = $PostToPr }
        paths = [ordered]@{
            project = $Project
            dist = $Dist
            distRelative = $DistRelative
            probe = $ProbePath
            probeRelative = $ProbeRelative
            ready = $ReadyPath
            readyRelative = $ReadyRelative
            auth = $AuthPath
            authRelative = $AuthRelative
            automatorRoot = $AutoRoot
            logDir = $LogDir
            logRelative = $LogRelative
        }
        endpoint = [ordered]@{ port = $Port; ws = $ws }
        routes = @($RouteList)
        # 任意页直达探针（#1682 · ADR-0039 拆分步骤 2 · ADR-0038 决策 1）：取证面从入口页扩到被改页逐页。
        # 机制唯一：把**构建产物** app.json 的 pages 首项临时换成目标页，plain 冷启动 ⇒ 落地页=目标页，
        # 探针以 --expect-page 模式**不调用任何导航 API**（reLaunch/navigateTo 在本机恒报 Uncaught [object Object]，
        # 那是「导航组」的老结论，不随本机制改变）。机制不成立 ⇒ 显式记 SKIP（诚实降级，不破坏门、不假绿）；
        # 每直达一页的 console error/exception 断言照旧生效（errorsTotal 语义不变）。
        directPages = [ordered]@{
            param = 'Pages'
            modes = @('auto', 'none', '<csv>')
            mode = $PagesMode
            autoSource = 'Get-MpWeixinChangedUvuePages（scripts/lib/mp-weixin-pages.ps1，#1853）：git -c core.quotepath=false + UTF-8 显式解码 diff --name-only origin/master...HEAD 命中 pages/**/*.uvue（本 PR 改动页）'
            mechanism = 'cold-start-direct'
            swapTarget = 'app.json'
            swapNote = '只换构建产物 app.json 的 pages 首项；不动源头 pages.json（运行时面判据来源，ADR-0038 决策 2）'
            restore = 'finally'
            exec = 'node'
            probeArgv = @($ProbePath, '--ws', $ws, '--expect-page', '<page>', '--shot-dir', $LogDir, '--require-root', $AutoRoot, '--timeout-ms', "$ProbeTimeoutMs", '--scroll-top', "$PageScrollTop", '--shots', "$Shots")
            scrollTop = $PageScrollTop
            shots = $Shots
            # 逐页冷启动的整段重试次数**固定 2**，与 `-AutoAttempts`（auto 段，默认 3）**不是同一个值**：
            # #2101 票面原写「逐页冷启动共用此值」，落库时按实测语义拆开 —— auto 段 3 次是冷起点/IDE 重启的
            # 排障余量（单发必红的那个坑），逐页失败只落该页 skip（`landed-elsewhere` / `auth-session-unavailable`）
            # 不牵连同组其它页，2 次已够；且逐页一旦挂住，代价是每页多等一整段，3 次的账在 ② 的 300 s 里付不起。
            attempts = 2
            retryDelaySeconds = $ProbeRetryDelaySeconds
            # 覆盖判据（#2101 落库，2026-10-29）：`-Pages auto` 的改动页清单**为空**时跑这组固定冒烟集。
            # 为什么要有：`-Pages auto` 只认 `pages/**/*.uvue`，改 `components/` / `utils/*.uts` / `App.uvue` /
            # `pages.json` 时清单为空 ⇒ 只跑入口页一张 = **覆盖为零的绿**（#2099 盘点出的洞）。
            # 冒烟页清单的唯一真源 = 这里（执行路径按 `$script:GatePlan` 取，不写第二份）。
            smokePages = @('pages/dashboard/dashboard', 'pages/profile/profile')
            # 结果行 `coverage=` 的三值形态（#2101/#2102）：只说明「这次绿覆盖了什么」，**不判取舍**。
            # 缺省值 `none` 不是「没覆盖」，而是「覆盖判据本身不可用（git diff 取不到）」——见执行侧分支。
            coverageShapes = [ordered]@{
                changed = 'changed:<N>'
                smoke = 'smoke:<N>'
                none = 'none'
            }
            skipReasons = @('direct-page-unsupported', 'page-not-in-build', 'app-json-invalid', 'auth-session-unavailable', 'landed-elsewhere')
            # 会话分相（#1817）：相判据是「该页在无会话时是否为合法落地页」，**不是**「是否入口页」。
            # 游客页族 = 已登录会自己 reLaunch 走的页（index.uvue:81 / login.uvue:242 / register.uvue:168 /
            # recruiter/login.uvue:101）——无会话时它们才是合法落地页；给它们注 auth 会话，冷启动必然落到
            # dashboard 之类的别处，截图就成了「别的页」（2026-10-23 实测：login 注 auth ⇒ 落地 dashboard ⇒
            # 截 miss-login.png 撞 automator 应答超时）。入口页由 $isEntry 覆盖，不在清单里重复。
            guestPhase = [ordered]@{
                param = '--auth'
                mode = 'guest'
                routes = @('pages/login/login', 'pages/register/register', 'pages/recruiter/login')
                rationale = '游客页族 = 已登录会自己 reLaunch 走的页面；这些页无会话时才是合法落地页，故走 guest 相（清会话），其余登录态页走 auth 相（注入）'
            }
            # 组级 navigation 形态（#1816）：部分达成必须如实出 partial，不得塌缩成整组 skip ——
            # 否则一页 miss 会吞掉同组其它页的成功读数（2026-10-23 实测：index 达成 + login 截图超时 ⇒ 组级只剩 skip）。
            navigationShapes = [ordered]@{
                ok = 'ok'
                partial = 'partial(ok=<N>,skip=<M>)'
                skip = 'skip(reason=<slug>)'
                envEarly = 'n/a'
            }
            # 锚点元素断言（#1919 · #1917 裁决承接）：逐页可选 {selector, text}，探针以
            # --assert-selector/--assert-text 按序配对执行（page.$ → element.text() 精确比对，8s 步超时）。
            # 首期只声明 login 页（#1917 裁决实验单验过的锚点）；uni-app x 不能跨组件选择 ⇒ 锚点必须页面级可达，
            # **新锚点逐页单验后才可加**。不在表里的页 argv 不变（诚实降级面，不新增失败面）。
            anchors = [ordered]@{
                'pages/login/login' = [ordered]@{ selector = '.app-title'; text = '叉车维修培训系统' }
            }
            # 带参页口径（#2110 单验定案，2026-10-29）：「devtools 编译条件」的两条入口都在本机单验判死——
            #   ① CLI `open --compile-condition` 被 yargs 非 strict **静默吞参**：exit 0、无任何报错，CLI 源里
            #      `open` 对 IDE HTTP 服务只转发 projectpath（`--compile-condition` 只挂在 preview / auto-preview 上）；
            #      实测冷启动落地 pages/index/index，锚点 skip（读数 .ci-verify/2110/testA-*）。
            #   ② IDE 编译条件持久层（产物 project.private.config.json 的 condition.miniprogram 段，pathName+query）
            #      对 `open→auto` 自动化冷启动同样**不生效**：落地页不变（读数 .ci-verify/2110/testB-*）。
            # ⇒ 带参页（featured-detail / ai-feature / job-detail / resume-detail 等依赖入口 query 的页）在
            #    「首项换页 + plain 冷启动」机制下**直达内容形态结构不可得**（与 ADR-0038 补注同族结论）：
            #    ② 对这些页只能记 SKIP（reason=direct-page-unsupported / landed-elsewhere），**不得用空壳 /
            #    「加载中…」截图充当该页证据**；内容取证落 ①（真机 --pagePath + --pageQuery，PR #1004 手法）
            #    或发版冒烟（移动端 ADR 0043 决策 3 第二选项，#2110 选定路线 2）。
            paramEntry = [ordered]@{
                verdict = 'route-2-accepted（#2110，2026-10-29）'
                spikeEvidence = '.ci-verify/2110（spike-plan.md / testA-* / testB-* / miss-featured-detail.png）'
                anchorRule = '带参页不得声明指向参数缺失态文本（如「加载中…」）的锚点；内容文本锚点须先经带参单验（① 链路）才可加'
            }
            criteria = @(
                '直达 = 首项临时换页 + plain 冷启动（ADR-0038 决策 1 已证的唯一可靠机制）；探针不得调用 reLaunch/navigateTo',
                '页身份判据 = 冷启动落地页 pageStack[0] 等于目标路由（与 #1678 真机侧「以进入页面日志为准」同源）',
                '换前 ConvertFrom-Json 校验、换后读回复验首项；用毕从备份还原 app.json 并复验（finally 路径）',
                '机制不成立 ⇒ navigation=skip(reason=…) 显式记录：不冒充门失败、不冒充取证成立；console error 照常进 failures',
                '部分达成（有页达成、有页未达成）⇒ navigation=partial(ok=<N>,skip=<M>)：已达成页读数照常计入门结论，未达成页逐页记入 navigation.pages，不得塌缩成整组 skip（#1816）',
                '逐页读数（reached/errors/exceptions/logs/shot/elapsed）写进日志与 sha 绑定评论，供 0039 窗口主门成本校准',
                '会话分相判据 = 入口页 ∪ 游客页族（guestPhase.routes），不得用「是否入口页」单相判据：游客页被注 auth 会话会自己跳走，落地页 ≠ 目标页（#1817）',
                '未达成页若落地到别处，页级 reason 记 landed-elsewhere(<落地页>)，不得一律记 direct-page-unsupported（#1817）',
                '带参页 ② = SKIP 定案（#2110，2026-10-29）：编译条件两入口（CLI --compile-condition 被静默吞参 / 私有配置 condition 段不生效）都带不了 query ⇒ 依赖入口参数的页直达内容形态结构不可得，不得用空壳或「加载中…」截图充当证据；内容取证落 ①（真机 --pagePath + --pageQuery）或发版冒烟',
                'auth 相页在注入 ok=false 时记 skip(reason=auth-session-unavailable) 且不换页不冷启动（#1863）：路由判据不得独自放行（#1845 实证——注入失败后旧过期 token 复活、路由可通过而内容是业务层 401 态）',
                '锚点断言只走 page.$+element.text()（8s 步超时、精确比对），失败进 failures ⇒ 门红可归因（不得以「环境不可用：」开头，避免被直达循环整组转 exit 2）',
                '不在 anchors 表的页行为与此前完全一致（argv 不变、不新增失败面）'
            )
        }
        steps = $steps
        timeouts = [ordered]@{
            publish = $PublishTimeoutSeconds
            # 构建段「按产物收流」两窗（#2163）：quiet = 落盘静默窗口，grace = 判据成立后再等 cli
            # 自然退出的窗口（健康跑的空洞实测 ≈90 s，落在 grace 内 ⇒ 仍走文案判据，行为不变）。
            publishSettleQuiet = $PublishSettleQuietSeconds
            publishSettleGrace = $PublishSettleGraceSeconds
            open = $OpenTimeoutSeconds
            auto = $AutoTimeoutSeconds
            autoAttempts = $AutoAttempts
            autoRetryDelay = $AutoRetryDelaySeconds
            portWait = $PortWaitSeconds
            readyWait = $ReadyWaitSeconds
            readyGate = $ReadyWaitSeconds + 60
            probe = $ProbeTimeoutSeconds
            probeTimeoutMs = $ProbeTimeoutMs
            probeAttempts = $ProbeAttempts
            probeRetryDelay = $ProbeRetryDelaySeconds
            # 预探（#1860）单次 TCP 试连预算（毫秒）：秒级预探的「秒」就是它
            preprobeMs = 2000
        }
        exit2 = $exit2Summary
        # 结果行前缀**唯一声明点**（#1861）：-BuildOnly 换成独立 token ⇒ 与 ② 的 MP_WEIXIN_RESULT 可判读分流
        # （同 TEST_UNIT_LOCAL_RESULT 先例）。执行路径一律经 Get-GateResultPrefix 取值，不硬写。
        resultLine = [ordered]@{
            prefix = $(if ($BuildOnly) { 'MP_WEIXIN_BUILDONLY_RESULT' } else { 'MP_WEIXIN_RESULT' })
            fields = $resultFields
        }
        # 结果行三分声明（#1655 / ADR-0036 处置二）：本计划的三种结果行形状与各自出口码。
        # 通过分支的判别 = errors= token 的**缺席**：'<none>' 是占位声明，产物不写这个 token，
        # 「errors=0」语义由 metric 行的 errorsTotal=0 承载（既有产物形状，零回归）。
        # classifier = 分类器真源（scripts/lib/mp-weixin-env.ps1 的函数名，守护对它做形状校验）；
        # envReasons 只能引用 exit2 已声明的 reason + 分类器派生的探针侧枚举（#1655 实测清单）。
        branches = [ordered]@{
            pass = [ordered]@{ errors = '<none>'; errorsTotal = '0'; exitCode = 0 }
            gateFailed = [ordered]@{ errors = '<N>'; failures = $true; exitCode = 1 }
            envBlocked = [ordered]@{
                errors = 'env-blocked'
                reason = $true
                failures = $true
                exitCode = 2
                classifier = 'Get-MpWeixinEnvClassification'
                envReasons = @(
                    'port-not-listening', 'hbuilderx-not-found', 'cli-unusable', 'devtools-not-found',
                    'node-not-found', 'cli-ipc-blocked', 'timeout', 'devtools-open-timeout', 'auto-timeout',
                    'no-dist', 'npm-not-found', 'automator-install-failed', 'no-ready-gate',
                    'no-ready-result', 'automation-not-ready', 'probe-timeout', 'no-probe-result',
                    'no-probe', 'no-routes',
                    'automation-unreachable', 'automation-timeout', 'automation-unknown',
                    'automator-missing', 'page-stack-empty', 'probe-env', 'exception', 'port-held-after-quit',
                    'ide-port-unreachable'
                )
            }
        }
    }
}

# 按 id 取用计划步骤：**执行路径唯一**的参数来源（取不到即抛，不给「脱节」留活口）。
function Get-GateStep {
    param($Plan, [string]$Id)
    foreach ($s in @($Plan['steps'])) {
        if ("$($s['id'])" -eq $Id) { return $s }
    }
    throw "门计划里没有步骤 '$Id'（Get-GateStep）—— 计划与执行脱节了"
}

# 结果行：按计划声明的字段清单拼（必填字段缺值即抛；可选字段无值就跳过）⇒ 清单不会与实际输出分叉。
function Format-GateMetricLine {
    param($Plan, [System.Collections.IDictionary]$Values)
    $parts = @()
    foreach ($f in @($Plan['resultLine']['fields'])) {
        $name = "$($f['name'])"
        $optional = if ($f.Contains('optional')) { [bool]$f['optional'] } else { $false }
        if (-not $Values.Contains($name)) {
            if ($optional) { continue }
            throw "结果行字段 $name 在 Values 里没有取值（计划与执行脱节）"
        }
        $parts += "$name=$($Values[$name])"
    }
    return ($parts -join ' ')
}

# -DryRun 的唯一输出：**单行** `GATE_PLAN {json}`。强制 UTF-8 输出，免得中文判据在重定向下变乱码。
function Write-GatePlanJson {
    param($Plan)
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $json = $Plan | ConvertTo-Json -Depth 12 -Compress
    Write-Output ('GATE_PLAN ' + $json)
}

# ---------- 项目与日志 ----------
if (-not $Project) { $Project = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path }
$Project = (Resolve-Path -LiteralPath $Project).Path
$script:workDir = $Project

# 以下都是**纯取值**（不建目录、不跑进程、不接 HBuilderX），先算齐 —— 门计划要用，而
# `-DryRun` 必须在任何副作用之前退出（守护会带着一个空临时项目目录跑 -DryRun，并断言没写工作树）。
$dist = Join-Path $Project $DistRelative
$logDir = Join-Path $Project '.ci-verify'
$logPath = Join-Path $logDir 'mp-weixin.log'
$logRelative = '.ci-verify/mp-weixin.log'
$probePath = Join-Path $Project $ProbeRelative
$readyPath = Join-Path $Project $ReadyRelative
$authPath = Join-Path $Project $AuthRelative
$autoRoot = Join-Path $tempRoot 'mp-weixin-automator'
$routeList = @($Routes -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
# 入口页 = 第一个路由；它进了门计划（探针的 --entry-url），这里不再另存一份，免得两处取值分叉

# 门计划的**唯一构造点**收成函数：端口可能因「残留自动化会话」被换掉（见下面 `Get-FreeAutoPort` 段），
# 换端口必须**重建计划** —— 计划是端点/argv 的唯一真源，不重建会让 `--auto-port`、就绪闸门 `--ws`、
# 探针端点与真实端口分叉。收成函数是为了让 `New-GatePlan` 只有一个调用点（计划与执行不分叉）。
function New-ThisGatePlan {
    param([int]$UsePort)
    return New-GatePlan -Project $Project -Dist $dist -ProbePath $probePath -ReadyPath $readyPath -AuthPath $authPath `
        -AutoRoot $autoRoot -LogDir $logDir -LogRelative $logRelative -RouteList $routeList -Port $UsePort `
        -OpenTimeoutSeconds $OpenTimeoutSeconds -AutoTimeoutSeconds $AutoTimeoutSeconds `
        -AutoAttempts $AutoAttempts -AutoRetryDelaySeconds $AutoRetryDelaySeconds `
        -PortWaitSeconds $PortWaitSeconds -ReadyWaitSeconds $ReadyWaitSeconds -ProbeTimeoutMs $ProbeTimeoutMs `
        -ProbeTimeoutSeconds $ProbeTimeoutSeconds -ProbeAttempts $ProbeAttempts `
        -ProbeRetryDelaySeconds $ProbeRetryDelaySeconds -PublishTimeoutSeconds $PublishTimeoutSeconds `
        -SkipBuild ([bool]$SkipBuild) -Doctor ([bool]$Doctor) -BuildOnly ([bool]$BuildOnly) -PostToPr $PostToPr `
        -PagesMode $Pages -PageScrollTop $PageScrollTop -Shots $Shots
}
$script:GatePlan = New-ThisGatePlan -UsePort $Port

# -DryRun：打印门计划即退出。**这里之前不许有任何外部命令/写工作树/接 HBuilderX**（守护会断言这一点）。
if ($DryRun) {
    Write-GatePlanJson -Plan $script:GatePlan
    exit 0
}

# ---------- 残留自动化会话：**换端口**（2026-09-23 实测，ADR-0008 坑位 6 续）----------
# 为什么不能只 warning：残留的自动化服务会让 `cli.bat auto` **静默复用它** —— 端口在监听、ws 也接得上，
# 但 `Tool.getInfo` **全程无应答**（就绪闸门 reason=sdk-version-missing），表象酷似「工具起不来 / 环境坏了」，
# 于是排查被引向工具与产物。实测处置：换一个空闲端口重跑，同一条命令立刻恢复正常。
# ⚠️ 只在**执行路径**做（`-DryRun` 已在上面退出）⇒ `-DryRun` 的门计划仍与机器状态无关（守护会断言它不跑外部命令）。
function Get-FreeAutoPort {
    param([int]$Start, [int]$Attempts = 20)
    for ($i = 1; $i -le $Attempts; $i++) {
        $candidate = $Start + $i
        if ($candidate -gt 65535) { break }  # 端口号上界：越界值会被当成「没人监听」而蒙过判据
        if (@(Get-NetTCPConnection -State Listen -LocalPort $candidate -ErrorAction SilentlyContinue).Count -eq 0) { return $candidate }
    }
    return 0
}
$portSwitchNote = ''
$occupiedPorts = @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
if ($occupiedPorts.Count -gt 0) {
    $ownerPids = (($occupiedPorts | ForEach-Object { $_.OwningProcess }) | Sort-Object -Unique) -join ','
    $freePort = Get-FreeAutoPort -Start $Port
    if ($freePort -gt 0) {
        $portSwitchNote = "端口 $Port 被 pid $ownerPids 占用（疑似残留自动化会话）：实测 auto 会**静默复用**它（Tool.getInfo 无应答）⇒ 本次改用端口 $freePort"
        Write-Host ">>> $portSwitchNote" -ForegroundColor Yellow
        $Port = $freePort
        $script:GatePlan = New-ThisGatePlan -UsePort $Port
    } else {
        $portSwitchNote = "端口 $Port 被 pid $ownerPids 占用，但窗口内（+20）没有空闲端口 ⇒ 沿用该端口（残留会话复用会让本轮 auto 绑到死会话）"
        Write-Host ">>> $portSwitchNote" -ForegroundColor Yellow
    }
}

if (-not (Test-Path -LiteralPath (Join-Path $Project 'manifest.json'))) {
    Write-Host "[error] 不像 uni-app-x 项目根（缺 manifest.json）：$Project" -ForegroundColor Red
    exit 2
}
New-Item -ItemType Directory -Force -Path $logDir | Out-Null
Set-Content -LiteralPath $logPath -Encoding utf8 -Value @(
    "# mp-weixin-check (②) 开始 $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')",
    "# project = $Project",
    "# 期望 appid = $AppId ; 自动化端口 = $Port ; 路由 = $Routes ; Pages = $Pages ; SkipBuild = $([bool]$SkipBuild)",
    "# 判成败只看输出：HBuilderX CLI / cli.bat / 探针的退出码一律不作门结论",
    "# 门计划（GATE_PLAN）是步骤/参数/预算/结果行字段的唯一真源；见 New-GatePlan 与 -DryRun"
)
function Write-Log {
    param([string]$Text)
    Add-Content -LiteralPath $logPath -Value $Text -Encoding utf8
    # #2162：行首结果行**原样透传** stdout —— 通过分支的裸行此前被「✅ ② 通过：<行>」的中文前缀
    # 遮住（行首判据匹配不上），环境受阻分支则一个字都不打 ⇒ pr-gates 的 mp= 只能读成 no-line。
    # 覆盖三种结果行（通过/门未过/env-blocked，互斥由构造器保证）与 -BuildOnly 的独立 token；
    # 日志文件仍是字节级真源，透传逐字节 = 日志里那行，非结果行维持「只进文件」原语义。
    # 排除 `comment=`（投递诊断行，#2830 先例）：它同样以裸 token 开头但**不是读数行** ——
    # pr-gates 取**最后**一条行首匹配（pr-gates.ps1:239），绿跑后若投递失败，诊断行会顶掉真读数。
    foreach ($ln in ($Text -split "`r?`n")) {
        if ($ln -match '^MP_WEIXIN(_BUILDONLY)?_RESULT(\s|$)' -and $ln -notmatch ' comment=') { Write-Host $ln }
    }
}
# -BuildOnly 的脏改提示（#1861 2026-10-28 裁定「跑完打印脏改提示」）：publish 经 HBuilderX 会回写工作树 ——
# manifest.json 的 mp-weixin.appid 可能被置 null、pages.json 可能被写入 condition（GUI 选的启动页，属本地开发
# 配置、禁止提交）。脚本**不自动改工作树**（自动 checkout 会把用户真实的改动一起抹掉），只把核对与还原动作印出来。
function Write-BuildOnlyWorktreeHint {
    Write-Host ''
    Write-Host '>>> 脏改提示（build-only 不自动改工作树，请人工核对）：' -ForegroundColor Yellow
    Write-Host '    git status --porcelain -- manifest.json pages.json' -ForegroundColor Yellow
    Write-Host '    · manifest.json 的 mp-weixin.appid 被置 null（HBuilderX 回写）⇒ git checkout -- manifest.json' -ForegroundColor Yellow
    Write-Host '    · pages.json 的 condition 段是 GUI 选的启动页（本地开发配置）⇒ 禁止提交，git checkout -- pages.json' -ForegroundColor Yellow
}
# 结果行前缀的**唯一取值点**（#1861）：门计划按 mode 声明（`-BuildOnly` ⇒ MP_WEIXIN_BUILDONLY_RESULT，
# 与 ② 的 MP_WEIXIN_RESULT 可判读分流，同 TEST_UNIT_LOCAL_RESULT 先例）。早期 exit 路径（计划已建但
# 未走主流程）与异常兜底都从这里取 —— 不靠各调用点自觉，否则分流会被「某处硬写前缀」破坏
# （先例：脚本异常兜底行曾硬写 MP_WEIXIN_RESULT，-BuildOnly 下异常就会打成 ② 的 token）。
function Get-GateResultPrefix {
    if ($script:GatePlan -and $script:GatePlan['resultLine']) { return [string]$script:GatePlan['resultLine']['prefix'] }
    return 'MP_WEIXIN_RESULT'
}
# 阶段收尾的**幂等**入口（#1861）：只在真有未闭段时才收尾（否则录制器会抛「无未闭段」）。
# 为什么要这一层而不是让调用点各自判断：-BuildOnly 有两条退出路径 —— 真跑过 HBuilderX 段（wait-hx /
# publish 已 Start，有未闭段）与 -SkipBuild（一段都没 Start）。「有没有未闭段」是录制器自己的状态，
# 不该由调用点各自猜：猜错要么丢掉该出的 STAGE_SUMMARY，要么把一次正常退出抛成 exit 2。
# ② 的主通过路径同样走这一层（语义等价），收尾点因此仍只有一处字面量（接线守护 gateStagesContract C6）。
function Stop-GateStageIfOpen {
    $open = Get-Variable -Name GateStageOpen -Scope Script -ErrorAction SilentlyContinue
    if ($null -ne $open -and $null -ne $open.Value) { Stop-GateStage }
}
# 端口被残留会话占用而换端口这条事实必须进日志（结果行字段清单是固定的，塞不进新字段）
if ($portSwitchNote) { Write-Log "PORT_SWITCH $portSwitchNote" }
# #2001 阶段时间戳：录制器指向本门日志（纯追加 [STAGE]/STAGE_SUMMARY 行，不改门判据）
. (Join-Path $PSScriptRoot 'lib\gate-stages.ps1')
Initialize-GateStageRecorder -LogPath $logPath

# ---------- -Doctor：逐层体检的记录与报告（只读；结论只用于诊断，**不是门的通过结论**）----------
$script:DoctorRows = [System.Collections.Generic.List[object]]::new()
# 体检的**应到层**：报告不全 ⇒ 结论必须是「未就绪」（防「早期 exit 却打出环境就绪」的假绿）
$script:DoctorLayers = @(
    'L1 工具链', 'L2 登录态', 'L3 进程端口', 'L4 构建产物',
    'L5 项目窗口', 'L6 自动化端口', 'L7 会话就绪'
)
function Add-DoctorRow {
    param([string]$Layer, [string]$Verdict, [string]$Detail)
    [void]$script:DoctorRows.Add([pscustomobject]@{ Layer = $Layer; Verdict = $Verdict; Detail = $Detail })
}
function Write-DoctorReport {
    Write-Host ''
    Write-Host '================ ② 环境体检（-Doctor）================' -ForegroundColor Cyan
    if ($script:DoctorRows.Count -eq 0) {
        Write-Host '  （无记录：脚本在记录任何层之前就退出了——读上面的 [error] 行）' -ForegroundColor Yellow
    }
    foreach ($r in $script:DoctorRows) {
        $color = switch ("$($r.Verdict)") { 'OK' { 'Green' } 'WARN' { 'Yellow' } default { 'Red' } }
        Write-Host ("  [{0}] {1,-4} {2}" -f $r.Layer, $r.Verdict, $r.Detail) -ForegroundColor $color
    }
    $fails = @($script:DoctorRows | Where-Object { $_.Verdict -eq 'FAIL' })
    $warns = @($script:DoctorRows | Where-Object { $_.Verdict -eq 'WARN' })
    # **报告不全一律不算就绪**（2026-09-13 首跑实测抓到的假绿）：脚本可能在某一层之前就 exit 2，
    # 此时 rows 里既没有 FAIL 也没有 WARN，旧逻辑会打出「环境就绪（0 条 WARN）」——结论与退出码自相矛盾。
    $seen = @($script:DoctorRows | ForEach-Object { $_.Layer })
    $missing = @($script:DoctorLayers | Where-Object { $seen -notcontains $_ })
    Write-Host ''
    if ($fails.Count -gt 0) {
        Write-Host "结论：环境**未就绪**（$($fails.Count) 层 FAIL）——先修这些层，再跑 ②。" -ForegroundColor Red
    } elseif ($missing.Count -gt 0) {
        Write-Host "结论：环境**未就绪**（报告不全：$($missing -join '、') 这些层没走到 —— 流程在它们之前就退出了，看上面的 [error] 行）。" -ForegroundColor Red
    } else {
        Write-Host "结论：环境**就绪**（$($warns.Count) 条 WARN 不阻断）。" -ForegroundColor Green
    }
    Write-Host '注意：-Doctor 只体检环境，**不产出 ② 门的通过结论**；门结论只能由探针输出（MP_WEIXIN_RESULT）。' -ForegroundColor Yellow
}
function Get-DoctorExitCode {
    if (@($script:DoctorRows | Where-Object { $_.Verdict -eq 'FAIL' }).Count -gt 0) { return 2 }
    # 报告不全是**失败**，不是「没意见」：否则早期 exit 会以 0 收尾，看着像体检通过。
    $seen = @($script:DoctorRows | ForEach-Object { $_.Layer })
    if (@($script:DoctorLayers | Where-Object { $seen -notcontains $_ }).Count -gt 0) { return 2 }
    return 0
}

# 探针脚本 / 路由 / node 都是**会话面**的前置：-BuildOnly 下不跑探针、不开自动化会话 ⇒ 它们缺失不该把
# 构建面判成环境受阻（否则一个「只缺 automator 依赖」的机器会把编译机检假红）。判据面收窄，不是放宽：
# 构建面真正要的东西（HBuilderX + 开发者工具）照旧在下面两条硬前置里。
if (-not $BuildOnly) {
    if (-not (Test-Path -LiteralPath $probePath)) {
        Write-Host "[error] 缺探针脚本：$probePath" -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=no-probe"
        exit 2
    }
    if ($routeList.Count -eq 0) {
        Write-Host '[error] -Routes 为空：至少给一个入口页路由' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=no-routes"
        exit 2
    }
}

# ---------- 环境面 ----------
$hbxRoot = Resolve-HBuilderXRoot -Explicit $HBuilderX -ExplicitCli $Cli
$cliExe = if ($hbxRoot) { Join-Path $hbxRoot 'cli.exe' } else { $null }
$devTools = Resolve-DevToolsCli -Explicit $DevToolsCli
if (-not $SkipBuild) {
    if (-not $hbxRoot) {
        Write-Host '[error] 找不到可用的 HBuilderX（需含 cli.exe 与 plugins/）。用 -HBuilderX <安装根> 或 -Cli <cli.exe> 指定。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=hbuilderx-not-found"
        exit 2
    }
} elseif ($Cli -and -not $hbxRoot) {
    Write-Host "[error] 指定的 cli.exe 不可用（不存在或 PE 头损坏）：$Cli" -ForegroundColor Red
    Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=cli-unusable"
    exit 2
}
if (-not $devTools) {
    Write-Host '[error] 找不到微信开发者工具 cli.bat。用 -DevToolsCli <路径> 指定。' -ForegroundColor Red
    Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=devtools-not-found"
    exit 2
}
# node 只为探针（miniprogram-automator）而存在：-BuildOnly 不跑探针 ⇒ 不把它当构建面前置（同上面的收窄理由）。
$nodeExe = $null
if (-not $BuildOnly) {
    $nodeExe = (Get-Command node -ErrorAction SilentlyContinue | Select-Object -First 1).Source
    if (-not $nodeExe) {
        Write-Host '[error] 找不到 node（跑探针需要 Node.js + miniprogram-automator）。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=node-not-found"
        exit 2
    }
}

# ---------- publish 参数：本轮唯一构造点（#2048）----------
# 为什么要提前到这里构造：指纹的第四道比对必须在**判命中之前**就知道本轮会用哪条命令 —— publish
# 参数住在 .ps1 里，而 .ps1 与 scripts/ 都被构建输入集排除（#2048 G1：改命令不动任何哈希组件、旧产物
# 原样在盘 ⇒ 三组件照样全等 ⇒ 命中 ⇒ 跳过 publish ⇒ 新命令从未执行、门在旧命令产物上出绿）。
# 构造点唯一 ⇒ 比对侧与执行侧用同一份数组，不存在「比 A 跑 B」的两份真相。
. (Join-Path $PSScriptRoot 'lib\mp-weixin-build-fingerprint.ps1')
$publishArgs = @('publish', 'mp-weixin', '--project', $Project)
if ($AppId) { $publishArgs += @('--appid', $AppId) }
$buildCommandKey = Get-MpWeixinBuildCommandKey -Arguments $publishArgs

# ---------- 产物内容指纹 ⇒ 自动 -SkipBuild（#1860）----------
# 白烧场景 = 同树重跑（#1494 §0.1 失败轮 / #1443 实测 -SkipBuild 省 ≈2–3 min）：环境失败轮重跑时
# publish 会重建一模一样的产物。「产物未变」机判化 —— 指纹键三组件**全等才命中**（判据库
# scripts/lib/mp-weixin-build-fingerprint.ps1，范式依据调研 §1.2 条 2）：
#   inputs   = 构建输入集内容哈希 —— 源码变了必不命中（防「改了源码却拿旧产物出绿」的假证据）；
#   product  = 产物目录内容哈希（排除指纹文件自身）—— 产物被换/被改必不命中（「产物或版本变化 ⇒ 必然重建」）；
#   hbuilderx = HBuilderX 版本令牌（目录版本号 + cli.exe 内容哈希；姊妹篇坑位 6b：工具版本会翻结论）。
# 失配方向一律 **fail 向重建**：任一组件算不出 / 指纹缺失或损坏 ⇒ 重建，绝不猜命中。
# 作用域 = 当前树：指纹落产物目录内（.mp-weixin-build-fingerprint.json），跨 worktree 不共享，
# 产物被清时自然失效。「门结论只对这份产物成立」语义不变 —— 指纹只是把「产物没变」机判化。
# 命中即翻 $SkipBuild 并**重建门计划**（mode.skipBuild 是计划面字段，翻值不重建 = 计划与执行脱节；
# 同款先例：端口被占换端口后重建计划）。-Doctor 跳过（体检不构建）；显式 -SkipBuild 无指纹事。
if (-not $SkipBuild -and -not $Doctor) {
    $fpVersion = Get-MpWeixinHBuilderXVersionToken -HbxRoot $hbxRoot -CliExe $cliExe
    $fpHit = $null
    if ("$fpVersion") { $fpHit = Test-MpWeixinBuildFingerprintHit -Project $Project -Dist $dist -HBuilderXVersion $fpVersion -BuildCommand $buildCommandKey }
    if ($fpHit -and $fpHit.Hit) {
        Write-Host ">>> 产物指纹命中 ⇒ 自动 -SkipBuild（$($fpHit.Detail)）" -ForegroundColor Yellow
        Write-Log "AUTO_SKIP_BUILD $($fpHit.Reason) version=$($fpVersion -replace 'cli=[0-9a-f]+', 'cli=<sha256>')"
        $SkipBuild = $true
        $script:GatePlan = New-ThisGatePlan -UsePort $Port
        # 如实记账（#1861）：-BuildOnly 的结果行要区分「本轮真构建过」与「指纹命中跳过」，不许含混。
        # 写在计划重建之后，保持「翻 $SkipBuild ⇒ 立刻重建计划」这条不变量的相邻性（守护 C27 钉的就是它）。
        $script:buildSkippedByFingerprint = $true
    } elseif ($fpHit) {
        Write-Log "build-fingerprint miss reason=$($fpHit.Reason)（$($fpHit.Detail)）"
    } else {
        Write-Log 'build-fingerprint check skipped reason=hbuilderx-version-unavailable（版本令牌算不出 ⇒ 必重建，fail 向重建）'
    }
}

# 端口占用已在**门计划构造之前**处理掉（换空闲端口 + 重建计划）—— 见上面 `Get-FreeAutoPort` 段。
# 这里不再判一次：残留会话会让 `cli.bat auto` **静默复用它**（端口在监听、ws 接得上，但 `Tool.getInfo`
# 无应答 ⇒ 就绪闸门 reason=sdk-version-missing），只 warning 等于把「复用死会话」当成正常路径
# （2026-09-23 实测定位，ADR-0008 坑位 6 续；ADR 坑位 1 讲的 pageStack 空是同一族的另一形态）。

# ---------- -Doctor 的 L1–L4（静态面；在动 close/open/auto 之前先记下来）----------
if ($Doctor) {
    Add-DoctorRow -Layer 'L1 工具链' -Verdict $(if ($devTools) { 'OK' } else { 'FAIL' }) -Detail "微信开发者工具 cli.bat = $devTools"
    Add-DoctorRow -Layer 'L1 工具链' -Verdict $(if ($nodeExe) { 'OK' } else { 'FAIL' }) -Detail "node = $nodeExe"
    # -Doctor 不接 HBuilderX（$SkipBuild 已置位），因此它缺失**不判 FAIL**，只如实报告
    Add-DoctorRow -Layer 'L1 工具链' -Verdict 'OK' -Detail "HBuilderX = $(if ($hbxRoot) { $hbxRoot } else { '(未探测到；-Doctor 不构建，故不判失败)' })"
    Add-DoctorRow -Layer 'L1 工具链' -Verdict $(if (Test-Path -LiteralPath (Join-Path $Project $ReadyRelative)) { 'OK' } else { 'FAIL' }) -Detail "就绪闸门 = $(Join-Path $Project $ReadyRelative)"

    # L2 登录态：开发者工具必须已登录；登录态过期需人补扫一次码（本步只报告，不改门结论）
    try {
        $loginOut = (Invoke-Process -FilePath $devTools -Arguments @('islogin') -TimeoutSeconds 120 -Tag 'doctor-islogin').Output
        Write-Log "`n>>> doctor-islogin`n$loginOut"
        $isLoggedIn = ($loginOut -match '"login"\s*:\s*true')
        Add-DoctorRow -Layer 'L2 登录态' -Verdict $(if ($isLoggedIn) { 'OK' } else { 'FAIL' }) -Detail ("cli.bat islogin -> " + (($loginOut -replace '\s+', ' ').Trim()))
    } catch {
        Add-DoctorRow -Layer 'L2 登录态' -Verdict 'FAIL' -Detail "cli.bat islogin 调用失败：$_"
    }

    # L3 残留会话/端口占用：残留会让 auto 静默复用旧会话（坑位 1）
    $portOwners = @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
    $ideCount = @(Get-Process -Name wechatdevtools -ErrorAction SilentlyContinue).Count
    $portDetail = if ($portOwners.Count -gt 0) {
        (($portOwners | ForEach-Object { "pid=$($_.OwningProcess)@$($_.LocalAddress)" }) -join ',') + '（疑似残留会话，下一步 close 会清）'
    } else { '无' }
    Add-DoctorRow -Layer 'L3 进程端口' -Verdict 'OK' -Detail "wechatdevtools.exe 进程数 = $ideCount（单实例资源）；端口 $Port 占用 = $portDetail"

    # L4 产物：源 manifest appid 与构建产物 appid 都读出来对比（HBuilderX 回写 manifest 置 null 是已知坑）
    $manRaw = Get-Content -LiteralPath (Join-Path $Project 'manifest.json') -Raw
    $manMatch = [regex]::Match($manRaw, '(?s)"mp-weixin"\s*:\s*\{.*?"appid"\s*:\s*"([^"]*)"')
    $srcId = if ($manMatch.Success) { $manMatch.Groups[1].Value } else { '' }
    $prodId = ''
    $cfgPath = Join-Path $dist 'project.config.json'
    if (Test-Path -LiteralPath $cfgPath) {
        $cfgMatch = [regex]::Match((Get-Content -LiteralPath $cfgPath -Raw), '"appid"\s*:\s*"([^"]*)"')
        if ($cfgMatch.Success) { $prodId = $cfgMatch.Groups[1].Value }
    }
    $distOk = Test-Path -LiteralPath (Join-Path $dist 'app.json')
    Add-DoctorRow -Layer 'L4 构建产物' -Verdict $(if ($distOk) { 'OK' } else { 'FAIL' }) -Detail "dist app.json = $distOk（$dist）"
    $idOk = [bool]($prodId -and ((-not $AppId) -or $prodId -eq $AppId))
    $srcNote = if ($srcId -ne $AppId) { '  ← 源 manifest 与期望不一致：HBuilderX 回写置 null 的痕迹，先 `git checkout -- manifest.json`' } else { '' }
    Add-DoctorRow -Layer 'L4 构建产物' -Verdict $(if ($idOk) { 'OK' } else { 'FAIL' }) -Detail "产物 appid = '$prodId'（期望 '$AppId'）；源 manifest appid = '$srcId'$srcNote"
}

# ---------- 收尾：cli.bat close 必须在 finally 路径里执行，避免端口/会话残留 ----------
function Invoke-DevToolsClose {
    if ($script:closed) { return }
    $script:closed = $true
    if (-not $devTools) { return }
    # #1860：楔死早退路径不收尾 —— close 对死端口 .ide 会自启 IDE 再关（cli 自启同族），且本路径
    # 从未创建自动化会话、无残留可收；处置已指向人工，跳过（哨兵见 $script:skipFinalClose 声明处）。
    if ($script:skipFinalClose) { return }
    # 参数（含 --project 与超时）一律取自门计划，不再写死在调用点（#914：判据面 = 计划，不是源码文本）
    $closeStep = Get-GateStep -Plan $script:GatePlan -Id 'devtools-close'
    try {
        $r = Invoke-Process -FilePath $devTools -Arguments $closeStep.argv -TimeoutSeconds $closeStep.timeoutSeconds -Tag $closeStep.tag
        # F4（2026-10-29）：close 后开发者工具**本来就会活着**（它只关项目窗口），继承 stdout 句柄是常态。
        # 把这层预期写进日志，免得下一个人把 Invoke-Process 的 [warn] 读成故障。
        if ($r.DrainAbandoned) { Write-Log ">>> $($closeStep.tag)：主进程已退出，输出流被常驻 IDE 继承（DrainAbandoned=true，预期行为，不影响结论）" }
        Write-Log "`n>>> $($closeStep.tag)`n$($r.Output)"
        Write-Host $r.Output
    } catch {
        Write-Host "[warn] cli.bat close 失败（不影响门结论）：$_" -ForegroundColor Yellow
    }
}

# ---------- Pre-flight：cli.bat quit 清场（#1634）----------
# 病根（#1634）：用脚本启动（close → open → auto 的 IPC 链）时，残留/堆叠的 IDE 实例会让 open 把项目窗口
# 叠进一个未初始化的实例 ⇒ 项目窗口白屏（连菜单/Console 都没有）。直接双击 exe 启动不复现 —— 所以这是
# 「跑门前」的清场问题，不是工具自身的缺陷。quit 退出**整个** IDE（close 只关项目窗口，对未初始化实例
# 无能为力），之后再 open 就是干净起点。等价于 ADR-0008 已记录的处置：跑前先 quit 清场，实测有效。
# ⚠️ 两条红线：① 不得用 Stop-Process 替代 quit（运行中 kill IDE 是 ADR-0008 红线）；② quit/轮询失败只
# [warn] 继续，不判 exit 2 —— 留下的白屏风险由 open/auto/就绪闸门的协议级判据兜（那里判得出真形态）。
# #1682 冒烟实测：直达分支的**第二次 close→open→auto 循环**（同一 IDE 进程内）会稳定复现此白屏形态
# （getInfo 全程 no-reply 直到 120s 超时），close 修不好、quit 重启即恢复（实测 12s 就绪）
# ⇒ 本函数体抽成 Invoke-DevToolsQuitAndPoll 供直达冷启动的重试轮升级调用（哨兵语义只留 pre-flight）。
# 端口收摊轮询（#1690 run5 语义）：等自动化端口随进程/服务关净；预算内关不掉 ⇒ $false
# （port-held-after-quit，直达冷启动据此诚实记环境性）。quit 路径与 #1860 的 0 进程跳过路径共用。
function Wait-MpWeixinAutoPortReleased {
    param([int]$Port, [int]$BudgetSeconds = 60)
    $held = 0
    while ($held -lt $BudgetSeconds) {
        if (@(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue).Count -eq 0) { return $true }
        Start-Sleep -Seconds 1
        $held++
    }
    return $false
}

function Invoke-DevToolsQuitAndPoll {
    if (-not $devTools) { return }
    # #1860：quit 之前先清点 IDE 进程 —— 0 个 ⇒ 不执行 quit。`cli quit` 在 IDE 未运行时会**自启**
    # 一个无项目实例并占住服务端口（官方 `--port` 全局选项「没启动则会启动」的同族机制；2026-10-23
    # 本机实测：自启实例占 43005 ⇒ open 120s 超时 ⇒ 一轮 9–26 min 白烧）。quit 只在「确实有 IDE 在跑」
    # 时才有意义；IDE 本体的退出仍只走 cli quit（这里只是不发起 quit，Stop-Process 红线不破）。
    $ideProcsBefore = @(Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match 'wechatdevtools|微信开发者工具' })
    if ($ideProcsBefore.Count -eq 0) {
        Write-Log "quit：IDE 进程数 = 0（wechatdevtools/微信开发者工具 双名匹配）⇒ 跳过 cli quit（防 quit 自启无项目实例占服务端口，#1860）"
        # 没跑 quit ⇒ 没有「quit 后静置」问题（10s 静置与进程归零轮询都是 quit 的善后，一并跳过）；
        # 但**端口收摊判据照旧**（Wait-MpWeixinAutoPortReleased 同一真源）：端口仍被占（垂死的自动化
        # 服务/别的进程）时等它关净，关不掉 ⇒ port-held-after-quit（诚实，不冒充干净）。
        if (Wait-MpWeixinAutoPortReleased -Port $Port) { return $true }
        Write-Log "quit：自动化端口 $Port 60s 后仍在 listening（无 IDE 进程 ⇒ 占端口的是别的服务）⇒ port-held-after-quit（照实报，不 kill）"
        return $false
    }
    $quitStep = Get-GateStep -Plan $script:GatePlan -Id 'devtools-quit'
    try {
        $r = Invoke-Process -FilePath $devTools -Arguments $quitStep.argv -TimeoutSeconds $quitStep.timeoutSeconds -Tag $quitStep.tag
        Write-Log "`n>>> $($quitStep.tag)（清场：退出整个 IDE，防残留实例把项目窗口叠成白屏）`n$($r.Output)"
        Write-Host $r.Output
    } catch {
        Write-Host "[warn] cli.bat quit 失败（继续，不影响门结论）：$_" -ForegroundColor Yellow
    }
    # 轮询 IDE 进程归零：quit 是异步的，端口/会话收干净要等。超时只 [warn]，绝不 kill。
    # #1690 实测修正：本机 IDE 的进程名是中文「微信开发者工具」（exe 随安装包语言），只数
    # -Name wechatdevtools 在这台机器恒 0 ⇒ 归零轮询形同虚设，quit 还没真收摊就 open，
    # 冷启动叠在垂死实例上 ⇒ storage 落盘不完整、直达页会话丢失。按 ProcessName 双名匹配。
    # #1690 run5 实测：首轮 quit 后 60s 仍有 4 个进程未退 ⇒ open 连上垂死实例、冷启动骑旧会话
    # （入口页探针拿到旧栈判 miss）。补第二轮 quit 再轮询；**只观测不 kill**（ADR-0008 红线不变）。
    $clean = $false
    for ($round = 1; $round -le 2; $round++) {
        if ($round -gt 1) {
            Write-Host "[warn] 首轮 quit 后进程未归零，补发第二次 quit 再轮询" -ForegroundColor Yellow
            Write-Log 'quit 第 2 轮：补发 cli quit 再归零轮询'
            try { [void](Invoke-Process -FilePath $devTools -Arguments $quitStep.argv -TimeoutSeconds $quitStep.timeoutSeconds -Tag "$($quitStep.tag)(retry)") } catch { Write-Host "[warn] 第二次 quit 失败（继续轮询）：$_" -ForegroundColor Yellow }
        }
        $waited = 0
        while ($waited -lt [int]$quitStep.waitSeconds) {
            if (@(Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match 'wechatdevtools|微信开发者工具' }).Count -eq 0) {
                Write-Log "quit：IDE 进程已归零（wechatdevtools/微信开发者工具 双名匹配，等了 ${waited}s）"
                $clean = $true
                break
            }
            Start-Sleep -Seconds 1
            $waited++
        }
        if ($clean) { break }
    }
    if (-not $clean) {
        $still = @(Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -match 'wechatdevtools|微信开发者工具' }).Count
        Write-Host "[warn] 两轮 quit（各 $([int]$quitStep.waitSeconds)s）后仍有 $still 个 IDE 进程未退出 —— 不 kill（ADR-0008 红线），继续跑门；若白屏复现优先怀疑残留实例。" -ForegroundColor Yellow
        Write-Log "两轮 quit 后仍有 $still 个 IDE 进程未退出（只警告，不 kill）"
    }
    # #1682 第四轮实测：进程归零 ≠ IDE server（43005）状态收干净——quit 后立刻 open，
    # auto 会挂满 180s 超时；手动实验 quit 后静置 8s 再 open→auto→ready 12s 成 ⇒ 固定静置。
    Start-Sleep -Seconds 10
    # 端口收摊判据（#1690 run5）：垂死实例会**占住自动化端口**，open/auto 接上去骑旧会话。
    # 等端口随进程关净；预算内关不掉 ⇒ 返回 $false，直达冷启动据此记 port-held-after-quit（环境性，诚实）。
    if (Wait-MpWeixinAutoPortReleased -Port $Port) { return $true }
    Write-Log "quit：自动化端口 $Port 60s 后仍在 listening（垂死实例占会话）⇒ 直达冷启动判 port-held-after-quit"
    return $false
}

function Invoke-DevToolsPreflightQuit {
    if ($script:quitDone) { return }
    $script:quitDone = $true
    # #1690：函数现在返回「端口是否收干净」；主链 pre-flight 只清场不改判 ⇒ 显式弃值（直达冷启动才消费）。
    $null = Invoke-DevToolsQuitAndPoll
}

# ---------- 任意页直达探针（#1682 · ADR-0038 决策 1/2 · ADR-0039 拆分步骤 2）----------
# 机制（已裁定，勿另选）：把**构建产物** app.json 的 pages 首项临时换成目标页，再走一遍
# close → open → auto → 端口 → 就绪闸门 的 plain 冷启动 ⇒ 小程序落地页就是目标页，全程**不调用任何
# 导航 API**（本机 automator 0.12.1 的 reLaunch/navigateTo 恒报 Uncaught [object Object]，#1674 先例）。
# 换页三纪律（ADR-0038 决策 2）：换前 ConvertFrom-Json 校验；换后读回复验首项；用毕从备份还原并复验。
# ⚠️ 只动 $dist 下的构建产物（gitignored），**源头 pages.json 一个字都不改** —— 它是运行时面判据来源。
function Switch-DistAppJsonFirstPage {
    param([string]$Path, [string]$TargetRoute)
    # 返回 $null = 已换好；否则返回 SKIP 原因（page-not-in-build / app-json-invalid）
    $target = ($TargetRoute -replace '^/', '')
    $j = $null
    try { $j = (Get-Content -LiteralPath $Path -Raw) | ConvertFrom-Json } catch { return 'app-json-invalid' }
    if ($null -eq $j -or -not $j.pages -or @($j.pages).Count -eq 0) { return 'app-json-invalid' }
    $pages = @($j.pages)
    $idx = -1
    for ($i = 0; $i -lt $pages.Count; $i++) { if ("$($pages[$i])" -eq $target) { $idx = $i; break } }
    if ($idx -lt 0) { return 'page-not-in-build' }
    if ($idx -ne 0) {
        $tmp = $pages[0]; $pages[0] = $pages[$idx]; $pages[$idx] = $tmp
        $j.pages = $pages
    }
    # 键序轮转（#1690 run6 实测口径）：IDE 编译缓存按 app.json **字节**认新——pages[0] 已是目标页时
    # 重序列化字节不变 ⇒ IDE 不重编译、窗口还原上一会话页栈 ⇒ 入口页直达拿旧栈假 miss（run6 实测
    # [dashboard] 顶掉 [index]）。顶层键序轮一位保证字节必变（语义不变）⇒ 强制 fresh 编译从 pages[0] 起。
    $keys = @($j.PSObject.Properties.Name)
    $out = $null
    if ($keys.Count -ge 2) {
        $rot = @($keys[1..($keys.Count - 1)]) + @($keys[0])
        $od = [ordered]@{}
        foreach ($k in $rot) { $od[$k] = $j.$k }
        $out = ($od | ConvertTo-Json -Depth 30)
    } else {
        $out = ($j | ConvertTo-Json -Depth 30) + ' '
    }
    Set-Content -LiteralPath $Path -Value $out -Encoding utf8NoBOM -NoNewline
    # 换后复验（判据 = 读回，不是「写了就当成对了」）：首项必须是目标页
    $re = $null
    try { $re = (Get-Content -LiteralPath $Path -Raw) | ConvertFrom-Json } catch { return 'app-json-invalid' }
    if ($null -eq $re -or -not $re.pages -or "$(@($re.pages)[0])" -ne $target) { return 'app-json-invalid' }
    # 唯一性复验（#1690 run7-10 实测教训）：重复路由 ⇒ 构建器拒启模拟器（症状 page-stack-not-answering），
    # 换页前文件已被外部污染时这里拦住，不让它烧进 120s 僵尸等待
    if (@($re.pages | Select-Object -Unique).Count -ne @($re.pages).Count) { return 'app-json-invalid' }
    return $null
}

function Restore-DistAppJson {
    # 幂等：先清哨兵再写回（finally 与主路径可能各命中一次）；还原失败只 [warn] 不改变门结论
    if (-not $script:AppJsonBackup) { return }
    $bk = $script:AppJsonBackup
    $script:AppJsonBackup = $null
    try {
        Set-Content -LiteralPath $bk.Path -Value $bk.Text -Encoding utf8NoBOM -NoNewline
        $firstOrig = ''
        try { $firstOrig = "$(@(((Get-Content -LiteralPath $bk.Path -Raw) | ConvertFrom-Json).pages)[0])" } catch { $firstOrig = '(复验失败)' }
        $wantFirst = ''
        try { $wantFirst = "$(@(($bk.Text | ConvertFrom-Json).pages)[0])" } catch { $wantFirst = '' }
        if ($wantFirst -and $firstOrig -ne $wantFirst) {
            Write-Host "[warn] app.json 已写回但首项与备份不一致（人工复核 $($bk.Path)）" -ForegroundColor Yellow
            Write-Log "direct-pages: app.json 还原复验不一致 path=$($bk.Path)"
        } else {
            Write-Log "direct-pages: dist app.json 已从备份还原并复验（首项=$firstOrig）"
        }
    } catch {
        Write-Host "[warn] app.json 还原失败（人工复核：$($bk.Path)）：$_" -ForegroundColor Yellow
    }
}

function Invoke-DirectColdStart {
    param($Steps, [string]$PageTag)
    # 逐页换页后的重冷启动单元：open → auto → 端口轮询 → 就绪闸门（参数一律取自传入的计划
    # 步骤——守护 C18 的「调用点原文至多一次」不允许这里重复写死 argv）。
    # ⚠️ 每一轮都整 IDE 重启（quit→open），不走 close→open 的第二窗口循环：#1682 冒烟四轮实测
    # 同一 IDE 进程内第二次 close→open→auto 循环大概率复现 #1634 白屏（getInfo 全程 no-reply 烧满
    # 120s 超时），而 quit 重启路径（含归零后静置）三轮全成 ⇒ 直达页首轮即 quit 重启，省掉白屏赌局。
    $attempts = [Math]::Max(1, [int]$script:GatePlan['timeouts']['autoAttempts'])
    $delay = [Math]::Max(0, [int]$script:GatePlan['timeouts']['autoRetryDelay'])
    $out = @{ ok = $false; reason = 'port-not-listening'; readySeconds = 0; sdk = ''; ide = '' }
    for ($attempt = 1; $attempt -le $attempts; $attempt++) {
        if ($attempt -gt 1) {
            Write-Log "`n>>> direct-coldstart($PageTag)：整段重试第 $attempt/$attempts 次（等 ${delay}s；上一轮 reason=$($out.reason)）"
            Start-Sleep -Seconds $delay
        }
        # quit 后旧窗口随进程消失；重启出的新窗口仍需 finally 的 close 收掉 ⇒ 复位哨兵。
        # 归零 + 端口收摊都过关才继续（#1690 run5：垂死实例占端口 ⇒ open 骑旧会话 ⇒ 假 miss）。
        $quitClean = Invoke-DevToolsQuitAndPoll
        if (-not $quitClean) { $out.reason = 'port-held-after-quit'; continue }
        $script:closed = $false
        $open = Invoke-Process -FilePath $devTools -Arguments $Steps.open.argv -TimeoutSeconds $Steps.open.timeoutSeconds -Tag "$($Steps.open.tag)(direct:$PageTag)"
        Write-Log "`n>>> direct-coldstart open($PageTag)`n$($open.Output)"
        if ($open.TimedOut) { $out.reason = 'devtools-open-timeout'; continue }
        $auto = Invoke-Process -FilePath $devTools -Arguments $Steps.auto.argv -TimeoutSeconds $Steps.auto.timeoutSeconds -Tag "$($Steps.auto.tag)(direct:$PageTag)"
        Write-Log "`n>>> direct-coldstart auto($PageTag)`n$($auto.Output)"
        if ($auto.TimedOut) { $out.reason = 'auto-timeout'; continue }
        $ma = [regex]::Match($auto.Output, 'Using AppID:\s*(\S+)')
        if ($AppId -and $ma.Success -and $ma.Groups[1].Value -ne $AppId) {
            Write-Log "direct-coldstart($PageTag)：auto 回显 AppID=$($ma.Groups[1].Value) 与期望不一致（首轮产物断言已把关，这里只记录）"
        }
        $listening = $false
        for ($i = 0; $i -lt $Steps.port.waitSeconds; $i++) {
            if (@(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue).Count -gt 0) { $listening = $true; break }
            Start-Sleep -Seconds 1
        }
        if (-not $listening) { $out.reason = 'port-not-listening'; continue }
        $ready = Invoke-Process -FilePath $nodeExe -Arguments $Steps.ready.argv -TimeoutSeconds $Steps.ready.timeoutSeconds -Tag "$($Steps.ready.tag)(direct:$PageTag)"
        Write-Log "`n>>> direct-coldstart ready($PageTag)`n$($ready.Output)"
        $rj = $null
        foreach ($line in ($ready.Output -split "`r?`n")) {
            if ($line -match '^\s*MP_WEIXIN_READY\s+(\{.*\})\s*$') { try { $rj = ($Matches[1] | ConvertFrom-Json) } catch { $rj = $null } }
        }
        if (-not $rj) { $out.reason = 'no-ready-result'; continue }
        if (((Get-Prop $rj 'ok') -ne $true)) { $out.reason = 'automation-not-ready'; continue }
        $re = Get-Prop $rj 'elapsedMs'
        if ($null -eq $re) { $re = 0 }
        $out.readySeconds = [int]([double]$re / 1000)
        $out.sdk = "$(Get-Prop $rj 'sdkVersion')"
        $out.ide = "$(Get-Prop $rj 'ideVersion')"
        $out.ok = $true
        $out.reason = ''
        return $out
    }
    return $out
}

function Invoke-DirectProbe {
    param($ProbeStep, $DirectPages, [string]$Page)
    # argv 模板取自门计划 directPages.probeArgv（唯一真源），<page> 占位在这里才落成具体路由。
    # 重试判据与旧逐页导航不同：**只在会话级短暂失败上重试**（没拿到结果行 / 没连上）；
    # 一旦连上，页报错/直达未达成都是**真实读数**，重试会把 errorsTotal 重复计数 ⇒ 不重试。
    $argv = @($DirectPages.probeArgv | ForEach-Object { if ("$_" -eq '<page>') { $Page } else { $_ } })
    # 锚点断言（#1919）：anchors 表命中该页才追加 --assert-selector/--assert-text（按序配对）；
    # 未命中页 argv 不变（诚实降级面——不新增失败面）。表真源在门计划 directPages.anchors。
    $anchors = Get-Prop $DirectPages 'anchors'
    $anchorOne = Get-Prop $anchors $Page
    if ($anchorOne) {
        $argv += @('--assert-selector', "$(Get-Prop $anchorOne 'selector')", '--assert-text', "$(Get-Prop $anchorOne 'text')")
    }
    $json = $null
    $timedOut = $false
    $attempts = [Math]::Max(1, [int]$DirectPages.attempts)
    for ($attempt = 1; $attempt -le $attempts; $attempt++) {
        Write-Log "`n>>> direct-probe($Page) 第 $attempt/$attempts 次"
        $r = Invoke-Process -FilePath $nodeExe -Arguments $argv -TimeoutSeconds $ProbeStep.timeoutSeconds -Tag "automator-probe(direct:$Page)#$attempt"
        Write-Log $r.Output
        $timedOut = [bool]$r.TimedOut
        if ($timedOut) { $json = $null }
        foreach ($line in ($r.Output -split "`r?`n")) {
            if ($line -match '^\s*MP_WEIXIN_PROBE\s+(\{.*\})\s*$') { try { $json = ($Matches[1] | ConvertFrom-Json) } catch { $json = $null } }
        }
        if ($json -and $json.connected) { break }
        if ($attempt -lt $attempts) { Start-Sleep -Seconds ([int]$DirectPages.retryDelaySeconds) }
    }
    return @{ json = $json; timedOut = $timedOut }
}

function Invoke-AuthInject {
    param($AuthStep, [string]$PageTag, [string]$Mode = 'auth')
    # 会话注入（#1690）：argv 取自门计划（唯一真源）；判成败只看 MP_WEIXIN_AUTH 结果行（退出码不作判据）。
    # 失败**不判环境受阻**：返回 ok=false + kebab reason，由调用方把该页如实记 miss —— 注入不可用
    # 既不得升级成 exit 2，也不得冒充取证成立。脚本自身幂等（已有会话 ⇒ existing 即返）。
    # #1863：detail 一并透传（HTTP 状态 / 网络错误 cause / 超时）—— 调用方要把失败归因带进结果行，
    # #1845 实测 reason=wx-login-api-failed 无 detail 时无法归因。
    $argv = @($AuthStep.argv) + @('--mode', $Mode)
    $r = Invoke-Process -FilePath $nodeExe -Arguments $argv -TimeoutSeconds $AuthStep.timeoutSeconds -Tag "auth-inject($PageTag)"
    Write-Log "`n>>> auth-inject($PageTag)`n$($r.Output)"
    $json = $null
    foreach ($line in ($r.Output -split "`r?`n")) {
        if ($line -match '^\s*MP_WEIXIN_AUTH\s+(\{.*\})\s*$') { try { $json = ($Matches[1] | ConvertFrom-Json) } catch { $json = $null } }
    }
    if (-not $json) { return @{ ok = $false; status = 'none'; reason = 'no-auth-result'; detail = '' } }
    return @{ ok = ((Get-Prop $json 'ok') -eq $true); status = "$(Get-Prop $json 'status')"; reason = "$(Get-Prop $json 'reason')"; detail = "$(Get-Prop $json 'detail')" }
}

# ---------- P1：门通过时把结果贴成「sha 绑定」的 PR 评论（② 门结果免手抄）----------
# Get-HeadSha 已上移到 scripts/lib/gate-common.ps1（三份**逐字节相同**、且无任何守护 pin ⇒ 唯一合格的零风险
# 切片）。共享库头部写明了准入门槛：只有「逐字节相同」**且**「未被 utils/*Contract.test.js 作为字面量锚点
# pin 住」的代码才允许搬进去 —— 本仓守护断言的是**源码文本**，搬走被 pin 的代码等于逼着后续放宽守护。
. (Join-Path $PSScriptRoot 'lib\gate-common.ps1')
# 截图入库的判据面（`Get-MadeTotalBytes` 的空安全）：**必须 dot-source 才可用**，
# 而 `Publish-ScreenshotArchive` 在 main 流程里才被调用 ⇒ 这里（main 之前）是它唯一可靠的落点。
. (Join-Path $PSScriptRoot 'lib\mp-weixin-archive.ps1')
# 环境性失败三分的分类与结果行构造（#1655 / ADR-0036 处置二）：判据面单点真源 ——
# :1535 一带的分类、env-blocked/门未过两种结果行的形状都出自它，
# 守护用夹具**真执行**这些函数（utils/mpWeixinGateEnvBehavior.test.js / mpWeixinGateResultLineBehavior.test.js），
# 不对源码文本断言（C20 先例）。分支声明形状见 GATE_PLAN 的 branches 键。
. (Join-Path $PSScriptRoot 'lib\mp-weixin-env.ps1')
# -Pages auto 的改动页清单判据（#1853）：真源在 scripts/lib/mp-weixin-pages.ps1（编码层双解耦的
# 根因叙事见该库头注释），守护用真实 git 仓 + gb2312 反条件**真执行**它
# （utils/mpWeixinGatePagesBehavior.test.js），不对源码文本断言（C20 先例）。
. (Join-Path $PSScriptRoot 'lib\mp-weixin-pages.ps1')
# sha 绑定口径（#1695）：评论里的 `- commit:` 只绑「GitHub 点得开」的提交 ——
# 判定与 push 收在 lib/gate-commit.ps1（头注释写明 A/B/C 三级口径）。
. (Join-Path $PSScriptRoot 'lib\gate-commit.ps1')

function Publish-GateComment {
    param(
        [int]$PrNumber,
        [string]$ResultLine,
        [string]$LogRelative,
        [string]$ShotRelative,
        [string]$ReproCommand,
        [string[]]$ArchivedRel = @(),
        [string[]]$ArchiveNotes = @(),
        [bool]$NavSkipped = $false,
        [string]$NavReason = '',
        [string[]]$DirectPageLines = @()
    )
    $sha = Get-HeadSha -ProjectDir $Project
    if (-not $sha) {
        Write-Host '[warn] 取不到 HEAD sha（git 不可用或不在仓库里）：跳过贴 PR 评论，门结论不受影响。' -ForegroundColor Yellow
        return
    }
    # #1695：只绑「GitHub 点得开」的 sha —— HEAD 还没推上去（rebase 后 / 归档提交仅本地）时绑远端
    # 分支 tip，并把「本地归档 sha（未推送）」如实写进正文（三级口径见 lib/gate-commit.ps1 头注释）。
    $binding = Resolve-CommitBinding -ProjectDir $Project -HeadSha $sha
    $sha = $binding.Sha
    $short = $sha.Substring(0, [Math]::Min(7, $sha.Length))
    # 正文格式被 .github/workflows/pr-evidence.yml 认（标记 ② + commit sha），改动前先看那边的注释
    # #1682 核验补记①：-NoArchive / 未入库时调用点传 @(Get-Prop $null 'Rel') = @($null)——
    # 单元素数组（元素是 null）Count=1，不过滤会渲染成「一条空反引号条目」的假入库清单，
    # 设计好的兜底文案反而永不出现。先滤掉 null/空再决定走清单还是兜底。
    $archRels = @(@($ArchivedRel) | Where-Object { $_ })
    $archNotes = @(@($ArchiveNotes) | Where-Object { $_ })
    $archLines = @()
    if ($archRels.Count -gt 0) {
        $archLines += '- 入库截图（仓库内相对路径，GitHub 可点开预览）：'
        foreach ($r in $archRels) { $archLines += ('  - ``' + $r + '``') }
    } else {
        $archLines += '- 入库截图：（无 —— 见脚本输出的 [archive] 说明，未入库不影响门结论）'
    }
    if ($archNotes.Count -gt 0) { $archLines += ('- 入库说明：' + ($archNotes -join '；')) }
    # 降级必须**显式写明**（不许假绿）：评论里不写清楚，读评论的人会把「可验证子集通过」当成「逐页都过了」。
    # #1682：直达探针（cold-start-direct）的 SKIP 与旧导航组的 SKIP 是**两种成因**，措辞分开——
    # 前者是「某页冷启动后落地页 ≠ 目标路由」，后者才是「导航 API 本机恒不可用」。
    $navLines = @()
    if ($NavSkipped) {
        if ($NavReason -like 'direct-*' -or $NavReason -eq 'page-not-in-build' -or $NavReason -eq 'app-json-invalid') {
            $navLines += "- ⚠️ 降级（SKIP，非 PASS）：任意页直达探针（#1682）有页未达成 = SKIP（reason=$($NavReason)）——机制是「构建产物 app.json 首项临时换页 + plain 冷启动」，未达成的页**不得**当成已取证。"
            $navLines += '- 带参页口径（#2110 定案，2026-10-29）：依赖入口 query 的页（featured-detail / ai-feature / job-detail / resume-detail 等）在编译条件下直达内容形态结构不可得（CLI 参数被静默吞、私有配置 condition 段不生效，单验读数 .ci-verify/2110）⇒ ② 记 SKIP，**不得用空壳 / 「加载中…」截图充当该页证据**；内容取证落 ①（真机 --pagePath + --pageQuery）或发版冒烟。'
            $navLines += '- 门只对**可验证子集**作结论：已达成页逐页 console/exception 读数 + 入口页 `pageStack` 落地判据；**未取证的页**见下面的逐页读数与日志 ``' + $LogRelative + '``。'
        } else {
            $navLines += "- ⚠️ 降级（SKIP，非 PASS）：逐页导航 + 每页截图 = SKIP（reason=$($NavReason)）——本机 ``miniprogram-automator@0.12.1`` 的 ``mp.reLaunch`` / ``mp.navigateTo`` 恒报 ``Uncaught [object Object]``，该组断言在此环境不成立。"
            $navLines += '- 门只对**可验证子集**作结论：`connected=true` + `pageStack` 非空 + 入口页在栈内 + `errorsTotal==0 && exceptionsTotal==0` + 至少 1 张当前页截图；**未取证的页**见上面的 `navigation=` 字段与日志 ``' + $LogRelative + '``。'
            $navLines += '- **未证实**：是否为版本组合（automator 0.12.1 × 开发者工具 Stable v2.01.2510290）问题，留给后续排查；本条不宣称已定位根因，**首跑须人工核对 `navigation=` 字段**。'
        }
    }
    # 逐页直达读数（#1682 验收 4：实测时长/逐页读数进 sha 绑定评论，供 0039 窗口主门成本校准）
    $directOut = @()
    if (@($DirectPageLines).Count -gt 0) {
        $directOut += '- 逐页直达读数（#1682，机制=cold-start-direct）：'
        foreach ($l in @($DirectPageLines)) { $directOut += ('  - ``' + $l + '``') }
    }
    $body = @(
        '<!-- gate-evidence:② -->',
        '**② 微信开发者工具无报错（半自动，agent 执行）**',
        "- commit: $sha",
        "- 结论（含产物）：``$ResultLine``；本地截图 ``$ShotRelative``；日志 ``$LogRelative``",
        '- 非等价声明：② ≠ ① 真机门，也 ≠ ④b 云打包门（content:// / 生物识别 / 第三方 SDK / 真机性能 挡不住）。'
    ) + @($binding.NoteLines) + $navLines + $directOut + $archLines + @(
        "- 复现：``$ReproCommand``"
    )
    $body = $body -join "`n"
    # #1770/#1841：判据评论先贴 commit（证据主键），同一份正文的展示副本贴号码 —— 两个落点都在 lib/gate-commit.ps1 里。
    # commit 端点回 404 时号码评论升格为正式证据落点（显式降级，不判红）；号码仍是「归档目录键 / 批次标识」。
    try {
        $sent = Send-GateComment -ProjectDir $Project -Sha $sha -Reachable ([bool]$binding.Reachable) -IssueNumber $PrNumber -Body $body
        # A1（#2100）：投递已退役 ⇒ 原先三支（降级 / 失败 / 成功）的判据**全部恒假**，整段已删：
        # CommitUnavailable 恒 $false、EvidenceUrl 恒空、CommitKind 恒 'retired'、Errors 恒空、CommitOk 恒 $false。
        # 证据块由 Send-GateComment 直接打印；这里只回声它的去路说明（Notes）。门结论不信报告层（#1209）。
        foreach ($note in @($sent.Notes)) { Write-Host $note }
    } catch {
        Write-Host "[warn] 门评论投递异常（门结论不受影响）：$_" -ForegroundColor Yellow
    }
}
# ---------- 截图入库（P2：② 的截图自动压缩提交到 PR 分支）----------
# 约束（维护者 2026-09-12 追加）：优先 WebP q75（探测 cwebp / ffmpeg / magick，择一），没有就退 JPEG q75
# （.NET System.Drawing，Windows 自带）；宽度上限 720px（等比、不放大）；每 PR ≤10 张、单张 ≤150KB、合计 ≤1.5MB；
# 超限先降质 → 再缩尺 → 再减图（减图必须在评论里写明省了哪几页）。
# **不得让门因此失败**：任何一步出错只打印警告 + 给出待执行的 git 命令。
$script:ArchiveMaxCount = 10
$script:ArchiveMaxBytes = 150KB
$script:ArchiveMaxTotal = 1.5MB
$script:ArchiveMaxWidth = 720
$script:ArchiveFail = $null

function Get-ImageCodec { return [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object { $_.MimeType -eq 'image/jpeg' } | Select-Object -First 1 }

function Get-WebpEncoder {
    foreach ($t in @('cwebp', 'ffmpeg', 'magick')) {
        $c = Get-Command $t -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($c) { return @{ Name = $t; Path = $c.Source } }
    }
    return $null
}

function Save-Jpeg {
    param($Bitmap, [string]$Path, [int]$Quality)
    $codec = Get-ImageCodec
    $eps = [System.Drawing.Imaging.EncoderParameters]::new(1)
    $eps.Param[0] = [System.Drawing.Imaging.EncoderParameter]::new([System.Drawing.Imaging.Encoder]::Quality, [int64]$Quality)
    $bmp2 = [System.Drawing.Bitmap]::new($Bitmap.Width, $Bitmap.Height, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp2)
    $g.Clear([System.Drawing.Color]::White)   # JPEG 无 alpha：先把透明底填白，免得出现黑块
    $g.DrawImage($Bitmap, 0, 0, $Bitmap.Width, $Bitmap.Height)
    $g.Dispose()
    try { $bmp2.Save($Path, $codec, $eps) } finally { $bmp2.Dispose(); $eps.Dispose() }
}

function Invoke-WebpEncode {
    param([string]$Tool, [string]$EncoderPath, [string]$PngPath, [string]$OutPath, [int]$Quality)
    switch ($Tool) {
        'cwebp' { $r = Invoke-Process -FilePath $EncoderPath -Arguments @('-q', "$Quality", '-quiet', $PngPath, '-o', $OutPath) -TimeoutSeconds 120 -Tag 'cwebp' }
        'ffmpeg' { $r = Invoke-Process -FilePath $EncoderPath -Arguments @('-y', '-loglevel', 'error', '-i', $PngPath, '-quality', "$Quality", $OutPath) -TimeoutSeconds 120 -Tag 'ffmpeg' }
        'magick' { $r = Invoke-Process -FilePath $EncoderPath -Arguments @($PngPath, '-quality', "$Quality", $OutPath) -TimeoutSeconds 120 -Tag 'magick' }
    }
    return (Test-Path -LiteralPath $OutPath)
}

function Export-ArchiveImage {
    param([string]$SourcePng, [string]$OutBase, [int]$MaxWidth, $Encoder, [int]$Quality)
    # 返回 @{ Path, Bytes, Width, Height, Format } 或 $null
    $src = $null
    try { $src = [System.Drawing.Image]::FromFile($SourcePng) } catch { return $null }
    try {
        $w = $src.Width; $h = $src.Height
        if ($w -gt $MaxWidth) { $h = [int][Math]::Round($h * ($MaxWidth / [double]$w)); $w = $MaxWidth }
        $scaled = [System.Drawing.Bitmap]::new($w, $h, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
        $g = [System.Drawing.Graphics]::FromImage($scaled)
        $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        $g.Clear([System.Drawing.Color]::White)
        $g.DrawImage($src, 0, 0, $w, $h)
        $g.Dispose()
        $out = "$OutBase.jpg"; $format = 'jpeg'
        try {
            if ($Encoder) {
                $tmpPng = "$OutBase.tmp.png"
                $scaled.Save($tmpPng, [System.Drawing.Imaging.ImageFormat]::Png)
                $out = "$OutBase.webp"
                if (Invoke-WebpEncode -Tool $Encoder.Name -EncoderPath $Encoder.Path -PngPath $tmpPng -OutPath $out -Quality $Quality) {
                    $format = 'webp'
                } else {
                    $out = "$OutBase.jpg"; $format = 'jpeg'
                    Save-Jpeg -Bitmap $scaled -Path $out -Quality $Quality
                }
                Remove-Item -LiteralPath $tmpPng -Force -ErrorAction SilentlyContinue
            } else {
                Save-Jpeg -Bitmap $scaled -Path $out -Quality $Quality
            }
        } finally { $scaled.Dispose() }
        if (-not (Test-Path -LiteralPath $out)) { return $null }
        return @{ Path = $out; Bytes = (Get-Item -LiteralPath $out).Length; Width = $w; Height = $h; Format = $format }
    } finally { $src.Dispose() }
}

<#
 入库图合计字节（`Get-MadeTotalBytes`）：判据面已抽到 `scripts/lib/mp-weixin-archive.ps1`（**空安全**）——
 为什么抽出去：`Measure-Object` 对空管道不产出对象，直接取 `.Sum` 在 StrictMode 下抛
 「在此对象上找不到属性"Sum"」，而这条路径要「真 git 仓库 + 真 PR + 干净工作树」才走得到 ⇒
 内联在这里就只能靠人眼；抽成库后由 `utils/mpWeixinArchiveSumBehavior.test.js` **真执行**它。
 本脚本在 main 流程里 dot-source 该库（见 `lib\mp-weixin-archive.ps1` 那行）。
#>

function Publish-ScreenshotArchive {
    param([int]$PrNumber, [string]$Module, [string]$ShotDir, $ProbeJson, [string]$RepoRoot)
    # 目标：docs/verification/<模块>/<号码>/<页名>-after.<ext>（#1770 起号码 = 票号，旧 PR 号目录同段仍被接受）；
    # 仅当「HEAD 在远端可达」且「工作树除本次改动外无其他改动」时才提交；其余情况只警告 + 给命令。
    #
    # ⚠️ **每个 return 必须是同一形状**（本函数所有分支都要带 `Rel`，取值用 @() 兜底）——
    #    #1209 血账：此前只有成功路径带 `Rel`，而调用点在 Set-StrictMode 下读 `$archive.Rel`，
    #    于是「未入库」分支（如"工作树有本次之外的改动"）**抛异常**，把贴 sha 绑定评论那一步打断：
    #    门明明通过却 exit 1，且证据评论根本没贴上。守护 C19 现在按「逐个 return 都带 Rel」拦它。
    $notes = [System.Collections.Generic.List[string]]::new()
    $cmdLines = [System.Collections.Generic.List[string]]::new()
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { return @{ Notes = @('未入库：找不到 gh CLI'); Commands = @(); Rel = @() } }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { return @{ Notes = @('未入库：找不到 git'); Commands = @(); Rel = @() } }

    $root = $RepoRoot
    $dirty = @()
    try { $dirty = @(& git -C $root status --porcelain 2>$null | Where-Object { $_ -and $_.Trim() }) } catch { }

    # #1770：归档不再依赖 PR —— 「这是不是 PR 的 head 分支 / PR 是否仍 OPEN」已无从问起（新流程是**证据齐了
    # 再开 PR**，跑门时 PR 往往还不存在）。改为问一句离线可答的：HEAD 在远端可达吗？归档提交要能推上去、
    # 评论里的 `- commit:` 才在 GitHub 上点得开（#1695）；不可达就跳过入库、只写说明 —— 门结论不变。
    $headSha = ''
    try { $headSha = (& git -C $root rev-parse HEAD 2>$null | Select-Object -First 1) } catch { }
    $headSha = ("$headSha").Trim()
    if (-not $headSha) { return @{ Notes = @('未入库：取不到 HEAD sha'); Commands = @(); Rel = @() } }
    if (-not (Test-ShaReachable -ProjectDir $root -Sha $headSha)) {
        return @{ Notes = @("未入库：HEAD $($headSha.Substring(0, 7)) 不在远端可达（还没推上去？）"); Commands = @(); Rel = @() }
    }
    $allowed = @("docs/verification/$Module/$PrNumber/")
    $blocking = @($dirty | Where-Object { $p = ($_ -replace '^..\s+', '').Trim('"'); -not ($allowed | Where-Object { $p.StartsWith($_) }) })
    if ($blocking.Count -gt 0) {
        return @{ Notes = @("未入库：工作树有本次之外的改动（$($blocking.Count) 项），为免夹带不做提交"); Commands = @(); Rel = @() }
    }

    $module = if ($Module) { $Module } else { 'mp-weixin' }
    $destDir = Join-Path $root ("docs/verification/$module/$PrNumber" -replace '/', '\')
    New-Item -ItemType Directory -Force -Path $destDir | Out-Null
    $encoder = Get-WebpEncoder
    if (-not $encoder) { $notes.Add('本机无 cwebp / ffmpeg / magick：按纪律退到 JPEG q75（System.Drawing）') }
    else { $notes.Add("WebP 编码器：$($encoder.Name)") }

    $shots = @($ProbeJson.shots | Where-Object { (Get-Prop $_ 'isPng') -and (Test-Path -LiteralPath $_.path) })
    # 超限处置顺序：先降质 → 再缩尺 → 再减图（减图写进评论）
    $plan = @(
        @{ MaxWidth = $script:ArchiveMaxWidth; Quality = 75 },
        @{ MaxWidth = $script:ArchiveMaxWidth; Quality = 55 },
        @{ MaxWidth = 560; Quality = 60 },
        @{ MaxWidth = 420; Quality = 55 }
    )
    $made = @()
    $skipped = @()
    foreach ($p in $plan) {
        if ($made.Count -ge [Math]::Min($shots.Count, $script:ArchiveMaxCount)) { break }
        Remove-Item -LiteralPath (Join-Path $destDir '*') -Force -ErrorAction SilentlyContinue
        $made = @()
        foreach ($s in $shots) {
            if ($made.Count -ge $script:ArchiveMaxCount) { break }
            $base = [System.IO.Path]::GetFileNameWithoutExtension($s.name)
            $outBase = Join-Path $destDir "$base-after"
            $r = Export-ArchiveImage -SourcePng $s.path -OutBase $outBase -MaxWidth $p.MaxWidth -Encoder $encoder -Quality $p.Quality
            if (-not $r) { continue }
            if ($r.Bytes -gt $script:ArchiveMaxBytes) { Remove-Item -LiteralPath $r.Path -Force -ErrorAction SilentlyContinue; continue }
            $made += $r
        }
        if ((Get-MadeTotalBytes $made) -le $script:ArchiveMaxTotal) { break }
    }
    # 仍超上限的图（按字节从大到小丢），并记下省了哪几页
    while ((Get-MadeTotalBytes $made) -gt $script:ArchiveMaxTotal -and $made.Count -gt 1) {
        $biggest = $made | Sort-Object -Property Bytes -Descending | Select-Object -First 1
        Remove-Item -LiteralPath $biggest.Path -Force -ErrorAction SilentlyContinue
        $skipped += ([System.IO.Path]::GetFileName($biggest.Path) + "（超合计上限）")
        $made = @($made | Where-Object { $_.Path -ne $biggest.Path })
    }
    if ($made.Count -eq 0) { return @{ Notes = @('未入库：压缩后没有任何图满足体积上限'); Commands = @(); Rel = @() } }
    if ($shots.Count -gt $script:ArchiveMaxCount) {
        $skipped += ($shots[$script:ArchiveMaxCount..($shots.Count - 1)] | ForEach-Object { $_.name })
    }

    $rel = $made | ForEach-Object { ('docs/verification/' + $module + '/' + $PrNumber + '/' + [System.IO.Path]::GetFileName($_.Path)) }
    foreach ($m in $made) {
        $notes.Add(("入库图：{0} {1}x{2} {3} {4}B" -f [System.IO.Path]::GetFileName($m.Path), $m.Width, $m.Height, $m.Format, $m.Bytes))
    }
    if ($skipped.Count -gt 0) { $notes.Add('省了这些页（体积/数量上限）：' + ($skipped -join '、')) }

    # 提交（只 add 本次入库的目录）
    try {
        & git -C $root add -- ("docs/verification/$module/$PrNumber") 2>$null | Out-Null
        & git -C $root commit -m "docs(verification): ② 微信开发者工具门截图（PR #$PrNumber）" --no-verify 2>&1 | Out-Null
        $sha2 = (& git -C $root rev-parse HEAD 2>$null | Select-Object -First 1)
        $notes.Add("已提交入库：$($sha2.Substring(0,7))（未用 [skip ci]：CI 会为新 sha 重跑，PR head 仍需挂在成功 run 上）")
    } catch {
        $notes.Add("入库提交失败：$_")
        $cmdLines.Add("git add docs/verification/$module/$PrNumber")
        $cmdLines.Add("git commit -m `"docs(verification): ② 门截图（PR #$PrNumber）`"")
    }

    # #1695：归档提交必须**先推上去**，评论里 `- commit:` 的 sha 才在 GitHub 上点得开（验收①）。
    # 推不动（分支保护 / 权限 / 离线）时**不假装成功**：如实记进入库说明 + 复现命令，
    # 由 Publish-GateComment 的绑定口径把评论降级成「绑远端 tip」或「如实标注未推送」（验收②）。
    $archBranch = (& git -C $root rev-parse --abbrev-ref HEAD 2>$null | Select-Object -First 1)
    $push = Push-ArchiveCommit -ProjectDir $root -Branch $archBranch
    if ($push.Pushed) {
        $notes.Add("已推送 $archBranch（#1695：sha 绑定评论只引远端可达的提交）")
    }
    else {
        $notes.Add("未推送 $archBranch：$($push.Reason) —— 本次归档 sha 仅本地（#1695）")
        $cmdLines.Add("git push origin HEAD:refs/heads/$archBranch")
    }
    return @{ Notes = $notes; Commands = $cmdLines; Rel = $rel }
}

# ---------- 主流程 ----------
$exitCode = 2
# publish 段墙钟（#1861 ④）：-BuildOnly 的结果行读它；-SkipBuild / 指纹跳过时保持 0（结果行写 n/a）。
$publishSeconds = 0
try {
    # -1) publish 之前的环境秒级预探（#1860，门计划 env-preprobe 步）：**否定性闸门** —— 只拦
    #     「.ide 端口文件在但试连不上」的楔死态，把「环境被楔死」的暴露点从 publish+open 之后
    #     （9–26 min/轮）提到秒级；其余一律放行且**不冒充**「会话已就绪」（就绪判据归就绪闸门）。
    #     试连预算取自计划 timeouts.preprobeMs（计划是预算唯一真源）；-Doctor 跳过（体检自带逐层报告）。
    #     判据库 scripts/lib/mp-weixin-preprobe.ps1（行为守护 utils/mpWeixinGatePreprobeBehavior.test.js）。
    if (-not $Doctor) {
        . (Join-Path $PSScriptRoot 'lib\mp-weixin-preprobe.ps1')
        $preRoots = @()
        if ("$IdeUserDataRoot") { $preRoots = @($IdeUserDataRoot) }
        $pre = Test-MpWeixinDevToolsSessionReachable -UserDataRoots $preRoots -ConnectTimeoutMs ([int]$script:GatePlan['timeouts']['preprobeMs'])
        Write-Log "pre-probe: $($pre.Observed)"
        Write-Host ">>> pre-probe（#1860 否定性预探）：$($pre.Observed)"
        # #2147：stale-port-file 是**放行**的第三态（文件在 + 连不上 + IDE 进程数 0）。它必须可见 ——
        # 否则「门为什么肯冷起点」下一次又要靠人翻日志推断；同时它**不冒充**「IDE 在跑」。
        if ($pre.StalePortFile) {
            Write-Host ">>> pre-probe：stale-port-file（IDE 进程数 = $($pre.IdeProcesses)）⇒ 按冷起点放行，交给后续 publish/open 拉起（本门不启停、不 kill IDE）" -ForegroundColor DarkYellow
        }
        if ($pre.Wedged) {
            Write-Host '[error] 开发者工具环境楔死（.ide 端口文件在但服务端口试连不上，且 IDE 进程数 ≥ 1）⇒ publish 之前判 env-blocked（exit 2），不烧构建与重试预算。' -ForegroundColor Red
            Write-Host '        处置（人工执行）：退出并重启微信开发者工具，或清点残留 IDE 进程后重开；本门绝不 kill IDE（ADR-0008 红线）。' -ForegroundColor Red
            Write-Log (New-MpWeixinEnvResultLine -Prefix $($script:GatePlan.resultLine.prefix) -Reason 'ide-port-unreachable' -Failures @("环境不可用：开发者工具 .ide 端口文件在但服务端口试连不上（$(@($pre.DeadPorts) -join ', '))"))
            # finally 的 close 在此路径会读死端口 ⇒ cli 自启 IDE 再关（#1860 三件套要消的正是自启族）；
            # 本门从未创建自动化会话、无残留可收 ⇒ 跳过收尾，处置交人工。
            $script:skipFinalClose = $true
            exit 2
        }
    }

    # HBuilderX 忙检测（单实例串行资源，ADR-0008 坑位段）：-SkipBuild 下不接主程序
    if (-not $SkipBuild) {
        . (Join-Path $PSScriptRoot 'lib\hx-busy.ps1')
        Start-GateStage -Name 'wait-hx'
        $hx = Wait-HxFree -CliExe $cliExe -TimeoutSeconds $HxWaitSeconds -NoWait:$HxNoWait -LogPath $logPath
    }
    Write-Host "proj  = $Project"
    Write-Host "dist  = $dist"
    Write-Host "cli   = $cliExe"
    Write-Host "wxcli = $devTools"
    Write-Host "log   = $logPath"

    if (-not $SkipBuild) {
        Start-GateStage -Name 'publish'
        # 0) **源 manifest 的 appid 前置断言（fail-closed）** —— 实测坑位（2026-09-12）：
        #    HBuilderX 在**首次导入 / 编译项目时会回写工作树的 manifest.json，把 mp-weixin.appid 置为 null**
        #    （实测 mtime 与 publish 启动同秒），于是产物 project.config.json 落成 touristappid，
        #    而 git 工作树里的 manifest 同时被改脏 —— 门会在「游客 appid」下静默跑通。
        #    ⚠️ 这条不止影响本门：manifest.json 是「运行时面」文件、正是 pr-evidence 的判据来源，
        #    任何 HBuilderX 门（④a / ④c / ②）在共享工作树里跑都可能静默改脏它 ⇒ 跑完请 `git status` 核对。
        #    故这里在动 CLI **之前**先校验源 manifest，不匹配即判门不过（而不是拿游客 appid 出个绿）。
        $manifestPath = Join-Path $Project 'manifest.json'
        $manifestRaw = Get-Content -LiteralPath $manifestPath -Raw
        $srcAppId = ''
        $mm = [regex]::Match($manifestRaw, '(?s)"mp-weixin"\s*:\s*\{.*?"appid"\s*:\s*"([^"]*)"')
        if ($mm.Success) { $srcAppId = $mm.Groups[1].Value }
        Write-Log "源 manifest mp-weixin.appid = '$srcAppId'（期望 '$AppId'）"
        Write-Host "源 manifest mp-weixin.appid = $srcAppId（期望 $AppId）"
        if ($AppId -and $srcAppId -ne $AppId) {
            if ($BuildOnly) {
                # -BuildOnly **跳过**这条断言（#1861 2026-10-28 裁定）：本模式不上传 ⇒ 产物落 touristappid 无害；
                # 而 HBuilderX 首次导入/编译会把工作树 manifest.json 的 mp-weixin.appid 置 null（正上方坑位），
                # 保留断言 ⇒ **第二次起必假红**。实读值照常进日志与结果行，不藏（下面的脏改提示负责提醒还原）。
                Write-Host ">>> -BuildOnly：源 manifest appid='$srcAppId' 与期望 '$AppId' 不一致 —— 本模式不做 appid 断言（不上传，产物落 touristappid 无害）" -ForegroundColor Yellow
                Write-Log "build-only: manifest-appid 不断言 manifest='$srcAppId' expected='$AppId'"
            } else {
                Write-Host "[error] 源 manifest 的 mp-weixin.appid 与期望不一致：manifest='$srcAppId' 期望='$AppId'" -ForegroundColor Red
                Write-Host '        这正是 HBuilderX 回写 manifest.json 把 appid 置 null 的痕迹（2026-09-12 实测）；' -ForegroundColor Red
                Write-Host '        产物 appid 只会取自 manifest（CLI 的 --appid 不改产物）⇒ 此时跑出来的门是「游客 appid 下的门」，不能算数。' -ForegroundColor Red
                Write-Host "        处置：先 `git checkout -- manifest.json` 还原真实 appid，再重跑本门。" -ForegroundColor Red
                Write-Log "$(Get-GateResultPrefix) errors=1 stage=manifest-appid manifest=$srcAppId expected=$AppId"
                exit 1
            }
        }

        # 1) 唤醒主程序 + 无人值守导入（HBuilderX 只认「已导入项目」，publish 未导入会报「项目不存在，请先导入」）
        foreach ($step in @(
                @{ Tag = 'cli-open'; Args = @('open') },
                @{ Tag = 'project-open'; Args = @('project', 'open', '--path', $Project) })) {
            $r = Invoke-Process -FilePath $cliExe -Arguments $step.Args -TimeoutSeconds $PublishTimeoutSeconds -Tag $step.Tag
            Write-Log "`n>>> $($step.Tag)`n$($r.Output)"
            Write-Host $r.Output
            if ($r.Output -match '与主程序的连接已中断|启动超时') {
                Write-Host '[error] HBuilderX CLI 连不上主程序：受限/沙箱会话会阻断 CLI↔主程序的本地 IPC。' -ForegroundColor Red
                Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=cli-ipc-blocked"
                exit 2
            }
            if ($r.TimedOut) {
                Write-Host "[error] $($step.Tag) 超时（$PublishTimeoutSeconds 秒）" -ForegroundColor Red
                Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=timeout"
                exit 2
            }
        }

        # 2) 构建 mp-weixin —— HBuilderX 构建成功后**自己**拉起开发者工具并打开项目（#883 实测）。
        #    $publishArgs 在指纹判定之前唯一构造（#2048），这里**不得再拼一次**：拼两次就是两份真相，
        #    第四道比对会退化成「比对用 A、执行用 B」的假保险。
        #    墙钟单独计（#1861 ④）：-BuildOnly 的结果行要报 publish 段秒数（参照值 161 s / 48 页、227 s / 59 页），
        #    不从 [STAGE] 行反推 —— 那些行在早退路径上可能缺席。
        #
        # 「按产物收流」（#2163）：基准时刻取在**发 publish 命令之前** ⇒ 上一次跑留下的产物必然判
        # stale（本票实测：中途终止的构建根本不写 dist，写完的才会新）。判据本体在
        # `lib/mp-weixin-publish-settle.ps1`，它复用 #1210 的产物合法性单点真源 —— 这里不复述第二份。
        . (Join-Path $PSScriptRoot 'lib\mp-weixin-publish-settle.ps1')
        $settleSince = Get-Date
        $pubSw = [System.Diagnostics.Stopwatch]::StartNew()
        $pub = Invoke-Process -FilePath $cliExe -Arguments $publishArgs -TimeoutSeconds $PublishTimeoutSeconds -Tag 'publish-mp-weixin' `
            -SettleDist $dist -SettleSince $settleSince -SettleQuietSeconds $PublishSettleQuietSeconds -SettleGraceSeconds $PublishSettleGraceSeconds
        $pubSw.Stop()
        $publishSeconds = [int]$pubSw.Elapsed.TotalSeconds
        Write-Log "`n>>> publish-mp-weixin`n$($pub.Output)"
        Write-Host $pub.Output
        if ($pub.Output -match '与主程序的连接已中断') {
            Write-Host '[error] HBuilderX CLI 连不上主程序（见日志）。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=cli-ipc-blocked"
            exit 2
        }
        if ($pub.TimedOut) {
            Write-Host "[error] publish 超时（$PublishTimeoutSeconds 秒）" -ForegroundColor Red
            # 读数把两种形态分开钉住：没产物（settle=product-missing files=0）与
            # 「产物齐了但 cli 不退」（旧实现只能等预算到点）处置完全不同，而旧的那行两种都只打
            # reason=timeout，现场读不出差别（票面 AC1 要的正是能被区分出来的两种形态）。
            $sr = if ($pub.SettleReason) { $pub.SettleReason } else { 'off' }
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=timeout settle=$sr files=$($pub.SettleFiles) budget=${PublishTimeoutSeconds}s"
            exit 2
        }
        if ($pub.Settled) {
            # 收流成立：cli 没退 ⇒ 输出流没 EOF，**没有文案可读**。依据换成产物判据（同 #1381 的
            # 教训：别让单条文案成为必要条件，采集面丢一条就把整扇门变恒红）。
            # 门结论依旧由下面的产物合法性 + appid 断言与探针跑出，不是「大概成功了」。
            # 红线：此处不终止任何进程（cli pid 记进日志，交下面的 close 段处置）。
            Write-Host "[settle] publish 按产物收流：$($pub.SettleDetail)（cli pid=$($pub.ChildPid) 未终止）" -ForegroundColor Green
            Write-Log "publish=settled-by-artifact cli_pid=$($pub.ChildPid) files=$($pub.SettleFiles) pages=$($pub.SettlePages) reason=$($pub.SettleReason) secs=$publishSeconds"
        }
        # 成败一律解析输出（CLI 退出码不可用）
        if (-not $pub.Settled -and $pub.Output -notmatch '导出微信小程序成功|编译成功') {
            Write-Host '[error] publish 未报「导出微信小程序成功」（见日志）：先解决构建失败，再跑 ②。' -ForegroundColor Red
            Write-Host '        提示：HBuilderX 按**项目名（目录名）**解析，同名目录会被误命中 ⇒ 请用唯一目录名（#883 坑位）。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=1 stage=publish reason=no-artifact"
            exit 1
        }
        if (-not (Test-Path -LiteralPath (Join-Path $dist 'app.json'))) {
            Write-Host "[error] 产物不完整（缺 app.json）：$dist" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=1 stage=publish reason=incomplete-artifact"
            exit 1
        }
    } else {
        Write-Log "`n>>> -SkipBuild：跳过 publish，直接用现有产物 $dist"
        Write-Host ">>> -SkipBuild：跳过 publish（用现有产物；门结论只对这份产物成立）" -ForegroundColor Yellow
        if (-not (Test-Path -LiteralPath (Join-Path $dist 'app.json'))) {
            Write-Host "[error] 没有可用产物（缺 $dist\app.json）：去掉 -SkipBuild 或先跑一次构建。" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=no-dist"
            exit 2
        }
    }

    # 3) **产物合法性 + appid 前置断言（#1210）** —— 判据面单点真源在 `scripts/lib/mp-weixin-product.ps1`。
    #    票面病根：旧实现用**正则**从 `project.config.json` 抠 appid ⇒ **非法 JSON 也能通过 appid 校验**；
    #    而这类坏产物会让开发者工具白屏 / `pageStack` 永不应答 / IDE 日志反复 `routeTo appLaunch timeout`，
    #    门最终只报一个**看起来像环境问题**的 `page-stack-not-answering`，把真正的病根藏起来。
    #    判据：必需文件齐（project.config.json / app.json / app.js / app.wxss）+ 两份 JSON 可解析
    #    （非法即打印**位置原文**）+ appid 由**解析后取值** + `app.json.pages` 每页有 `<page>.js`。
    #    失败 = **产物坏了 ⇒ 门未过（exit 1）**，与「环境不可用（exit 2）」严格分开。
    . (Join-Path $PSScriptRoot 'lib\mp-weixin-product.ps1')
    $product = Get-MpWeixinProductReport -Dist $dist
    Write-Log "产物合法性 = $($product.detail)"
    Write-Host "产物合法性 = $($product.detail)"
    if (-not $product.ok) {
        Write-Host "[error] 产物不合法（stage=$($product.stage)）：$($product.detail)" -ForegroundColor Red
        Write-Host '        这是**产物坏了**（构建被中断 / 写盘不完整），不是环境不可用 ⇒ 判门未过（exit 1）。' -ForegroundColor Red
        Write-Host '        处置：先 `Remove-Item -Recurse -Force <产物目录>` 干净重建（`npm run build:mp-weixin-check`），再重跑本门。' -ForegroundColor Red
        Write-Host '        排查提示：产物坏掉时开发者工具常见白屏 + `routeTo appLaunch timeout`，那不是「环境/锁/端口」问题。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=1 stage=$($product.stage) detail=$($product.detail)"
        exit 1
    }

    # 3.5) appid 断言 —— 实测：产物 appid 一律取自 manifest.json 的 mp-weixin.appid；
    #      manifest 缺该项时落成 touristappid（游客 appid）。CLI 的 --appid **不改**产物。
    $productAppId = [string]$product.appid
    Write-Log "产物 appid = '$productAppId'（期望 '$AppId'）"
    Write-Host "产物 appid = $productAppId（期望 $AppId）"
    if ($AppId -and $productAppId -ne $AppId) {
        if ($BuildOnly) {
            # -BuildOnly 下这条也**不断言**（与源 manifest 那条同源同因）：产物 appid 由 manifest 决定，
            # 回写后必然 touristappid；本模式不上传 ⇒ 无害，且产物结构判据（#1210）已照跑。
            Write-Host ">>> -BuildOnly：产物 appid='$productAppId' 与期望 '$AppId' 不一致 —— 本模式不做 appid 断言（不上传；产物结构判据已过）" -ForegroundColor Yellow
            Write-Log "build-only: product-appid 不断言 product='$productAppId' expected='$AppId'"
        } else {
            Write-Host "[error] 产物 appid 与期望不一致：产物=$productAppId 期望=$AppId" -ForegroundColor Red
            Write-Host '        appid 来源是 manifest.json 的 mp-weixin.appid（不是本脚本的 -AppId，也不是 CLI 的 --appid）；' -ForegroundColor Red
            Write-Host '        产物落成 touristappid = manifest 的 mp-weixin.appid 为 null/缺失（游客态）。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=1 stage=appid product=$productAppId expected=$AppId"
            exit 1
        }
    }

    # 指纹落盘（#1860）：只有**本轮真构建过**（走到这里时 -SkipBuild 仍为 false = 没被指纹/人工跳过）
    # 且产物合法性 / appid 两道 exit 1 断言都过了才写 —— 产物坏不写（下次必然重建）。
    # 写失败只警告，不影响门结论（fail 向重建：没写成 = 下次没有指纹 = 重建）。
    if (-not $SkipBuild) {
        try {
            $fpSave = Save-MpWeixinBuildFingerprint -Project $Project -Dist $dist -HBuilderXVersion $fpVersion -BuildCommand $buildCommandKey
            if ($fpSave.Saved) {
                Write-Log "build-fingerprint saved $($fpSave.Detail)"
                Write-Host ">>> 产物指纹已落盘（下次同树重跑命中即自动 -SkipBuild）"
            } else {
                Write-Log "build-fingerprint NOT saved reason=$($fpSave.Reason)（$($fpSave.Detail)）"
                Write-Host "[warn] 产物指纹未落盘（$($fpSave.Reason)）—— 下次重跑将重建（fail 向重建）" -ForegroundColor Yellow
            }
        } catch {
            Write-Host "[warn] 产物指纹落盘异常（不影响门结论，下次将重建）：$_" -ForegroundColor Yellow
            Write-Log "build-fingerprint save exception: $_"
        }
    }

    # ---------- -BuildOnly 的收口（#1861 第一段）----------
    # 切点 = 上面「产物合法性 + 指纹落盘」之后、会话段之前（`Start-GateStage -Name 'automation'` 是唯一分界）。
    # 走到这里说明构建面的三条判据都成立：publish 成功行已解析、产物合法性（#1210）已过、指纹已落盘。
    # 之后整段（pre-flight quit / close→open→auto 重试 / 就绪闸门 / 探针 / 截图入库 / 贴门评论）**不执行** ——
    # build-only 的全部价值就是「拿编译面判据而不付会话成本」，跑进去等于又变成 ②。
    # ⚠️ 两条必须继承的东西仍在：`finally` 的 `Invoke-DevToolsClose`（publish 尾段 5–6 ms 内**必自启**微信
    # 开发者工具，收掉那个自启实例靠的就是这条 finally —— 抽库/薄壳方案没有它）与 `Release-HxLock`（同 finally）。
    # `exit` 位于 try 内时会照常执行 finally（2026-10-28 实测），故这里用 exit 而不写绕过 finally 的收尾。
    if ($BuildOnly) {
        # build= 只报**事实**：publish（真构建）/ skipped(fingerprint)（指纹命中，见 #1860）/
        # skipped(explicit)（人显式 -SkipBuild ⇒ 只查产物，不冒充构建过）。
        $buildMode = if (-not $SkipBuild) { 'publish' } elseif ($script:buildSkippedByFingerprint) { 'skipped(fingerprint)' } else { 'skipped(explicit)' }
        $buildOnlyValues = [ordered]@{
            appid = "$productAppId"
            pages = "$($product.pages)"
            product = 'ok'
            build = $buildMode
            elapsed = $(if (-not $SkipBuild) { "${publishSeconds}s" } else { 'n/a' })
            log = $logRelative
        }
        $buildOnlyLine = "$($script:GatePlan.resultLine.prefix) " + (Format-GateMetricLine -Plan $script:GatePlan -Values $buildOnlyValues)
        Write-Log "`n$buildOnlyLine"
        # 阶段收尾走幂等入口：真跑过 HBuilderX 段（wait-hx / publish）时有未闭段可收；-SkipBuild 下从未 Start
        # ⇒ 幂等入口自己判出「无未闭段」不动作，不靠调用点猜（那正是它会抛 exit 2 的地方）。
        Stop-GateStageIfOpen
        Write-Host ''
        Write-Host "✅ mp-weixin build-only 通过（构建面）：$buildOnlyLine" -ForegroundColor Green
        Write-Host '   非等价声明：build-only 只证明「publish 成功 + 产物合法」，**不是** ② 的替代 ——' -ForegroundColor Yellow
        Write-Host '   console error / exception / pageStack / 截图一概未取证（ADR-0043 决策 3 不动：建成它不构成降频 ② 的理由）。' -ForegroundColor Yellow
        if ($PostToPr -gt 0) {
            Write-Host '   注意：-BuildOnly 不产出 ② 门证据块（-PostToIssue 在本模式下被忽略）；本模式也不入库（-Archive 同样不生效）。' -ForegroundColor Yellow
            Write-Log 'build-only: -PostToIssue / -Archive 忽略（本模式不产出门证据、不入库）'
        }
        exit 0
    }

    # 4) 清残留自动化会话 + 4.5) 重开项目窗口 + 5) 开自动化端口 —— **当作一个可重试的单元**（坑位 6）：
    #    2026-09-23 实测：冷起点 / IDE 重启窗口下，第一次 `cli.bat auto` 会「回显 √ auto 但端口 180s 不起」
    #    或「直接挂住 >180s」；**同一条命令**等 IDE 稳定后重跑 1–8s 即起。故整段
    #    `close → open --project <dist> → auto → 端口` 重试 attempts 次（次数/间隔取自门计划的预算面）。
    #    ⚠️ **不得只重试 auto**：实测只重跑 auto 会绑到一个**没加载项目的新 IDE 实例**上 —— 端口通了但
    #    `Tool.getInfo` 缺 SDKVersion、`App.getPageStack` 永不应答（门会以为会话就绪而实际什么都没有）。
    #    判据始终是协议级（SDKVersion + pageStack，见就绪闸门），不是「端口出现在 Get-NetTCPConnection 里」。
    # 0) Pre-flight（#1634，重试单元**之前**跑一次，幂等）：quit 清掉残留/堆叠的 IDE 实例 —— 脚本启动（IPC 链）
    #    下残留实例会让 open 把项目窗口叠成白屏（连菜单/Console 都没有；#1634 票面形态）。-Doctor 不跑
    #    （体检不操控 IDE）；quit/轮询失败只 [warn]，不判 exit 2，红线是不得用 Stop-Process 替代 quit。
    Start-GateStage -Name 'automation'
    if (-not $Doctor) { Invoke-DevToolsPreflightQuit }
    $attempts = [Math]::Max(1, [int]$script:GatePlan['timeouts']['autoAttempts'])
    $retryDelaySeconds = [Math]::Max(0, [int]$script:GatePlan['timeouts']['autoRetryDelay'])
    $listening = $false
    $usingAppId = ''
    $lastReason = 'port-not-listening'
    for ($attempt = 1; $attempt -le $attempts; $attempt++) {
        if ($attempt -gt 1) {
            Write-Host ">>> 整段重试第 $attempt/$attempts 次：先等 ${retryDelaySeconds}s 让开发者工具把上一次的 IDE 实例收干净（坑位 6）" -ForegroundColor Yellow
            Write-Log "`n>>> 整段重试第 $attempt/$attempts 次（等 ${retryDelaySeconds}s 后重跑 close → open → auto）；上一轮 reason=$lastReason"
            Start-Sleep -Seconds $retryDelaySeconds
        }

        # 4) 清残留自动化会话（坑位 1：不 close 会撞 pageStack 空 ⇒ page 级 API 全废且报误导错）。
        #    重试要能再来一次 ⇒ 每轮先复位幂等标志；finally 那次 close 的「不重复收尾」语义不受影响
        #    （本步 close 会把标志重新置 true）。
        $script:closed = $false
        Invoke-DevToolsClose

        # 4.5) **重新打开项目窗口**（时序坑位 4，2026-09-12 复测钉死的缺失步骤，勿删）：
        #      `cli.bat close` 把项目窗口一起关掉，而 `cli.bat auto` **不会重开** ⇒ 少了这一步 pageStack 恒空。
        #      故顺序写死 `close → open --project <dist> → auto`（守护测试 C12），本步为**显式步骤并写进日志**。
        #      `-SkipBuild` 下同样定位构建产物目录：$dist 在上面两个分支里都已做过存在性校验（缺 app.json 即 exit 2）。
        #      判据仍是输出/产物：本步只在**超时**时判环境不可用（窗口没重开就没法继续），其余一律记日志、
        #      由探针的 pageStack 前置断言给出真正的门结论（开发者工具 cli.bat 的输出与退出码不作门结论）。
        $openStep = Get-GateStep -Plan $script:GatePlan -Id 'devtools-open'
        $open = Invoke-Process -FilePath $devTools -Arguments $openStep.argv `
            -TimeoutSeconds $openStep.timeoutSeconds -Tag $openStep.tag
        Write-Log "`n>>> $($openStep.tag)（close 之后 auto 之前重开项目窗口；缺这步 pageStack 恒空）`n$($open.Output)"
        Write-Host $open.Output
        # 体检行必须记在**各自那一步**上，不能攒到就绪闸门再记：早期 exit（超时 / 端口没起来）会让报告缺层，
        # 而报告缺层恰恰发生在最需要它的时候（-Doctor 首跑实测抓到过这个形态）。
        if ($Doctor) {
            Add-DoctorRow -Layer 'L5 项目窗口' -Verdict $(if ($open.TimedOut) { 'FAIL' } else { 'OK' }) -Detail ("第 $attempt/$attempts 次尝试：close → open 已执行；open 回显 = " + (($open.Output -replace '\s+', ' ').Trim()))
        }
        if ($open.TimedOut) {
            $lastReason = 'devtools-open-timeout'
            if ($attempt -lt $attempts) { continue }
            Write-Host "[error] cli.bat open 超时（$OpenTimeoutSeconds 秒）：项目窗口未重开 ⇒ auto 不会替你重开 ⇒ pageStack 必为空格。" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=devtools-open-timeout"
            exit 2
        }

        # 5) 开自动化端口（无人值守，不需要人在 GUI 里点任何开关）
        #    ⚠️ 2026-09-12 实测最要紧的一条：auto **必须跑完**（它会派生子进程去起自动化服务；
        #    中途 kill 掉 auto 会让端口永远不监听），且跑完后端口是**延迟出现**的 ⇒ 之后必须轮询等待。
        $autoStep = Get-GateStep -Plan $script:GatePlan -Id 'devtools-auto'
        $auto = Invoke-Process -FilePath $devTools -Arguments $autoStep.argv `
            -TimeoutSeconds $autoStep.timeoutSeconds -Tag $autoStep.tag
        Write-Log "`n>>> $($autoStep.tag)`n$($auto.Output)"
        Write-Host $auto.Output
        if ($auto.TimedOut) {
            $lastReason = 'auto-timeout'
            if ($Doctor) { Add-DoctorRow -Layer 'L6 自动化端口' -Verdict 'FAIL' -Detail "第 $attempt/$attempts 次尝试：cli.bat auto 超时（$AutoTimeoutSeconds 秒）：自动化端口没开起来（残留会话多时见过此形态）" }
            if ($attempt -lt $attempts) { continue }
            Write-Host "[error] cli.bat auto 超时（$AutoTimeoutSeconds 秒，整段重试 $attempts 次用尽）" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=auto-timeout"
            exit 2
        }
        $usingAppId = ''
        $ma = [regex]::Match($auto.Output, 'Using AppID:\s*(\S+)')
        if ($ma.Success) { $usingAppId = $ma.Groups[1].Value }
        Write-Log "cli auto 回显 AppID = '$usingAppId'"
        Write-Host "cli auto 回显 AppID = $usingAppId"
        if ($AppId -and $usingAppId -and $usingAppId -ne $AppId) {
            Write-Host "[error] cli.bat auto 使用的 AppID（$usingAppId）与期望（$AppId）不一致。" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=1 stage=auto-appid using=$usingAppId expected=$AppId"
            exit 1
        }

        # 端口延迟出现：轮询到 listening（预算取自门计划的 port-listening 步骤；每轮整段尝试各有一份本预算）
        $portStep = Get-GateStep -Plan $script:GatePlan -Id 'port-listening'
        $listening = $false
        for ($i = 0; $i -lt $portStep.waitSeconds; $i++) {
            if (@(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue).Count -gt 0) { $listening = $true; break }
            Start-Sleep -Seconds 1
        }
        $addrs = ''
        if ($listening) {
            $addrs = (@(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) |
                ForEach-Object { "$($_.LocalAddress):$($_.LocalPort)" }) -join ','
        }
        Write-Log "端口 $Port listening=$listening addrs=$addrs（第 $attempt/$attempts 次尝试）"
        Write-Host "端口 $Port listening=$listening addrs=$addrs（第 $attempt/$attempts 次尝试）"
        if ($Doctor) {
            Add-DoctorRow -Layer 'L6 自动化端口' -Verdict $(if ($listening) { 'OK' } else { 'FAIL' }) -Detail "第 $attempt/$attempts 次尝试：port $Port listening=$listening（等 ${PortWaitSeconds}s）addrs=$addrs；auto 回显 Using AppID='$usingAppId'"
        }
        if ($listening) { break }
        $lastReason = 'port-not-listening'
    }
    if (-not $listening) {
        Write-Host "[error] 自动化端口 $Port 未监听：cli.bat auto 未真正生效（整段 close → open → auto 重试 $attempts 次用尽）。" -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=port-not-listening"
        exit 2
    }

    # 6) 探针：pageStack 前置断言 + 入口页/被改页导航 + console/exception + 截图
    #    先确保 miniprogram-automator 可用：装到**项目外**的临时根（--no-save + --ignore-scripts），
    #    不动仓库的 package.json / package-lock.json；探针用 --require-root 指过去解析。
    #    $autoRoot 在取计划时已算好（计划里的 --require-root 用它），此处只装配。
    $autoModule = Join-Path $autoRoot 'node_modules\miniprogram-automator\package.json'
    if (-not (Test-Path -LiteralPath $autoModule)) {
        $npmCmd = (Get-Command npm.cmd -ErrorAction SilentlyContinue | Select-Object -First 1).Source
        if (-not $npmCmd) { $npmCmd = (Get-Command npm -ErrorAction SilentlyContinue | Select-Object -First 1).Source }
        if (-not $npmCmd) {
            Write-Host '[error] 找不到 npm：无法准备 miniprogram-automator。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=npm-not-found"
            exit 2
        }
        New-Item -ItemType Directory -Force -Path $autoRoot | Out-Null
        $npmStep = Invoke-Process -FilePath $npmCmd -WorkingDirectory $autoRoot `
            -Arguments @('i', '--no-save', '--ignore-scripts', '--no-audit', '--no-fund', '--loglevel=error', 'miniprogram-automator@0.12.1') `
            -TimeoutSeconds 600 -Tag 'npm-automator'
        Write-Log "`n>>> npm-automator（$autoRoot）`n$($npmStep.Output)"
        Write-Host $npmStep.Output
        if (-not (Test-Path -LiteralPath $autoModule)) {
            Write-Host '[error] miniprogram-automator 装不上（见日志）：探针无法连自动化端口。' -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=automator-install-failed"
            exit 2
        }
    }
    Write-Log "automator require-root = $autoRoot"

    # 4.9) **就绪闸门（2026-09-13 实测新增；缺它 ② 时好时坏 —— 见脚本头坑位 5）**：
    #      端口 listening ≠ 会话可用。实测三个里程碑：端口 t≈1s → Tool.getInfo 带 SDKVersion t≈1–3s
    #      → App.getPageStack 开始应答 t≈24–34s。`automator.connect()` 内部的 `checkVersion()` 落在
    #      第二个里程碑之前就抛 `Cannot read properties of undefined (reading 'split')`，而探针原来的
    #      30s 超时又正落在 24–34s 中间 ⇒ 红绿不定。故这里先用裸 ws 等齐两个里程碑（只读、用完即关；
    #      实测**不影响**随后的 automator 会话），超预算才判环境不可用，且 reason 会明确写出缺的是
    #      哪个里程碑 —— 不再笼统说成「端口连不上」。
    if (-not (Test-Path -LiteralPath $readyPath)) {
        Write-Host "[error] 缺就绪闸门脚本：$readyPath" -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=no-ready-gate"
        exit 2
    }
    # 命令、超时、wait 预算一律取自门计划（计划里写明判据：SDKVersion + App.getPageStack）
    $readyStep = Get-GateStep -Plan $script:GatePlan -Id 'ready-gate'
    $ready = Invoke-Process -FilePath $nodeExe -Arguments $readyStep.argv -TimeoutSeconds $readyStep.timeoutSeconds -Tag $readyStep.tag
    Write-Log "`n>>> $($readyStep.tag)（等端口 + SDKVersion + pageStack 三个里程碑）`n$($ready.Output)"
    Write-Host $ready.Output
    $readyJson = $null
    foreach ($line in ($ready.Output -split "`r?`n")) {
        if ($line -match '^\s*MP_WEIXIN_READY\s+(\{.*\})\s*$') {
            try { $readyJson = ($Matches[1] | ConvertFrom-Json) } catch { $readyJson = $null }
        }
    }
    if (-not $readyJson) {
        Write-Host '[error] 就绪闸门未输出可解析的 MP_WEIXIN_READY 结果行（见日志）。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=no-ready-result"
        exit 2
    }
    $readyElapsed = Get-Prop $readyJson 'elapsedMs'
    if ($null -eq $readyElapsed) { $readyElapsed = 0 }
    $readySeconds = [int]([double]$readyElapsed / 1000)
    $sdkVersion = "$(Get-Prop $readyJson 'sdkVersion')"
    $ideVersion = "$(Get-Prop $readyJson 'ideVersion')"
    $readyOk = ((Get-Prop $readyJson 'ok') -eq $true)
    if ($Doctor) {
        Add-DoctorRow -Layer 'L7 会话就绪' -Verdict $(if ($readyOk) { 'OK' } else { 'FAIL' }) -Detail ("端口 t=$(Get-Prop $readyJson 'portReadyAt')s、SDKVersion t=$(Get-Prop $readyJson 'sdkReadyAt')s、pageStack t=$(Get-Prop $readyJson 'pageStackAt')s；sdk=$sdkVersion ide=$ideVersion pageStack=$(Get-Prop $readyJson 'pageStack')")
        Write-Log "doctor ready-gate: ok=$readyOk reason=$(Get-Prop $readyJson 'reason') detail=$(Get-Prop $readyJson 'detail')"
        exit (Get-DoctorExitCode)
    }
    if (-not $readyOk) {
        Write-Host "[error] 自动化会话未就绪（reason=$(Get-Prop $readyJson 'reason')）：$(Get-Prop $readyJson 'detail')" -ForegroundColor Red
        Write-Host '        注意：端口是通的 —— 这不是「端口连不上」，是会话还没到可用状态（坑位 5）。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=automation-not-ready detail=$(Get-Prop $readyJson 'detail')"
        exit 2
    }

    Start-GateStage -Name 'auto-probe'
    $probeStep = Get-GateStep -Plan $script:GatePlan -Id 'automator-probe'
    $probeArgs = $probeStep.argv
    $probeTimeoutSec = $probeStep.timeoutSeconds
    # ---------- 任意页直达探针（#1682 · ADR-0039 拆分步骤 2）：取证面从入口页扩到被改页逐页 ----------
    # -Pages 三态：'auto'（缺省）= 入口页 + 本 PR 改动页（git diff 命中 pages/**/*.uvue）；
    # 'none' = 关闭直达、走旧逐页导航组（保留原 SKIP 面）；其余 = 逗号分隔页清单。
    # 直达机制 = 首项临时换页 + plain 冷启动（ADR-0038 决策 1 已证的唯一可靠机制）；探针 --expect-page
    # 模式**不调用任何导航 API**，页身份判据 = 落地页 pageStack[0] 等于目标路由。
    $directLines = @()
    $directList = @()
    $distAppJson = Join-Path $Dist 'app.json'
    # 入口漂移自愈（#1690 run14 实测教训）：上一次直达循环被强杀 ⇒ finally 未跑 ⇒ 产物 app.json 首项
    # 停在上次换的页；本次会把孤儿页当入口静默取证（index 漏测还判 ✅）。以源 pages.json 首项为真源，
    # 漂移且该页在产物里 ⇒ 换回（复用 Switch 的键序轮转 + 读回/唯一性复验纪律）；源首项不在产物
    # （产物过期）⇒ 不动，门结论按实际入口如实记录。
    if ((Test-Path -LiteralPath $distAppJson) -and ($Pages -ne 'none')) {
        $srcPagesJson = Join-Path $Project 'pages.json'
        if (Test-Path -LiteralPath $srcPagesJson) {
            try {
                $srcFirst = "$(@(((Get-Content -LiteralPath $srcPagesJson -Raw) | ConvertFrom-Json).pages)[0].path)"
                $curFirst = "$(@(((Get-Content -LiteralPath $distAppJson -Raw) | ConvertFrom-Json).pages)[0])"
                if ($srcFirst -and $curFirst -and ($curFirst -ne $srcFirst)) {
                    $heal = Switch-DistAppJsonFirstPage -Path $distAppJson -TargetRoute $srcFirst
                    if ($null -eq $heal) {
                        Write-Host ">>> 入口漂移自愈（#1690）：产物 app.json 首项 $curFirst ⇒ $srcFirst（上次强杀遗留换页）" -ForegroundColor Yellow
                        Write-Log "入口漂移自愈：$curFirst ⇒ $srcFirst"
                    } else {
                        Write-Host "[warn] 入口漂移自愈未成（$heal）：产物入口仍为 $curFirst，门按实际入口记录" -ForegroundColor Yellow
                        Write-Log "入口漂移自愈未成：$heal（入口保持 $curFirst）"
                    }
                }
            } catch { Write-Host "[warn] 入口漂移检测读页清单失败（跳过自愈）：$_" -ForegroundColor Yellow }
        }
    }
    $entryRoute = ''
    # 覆盖判据读数（#2101）：只有 auto 模式有「改动页覆盖」语义。**必须在分支外初始化** —— 它只在
    # `$Pages -ne 'none'` 且构建产物存在时才算得出来，而结果行在更外层拼；StrictMode 下读未定义变量直接抛。
    $coverageField = ''
    if ($Pages -ne 'none' -and (Test-Path -LiteralPath $distAppJson)) {
        try { $entryRoute = "$(@(((Get-Content -LiteralPath $distAppJson -Raw) | ConvertFrom-Json).pages)[0])" } catch { $entryRoute = '' }
        $want = @()
        if ($Pages -eq 'auto') {
            # auto 判据 = 本 PR 改动页，真源在 scripts/lib/mp-weixin-pages.ps1（#1853，编码层双解耦见该库头）。
            # 「改了哪些页」与「该跑什么页 + 覆盖读数」两件事都在那个库里（#2101）：清单为空 ⇒ 退固定冒烟集，
            # 否则改 components/utils/App.uvue/pages.json 的 PR 只剩入口页一张 = 覆盖为零的绿（#2099）。
            # 门脚本这里只接结果 —— 三分支不复述（复述品会漂移，且改坏时文本断言照样绿）。
            $autoPages = Get-MpWeixinChangedUvuePages -Project $Project
            $resolution = Resolve-MpWeixinDirectCoverage -AutoPages $autoPages `
                -SmokePages @(Get-Prop (Get-Prop $script:GatePlan 'directPages') 'smokePages')
            $want = @($resolution.Pages)
            $coverageField = "$($resolution.Coverage)"
            if ([bool]$resolution.Degraded) {
                Write-Host "[warn] auto 页清单：git diff 不可用（$($autoPages.Reason)），直达降级为仅入口页（不冒充改动页已核）" -ForegroundColor Yellow
                Write-Log "auto-pages degraded reason=$($autoPages.Reason) coverage=$coverageField"
            } elseif ($coverageField -like 'smoke:*') {
                Write-Host ">>> auto 页清单为空（无 pages/**/*.uvue 改动）⇒ 跑固定冒烟集：$($want -join ',')（#2101）" -ForegroundColor Yellow
                Write-Log "auto-pages empty ⇒ coverage=$coverageField pages=$($want -join ',')"
            } else {
                Write-Log "auto-pages changed=$($want.Count) pages=$($want -join ',')"
            }
        } else {
            $want = @($Pages -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        }
        # 入口页打头 = 两相序的唯一真源（#1690 run12 终版）：入口页是 welcome 着陆页（index.uvue
        # #1478「已登录仍无条件 reLaunch dashboard」业务不变量）⇒ 必须先在**无会话**下取证（guest 相），
        # 再注入会话跑其余登录态页（auth 相）。run4/6/11 的入口页 miss 曾误诊为「IDE 会话还原」，
        # run12 证实真凶就是这条业务路由——殿后/键序轮转都救不了有会话的 index 冷启动。
        $directList = @(@($entryRoute) + $want | Where-Object { $_ } | Select-Object -Unique)
    }
    if ($directList.Count -gt 0) {
        Write-Host ">>> 直达探针（#1682，机制=cold-start-direct）：页清单 = $($directList -join ',')"
        Write-Log "`n>>> direct-pages-loop: list=$($directList -join ',')"
        $script:AppJsonBackup = @{ Path = $distAppJson; Text = (Get-Content -LiteralPath $distAppJson -Raw) }
        # 复用首轮整段重试与就绪闸门已按 id 取过的步骤对象（同一计划、同一参数真源；
        # 不再写第二处 -Id 字面量——守护 C12/C18 的注入判据按「首个取用点」定位，重复取用会让注入失配）
        $coldSteps = [ordered]@{ open = $openStep; auto = $autoStep; port = $portStep; ready = $readyStep }
        $directStart = Get-Date
        $agg = @{ errors = 0; exceptions = 0; logs = 0; shots = @(); steps = @(); failures = @(); anchorsTotal = 0; noise = 0 }
        $firstJson = $null
        $missReason = ''
        $missPages = @()
        # 页级读数（#1816）：每一页（达成 / 未达成）都记一条，组级 partial 与 navigation.pages 都从这里出
        $pageReports = @()
        $directEnv = $false
        $probeJson = $null
        # 会话两相（#1690 run12）：auth-inject 步取用点唯一 —— 守护 C12/C18 按「首个 -Id 字面量取用点」定位。
        # guest 相（入口页，清会话）/ auth 相（其余页，探针前注入）都在**当页冷启动之前**执行——
        # storage 跨 IDE 重启持久（实测），冷启动后 app 直接消费；冷启动后再注入救不回当页落地。
        $authStep = Get-GateStep -Plan $script:GatePlan -Id 'auth-inject'
        # 游客页族（#1817）：相判据的**声明侧**真源 = 计划里的 guestPhase.routes（不在这里手抄第二份清单）
        $guestPhase = Get-Prop $script:GatePlan['directPages'] 'guestPhase'
        $guestRoutes = @(Get-Prop $guestPhase 'routes')
        $authOk = $true
        $authField = 'off'
        if ($Auth -ne 'off') { $authField = 'none(no-auth-pages)' }
        foreach ($pg in $directList) {
            $leaf = ($pg -split '/')[-1]
            $isEntry = ($pg -eq $entryRoute)
            # 相判据（#1817）：入口页 ∪ 游客页族 ⇒ guest；其余 ⇒ auth。
            # 旧实现只看 $isEntry，login/register 这类游客页被注了 auth 会话后自己跳走（落地 dashboard），
            # 截图取到的是「别的页」且常撞 automator 超时 ⇒ 页级 reason 被记成 direct-page-unsupported，真因被掩掉。
            $isGuestPage = ($isEntry -or ($guestRoutes -contains $pg))
            # 每页（含入口页）一律 会话相→换页→冷启动→探针（#1690 实测修正）。
            # 入口页不换页不可靠：上一相窗口落在别的页 ⇒ 靠换页写盘（Switch 恒 Set-Content +
            # 键序轮转 ⇒ 字节必变 ⇒ 强制重编译）保证冷启动从目标页起。
            # 会话相（#1690 run12/run13 两相）：入口页 = guest（游客盖帽——welcome 页有会话必弹 dashboard；
            #   remove 不跨重启落盘、set 落盘 ⇒ 盖帽用 setStorageSync 写坏值）；
            # 其余页 = auth（探针前注入，幂等 existing 秒返；上一相注入失败/被 app 401 清掉 ⇒ 本页自愈重注）。
            if ($Auth -ne 'off') {
                if ($isGuestPage) {
                    $ag = Invoke-AuthInject -AuthStep $authStep -PageTag "guest($leaf)" -Mode 'guest'
                    $authOk = [bool]$ag.ok
                    if (-not $ag.ok) { Write-Host ">>> guest 相游客盖帽未成（$($ag.reason)）：入口页若被弹走如实记 miss，不假绿" -ForegroundColor Yellow }
                } else {
                    $ai = Invoke-AuthInject -AuthStep $authStep -PageTag "auth($leaf)" -Mode 'auth'
                    $authOk = [bool]$ai.ok
                    # #1863：auth 失败 detail 透传进结果行（可归因：HTTP 状态 / 网络错误 / 超时）。
                    # 结果行 token 是空格分隔的，detail 先压成无空白 slug；原样全文在日志 auth-inject 段。
                    $aiDetail = ("$($ai.detail)".Trim() -replace '\s+', '_')
                    if ($aiDetail.Length -gt 60) { $aiDetail = $aiDetail.Substring(0, 60) }
                    $authField = $(if ($ai.ok) { $ai.status } else { "none($($ai.reason)$(if ($aiDetail) { ':' + $aiDetail }))" })
                    Write-Host ">>> auth 相会话注入（#1690）$leaf：ok=$([bool]$ai.ok) status=$authField" -ForegroundColor $(if ($ai.ok) { 'Green' } else { 'Yellow' })
                    if (-not $ai.ok) {
                        # #1863 诚实降级（mp-weixin-auth.mjs 头注条款的调用方执行）：注入失败 ⇒ 该页没有
                        # 有效的登录态，冷启动拍到的只能是业务层 401 态（#1845 实证：storage 旧过期 token
                        # 复活、路由停在目标页 ⇒ 路由判据独自放行、截图实拍「登录已过期」Toast）⇒ 该页记
                        # SKIP、**不换页不冷启动不探针**，不得让 navigation=ok 全绿。guest 相不在此列：
                        # 游客页无会话才是合法落地态，由落地判据兜（#1817）。
                        Write-Log "direct-page $pg reached=skip(reason=auth-session-unavailable) (page not launched: auth inject failed reason=$($ai.reason) detail=$($ai.detail))"
                        $directLines += "direct-page $pg reached=skip(reason=auth-session-unavailable) errors=- exceptions=- logs=- shot=- elapsed=-"
                        $pageReports += [pscustomobject]@{ page = $pg; status = 'skip'; reason = 'auth-session-unavailable'; shot = ''; elapsedSeconds = 0 }
                        if (-not $missReason) { $missReason = 'auth-session-unavailable' }
                        $missPages += $pg
                        continue
                    }
                }
            }
            $sw = Switch-DistAppJsonFirstPage -Path $distAppJson -TargetRoute $pg
            if ($sw) {
                # 页不在构建产物 / 产物 JSON 坏 ⇒ 该页记 SKIP 不 launch（诚实降级），其余页照跑
                Write-Log "direct-page $pg reached=skip(reason=$sw) (page not launched)"
                $directLines += "direct-page $pg reached=skip(reason=$sw) errors=- exceptions=- logs=- shot=- elapsed=-"
                $pageReports += [pscustomobject]@{ page = $pg; status = 'skip'; reason = "$sw"; shot = ''; elapsedSeconds = 0 }
                if (-not $missReason) { $missReason = $sw }
                $missPages += $pg
                continue
            }
            $cs = Invoke-DirectColdStart -Steps $coldSteps -PageTag $leaf
            if (-not $cs.ok) {
                $agg.failures += "环境不可用：直达冷启动未成（page=$pg reason=$($cs.reason)）"
                $directEnv = $true
                break
            }
            $readySeconds = $cs.readySeconds
            $sdkVersion = $cs.sdk
            $ideVersion = $cs.ide
            $pr = Invoke-DirectProbe -ProbeStep $probeStep -DirectPages $script:GatePlan['directPages'] -Page $pg
            $pj = $pr.json
            if (-not $pj) {
                $agg.failures += "环境不可用：直达探针无结果行（page=$pg timedOut=$([bool]$pr.timedOut)）"
                $directEnv = $true
                break
            }
            if (@($pj.failures | Where-Object { $_ -match '^环境不可用：' }).Count -gt 0) {
                # 会话级环境失败（连不上 / 空栈 / 缺模块）：探针已判 exit 2 语义 ⇒ 整门 env-blocked。
                # **原条目**入 failures、不包写：尾部分类器按「环境不可用：」前缀锚派生具体 slug（#1655 单点真源），
                # 包一层会让 detail 失配、全部落进 probe-env 兜底。
                $agg.failures += @($pj.failures)
                $directEnv = $true
                break
            }
            if (-not $firstJson) { $firstJson = $pj }
            $agg.errors += [int]$pj.errorsTotal
            # 平台噪音读数（2026-10-29 F3）：与 errors 并列聚合——过滤过的条数必须一路上报到结果行
            $agg.noise += [int](Get-Prop $pj 'noiseTotal')
            $agg.exceptions += [int]$pj.exceptionsTotal
            $agg.logs += [int]$pj.logsTotal
            $agg.shots += @($pj.shots)
            $agg.steps += @($pj.steps)
            $agg.failures += @($pj.failures)
            # 锚点执行数聚合（#1919）：旧导航组路径的探针 JSON 无 anchors 字段 ⇒ Get-Prop 回 null → 0
            $agg.anchorsTotal += [int](Get-Prop (Get-Prop $pj 'anchors') 'total')
            $navOne = Get-Prop $pj 'navigation'
            $reached = ("$(Get-Prop $navOne 'status')" -eq 'ok')
            $pngOne = @($pj.shots | Where-Object { Get-Prop $_ 'isPng' })
            $shotLeaf = $(if ($pngOne.Count -gt 0) { Split-Path (@($pngOne)[-1]).path -Leaf } else { '' })
            $emOne = Get-Prop $pj 'elapsedMs'
            if ($null -eq $emOne) { $emOne = 0 }
            $line = "direct-page $pg reached=$([bool]$reached) errors=$($pj.errorsTotal) noise=$([int](Get-Prop $pj 'noiseTotal')) exceptions=$($pj.exceptionsTotal) logs=$($pj.logsTotal) shot=$shotLeaf elapsed=$([int]([double]$emOne / 1000))s"
            Write-Log $line
            $directLines += $line
            $pageReason = ''
            if (-not $reached) {
                $nr = "$(Get-Prop $navOne 'reason')"
                # 会话注入不可用时，miss 的真实原因是缺会话，不得冒充「直达机制不支持」（#1690）
                $mrAuth = ($Auth -ne 'off' -and -not $authOk)
                # 落地页（#1817）：探针在 steps[-1].currentPage 里如实记了冷启动落到的页；
                # 落到别处 ⇒ 页级 reason 点名落到哪（landed-elsewhere(<落地页>)），不再一律冒充 direct-page-unsupported。
                $landed = ''
                $stepList = @(Get-Prop $pj 'steps')
                if ($stepList.Count -gt 0) { $landed = "$(Get-Prop (@($stepList)[-1]) 'currentPage')" }
                $pageReason = $(if ($mrAuth) { 'auth-session-unavailable' } elseif ($landed -and $landed -ne $pg) { "landed-elsewhere($landed)" } elseif ($nr) { $nr } else { 'direct-page-unsupported' })
                if (-not $missReason) { $missReason = $pageReason }
                $missPages += $pg
            }
            # 页级读数（#1816）：达成与未达成**都**记——只记 miss 会让组级形态塌缩成 skip，吞掉已达成页的成功读数
            $pageReports += [pscustomobject]@{
                page = $pg
                status = $(if ($reached) { 'ok' } else { 'skip' })
                reason = $pageReason
                shot = $shotLeaf
                elapsedSeconds = [int]([double]$emOne / 1000)
            }
        }
        # 用毕还原（ADR-0038 决策 2）：env 早退路径也走这里；finally 里的 Restore-DistAppJson 靠哨兵幂等不再重复
        Restore-DistAppJson
        $directElapsed = [int]((Get-Date) - $directStart).TotalSeconds
        $totalLine = "direct-total pages=$($directList.Count) accepted=$(@($directLines).Count) elapsed=${directElapsed}s"
        Write-Log $totalLine
        $directLines += $totalLine
        if ($directEnv) {
            # env 情形**不再裸行早退**（#1682 初版写死 `reason=probe-env` 的简化行，违 #1655 三分线形状——
            # 无 failures= 原文、无 appid= metric 行）：失败条目已在 $agg.failures，随下面的聚合与
            # 通用装配段的分类器走完整 env 行（exit 2）。
            Write-Host "[error] 直达探针环境受阻（页：$($directList -join ',')）：$($agg.failures -join ' / ')" -ForegroundColor Red
        }
        if (-not $firstJson -and -not $directEnv) {
            # 列出的页零取证（不在构建产物 / #1863 起 auth 注入失败整组 SKIP 同理）⇒ **不得**当通过
            # （failures ⇒ exit 1 门未过）；missReason 写进失败原文，归因不靠猜。
            $zeroWhy = $(if ($missReason) { "页零取证（miss reason=$missReason；产物缺页或会话注入不可用）" } else { '列出的页都不在构建产物中' })
            $agg.failures += "断言未过：直达探针没有任何被接受的运行（$zeroWhy）"
        }
        $okPageList = @($pageReports | Where-Object { $_.status -eq 'ok' })
        $skipPageList = @($pageReports | Where-Object { $_.status -ne 'ok' })
        if ($directEnv) {
            $navAgg = [pscustomobject]@{ status = 'n/a'; reason = 'env-early-exit'; detail = 'env-early-exit'; at = 'direct-loop'; skippedAssertions = @(); skippedSteps = @(); pages = @($pageReports) }
        } elseif ($missPages.Count -eq 0 -and $firstJson) {
            $navAgg = [pscustomobject]@{ status = 'ok'; reason = ''; detail = 'cold-start-direct'; at = "direct-loop($($directList.Count)页)"; skippedAssertions = @(); skippedSteps = @(); pages = @($pageReports) }
        } elseif ($okPageList.Count -gt 0 -and $firstJson) {
            # 部分达成（#1816）：已达成页读数照常计入门结论；未达成页逐页记入 pages / skippedSteps，
            # **不得**塌缩成整组 skip（否则一页 miss 吞掉同组其它页的成功读数）
            $navAgg = [pscustomobject]@{ status = 'partial'; reason = $missReason; detail = 'cold-start-direct-partial'; at = "direct-loop(ok=$($okPageList.Count),skip=$($skipPageList.Count))"; skippedAssertions = @('allRoutesVisited'); skippedSteps = @($missPages); pages = @($pageReports) }
        } else {
            $navAgg = [pscustomobject]@{ status = 'skip'; reason = $missReason; detail = $missReason; at = 'direct-loop'; skippedAssertions = @('allRoutesVisited', 'screenshotsProduced'); skippedSteps = @($missPages); pages = @($pageReports) }
        }
        $probeJson = [pscustomobject]@{
            connected = (-not $directEnv)
            probeOk = ($agg.failures.Count -eq 0)
            pageStack = @($(if ($firstJson) { $firstJson.pageStack } else { @() }))
            entryPage = $(if ($firstJson) { $firstJson.entryPage } else { '' })
            errorsTotal = $agg.errors
            noiseTotal = $agg.noise
            exceptionsTotal = $agg.exceptions
            logsTotal = $agg.logs
            shots = @($agg.shots)
            steps = @($agg.steps)
            failures = @($agg.failures)
            navigation = $navAgg
            consoleTail = @()
            elapsedMs = ($directElapsed * 1000)
            # 锚点执行数（#1919）：随聚合透出，metricValues 据此写 optional asserts 字段
            anchorsTotal = $agg.anchorsTotal
        }
    } else {
    # 探针重试：开发者工具把项目/小程序拉起来是**异步**的（实测连接与 pageStack 常在第 2–3 次才就绪，
    # 首次报 "check if target project window is opened with automation enabled"）⇒ 必须重试而不是一次定生死。
    $probe = $null
    $probeJson = $null
    for ($attempt = 1; $attempt -le $probeStep.attempts; $attempt++) {
        Write-Log "`n>>> $($probeStep.tag)（第 $attempt/$($probeStep.attempts) 次）"
        $probe = Invoke-Process -FilePath $nodeExe -Arguments $probeArgs -TimeoutSeconds $probeTimeoutSec -Tag "$($probeStep.tag)#$attempt"
        Write-Log $probe.Output
        Write-Host $probe.Output
        if ($probe.TimedOut) {
            Write-Host "[error] 探针超时（$($probeStep.timeoutSeconds) 秒）：``page.waitFor(选择器)`` 与导航 API 会静默挂起仍禁（#883/#1917）；锚点断言自带 8s 步超时不糊成整门超时（#1919）。" -ForegroundColor Red
            Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=probe-timeout"
            exit 2
        }
        # **只看输出**：解析探针的 MP_WEIXIN_PROBE JSON（退出码不作判据）
        foreach ($line in ($probe.Output -split "`r?`n")) {
            if ($line -match '^\s*MP_WEIXIN_PROBE\s+(\{.*\})\s*$') {
                try { $probeJson = ($Matches[1] | ConvertFrom-Json) } catch { $probeJson = $null }
            }
        }
        if ($probeJson -and $probeJson.probeOk) { break }
        # ⚠️ 2026-09-13 修正（坑位 5）：旧版把「自动化端口连不上」也算进「重试无益」⇒ attempt≥2 就放弃，
        #    而那一类**恰恰是短暂**的（会话未就绪，实测 24–34s 才好）⇒ 门必红。现在只对**真正不会自愈**的
        #    环境问题（缺 automator 模块）提前放弃；连不上 / 未就绪一律按次数重试到底。
        if ($probeJson -and @($probeJson.failures | Where-Object { $_ -match '找不到 miniprogram-automator' }).Count -gt 0 -and $attempt -ge 2) {
            break  # 缺模块重试无益
        }
        if ($attempt -lt $probeStep.attempts) {
            Write-Host ">>> 探针未就绪（第 $attempt 次），$($probeStep.retryDelaySeconds) 秒后重试 —— 开发者工具拉起项目是异步的" -ForegroundColor Yellow
            Start-Sleep -Seconds $probeStep.retryDelaySeconds
        }
    }
    }

    if (-not $probeJson) {
        Write-Host '[error] 探针未输出可解析的 MP_WEIXIN_PROBE 结果行（见日志）。' -ForegroundColor Red
        Write-Log "$(Get-GateResultPrefix) errors=env-blocked reason=no-probe-result"
        exit 2
    }

    $stackLen = @($probeJson.pageStack).Count
    $shotPng = @($probeJson.shots | Where-Object { Get-Prop $_ 'isPng' })
    $shotNames = (@($shotPng | ForEach-Object { Split-Path $_.path -Leaf }) -join ',')
    # 导航降级（诚实降级，不许假绿）：探针把「逐页导航 + 每页截图」记成 SKIP ⇒ 结果行显式写出
    # navigation=skip(unsupported) + navigationSkipReason=navigation-api-unsupported，
    # **既不当通过也不当失败**；门只对可验证子集（connected / pageStack 非空 / 入口页在栈内 /
    # errorsTotal==0 && exceptionsTotal==0 / ≥1 张当前页截图）作结论，failures= 只在可验证子集不过时非空。
    $nav = Get-Prop $probeJson 'navigation'
    $navStatus = "$(Get-Prop $nav 'status')"
    $navSkip = ($navStatus -eq 'skip')
    $navPartial = ($navStatus -eq 'partial')
    $navReason = "$(Get-Prop $nav 'detail')"
    $navPageReports = @(Get-Prop $nav 'pages')
    $navOkCount = @($navPageReports | Where-Object { "$(Get-Prop $_ 'status')" -eq 'ok' }).Count
    $navSkipCount = @($navPageReports | Where-Object { "$(Get-Prop $_ 'status')" -ne 'ok' }).Count
    # ok = 逐页导航可用；partial = 直达部分达成（#1816，已达成页读数照常计入）；skip(unsupported) = 导航 API 不可用（诚实降级）；n/a = 没走到导航（环境早退）
    $navField = if ($navSkip) { "skip($(Get-Prop $nav 'reason'))" } elseif ($navPartial) { "partial(ok=$navOkCount,skip=$navSkipCount)" } elseif ($navStatus -eq 'ok') { 'ok' } else { 'n/a' }
    # 结果行：**按门计划声明的字段清单**拼（顺序与字段名都在计划里，Format-GateMetricLine 只取值）
    $metricValues = [ordered]@{
        appid = "$productAppId"
        pageStack = "$stackLen"
        entry = "$($probeJson.entryPage)"
        errorsTotal = "$($probeJson.errorsTotal)"
        exceptionsTotal = "$($probeJson.exceptionsTotal)"
        logsTotal = "$($probeJson.logsTotal)"
        shots = "$($shotPng.Count)"
        navigation = "$navField"
        ready = "${readySeconds}s"
        sdk = "$sdkVersion"
        ide = "$ideVersion"
        log = "$logRelative"
    }
    # 覆盖判据（#2101）：`changed:N` / `smoke:N` / `none`，**只有 auto 模式产出**（其余模式字段缺席，见计划注释）。
    # none 不是「没覆盖」而是「判据不可用」——必须说破，否则读的人会把入口页那张当成改动页已核。
    if ($coverageField) { $metricValues['coverage'] = "$coverageField" }
    if ($coverageField -eq 'none') {
        Write-Host '[warn] 覆盖判据不可用（coverage=none）：本次直达只有入口页，不冒充改动页已核' -ForegroundColor Yellow
        Write-Log 'coverage=none（git diff 不可用；刻意不退化为冒烟集）'
    }
    # 降级字段是**可选**的：计划里标了 optional，只在真的降级时出现（不降级就不写这个字段）
    if ($navSkip) { $metricValues['navigationSkipReason'] = "$navReason" }
    # 会话注入读数（#1690）：只在直达循环启用且未关闭注入时出现（计划里 optional）
    if ($directList.Count -gt 0 -and $Auth -ne 'off') { $metricValues['auth'] = "$authField" }
    # 锚点执行数（#1919）：只在有锚点执行时写——无锚点跑次该 token 缺席、结果行逐字节不变（计划里 optional）
    $anchorsTotal = [int](Get-Prop $probeJson 'anchorsTotal')
    if ($anchorsTotal -gt 0) { $metricValues['asserts'] = "$anchorsTotal" }
    # 平台噪音读数（2026-10-29 F3）：过滤过的条数必须上结果行——沉默的过滤等于「覆盖为零的绿」
    $noiseTotal = [int](Get-Prop $probeJson 'noiseTotal')
    if ($noiseTotal -gt 0) { $metricValues['noise'] = "$noiseTotal" }
    $metricLine = Format-GateMetricLine -Plan $script:GatePlan -Values $metricValues
    $resultLine = "$($script:GatePlan.resultLine.prefix) $metricLine"
    Write-Log "`n$resultLine"
    Stop-GateStageIfOpen
    Write-Log ("shots: " + (@($shotPng | ForEach-Object { "$($_.path) $(Get-Prop $_ 'width')x$(Get-Prop $_ 'height') $(Get-Prop $_ 'bytes')B" }) -join ' | '))
    if ($navSkip) {
        $skipLine = "navigation=SKIP reason=$navReason skippedAssertions=$((@(Get-Prop (Get-Prop $probeJson 'navigation') 'skippedAssertions') -join ',')) " +
            "skippedSteps=$((@(Get-Prop (Get-Prop $probeJson 'navigation') 'skippedSteps') -join ','))"
        Write-Log $skipLine
        Write-Host ''
        if ($navReason -eq 'navigation-api-unsupported') {
            Write-Host "⚠️  降级：逐页导航 + 每页截图 = SKIP（reason=$navReason）" -ForegroundColor Yellow
            Write-Host '    本机 miniprogram-automator@0.12.1 的 mp.reLaunch / mp.navigateTo 恒报 `Uncaught [object Object]`；' -ForegroundColor Yellow
            Write-Host '    门只对可验证子集（连通 + pageStack 非空 + 入口页在栈内 + 无 error/exception + ≥1 张当前页截图）作结论。' -ForegroundColor Yellow
            Write-Host '    未证实是否为版本组合问题（留给后续排查）；首跑请人工核对结果行的 navigation= 字段。' -ForegroundColor Yellow
        } else {
            Write-Host "⚠️  降级：任意页直达探针（#1682）有页未达成 = SKIP（reason=$navReason）" -ForegroundColor Yellow
            Write-Host '    机制 = 「构建产物 app.json 首项临时换页 + plain 冷启动」（ADR-0038 决策 1）；未达成的页**不得**当成已取证。' -ForegroundColor Yellow
            Write-Host '    门只对可验证子集（已达成页逐页 console/exception 读数 + 落地页判据）作结论；未取证页见日志的 direct-page 读数行。' -ForegroundColor Yellow
            Write-Host '    首跑请人工核对结果行的 navigation= 字段与 direct-total 时长行（0039 窗口主门成本校准）。' -ForegroundColor Yellow
        }
        Write-Host "    $skipLine" -ForegroundColor Yellow
    }
    if ($navPartial) {
        # 部分达成（#1816）：结果行已写 partial(ok=N,skip=M)；这里把页级读数与未达成页原因写进日志，
        # 让「一页 miss」不再表现为「整组没取证」（黄字提醒与 SKIP 分开，避免读成同一件事）
        $partialLine = "navigation=PARTIAL ok=$navOkCount skip=$navSkipCount " +
            "skippedSteps=$((@(Get-Prop (Get-Prop $probeJson 'navigation') 'skippedSteps') -join ','))"
        Write-Log $partialLine
        Write-Host ''
        Write-Host "⚠️  部分达成：直达探针 $navOkCount 页已取证 / $navSkipCount 页未达成 = PARTIAL（#1816）" -ForegroundColor Yellow
        Write-Host '    已达成页的 console/exception 读数与截图**照常计入**门结论，不得被未达成页整组塌缩。' -ForegroundColor Yellow
        Write-Host '    未达成页的页级原因见 navigation.pages 与日志的 direct-page 读数行；**不得**把 PARTIAL 当 PASS。' -ForegroundColor Yellow
        Write-Host "    $partialLine" -ForegroundColor Yellow
    }

    # 截图**入库**（把图提交进 git）是显式动作（`-Archive`），默认不发生。
    # A1（#2100）之前它是「传了号码就入库」，而 A1 之后为了拿证据块**必须**传号码 ⇒ 「正常取证」
    # 会顺手 commit + push，把 head sha 挪走、让正文里的 `跑于 <sha7>` 从 head 变成祖先。
    # 任何失败都只警告 + 给命令，**不让门因此失败**（维护者 2026-09-12 要求）。
    $archive = $null
    if ($Archive -and -not $NoArchive -and $PostToPr -gt 0) {
        try {
            $archive = Publish-ScreenshotArchive -PrNumber $PostToPr -Module $Module -ShotDir $logDir -ProbeJson $probeJson -RepoRoot $Project
        } catch {
            Write-Host "[warn] 截图入库失败（不影响门结论）：$_" -ForegroundColor Yellow
            $archive = @{ Notes = @("入库异常：$_"); Commands = @() }
        }
        foreach ($an in @($archive.Notes)) { Write-Host "   [archive] $an"; Write-Log "archive: $an" }
        if (@($archive.Commands).Count -gt 0) {
            Write-Host "   [archive] 请手动执行：" -ForegroundColor Yellow
            foreach ($ac in @($archive.Commands)) { Write-Host "     $ac" -ForegroundColor Yellow }
        }
    } elseif ($NoArchive) {
        Write-Host "   [archive] -NoArchive：跳过截图入库" -ForegroundColor Yellow
    } elseif ($PostToPr -gt 0) {
        Write-Host "   [archive] 未开 -Archive：截图不入库（A1：图按需贴 PR，需要存档时加 -Archive）" -ForegroundColor DarkGray
    }

    $failed = @($probeJson.failures)
    if ($probeJson.probeOk -and $failed.Count -eq 0) {
        Write-Host ''
        if ($navSkip) {
            $navGroupCn = $(if ($navReason -eq 'navigation-api-unsupported') { '逐页导航组' } else { '直达组（#1682）' })
            Write-Host "✅ ② 通过（可验证子集；$navGroupCn = SKIP）：$resultLine" -ForegroundColor Green
            Write-Host "   ⚠️ 未取证的部分：$navGroupCn = SKIP（reason=$navReason）—— 见上面的降级说明与 ADR-0008 ② 段。" -ForegroundColor Yellow
        } else {
            Write-Host "✅ ② 通过：$resultLine" -ForegroundColor Green
        }
        Write-Host "   截图：$shotNames（日志 $logPath）" -ForegroundColor Green
        Write-Host '   非等价声明：② ≠ ① 真机门，也 ≠ ④b 云打包门；content:// / 生物识别 / 第三方 SDK / 真机性能 挡不住。' -ForegroundColor Yellow
        Write-Host '   注意：不得由 agent 代填 ② 的「执行人」、不得由 agent 写「已通过」——本脚本只产出证据。' -ForegroundColor Yellow
        $exitCode = 0
    } else {
        Write-Host ''
        Write-Host "❌ ② 未过：$($failed.Count) 条断言未过 ——" -ForegroundColor Red
        $failed | Select-Object -First 20 | ForEach-Object { Write-Host "   $_" -ForegroundColor Red }
        # 用 Get-Prop 而不是 `$_.error`：Set-StrictMode 下访问**不存在的属性**会抛异常，
        # 而探针的 step 只在真出错时才带 error 字段 ⇒ 旧写法会把「❌ 门未过」的失败路径炸成
        # `errors=env-blocked reason=exception`（本机 2026-09-12 复测实测到：失败详情被异常顶掉）。
        @($probeJson.steps | Where-Object { Get-Prop $_ 'error' }) | Select-Object -First 10 |
            ForEach-Object { Write-Host "   [step] $($_.label) $($_.route)：$(Get-Prop $_ 'error')" -ForegroundColor Red }
        Write-Host "完整日志：$logPath" -ForegroundColor Yellow
        # 三分分类（#1655 / ADR-0036 处置二）：判据锚 = 探针失败条目前缀「环境不可用：」（StartsWith，
        # 不是裸 Contains —— 引用这句话的普通断言失败不得误判成环境受阻）；reason 派生也出自同一真源。
        # 行构造器保证两种结果行形状互斥：门未过行永不写 env-blocked，env 行 reason= 只允许小写连字符 slug。
        $envClass = Get-MpWeixinEnvClassification -Failures $failed
        if ($envClass.isEnv) {
            Write-Log (New-MpWeixinEnvResultLine -Prefix $($script:GatePlan.resultLine.prefix) -Reason $envClass.envReasons[0] -Failures $failed -MetricLine $metricLine)
        } else {
            # failures= 只在**可验证子集**不过时非空（导航降级不产生 failures，只记 SKIP）
            Write-Log (New-MpWeixinGateResultLine -Prefix $($script:GatePlan.resultLine.prefix) -Count $failed.Count -FailuresJoined ($failed -join ' / ') -MetricLine $metricLine)
        }
        $exitCode = if ($envClass.isEnv) { 2 } else { 1 }
    }
} catch {
    Write-Host "[error] ② 脚本异常：$_" -ForegroundColor Red
    # 前缀取自计划（#1861）：异常兜底此前**硬写** MP_WEIXIN_RESULT，-BuildOnly 下异常就会打成 ② 的 token，
    # 分流被破坏 ⇒ 改走 Get-GateResultPrefix（计划未建时它自己回落到默认前缀，不在这里再写一份）。
    Write-Log (New-MpWeixinEnvResultLine -Prefix (Get-GateResultPrefix) -Reason 'exception' -Failures @("环境不可用：脚本异常（$_）"))
    $exitCode = 2
} finally {
    # 任意页直达探针（#1682）：异常/早退路径也必须还原 dist app.json（幂等，主路径已还原则是空操作；
    # ADR-0038 决策 2「用毕从备份还原并复验」的 finally 兜底）
    Restore-DistAppJson
    # 必须在 finally 路径里关：否则自动化会话与端口会残留（下次 pageStack 空 ⇒ 误导弹错）
    Invoke-DevToolsClose
    # 释放 agent 互斥锁（-SkipBuild 下未取锁，Release-HxLock 是安全的空操作）
    if (Get-Command Release-HxLock -ErrorAction SilentlyContinue) { Release-HxLock }
    # -Doctor：无论从哪条路径退出都要把逐层体检报告打出来（早期 exit 也要能看到结论）
    if ($Doctor) { Write-DoctorReport }
    # -BuildOnly（#1861 裁定「跑完打印脏改提示」）：**每条退出路径**都提示，且只在真的可能回写过工作树时
    # 有意义 —— 这里不判「回写过没有」（那要跑 git、且会把「没脏」也当成结论），只把核对动作印给人。
    if ($BuildOnly) { Write-BuildOnlyWorktreeHint }
}

# P1：仅门通过时贴「sha 绑定」评论（门结果免手抄；gh 不可用/取不到 sha 只警告，不改门结论）
if ($exitCode -eq 0 -and $PostToPr -gt 0) {
    # ⚠️ 续行符一个都不能少：本调用此前在 `-ShotRelative … -ReproCommand …` 一行**漏了行尾反引号**，
    #    于是 `-ArchivedRel` / `-ArchiveNotes` 被解析成**一条新命令**（运行到贴评论后会 CommandNotFound），
    #    入库截图清单也就从来没进过评论。守护测试 C14 现在按「以 `-参数名` 单独起行」拦这类断链。
    #
    # ⚠️ #1209 两条纪律（守护 C19）：
    #    (1) 归档结果一律用 **Get-Prop** 取值 —— Set-StrictMode 下访问不存在的属性会抛异常，
    #        而归档函数「未入库」分支曾返回不带 `Rel` 的对象 ⇒ 这里抛异常 ⇒ **贴评论被打断**，
    #        门明明通过却 exit 1、证据评论也没贴上（真实事故，本会话 #1199 就是这么绕过去的）；
    #    (2) 整段贴评论包 try/catch：它是**报告层**，门结论在上面已经定了 ——
    #        崩了要**响亮地警告 + 记日志**，但不得改变退出码、更不得让证据悄悄消失（禁止静默例外）。
    try {
        Publish-GateComment -PrNumber $PostToPr -ResultLine $resultLine -LogRelative $logRelative `
            -ShotRelative ".ci-verify/$shotNames" -ReproCommand 'npm run build:mp-weixin-check' `
            -ArchivedRel @(Get-Prop $archive 'Rel') `
            -ArchiveNotes @(Get-Prop $archive 'Notes') `
            -NavSkipped ([bool]$navSkip) -NavReason $navReason `
            -DirectPageLines @($directLines)
    } catch {
        Write-Host "[warn] 贴 ② 门评论失败（门结论不受影响，退出码仍为 $exitCode）：$_" -ForegroundColor Yellow
        Write-Log "$(Get-GateResultPrefix) comment=FAILED exit=$exitCode detail=$_"
    }
}
exit $exitCode
