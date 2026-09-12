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

### ACL 修复（需管理员，一次性）

现状：`E:\FL\.git\worktrees` **存在但列不出内容**（`Get-ChildItem` 返回 0 项；`git worktree list` 尚可列注册项），导致 `git worktree add` 连续失败 ⇒ 多会话隔离临时改用独立 clone（每个都要单独 `npm ci`）。已核实该目录当前**无残留注册项**（`git worktree list` 只有主树一条）与**无残留目录**（枚举为空），所以下面「删掉让 git 重建」是低风险的。

优先配方——删掉坏目录让 git 重建（普通 PowerShell，路径无风险）：
```
Remove-Item -Recurse -Force E:\FL\.git\worktrees
cd E:\FL; git worktree prune
```
若删除失败或之后仍列不出内容，改对目录重置权限（**管理员** PowerShell）：
```
icacls "E:\FL\.git\worktrees" /reset /T /C
```
修好后先 `git worktree list`，再跑一次 `git worktree add` 试通（试通后可 `git worktree remove` 收回）。

⚠️ **该修复尚未执行**：截至 2026-09-12 仍是待办（`docs/agents/handoff-验收门-2026-09-11.md` 六、待办）。**修复前**多会话隔离继续用独立 clone，修好后再清理那些重复副本（已产生约 618 MB 重复副本，实测含 `node_modules` 的 6 个克隆目录合计约 892 MB）。

> 红线：ACL 坏着时**不要执行 `git worktree add`**（必失败），也**不要为了绕过它去删 `.git` 下的其他内容**——只有上面这一条明确列出的目录可删，且需管理员执行。

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
