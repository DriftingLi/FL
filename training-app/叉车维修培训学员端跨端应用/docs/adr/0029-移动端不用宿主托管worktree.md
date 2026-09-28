# 0029 - 移动端不用宿主托管 worktree：③ 门在其默认落点恒假绿，工具改为响亮拒绝

**状态**：已接受（2026-09-28。来源：为 Qoder「本地任务的 Worktree 配置」启动框定稿而做的调研会话，全部判据当场实测。）

> 本文件属**移动端**ADR 编号体系（`training-app/叉车维修培训学员端跨端应用/docs/adr/`），与根仓库 `docs/adr/ADR-00xx` 无关；引用时须写全路径（根仓库另有 `ADR-0029` 的风险由 AGENTS.md 的同名坑位条款覆盖）。

## ① 目标与范围

只裁一件事：**移动端与小程序（uni-app-x 的 Android + mp-weixin 两侧）的改动，不在宿主（Qoder）本地任务自建的 worktree 里做与验证。**

不裁的：Web 前端与后端用不用宿主树、宿主树适合干什么 —— 那是它们的负责人的决定，本 ADR 不表态（维护者 2026-09-28 明确射程：本侧不改 `frontend/`、不改 `backend/`）。

## ② 选型 + 一句理由

**边界按「③ 要不要在这棵树里跑」划，不按「命中运行时面」划。**

因为 CI 的 `changes.mobile` 过滤器是 `'training-app/**'` —— 任何该路径下的改动（**哪怕只改一个 `.md`**）都让 ③ 成为必过门。而 ③ 在宿主树里恒不可信（见⑤判据 A），所以「命中运行时面才走闸门树」这条更宽的口径会被一个 `.md` 直接证伪。

一句话理由：**判据必须与必过门的触发面同源，否则边界上有洞。**

## ③ 明确不做的事

- **不改 `jest.config.unit.js` 的 `testMatch`。** 实测过绕过方案成立（`**/utils/**` 前导形式在点路径下也列出 140 套件；调研基线 140，本票新增守护后为 141），但它是**为迁就一个落点而永久放宽 ③ 的判据面**（`**/utils/**` 会收进任何层级里叫 `utils` 的目录），且该 bug 是 Windows-only、代价却由所有平台承担。真要做：另票、另留 POSIX 侧证据。
- **不让宿主树承载任何移动端交付动作**（写码可以，验证不行 —— 而验证与写码在同一次会话里，所以实际结论就是"不在里面开工"）。
- **不替宿主树规定命名规范**（它由宿主命名与回收，本侧不接管）。
- **启动配置框里不写逻辑**：只允许一行 `wt-bootstrap.ps1` 调用，逻辑进仓内真源（框里的内容 PR 审查不到，且宿主私有环境变量会让命令行与闸门那条链路断掉）。

## ④ 拆分步骤

1. `scripts/lib/wt-bootstrap.ps1` —— 纯函数真源（主树反推 / 子工程枚举 / 点段判定 / 初始化计划），**无副作用**，可被 ③ 在 ubuntu CI 上 dot-source 断言。
2. `utils/wtBootstrapBehavior.test.js` —— 行为守护 W1–W8，token `wtBootstrap` 接进 `Get-ContractTestPattern`（自指，否则是"永不执行的守护"）。
3. `scripts/wt-bootstrap.ps1` —— 入口：落 junction / 打红字 / 退出码分工（`0` 正常与告知、`2` 取不到树、`3` 仅当 `-ExpectEligible`）；`new-worktree.ps1` 建完树即带 `-ExpectEligible` 调它。
4. 规范落文：本侧 `AGENTS.md`「Qoder 托管 worktree 的使用边界」+ 根 `docs/agents/multi-agent-git.md`（收编原 `mklink /J` 手工配方，并登记术语）。

## ⑤ 约束与代价

- **`refuse` 态是故意的**：真源在 ③ 不可信的树里**不链**移动端 `node_modules`，让 `npx jest` 直接报"命令找不到"（fail-loud），而不是链上之后扫到 0 套件报全绿（fail-silent）。代价：这条硬报错历史上被误读成"测试坏了"（已有血账）⇒ 入口必须**先打红字**，红字里写明"这不是测试坏了"。
- **拦截靠红字，不靠失败码**：宿主树不可信是**常态不是故障**，所以入口在宿主树里打完红字后 `exit 0`；`exit 3` 只留给「闸门建的树仍不可信」（即 `new-worktree.ps1` 传入的 `-ExpectEligible`）。否则每建一棵宿主树都返一次失败码 —— 那会让"正常"长得像"坏了"，把信号成本转嫁给不相关的人（包括只干 Web 活的会话）。
- **主树 `npm ci` 有并发窗口**：junction 是共享读，重装期间别的树会看到依赖半套 ⇒ 纪律是"没人跑移动端命令时才装，装完不再在主树重装"。
- **宿主树会被自动回收**（`worktreeMaxCount`，现为 5，删最旧，且连带删掉 gitignored 的 `.ci-verify/` 原始判据输入）⇒ 进树第一件事是建分支、尽早推 origin；判据输入当场拷出或入 `docs/verification/`。
- **HBuilderX 项目名冲突不是本决定的理由**：改成唯一名（`fl-mobile-wt<票号>`）是所有 worktree 的共同代价、且已有 5 处取证先例；闸门树同样要付。把它当宿主树专属阻断是**误判**（本 ADR 明确更正这一点，防止后人拿它去论证别的方案）。

## ⑥ 关键判据（全部当场实测，可复算）

| | 判据 | 复算方式 |
| --- | --- | --- |
| A | 宿主落点 `<userHome>/.qoder/worktree/…` 父段段首带点 ⇒ ③ 的 `--listTests` **0 套件且 exit 0**；同内容无点父段 **140 套件**（调研基线 140；本票新增守护后为 141） | 建两棵只差父段名的树各跑一次；机制是 jest-util `replacePathSepForGlob` 不转换后跟 `{}()+?.^$` 的分隔符（#1144，Windows-only） |
| B | 该落点**不可配置**（bundle 里 `worktreePrefix = join(userHome, dataFolderName, 'worktree')`） | 读 `workbench.desktop.main.js` |
| C | CI `changes.mobile = 'training-app/**'` ⇒ 任何该路径改动都要 ③ | 读 `ci.yml` 的 `changes` job |
| D | `pr-evidence.yml` 校验器"只判结构与 sha 绑定，不校真伪" ⇒ 本地假绿抄进正文过得去（CI 是最后防线，不是本地反馈环的替代品） | 读该校验器注释 |
| E | junction 共享这个模式本仓已跑了一段时间没爆过（移动端依赖主树已装，3 棵树 junction、1 棵独立真装） | `Get-Item …\node_modules -Force).LinkType` |

## 重审条件

下列任一成立即重审本 ADR（否则它会变成"永久正确但没人敢碰"的化石）：

1. 宿主的 worktree 落点变得**可配置**，能落在无点路径；
2. `testMatch` 在独立票里被改成路径无关形态，**并在 POSIX 侧留了证据**（判据 A 失效）；
3. `worktreeMaxCount` 自动清理被关掉或大幅调大（判据⑤第 3 条降级）—— 但注意：这一条**不足以**推翻本决定，因为 A 与 C 不受它影响。

## 术语

**③ 可信树（gate-3-eligible tree）** = 其绝对路径下 `jest --config jest.config.unit.js -i --listTests` 能列出**非 0** 套件的工作树。工具词汇登记在根 `docs/agents/multi-agent-git.md`，**不进 `CONTEXT.md`**（那是业务领域词表，工具词条为 0，混入会污染射程）。
