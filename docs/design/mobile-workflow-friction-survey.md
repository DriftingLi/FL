# 移动端开发流程摩擦清单（friction survey）

> 用途：一位觉得「移动端开发流程太费时间」但说不清卡在哪的人，可凭本文件看到**每一条有出处的摩擦点**。
> 立场：只查本仓 primary sources（docs / scripts / CI / 源码），不联网、不发明数字；所有时间成本一律标注为 **ESTIMATE**（由源文件描述的步骤推导，非实测数据；凡源文件自带实测数字的，标注为「源实测」）。
> 路径含中文，一律逐字书写。引用 ADR 时按本仓纪律写全路径：`training-app/叉车维修培训学员端跨端应用/docs/adr/0008-…` 属**移动端独立编号体系**，根仓库另有同名 `ADR-0008`（根 `AGENTS.md`「领域文件」）。

---

## 1. 移动端开发循环端到端（一段总述）

一次移动端改动从起念到落地要穿过**九个阶段**，每个阶段由不同文件治理：**① 立项/派活**由 `training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md`（移动端 `AGENTS.md`「派活与执行契约」:15-18）裁定形态与阶段闸门；**② 建工作树**是硬闸门——`training-app/…/scripts/new-worktree.ps1 -Task <票号>`（根 `AGENTS.md`「建 worktree 一律走闸门」段；移动端 `AGENTS.md`「Qoder 托管 worktree 的使用边界」:133-156 规定 `training-app/**` 下任何活都不得在宿主树做，判据是「③ 要不要在这棵树里跑」，因为 CI `changes.mobile` 过滤器就是 `'training-app/**'`（`.github/workflows/ci.yml` 的 changes job）），建完由 `scripts/wt-bootstrap.ps1` 做 junction 共享 `node_modules` + 路径安全性机检；**③ 编码**阶段的反馈节奏由移动端 `AGENTS.md`「开发内循环」:90-118 的三层节奏表规定（`hx:compile-only` / 热刷新 / `hx:run` / 中循环 / 外循环），实测数字与坑位在 `training-app/…/docs/agents/dev-loop.md`「时间账与三样慢」「成本账反模式」；**④ 自测/中循环** = 每个「视觉满意点」一次 `npm run test:unit` + ④ 本地编译门（`build:kotlin-all` 默认，必要时 `build:compile`）；**⑤ 取证/收口**由 `scripts/dev-finish.ps1`（`npm run dev:finish`）串起 ①a 真机截图、② 微信门、③、④，产物按 `docs/adr/0008-…` 的纪律入 `docs/verification/<模块>/<PR号>/` 或经 `-PostToPr` 贴 sha 绑定评论；**⑥ 开 PR 的时机**由根 `docs/agents/release.md`「开 PR 的时机」段裁定（一件事做完并验完之后才开）；**⑦ 证据结构校验**由 `.github/workflows/pr-evidence.yml` 对每个 PR 运行，校验器逻辑被 `.github/scripts/pr-evidence-check.test.mjs` 以抽取式单测钉住；**⑧ CI 门禁**只有一条真正跑代码——`ci.yml` 的 `mobile-test` job（`npm run test:unit`，经 `ci-summary` 成为 ruleset 唯一必检），而 `pull_request` 事件不触发任何 CI，检查来自分支 push 的同 commit run（`release.md`:3-5）；**⑨ 合并与收尾**由 `release.md` 七步治理（squash 合并 → `cd.yml` gate 直发 production → `git worktree remove` → `branch -D` → 删远端，反序会让 `--delete-branch` 静默不删）。`.github/workflows/` 现只有 `pr-evidence.yml`、`testing-smoke.yml`、`cd.yml`、`ci.yml` 四个文件——**没有任何其他 workflow gate 移动端 PR**。

---

## 2. 摩擦清单表（按可疑总成本降序）

| # | 阶段 | 摩擦点 | 源码依据 (file:line / 小节) | 为什么费时间 | 量级估计（标注为估计） | 频率 |
|---|---|---|---|---|---|---|
| 1 | 取证/收口 | **真机取证一条链本身就要跑完「编译 → 装机 → 导航 → 截图 → 像素比」**，是单次最长的一段机器时间 | `dev-loop.md`「样式内循环：一条命令、一行结论」时间账（2026-10-07 实测）：顺利一轮链自报 235 秒（编译 66 + 装机 78 + 导航 83 + 截图/比） | 一次「视觉满意点」要先花约 4 分钟在设备侧动作上，才拿到「这一页有没有变」的结论；收口取证页数多于 1 页时按页放大 | 源实测 235 秒/页；多页模块按页数线性放大（ESTIMATE：10 页 ≈ 40 分钟量级） | 每个视觉满意点 / 每分支收口一次 |
| 2 | 取证/收口 | **`dev:finish` 全链路 = ①a 取证 + ② 微信门 + ③ + ④c 整模块编译门 + 证据生成**，与 #1 的差集正是「门的活」 | `dev-loop.md` 同节：同设备同页 `dev:finish -Level standard` 实测 **1007 秒**（其间含 151 秒排队等另一会话的锁），与 235 秒的一轮差 772 秒来自「不做 ④c 整模块编译门、契约测试与证据生成」 | 收口一次 ≈17 分钟机器时间；② 失败重跑还要再加 9–26 分钟/次 | 源实测 1007 秒/次（标准级）；ESTIMATE：更高 Level 只增不减 | 每分支收口一次（①a 按「一次分支收口」跑，不跨 PR 共用——移动端 `AGENTS.md`「验收门与合并纪律」:179-195） |
| 3 | 自测/中循环 | **③ 门串行跑（`-i`）与并行跑的差距被写死在配置里**：门命令必须稳、可复现，所以日常内循环要另走一条并行命令 | `jest.config.unit.js`:1-13 头部注释（#1410）：本地多核用 `npm run test:unit:local`（并行，实测 ~60-90s vs 串行 ~236s）；`-i` 在挡的是 CPU 争抢下 5s 默认超时抖动（12 核实测 5 次并行 1 次红，红的是墙钟不是逻辑） | 每个视觉满意点都要跑一次 ③；走错命令（跑了串行）就白付约 3 分钟的差 | 源实测差 ≈146–176 秒/次；ESTIMATE：一次主题 10 个满意点 ≈ 15–30 分钟 | 每次改动（每个视觉满意点一次） |
| 4 | 自测/中循环 | **③ 门历史上大量「全绿」是恒真的**（手抄镜像实现），所以「跑绿」不能当验收，被迫补「故意弄坏它会不会红」的成对取证 | `training-app/…/docs/agents/guards.md`「为什么『全绿』不是证据」：改坏 `utils/format.uts` 的 `return '0s'`，`format.test.js` 照样 8 passed；现状快照 行为守护 15 / 接线守护 88 / 合计 103（2026-09-19，真源是 `scripts/classify-guards.mjs`） | 每加/改一处守护都要先判它属行为还是接线，行为守护才承重；接线守护不构成 ③ 证据 ⇒ 验收结论要另找真链路 | ESTIMATE：每处守护多一次分类器运行 + 一次「弄坏-复跑」成对取证（分钟级） | 每次新增/修改守护（≈ 每 PR） |
| 5 | 建工作树 | **移动端活一律不得在宿主树做**，必须先建合规 worktree，且初始化只有单一真源 | 移动端 `AGENTS.md`「Qoder 托管 worktree 的使用边界」:133-156；根 `AGENTS.md`「建 worktree 一律走闸门」段；`new-worktree.ps1` 流程（fetch → worktree add → bootstrap → `jest --listTests` 实测计数 >0，0 即 exit 3） | 每票一次建树 + 一次全量 `--listTests` 验证；选错落点（宿主树 `<userHome>/.qoder/worktree/…` 父段带点）会让 ③ 在本地**静默假绿**（`dev-loop.md`「三条原因的实测原文」第 1 条：点父段 `--listTests` 0 套件且 exit 0，无点父段 141 套件） | ESTIMATE：建树 + 验证 ≈ 数分钟（源未记墙钟）；踩坑改名要 `git worktree move` 重来 | 每分支（每票一次） |
| 6 | 建工作树 | **Qoder「本地任务的 Worktree 配置」框是第二真源陷阱**：框里必须且只能填一行 `wt-bootstrap.ps1` 调用，而框由宿主维护、PR 审查不到、本仓文档不校验其现值 | 移动端 `AGENTS.md`:133-156 末段；`dev-loop.md`「三条原因的实测原文」末段：核对方式 = 读 `%APPDATA%\Qoder\logs\<run>\questWindow\renderer.log` 的 `worktreeSetup` 键与那一行比对；填逻辑 = 开了第二真源（已记过账的后果：worktree 里 `npm test` 报 `'jest' 不是内部或外部命令`） | 漂移了不会报错，只在该跑门时以「命令不存在」或「0 套件假绿」的形式出现，排查要回读宿主日志 | ESTIMATE：每次漂移一次排查（分钟级～小时级，取决于何时发现） | 不定期（宿主每次改动/升级） |
| 7 | 并发/排队 | **HBuilderX 是单实例串行资源，真机步骤被钉成不可并发**，多会话只会排队；上限到点 `exit 2` 判环境不可用 | 移动端 `AGENTS.md`「并发会话纪律」:120-131；`training-app/…/docs/agents/concurrency.md`「会话时间预算与『先写代码、后集中上机』」：`hx-run` 120 秒 / `dev:finish` 1800 秒 / `build:*` 600 秒；实测一笔账：排队一次约 10 分钟、卡死一次白等约 20 分钟（`dev-loop.md`「时间账与三样慢」） | 并行写代码时真机阶段必须攒批集中到一个会话；中途反复进出会在锁上重新排队 | 源实测：单次排队 ≈10 分钟、卡死 ≈20 分钟；ESTIMATE：一天多次并发 ≈ 累计数十分钟 | 每次并跑真机步骤 |
| 8 | 证据结构 | **`## 验收证据` 每门必须一行四字段 + 行内可核验产物**，写错位置/形态校验器一律不认 | `.github/workflows/pr-evidence.yml`:291-308（截图判据三支 + 不接受 `.ci-verify/*.png` 与任意 http(s)）、:309-314（产物判据）、:275-281（GATES 表）；`.github/scripts/pr-evidence-check.test.mjs`「② 结论须引用截图、.ci-verify/entry.png 不算产物」 | 产物必须入 `docs/verification/<模块>/<PR号>/`（PR 号须数字）或贴 sha 绑定评论；裸本地路径、子条目、缺扩展名都判「未引用可核验截图」 | ESTIMATE：每 PR 一次结构整理（分钟级）；返工则加一轮 ②/④ 重跑（分钟～十几分钟） | 每 PR |
| 9 | 证据结构 | **sha 绑定门证据在 master 前进后可能作废，必须重跑重贴**（2026-09-12 放宽后仍有条件：其间不得有运行时面改动） | `.github/workflows/pr-evidence.yml`:148-232（`judgeGateComment`：commit==head 免 compare；否则 compare 须 status∈{ahead,identical}、mergeBase==commit、`!filesTruncated`、其间无 `isRuntimeFile` 改动）；fail-closed 条件见 `.github/scripts/pr-evidence-check.test.mjs` e/e2/e3 组；动机原文：ruleset `strict_required_status_checks_policy` × master 高频前进 ⇒ #624 反复被拖 | 每次 sync master 都要现测 `git diff --name-only <旧证据sha> <head>`；「我觉得没改」不算判据；compare 拿不到/超大 diff 截断 ⇒ 仍须重跑 | ESTIMATE：每次 master 前进一次判读（分钟级）；命中重跑则加 ④c ≈2.9 min 或 ② ≈2–26 分钟（源实测账：`dev-loop.md`「成本账反模式」：#1443 为绑一个 sha 重跑完整 ④c，其间只动了一张 jpg） | 每 PR × master 前进次数 |
| 10 | ② 微信门 | **② 失败轮 9–26 分钟/次**，且默认重试每轮都重新 publish；不带 `-SkipBuild` 是记过账的反模式 | `training-app/…/docs/agents/dev-loop.md`「成本账反模式（2026-10-03 立）」第二条：`mp-weixin-check.ps1:6` 自述「≈4.5 分钟」是**成功那次**；失败轮构成是 `close→open→auto` 整段重试（`-AutoAttempts` 默认 3）每轮重新 publish；产物没变时第二次起带 `-SkipBuild` 实测 ≈2 分钟 | ② 的历史坑位密集（端口不监听、auto 挂住、残留会话复用、工具版本本身坏），每坑都按「分钟～小时」计 | 源实测：失败 9–26 分钟/次、带 `-SkipBuild` ≈2 分钟；ESTIMATE：一轮排查 30–120 分钟 | 每 PR 命中 ② 触发面 × 失败次数 |
| 11 | ④ 本地编译门 | **④c 整模块编译门（`build:kotlin-all`）是收口默认项**，④a（`build:compile`）在 dev 面追加；两条脚本都须全访问权限执行 | 移动端 `AGENTS.md`「验收门与合并纪律」:179-195（④c 默认 → `.ci-verify/kotlin-all.log`；dev 面追加 ④a → `build.log`；受限沙箱下与 HBuilderX IPC 报「与主程序的连接已中断」）；`dev-loop.md` 时间账：全量编译 4.8–8 分钟 | 每个收口 PR 至少一次整模块编译；受限沙箱里跑不了 ⇒ 环境不可用 | 源实测：全量编译 4.8–8 分钟；ESTIMATE：④c publish + kotlinc ≈ 3.5 分钟（#1443 账：publish 53 s + kotlinc ≈2.9 min） | 每 PR |
| 12 | CI 门禁 | **ruleset 要求分支 up-to-date**：master 一前进就得 `git merge origin/master` → 重推 → 重跑 CI，才能合并 | 根 `docs/agents/release.md`:11（第 ⑤ 步）；`ci.yml` 的 `ci-summary` job 是 ruleset 唯一必检且 `skipped` 不满足必检 | 高频前进的 master 上，等门禁的时间被「同步-重跑」循环放大；CI 只在分支 push 触发（`ci.yml` `on.push.branches-ignore: [master]`），PR 事件不跑 | ESTIMATE：每次同步一轮全量 CI（源未记移动端单 job 墙钟；纯文档改动约 20 秒——`release.md`:10） | 每 PR × master 前进次数 |
| 13 | 环境卫生 | **跑完 HBuilderX 步骤必须查工作树**：`manifest.json` 被回写置空 appid、`pages.json` 被写入一段 `condition`，误提交会让 PR 凭空命中 ④b | 移动端 `AGENTS.md`「反模式（逐条禁）」:158-177 末条；`dev-loop.md`「工作树与会话坑位原文」：`pages.json` 同时是运行时面/打包面判据来源；`docs/adr/0008-…` appid 口径段 | 每轮 HBuilderX 步骤后一次 `git status` + 还原；漏一次就把一个本地开发配置送进 PR，触发额外的 ④b 云打包门 | ESTIMATE：每轮分钟级；漏检则加一轮 ④b（云打包墙钟源未记，标注不可估） | 每轮 HBuilderX 步骤 |
| 14 | 清理/收尾 | **真运行会话常驻攥住项目目录**，删 worktree / 改名前目录既删不掉也改不了名；`cli project close` 在本机 HBuilderX 5.23 不可用 | `dev-loop.md`「真运行会话常驻与收口」「工作树与会话坑位原文」（一个会话活了 51 分钟，每约 10 分钟重试 `wakeUpDevice` 被设备以 `INJECT_EVENTS` 拒绝）；`docs/agents/multi-agent-git.md`「HBuilderX 仍要唯一项目名」段（2026-09-17 血账：`Rename-Item` 报 `being used by another process`；唯一非破坏解是发起下一次 launch 顶掉） | 收尾顺序被钉死：先想清楚「这个目录本轮还要不要改名/删树」；持锁者可能不止 cli.exe（2026-09-21 #1240 P2：close+quit 后仍 Access denied） | ESTIMATE：每次收尾多一次「等人/等下一次 launch」（分钟级～不确定） | 每分支 |
| 15 | 清理/收尾 | **`git worktree remove` 连带删掉 gitignored 的 `.ci-verify/`**（原始判据输入，删了不可复算）；`worktreeMaxCount` 超限还会自动回收别人正开着的树 | `docs/agents/multi-agent-git.md`「删 worktree 之前必须把 gitignore 覆盖的证据产物拷出来」（2026-09-22 血账：结论文里的像素读数 27090/17286/9804 不再可复算，入库 JPEG 是重编码件）；移动端 `AGENTS.md`:133-156 第 3 条原因 | 每棵树的证据要么当场拷出、要么入 `docs/verification/`；「日志还在」的前提是树还在 | ESTIMATE：每树一次拷贝（秒级）；丢失则整条取证结论作废（不可估） | 每分支 |
| 16 | 清理/收尾 | **`--delete-branch` 会静默不删分支**，残留分支要靠机检脚本兜底；反序清理（先删分支再删树）也删不掉 | `docs/agents/release.md`:13-15（14 个远端残留分支、13 个对应已 MERGED 的 PR，时间线无一条 `head_ref_deleted`；gh 2.97.0 把远端删除包在 `if !m.merged` 内；分支被 worktree 占用时本地删除先失败）；收尾机检 = `training-app/…/scripts/check-branch-residue.ps1` | 合并后只信 API 当场读数；清理顺序必须是 `git worktree remove` → `branch -D` → 删远端 | ESTIMATE：每 PR 一次机检 + 可能的残留清理（分钟级） | 每 PR |
| 17 | 派活/会话 | **一件票一个会话、每票冷启动**，只读票 + 对应 ADR + `docs/refactor-decisions.md`；接近 150k 要 `/compact` | 移动端 `AGENTS.md`「会话切分约定」:46-51 | 每票重读上下文（含 222 行常驻层 + 按需层 ADR）；`AGENTS.md` 自身 29,116 B / 222 行（`multi-agent-git.md` 量级读数） | ESTIMATE：每票一次冷启动读面（分钟级） | 每票 |
| 18 | 派活/会话 | **主树落后 origin/master 时，会话拿到的是过期常驻层**（AGENTS.md 从磁盘现取，树建在落后 HEAD 上） | `docs/agents/multi-agent-git.md`（2026-10-04 补 #1524）：判据 = `git diff --name-only origin/master -- "training-app/叉车维修培训学员端跨端应用/AGENTS.md"` 非空 ⇒ 先同步再起树；实测 #1524 票内本地脏副本 52,033 B 与 origin/master 29,116 B 并存过 | 按过期口径执行会把上游已删的规则当现行；同步一步可防，但判据要每次现测 | ESTIMATE：每次建树前一次 diff（秒级）；漏判则整票按错口径做（不可估） | 每分支 |
| 19 | ② 微信门 | **② 触发判据曾把「在文档里提到 MP-WEIXIN」当命中**，反向激励是「想不被误报只能不写该术语」 | `docs/adr/0008-…` #1030 修订段：旧判据误报实例 = 在 ADR 里写「② 免（未命中 MP-WEIXIN 面）」这句话本身就把门点亮；新判据 = 条件编译指令行增删（限定运行时面文件）+ 两份 json 清单 | 文档措辞受门判据反向塑造；现行判据已修，但模板/文档仍需按新口径写 | ESTIMATE：每次文档修订多一次判据核对（分钟级）；旧口径下每误报一次 ② ≈4.5 分钟（源实测成功那次） | 每 PR（文档/门判据相关） |
| 20 | ① 真机门 | **① 曾按「碰了运行时面」触发 ⇒「改一行 .uvue 文案」与「改指纹门控」付同样代价**（真机 10–20 分钟 + 人签整条 ①）；收缩为 ①b 后，能力面可机检口径 = 路径白名单 + PR 模板自报勾选（自报是声明不是判据） | `training-app/…/docs/adr/0016-真机门的人工性收缩与按批取证.md`（实测锚点 #1028：纯样式 PR 走了 ① 真机、7 张入库截图；同一份 diff 的 `level-detect.ps1` 判 🟢） | 现行 ①a 按一次分支收口跑（不跨 PR 共用），①b 只在能力面必过；漏报靠 ①a logcat 与评审兜 | ESTIMATE：旧口径每运行时面 PR 10–20 分钟真机 + 人签；现行 ①a 每分支一次（见 #1/#2） | 每分支 |
| 21 | 证据真伪 | **`pr-evidence` 只校结构不校真伪**，且裁定不装「必检 + approve」（逐 PR 审批成本高于约束收益）；发生率史实：最近 20 个已合并 PR 里 14 个声明编译门、5 个声明真机门，可核验产物 0 个 | 移动端 `AGENTS.md`「验收门与合并纪律」:179-195 末条；`docs/adr/0008-…` 机制层事实段；`dev-loop.md`「三条原因的实测原文」第 1 条引 pr-evidence.yml 顶部 P1 注释「判据只为『结构与 sha 绑定』，不校真伪」 | 结构校验便宜但挡不住「抄宿主树假绿」；本地反馈环失效（宿主树 0 套件 exit 0）时，抄进来的结论过得去 | 不可估（非时间问题，是判据强度问题） | 每 PR |
| 22 | 环境前提 | **两条门脚本都须全访问权限执行**（受限沙箱下与 HBuilderX IPC 报「与主程序的连接已中断」；微信 CLI 连自己端口文件都写不了，EPERM 表象是「工具没装」） | 移动端 `AGENTS.md`:179-195（④ 两条脚本）；`docs/adr/0008-…`「② 门三条硬前提与证据边界」 | 沙箱权限错一次，门以环境问题形式失败，白查一轮（源实例：坏产物被报成 exit 2 环境问题，排查被引向环境/锁/端口） | ESTIMATE：每次误判一次排查（分钟级～小时级） | 每次在受限环境跑门 |

---

## 3. 已知血账（repo 文档自己承认的缺陷）

按「文档原文记过账」列出，均为本仓 primary sources 自述，非本报告推断：

1. **③ 门在宿主树里静默假绿（血账 #1144，闸门 #1185）**：D:\FL\.wt-<task> 下 `npm run test:unit` 报 `No tests found … testMatch: … - 0 matches`，且 `--listTests` **空输出 + exit 0**；机制已定位到 jest-util 的 `replacePathSepForGlob`（jest 27.5.1：`path.replace(/\\(?![{}()+?.^$])/g, '/')`，故意不转换点段前的反斜杠，picomatch 把 `\.` 读成转义的 `.`、分隔符消失）。对照实测：`D:\FL\wt-1144` 97 条 vs `D:\FL\.wt-1144probe` 0 条。来源：`docs/agents/multi-agent-git.md`「worktree 目录名不要以 . 开头」；`training-app/…/scripts/new-worktree.ps1` 的 SYNOPSIS 与判据段；`dev-loop.md`「三条原因的实测原文」第 1 条（点父段 0 套件 vs 无点父段 141 套件，宿主落点 `<userHome>/.qoder/worktree/…` 不可配置）。
2. **② 门的历史坑位密集（#883 起多轮修订）**：close/open/auto 全部回显成功但项目窗口从未真正打开（须人在 GUI 里导入一次）；就绪闸门要求 `App.getPageStack` 应答（实测 t≈18s，两次实测 24s/34s）；两条门红被误报成「自动化端口连不上」（`Cannot read properties of undefined (reading 'split')`）；残留自动化服务被静默复用（换端口即恢复）；微信开发者工具 Stable 2.01.2510290 上 auto「接受 AUTO 随后把 IDE 一起带走」，处置 = 重装（换 2.02.2608070 后一次通过）。来源：`docs/adr/0008-…`「小程序门（第②门）」。
3. **② 产物合法性判据（#1210）**：旧实现用正则从 `project.config.json` 抠 appid，非法 JSON 也能过；坏产物被报成 exit 2 环境问题，白查一轮。来源：同上。
4. **`compile-check.ps1` 错误行判据漏整族诊断（#1196）**：实测一棵编译不过的树打 `COMPILE_RESULT errors=0 clean=True` 并自陈「✅ 编译门通过」（日志 `Could not resolve` 命中 1、脚本正则命中 0）；同族潜在盲区：`kotlin-all-check.ps1:64` 的 `$ErrorLinePattern = '(?m)(^\s*e: )|(:\d+:\d+: *error:)'` 对「时间戳 + error:」与「⛔error:」match=False。来源：`docs/adr/0008-…`。
5. **`kotlin-all-check.ps1` 第二族假绿（#1272）**：publish 段失败只认两条文案（`-1:cli:命令'publish app-android'不存在或缺少参数` 被放过），且只看有没有 `.kt` 不看新鲜度 ⇒ 磁盘旧导出就算过。来源：同上。
6. **`Invoke-Process` 超时只看进程退出段（#1285）**：实测总耗时 41137 ms 且 `TimedOut=False`；同款写法在 `mp-weixin-check.ps1` 与 `scripts/lib/hx-busy.ps1` 各一份。来源：同上。
7. **`dev:finish` 步骤 7 截图对比曾是假绿（#1139）**：只比 MD5、基线目录永远为空、有变化只 `Write-Host` 一行黄字永不 fail；改为像素差 + 阈值 0.5%。来源：同上。
8. **adb 解析三份实现漂移（#1027）**：内联那份漏 D 盘兜底 ⇒ 步骤 6 恒报「找不到 adb.exe」，而步骤 4/5 已真跑完（编译 ~5 分钟 + 推送 162 秒），步骤 7–9 永远到不了。来源：同上。
9. **「画面稳定」判据被系统状态栏钉死（#1027）**：全帧 hash 相等被 MIUI「显示实时网速」钉死 ⇒ 步骤 6 必然 420 秒超时；改为采样网格差异像素占比 ≤0.5%。来源：同上。
10. **`--compile` 语义坑位（血账 #917）**：官方语义是「仅编译代码」，从前真运行路径照抄了门的调用 ⇒ 从未真正请求过运行，「编译成功 → 已停止运行...」被误读成「运行失败」并追了整整一票。来源：`dev-loop.md`「`--compile` 语义坑位（血账 #917）」。
11. **成本账反模式（2026-10-03 立）**：sha 无害前进就重跑整道门（#1443 为绑 `c3adc58` 重跑完整 ④c，其间只动了一张 jpg；#1467 merge 只带进两个 Go 域搬包，② 与 ④c 仍各重跑一次）；② 失败后不带 `-SkipBuild` 重试。来源：`dev-loop.md`「成本账反模式（2026-10-03 立）」。
12. **③ 门「全绿」可恒真（#1156 / `docs/agents/guards.md`）**：改坏 `format.uts` 的 `return '0s'`，`format.test.js` 照样 8 passed；分类器第一版把 9 个手抄镜像误判成行为守护（20/78），修正后 13 行为 / 86 接线。来源：`docs/agents/guards.md`。
13. **真运行会话常驻锁住项目目录（2026-09-13 一天踩两次；2026-09-17 / 2026-09-21 #1240 P2 追记）**：`Rename-Item` 报 `being used by another process`；HBuilderX 5.23 无 `project` 子命令，`cli project close --path` 不可用；持锁者不止 cli.exe。来源：`dev-loop.md`「真运行会话常驻与收口」「工作树与会话坑位原文」；`docs/agents/multi-agent-git.md`。
14. **`.ci-verify/` 随工作树回收一起消失（2026-09-22 血账）**：判据输入（原始 screencap PNG / uiautomator XML / launch 与 logcat 日志）删树后永久丢失，像素读数不再可复算。来源：`docs/agents/multi-agent-git.md`。
15. **`--delete-branch` 静默不删（2026-09-19 实测）**：14 个远端残留分支、13 个对应已 MERGED 的 PR；gh 2.97.0 `deleteRemoteBranch()` 包在 `if !m.merged` 内。来源：`docs/agents/release.md`:13。
16. **`gh pr merge --squash --subject` 抑制 `(#N)` 后缀（2026-10-03 实测，P2 波 4e）**：gate 报「无法从 commit 主题解析 PR 号」并 exit 1，生产构建全部被跳过（实测 run 37090597016）。来源：`docs/agents/release.md`:14。
17. **`ci-summary` 曾漏 `changes` 于 `needs`（2026-09-28 修，真实缺陷 #2）**：changes 失败 ⇒ 子 job 全 skipped ⇒ FAILED 为空 ⇒ 汇总 success，而它是 ruleset 唯一必检 + cd.yml 生产 gate 唯一 CI 判据 ⇒ 零检查的 PR 可直发 production。来源：`.github/workflows/ci.yml` `ci-summary` job 注释。
18. **Windows 本机 golangci-lint 不可用（根线血账）**：v1.64.8 由 go 1.26.4 构建，面对 go 1.27.1 导出数据报 `cannot decode "internal/goarch": export data version 4 is greater than maximum supported version 2`；本机四件套实为三条 + CI 兜底。来源：`docs/agents/checks.md`「环境 A」。
19. **pr-evidence 校验器的两条硬赢规则**：① 逐文件低风险豁免是 per-file 的（`pr-evidence.yml:121-130`），顺手改一处 `scripts/*.ps1`（哪怕只为契约套件补一条 token 注册）就拿不到豁免；②「结论」栏引用的可核验产物必须写在该行**行内**，写在子条目、裸 `.ci-verify/*.png` 或任意 http(s) 链接一律不认。来源：根 `AGENTS.md`「两条不看源码就读不出来的硬口径」；`pr-evidence.yml`:291-308。
20. **模板门行形状锁（#1403）**：生成器吐「多行块」而校验器按「每门一行」取门 ⇒ ①③④ 全判「缺该行」，移动端 PR 一开即红、逼人手工重排结构；为什么不抽「门规格表」统一驱动三处（校验器内联不 checkout ⇒ 共享 lib 读不到 ⇒ 表只能内联副本 + 逐字相等锁 ⇒ 用锁把复述品包装成真源是假整合）。来源：`.github/scripts/pr-evidence-check.test.mjs` #1403 段。
21. **Qoder Worktree 配置框第二真源陷阱**：框里那段脚本依赖宿主注入的私有环境变量，命令行/闸门/别的编辑器都拿不到；已记过账的后果 = worktree 里 `npm test` 报 `'jest' 不是内部或外部命令`。来源：`training-app/…/scripts/wt-bootstrap.ps1` SYNOPSIS；`dev-loop.md`「三条原因的实测原文」末段。

---

## 4. Open questions / 无法从来源确定

- **真实墙钟分布未知**：源文件只给了「顺利一轮」的单点实测（235 秒 / 1007 秒 / 全量编译 4.8–8 分钟），没有跨票的分布、p50/p90、也没有「一天实际几轮」。`docs/design/1494-mobile-dev-wallclock-audit.md` 与 `docs/design/1545-mobile-gate3-rerun-reading.md` 存在但本报告未展开读取，其数字未经本报告复核。
- **①b 人工签核多久卡一次未知**：ADR-0016 记了「能力面」触发集合与「漏报靠评审兜」，但没有签核等待时间的任何记录；「执行人」栏由人给出原文这一动作的耗时无数据。
- **真机实验室周转时间未知**：设备数量、谁在排队、adb 无线调试的日常可用率都没有量化来源（`dev-loop.md`「真机无线调试入口」只记了 mDNS 候选逐个现测等判据）。
- **④b 云打包墙钟未知**：源文件只说「正式发版前必须跑一次云打包并装机自测」，未记打包耗时。
- **CI `mobile-test` 单 job 墙钟未知**：只记了「纯文档改动约 20 秒」指 CI 整体跑完的观察，146 套件在 2 核 ubuntu 上的耗时无实测记录。
- **「太费时间」的主观分布未知**：本报告只能按源文件描述的步骤排序可疑成本；哪一段是使用者体感的主要来源，需要使用者指认（本表 #1/#2/#3/#7/#10 是候选）。
- **`worktreeMaxCount` 现值未知**：源文明令「现值读 `%APPDATA%\Qoder\SharedClientCache\cache\app-config.json` 的 `worktreeMaxCount` 键，别冻结一个数字」，本报告不填任何数字。
- **同名项目数未知**：源文要求「现算：`cli project list` 数同名条目，别照抄历史数字」。
- **ADＲ-0016 第 2、3 项（口径同步与三个写实 bug）未落地**：源文明确「生效前 ① 仍按 0008 现行口径执行」——即收缩是否已实际生效，取决于那两项的完成状态，来源未给出完成判据。

---

## 5. Candidate fixes（未设计，每行一句）

- 把 #1/#2 两条时间账的「每页成本」做成可累加的预估器，让开跑前就知道这次收口要几分钟（判据输入已在 `dev-loop.md`）。
- 给 ③ 门一条「并行但稳」的中间档（既非 `-i` 也非全并行），消掉 #3 的 146–176 秒差而不继承 flaky。
- 让 `pr-evidence.yml` 的门证据 sha 绑定在「其间无运行时面改动」之外，接受「其间只有白名单产物文件（如 `docs/verification/*.jpg`）」的等价判据，直接消掉 #9 的 #1443 型重跑。
- 把「建树前必现测 `git diff origin/master -- AGENTS.md`」与「宿主 Worktree 配置框现值核对」两道判据并成闸门 `new-worktree.ps1` 的内置步骤，消掉 #6/#18 的人工核对。
- 给 HBuilderX 单实例资源加一层「会话级排班表」（谁在临界区、预计多久），把 #7 的安静排队变成可见等待。
- 让 ② 门在产物未变时默认带 `-SkipBuild`（仅当改动集未碰 mp-weixin 构建面），把 #10 的 9–26 分钟失败轮压到 ≈2 分钟。
- 把 #13 的 `manifest.json`/`pages.json` 字节还原从「人记得查」改成 `dev:finish` 与 `style:loop` 共用的强制临界区（`style-loop.ps1` 已有雏形）。
- 给 `check-branch-residue.ps1` 接一个合并后自动触发点，消掉 #16 的人工机检。
- 把「证据必须行内可核验」这条硬口径前移到 PR 模板生成器（与校验器同一份门规格表的唯一真源讨论见 #20 血账），减少 #8 的结构返工。

---

*报告完。所有引用的 file:line 均可在本仓对应文件复核；标注「源实测」的数字出自该文件自述的实测记录，标注「ESTIMATE」的为本报告按源文件描述步骤推导的量级。*
