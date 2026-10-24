# 派活体系与 Matt Pocock skills 冲突调研（2026-10-24）

## 0. 结论（先行）

1. **总体不冲突，局部真冲突。** 本仓的派活前半链（intent 路由、issue triage、拆票）**本来就是用 Matt Pocock 的 `setup-matt-pocock-skills` scaffold 出来的**，两边同构互补；多数表面冲突（多问迭代、handoff 落点、label 变体、裸建 worktree）已被 `docs/agents/skill-routing.md` / `triage-labels.md` 的显式裁定压住。
2. **真冲突集中在交付后半链，且有一处无任何压制条款的正面相撞**：`implement-spec`（上游自述「整 spec 落单一 integration branch」「首次 merge 后开 draft PR 边做边填」）与 `implement`（「Commit your work to the current branch」）对本仓两条铁律——「一个 PR = 一件做完且验完的事，不得当进度容器」（`docs/agents/release.md:19-21`）与「证据 sha 绑定先于 PR」（`release.md:30`、`AGENTS.md:76`）——技能要求做、本仓明文禁止做，而**路由表没有「执行票」这一行**去压它（`skill-routing.md` 表未覆盖 ⇒ 可自主选择）。
3. **最大差异点（单一）**：**交付单元与 PR 时机**——Matt 流的交付单元是「spec → integration branch → draft PR 渐进填充」，本仓是「一票一事一 PR、验完才开、证据先出」。详见 §4。
4. **附带发现（AGENTS.md 描述已漂移）**：「Matt Pocock 集在 `~/.qoder/skills`」的归属描述过时——本会话（ZCode 宿主）实际注入的是 `~/.agents/skills` 里**直接来自上游 `mattpocock/skills` 的新版快照**（2026-10-23 更新），qoder 根那份是甲板包旧快照（2026-09-23），两份同名 7 处内容有 diff。详见 §2。

---

## 1. 「派活」的操作性定义与本仓派活链

**操作性定义**：本仓派活 = ① intent→skill 路由 + ② issue triage（ready-for-agent）→ ③ agent 执行（含会话/worktree 隔离）→ ④ 验收门/PR 证据纪律，四段整链。

各段权威落点（均为一手来源）：

| 段 | 权威文件 | 关键句 |
| --- | --- | --- |
| ① 路由 | `AGENTS.md:23`（常驻摘要）+ `docs/agents/skill-routing.md`（全表） | 「本表列出的意图以本表为准，技能自身描述里的触发词冲突时让位」（skill-routing.md:20） |
| ② triage | `docs/agents/triage-labels.md` + `docs/agents/issue-tracker.md` | 五 canonical roles 源自「Label in mattpocock/skills」列（triage-labels.md:5-11）；`ready-for-agent` 移动端变体 `ready-for-mobile-agent`（:9、:13-16） |
| ③ 执行 | 移动端：`training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md`（「派活只需一行」）；根侧：`docs/agents/multi-agent-git.md`（一会话一 worktree 一分支 :15、绝不 `git add -A` :21）+ worktree 闸门（`AGENTS.md:51`） | 认领→查承载面→阶段闸门→进度契约收尾（issue-dispatch.md §1-§4） |
| ④ 验收交付 | `docs/agents/release.md:19-30` + `AGENTS.md:74-83` | DoD 三条（release.md:22）；证据主键 = sha 绑定门评论（release.md:30）；证据齐 agent 直接合并（AGENTS.md:83） |

本仓**明文采纳 Matt 主流程**为执行票规范形态：`docs/agents/issue-tracker.md:43`「本仓的执行票属 main flow 的 `/to-tickets`（`grill-with-docs` → ADR → `/to-spec` → `/to-tickets` → `/implement`），不挂 `wayfinder:*` label」。

## 2. 事实澄清：哪一套才是「Matt Pocock 集」（附带发现）

实测（2026-10-24，本机）：

- **`~/.qoder/skills/`**：存在，26 个技能目录 + `README.md`。来源 = `dsh-mattpocock-skills-deck` 包（包 1.7.22 / bundled v1.2.3），2026-09-23 首装 6 个、后整包扩到全量；README 自述「每个目录拷完用 `diff -r` 验过与源字节一致」「不要改这些文件里的任何一行」（`~/.qoder/skills/README.md`）。仅 Qoder 宿主读取（README：「`~/.qoder/skills/` 只有 Qoder 读 ⇒ 单侧生效」）。
- **`~/.agents/skills/`**：存在，50 个目录，**无 README**；来源见 `~/.agents/.skill-lock.json`：其中 **35 个直接来自上游 `mattpocock/skills`（github.com/mattpocock/skills.git），updatedAt = 2026-10-23**（比 qoder 甲板快照新一个月），另含 stablyai/orca、vercel-labs/skills、wolke/* 等。README（qoder 侧）称它是「DSH 的活根（rank 500）和 Qoder 的活根」双命名空间共享根。
- **两份同名不同版**：`grilling`/`to-tickets`/`ask-matt`/`handoff`/`implement`/`triage`/`setup-matt-pocock-skills` 七个实测 `diff` 内容不同，仅 `to-spec` 字节一致。最硬的分歧：setup 技能 qoder 版写 `CONTEXT.md`/`CONTEXT-MAP.md`，agents（上游）版写 `GLOSSARY.md`/`GLOSSARY-MAP.md` —— 本仓根 `CONTEXT.md` 布局（`AGENTS.md:13`）匹配的是 **qoder 甲板版**。skill-routing.md:43「同一份技能存在多版本快照，行号必然漂移」的既有约定已预见了这一情况。
- **本会话（ZCode 宿主）注入面**：技能清单全部落在 `~/.agents/skills`；**无任何 `superpowers:*` 条目**。superpowers 实体未找到：`~/.qoder/plugins/installed_plugins.json` 仅 better-harness / qoder-create-plugin / security-scan；`~/.qoder/plugins/data/superpowers-qoder-marketplace/` 为空目录；`~/.zcode/cli/plugins/cache/` 无 superpowers。⇒ 「三套并存」对本宿主不成立（仅一套可注入），superpowers 依据 `AGENTS.md:23` / skill-routing.md:9-10 转述（Qoder 宿主 bundled），未找到实体。
- **计数互不一致（均为不同时点快照）**：skill-routing.md:9 记 qoder 根「25 个目录」，实测 26；qoder README 记「6 装 + 19 未装」（2026-09-23 时点），现已全装。

**结论**：AGENTS.md:23「三套技能并存（`~/.qoder/skills` 的 Matt Pocock 集 / `~/.agents/skills` / superpowers 插件）」**不算错但归属已漂移**——「Matt Pocock 集」现在有**两份**：qoder 根（甲板旧快照，Qoder 专属）与 agents 根（上游本体新版，DSH/Qoder/ZCode 共享），而**当前会话实际可调用的是后者**。

**派活链相关的技能归属**（按 `.skill-lock.json`）：

- 属 Matt 集（上游 mattpocock/skills）：`grilling`、`grill-with-docs`、`to-spec`、`to-tickets`、`triage`、`implement`、`implement-spec`、`tdd`、`code-review`、`pr`、`retro`、`handoff`、`ask-matt`、`chief-of-staff`、`wayfinder`、`diagnosing-bugs` 等 35 个。主流程自述见 `~/.agents/skills/ask-matt/SKILL.md:19-34`：「`/to-spec`（turn the thread into a spec），then `/to-tickets`… Then work the tickets… `/implement` per ticket, `/clear`ing context between each one… or `/implement-spec` for the whole spec in one run… lands everything on one **integration branch**」。
- **不属 Matt 集**：`execute-task`、`plan-feature`（不在 `.skill-lock.json`，SKILL.md 中文触发词/中文 key rules，风格与上游迥异）——它们构成与 Matt 主流程**平行的第二条拆票-执行链**：`plan-feature` 产出 `docs/plans/{feature}/plan.md→PRD.md→issues.md→tasks.md` 四件套（`~/.agents/skills/plan-feature/SKILL.md` 各 Step 的 Output），`execute-task` 读 `tasks.md` 逐任务执行（`~/.agents/skills/execute-task/SKILL.md:22-24` prerequisites）。本仓存量一处：`docs/plans/mobile-auto-deploy/`（四件套齐，2026-09-20），**未入库**（`git ls-files docs/plans/` 为空；`.gitignore:97-103` 的 `docs/*` 白名单不含 plans，与 `docs/README.md:16`「本地不入库」一致）。用量佐证：`~/.qoder/skill-usage.json` 记 `execute-task` 1 次。

## 3. 三分类比对

### 3.1 真冲突（技能自述 vs 本仓裁定，无压制条款覆盖）

| # | 技能主张（原句 + 路径） | 本仓裁定（路径） | 冲突性质 |
| --- | --- | --- | --- |
| T1 | `implement-spec`：「The goal is the entire spec implemented on a single **integration branch**」（`~/.agents/skills/implement-spec/SKILL.md:11`）＋「open a **draft PR** after the first merge in step 5」（:25）＋「each in its own worktree」（:27） | 「一个 PR = 一件做完且已在目标环境验证过的事。不得边做边开 PR、不得把 PR 当进度容器」（`release.md:21`）；「建 worktree 一律走闸门…别裸用 `git worktree add`」（`AGENTS.md:51`）；多票混一 PR 还会撞「正文关联票 ≤5 + sha 祖先判据」（`AGENTS.md:74`） | **正面相撞且无压制条款**：路由表没有「执行票」意图行，表未覆盖 ⇒ 可自主选择（`skill-routing.md:20`），agent 可能自主选到它 |
| T2 | `implement`：「Commit your work to the current branch.」（`~/.agents/skills/implement/SKILL.md:17`） | 多会话共享主树：「提交前验分支归属」「绝不 `git add -A`」「共享文件最小面」（`multi-agent-git.md:17,21,23`） | 技能不知道本仓多会话并发环境；盲提交会踩别的会话分支/索引。multi-agent-git.md 是权威纪律，但**没有任何文件把「implement 的 commit 行为让位」写进选择面** |

### 3.2 表面冲突但已被本仓裁定压住

| # | 技能主张（原句 + 路径） | 压制条款（路径） |
| --- | --- | --- |
| S1 | `grilling`「Ask the whole frontier in one round… Then wait」（SKILL.md:6-8）；`to-tickets`「Iterate until the user approves the breakdown」（:56） | 维护者 2026-09-23 口径「最多留一件要人拍板的事、用散文推进、不用问题组件」；skill-routing.md:36-38「提问形态」段改写落点（`.scratch/<票号>/grilling-round-N.md`），**不改技能文件** |
| S2 | `to-spec`「Apply the `ready-for-agent` triage label」（SKILL.md:19）；`to-tickets` 同（:63） | `triage-labels.md:31`「当某个 skill 提到 role…使用此表中对应的 label 字符串」＋移动端 `ready-for-mobile-agent` 变体（:9,13-16） |
| S3 | `handoff`「Save to the temporary directory of the user's OS… not the current workspace」（SKILL.md:8） | skill-routing.md:31「临时产物 / handoff 落点 `.scratch/`，压过 handoff 技能文档里那条」 |
| S4 | superpowers:`using-git-worktrees` 的裸建树兜底 | skill-routing.md:30「**禁止**…兜底路径」＋闸门（`AGENTS.md:51`） |
| S5 | `setup-matt-pocock-skills` 重跑会覆盖 `docs/agents/{issue-tracker,triage-labels,domain}.md` 的承重本地漂移（ready-for-mobile-agent、CONTEXT.md、重复票处置） | 压制只写在**用户级** `~/.qoder/skills/README.md`（「`setup-matt-pocock-skills` 不要再跑」），**仓内无防线**——半压住，见方案 P3 |

### 3.3 互补无冲突

| # | 证据 |
| --- | --- |
| C1 | 拆票主链同构：本仓 scaffold 即 setup 技能产物——`AGENTS.md:25-35` 的「### Issue tracker / Triage labels / Domain docs」块 = `setup-matt-pocock-skills/SKILL.md:86-100` 的模板；`issue-tracker.md` 含 GitHub 模板特有的「PRs as a request surface」flag（:21）；`triage-labels.md` 表头即「Label in mattpocock/skills」（:5） |
| C2 | 本仓明文采纳 Matt main flow（`issue-tracker.md:43`，见 §1） |
| C3 | 会话切分同构：移动端「一条思考链一个会话…一件票一个会话…换气只在阶段边界」（移动端 `AGENTS.md:46-50`）≈ ask-matt「Keep steps 1–3 in one unbroken context window…Each `/implement` then starts fresh…`/compact` at the nearest phase boundary」（SKILL.md:38,40） |
| C4 | `tdd`/`code-review`/`diagnosing-bugs`/`domain-modeling` 已被路由表收编为默认入口（skill-routing.md:26-29）；用量实测 Matt 侧 `code-review` 16 次居首（`~/.qoder/skill-usage.json`，2026-10-24 读数） |
| C5 | `triage` 五 role 状态机与 `issue-tracker.md` 的 wayfinding operations 节直接对接（:45-52）；`/pr` 与本仓 `submit-pr.ps1` 流程不撞（前者管正文形状，后者管开 PR 时机，release.md:10） |

## 4. 最大差异点（单一）

**交付单元与 PR 时机。**

- Matt 流（`ask-matt/SKILL.md:26-27`、`implement-spec/SKILL.md:11,25`）：交付单元是**整 spec**——「the entire spec implemented on a single integration branch」，且「open a draft PR after the first merge」允许 PR 作为**渐进容器**存在；`implement`（:17）默认「commit to the current branch」。
- 本仓（`release.md:19-26`）：交付单元是**一件做完且验完的事**——「不得边做边开 PR、不得把 PR 当进度容器」；PR 是**证据链的终点**而非工作面：sha 绑定门评论先出 → 预检绿 → 才建 PR（`release.md:30`、`AGENTS.md:76`），证据齐 agent 直接合并（`AGENTS.md:83`）。

为什么是它最大：派活链前半（路由/triage/拆票）本仓就是 Matt scaffold 的本地化，**同构**；分叉发生在执行→交付段，且这是唯一一处「技能明文要求做、本仓明文禁止做、又没有已落案压制条款」的正面相撞（§3.1 T1/T2）——其余所有相撞点都已有显式裁定（§3.2）。一旦 agent 在「表未覆盖可自主选择」的空间里自主选了 `implement-spec`，产出的 integration-branch + draft-PR 形态会同时违反 DoD、证据主键、票号关联 ≤5 三条仓库级机器判据。

## 5. 最佳实施方案

原则：**只改两个既有文件，不新建文件、不加新机制**；遵循本仓「指路而非抄写」（`AGENTS.md:59`）与「本表列出的意图以本表为准」（skill-routing.md:20）。合计四处改动。

| # | 文件 | 改动 | 内容要点 |
| --- | --- | --- | --- |
| P1 | `docs/agents/skill-routing.md` 「为什么需要这张表」首条（:9） | 修事实漂移 | 把「三套技能并存：`~/.qoder/skills`（Matt Pocock 集，25 个目录）、`~/.agents/skills`、superpowers 插件」改为按宿主现测的表述：Matt 集**两份快照**——qoder 根 = 甲板包版（2026-09-23，仅 Qoder 宿主），agents 根 = 上游 `mattpocock/skills` 本体（`.skill-lock.json` 可验，2026-10-23 更新，DSH/Qoder/其他宿主共享）；superpowers 仅 Qoder 宿主可注入。补一句「两份快照同名不同版（setup 技能 CONTEXT/GLOSSARY 之别），既有约定『引小节名不引行号』（:43）已覆盖其漂移」。 |
| P2 | `docs/agents/skill-routing.md` 表（:22-32） | **加一行**「执行已拆好的票」 | 默认入口：移动端票 = `training-app/…/docs/agents/issue-dispatch.md` 一行派活（指路，不抄内容）；根侧票 = 按 `release.md` 七步 + DoD。什么时候换另一个：可让 `implement` 读票实现（它要求先 `gh issue view` 报票题，与 §1 认领兼容），但其「commit to the current branch」让位于 `multi-agent-git.md`；**`implement-spec` 禁用于本仓**——integration branch + draft PR 与「开 PR 的时机」（release.md:19-26）及证据主键（:30）正面冲突。 |
| P3 | `docs/agents/skill-routing.md` 「两条边界」（:40-43） | 加第三条 | 「`setup-matt-pocock-skills` 不再跑」入仓：`docs/agents/{issue-tracker,triage-labels,domain}.md` 含承重本地漂移（`ready-for-mobile-agent`、`CONTEXT.md` 布局、重复票动作化处置），重跑会被 seed 模板覆盖（该警示此前只在用户级 `~/.qoder/skills/README.md`，仓内无防线）。 |
| P4 | `AGENTS.md:23` 常驻句 | 最小同步 | 括号内「`~/.qoder/skills` 的 Matt Pocock 集」改为「两根各一份 Matt Pocock 快照（版本不同）」或等义短语，长度不增；其余保持指路 skill-routing.md。 |

**迁移/兼容策略**：

- **不改技能文件**（skill-routing.md:38 既有裁定：上游文件保持字节一致）；所有压制走选择面。
- **移动端例外不动**：`ready-for-mobile-agent` 已由 triage-labels.md 压住（S2）；issue-dispatch.md 是移动端唯一事实源，P2 新行只指路不复制（沿 skill-routing.md:42「不承载移动端约定」边界）。
- **docs/plans 双轨不拆**：`plan-feature`/`execute-task` 是非 Matt 的平行链，其落点本就「本地不入库」（.gitignore 白名单无 plans），存量一处（mobile-auto-deploy）；P2 表行「什么时候换另一个」可带半句「本地文件形拆票（plan-feature→execute-task，落 `docs/plans/<feature>/`，本地不入库）存量一处，新拆票默认 `to-spec`→`to-tickets` 上 GitHub」——不新增禁令，只声明默认。
- **不为 chief-of-staff/orchestration 立规**：前者带 `disable-model-invocation: true`（仅人唤），后者属 stablyai/orca 生态且未见于用量；表未覆盖可自主选择的兜底已够。

**验证方式（沿本仓先例流程）**：本方案为纯文档改动，不命中运行时面（无 `*.uvue`/`*.uts`/training-app 三 json）⇒ PR 正文按先例 #908 写「免（未命中运行时面）」，并按 `release.md:23-24` 补「开 PR 前已完成的验证：重跑本文件 §2 的现测命令（`ls` 两根、`node` 读 `~/.agents/.skill-lock.json`、同名技能 `diff` 抽样、会话技能清单核对）⇒ 结果与 P1/P4 改后表述一致」。流程照 `release.md` 第 4 步：`preflight-pr.ps1` 绿 → `submit-pr.ps1` 建 PR；文档无 CI 门与新守卫需求（checks.md 四件套不涉代码）。

## 6. 证据索引（一手来源清单）

仓内：`AGENTS.md:23,25-31,51,53-55,72-83` · `docs/agents/skill-routing.md`（全文 1-43）· `docs/agents/issue-tracker.md:21,41-52` · `docs/agents/triage-labels.md:5-31` · `docs/agents/multi-agent-git.md:15-38` · `docs/agents/release.md:10,19-30` · `docs/README.md:16` · `.gitignore:97-103` · 移动端 `AGENTS.md:15-17,46-50,196` · 移动端 `docs/agents/issue-dispatch.md`（§0-§4）
仓外（用户级）：`~/.agents/.skill-lock.json` · `~/.agents/skills/{ask-matt,grilling,to-spec,to-tickets,implement,implement-spec,triage,handoff,chief-of-staff,execute-task,plan-feature,setup-matt-pocock-skills}/SKILL.md` · `~/.qoder/skills/README.md` · `~/.qoder/skill-usage.json` · `~/.qoder/plugins/installed_plugins.json` · `~/.qoder/plugins/data/superpowers-qoder-marketplace/`（空）· 本会话技能清单实测（无 superpowers 条目）

---

## 7. 补记（2026-10-24 裁定与执行）

维护者裁定：**删旧副本，全局 `~/.agents/skills` 为 Matt 集唯一事实源**。已执行：

1. `~/.qoder/skills` 删 24 个与全局同名重复（逐个核对全局根确有同名后才删，无跳过项；保留 README 与仅存的 `obsidian-official-cli`、`resolving-merge-conflicts`）。
2. §5 方案落**简化版**：P1/P4 由「描述两份快照」缩为「Matt 集 = 全局安装（上游本体）」——落点 `docs/agents/skill-routing.md`（判据首条改写 + 计数换全局根现测 51/24 + 表加「执行已拆好的票」行 + 边界两条→三条）与根 `AGENTS.md`（Skill routing 常驻句同步）；P2（执行票行）、P3（setup 警示入仓）照落。纯文档改动，只改工作区，未开 PR。
3. §2 的两份快照实测是时点记录，按账本惯例保留不改写；此后以本节裁定为准。
4. 同日维护者确认 **superpowers 未在用**，操作性引用全部移除：`skill-routing.md` 判据首条的现存描述尾注、superpowers 专论两条（注入不对称删 / 只有 AGENTS.md 压得住它改写为通用「常驻」理据）、表中三处（`brainstorming` 与 `systematic-debugging` 备选删、`using-git-worktrees` 禁令改写为通用裸建禁令）、根 `AGENTS.md` 常驻句的 superpowers 分句；「用量实测」条是时点记录，保留不动。另修一处计数漂移：表加「执行票」行后 `AGENTS.md`「其余六个意图」改「七个」。
5. 同日维护者要求**不泄露用户本机路径**：本文件 §2 与 `skill-routing.md` 判据段共三处用户主目录绝对路径改写为 `~/` 形态（其中用量实测一处系沿用原文件旧写法，一并归一）。已合并历史提交里仍可见原串，属 git 史实，不做历史改写。
