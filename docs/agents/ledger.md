# 血账登记簿（ledger）· canonical

> **用途**：本仓已知血账的**唯一清点处**。每条血账 = 现象 + 成因 + 复发的后果，附状态与证据指向。
> **裁定（2026-10-20 复盘，用户裁定）**：血账散在 ADR / verification / agents 各处的现状不满足「唯一清点处」纪律。本文件为 canonical 登记簿；各散布点原文保留、逐条回链本簿，不迁移正文。新增血账一律落此簿，散布点只留一行回链。
> **底稿**：`docs/design/mobile-workflow-friction-survey.md` §3 的 21 条（逐条转录，原文出处列指回调查表行号）。后续新增接续编号。
> **状态口径**：`open`＝机制仍在，无更强机制拦截；`fixed`＝机制已修，附证据指向；`superseded`＝由更强机制取代。
> **裁定日**：状态由每次盘点裁定，本版为 2026-10-20 首录。

---

## A. 移动端工具链血账（底稿 = 摩擦调查 §3，#1–#21）

| # | 简名 | 现象 → 成因 → 后果 | 状态 | 证据指向 | 原文出处 |
|---|------|-------------------|------|----------|----------|
| 1 | ③门宿主树假绿 | 点父段 worktree 里 `npm run test:unit` 报 `No tests found … 0 matches` 且 `--listTests` 空输出 exit 0 → jest-util `replacePathSepForGlob` 对点段前反斜杠不转换（jest 27.5.1），picomatch 把 `\.` 读成转义 `.` → 假绿装配冒充验收。对照实测：`D:\FL\wt-1144` 97 套件 vs `D:\FL\.wt-1144probe` 0 套件；闸门 #1185 教学闸门之外，宿主落点 `<userHome>/.qoder/worktree/…` 不可配置 | open | 点父段 0 套件 vs 无点父段 141 套件（dev-loop.md「三条原因的实测原文」第 1 条） | 调查表:48 |
| 2 | ②门历史坑位密集 | close/open/auto 全部回显成功但项目窗口从未打开（须人在 GUI 导入一次）→ 门回报不可信；就绪闸门要求 `App.getPageStack` 应答（t≈18s，两测 24s/34s）；两条门红被误报成「自动化端口连不上」；残留自动化服务被静默复用（换端口即恢复）；Stable 2.01.2510290 的 auto 会把 IDE 一起带走，换 2.02.2608070 一次通过 | open | ADR-0008 多轮修订（#883 起） | 调查表:49 |
| 3 | ②产物判据漏报 | 正则从 `project.config.json` 抠 appid，非法 JSON 也能过 → 坏产物被报成 exit 2 环境问题，白查一轮 | open | #1210 | 调查表:50 |
| 4 | compile-check 判据漏族 | `compile-check.ps1` 错误行判据漏整族诊断 → 编译不过的树打 `COMPILE_RESULT errors=0 clean=True` 且自陈「✅ 编译门通过」→ 假绿送达收口；同族盲区：④c 的 kotlinc 判据（原 `kotlin-all-check.ps1:64`，#2045 后住 `scripts/lib/gate-error-lines.ps1`）对「时间戳+error:」与「⛔error:」match=False | fixed（#2045，2026-10-27） | 判据抽 lib 并补 `Could not resolve` 族；**销账证据 = 机检**：`utils/gateErrorLinesBehavior.test.js` G0/G4（首跑 9 绿 2 红、修复后 11/11 绿），接线由 `utils/gateErrorLinesWiringContract.test.js` W1–W5 钉；④c 那两形未复现，作为已知边界写进 `lib/gate-error-lines.ps1` 头注；**第三形态 `lib/hx-errors.ps1` 同族仍缺 → 见本簿 #25** | 调查表:51 |
| 5 | kotlin-all 第二族假绿 | publish 段失败只认两条文案（`-1:cli:命令'publish app-android'不存在或缺少参数` 被放过）→ 编译根本没跑也过门；只看有没有 `.kt` 不看新鲜度 → 磁盘旧导出就算过 | fixed（判据 #1272/#1381；错题入册 #2045） | `scripts/lib/publish-freshness.ps1`（正向标记 + 联合判据 + 新鲜度/完成度实测量）；行为守护 `utils/kotlinAllStaleExportBehavior.test.js`（真执行）；④c kotlinc 族错题入册 `utils/gateErrorLinesBehavior.test.js` G5–G8（#2045，可反复重跑的销账证据） | 调查表:52 |
| 6 | Invoke-Process 超时失真 | 超时只看进程退出段 → 总耗时 41137 ms 且 `TimedOut=False` → 挂死被误判为成功；同款写法在 `mp-weixin-check.ps1` 与 `scripts/lib/hx-busy.ps1` 各一份 | open | 实测（#1285）；2026-10-20 复盘核实现存两份（`mp-weixin-check.ps1`、`process-capture.ps1`），收敛自三份但逐份未核 | 调查表:53 |
| 7 | dev:finish 截图对比假绿 | 步骤 7 只比 MD5、基线目录永远为空、有变化只 Write-Host 黄字永不 fail → 截图回归门恒绿 | fixed | 改为像素差 + 阈值 0.5%（#1139 修复，ADR-0008） | 调查表:54 |
| 8 | adb 解析三份漂移 | 三份解析实现各自漂移，内联那份漏 D 盘兜底 → 步骤 6 恒报「找不到 adb.exe」，而步骤 4/5 已真跑完（编译 ~5 min + 推送 162 s），7–9 永远到不了 → 坏的那份只在真机露出来 | open | #1027；同族教训已立「adb 解析的唯一真源」纪律 | 调查表:55 |
| 9 | 画面稳定判据被状态栏钉死 | 全帧 SHA256 相等判据被 MIUI「显示实时网速」钉死（1–2 秒变一次）→ 步骤 6 必然 420 秒超时、页面记 Skipped、7–9 跑不到 → 判据在该类设备上恒不可满足 | fixed | 改为 100px 横带采样差异像素占比 ≤0.5%（#1027 第二阻塞修复） | 调查表:56 |
| 10 | --compile 语义坑位 | 官方语义「仅编译代码」，真运行路径照抄了门的调用 → 从未真正请求过运行，「编译成功 → 已停止运行…」被误读成「运行失败」→ 追了整整一票 | open | 血账 #917；`hx-run.ps1` 真 rho 路径现已禁用 `-CompileOnly` 分支外溢 | 调查表:57 |
| 11 | 成本账反模式 | sha 无害前进就重跑整道门（#1443 为绑 `c3adc58` 重跑完整 ④c，其间只动一张 jpg；#1467 merge 只带两个 Go 域搬包，② 与 ④c 仍各重跑）→ 白付分钟级机器时间；② 失败后不带 `-SkipBuild` 重试 → 9–26 min/轮（已退役，#2002；现测 126–327 s，#1987） | open | 2026-10-03 立（dev-loop.md）；**② 失败轮半边已收敛（#1860，2026-10-24）**：产物指纹自动 `-SkipBuild` + publish 前秒级预探 + quit 防自启（判据库 `training-app/…/scripts/lib/mp-weixin-build-fingerprint.ps1` / `mp-weixin-preprobe.ps1`，ADR-0008 ② 段「白烧收敛三件套」）；sha 无害前进重跑的预估器半边仍 open | 调查表:58 |
| 12 | ③门全绿可恒真 | 手抄镜像守护改坏被测面照样绿（改坏 `format.uts` 的 `return '0s'`，`format.test.js` 照样 8 passed）→ 「跑绿」不能当验收；分类器第一版把 9 个手抄镜像误判成行为守护（20/78），修正后 13 行为 / 86 接线 | superseded | #1156 修；后被 ③ 拆腿（guards.md「行为守护才承重」）与 #1702 拆腿接手 | 调查表:59 |
| 13 | 真运行会话常驻锁目录 | 真运行会话常驻攥住项目目录 → `Rename-Item` 报 `being used by another process`、删/改名收尾被卡；HBuilderX 5.23 无 `project` 子命令，`cli project close` 不可用；持锁者不止 cli.exe（#1240 P2：close+quit 后仍 Access denied）→ 唯一非破坏解是发起下一次 launch 顶掉 | open | 2026-09-13 一天两踩；09-17 / 09-21 追记 | 调查表:60 |
| 14 | .ci-verify 随树消失 | `git worktree remove` 连带删掉 gitignored 的 `.ci-verify/`（被忽略的未跟踪文件不使树判 dirty）→ 判据输入（原始 screencap PNG / uiautomator XML / launch 与 logcat 日志）永久丢失 → 像素读数（27090/17286/9804）不再可复算，入库 JPEG 是重编码件非判据输入 | open | 2026-09-22 实测；`multi-agent-git.md`「删 worktree 之前必须把 gitignore 覆盖的证据产物拷出来」 | 调查表:61 |
| 15 | --delete-branch 静默不删 | `gh pr merge --delete-branch` 静默不删 → 14 个远端残留分支、13 个对应已 MERGED 的 PR，时间线无一条 `head_ref_deleted`；gh 2.97.0 `deleteRemoteBranch()` 包在 `if !m.merged` 内；分支被 worktree 占用时本地删除先失败 → 票关闭 ≠ 分支回收 | open | 2026-09-19 实测（release.md:13）；2026-10-20 复盘又见 #1718 族三残支，同一成因第四次复发 | 调查表:62 |
| 16 | squash subject 丢 (#N) | `gh pr merge --squash --subject` 抑制 `(#N)` 后缀 → gate 报「无法从 commit 主题解析 PR 号」exit 1 → cd.yml 生产 gate 无 CI 判据，生产构建全部被跳过（run 37090597016）→ 直发 production 的门失效 | fixed | P2 波 4e 当场修（release.md:14） | 调查表:63 |
| 17 | ci-summary 漏 needs | `ci-summary` 曾漏 `changes` 于 `needs` → changes 失败 ⇒ 子 job 全 skipped ⇒ FAILED 为空 ⇒ 汇总 success → ruleset 唯一必检 + cd.yml 生产 gate 唯一 CI 判据失效，零检查的 PR 可直发 production | fixed | 2026-09-28 修（ci.yml ci-summary job 注释） | 调查表:64 |
| 18 | 本机 golangci-lint 不可用 | v1.64.8 由 go 1.26.4 构建，面对 go 1.27.1 导出数据报 `cannot decode "internal/goarch": export data version 4 is greater than maximum supported version 2` → 本机四件套实为三条，lint 靠 CI 兜底 | open | checks.md「环境 A」 | 调查表:65 |
| 19 | pr-evidence 硬赢规则 | ① 逐文件低风险豁免是 per-file 的，顺手改一处 `scripts/*.ps1`（哪怕只为契约套件补一条 token 注册）就拿不到豁免；②「结论」栏引用的可核验产物必须写在该行**行内**——子条目、裸 `.ci-verify/*.png`、任意 http(s) 一律不认 → PR 结构返工 | open | 这是**设计裁定**不是 bug，记在簿上防「当 bug 改」误修 | 调查表:66 |
| 20 | 模板门行形状锁 | 生成器吐「多行块」而校验器按「每门一行」取门 → ①③④ 全判「缺该行」，移动端 PR 一开即红、逼人手工重排结构；为什么不抽共享「门规格表」：校验器内联不 checkout ⇒ 共享 lib 读不到 ⇒ 表只能内联副本 + 逐字相等锁 ⇒ 用锁把复述品包装成真源是假整合（#1403） | open | #1403；结构返工每 PR 一次 | 调查表:67 |
| 21 | Worktree 配置框二真源 | Qoder「本地任务的 Worktree 配置」框依赖宿主注入的私有环境变量，命令行/闸门/别的编辑器都拿不到 → 框值漂移不报错，只以「命令不存在」或「0 套件假绿」显形，排查要回读 `%APPDATA%\Qoder\logs` → 第二真源陷阱 | open | 已记账后果：worktree 里 `npm test` 报 `'jest' 不是内部或外部命令` | 调查表:68 |

---

## B. 2026-10-20 复盘新增（接续编号）

| # | 简名 | 现象 → 成因 → 后果 | 状态 | 证据指向 | 原文出处 |
|---|------|-------------------|------|----------|----------|
| 22 | 已关票分支未回收 | #1718 已关、master 已由 #1722 收编，但 `fix/1718-stub-orphans`（6 提交 229 行差异）+ 同族 4 条本地/远端分支仍挂 → 血账 #15 的成因在 2026-10-20 第四次复发 → 票关闭 ≠ 分支回收，缺「关票即清分支」机检 | fixed（本地） | 2026-10-20 复盘当日删净（tag `fix/1718-stub-orphans-archived` 归档 c02c279f；本地 2 条 + 远端 4 条删净，`fetch --prune` 后 `branch -a` 无 1718 残留）；机检闭环 `check-branch-residue.ps1` 自动触发点仍未接（调查 §5 候选） | 本簿首录 |
| 23 | 孤儿目录壳取证失联 | 5 个无 `.git` 的孤儿目录壳（wt-1484/1542/1554/1607/1713-verify）混在主树 untracked 清单里，无人认领；`wt-1713-verify` 装着 #1713 真机取证 25 文件（rl-1~rl-9 截图 + logcat + desktop，6,575,224 字节），而该票正文只以行内附件引用证据、未仓库化 → 判据输入濒临永久丢失（#14 同族）；`cleanup-orphan-worktrees.ps1` 目标列表（1514/1543/1570/1602/1603/1604）与磁盘现状脱节，脚本自身过期 | fixed（本地） | 2026-10-20 复盘当日抢救入库 `docs/verification/forum/1713/`（25 文件 + README，src=dst=25 验证）后删净 5 壳；脚本已加 STALE-GUARD（#1724 合并 c0153bda），targets 表重写待办在 #1723 | 本簿首录 |
| 24 | CD gate 把主题尾缀 (#N) 一律当 PR 号 | squash 主题合法地引用 issue 号（`(#1723)` 是 issue，PR 是 #1724）时，push 触发的 cd.yml gate `grep -oE '\(#[0-9]+\)$'` 解析出 1723 后直接 `gh pr view 1723` → GraphQL `Could not resolve to a PullRequest` exit 1 → 合并后生产部署 gate 失败、全链 skipped。#16 血账只要求「后缀必须存在」，没管「尾缀必须是 PR 而非 issue」——GitHub issue/PR 共享编号空间，先占号者赢 | open | 实测 2026-10-20 c0153bda，run 39426628397（`GraphQL: Could not resolve to a PullRequest with the number of 1723`）；应急通道 dispatch production run 39426988432 success 放行；修复候选 = gate 查询失败回退 `gh issue view` 判类型，或约定合并主题尾缀一律指 PR | 本簿首录 |
| 25 | ④ 第三形态判据仍缺 Could not resolve 族 | #2045 把 ④ 的两处判据抽进 lib 并补上 `Could not resolve`，但**第三形态** `scripts/lib/hx-errors.ps1` 的 `Get-HxErrorLines`（`hx-run.ps1` 日常「仅编译拿诊断」路径用，票面明写「只加样品、不搬家」）**仍缺同一族** → 门（④）现在拦得住、日常内循环照样漏 → 同一条假绿在读码上分不出远近：谁走哪条判据决定它瞎不瞎 → 后果是把 #4 的形态原样搬进 `hx:compile-only` 的输出判读 | fixed（#2166，2026-10-31） | 现测（2026-10-27，本票）：`lib/hx-errors.ps1:30` 的 token 清单为 `\berror\b\|unresolved reference\|cannot infer type\|找不到名称\|类型不匹配\|编译失败\|运行失败`，与 `lib/gate-error-lines.ps1` 那份相比恰缺 `Could not resolve`；样品 E8 已入 `utils/hxErrorLinesBehavior.test.js`；措辞修复原留**三份判据合一票**（#2045 正文 Out of Scope 第一条），**提前由 #2166 单独补族销账**——**销账证据（#2166 成对取证）**：E9 反向题 + E3/E5 题库补血账原形行，红腿 3 failed（E3/E5 `Expected 5 Received 4`、E9 插件形 `Expected 1 Received 0`），绿腿 `hxErrorLinesBehavior` 9/9、④ 侧 `gateErrorLinesBehavior` 11/11 无串扰、接线腿全量 2812 全绿；带时间戳设备形照旧被排除（ADR-0012 判据未翻转） | 本簿首录 |
| 26 | ② 指纹只哈希文件，漏非文件型来源 | 改 `publish` 参数**不动任何哈希组件**（门脚本与一切 `.ps1` 被构建输入集排除）→ 三组件全等、盘上还是旧命令的产物 → 自动 `-SkipBuild` 跳过 publish → 新命令从未执行而门在旧产物上出绿（fail-open，门可信度随它去） | fixed（#2048，2026-10-28） | 第四道命中前置条件 = publish 命令规范化串随指纹落盘并比对（`training-app/叉车维修培训学员端跨端应用/scripts/lib/mp-weixin-build-fingerprint.ps1`，schema `/2`）；行为守护 `utils/mpWeixinBuildFingerprintBehavior.test.js`（命令变 / 记录未含 / 本轮没给 三形必红 + 长度前缀规范化真执行）；成对销账证据 = 本票门评论（修复前 `AUTO_SKIP_BUILD fingerprint-hit` 跳过 publish、修复后 `miss reason=build-command-changed` 真重建）；**残余 G2**（HBuilderX `plugins/` 编译器本体不在键里）由维护者裁定不在本票收口，见 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md` ② 段回记 | 本票 #2048 |
| 27 | 门链证据日志不可解码（存量 36 份） | `.ci-verify` 下的证据日志大面积读不出来 → 捕获层**按块无状态解码** + 控制台代码页误读，两类损坏都**不可逆**（A 类反向还原丢字）→ A1 形态下证据是荣誉制（校验器不读图、不验内容真伪），**没有任何门会发现手里的日志是坏的**，PR 正文抄的读数行与留作判据输入的原始产物双双被污染 | fixed（#2139，2026-10-30：判据进门 + 采集层止血；**存量 36 份不救**，已证不可逆） | 扫描口径（2026-10-30 本机现测）：`node .github/scripts/check-evidence-encoding.mjs --root D:/FL` ⇒ `EVIDENCE_ENCODING files=90 corrupt=34 clean=56`（诊断脚本原读数 `files=99 corrupt=36 clean=63`；差额 2 份属并发会话树 `wt-2124` 与 `.scratch/1279-verify`，新判据按「一票一树」剪掉、不替别的会话背红）。分三类：A-gbk-mojibake 29 / B1-raw-bytes 2 / B2-replacement-char 3。**更正票面**：「B 类 = 原始 GBK 字节直接落盘」只对 2 份（`mp-weixin-1941-rerun*.log`）成立；另 3 份（含 ③ 门 `test-behavior.log`、`2045-behavior.log`）是**合法 UTF-8 里烤进 U+FFFD**、位置成簇（2376/2378/2379/2380…）⇒ 根因是 `dev-finish.ps1` 的 `$readNew` 每块调无状态 `UTF8.GetString`，正解是「只解码凑齐的前缀、半截尾字节留在文件里」而非补 `StandardOutputEncoding`（`scripts/lib/process-capture.ps1` 的 `Read-Utf8Chunk`；`System.Text.Decoder` 那条路 PowerShell 绑不到 —— 实测 `DecoderNLS` 是 internal、无可见 `GetString` 重载）。守护：`.github/scripts/evidence-encoding.test.mjs`（九条）+ `utils/evidenceCaptureEncodingBehavior.test.js`（四条腿，旧写法变异腿必红 2 个 U+FFFD）；接线：`preflight-pr` 只对**本次新产出**（mtime ≥ 分支点）判红即不绿，存量豁免条数如实报 `skippedStale=` | 本票 #2139 |

---

## 附：状态统计与待办

- **open**：#1 #2 #3 #6 #8 #10 #11 #13 #14 #18 #19 #20 #21 #24（14 条，其中 #19 是「设计裁定记档防误修」，不算待修）
- **fixed**：#7 #16 #17 + #22/#23（2026-10-20 当日收口两条）+ **#4/#5**（#2045，2026-10-27：判据抽 lib + 金丝雀题库销账，状态改动由机检读出） + **#26**（#2048，2026-10-28：② 产物指纹补第四道「publish 命令一致」前置条件，改命令复用旧产物的 fail-open 由成对门评论销账；G2 = HBuilderX `plugins/` 编译器本体不在键里，维护者裁定不在本票收口） + **#27**（#2139，2026-10-30：证据日志可解码性判据进 `preflight-pr`（三类 A/B1/B2、只判本次新产出）+ 采集层按块解码止血（`Read-Utf8Chunk` 前缀留存）；**存量 36 份登记为不可复算、不救**） + **#25**（#2166，2026-10-31：第三形态 `lib/hx-errors.ps1` 补 `Could not resolve` 族，E9 成对样品先红后绿销账；「带时间戳的编译诊断归哪一路判」留在合一候选，见本簿尾）
- **superseded**：#12（被 #1156 修正 + #1702 拆腿取代）
- **待办（按 §5 候选与复盘发现）**：
  - `check-branch-residue.ps1` 接合并后自动触发点（防 #15/#22 复发——#1718 族已四犯，这是簿上最高优先复）→ **#1723 立票跟进**
  - ~~② 门产物未变默认 `-SkipBuild`（压 #11 的失败轮成本）~~ → 已落地（#1860 指纹自动 `-SkipBuild`，2026-10-24）
  - 每页成本预估器（#1/#2/§5 第一条）
  - ~~`cleanup-orphan-worktrees.ps1` 的 targets 表按现状重写或废弃（#23 遗留物）~~ → **已按「废弃」处置（2026-10-27）**：脚本自 #1724 起带 STALE-GUARD 拦停，targets 表始终未按现状重写；本地 `wt-` 目录已清零，脚本无再跑场景，**删除**。判据 `git worktree list` 现为单棵树，清理改用 README 里那条一次性命令即可
  - `Invoke-Process` 两份现存写法逐份核超时语义（#6 收敛后未复核）
  - cd.yml gate 主题尾缀 (#N) 的 PR/issue 判型回退（#24，2026-10-20 首犯）
  - **三份错误行判据合一**（#4 残留面；#25 的同族缺账已由 #2166 补族销账，本候选动机转纯整合 + 「带时间戳的编译诊断归哪一路判」的归属裁决）：`lib/gate-error-lines.ps1` 的 ④a/④c 两份 + `lib/hx-errors.ps1` 那份，输入面不同（CLI 编译日志 / 纯 kotlinc 文本 / CLI stdout 混设备日志），合一前先裁「哪条判据对哪一路输出负责」。样品已按**判据族**组织（`utils/gateErrorLinesBehavior.test.js`），合一那天可原样平移、且自带验收（#2045 正文 Out of Scope 第一条）
