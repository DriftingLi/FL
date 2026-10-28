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

- **一条思考链一个会话**：grilling → to-spec → to-tickets 连续完成，中途不 clear/compact——被否决的分支与否决理由也是 spec 的依据。
- **一件票一个会话**：每票冷启动，只读票 + 对应 ADR + `docs/refactor-decisions.md`；票间 `/clear`。
- **换气只在阶段边界**：接近 smart zone（约 150k）用 `/compact`；跨目录 / 跨工具 / 会话中途分叉才用 `/handoff`。
- **收束与执行分座、重构与 UI 分时**：回填结论/补复盘另开短会话；同一模块内 UI 对齐 PR 排在手术合并之后。

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

- **本线唯一在跑的验证层 = ③ 门两腿**（#1702 拆分，来路见 [`docs/adr/0042-测试门拆腿与逐套件心跳.md`](docs/adr/0042-测试门拆腿与逐套件心跳.md)）：`npm run test:unit`（**接线腿**，`jest.config.unit.js`）+ `npm run test:behavior`（**行为腿**，`jest.config.behavior.js`），各扫一半 `utils/**/*.test.[jt]s?(x)`，`npm test` 两腿顺序跑。只跑一个：`npx jest --config <该套件所在腿的 config> <相对路径>`（`-t "片段"` 按名）——**选错腿的 config 会让 jest 当那个套件不存在（0 套件，长得像绿）**，归腿问 `node scripts/classify-guards.mjs`。判据与守护分类见 ③ 条；**跑完只带结果行进上下文**（`TEST_UNIT_RESULT` / `TEST_BEHAVIOR_RESULT`，整日志留 `.ci-verify/*.log`，读法见 [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)「agent 判读纪律」）。
- ⚠️ **本仓没有 `type-check` 脚本** —— 那是 Web 前端栈的入口，别照搬。后端四件套（含 Windows 本机 golangci-lint 由 CI 兜底、WSL 例外）、生成链顺序（swagger → gen-apitypes → 再跑测试）、PG 契约测试纪律、部署配置校验、DeepSec 扫描的**权威版本在根** [`docs/agents/checks.md`](../../docs/agents/checks.md)，本处不抄。
- **端到端（E2E）自动化本仓不做，别再按 uni-app 项目模板去接**：微信小程序通道只能做 page 级断言、做不了「点击 → 跳转 → 断言」的流程验证（PR #1136 裁决），H5 通道在 `require` 阶段即炸 ⇒ 本仓**没有** E2E 入口与装配。裁决、实测与**将来重启的配方**见 `docs/adr/0020-端到端测试通道裁决落锁与装配清退.md`；补强验证层的正确方向是**把镜像实现迁到 `utils/utsHarness.js` 真执行**（`docs/spec-永绿整改.md` §④ S3 / S6a–S6c），不是引入 E2E。

## 开发内循环（移动端 UI 迭代）

> 口径：**门是全量的，日常是分层的**。`scripts/compile-check.ps1`（④a，945 秒 #1989；旧值已退役）是**门**，默认值不动；
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
| **内循环（只编译拿诊断）** | 改完 `.uvue` / `.uts`，**先确认没有编译期诊断** | `npm run hx:compile-only`（= `hx-run.ps1 -CompileOnly`：官方语义的「仅编译」，**不推送 / 不启动 / 不轮询 / 不需要设备**，机检行 `mode=compile-only`） | **热缓存 257 秒**（2026-09-14 实测）；**冷 / 上次被打断后 826–901+ 秒**（同日两次实测）—— **勿再引用「约 3–4 分钟」当常态** |
| **内循环（热刷新）** | 只改模板 / 样式，想看真机效果 | HBuilderX「运行到手机」+ 热刷新；**不勾干净缓存重建、不重装基座** | 秒级～1 分钟 |
| **内循环（真运行到真机）** | 要在真机上看行为 / 收口取证前的部署 | `npm run hx:run`（**真运行**：无干净缓存重建、不重装基座、不做 `adb install`；判定靠**设备侧事实相对基线前进**） | 中位 73 秒 |
| **中循环** | **每个「视觉满意点」一次**（非每次微调） | `npm run test:unit:changed`（接线全量 + 行为腿子集，**不当 PR 证据**）+ ④ 本地编译门（`npm run build:kotlin-all` 够用；`build:compile` 全量）→ **一次提交** | 分钟级 |
| **外循环** | **主题收口一次** | ①a 真机自动取证（`adb` 只读逐页截图 + logcat，**不抢焦点**）→ 开 PR；证据 sha 对齐 | 一次 |

**提交粒度**：**一个视觉主题一个 commit**。色值微调与结构删除**不要混在一个提交里**——反例 `56d0fe3`（+21 / −71）：「背景实色兜底」与「去掉小程序 chrome 胶囊」压在一起，回滚无从下手。

**时间账、机检行、样式一行结论与对策**（慢的是**返工 / 排队 / 卡死**，不是编译：返工用 `hx:compile-only` 拦在本地）全在按需层 [`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)「时间账与三样慢」「真运行会话常驻与收口」「样式内循环」——改完样式要一行结论：`npm run style:loop`。

### 并发会话纪律（多会话并行时必读）

**HBuilderX 是单实例串行资源** ⇒ 真机自动化**不可并发**：多会话同时跑 `hx-run` / 编译门 / 截图只会**排队**（`scripts/lib/hx-busy.ps1` 的锁与忙探测，日志 `HX_BUSY wait=<秒>`），到取锁上限即 `exit 2` 判**环境不可用**——**这是架构约束，不是可调优的缺陷**；忙就改跑不需要 HBuilderX 的检查（③ 门两腿 `npm run test:unit` / `test:behavior`、`npm run build:kotlin-all`），**绝不 kill 主程序、绝不抢占**。**职责边界**：真机步骤（`dev:finish` 的 4–6）**集中到一个会话**，其余会话只做到编译门为止。取锁上限（120/1800/600 秒）、锁交接（`$env:HX_LOCK_OWNER`）、「先写代码、后集中上机」攒批口径与时间预算在按需层 [`docs/agents/concurrency.md`](docs/agents/concurrency.md)；判据与守护见 `docs/adr/0011-一键完成的并发与失败语义.md`、`utils/hxBusyGateContract.test.js` H1–H11、`utils/devFinishContract.test.js` F6–F10。

### Qoder 托管 worktree 的使用边界（2026-09-28 立）

**一句话**：**改动落在 `training-app/**` 下的活，一律不在宿主树里做**（哪怕只改一个 `.md`）——走
`pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`。看的是**这轮打算动哪些路径的意图**
（含还没 `git add` 的新文件），不是 `git diff` 现在显不显示；事后回检：`git status --porcelain | Select-String 'training-app/'`。

三条原因（③ 门在宿主树静默假绿 #1144 / 项目名冲突不是理由 / 树被自动回收连删 `.ci-verify/`）实测原文在
[`docs/agents/dev-loop.md`](docs/agents/dev-loop.md)「三条原因的实测原文」。**回退与复用**：进树第一件事建分支、
尽早推 origin；没提交的改动与 gitignored 证据树没了就没了，判据输入当场拷到树外或入 `docs/verification/<模块>/<票号>/`（#1770 起归档目录键 = 票号；旧 PR 号目录仍被接受）。
**初始化单一真源** = `pwsh training-app/叉车维修培训学员端跨端应用/scripts/wt-bootstrap.ps1`；宿主「本地任务的
Worktree 配置」框**只填这一行调用**（填逻辑＝开第二真源），现值核对：读回 `%APPDATA%\Qoder\logs\<run>\questWindow\renderer.log`
里 `worktreeSetup` 键的值与之比对（**别拿「应该已填」当作已填**）。决策论证全文见
`docs/adr/0029-移动端不用宿主托管worktree.md`（**移动端编号体系，须写全路径**；根仓库另有同名 `ADR-0029`）。

### 反模式（逐条禁）

- **在真机上验「本该编译期就红」的东西**——类型名义不一致（`ClassCastException`）、uvue 样式规则违反、模板编译错误
  **都是编译期诊断**：先 `npm run hx:compile-only`（**不碰设备**），别让它变成一次 5 分钟的真机返工。
- 每次微调都**全量重编译 / 重装基座 / 跑全量门 + 提交**——全量是门的活（`compile-check.ps1`），**基座只装一次**（`hx-run.ps1` 无 `adb install`）。
- **kill `cli` 或 HBuilderX 主程序**——违反「HBuilderX 是单实例串行资源」：**忙就等**
  （`scripts/lib/hx-busy.ps1` 的锁 + 忙探测 + 等待上限），超时 `exit 2` 并改跑不需要 HBuilderX 的检查
  （③ 门两腿 `npm run test:unit` / `test:behavior` / `build:kotlin-all -SkipPublish` / `smoke:emulator`）。
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

改动触及运行时面（改动集含 `*.uvue` / `*.uts`，或 training-app 下的 `manifest.json` / `pages.json` / `platformConfig.json`）时，适用下列五道门——**判据与原因全文在** `docs/adr/0008-移动端验收门与证据.md`（**须写全路径**，根仓库另有同名 `ADR-0008`），本节只留执行要点。

- **①a 真机逐页取证已降级为发版前一次**（ADR-0043）：运行时面 PR 的 ① 行固定写 `免（①a 降级发版前一次，ADR-0043）`（执行人/日期照填、复测对象可写「无」）；取证手法（发版冒烟用）见 ADR-0008「取证手法补遗」与 0043。
- **真机人工签收**：**只在命中「能力面」时必过**——指纹 / 运行时权限弹窗 / 真机上传 / 厂商 ROM 交互，**不是**「任何运行时面」；「执行人」栏由**人**给出原文（agent 代录、不得自拟、不得写「已通过」）。触发口径与**明确不做**清单（diff 文本触发、跨 PR 共用真机等）见 `docs/adr/0016-真机门的人工性收缩与按批取证.md`。
- **小程序门（② 微信开发者工具门 = 半自动门，#883 收口）**：`npm run build:mp-weixin-check` → `MP_WEIXIN_RESULT` + `.ci-verify/*.png`，结果以 `-PostToIssue <票号>` 贴 sha 绑定评论（判据/展示落点与 404 降级判据见「签齐即合并」；正文该行写「见评论 <链接>」）。**每个运行时面 PR 的常态主门、执行人允许「agent 执行」**（ADR-0043 决策 3；先例 #1674）。三条硬前提与证据边界原文在 ADR-0008「小程序门」「三条硬前提与证据边界」两节；**贴证前两核**见 `docs/agents/dev-loop.md` 同名节。
- **单元测试门**（#1702 起**两腿**：`npm run test:unit` 接线 + `npm run test:behavior` 行为；拆分轴就是守护分类本身——behavior ⇒ 行为腿、wiring ⇒ 接线腿，两腿 `testMatch` 由 `scripts/lib/guard-split.js` 从同一份分类算出，**不是第二份手写清单**；两腿并集才是全量，`utils/jestSplitContract.test.js` P1–P6 钉这个形态）：判据三条（**只跑通过的那一次不算验收**，成对取证）与守护分类（**行为守护承重、接线守护不构成证据**）见 [`docs/agents/guards.md`](docs/agents/guards.md)；分类真源 `node scripts/classify-guards.mjs`（`utils/guardClassification.test.js` H1–H5 钉形）。**新增守护**在 PR 正文答 `guards.md` 末节三问；本条只加判据，**不加人工门、不改其他门触发面**。
- **编译门**：默认 `npm run build:kotlin-all` → `.ci-verify/kotlin-all.log`（执行人注明「agent 执行」、可 `-PostToIssue <票号>` 贴 commit 判据评论）；命中三份 json 改动、新增页面、模块手术收口 PR 时追加 `npm run build:compile`。**云打包按「打包面」触发**（三份 json / `uni_modules` app-android / 新增 aar·jar），**正式发版前必跑云打包并装机自测**。两条门脚本**须以全访问权限执行**。
- **低风险分层已退役**（ADR-0043 决策 8）：纯 `.uts` 不再免 ①（走 ①a 免行）、不再因「低风险」免 ②——② 触发面命中即必跑；真机/渲染问题由发版冒烟分级人看兜底。**③ 与 ④ 照旧必过**。
- **门证据管家**（`.github/workflows/mobile-gates.yml`，2026-10-16 立）：PR 上**一条常驻评论**自动回答「②④ 的证据是不是绑当前 head」，判据与 `pr-evidence` **同源**（同一份 `.github/scripts/lib/gate-validator.mjs`：两个工作流都不 checkout，用 `repos.getContent` 取模块文本执行，不另算口径；**候选同源**：#1841 起含票评论）；**不判红、不进 ruleset、不代贴证据**。真门仍在**本机 / 真机**执行——托管 runner 上两道门卡在账号登录态（DCloud `Please Login in!` / 微信 `islogin` ENOENT），已实测证伪，见 `docs/adr/0041-三层门分离与云上证据管家.md` 决策 1。`-PostToIssue` 贴完证据评论管家会自动刷新，**不必再手动触发**；分层口径（云上纯逻辑 / 本机运行时门 / 真机取证，**勿与 ② 门 `-Doctor` 的 L1–L7 混淆**）见 `0041` 决策 4。
- **③ 门在 CI 是 unit 腿 ×2 + behavior 腿 ×4 = 6 个子 job**（#1890；`0041` 决策 2 的分片 + `#1702` 的拆腿；腿→config 与各腿总片数的映射只在 `ci.yml` 的 `include:` 一处，unit 片集由顶层积 exclude 摘 3/4）：分的是**机器**、每个子 job 内部仍 `-i` 串行（`jest.config.unit.js` 文件头的 #1410 结论不变）；本地内循环仍按 `0036` 处置一走 `test:unit:local`，CI 快慢不改变本地判读纪律。**CI 里别写 `--shard=k/4`**：jest **27.5.1 没有这个选项**，未知参数被**静默忽略** ⇒ 每片照跑本腿全量（2026-10-16 实测）；正确做法是 `--listTests` 取**本腿**清单 + `scripts/jest-fanout.mjs` 按耗时 LPT 装箱（权重表 `utils/jestSuiteTimings.json`，重采脚本 `scripts/gen-jest-timings.mjs`）+ `--runTestsByPath`，切到 0 个套件即红（**#1715 起不再按 `NR%4` 轮转**：不看单套件耗时）。
- **①a 降级常态化（ADR-0043）**：真机取证降级为**发版前一次**（发版冒烟），窗口机制（`0039`/`0040`）整体退役、只作历史；② 升为每个运行时面 PR 的常态主门；低风险分层退役（见上条）；**能力面 PR 完成 ①b 人签即可合并**、不再排队（0040 决策 5 被 0043 改写）；视觉回归仍记「无人验」，落点 = 发版冒烟分级人看。
- **签齐即合并**：**证据主键是 sha 绑定的门评论（#1770；#1841 起含降级常态）**（判据贴 commit；**仅**本仓 commit 评论 POST 回 HTTP 404 才降级：把**同一份正文**（marker 与 `commit: <sha>` 不变）贴 PR 正文关联的票当判据评论、打印落点 URL，展示副本照旧，判据与 commit 评论**同构、不放宽**）；证据填齐后 **agent 直接合并**，不必停在「待人工签收」——**「不必停在」不等于「还要问」**：证据齐且不走例外通道时，「是否合并 / 是否清理工作树」**不是问题**，直接执行（授权全文与合并前一行披露 production 部署口径见根 `docs/agents/release.md`:16）。合并后收尾顺序见根 `docs/agents/release.md`「收尾」段；拆树只走 `scripts/remove-worktree.ps1 -Task <n>`（先摘 junction、删后校验）。非运行时面的 PR（纯文档 / 测试 / CI 配置）agent 可自行合并。**例外通道**（「已接受未验证风险 + 理由 + 事后验证计划」）**仍必须由人执行合并**，禁止静默例外。
- **批收口先答「减法仪式」再合**（#1729 立）：每批 UI 收口 PR（`docs/ui-spec.md` §11）正文必须回答「**本批有无该消的值？**」——逐条点名该消的 `utils/uiHexWhitelist.js` 条目并说明为何可消，或写「无」（答了才许合，答「无」也是答）；棘轮只锁增长不锁消退，批 1→6 六批全净增、无一净减；「何时算死」见 ADR-0038 决策 3（白名单值在受管页并集里 grep 不到即为死条目）。
- 多会话并发下不动别人工作树就提交 / 同步 master 的配方见 `docs/agents/multi-agent-git.md`「游离提交：完整配方」。`pr-evidence` 检查**只校结构、不校真伪**，是可见检查而非 ruleset 必检（admin 通道**裁定不装**，见 ADR-0008）。

## 发布流程（push / PR / merge）

**权威版本在根** [`docs/agents/release.md`](../../docs/agents/release.md)：分支 + PR + ruleset 门禁（protect master 拒直推、`ci-summary` 必检且要求分支 up-to-date）+ squash 直发 production 的七步、CI 触发模型（分支 push = CI + testing 冒烟，PR 事件不触发任何 CI/CD）、应急通道，全在那一份里。本处不抄——抄过就漂（本线这段此前正是根某次拆分留下的残骸）。

移动端只补两条：

- PR 正文的 `## 验收证据` 段结构见上节；四门的证据绑定与「签齐即 agent 直接合并」见 `docs/adr/0008-移动端验收门与证据.md`。门脚本 `-PostToIssue <票号>` 贴 **commit 判据评论**（404 时改贴**票判据评论**，见上条）+ 票/PR 展示副本（归档 `docs/verification/<模块>/<票号>/`）；**建 PR 前先跑预检绿、再由 `submit-pr.ps1` 建**（首次推送即判绿，见根 `docs/agents/release.md` 第 4 步）。

- ⚠️ git/gh 写操作一律**后台跑、等它自然结束**：`timeout` 会把它截半（比失败更糟），事故全文见根 `docs/agents/release.md`。

## 代码规范

- 命名：文件 kebab-case、组件名 PascalCase；导入顺序 `@vue/*` → uni-app API → 第三方库 → 本地模块。
- 列表渲染：`v-for` 必须带 `key`，**避免用 `index` 当 key**（旧「常见问题·性能优化」段唯一被外部引用的承重条，`docs/adr/0025` 引它）。
- props / emit 优先，跨层才用 provide/inject 或 Pinia；平台差异走条件编译 `#ifdef` / `#ifndef`，样式用 rpx、触摸目标不小于 44×44。
- **api 层禁裸打印整段 response/`data` 的调试 `console.log`**（#1730 立；先例来源票 #1730）：调试输出一律走 `api/request.uts` 的 `sanitizeForLog` 管道（或专门 debug flag 分支），不得绕过脱敏。

## 注意事项

1. **每次改动完成后都创建一个对应的 git commit**，便于追踪与回滚。
2. **每次改动后都编写或更新相关测试**，并在回复用户之前确保所有测试与验证通过。
3. **避免 `setTimeout` / `setInterval` 这类容易造成内存泄漏的裸用法**，需要定时器时挂到 uni-app 生命周期上管理。

## 相关文档

- **移动端 ADR（独立编号体系，与根仓库 `docs/adr/` 的 `ADR-0001+` 互不相关，引用须写全路径）**：目录 `docs/adr/`。本文件直接依赖的几条：`0008` 四门判据与取证补遗、`0011` 并发与失败语义、`0016` 真机门收缩与按批取证、`0020` E2E 通道裁决、`0024` 技能供给与管线归属、`0029` 不用宿主托管 worktree、`0033` 常驻与按需分界、`0036` 工具链交付慢的三条处置、`0039` 小程序优先窗口、`0040` 10-15 收紧裁定（真机零跑 / 能力面 PR 排队到退出后）、`0041` 三层门分离与云上证据管家、`0042` 测试门拆腿与逐套件心跳、`0043` 真机门降级常态化与发版冒烟。其余按目录逐个标题查。
- **按需层（常驻层指向它们，不反向抄）**：`docs/agents/guards.md`（守护分类与新增守护三问）、`docs/agents/issue-dispatch.md`（派活一行）、`docs/agents/uvue-css.md`（uvue 样式与 UTS 编译约束全表）、`docs/agents/concurrency.md`（并发纪律与时间预算）、`docs/agents/dev-loop.md`（内循环时间账与坑位原文、样式内循环一行结论）、`docs/agents/skills.md`（技能供给与管线归属）。
- **其余**：`docs/GIT_WORKFLOW.md`、`docs/GIT_CHEATSHEET.md`、`docs/ui-spec.md`、`docs/product-design.md`、`docs/refactor-decisions.md`、`docs/spec-永绿整改.md`。