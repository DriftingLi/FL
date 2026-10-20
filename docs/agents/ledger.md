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
| 4 | compile-check 判据漏族 | `compile-check.ps1` 错误行判据漏整族诊断 → 编译不过的树打 `COMPILE_RESULT errors=0 clean=True` 且自陈「✅ 编译门通过」→ 假绿送达收口；同族盲区：`kotlin-all-check.ps1:64` 对「时间戳+error:」与「⛔error:」match=False | open | 实测日志 `Could not resolve` 命中 1、脚本正则命中 0（#1196） | 调查表:51 |
| 5 | kotlin-all 第二族假绿 | publish 段失败只认两条文案（`-1:cli:命令'publish app-android'不存在或缺少参数` 被放过）→ 编译根本没跑也过门；只看有没有 `.kt` 不看新鲜度 → 磁盘旧导出就算过 | open | #1272 | 调查表:52 |
| 6 | Invoke-Process 超时失真 | 超时只看进程退出段 → 总耗时 41137 ms 且 `TimedOut=False` → 挂死被误判为成功；同款写法在 `mp-weixin-check.ps1` 与 `scripts/lib/hx-busy.ps1` 各一份 | open | 实测（#1285）；2026-10-20 复盘核实现存两份（`mp-weixin-check.ps1`、`process-capture.ps1`），收敛自三份但逐份未核 | 调查表:53 |
| 7 | dev:finish 截图对比假绿 | 步骤 7 只比 MD5、基线目录永远为空、有变化只 Write-Host 黄字永不 fail → 截图回归门恒绿 | fixed | 改为像素差 + 阈值 0.5%（#1139 修复，ADR-0008） | 调查表:54 |
| 8 | adb 解析三份漂移 | 三份解析实现各自漂移，内联那份漏 D 盘兜底 → 步骤 6 恒报「找不到 adb.exe」，而步骤 4/5 已真跑完（编译 ~5 min + 推送 162 s），7–9 永远到不了 → 坏的那份只在真机露出来 | open | #1027；同族教训已立「adb 解析的唯一真源」纪律 | 调查表:55 |
| 9 | 画面稳定判据被状态栏钉死 | 全帧 SHA256 相等判据被 MIUI「显示实时网速」钉死（1–2 秒变一次）→ 步骤 6 必然 420 秒超时、页面记 Skipped、7–9 跑不到 → 判据在该类设备上恒不可满足 | fixed | 改为 100px 横带采样差异像素占比 ≤0.5%（#1027 第二阻塞修复） | 调查表:56 |
| 10 | --compile 语义坑位 | 官方语义「仅编译代码」，真运行路径照抄了门的调用 → 从未真正请求过运行，「编译成功 → 已停止运行…」被误读成「运行失败」→ 追了整整一票 | open | 血账 #917；`hx-run.ps1` 真 rho 路径现已禁用 `-CompileOnly` 分支外溢 | 调查表:57 |
| 11 | 成本账反模式 | sha 无害前进就重跑整道门（#1443 为绑 `c3adc58` 重跑完整 ④c，其间只动一张 jpg；#1467 merge 只带两个 Go 域搬包，② 与 ④c 仍各重跑）→ 白付分钟级机器时间；② 失败后不带 `-SkipBuild` 重试 → 9–26 min/轮 | open | 2026-10-03 立（dev-loop.md）；§5 候选 fix（预估器）未落地 | 调查表:58 |
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
| 23 | 孤儿目录壳取证失联 | 5 个无 `.git` 的孤儿目录壳（wt-1484/1542/1554/1607/1713-verify）混在主树 untracked 清单里，无人认领；`wt-1713-verify` 装着 #1713 真机取证 25 文件（rl-1~rl-9 截图 + logcat + desktop，6,575,224 字节），而该票正文只以行内附件引用证据、未仓库化 → 判据输入濒临永久丢失（#14 同族）；`cleanup-orphan-worktrees.ps1` 目标列表（1514/1543/1570/1602/1603/1604）与磁盘现状脱节，脚本自身过期 | fixed（本地） | 2026-10-20 复盘当日抢救入库 `docs/verification/forum/1713/`（25 文件 + README，src=dst=25 验证）后删净 5 壳；脚本 targets 表待更新或废弃 | 本簿首录 |

---

## 附：状态统计与待办

- **open**：#1 #2 #3 #4 #5 #6 #8 #10 #11 #13 #14 #18 #19 #20 #21（15 条，其中 #19 是「设计裁定记档防误修」，不算待修）
- **fixed**：#7 #16 #17 + #22/#23（2026-10-20 当日收口两条）
- **superseded**：#12（被 #1156 修正 + #1702 拆腿取代）
- **待办（按 §5 候选与复盘发现）**：
  - `check-branch-residue.ps1` 接合并后自动触发点（防 #15/#22 复发——#1718 族已四犯，这是簿上最高优先复）→ **#1723 立票跟进**
  - ② 门产物未变默认 `-SkipBuild`（压 #11 的失败轮成本）
  - 每页成本预估器（#1/#2/§5 第一条）
  - `cleanup-orphan-worktrees.ps1` 的 targets 表按现状重写或废弃（#23 遗留物）→ **已加 STALE-GUARD 拦停（#1723），重写待办仍在**
  - `Invoke-Process` 两份现存写法逐份核超时语义（#6 收敛后未复核）
