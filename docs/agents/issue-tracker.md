# Issue tracker: GitHub

这个 repo 的 issues 和 PRDs 存放在 GitHub issues 中。所有操作都使用 `gh` CLI。

> **`gh` 的路径与登录态按机器现测**，不要照抄任何一份记录：`command -v gh` + `gh auth status`。2026-09-20 在 `D:\FL` 这台机器实测 `gh` 就在 PATH 里（`C:\Program Files\GitHub CLI\gh.exe`），`~/.bun/bin` 下没有 gh、`~/.config/gh` 也不存在（Windows 的登录态落在别处）。
> ⚠️ 另有一条与权限档相关的实测：`gh` 的**任何**子命令（含 `issue list` 这类只读）在 auto 权限档下会被 classifier 拦下 —— 需要发布时换档或由维护者自己跑。
> ⚠️ 通道分叉（2026-10-28 实测，#2079）：harness **代码通道**（run_code 一类起的子进程）**空 env 起步**——既无 gh 登录态也无 git 凭据，报「Please run: gh auth login」不是凭据失效而是**通道选错**；gh 的一切读写走宿主 shell 通道（bash / pwsh），登录态现测只认那里的 `gh auth status`。
> **给 deck_* 远端调用把超时放足（≥300s）**（2026-10-28 实测，#2044 会话 retro 复盘 / #2052）：默认 120s 预算内首调实测报 `backend-timeout`，同参重试即成 —— 那个回包是**超时不是失败**，只读操作（读票 / 列票 / 看地图）按重试处置，别读成「查无此票」；**写**侧是另一条判读（超时/报错 ≠ 未落地，先 `gh api` 现测存在性再重试），落点在下方 Conventions 的「deck 与 gh 的分工」与「deck 写操作超时/报错 ≠ 未落地」两条，按这句粗体引导语找。
> ⚠️ **`gh` 的间歇性 EOF 按重试处置**（2026-11-05 实测，#2312 会话 retro 复盘）：`gh` 会偶发 `Get "https://api.github.com/...": EOF`。**实测不区分 REST / GraphQL** —— 当日 3 次 EOF 全部落在 REST `gh api` 上，而 GraphQL（`gh issue view` / `gh pr checks`）9/9 成功；随后两族各 12 次连打**均 0 失败** ⇒ 判据是「**间歇性网络 EOF，两族都可能是它**」，**不是**「GraphQL 坏、REST 好」（别按端点族挑通道）。只读操作同参重试即可；**写操作先现测对象存在性再重试**，防重复写（与下方「deck 写操作超时/报错 ≠ 未落地」同族）。**不做硬门禁**：网络抖动属环境，不是仓库状态。

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`。多行 body 使用 heredoc。
- **Read an issue**: `gh issue view <number> --comments`，用 `jq` 过滤 comments，并同时获取 labels。
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'`，按需加上 `--label` 和 `--state` filters。
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`
- **票面验收口径只引已合库 ADR**（2026-10-24 立，#1845-#1848 复盘）：开票时 What to build / Acceptance criteria 里引用的规则，只引**已合并进 master 的 ADR/文档**；「已裁定未落库」的口径先落库再进票面——两套口径并存时（如 #1853/#1858 那种文档包与判据包分飞的场景），执行会话无法机械判定哪套生效，只能人肉考古（当日实测代价：票面引 0040/0039 窗口径、而退役窗口的 0043 处于已裁定未合库态）。
- **票面引用完整性机检**（#2053，2026-10-28 立；上一条散文规的确定性检查面）：三条红线——① 反引号内、仓顶层目录前缀（`docs/`、`scripts/`、`.github/`、`training-app/`、`deploy/`、`frontend/`、`backend/`）的仓内相对路径必须存在于仓——**双承载面探**（仓根 / 移动端根任一命中即算存在；行号后缀 `:35` 先剥再判，占位/示意形态与非顶层裸路径不判）；② `ADR-00NN` /「移动端 ADR 00NN」号位引用必须有对应文件在库，两套编号互不相关、同号不互相豁免；「新/待建/拟」紧贴 ADR 词头的规划形态写死号位、且号位已被占 ⇒ 判红——合规写法「编号 = 执行时 docs/adr 最大号 + 1」不写死号、天然不在捕获面；尾窗「已被占用/已占/已在库/复盘」类既成陈述豁免（#1861 血账票面记法不算漂移）；③ `#<N>` 必须能在 DriftingLi/FL 解析；小节号（§6.3 类）显式不判。**接线点**：`to-tickets` / `triage` 落票最后一步跑 `node .github/scripts/lib/lint-issue-references.mjs --body-file <正文文件>` 预检——报红项、**不自动改**，红清零才落票；PR 正文由 `preflight-pr.mjs` 跑同一模块（与证据段判据同层、共决预检绿红，不新增门——预检绿是门绿的超集）。行为守护：`.github/scripts/lint-issue-references.test.mjs`（夹具真执行 lint 函数；红夹具即 #1861 修复前形态，绿夹具即 #2049 合规形态）。只 lint 新写入，不回扫历史票。
- **反引号路径引「当前树磁盘上存在」的文件；成本 / 套件数一律现测**（2026-11-05 立，#2312 会话 retro 复盘）：上一条①判据的 `existsPath` 用 `existsSync` 判**磁盘存在**、**不查 git 是否入库** ⇒ 判红的真实条件不是「未入库」，是「**该正文所跑的树里磁盘上没有这个文件**」。实测同一份正文：主树跑 ⇒ **绿**（文件在主树磁盘上）；worktree 跑 ⇒ **红**（该文件不在 worktree 磁盘上）⇒ **同一段正文在主树绿、在 worktree 红**，而 PR 预检的 cwd 正是 worktree ⇒ 落票 / 落 PR 前在 worktree 内只引**已入库**路径，未入库文件**只叙述、不引路径**。另一半：**数字会漂，一律现测**（套件数分母已见 103 / 109 / 107 三个值；成本分钟数同理）⇒ 历史读数只当历史，不写进代码或断言，引用前现跑取当次值并注明日期（与移动端 `training-app/叉车维修培训学员端跨端应用/docs/agents/dev-loop.md`「套件数不写死」同族）。
- **引用必须逐字回读一手，转述归 reviewer**（2026-10-28 立，#2079；实证 #2053 实施会话）：把**他文件的内容**转述进票面/文档前，落笔会话必须打开原文逐字核对；票面已有的转述属二手，执行会话直查一手原文、不继承（实测：#2053 票面把移动端一条「不写死套件数、以现测为准」的 ADR 原话转述成「写死会变数字」——讲的是不同东西，执行时把带伤转述带上 §5 行，评审逐字回读原文才拦下；同族前科：#2052 评审包修过四处措辞失真）。上一条机检只判**存在性**、不判语义等价——等价是 judgement，归 reviewer agent，不是 lint。
- **deck 与 gh 的分工**（2026-10-28 retro #1861 triage 会话实测）：**写、边、上报、维护者原话走 deck_ 工具**（幂等锚与 workRoute 账只在 deck 侧）；**验证与读回走 `gh api --jq`**——deck 每次回执带约 2KB 记账 JSON（cost/quota/workRoute），反复用 deck 读同一张票就是在付噪声。
- **deck 写操作超时/报错 ≠ 未落地；写后现测只认 gh**（2026-10-28 实测，#2049 建票）：一次 create 返回 `unsupported: repo.refId missing`、一次 120s 超时无回执，随后 `deck_issue_list` 查重**假阴性报「无」**（索引滞后）；`gh api` 才见票实际已建成。⇒ deck 写操作失败/超时后，先 `gh api` 现测对象存在性，确认不存在才重试，防重复票（重复票处置税见 `docs/agents/triage-labels.md`）。
- **deck 的额度闸不是后端故障：按通道分族，见底就等整点、再走 gh 回退**（2026-11-07 立，#2345 复盘 #2317）：deck_ 工具给自己设了**每小时自留额度**（`min(GitHub 剩余 × 0.35, 1750)`，其中三成是写保底、不可被读吃掉），用满后**所有走 GraphQL 的动作被整体拒**，回 `status: unsupported` / `reason: ai-tool-refused`、正文写「额度见底：GraphQL 点数额度不够了（还剩 N 点，约 HH:MM 恢复）」。这是**限流不是故障**——别读成「票不存在」、别按端点族挑通道、也别重试到窗口翻转（整点复位）。按通道分族：`create` / `comment` / `setBlockedBy` 是 **REST 与 GraphQL 任一可用即可**；`update` / `setLabels` / `setAssignees` / `setParent` / `close` / `reopen` 与全部读动作（`list` / `get` / `listPage` / `listLabels` / `counts` …）**只走 GraphQL**，见底必被拒（读侧还要额外给写让路：剩余落进写保底区间时读先拒、写随后也拒）。
- **「N 步成了 M 步」回执必须逐项 gh 核验，不许当整体 ok**（2026-11-07 立，#2345 复盘 #2317）：`deck_issue_patch` 一次可点多个动作（评论 + 关票 + 改标签…），回执「票 <key>：这次点了 N 步，成了 M 步」里 **M < N 就是有步没落地**；典型形态是「评论经 REST 发成了、关票卡在 GraphQL 没成」（2 步成 1 步），或 `close:true` 回「1 步成了 0 步」= 票**没关**。⇒ M < N 时逐项现测：状态看 `gh api repos/DriftingLi/FL/issues/<N> --jq .state`、评论看同一对象的 `.comments`、标签看 `.labels`；缺哪步补哪步，**不重发整个 deck 调用**（会重复评论）。回退命令：列票 `gh issue list --search "<关键词>" --state all --json number,title,state,labels`、关票 `gh issue close <N> --reason completed`、摘标签 `gh api -X DELETE repos/DriftingLi/FL/issues/<N>/labels/<label>`、改正文 `gh issue edit <N> --body-file <文件>`。**关票一律先评论、再关闭**（对已关闭的票再「带评论关闭」，评论会丢、调用仍回成功）——一步到位就 `gh issue close <N> --comment "..."`。

从 `git remote -v` 推断 repo；在 clone 内运行时，`gh` 会自动处理。

## Pull requests as a triage surface

**PRs as a request surface: no.** _（如果这个 repo 把 external PRs 当作 feature requests，则设为 `yes`；`/triage` 会读取这个 flag。）_

设为 `yes` 时，PRs 走与 issues 相同的 labels 和 states，使用 `gh pr` 对应命令：

- **Read a PR**: `gh pr view <number> --comments`，以及 `gh pr diff <number>` 获取 diff。
- **List external PRs for triage**: `gh pr list --state open --json number,title,body,labels,author,authorAssociation,comments`，然后只保留 `authorAssociation` 为 `CONTRIBUTOR`、`FIRST_TIME_CONTRIBUTOR` 或 `NONE` 的（丢弃 `OWNER`/`MEMBER`/`COLLABORATOR`）。
- **Comment / label / close**: `gh pr comment`、`gh pr edit --add-label`/`--remove-label`、`gh pr close`。

GitHub 在 issues 和 PRs 之间共享一个 number space，因此裸 `#42` 可能是两者之一——用 `gh pr view 42` 解析，失败则回退到 `gh issue view 42`。

## When a skill says "publish to the issue tracker"

创建一个 GitHub issue。

## When a skill says "fetch the relevant ticket"

运行 `gh issue view <number> --comments`。

## Wayfinding operations

> ⛔ **本仓的 map 实验已于 2026-10-10 短暂收口后重新启用**：五个 `wayfinder:*` label 保留可用，sub-issues 与原生 `blocked_by` 边均实测可用；首张 map = #1585 当天开、当天关（not planned），五张执行票 #1586-#1590 改为直接挂 ADR-0035；但**第二张 map = #1591 已开且在用**（小程序 60 页 UI 改造，3 张 grilling + 4 张 research 子票），「唯一一张 map」的说法作废。
> **为什么关**：① 原判断「本仓不用 wayfinder」的依据「准入三判据未命中」是**自我实现的**（最硬那条是「label 不存在、map 数为 0」，没有 map 就永远判不通过）——这部分旧判断确实作废；② 但当天实践**证伪了反向的过度启用**：60 页 UI 改造能一手握住（先 grep 出事实再 grill，十八问一个会话收敛），这正是 `/grill-with-docs` 的形状，而 `/wayfinder` 的准入判据是「**能不能一手握住**」，不是「任务大不大」；③ 更关键的是**「承载执行」这处覆盖是错的**：它让地图的 `Destination` 自指 `/to-spec` 的产物（spec 是地图自己下游的东西），地图因此失去下游出口——详见 #1585 的关闭评论。
> **准入判据的唯一事实源**是 `training-app/叉车维修培训学员端跨端应用/docs/agents/issue-dispatch.md` 文末节「什么时候用 wayfinder」（**该文件属移动端层**）。本仓的执行票属 main flow 的 `/to-tickets`（`grill-with-docs` → ADR → `/to-spec` → `/to-tickets` → `/implement`），**不挂 `wayfinder:*` label**；若将来真出现「连第一个决定是什么都说不清」的情况，照 `/wayfinder` SKILL.md 原样用，**不要带本仓那处覆盖**。

供 `/wayfinder` 使用。**map** 是单个 issue，以 **child** issues 作为 tickets。

- **Map**: 单个带 `wayfinder:map` label 的 issue，保存 Notes / Decisions-so-far / Fog body。`gh issue create --label wayfinder:map`。
- **Child ticket**: 作为 GitHub sub-issue 链接到 map 的 issue（通过 sub-issues endpoint 使用 `gh api`）。未启用 sub-issues 时，把 child 加入 map body 中的 task list，并在 child body 顶部写 `Part of #<map>`。Labels：`wayfinder:<type>`（`research`/`prototype`/`grilling`/`task`）。一旦被 claim，ticket 被 assign 给 driving dev。
- **Blocking**: GitHub 的 **native issue dependencies**——canonical、UI 可见的表达。用 `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>` 添加 edge，其中 `<blocker-db-id>` 是 blocker 的数字 **database id**（`gh api repos/<owner>/<repo>/issues/<n> --jq .id`，_不是_ `#number` 或 `node_id`）。GitHub 报告 `issue_dependencies_summary.blocked_by`（仅 open blockers——即 live gate）。依赖不可用时，回退到 child body 顶部的 `Blocked by: #<n>, #<n>` 行。当所有 blockers 都关闭时，ticket 即为 unblocked。
- **Frontier query**: 列出 map 的 open children（`gh issue list --state open`，限定在 map 的 sub-issues / task list），丢弃任何带有 open blocker 的（`issue_dependencies_summary.blocked_by > 0`，或 `Blocked by` 行中的 open issue）或带有 assignee 的；按 map 顺序第一个胜出。
- **Claim**: `gh issue edit <n> --add-assignee @me`——session 的第一次写入。
- **Resolve**: `gh issue comment <n> --body "<answer>"`，然后 `gh issue close <n>`，再向 map 的 Decisions-so-far 追加 context pointer（gist + link）。
