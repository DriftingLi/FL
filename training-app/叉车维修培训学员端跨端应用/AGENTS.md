# 叉车维修培训学员端（移动端）开发约定

uni-app-x 跨端应用（Vue 3 + TypeScript，页面用 `<script setup>`；目标 Android / iOS / 微信小程序）的 agent 工作约定。**系统级全局约定在仓库根** [`AGENTS.md`](../../AGENTS.md)，本文件只补本线特有规范。主树为 `D:\FL`（搬迁记录见 `docs/agents/multi-agent-git.md`「主树位置」段；原备胎盘 `E:` 已废弃 —— **别照抄任何 `E:\FL` 路径**）。

- **领域词汇表**：根 `CONTEXT.md`
- **架构决策记录**：移动端 `docs/adr/`（**独立编号体系**，与根仓库 `docs/adr/` 的 `ADR-0001+` 互不相关，引用时须写全路径）
- **按需层**：本线 `docs/agents/`（清单见文末「相关文档」）

## Agent skills

### Issue tracker

Issues 存放在 GitHub Issues（使用 `gh` CLI）。See `docs/agents/issue-tracker.md`.

### 派活与执行契约（移动端）

接到**移动端** issue 时的唯一事实源：形态判定 / 读现状（认领、**查承载面**、状态现测）/ 执行 / **阶段闸门** / 收尾（进度契约、重复票处置）/ 正文格式。派活只需一行：`按 training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md 执行 <issue URL>`。See `docs/agents/issue-dispatch.md`.
（**仅移动端**：这套「阶段闸门 / 进度契约」是这条线的工作方式，不是全仓约定 —— 故它落在移动端 `docs/agents/`，前后端会话不读、不受约束。）

### Triage labels

五个 canonical triage roles，label 与 role 同名（`needs-triage` 等）。See `docs/agents/triage-labels.md`.

### 技能供给与管线归属（仅移动端）

技能的**生效面**（harness 扫描根止于 git 根 ⇒ 子树里的技能副本不生效）与四条约定：生成物不入库 / 第三方技能内容不入库 / 技能改动独立成 PR / 引用技能文档引小节名不引行号。See `docs/agents/skills.md`；决策与逐项实测见 `docs/adr/0024-技能供给与管线归属.md`.
（**仅移动端**：前后端会话不读、不受约束；根 `D:\FL\.dsh\skills` 落点与根 `docs/agents/skills.md` 两项已裁定**不做**。）

### 技能路由表（意图 → 唯一默认入口，#1504）

**表体已上收到根** [`docs/agents/skill-routing.md`](../../docs/agents/skill-routing.md)，常驻摘要在根 `AGENTS.md`「Skill routing」段——路由是全环境的事（三套技能都装在用户级，对本机所有会话生效），不该只挂移动端。

移动端只需记住两条**与本线相关**的偏差：建隔离工作树**只走** `scripts/new-worktree.ps1`（见下方「Qoder 托管 worktree 的使用边界（2026-09-28 立）」）、临时产物与 handoff 进 `.scratch/`。其余一律按根表。

### Domain docs

Single-context：root `CONTEXT.md` + `docs/adr/`。See `docs/agents/domain.md`.

### 决策文档约定（所有会话必须遵守）

- **ADR 文件名一律中文命名**：`docs/adr/NNNN-中文标题.md`（先例 `0007-渐进式重构手册.md`）；新建或重命名 ADR 禁止英文文件名；无法翻译的技术专名（如 SSE、JSON）可保留，但凡有中文对应的词（如 playbook→手册）必须用中文。
- **决策/重构清单固定六段、≤25 行**，顺序不变：① 目标与范围 → ② 选型 + 一句理由 → ③ 明确不做的事 → ④ 拆分步骤（标可并行项）→ ⑤ 约束（不能动的模块、API 与 uvue 兼容性）→ ⑥ 验收标准（怎么算完成）。逐段写法与先例见 `docs/refactor-decisions.md`。
- **新决策回写纪律**：手术/实现会话收口时，若产生了 playbook 未覆盖的新决策或新坑位，须先回写对应 ADR（含守护规则落锁情况）再关票——issue 评论不是冷启动会话的必读面，ADR 才是。
- **写票时交付物必须列「承载面 UI」**：验收标准若要求某个界面形态（如「某页只列某区」），**交付物清单必须含承载它的 UI 改动**——按票面字面做完会**复现要防的回归**，且守护的不变式在代码里为假（先例 #1041 与逐步判定见 `docs/adr/0013-专项页会话分区与列该区前置.md`）。

### 会话切分约定（所有技能会话遵守）

- **一条思考链一个会话**：grilling → to-spec → to-tickets 在同一会话内连续完成，中途不 clear/compact——spec 与票不只依赖结论，还依赖被否决的分支和否决理由。
- **一件票一个会话**：implement/execute-task 每票冷启动新会话，只读票 + 对应 ADR + `docs/refactor-decisions.md`；票间 `/clear`，上一票的讨论残留不进下一票。
- **换气只在阶段边界**：接近 smart zone（约 150k）用 `/compact`；跨目录、跨工具或会话中途分叉才用 `/handoff`。
- **收束与执行分座，重构与 UI 分时**：回填结论/补复盘另开短会话，不占手术会话；UI 原型对齐走独立 grilling 链，且同一模块内 UI 对齐 PR 一律排在该模块手术合并之后。

### Security scan

AI 安全审计用 DeepSec（Shield）。See `docs/agents/security-scan.md`.

## 前端 UI 约定

移动端只留**与 Web 栈不同**的部分。页面保持整洁的通则、Tailwind 共存四条规则（R1–R4）与「不得触碰的边界」的**权威版本在根** [`docs/agents/ui-conventions.md`](../../docs/agents/ui-conventions.md)，本处不抄——那些条对本线**不适用**（2026-10-03 现测：未引入 Tailwind、无 `src/`、无残值页面、无 `--color-brand-*` 消费点）。

清理 hint 时**跳过这两条例外（不要清）**：

1. 论坛发帖 / 回复输入区的**属地披露提示**——文案单点 `utils/forumDisplay.uts` 的 `FORUM_REGION_NOTICE`；属地这个事实只在点发布那一刻才产生，事前告知比事后解释便宜（根 ADR-0045）。
2. Markdown 档的**边界提示行**——同文件的 `FORUM_MARKDOWN_BOUNDARY_NOTICE`，**只讲边界、不列能力清单**（移动端 ADR-0025 ⑥-5），依据是根 ADR-0046「判据放在作者看得见的地方」。Web 侧同款叫 `FORUM_MARKDOWN_HINT`，**两端措辞不同是故意的**，别去「统一」它。

两条都由共享输入区组件 `pages/forum/components/forum-markdown-input.uvue` 渲染（发帖与回复同一份形态，ADR-0025 P3），样式类 `.region-notice` / `.boundary-notice`。删 hint 时同步删掉它的 CSS class 与 scoped 规则，别留死代码。

### uni-app-x (uvue) CSS 兼容性规则（判据摘要）

uvue 的 CSS 引擎是**原生渲染器**，只吃 CSS 的一个子集。**全表（选择器限制、不支持的属性与单位、`gap` 替换手法、
UTS 类型限制与 Kotlin 编译期错误对照）在按需层** [`docs/agents/uvue-css.md`](docs/agents/uvue-css.md)；本层只留四条**不看它就会踩**的判据：

1. `.uvue` 的 `<style>` 块**必须**声明 `lang="scss"` —— 否则 SCSS 变量不进预处理，原样传给原生引擎就报错。
2. 选择器只认 `.class`（与 `.a, .b` 分组）；**tag 选择器、ID、伪类 / 伪元素、属性选择器一律不可用**，
   模板里的 `view` / `text` 标签名不得出现在选择器里。
3. **文字类样式只能落在文字类元素上**（`<text>` / `<button>` / `<input>` / `<textarea>`，`font-weight`
   另多一个 `<loading>`）：落在 `<view>` 上会被渲染层**静默忽略**并**每个非法位打一条 error**，
   直接污染 ①a 的错误行判据。机检 = `utils/uvueFontCarrierContract.test.js`（存量 `DEFERRED` **只减不增**）。
4. `gap` / `calc()` / `env()` / `vh` / `vw` / `grid` / CSS 自定义属性 `var(--x)` / `currentColor` /
   CSS `transition` 与 `animation` **都不支持** —— 替代方案写在按需层那几条里，别照 Web 写法试。

改完 `.uvue` 看 HBuilderX 重编译控制台：**ERROR** 阻断编译必须修；**WARNING** 不阻断，但原生端可能不生效（`gap` 被静默忽略把布局搞坏就是这一类）。

## 测试与检查流程

- **本线唯一在跑的验证层 = `npm run test:unit`（它就是 ③ 门）**：`jest.config.unit.js`，`testEnvironment: node`，只扫 `utils/**/*.test.[jt]s?(x)`；`npm test` 是等价别名。只跑一个测试：`npx jest --config jest.config.unit.js <相对路径>`，按名加 `-t "片段"`。判据三条与守护分类（**接线守护不构成 ③ 证据**）见下节 ③ 条与 [`docs/agents/guards.md`](docs/agents/guards.md)。
- ⚠️ **本仓没有 `type-check` 脚本** —— 那是 Web 前端栈的入口，别照搬。后端四件套（含 Windows 本机 golangci-lint 由 CI 兜底、WSL 例外）、生成链顺序（swagger → gen-apitypes → 再跑测试）、PG 契约测试纪律、部署配置校验、DeepSec 扫描的**权威版本在根** [`docs/agents/checks.md`](../../docs/agents/checks.md)，本处不抄。
- **端到端（E2E）自动化本仓不做，别再按 uni-app 项目模板去接**：微信小程序通道只能做 page 级断言、做不了「点击 → 跳转 → 断言」的流程验证（PR #1136 裁决），H5 通道在 `require` 阶段即炸 ⇒ 本仓**没有** E2E 入口与装配。裁决、实测与**将来重启的配方**见 `docs/adr/0020-端到端测试通道裁决落锁与装配清退.md`；补强验证层的正确方向是**把镜像实现迁到 `utils/utsHarness.js` 真执行**（`docs/spec-永绿整改.md` §④ S3 / S6a–S6c），不是引入 E2E。

## 开发内循环（移动端 UI 迭代）

> 口径：**门是全量的，日常是分层的**。`scripts/compile-check.ps1`（④a，实测 4.8–8 分钟）是**门**，默认值不动；
> `scripts/hx-run.ps1` 是**日常载体**，两条路：`npm run hx:compile-only`（**只编译拿诊断**，不碰设备）与
> `npm run hx:run`（**真运行到真机**，编译 + 推送 + 启动）。两者**都不是门、不进 `## 验收证据`**。
>
> **`--compile` 的官方语义是「仅编译代码」**，不是「先编译再运行」⇒ `--compile true` 只允许出现在全量编译门 `compile-check.ps1` 与
> `hx-run.ps1` 的 `-CompileOnly` 分支，**真运行路径一个都不许有**（血账 #917 的误读链与逐条实测在按需层
> [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)「`--compile` 语义坑位（血账 #917）」）。
> **判据必须带基线**：判「有没有到设备」要运行前后各取一次设备侧事实相对比（如资源目录 mtime 前进）；
> 拿「当前前台是不是基座」当判据，在**重复运行到同一台机器**时恒为真 ⇒ 假绿。**反过来说，「仅编译」正是拿编译期诊断的捷径**。

### 三层节奏

| 层 | 触发 | 动作 | 成本 |
| --- | --- | --- | --- |
| **内循环（只编译拿诊断）** | 改完 `.uvue` / `.uts`，**先确认没有编译期诊断** | `npm run hx:compile-only`（= `hx-run.ps1 -CompileOnly`：官方语义的「仅编译」，**不推送 / 不启动 / 不轮询 / 不需要设备**，机检行 `mode=compile-only`） | **热缓存 257 秒**（2026-09-14 实测，落在旧口径 205–261 秒内 ✅）；**冷 / 上次被打断后 826–901+ 秒**（同日两次实测）—— **勿再引用「约 3–4 分钟」当常态** |
| **内循环（热刷新）** | 只改模板 / 样式，想看真机效果 | HBuilderX「运行到手机」+ 热刷新；**不勾干净缓存重建、不重装基座** | 秒级～1 分钟 |
| **内循环（真运行到真机）** | 要在真机上看行为 / 收口取证前的部署 | `npm run hx:run`（**真运行**：无干净缓存重建、不重装基座、不做 `adb install`；判定靠**设备侧事实相对基线前进**） | 约 4–5 分钟（实测编译 205–261 秒 + 推送 15–21 秒） |
| **中循环** | **每个「视觉满意点」一次**（不是每次微调） | `npm run test:unit` + ④ 本地编译门（`npm run build:kotlin-all` 够用；必要时 `npm run build:compile` 全量）→ **一次提交** | 分钟级 |
| **外循环** | **主题收口一次** | ①a 真机自动取证（`adb` 只读逐页截图 + logcat，**不抢焦点**）→ 开 PR；证据 sha 对齐 | 一次 |

**提交粒度**：**一个视觉主题一个 commit**。色值微调与结构删除**不要混在一个提交里**——反例 `56d0fe3`（本地分支 `feat/ai-basic-ui-prototype`，+21 / −71）：「背景实色兜底」与「去掉小程序 chrome 胶囊」压在一起，回滚无从下手。

**时间账、机检行与「真运行会话常驻但会被下一次 launch 顶掉」的实测原文**，在按需层
[`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)「时间账与三样慢」「真运行会话常驻与收口」。
改完样式要一行结论 ⇒ `npm run style:loop`（实测原文在同文件「样式内循环：一条命令、一行结论」）。
结论与对策：**慢的从来不是编译，而是返工 / 排队 / 卡死** —— 返工用 `hx:compile-only` 拦在本地、
排队的取锁上限默认 120 秒、卡死靠部署停滞 300 秒提前判环境不可用（`-DeployStallSeconds`）。

### 并发会话纪律（多会话并行时必读）

**HBuilderX 是单实例串行资源** ⇒ 「真机自动化」被钉成**不可并发**：多个会话同时跑 `hx-run` / 编译门 /
截图只会**排队**（`scripts/lib/hx-busy.ps1` 的锁 + 忙探测，日志打 `HX_BUSY wait=<秒> result=free|timeout`），
到等待上限即 `exit 2` 判**环境不可用** —— 这是**架构约束，不是可以调优的缺陷**；忙就改跑不需要
HBuilderX 的检查（`npm run test:unit` / `npm run build:kotlin-all`），**绝不 kill 主程序、绝不抢占**。

**职责边界**：真机步骤（`dev:finish` 的 4–6）**集中到一个会话**；其余会话只做到编译门为止（🟢 Q-A 默认路径：契约测试 + 静态守护，秒级、不取锁、不占设备）。

分步的取锁上限（`hx-run` 120 秒 / `dev:finish` 1800 秒 / `build:*` 600 秒）、锁的交接机制（`$env:HX_LOCK_OWNER`）、「**先写代码、后集中上机**」的攒批口径与四条时间预算，全在按需层 [`docs/agents/concurrency.md`](docs/agents/concurrency.md)。
判据与守护：`docs/adr/0011-一键完成的并发与失败语义.md`、`utils/hxBusyGateContract.test.js` H1–H11、
`utils/devFinishContract.test.js` F6–F10。

### Qoder 托管 worktree 的使用边界（2026-09-28 立）

**一句话**：**改动落在 `training-app/**` 下的活，一律不在宿主树里做**（哪怕只改一个 `.md`）——走
`pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`。本段只声明移动端与小程序的禁令；
宿主树适合干什么由 Web / 后端的负责人定，本段不表态。

判据不是「命中运行时面」，而是「**③ 要不要在这棵树里跑**」：CI 的 `changes.mobile` 过滤器是 `'training-app/**'`
（`ci.yml` 的 `changes` job）⇒ 该路径下**任何**改动都把 ③ 变成必过门。**看的是「你这轮打算动哪些路径」的意图，
不是「`git diff` 现在显不显示东西」**：计划新建 / 修改的任一路径落在 `training-app/**`（**含还没 `git add` 的新文件**）就命中。

⚠️ **别拿开工前的一句 `git diff` 当许可** —— 它在最该报警的时刻恰好返回空：开工前什么都没改、`git diff` 不含未跟踪新文件、`origin/master` 可能陈旧。要机检就连未跟踪一起看，且当作**事后回检**（确认没漏，不是开工前的通行证）：`git status --porcelain | Select-String 'training-app/'` —— 任一行路径以 `training-app/` 开头 ⇒ 本不该在这棵宿主树里做。

三条原因（各一句，实测原文在 [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)「三条原因的实测原文」）：

1. **③ 门在宿主树里静默假绿，且这是永久属性**：宿主落点父段带点 ⇒ jest 的 glob 匹配 0 套件、还 `exit 0` 不报错（#1144），
   而**该落点不可配置**。CI 侧不受此坑 ⇒ 最后一道防线还在，但**本地反馈环失效**。
2. **HBuilderX 项目名冲突不是理由**（2026-09-28 更正）：改成唯一名是所有 worktree 的共同代价，闸门树同样要付。
3. **树会被自动回收**：`worktreeMaxCount` 超限即删最旧（**包括别的编辑器正开着的那一棵**），且 `git worktree remove`
   连带删掉 gitignored 的 `.ci-verify/`（原始判据输入，删了不可复算）。

**回退与复用**：宿主树是普通 git worktree ⇒ 提交并推了分支就能回退；反过来**没提交的改动与 gitignored 证据，树没了就没了** ⇒ 进树第一件事建分支、尽早推 origin，判据输入当场拷到树外或直接入 `docs/verification/<模块>/<PR号>/`。
**初始化单一真源** = `pwsh training-app/叉车维修培训学员端跨端应用/scripts/wt-bootstrap.ps1`；宿主「本地任务的 Worktree 配置」框
**只填这一行调用**（填逻辑＝开第二真源），现值怎么核对见根 `AGENTS.md` 的命令速查块。
决策论证全文见 `docs/adr/0029-移动端不用宿主托管worktree.md`（**移动端编号体系，须写全路径**；根仓库另有同名的 `ADR-0029`）。

### 反模式（逐条禁）

- **在真机上验「本该编译期就红」的东西**——类型名义不一致（`ClassCastException`）、uvue 样式规则违反、模板编译错误
  **都是编译期诊断**：先 `npm run hx:compile-only`（**不碰设备**），别让它变成一次 5 分钟的真机返工。
- 每次微调都**全量重编译**——全量是门的活（`compile-check.ps1`），日常走 `hx-run.ps1`。
- 每次微调都**重装基座**——**基座只装一次**；`hx-run.ps1` 里没有任何 `adb install`，装机由 HBuilderX 自己处理。
- 每次微调都**跑全量门 + 提交**——门按中循环跑（每个视觉满意点一次），提交按「**一个视觉主题一个 commit**」。
- **kill `cli` 或 HBuilderX 主程序**——违反「HBuilderX 是单实例串行资源」：**忙就等**
  （`scripts/lib/hx-busy.ps1` 的锁 + 忙探测 + 等待上限），超时 `exit 2` 并改跑不需要 HBuilderX 的检查
  （`npm run test:unit` / `build:kotlin-all -SkipPublish` / `smoke:emulator`）。
- 拿**仿真机当热刷新用**——仿真机是**前置冒烟**（`npm run smoke:emulator`，非门、不替代 ①），
  装一次 SDK 3–4 GB / 20–40 分钟，不适合秒级迭代。
- **成本账反模式两条**（sha 无害前进就重跑整道门 / ② 失败后不带 `-SkipBuild` 重试）：**判据与实测数字在**
  [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)「成本账反模式（2026-10-03 立）」。一句话处置：贴证据前现测一次
  `git diff --name-only <旧证据sha> <head>` 看改动集，判据不满足才重跑（**「我觉得没改」不算判据**）；
  产物没变时 ② 第二次起重试带 `-SkipBuild`。
- **跑完 HBuilderX 步骤不查工作树**：`manifest.json` 会被回写置空 appid，`pages.json` 会被写入一段 `condition`
  （GUI 选的启动页，**属本地开发配置、禁止提交**）——两者都是「运行时面 / 打包面」判据来源，误提交会让 PR
  **凭空命中 ④b 云打包门**。⇒ 每轮先 `git status`，脏了还原再继续。**删 worktree / 给项目目录改名之前**先
  `cli project close --path <项目>` 结束常驻会话，否则目录既删不掉也改不了名（`hx:compile-only` 无此副作用）。

## 验收门与合并纪律（ADR-0008）

改动触及运行时面（改动集含 `*.uvue` / `*.uts`，或 training-app 下的 `manifest.json` / `pages.json` / `platformConfig.json`）时，适用 `docs/adr/0008-移动端验收门与证据.md` 的四门与证据要求。

- **签收在人、合并不限人**：把 PR 正文的 `## 验收证据` 段填齐（每门四字段 + 产物），人工门（**①b**）的「执行人」栏由**人**给出原文（agent 可代录、不得自拟）；填齐后 **agent 可直接合并**，不必停在「待人工签收」。**「不必停在」不等于「还要问」**——这一条是**授权**：证据齐且不走例外通道时，「是否合并」不构成需要维护者拍板的问题（授权全文与合并前一行披露 production 部署的口径见根 `docs/agents/release.md`:16）。
- **合并后的收尾是默认动作，不是问题**：「是否清理工作树」不问。顺序是**先 `git worktree remove` → 再 `git branch -D` → 最后删远端**（`release.md`:13 —— 反序会让 `--delete-branch` 静默不删）；摘 `node_modules` junction **必须早于**删目录，且 `git worktree remove` 后要用 `Test-Path` 复验目录真消失（判据与不可逆血账见根 `docs/agents/multi-agent-git.md`）。
- 非运行时面的 PR（纯文档 / 测试 / CI 配置）不受此限，agent 可自行合并。
- **人工门只剩 ①b Android 真机「能力面」冒烟**（2026-09-12 修订：② 已移出人工门清单，见下条；**2026-09-16 修订**：① 拆成 ①a／①b，只有 ①b 由人签）；①b 的「执行人」栏由**人**给出原文（agent 代录，不得自拟、不得写「已通过」）。
- **① 的触发面与取证节奏（2026-09-16 修订）**：**①a**（agent 出证：逐页截图 + logcat + 机检行 + 入仓截图）**按「一次分支收口」跑一次**，不在 PR 内每次微调、**不跨 PR 共用**。**①b**（人签）**只在命中「能力面」时必过**：指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互 —— **不是**「任何运行时面」。① 行的「执行人」栏在该行只覆盖 ①a 时允许写「agent 执行」。能力面可机检口径 = **路径白名单 + PR 模板自报勾选**（自报是声明、不是判据，漏报靠 ①a 的 logcat 与评审兜）。**明确不做**：基于 diff 文本 / API 字符串的触发判据、跨 PR 共用真机、「冷态降级 ④」。逐条理由见 `docs/adr/0016-真机门的人工性收缩与按批取证.md`。
- **①a 真机取证的具体手法**（`--pagePath` + `--pageQuery` 深链、点击坐标取 `uiautomator dump` 的 bounds、`input` 可注入性**每次现测**、取证夹具的「人登录一次 + CDP 驱动 + 页面内 fetch」）见 `docs/adr/0008-移动端验收门与证据.md` 的「①a 取证手法补遗（2026-09-15 实测）」。**多会话并发时如何不动别人的工作树就提交 / 同步 master** 见 `docs/agents/multi-agent-git.md` 的「游离提交：完整配方」。
- **② 微信开发者工具门 = 半自动门（2026-09-12，#883 收口）**：agent 可执行 `npm run build:mp-weixin-check`（= `scripts/mp-weixin-check.ps1`）产出 `MP_WEIXIN_RESULT` + `.ci-verify/mp-weixin.log` + `.ci-verify/*.png`，**结果可由脚本 `-PostToPr <PR号>` 贴成 sha 绑定的 PR 评论承载**（正文该行写「见评论 <链接>」即可）。**「执行人」栏仍由人给出原文**（agent 代录，不得自拟）。三条硬前提：**全访问权限执行**（与 HBuilderX 的本地 IPC）、开发者工具**已登录**、项目目录**唯一名**；其余（`hx-busy.ps1` 的锁与忙探测、HBuilderX 回写 `manifest.json` 置空 appid、端口绑 `::` 的暴露面、② ≠ ① ≠ ④b 的证据边界）**原文在** `docs/adr/0008-移动端验收门与证据.md` 的「小程序门（第②门）」「② 门三条硬前提与证据边界」「HBuilderX 是单实例串行资源」三节。**跑完任何 HBuilderX 门先 `git status` 查 `manifest.json` 有没有被改脏**，脏了就 `git checkout -- manifest.json` 再继续。
- **低风险运行时面（仅 `.uts` 逻辑改动，无 `.uvue` / 三份 json / `uni_modules`）免 ① ②**：正文里这两行可以整行不写（写「免（低风险运行时面：仅 .uts 逻辑改动）」也行）；代价是真机 / 渲染类问题推迟到发版前的 ① 全量冒烟兜底。③ 与 ④ 仍必过。
- **③ `npm run test:unit` 门 = 有判据的门（2026-09-18 立，#1156）**：判据三条 —— **①「我故意弄坏被测物，它会不会红？」**（判别力）**②「它测的是该测的那一支吗？」**（条件编译 / 镜像 / 分支选错 ⇒ 测的是另一支）**③「只跑通过的那一次，不算验收」**（成对取证：必不红 / 必红各一条）。「全绿」可恒真的逐条实测证伪、以及「行为守护承重 / 接线守护不构成 ③ 证据」的分类见 [`docs/agents/guards.md`](docs/agents/guards.md)；分类真源 `node scripts/classify-guards.mjs`（机检形状由 `utils/guardClassification.test.js` H1–H5 钉住）。**新增守护**在 PR 正文回答 `guards.md` 末节三问。本条只加判据，**不加人工门、不改 ①②④ 的触发面**。
- **④ 本地编译门（2026-09-11 修订：④a 与 ④c 合并，裁定见 #870 / #859）**：默认载体是 **④c 整模块编译**（`npm run build:kotlin-all` → `.ci-verify/kotlin-all.log`）；**dev 专属面追加 ④a**（`npm run build:compile` → `.ci-verify/build.log`）——命中 `pages.json` / `manifest.json` / `platformConfig.json` 改动、新增页面、模块手术收口 PR 时。「执行人」栏填执行会话所用账号并注明「agent 执行」；结果同样可由 `-PostToPr` 贴成 sha 绑定评论。**两条门脚本都须以全访问权限执行**（受限沙箱下与 HBuilderX 的本地 IPC 报「与主程序的连接已中断」）。
- **④b release 云打包按「打包面」触发（2026-09-11 修订）**：改动 `manifest.json` / `pages.json` / `platformConfig.json`、`uni_modules/**/utssdk/app-android/**`、新增 `*.aar` / `libs/*.jar` 时必填；**正式发版前必须跑一次云打包并装机自测**（发布前置条款）。旧口径「新增 async / 新增 composable 即触发」已废止。
- 例外通道：正文写明「已接受未验证风险 + 理由 + 事后验证计划」，检查会打警告放行，但**仍必须由人执行合并**。禁止静默例外。
- `pr-evidence` 检查只校证据结构、不校真伪；它是**可见检查**而非 ruleset 必检——本仓**有可用的 admin 通道**（维护者持有仓库所有者账号），但**裁定不装**（逐 PR 审批成本高于约束收益，见 ADR-0008「为何不装『必检 + approve』」）。

## 发布流程（push / PR / merge）

**权威版本在根** [`docs/agents/release.md`](../../docs/agents/release.md)：分支 + PR + ruleset 门禁（protect master 拒直推、`ci-summary` 必检且要求分支 up-to-date）+ squash 直发 production 的七步、CI 触发模型（分支 push = CI + testing 冒烟，PR 事件不触发任何 CI/CD）、应急通道，全在那一份里。本处不抄——抄过就漂（本线这段此前正是根某次拆分留下的残骸）。

移动端只补两条：

- PR 正文的 `## 验收证据` 段结构见上节；四门的证据绑定与「签齐即 agent 直接合并」见 `docs/adr/0008-移动端验收门与证据.md`。

- ⚠️ git/gh 的写操作一律**后台跑并等它自然结束**，用 `timeout` 包它们会把操作**截在执行到一半**（比失败更糟，事故全文见根 `docs/agents/release.md`）。

## 代码规范

- 命名：文件 kebab-case、组件名 PascalCase；导入顺序 `@vue/*` → uni-app API → 第三方库 → 本地模块。
- 列表渲染：`v-for` 必须带 `key`，**避免用 `index` 当 key**（旧「常见问题·性能优化」段唯一被外部引用的承重条，`docs/adr/0025` 引它）。
- props / emit 优先，跨层才用 provide/inject 或 Pinia；平台差异走条件编译 `#ifdef` / `#ifndef`，样式用 rpx、触摸目标不小于 44×44。

## 注意事项

1. **每次改动完成后都创建一个对应的 git commit**，便于追踪与回滚。
2. **每次改动后都编写或更新相关测试**，并在回复用户之前确保所有测试与验证通过。
3. **避免 `setTimeout` / `setInterval` 这类容易造成内存泄漏的裸用法**，需要定时器时挂到 uni-app 生命周期上管理。

## 相关文档

- **移动端 ADR（独立编号体系，与根仓库 `docs/adr/` 的 `ADR-0001+` 互不相关，引用须写全路径）**：目录 `docs/adr/`。本文件直接依赖的几条：`0008` 四门判据与取证补遗、`0011` 并发与失败语义、`0016` 真机门收缩与按批取证、`0020` E2E 通道裁决、`0024` 技能供给与管线归属、`0029` 不用宿主托管 worktree、`0033` 常驻与按需分界。其余按目录逐个标题查。
- **按需层（常驻层指向它们，不反向抄）**：`docs/agents/guards.md`（守护分类与新增守护三问）、`docs/agents/issue-dispatch.md`（派活一行）、`docs/agents/uvue-css.md`（uvue 样式与 UTS 编译约束全表）、`docs/agents/concurrency.md`（并发纪律与时间预算）、`docs/agents/dev-loop.md`（内循环时间账与坑位原文、样式内循环一行结论）、`docs/agents/skills.md`（技能供给与管线归属）。
- **其余**：`docs/GIT_WORKFLOW.md`、`docs/GIT_CHEATSHEET.md`、`docs/ui-spec.md`、`docs/product-design.md`、`docs/refactor-decisions.md`、`docs/spec-永绿整改.md`。
