# 发布流程（push / PR / merge）

> push / PR / merge 前必读。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

master 有仓库 ruleset「protect master」保护（直接 push 会被拒，`push declined due to repository rule violations`），且限定 squash 合并。发布必须走分支 + PR：

1. **本地提交**（只 add 本次改动的文件，勿 `git add -A`）。
2. **建分支推送**：若提交已在本地 master 上，`git branch feat/xxx` 后 `git reset --hard origin/master` 还原本地 master；然后 `git push -u origin feat/xxx`。
3. **分支 push = CI + testing 冒烟**：push 触发 `ci.yml` 全量 CI（不再同时跑 `pull_request`，一次改动只跑一遍）。CI 全绿**且该分支已有开启的 PR** 时，`ci-summary` 用 `workflow_dispatch --ref <分支>` 派发 `cd.yml` 部署 testing（起栈 → 健康检查 → 立即 down）。孤儿分支/无 PR 分支不占 testing。
4. **建 PR：先预检绿，再交给 `submit-pr.ps1`（#1770）**：先跑 `pwsh scripts/preflight-pr.ps1`（**仓库根**的 `scripts/`，不是移动端目录）——本地对将要推的 commit 跑**与 CI 同一判据**（`.github/scripts/lib/gate-validator.mjs`，不抄第二份），**#2053 起再叠一条超集判据**：对 PR 正文跑引用完整性 lint（`.github/scripts/lib/lint-issue-references.mjs`，红项并进 errors、共决预检绿红——CI 合并门不跑它，「预检绿」= 门绿 + 引用零漂移）；绿了再跑 `pwsh scripts/submit-pr.ps1 -Title <标题> -BodyFile <正文>` 建 PR（`gh pr create --base master --head feat/xxx` 这一步由它执行），预检不绿就打印差哪几门、**不建 PR**。**前置：HEAD 已 push 到 `origin/<branch>`**（预检用 compare API 与 commit 评论读远端，未 push 会读不到）；`submit-pr.ps1` 还要求该 commit 的 `ci-summary` check 为 `completed + success`，并**从 check-runs 自动发现** ③ 门的 run 链接补进正文结论（占位/空结论不补 —— 补上去会让「占位如实判红」失效）。**PR 事件不触发任何 CI/CD**；PR 页上的 `ci-summary` 等检查来自第 3 步分支 push 的同 commit run。该 commit 若缺冒烟记录（新顺序下 CI 通常已在开 PR 前跑完），`testing-smoke.yml` 会在 PR 开启时补发一次（仅此一种条件下动作，不重跑 CI）。
5. **等门禁**：`gh run watch <id> --exit-status` 等 CI 全绿（纯前端改动时 backend-\* / migration-check 跳过属正常，`ci-summary` 仍会 success）。ruleset 把 `ci-summary` 与 `ensure-testing-smoke` **两条**设为必检、并要求分支 up-to-date —— 未跑绿、或 master 已前进时不能合并（后者要 `git merge origin/master` 同步后重推、重跑 CI）。
   第二条是 #1852 的修法：`gh pr merge --auto` 会在检查绿后**立刻**合并并删掉 head 分支，而 `ensure-testing-smoke`（补发 testing 冒烟的那个 job）当时还在排队 —— 轮到它时分支已经没了，`gh workflow run --ref <分支>` 报 `HTTP 422 No ref found`，该 commit 永远没有冒烟记录，生产 gate 只能阻断（实测 #1850 / #1851 各被挡一次，靠应急通道才上线）。把它变成必过检查后，合并会等它跑完；它自己则先等 `ci-summary` 出结论（最多 15 分钟）再决定是否补发。
   多会话并发期，建好 PR 后立即 `gh pr merge <n> --squash --auto` 兜底——master 被并行推进时只需 `git merge origin/master` 同步重推，requirements 齐后自动合并（#1852 修复后 auto 不会再抢在冒烟前合并，安全；2026-10-24 单日 3 次 up-to-date 抢道实测有效）。
6. **Squash merge → 直发 production**：`gh pr merge <n> --squash --delete-branch`。master 的 push **不跑 CI**，直接触发 `cd.yml` 的 `gate` job：从 commit 主题解析 `(#N)` → 校验该 PR head 的 `ci-summary=success`、该 commit 的 testing 冒烟 `success`（冒烟可能晚于合并，gate 最多轮询 15 分钟）→ 通过后才构建镜像并部署 production。若报 "requirements have not been met"，用 `gh pr view <n> --json statusCheckRollup` 排查。
   ⚠️ **`--delete-branch` 会静默不删分支**（2026-09-19 实测：本仓 14 个远端残留分支，13 个对应已 MERGED 的 PR，时间线里**连一条 `head_ref_deleted` 都没有** ⇒ 删除动作从未执行）。两条失败路径都不报错：① gh 2.97.0 `deleteRemoteBranch()` 把远端删除包在 `if !m.merged` 内 —— 命令启动时该 PR 已是 MERGED（**重试** / 网页合并 / REST 兜底 / auto-merge）就整段跳过，**之后仍照样打印 `Deleted remote branch`**；② 分支被 worktree 占用时 `deleteLocalBranch()` 先 `return err`，而调用顺序是本地在前 ⇒ 远端删除也跑不到（故**先 `git worktree remove`，再 `branch -D`，最后删远端**）。合并后**只信 API 的当场读数**，别信命令回显。
   ⚠️ **别给 `gh pr merge --squash` 传 `--subject`**（2026-10-03 实测，P2 波 4e）：显式传主题会**抑制 GitHub 自动追加的 `(#N)` 后缀**，而 `gate` 的判据是主题**以 `(#数字)` 结尾**（`printf '%s' "$SUBJECT" | grep -oE '\(#[0-9]+\)$'`，`cd.yml:120`）⇒ gate 直接报 `无法从 commit 主题解析 PR 号:[…]` 并 `exit 1`，生产的 build-backend / build-frontend / build-libreoffice / deploy 全部被跳过（实测 run 37090597016 失败，只能走下方「应急通道」手动 dispatch 放行）。主题确需自定义时，务必让它**以 `(#<PR号>)` 结尾** —— 注意是**半角**圆括号，写成全角「（#1445 P2）」不算。
7. **收尾**：`git fetch --prune` → `git checkout master && git pull --ff-only` → 删除本地 feat 分支（若 gh 已自动删）→ **机检分支真的从远端消失**：`pwsh training-app/叉车维修培训学员端跨端应用/scripts/check-branch-residue.ps1`（按关联 PR 状态判定，带 `-Execute` 可删 MERGED 残留）。**别用 `git branch -r` 的 ahead 数或 `merge-base --is-ancestor` 当判据**：squash-only 仓库里多提交分支的 patch-id 与任一原提交都不同，会把「已落地」读成「未合并」（本次 12/13 的 squash 提交其实已是 master 祖先）。仓库级兜底须为 `delete_branch_on_merge=true`（Settings → Automatically delete head branches）—— 它属**仓库设置**、需 admin 权限，agent token 通常没有（本仓实测 PATCH 返回 404），要由人点。

**合并前一行披露（2026-09-16 立）**：**授权不变** —— 证据齐、无例外通道时 **agent 可直接合并**（口径见下方「验收门」段）。加的是**可见性**：合并前必须在 PR 上留**一行**说明「**本次合并将触发 `cd.yml` 的 production 部署**」（贴 PR 评论或正文均可，先例 #1063）—— 让维护者知道这一步会动生产，而不必自己推断「这个 PR 改的是文档，为什么也会上生产」。走**例外通道**（「已接受未验证风险」）的 PR 仍**必须由人**执行合并。

**开 PR 的时机：一件事「做完并验完」之后才开 PR（2026-09-15 裁定）**。上面七步讲的是**机制**（机制允许你随时 push / 开 PR），不是节奏 —— 节奏由本条管。

- **一个 PR = 一件做完且已在目标环境验证过的事**。不得边做边开 PR、不得把 PR 当进度容器，也不得用「后续 PR 再修」把**已知未验证**的改动送进 master。
- **完成定义（DoD，三条都要）**：① **目标行为在目标环境上被观测到**（真机 / 真链路 / 真数据），并留下可核验产物；② 本次改动涉及的检查在本地跑绿（至少 `npm run test:unit`；触及移动端运行时面再按 ADR-0008 跑对应门）；③ 现场可还原（临时改动已还原、`manifest.json` / `pages.json` 未被回写改脏、无 `hx-agent.lock` 残留）。
- ⚠️ **「未命中运行时面 ⇒ 免四门」不等于「不需要验证」**：工具链 / 脚本 / 测试类改动的验收判据是「**它要达成的那个行为在真实链路上成立**」。例：改 `auto-screenshot` 的截图判据 ⇒ 验收 = **跑一次真机 🟡** 并留下截图与机检行；只跑 `test:unit` 就写「免」**不算**验收。
- 这类改动请在正文 `## 验收证据` 里补一行：**`开 PR 前已完成的验证：<命令 / 链路> ⇒ <观测到的结果与产物>`**。
- **唯一例外**：确实只有 CI / testing 环境才能验的部分（`ci-summary`、testing 冒烟等）允许带着 PR 去跑；正文必须写明「**待验证项 / 由谁验 / 何时补**」，且**补齐之前不得合并**（例外通道仍由人执行合并）。
- **反例（写这条的由来）**：2026-09-15 一轮里三个工具链修复 PR 都在真机验证**之前**合并，随后真机又接连发现「黑屏骗过落定判据」「日志编码致页身份行读不到」「换设备装不上基座」三处缺陷，只好再补第四个 PR —— 根因就是把「免四门」误读成了「免验证」。

**验收门（合并前置，与上面的 CI/CD 机制是两道独立的门）**：上面第 3–6 步只讲 CI/CD；`.github/workflows/pr-evidence.yml` 对**每一个 PR** 运行，但**只对命中运行时面**（`*.uvue` / `*.uts` / training-app 下的三份 json）**的 PR 校验正文的 `## 验收证据` 段** —— 未命中时校验器在校验段**之前**就放行（`runtime.length === 0` 早退；用例见 `.github/scripts/pr-evidence-check.test.mjs`「非运行时面 PR：即使正文为空也通过」），命中而缺段则判红：未命中运行时面仍**约定**写 `免（未命中运行时面）`（先例 #908），命中移动端运行时面则按四门填「执行人 / 日期 / 复测对象 / 结论（含产物）」。**人工门已收缩为 ①b**（只在命中「能力面」时必过；①a 由 agent 出证、按一次分支收口跑）——四门判据、豁免与「**签收在人、合并不限人**」纪律见 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md`（**写全路径**，根仓库另有一个同名的 `ADR-0008`），**① 门自身的现行口径**见同目录 `0016-真机门的人工性收缩与按批取证.md`。

**证据主键 = sha 绑定的门评论，不是 PR 号（#1770 起；#1841 起含降级常态）**：门脚本以 `-PostToIssue <票号>` 出证——正常路径**判据评论贴 commit**（`gh api repos/{owner}/{repo}/commits/<sha>/comments`），`POST /repos/<owner>/<repo>/issues/<N>/comments` 上是同一份正文的副本。**降级触发条件只有一个：commit 评论 POST 返回 HTTP 404**（本仓长期如此、同 token 在其他仓正常，复盘见 [#1838](https://github.com/DriftingLi/FL/issues/1838)）——此时**同一份评论**（`<!-- gate-evidence:<门号> -->` 与 `commit: <sha>` 一字不改）**贴到票**（`/issues/<N>/comments`）当**正式证据落点**，并要求脚本在输出里打印降级结论与落点 URL（不得静默 skip、不得改成 exit 1）。校验器候选集 = PR 评论 + commit 评论 + **PR 正文关联的票评论**，三来源**同构**：marker、`commit: <sha>` 绑定、祖先判定、其间运行时面（`*.uvue` / `*.uts` / training-app 三份 json）无改动、fail-closed **全部沿用，一条都不放宽**。哪些票算候选由正文决定（判据源 `issueNumbersInBody`：剥 HTML 注释后取 `Closes` / `Refs` / `Fixes` / `Resolves` / `Related to #N` 与 issue 直链，去重后**最多 5 张**）——**未在正文关联的票不进判据**（关联只认正文这一处）。评论链接形态同样是 `#issuecomment-<id>`，三来源 id 同源。截图归档目录键 = 票号：`docs/verification/<模块>/<票号>/`（旧 PR 号目录是数字段、仍被接受，新写法不再教）；`-PostToPr` 保留为**别名**、一个版本后退役。⇒ **顺序不因降级而变：证据先出（commit 评论；404 时票评论）→ 预检绿 → 再开 PR**（见第 4 步），首次推送就让 `pr-evidence` 判绿。

**应急通道**：gate 阻断但确认可以上生产时，手动放行 `gh workflow run cd.yml -f environment=production -f ref=<master sha>`（dispatch 不经门禁）。若是 testing 冒烟失败，先到 Actions 重跑该 CD run，再重新合并或走应急通道。

> ⚠️ 本次流水线变更**合并前**切出的分支：其 `ci-summary` 受旧条件约束（仅 master 上报），在 PR 上显示 skipped，既不满足必检也会被 gate 判为 `other`。先 `git merge origin/master` 重推、等 CI 重跑，再走合并流程。

> ⚠️ **不要用** **`timeout N`** **包裹 git/gh 的写操作**（merge / push / rebase / checkout）。被 SIGTERM 杀掉的是**执行到一半**的操作，比失败更糟：曾因 `timeout 180 gh pr merge --squash --delete-branch` 被中断，残留 `.git/index.lock` 且分支清理删了一半，`frontend/src` 下 265 个文件被删。这类操作一律用后台任务跑并等其自然结束。
