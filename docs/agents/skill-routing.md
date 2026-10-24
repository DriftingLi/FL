# 技能路由表（意图 → 唯一默认入口）

**适用面**：本仓**所有**会话 —— Web 前端（`frontend/`）、后端（`backend/`）、部署面、移动端（`training-app/叉车维修培训学员端跨端应用/`）。

**这份文件管的是「选择面」**（同一个意图该叫哪个技能），**不管「供给面」**（技能文件进不进库、装在哪个扫描根、改动怎么成 PR）——后者见移动端 `docs/agents/skills.md` 与移动端 `docs/adr/0024-技能供给与管线归属.md`。两份不重叠，不要互相抄。

## 为什么需要这张表（判据，逐项实测）

- **技能根现测（2026-10-24 清理后）**：Matt Pocock 集**只装一份**——全局 `C:\Users\ZHENG\.agents\skills\`（上游 `mattpocock/skills` 本体，`~/.agents/.skill-lock.json` 可验）；`~/.qoder/skills\` 的甲板旧快照 24 个同名重复已删（余 README 与 `obsidian-official-cli`、`resolving-merge-conflicts` 两个非重复目录）。技能根装在**用户级** ⇒ 对本机所有会话生效，**与仓里写不写约定无关**。
- **默认入口必须落在常驻文件里**：技能触发词让位于用户指令（`AGENTS.md` 等），而按需读的 `docs/agents/*.md` 只能作补充细则——这就是本文件与根 `AGENTS.md`「Skill routing」段分两半的原因：**摘要常驻、全表在案**。（superpowers「每次会话注入、只有 AGENTS.md 压得住」的那段旧理据已随其退场删除，历史实测见下条与移动端语义负担文档。）
- **全局根 51 个技能目录里 24 个带 `disable-model-invocation: true`**（2026-10-24 现测；含目录式路由器 `ask-matt` 自己）⇒ 它们不进模型的自主清单，只有人能 `/` 唤起；「谁存在、何时用」这笔认知负担没有自动出口。
- **用量实测**（`C:\Users\ZHENG\.qoder\skill-usage.json`，用户级、跨项目累计，无项目维度）：`code-review` 9、`superpowers:brainstorming` 2、`superpowers:systematic-debugging` 1、`codebase-design` 1、`research` 1 —— 调试意图走的是 superpowers 那份而 `diagnosing-bugs` 零使用，说明**竞争已经打过、Matt 侧输了**，本表是事后裁决而非预防。

完整的语义负担测量、提问形态清点与三套技能的重叠矩阵见
`training-app/叉车维修培训学员端跨端应用/docs/skill-语义负担与路由裁决-2026-10-03.md`（§②–§⑨）。

## 表

规则：**本表列出的意图以本表为准，技能自身描述里的触发词冲突时让位**；表未覆盖的意图可自主选择。

| 意图 | 默认入口 | 什么时候换另一个 |
| --- | --- | --- |
| 需求/目标不清，要澄清 | `grilling`（人唤） | 要出书面问卷 → `to-questionnaire` |
| 把结论写成 spec / 拆成票 | `to-spec` → `to-tickets` | 无（移动端另有「会话切分约定」，见移动端 `AGENTS.md`，本表不改它） |
| 执行已拆好的票 | 移动端票 = `training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md` 一行派活；根侧票 = `docs/agents/release.md` 七步 + DoD | 可用 `implement`（先取票报题、自带 `tdd`/`code-review`），但其「Commit your work to the current branch」让位 `docs/agents/multi-agent-git.md`；**`implement-spec` 禁用于本仓**——integration branch + draft PR 与「开 PR 的时机」及证据主键（`release.md`）正面冲突。本地文件形拆票（`plan-feature`→`execute-task`，落 `docs/plans/<feature>/`、本地不入库）存量一处，新拆票默认 `to-spec`→`to-tickets` 上 GitHub |
| 调试疑难、报错归因 | `diagnosing-bugs`（它读 `CONTEXT.md` 与所属侧 ADR） | 无 |
| 测试先行实现行为 | `tdd` | 无 |
| 审「是否符合本仓规范与 spec」 | `code-review`（显式给固定点） | 对实现做穷举式缺陷扫描 → `review-code`（六轮） |
| 领域词汇 / ADR | `domain-modeling` + `docs/agents/domain.md` | ADR 文件名一律中文（本条在册裁定压过技能的英文命名习惯） |
| 建隔离工作树 | **只走 `training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`** | **禁止**裸用 `git worktree add`（含任何技能自带的兜底路径）—— 见根 `AGENTS.md`「建 worktree 一律走闸门」与移动端 `AGENTS.md`「Qoder 托管 worktree 的使用边界」 |
| 临时产物 / handoff 落点 | `.scratch/` | 压过 `handoff` 技能文档里「存 OS 临时目录、不进工作区」那条 |
| 不知道用哪个技能 | 先查本表 | 表里没有 → `ask-matt`（它带「仅人唤」标记，模型永远不会自己想起它） |

## 提问形态：本表对重型技能的落点做了改写

`grilling` 的现行版（上游 PR #917 起）要求「**一轮把整条 frontier 全问出来，然后等**」，`to-tickets` 要求「三问 + **迭代到点头**」，这与 superpowers 的「一次只问一个」正面冲突，也与本仓维护者 2026-09-23 定下的口径（**最多留一件要人拍板的事、用散文推进、不用问题组件**）冲突。

⇒ **执行口径**：这类多问技能的产出改写成**一次性书面产物**（`grilling` 每轮 → `.scratch/<票号>/grilling-round-N.md`；`to-tickets` 的三问 → 一份 ≤25 行的粒度评审表），散文里只留「拍板 / 不拍板」一件。**不改技能文件本身** —— 那些文件与上游甲板字节一致，改一行就没有工具会报警。

## 三条边界（避免与既有裁定相撞）

- **本文件不承载移动端那四条供给约定**，也不复制它们 ⇒ 移动端 `docs/agents/skills.md` 里「不新建根 `docs/agents/skills.md`、不在根 `AGENTS.md` 加指针」那条裁定约束的是**供给四条进入所有会话的常驻上下文**，本文件是选择面，不在其射程内。
- **引用技能文档一律引小节名、不引行号**（沿用移动端 `docs/agents/skills.md` 的第 4 条约定）：同一份技能存在多版本快照，行号必然漂移。
- **`setup-matt-pocock-skills` 不再跑**：`docs/agents/{issue-tracker,triage-labels,domain}.md` 含承重本地漂移（`ready-for-mobile-agent` 变体、`CONTEXT.md` 双份布局、重复票动作化处置），重跑会被 seed 模板覆盖。此前警示只在用户级 `~/.qoder/skills/README.md`，仓内无防线，故入仓。
