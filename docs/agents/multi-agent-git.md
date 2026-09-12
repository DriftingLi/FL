# 多 Agent 并发与 git 隔离（可选）

> 多个会话/自动化工具同时操作仓库前必读；单人单会话可跳过。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。

多个 agent 会话（含 aider / 自动化监控）若**共用同一份 checkout 操作 git，会互相踩踏**：并发会话可能擅自 `checkout master` 并清掉分支，把另一会话刚 `commit`、尚未 `push` 的提交甩成无引用游离状态；`add`/`commit`/`push` 也会抢 `.git/index.lock`。

**单人单会话开发时不需要 worktree**，直接在主目录切分支即可。只有当多个 AI 会话/自动化工具同时操作同一个仓库时，才需要用 worktree 隔离：

- **一会话一 worktree 一分支**：动代码前先 `git worktree add ../wt-<task> -b feat/<task> origin/master`，全程在 `../wt-<task>` 内改、提交、push、开 PR；HEAD 与 index 天然隔离，避免并发 checkout/reset 互踩。

- **提交前验分支归属**：`git branch --show-current` + `git log --oneline origin/master..HEAD`，确认 HEAD 与待推提交属本会话任务，不在别的会话占用的分支上落 commit。

- **发现游离提交先取证、勿覆盖**：工作区被并发切走、自己的 commit 在 `git branch` 里消失时，用 `git log --oneline <sha>` + `git diff <sha> origin/master -- <paths>` 只读确认内容是否已在 master；已在则不 push、不 `reset`，交 gc 自然回收，绝不 `cherry-pick` 造冲突。

- **绝不** **`git add -A`**：共享工作区常混有其他会话/工具的未提交改动（如 `forum.uts`、`.aider-desk/`、`.monitor/`），只 `git add <本次文件>`。

用完 worktree 后记得清理：`git worktree remove <dir>` + `git branch -D <branch>`。

## Windows 上用 worktree 的注意事项

Windows 本机（`E:\` 盘）上 worktree 可用，但有几处与 Linux 不同，照下面做：

- **一 worktree 一分支一会话**：`git worktree add E:\wt-<task> -b feat/<task> origin/master`，全程在 `E:\wt-<task>` 内改、提交、push、开 PR；用完 `git worktree remove E:\wt-<task>` + `git branch -D feat/<task>`。放在 `E:\wt-<task>`（与主树同级）而非盘符根，便于一眼看清是哪个会话的目录。

- **`node_modules` 不要每个 worktree 重装**：目录联接（junction）共享主树那一份，省掉每个 worktree 2–5 分钟的 `npm ci`：
  ```
  cmd /c mklink /J <worktree>\training-app\叉车维修培训学员端跨端应用\node_modules <主树>\training-app\叉车维修培训学员端跨端应用\node_modules
  ```
  若确实需要孤立依赖（例如验证 lockfile 变更），才在 worktree 内 `npm ci --ignore-scripts`。

- **junction 要先删再删目录**：PowerShell 7 的 `Remove-Item -Recurse -Force` 只删联接本身、不跟进主树（本机 PS 7.6.5 实测；Windows PowerShell 5.1 不保证），但**删 worktree 时先单独拆掉联接更稳**：`cmd /c rmdir <worktree>\training-app\叉车维修培训学员端跨端应用\node_modules`，再 `git worktree remove`。Windows PowerShell 5.1 下 `Remove-Item -Recurse` 是否会跟进 junction 并删掉主树内容，**未证实**——所以不要靠它。

- **HBuilderX 仍要唯一项目名（worktree 解决不了）**：HBuilderX 按**项目名**解析，worktree / 完整 clone 里的项目目录 basename 与主树相同 ⇒ 跑 HBuilderX 前要把该项目目录改成**唯一名**。改名期间 `git diff --name-only` 会误报整目录删除（删除 + 未跟踪新增），所以**任何 git 操作前必须先把项目目录名改回**。

- **`stash` 是全仓共享的**：所有 worktree 共用同一个 stash 栈。别在多 worktree 之间长期留 stash，用完即 `pop`/`drop`；否则另一会话 `/clear` 后无法分辨哪条 stash 是自己的。

- **worktree 元数据也在 `.git` 下**：`.git/worktrees/` 与 `git worktree list` 全局共享，不是每个 worktree 一份。

### worktree 元数据异常：先实测，再决定要不要修（2026-09-13 重写）

**先实测，别照配方动手。** 历史症状是 `E:\FL\.git\worktrees` **列不出内容**（`Get-ChildItem` 返回 0 项），导致 `git worktree add` 连续失败。**但 2026-09-13 复测该症状已不复现**：枚举正常返回 1 项、`git worktree list` 正常返回 2 条。所以第一步永远是这条只读探测：

```powershell
Get-ChildItem E:\FL\.git\worktrees -Force           # 能列出条目 = 枚举正常
cd E:\FL; git worktree list                         # 能列出全部 worktree = 注册面正常
git worktree add E:\wt-probe -b chore/wt-probe HEAD # 唯一的实证：能建 = 没问题（试通后 worktree remove）
```

**三条判据全过 ⇒ 无需修复**，下面那条破坏性配方**不要执行**。

#### ⚠️ 旧配方的实害（此前没写，务必先读）

旧配方的「优先」路径是 `Remove-Item -Recurse -Force E:\FL\.git\worktrees` + `git worktree prune`。它当时声明「低风险」的依据是**「该目录枚举为空、只有主树一条注册」** —— **该前提现在已经不成立**：

`E:\FL\.git\worktrees\wt-tabbar-align` 是**活 worktree**（`E:\wt-tabbar-align`）的登记目录，而活 worktree 的 `.git` 是个**文件**，内容写着 `gitdir: E:/FL/.git/worktrees/wt-tabbar-align`。**删掉那个目录 = 把该 worktree 孤儿化**（`.git` 指向不存在的路径）。

所以：**只要 `git worktree list` 里除主树外还有别的 worktree，就绝对不要删 `.git\worktrees`。** 先检查（有输出就别删）：

```powershell
cd E:\FL; git worktree list | Select-Object -Skip 1
```

#### 真要修时的安全顺序（需管理员）

1. 先按上面的只读探测确认「确实坏了」，并**把报错原文留证**；
2. **不要删目录**，改成重置该目录 ACL（**管理员** PowerShell）：
   ```powershell
   icacls "E:\FL\.git\worktrees" /reset /T /C
   ```
   若确实要删目录重建，必须先确认没有活 worktree，并逐个 `git worktree remove` 之后再删；
3. 修完重跑一次 `git worktree add E:\wt-probe -b chore/wt-probe HEAD` 试通，再 `git worktree remove E:\wt-probe`。

⚠️ **本节点截至 2026-09-13 仍是待办**（`docs/agents/handoff-验收门-2026-09-11.md` 六、待办）：复测显示枚举与 `worktree list` 都正常，但**尚未做过 `git worktree add` 试通** ⇒ 「ACL 是否已自愈」**未证实**，**不要**据本节的复测宣布待办已闭环。**在试通之前**，多会话隔离继续用独立 clone。

#### 重复副本的现状与清理纪律（2026-09-13 实测）

- 实测 `E:\` 下有 **11 个同仓库克隆**，约 **1443 MB**（旧文档记的「约 618 MB / 892 MB」已过时）。
- **不能按目录名盲删**：其中 **9 个持有未推送提交或脏文件**（`_g624c` 577 未推送 + 440 脏、`_spike` 411 脏）。删前必须逐个体检 `git status` 与 `git log --branches --not --remotes`，并查该分支的 PR 是否已合并（`gh pr list --head <分支> --state all`）—— **不要凭分支名或印象判断**。
- **候选的正确判据**（首版按目录名扫，把无关仓库与活 worktree 都列成了待删项）：① `.git` 是**目录**（worktree 的 `.git` 是**文件**，`Test-Path` 对两者都为真，所以必须区分）；② 路径不在 `git worktree list` 里；③ `origin` 归一后与主树同仓库（`git@github.com:Owner/Repo.git` 与 `https://github.com/Owner/Repo.git` 视为同一个）。按此判据，`E:\deepseek-harness`（**别的仓库**）与 `E:\wt-tabbar-align`（**活 worktree**）必须排除。
- 主树 `E:\FL` 上的游离提交 `5446e6f`（重复 #849）**勿 push / 勿 reset**。

> 红线：**坏没坏要先实测**（上面的只读探测）。确认坏了之后也**不要**为了绕过它去删 `.git` 下的其他内容；只有本条明确列出的目录可删，且需管理员执行。**任何情况下都不要 `Stop-Process` 杀 HBuilderX**（见 `docs/adr/0008` 的单实例约定）。

## `E:` 盘是 exFAT：写入工具会失败（2026-09-13 实测）

`E:\` 与 `E:\FL` 所在卷的文件系统是 **exFAT**，而 exFAT **不支持硬链接**：

```powershell
Get-Volume | Where-Object DriveLetter -eq 'E' | Select DriveLetter, FileSystemType   # → exFAT
New-Item -ItemType HardLink -Path E:\_t.txt -Target E:\_gh\README.md                  # → Hard links are not supported
```

后果：**以「硬链接 + 原子改名」落盘的写入工具在该卷上会直接失败**，报 `EISDIR: illegal operation on a directory, link ... -> ...`。
症状是**对任何路径都失败**（含仓库根、含新建文件），很容易被误读成路径或权限问题。

**处置（按序取用）**：

1. **在 NTFS 卷上写，再拷回去**：写文件用 `E:\` 之外的暂存目录（`C:\` / `D:\` 都是 NTFS），再 `Copy-Item` 到目标路径 —— 字节原样、`*.ps1` / `*.js` / `*.mjs` 的 LF 不受影响。
2. **改已有文件**：把「精确旧文本 → 新文本」的替换对写成文件，用一个 10 行的 Node 脚本做**字面量拼接**（先断言锚点唯一），比在 shell 里做正则转义可靠。
   ⚠️ 拼接时**必须用函数式替换**（`s.replace(old, () => next)`）。用字符串替换会把替换文本里的 ``$` `` / `$'` / `$&` / `$1` 当成替换模式展开 —— 实测有一次把 PowerShell 脚本的**后半份整个复制了一遍**。
3. **不要**把仓库整体搬到 `C:` 来规避（空间与 clone 体积都不划算），也不要因此改用 `git worktree`（见上一节的 ACL 待办）。
